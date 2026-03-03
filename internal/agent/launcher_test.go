package agent

import (
	"context"
	"strings"
	"testing"
)

func TestRun_NoSandbox_DispatchesToRunner(t *testing.T) {
	var capturedArgs []string
	stubRunnerFunc(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		capturedArgs = append([]string{name}, args...)
		return []byte("out"), []byte("err"), 0, nil
	})

	opts := RunOpts{
		Prompt:      "test prompt",
		ClaudeFlags: []string{"--model", "opus"},
	}
	result, err := Run(context.Background(), opts, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Stdout != "out" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "out")
	}

	// Check claude was called with correct args
	if capturedArgs[0] != "claude" {
		t.Errorf("expected command 'claude', got %q", capturedArgs[0])
	}
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "test prompt") {
		t.Error("expected prompt in args")
	}
	if !strings.Contains(joined, "--model") {
		t.Error("expected ClaudeFlags in args")
	}
	if !strings.Contains(joined, "--dangerously-skip-permissions") {
		t.Error("expected --dangerously-skip-permissions in args")
	}
}

func TestRun_NoSandbox_NonZeroExit(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte("out"), []byte("err"), 1, nil
	})

	result, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
}

func TestRun_NoSandbox_Timeout(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		<-ctx.Done()
		return []byte("partial"), []byte(""), 0, ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	result, err := Run(ctx, RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.TimedOut {
		t.Error("expected TimedOut = true")
	}
}
