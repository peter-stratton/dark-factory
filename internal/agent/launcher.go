package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/peter-stratton/dark-factory/internal/agent/judge"
	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/ghapp"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

// RunOpts configures an agent invocation.
type RunOpts struct {
	Prompt            string
	Role              string
	Env               map[string]string
	Image             string
	Repo              string
	Branch            string
	WorkDir           string
	Timeout           time.Duration
	Model             string        // claude CLI model alias: "sonnet", "opus", or "" (default)
	MountDockerSocket bool          // mount /var/run/docker.sock into sandbox container
	JudgeConfig       *config.Judge // nil means judge is disabled
}

// Result holds the outcome of an agent run.
type Result struct {
	ExitCode          int
	Stdout            string
	Stderr            string
	TimedOut          bool
	JudgeKilled       bool                // true when the judge stopped the container early
	JudgeIntervention *judge.Intervention // the intervention that triggered a kill, if any
	SessionID         string
	CostUSD           float64
	ResultText        string   // agent's final text output (from SDK result message)
	Verdict           string   // review verdict: "APPROVED", "CHANGES_REQUESTED", or "" (reviewer only)
	ToolTrace         []string // summary of tool calls made by the agent
	StartedAt         time.Time
	FinishedAt        time.Time
	ContainerLog      string // bounded combined log for post-mortem; only populated on failure
	PeakMemoryBytes   int64  // peak RSS in bytes; 0 if unavailable
	CPUNanoseconds    int64  // total CPU time (user + system) in nanoseconds; 0 if unavailable
	CloneSHA          string // HEAD SHA captured inside the container immediately after clone
	UsageLimited      bool      // true when Claude hit its usage limit during this run
	ResetsAt          time.Time // when the usage limit resets; zero if unknown
}

// SandboxRunner executes a container run. Replaceable for testing.
var SandboxRunner = func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
	return sandbox.RunContainer(ctx, opts, logger)
}

// Run invokes a Claude Code agent with the given prompt inside a Docker container.
func Run(ctx context.Context, opts RunOpts, logger *slog.Logger) (*Result, error) {
	return runSandbox(ctx, opts, logger)
}

func refreshGHToken(env map[string]string, logger *slog.Logger) {
	token, err := ghapp.RefreshToken()
	if err != nil {
		logger.Warn("failed to refresh GitHub App token, using existing token", "error", err)
		return
	}
	if token == "" {
		return // no GitHub App configured, nothing to refresh
	}
	env["GH_TOKEN"] = token
	// Update the host process environment so gh CLI commands that run outside
	// the container (e.g. FindPR, EnsureClosesRef) also use the fresh token.
	if err := os.Setenv("GH_TOKEN", token); err != nil {
		logger.Warn("failed to update host GH_TOKEN", "error", err)
	}
	logger.Info("refreshed GitHub App installation token")
}

// judgeHeartbeatInterval controls how often the heartbeat tick is sent to the
// judge so it can detect idle timeouts even when no log lines arrive.
// Replaceable for testing.
var judgeHeartbeatInterval = 30 * time.Second

// buildJudgeCallback creates a LogCallback for the sandbox that feeds lines
// to the judge, and starts a heartbeat goroutine that periodically ticks the
// judge so idle timeouts fire even when the container produces no output.
// Returns nil when the judge is disabled or not configured.
func buildJudgeCallback(
	ctx context.Context,
	opts RunOpts,
	cancel context.CancelFunc,
	interventionPtr *atomic.Pointer[judge.Intervention],
	judgeKilled *atomic.Bool,
	logger *slog.Logger,
) func(line string) {
	if opts.JudgeConfig == nil {
		return nil
	}
	if opts.JudgeConfig.Enabled != nil && !*opts.JudgeConfig.Enabled {
		return nil
	}

	j := judge.NewJudge(opts.Role, judge.Config{
		IdleTimeoutByRole:         opts.JudgeConfig.IdleTimeoutByRole,
		DefaultIdleTimeout:        opts.JudgeConfig.DefaultIdleTimeout,
		NoProgressTimeoutByRole:   opts.JudgeConfig.NoProgressTimeoutByRole,
		DefaultNoProgressTimeout:  opts.JudgeConfig.DefaultNoProgressTimeout,
		ToolThrashThreshold:       opts.JudgeConfig.ToolThrashThreshold,
		ToolThrashWindowSecs:      opts.JudgeConfig.ToolThrashWindowSecs,
		TransportFailureThreshold: opts.JudgeConfig.TransportFailureThreshold,
	})
	logger.Info("judge enabled for sandbox run", "role", opts.Role)

	// Mutex serializes access to the judge since both the log callback
	// and the heartbeat goroutine call ProcessLine concurrently.
	var mu sync.Mutex

	processAndHandle := func(line string) {
		mu.Lock()
		iv := j.ProcessLine(line, time.Now())
		mu.Unlock()
		if iv == nil {
			return
		}
		interventionPtr.CompareAndSwap(nil, iv)
		if iv.Judgment == judge.Kill || iv.Judgment == judge.RetryContainer {
			judgeKilled.Store(true)
			cancel()
		}
	}

	// Heartbeat: tick the judge periodically so idle timeouts fire even
	// when the container produces no log output.
	go func() {
		ticker := time.NewTicker(judgeHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processAndHandle("")
			}
		}
	}()

	return func(line string) {
		processAndHandle(line)
	}
}

func runSandbox(ctx context.Context, opts RunOpts, logger *slog.Logger) (*Result, error) {
	logger.Info("running agent in sandbox", "image", opts.Image, "timeout", opts.Timeout, "prompt_bytes", len(opts.Prompt), "role", opts.Role)

	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "/workspace"
	}

	// Pass the prompt and role via environment variables to avoid shell quoting issues.
	env := make(map[string]string, len(opts.Env)+2)
	for k, v := range opts.Env {
		env[k] = v
	}
	env["GODARK_PROMPT"] = opts.Prompt
	if opts.Role != "" {
		env["GODARK_ROLE"] = opts.Role
	}

	// Refresh GitHub App token so each container gets a fresh 1-hour token.
	refreshGHToken(env, logger)

	// Invoke the Claude CLI directly instead of the Python SDK wrapper.
	// stream-json output gives us real-time tool call visibility and a
	// structured result line the judge and parseRunnerOutput both understand.
	// --verbose is required for stream-json in print mode.
	agentCmd := "cd " + workDir + " && claude -p" +
		" --output-format stream-json" +
		" --verbose" +
		" --permission-mode bypassPermissions" +
		" --setting-sources project"
	if opts.Model != "" {
		agentCmd += " --model " + opts.Model
	}
	agentCmd += ` "$GODARK_PROMPT"`
	cloneScript, err := sandbox.CloneScript(opts.Repo, opts.Branch, workDir)
	if err != nil {
		return nil, fmt.Errorf("building clone script: %w", err)
	}
	entrypoint := sandbox.EntrypointScript(cloneScript, agentCmd)

	// Determine the container retry limit for transport failures.
	containerRetryLimit := 0
	if opts.JudgeConfig != nil {
		containerRetryLimit = opts.JudgeConfig.ContainerRetryLimit
		if containerRetryLimit == 0 {
			containerRetryLimit = 2 // default
		}
	}

	sandboxOpts := sandbox.RunOpts{
		Image:             opts.Image,
		Cmd:               []string{"sh", "-c", entrypoint},
		Env:               env,
		Timeout:           opts.Timeout,
		MountDockerSocket: opts.MountDockerSocket,
	}

	startedAt := time.Now()
	var res *Result

	for attempt := 0; attempt <= containerRetryLimit; attempt++ {
		if attempt > 0 {
			logger.Warn("retrying container after transport failure",
				"attempt", attempt+1,
				"max_attempts", containerRetryLimit+1,
			)
			refreshGHToken(env, logger)
		}

		var err error
		res, err = runSandboxOnce(ctx, opts, sandboxOpts, startedAt, logger)
		if err != nil {
			return nil, err
		}

		// If judge returned RetryContainer and we have retries left, loop.
		if res.JudgeKilled && res.JudgeIntervention != nil &&
			res.JudgeIntervention.Judgment == judge.RetryContainer &&
			attempt < containerRetryLimit {
			logger.Warn("judge recommends container retry",
				"rule", res.JudgeIntervention.Rule,
				"detail", res.JudgeIntervention.Detail,
				"attempt", attempt+1,
			)
			continue
		}
		break
	}

	return res, nil
}

// runSandboxOnce executes a single container attempt with judge monitoring.
// It creates a fresh judge and context cancel per attempt so previous state
// doesn't carry over.
func runSandboxOnce(ctx context.Context, opts RunOpts, sandboxOpts sandbox.RunOpts, startedAt time.Time, logger *slog.Logger) (*Result, error) {
	attemptCtx, attemptCancel := context.WithCancel(ctx)
	defer attemptCancel()

	var (
		interventionPtr atomic.Pointer[judge.Intervention]
		judgeKilled     atomic.Bool
	)
	sandboxOpts.LogCallback = buildJudgeCallback(attemptCtx, opts, attemptCancel, &interventionPtr, &judgeKilled, logger)

	result, err := SandboxRunner(attemptCtx, sandboxOpts, logger)
	if err != nil {
		return nil, fmt.Errorf("running sandbox container: %w", err)
	}

	if result.ExitCode != 0 && !result.TimedOut {
		logger.Warn("container exited with error",
			"exit_code", result.ExitCode,
			"stderr", truncate(result.Stderr, 2000),
			"stdout_tail", truncate(result.Stdout, 500),
		)
	}

	killed := judgeKilled.Load()
	res := &Result{
		ExitCode:          result.ExitCode,
		Stdout:            result.Stdout,
		Stderr:            result.Stderr,
		TimedOut:          result.TimedOut && !killed,
		JudgeKilled:       killed,
		JudgeIntervention: interventionPtr.Load(),
		StartedAt:         startedAt,
		FinishedAt:        time.Now(),
		PeakMemoryBytes:   result.PeakMemoryBytes,
		CPUNanoseconds:    result.CPUNanoseconds,
	}

	// Detect usage limit before processing the runner output so the IsError
	// guard below can check res.UsageLimited correctly.
	if rawLine, resetsAt := parseRateLimitEvent(result.Stdout); !resetsAt.IsZero() {
		res.UsageLimited = true
		res.ResetsAt = resetsAt
		logger.Warn("rate_limit_event detected in container stdout",
			"raw_json", rawLine,
			"resets_at", resetsAt,
			"resets_at_unix", resetsAt.Unix(),
			"exit_code", res.ExitCode,
		)
	}

	if parsed := parseRunnerOutput(result.Stdout); parsed != nil {
		res.SessionID = parsed.SessionID
		res.CostUSD = parsed.CostUSD
		res.ResultText = parsed.Result
		res.Verdict = parsed.Verdict
		res.ToolTrace = parsed.ToolTrace
		if parsed.IsError && res.ExitCode == 0 && !res.UsageLimited {
			res.ExitCode = 1
		}
		// CLI stream-json mode doesn't include verdict or tool_trace
		// fields. Extract them from the output.
		if res.Verdict == "" && res.ResultText != "" {
			res.Verdict = extractVerdict(res.ResultText)
		}
		if len(res.ToolTrace) == 0 {
			res.ToolTrace = extractToolTrace(result.Stdout)
		}
	}

	// Belt-and-suspenders fallback: detect usage limit from result text if
	// the structured rate_limit_event was not emitted.
	if !res.UsageLimited && strings.Contains(res.ResultText, "You've hit your limit") {
		res.UsageLimited = true
		// ResetsAt stays zero — orchestrator will apply max-hold guard.
	}

	res.CloneSHA = extractCloneSHA(result.Stdout)

	// Capture bounded container log for post-mortem on failure only.
	if res.TimedOut || res.JudgeKilled || res.ExitCode != 0 {
		combined := result.Stdout + result.Stderr
		res.ContainerLog = boundLog(combined, maxPostMortemLines, maxPostMortemBytes)
	}

	return res, nil
}

// runnerFinalResult is the structured JSON output printed as the last line by
// either agent_runner.py (SDK mode) or the Claude CLI (stream-json mode).
//
// SDK format:  {"session_id":"...", "result":"...", "cost_usd":0.04, "is_error":false}
// CLI format:  {"type":"result", "session_id":"...", "result":"...", "total_cost_usd":0.04, "is_error":false}
type runnerFinalResult struct {
	Type          string   `json:"type,omitempty"`           // "result" in CLI stream-json mode
	SessionID     string   `json:"session_id"`
	Result        string   `json:"result"`
	CostUSD       float64  `json:"cost_usd"`
	TotalCostUSD  float64  `json:"total_cost_usd,omitempty"` // CLI stream-json uses this field
	IsError       bool     `json:"is_error"`
	Verdict       string   `json:"verdict,omitempty"`
	ToolTrace     []string `json:"tool_trace,omitempty"`
}

// rateLimitEvent is the JSON structure emitted by the Claude CLI when a usage
// limit is hit during a stream-json run.
type rateLimitEvent struct {
	Type          string `json:"type"`
	RateLimitInfo struct {
		Status        string `json:"status"`
		ResetsAt      int64  `json:"resetsAt"` // Unix timestamp
		RateLimitType string `json:"rateLimitType"`
	} `json:"rate_limit_info"`
}

// parseRateLimitEvent scans stdout for a rate_limit_event line and returns the
// raw JSON line and parsed reset time. Returns ("", zero Time) if no such event is found.
func parseRateLimitEvent(stdout string) (string, time.Time) {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, `"rate_limit_event"`) {
			continue
		}
		var ev rateLimitEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Type == "rate_limit_event" && ev.RateLimitInfo.ResetsAt > 0 {
			return line, time.Unix(ev.RateLimitInfo.ResetsAt, 0)
		}
	}
	return "", time.Time{}
}

// parseRunnerOutput extracts the structured final result from runner stdout.
// It scans from the end of stdout for the last valid JSON line that looks like
// a result (has session_id or type=="result").
func parseRunnerOutput(stdout string) *runnerFinalResult {
	lines := strings.Split(stdout, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var r runnerFinalResult
		if err := json.Unmarshal([]byte(line), &r); err == nil {
			// Normalize: CLI stream-json uses total_cost_usd, SDK uses cost_usd.
			if r.CostUSD == 0 && r.TotalCostUSD > 0 {
				r.CostUSD = r.TotalCostUSD
			}
			return &r
		}
		break
	}
	return nil
}

// truncate returns the last n bytes of s.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}

// extractToolTrace scans CLI stream-json stdout for assistant messages
// containing tool_use content blocks and builds a tool trace summary.
func extractToolTrace(stdout string) []string {
	var trace []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, `"tool_use"`) {
			continue
		}
		var msg struct {
			Type    string `json:"type"`
			Message struct {
				Content []struct {
					Type  string         `json:"type"`
					Name  string         `json:"name"`
					Input map[string]any `json:"input"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &msg) != nil || msg.Type != "assistant" {
			continue
		}
		for _, block := range msg.Message.Content {
			if block.Type != "tool_use" || block.Name == "" {
				continue
			}
			switch block.Name {
			case "Edit", "Write", "Read":
				fp, _ := block.Input["file_path"].(string)
				if fp != "" {
					trace = append(trace, block.Name+" "+fp)
				} else {
					trace = append(trace, block.Name)
				}
			case "Bash":
				cmd, _ := block.Input["command"].(string)
				if len(cmd) > 80 {
					cmd = cmd[:80]
				}
				trace = append(trace, "Bash: "+cmd)
			default:
				trace = append(trace, block.Name)
			}
		}
	}
	return trace
}

var cloneSHARe = regexp.MustCompile(`(?m)^CLONE_SHA=([0-9a-f]{40})$`)

// extractCloneSHA scans container stdout for the CLONE_SHA sentinel line
// emitted by CloneScript and returns the 40-character hex SHA, or "" if absent.
func extractCloneSHA(stdout string) string {
	m := cloneSHARe.FindStringSubmatch(stdout)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

var verdictRe = regexp.MustCompile(`(?i)REVIEW\s+RESULT:\s*(APPROVED|CHANGES_REQUESTED)`)

// extractVerdict scans text for a "REVIEW RESULT: ..." pattern and returns
// the normalized verdict string, or "" if none found.
func extractVerdict(text string) string {
	m := verdictRe.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.ToUpper(m[1])
}
