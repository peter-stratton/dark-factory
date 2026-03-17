package agent

import (
	"context"
	"errors"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRun_NoSandbox_InvokesPython3(t *testing.T) {
	var capturedName string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedName = name
		return []byte(`{"session_id":"","result":"done","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	opts := RunOpts{Prompt: "test prompt"}
	result, err := Run(context.Background(), opts, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if capturedName != "python3" {
		t.Errorf("expected command 'python3', got %q", capturedName)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestRun_NoSandbox_ScriptPathAsArg(t *testing.T) {
	var capturedArgs []string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedArgs = args
		return []byte(`{"session_id":"","result":"done","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	_, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(capturedArgs) == 0 {
		t.Fatal("expected at least one arg (script path)")
	}
	if !strings.HasSuffix(capturedArgs[0], ".py") {
		t.Errorf("first arg should be .py file, got %q", capturedArgs[0])
	}
}

func TestRun_NoSandbox_PromptInEnv(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	opts := RunOpts{Prompt: "my special prompt"}
	_, err := Run(context.Background(), opts, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if capturedEnv["GODARK_PROMPT"] != "my special prompt" {
		t.Errorf("GODARK_PROMPT = %q, want %q", capturedEnv["GODARK_PROMPT"], "my special prompt")
	}
}

func TestRun_NoSandbox_RoleInEnv(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	opts := RunOpts{Prompt: "test", Role: "implementer"}
	_, err := Run(context.Background(), opts, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "implementer" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "implementer")
	}
}

func TestRun_NoSandbox_AuthEnvForwarded(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	opts := RunOpts{
		Prompt: "test",
		Env:    map[string]string{"GH_TOKEN": "tok-abc"},
	}
	_, err := Run(context.Background(), opts, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if capturedEnv["GH_TOKEN"] != "tok-abc" {
		t.Errorf("GH_TOKEN = %q, want %q", capturedEnv["GH_TOKEN"], "tok-abc")
	}
}

func TestRun_NoSandbox_ParsesStructuredResult(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		out := "some message line\n" + `{"session_id":"sess-123","result":"Implementation complete","cost_usd":0.42,"is_error":false}`
		return []byte(out), []byte(""), 0, nil
	})

	opts := RunOpts{Prompt: "test"}
	result, err := Run(context.Background(), opts, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "sess-123")
	}
	if result.CostUSD != 0.42 {
		t.Errorf("CostUSD = %v, want 0.42", result.CostUSD)
	}
}

func TestRun_NoSandbox_ErrorResult(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		out := `{"session_id":"sess-456","result":"error","cost_usd":0,"is_error":true}`
		return []byte(out), []byte(""), 0, nil
	})

	opts := RunOpts{Prompt: "test"}
	result, err := Run(context.Background(), opts, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1 for is_error=true", result.ExitCode)
	}
	if result.SessionID != "sess-456" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "sess-456")
	}
}

func TestRun_NoSandbox_NonZeroExit(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
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
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
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

func TestRun_NoSandbox_NoDangerouslySkipPermissions(t *testing.T) {
	var capturedArgs []string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedArgs = append([]string{name}, args...)
		return []byte(`{"session_id":"","result":"","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	_, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	joined := strings.Join(capturedArgs, " ")
	if strings.Contains(joined, "--dangerously-skip-permissions") {
		t.Error("should not contain --dangerously-skip-permissions")
	}
}

func TestRun_NoSandbox_ParsesVerdict(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		out := `{"session_id":"","result":"text","cost_usd":0,"is_error":false,"verdict":"APPROVED"}`
		return []byte(out), []byte(""), 0, nil
	})

	result, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}
}

func TestRun_NoSandbox_NoVerdictField_EmptyVerdict(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		// Implementer output — no verdict field.
		out := `{"session_id":"","result":"Implementation done","cost_usd":0,"is_error":false}`
		return []byte(out), []byte(""), 0, nil
	})

	result, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Verdict != "" {
		t.Errorf("Verdict = %q, want empty for non-reviewer output", result.Verdict)
	}
}

func TestRun_NoSandbox_ParsesToolTrace(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		out := `{"session_id":"","result":"done","cost_usd":0,"is_error":false,"tool_trace":["Read foo.go","Edit bar.go"]}`
		return []byte(out), []byte(""), 0, nil
	})

	result, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.ToolTrace) != 2 {
		t.Fatalf("ToolTrace len = %d, want 2", len(result.ToolTrace))
	}
	if result.ToolTrace[0] != "Read foo.go" {
		t.Errorf("ToolTrace[0] = %q, want %q", result.ToolTrace[0], "Read foo.go")
	}
	if result.ToolTrace[1] != "Edit bar.go" {
		t.Errorf("ToolTrace[1] = %q, want %q", result.ToolTrace[1], "Edit bar.go")
	}
}

func TestRun_NoSandbox_PromptNotInArgs(t *testing.T) {
	var capturedArgs []string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedArgs = append([]string{name}, args...)
		return []byte(`{"session_id":"","result":"","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	const prompt = "my unique prompt text"
	_, err := Run(context.Background(), RunOpts{Prompt: prompt}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	joined := strings.Join(capturedArgs, " ")
	if strings.Contains(joined, prompt) {
		t.Error("prompt should not appear in CLI args (it is passed via env var)")
	}
}

func TestRun_NoSandbox_TimingPopulated(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte(`{"session_id":"","result":"done","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	before := time.Now()
	result, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	after := time.Now()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.StartedAt.IsZero() {
		t.Error("StartedAt should be non-zero after Run()")
	}
	if result.FinishedAt.IsZero() {
		t.Error("FinishedAt should be non-zero after Run()")
	}
	if result.StartedAt.Before(before) {
		t.Errorf("StartedAt %v is before test start %v", result.StartedAt, before)
	}
	if result.FinishedAt.After(after) {
		t.Errorf("FinishedAt %v is after test end %v", result.FinishedAt, after)
	}
}

func TestRun_NoSandbox_DurationPositive(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte(`{"session_id":"","result":"done","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	result, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	duration := result.FinishedAt.Sub(result.StartedAt)
	if duration < 0 {
		t.Errorf("FinishedAt.Sub(StartedAt) = %v, want non-negative", duration)
	}
}

func TestRun_NoSandbox_TimedOut_HasTiming(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
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
	if result.StartedAt.IsZero() {
		t.Error("StartedAt should be non-zero even for timed-out runs")
	}
	if result.FinishedAt.IsZero() {
		t.Error("FinishedAt should be non-zero even for timed-out runs")
	}
}

func TestRun_NoSandbox_ResourceStats_Populated_MacOS(t *testing.T) {
	// Scenario: Host mode captures resource stats (macOS platform).
	// Maxrss = 204800 KB on macOS → PeakMemoryBytes = 209715200 (204800 * 1024).
	// Utime = 2s + Stime = 1s → CPUNanoseconds = 3000000000.
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte(`{"session_id":"","result":"done","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})
	stubGetrusageFn(t, func(who int, rusage *syscall.Rusage) error {
		rusage.Maxrss = 204800
		rusage.Utime = syscall.Timeval{Sec: 2, Usec: 0}
		rusage.Stime = syscall.Timeval{Sec: 1, Usec: 0}
		return nil
	})
	stubGOOS(t, "darwin")

	result, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	const wantMem = 204800 * 1024
	if result.PeakMemoryBytes != wantMem {
		t.Errorf("PeakMemoryBytes = %d, want %d", result.PeakMemoryBytes, wantMem)
	}
	const wantCPU = 3_000_000_000
	if result.CPUNanoseconds != wantCPU {
		t.Errorf("CPUNanoseconds = %d, want %d", result.CPUNanoseconds, wantCPU)
	}
}

func TestRun_NoSandbox_ResourceStats_GetrusageFailure_ZeroAndNoError(t *testing.T) {
	// Scenario: Getrusage failure returns zero values and no error.
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte(`{"session_id":"","result":"done","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})
	stubGetrusageFn(t, func(who int, rusage *syscall.Rusage) error {
		return errors.New("getrusage: operation not permitted")
	})

	result, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() returned error on getrusage failure = %v", err)
	}
	if result.PeakMemoryBytes != 0 {
		t.Errorf("PeakMemoryBytes = %d, want 0 on getrusage failure", result.PeakMemoryBytes)
	}
	if result.CPUNanoseconds != 0 {
		t.Errorf("CPUNanoseconds = %d, want 0 on getrusage failure", result.CPUNanoseconds)
	}
}

func TestRun_NoSandbox_ResourceStats_PlatformNormalization_MacOS(t *testing.T) {
	// Scenario: Platform normalization on macOS.
	// Maxrss = 1024 KB on macOS → PeakMemoryBytes = 1048576 (1024 * 1024).
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte(`{"session_id":"","result":"done","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})
	stubGetrusageFn(t, func(who int, rusage *syscall.Rusage) error {
		rusage.Maxrss = 1024
		return nil
	})
	stubGOOS(t, "darwin")

	result, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	const want = 1024 * 1024
	if result.PeakMemoryBytes != want {
		t.Errorf("PeakMemoryBytes = %d, want %d (macOS KB→bytes conversion)", result.PeakMemoryBytes, want)
	}
}

func TestRun_NoSandbox_ResourceStats_PlatformNormalization_Linux(t *testing.T) {
	// Scenario: Platform normalization on Linux.
	// Maxrss = 1048576 bytes on Linux → PeakMemoryBytes = 1048576 (no conversion).
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte(`{"session_id":"","result":"done","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})
	stubGetrusageFn(t, func(who int, rusage *syscall.Rusage) error {
		rusage.Maxrss = 1048576
		return nil
	})
	stubGOOS(t, "linux")

	result, err := Run(context.Background(), RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	const want = 1048576
	if result.PeakMemoryBytes != want {
		t.Errorf("PeakMemoryBytes = %d, want %d (Linux bytes, no conversion)", result.PeakMemoryBytes, want)
	}
}

func TestRun_NoSandbox_ResourceStats_SkippedOnTimeout(t *testing.T) {
	// On timeout the process may still be running, so resource fields should be zero.
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		<-ctx.Done()
		return []byte("partial"), []byte(""), 0, ctx.Err()
	})
	getrusageCalled := false
	stubGetrusageFn(t, func(who int, rusage *syscall.Rusage) error {
		getrusageCalled = true
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := Run(ctx, RunOpts{Prompt: "test"}, true, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.TimedOut {
		t.Error("expected TimedOut = true")
	}
	if getrusageCalled {
		t.Error("GetrusageFn should not be called on timeout")
	}
	if result.PeakMemoryBytes != 0 {
		t.Errorf("PeakMemoryBytes = %d, want 0 on timeout", result.PeakMemoryBytes)
	}
	if result.CPUNanoseconds != 0 {
		t.Errorf("CPUNanoseconds = %d, want 0 on timeout", result.CPUNanoseconds)
	}
}
