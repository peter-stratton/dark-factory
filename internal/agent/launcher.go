package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/phs/dark-factory/internal/sandbox"
)

// RunOpts configures an agent invocation.
type RunOpts struct {
	Prompt      string
	Env         map[string]string
	Image       string
	Repo        string
	Branch      string
	WorkDir     string
	ClaudeFlags []string
	Timeout     time.Duration
}

// Result holds the outcome of an agent run.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
}

// Runner executes a command on the host. Replaceable for testing.
var Runner = func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
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
	return outBuf, errBuf, code, err
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

func runHost(ctx context.Context, opts RunOpts, logger *slog.Logger) (*Result, error) {
	logger.Info("running agent on host", "timeout", opts.Timeout)

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	args := []string{"-p", "--dangerously-skip-permissions", opts.Prompt}
	args = append(args, opts.ClaudeFlags...)

	stdout, stderr, exitCode, err := Runner(ctx, "claude", args...)
	if ctx.Err() != nil {
		return &Result{TimedOut: true, Stdout: string(stdout), Stderr: string(stderr)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("running claude on host: %w", err)
	}

	logger.Info("host agent finished", "exit_code", exitCode)
	return &Result{
		ExitCode: exitCode,
		Stdout:   string(stdout),
		Stderr:   string(stderr),
	}, nil
}

func runSandbox(ctx context.Context, opts RunOpts, logger *slog.Logger) (*Result, error) {
	logger.Info("running agent in sandbox", "image", opts.Image, "timeout", opts.Timeout)

	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "/workspace"
	}

	// Pass the prompt via environment variable to avoid shell quoting issues.
	env := make(map[string]string, len(opts.Env)+1)
	for k, v := range opts.Env {
		env[k] = v
	}
	env["GODARK_PROMPT"] = opts.Prompt

	claudeArgs := "claude -p --dangerously-skip-permissions \"$GODARK_PROMPT\""
	for _, f := range opts.ClaudeFlags {
		claudeArgs += " " + fmt.Sprintf("%q", f)
	}

	cloneScript := sandbox.CloneScript(opts.Repo, opts.Branch, workDir)
	entrypoint := sandbox.EntrypointScript(cloneScript, "cd "+workDir+" && "+claudeArgs)

	sandboxOpts := sandbox.RunOpts{
		Image:   opts.Image,
		Cmd:     []string{"sh", "-c", entrypoint},
		Env:     env,
		Timeout: opts.Timeout,
	}

	result, err := sandbox.RunContainer(ctx, sandboxOpts, logger)
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

	return &Result{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		TimedOut: result.TimedOut,
	}, nil
}

// truncate returns the last n bytes of s.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
