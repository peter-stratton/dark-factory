package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/peter-stratton/dark-factory/internal/agent/runner"
	"github.com/peter-stratton/dark-factory/internal/ghapp"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

// RunOpts configures an agent invocation.
type RunOpts struct {
	Prompt  string
	Role    string
	Env     map[string]string
	Image   string
	Repo    string
	Branch  string
	WorkDir string
	Timeout time.Duration
}

// Result holds the outcome of an agent run.
type Result struct {
	ExitCode        int
	Stdout          string
	Stderr          string
	TimedOut        bool
	SessionID       string
	CostUSD         float64
	ResultText      string   // agent's final text output (from SDK result message)
	Verdict         string   // review verdict: "APPROVED", "CHANGES_REQUESTED", or "" (reviewer only)
	ToolTrace       []string // summary of tool calls made by the agent
	StartedAt       time.Time
	FinishedAt      time.Time
	ContainerLog    string // bounded combined log for post-mortem; only populated on failure
	PeakMemoryBytes int64  // peak RSS in bytes; 0 if unavailable
	CPUNanoseconds  int64  // total CPU time (user + system) in nanoseconds; 0 if unavailable
}

// goosForRusage is the GOOS value used for Maxrss unit normalization.
// On macOS, Maxrss is in kilobytes; on Linux it is in bytes.
// Replaceable for testing to validate both platform behaviours.
var goosForRusage = runtime.GOOS

// Runner executes a command on the host with the given environment. Replaceable for testing.
// The returned *syscall.Rusage comes from cmd.ProcessState (populated by wait4) and
// contains accurate per-child resource usage including peak RSS. This is more reliable
// than a separate getrusage(RUSAGE_CHILDREN) call, which returns 0 for Maxrss on macOS.
var Runner = func(ctx context.Context, env map[string]string, name string, args ...string) (stdout, stderr []byte, exitCode int, rusage *syscall.Rusage, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	var outBuf, errBuf []byte
	cmd.Stdout = &writerFunc{fn: func(p []byte) { outBuf = append(outBuf, p...) }}
	cmd.Stderr = &writerFunc{fn: func(p []byte) { errBuf = append(errBuf, p...) }}
	err = cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
			err = nil // non-zero exit is not an error for us
		}
	}
	if cmd.ProcessState != nil {
		if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
			rusage = ru
		}
	}
	return outBuf, errBuf, code, rusage, err
}

// writerFunc adapts a function to the io.Writer interface.
type writerFunc struct {
	fn func([]byte)
}

func (w *writerFunc) Write(p []byte) (int, error) {
	w.fn(p)
	return len(p), nil
}

// Run invokes a Claude Code agent with the given prompt, either inside a
// Docker container (sandbox mode) or directly on the host (no-sandbox mode).
func Run(ctx context.Context, opts RunOpts, noSandbox bool, logger *slog.Logger) (*Result, error) {
	if noSandbox {
		return runHost(ctx, opts, logger)
	}
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

func runHost(ctx context.Context, opts RunOpts, logger *slog.Logger) (*Result, error) {
	logger.Info("running agent on host", "timeout", opts.Timeout)

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Write embedded agent_runner.py to a temp file.
	pyContent, err := runner.FS.ReadFile("agent_runner.py")
	if err != nil {
		return nil, fmt.Errorf("reading embedded agent_runner.py: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "godark-agent-*.py")
	if err != nil {
		return nil, fmt.Errorf("creating temp file for agent runner: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(pyContent); err != nil {
		_ = tmpFile.Close()
		return nil, fmt.Errorf("writing agent_runner.py to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("closing temp agent runner file: %w", err)
	}

	// Build env vars: merge RunOpts.Env, then add GODARK_* vars.
	env := make(map[string]string, len(opts.Env)+2)
	for k, v := range opts.Env {
		env[k] = v
	}
	env["GODARK_PROMPT"] = opts.Prompt
	if opts.Role != "" {
		env["GODARK_ROLE"] = opts.Role
	}

	// Refresh GitHub App token so each agent invocation gets a fresh 1-hour token.
	refreshGHToken(env, logger)

	startedAt := time.Now()
	stdout, stderr, exitCode, rusage, err := Runner(ctx, env, "python3", tmpFile.Name())
	finishedAt := time.Now()
	if ctx.Err() != nil {
		return &Result{TimedOut: true, Stdout: string(stdout), Stderr: string(stderr), StartedAt: startedAt, FinishedAt: finishedAt}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("running agent runner on host: %w", err)
	}

	res := &Result{
		ExitCode:   exitCode,
		Stdout:     string(stdout),
		Stderr:     string(stderr),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}

	if parsed := parseRunnerOutput(string(stdout)); parsed != nil {
		res.SessionID = parsed.SessionID
		res.CostUSD = parsed.CostUSD
		res.ResultText = parsed.Result
		res.Verdict = parsed.Verdict
		res.ToolTrace = parsed.ToolTrace
		if parsed.IsError && res.ExitCode == 0 {
			res.ExitCode = 1
		}
	}

	// Capture resource usage from the child process (best-effort).
	// The rusage comes from cmd.ProcessState (populated by wait4), which gives
	// accurate per-child stats including peak RSS. This is more reliable than
	// a separate getrusage(RUSAGE_CHILDREN) call, which returns 0 for Maxrss
	// on macOS.
	if rusage != nil {
		mem := int64(rusage.Maxrss)
		if goosForRusage == "darwin" {
			mem *= 1024 // macOS reports Maxrss in kilobytes; convert to bytes
		}
		res.PeakMemoryBytes = mem
		res.CPUNanoseconds = (int64(rusage.Utime.Sec)+int64(rusage.Stime.Sec))*1e9 +
			(int64(rusage.Utime.Usec)+int64(rusage.Stime.Usec))*1e3
	}

	// Capture bounded log for post-mortem on failure only.
	if res.TimedOut || res.ExitCode != 0 {
		combined := string(stdout) + string(stderr)
		res.ContainerLog = boundLog(combined, maxPostMortemLines, maxPostMortemBytes)
	}

	logger.Info("host agent finished", "exit_code", res.ExitCode)
	return res, nil
}

func runSandbox(ctx context.Context, opts RunOpts, logger *slog.Logger) (*Result, error) {
	logger.Info("running agent in sandbox", "image", opts.Image, "timeout", opts.Timeout)

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

	agentCmd := "cd " + workDir + " && python3 /usr/local/bin/agent_runner.py"
	cloneScript, err := sandbox.CloneScript(opts.Repo, opts.Branch, workDir)
	if err != nil {
		return nil, fmt.Errorf("building clone script: %w", err)
	}
	entrypoint := sandbox.EntrypointScript(cloneScript, agentCmd)

	sandboxOpts := sandbox.RunOpts{
		Image:   opts.Image,
		Cmd:     []string{"sh", "-c", entrypoint},
		Env:     env,
		Timeout: opts.Timeout,
	}

	startedAt := time.Now()
	result, err := sandbox.RunContainer(ctx, sandboxOpts, logger)
	finishedAt := time.Now()
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

	res := &Result{
		ExitCode:        result.ExitCode,
		Stdout:          result.Stdout,
		Stderr:          result.Stderr,
		TimedOut:        result.TimedOut,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		PeakMemoryBytes: result.PeakMemoryBytes,
		CPUNanoseconds:  result.CPUNanoseconds,
	}

	if parsed := parseRunnerOutput(result.Stdout); parsed != nil {
		res.SessionID = parsed.SessionID
		res.CostUSD = parsed.CostUSD
		res.ResultText = parsed.Result
		res.Verdict = parsed.Verdict
		res.ToolTrace = parsed.ToolTrace
		if parsed.IsError && res.ExitCode == 0 {
			res.ExitCode = 1
		}
	}

	// Capture bounded container log for post-mortem on failure only.
	// Must run after parseRunnerOutput so ExitCode reflects IsError upgrades.
	if res.TimedOut || res.ExitCode != 0 {
		combined := result.Stdout + result.Stderr
		res.ContainerLog = boundLog(combined, maxPostMortemLines, maxPostMortemBytes)
	}

	return res, nil
}

// runnerFinalResult is the structured JSON output printed as the last line by agent_runner.py.
type runnerFinalResult struct {
	SessionID string   `json:"session_id"`
	Result    string   `json:"result"`
	CostUSD   float64  `json:"cost_usd"`
	IsError   bool     `json:"is_error"`
	Verdict   string   `json:"verdict,omitempty"`
	ToolTrace []string `json:"tool_trace,omitempty"`
}

// parseRunnerOutput extracts the structured final result from runner stdout.
// The runner prints a final JSON line with session_id, result, cost_usd, and is_error.
func parseRunnerOutput(stdout string) *runnerFinalResult {
	lines := strings.Split(stdout, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var r runnerFinalResult
		if err := json.Unmarshal([]byte(line), &r); err == nil {
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
