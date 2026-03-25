package agent

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/peter-stratton/dark-factory/internal/agent/judge"
	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

// stubSandboxRunner replaces SandboxRunner for the duration of the test.
func stubSandboxRunner(t *testing.T, fn func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error)) {
	t.Helper()
	orig := SandboxRunner
	t.Cleanup(func() { SandboxRunner = orig })
	SandboxRunner = fn
}

// enabledJudge returns a *config.Judge with judge enabled and a very short
// idle timeout so tests can trigger a kill without sleeping.
func enabledJudge(idleTimeoutSecs int) *config.Judge {
	enabled := true
	return &config.Judge{
		Enabled:            &enabled,
		DefaultIdleTimeout: idleTimeoutSecs,
	}
}

// disabledJudge returns a *config.Judge with judge explicitly disabled.
func disabledJudge() *config.Judge {
	disabled := false
	return &config.Judge{Enabled: &disabled}
}

// idleLineTrigger returns a log line that the idle timeout rule recognises as
// an activity event, followed by enough silence to trip the idle timeout.
// Since we cannot actually wait for time to pass in a unit test, we rely on
// the sandbox stub to call the LogCallback with lines and then verify that the
// judge is wired; the idle timeout rule test is covered in the judge package.
// These tests verify integration: judge created, callback wired, result fields set.

// sandboxResult builds a minimal sandbox.RunResult for stubs to return.
func sandboxResult(stdout string, exitCode int, timedOut bool) *sandbox.RunResult {
	return &sandbox.RunResult{
		Stdout:   stdout,
		ExitCode: exitCode,
		TimedOut: timedOut,
	}
}

// TestRunSandbox_JudgeDisabled verifies that no LogCallback is set when
// judge config has Enabled=false.
func TestRunSandbox_JudgeDisabled(t *testing.T) {
	var capturedOpts sandbox.RunOpts
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedOpts = opts
		return sandboxResult(`{"session_id":"s1","result":"ok","cost_usd":0,"is_error":false}`, 0, false), nil
	})

	opts := RunOpts{
		Prompt:      "test",
		Role:        "implementer",
		Repo:        "owner/repo",
		JudgeConfig: disabledJudge(),
	}
	result, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if capturedOpts.LogCallback != nil {
		t.Error("LogCallback should be nil when judge is disabled")
	}
	if result.JudgeKilled {
		t.Error("JudgeKilled should be false when judge is disabled")
	}
	if result.JudgeIntervention != nil {
		t.Error("JudgeIntervention should be nil when judge is disabled")
	}
}

// TestRunSandbox_JudgeNilConfig verifies that no LogCallback is set when
// JudgeConfig is nil (judge not configured).
func TestRunSandbox_JudgeNilConfig(t *testing.T) {
	var capturedOpts sandbox.RunOpts
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedOpts = opts
		return sandboxResult(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`, 0, false), nil
	})

	opts := RunOpts{
		Prompt:      "test",
		Repo:        "owner/repo",
		JudgeConfig: nil,
	}
	result, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if capturedOpts.LogCallback != nil {
		t.Error("LogCallback should be nil when JudgeConfig is nil")
	}
	if result.JudgeKilled {
		t.Error("JudgeKilled should be false when JudgeConfig is nil")
	}
}

// TestRunSandbox_JudgeEnabled_NoIntervention verifies that normal log lines
// do not trigger a kill and the result is clean.
func TestRunSandbox_JudgeEnabled_NoIntervention(t *testing.T) {
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		// Feed normal lines to the callback; none should trigger an intervention.
		if opts.LogCallback != nil {
			opts.LogCallback(`{"tool":"Read","path":"foo.go"}`)
			opts.LogCallback(`{"tool":"Edit","path":"bar.go"}`)
			opts.LogCallback("compilation succeeded")
		}
		return sandboxResult(`{"session_id":"s2","result":"done","cost_usd":0.1,"is_error":false}`, 0, false), nil
	})

	opts := RunOpts{
		Prompt:      "test",
		Role:        "implementer",
		Repo:        "owner/repo",
		JudgeConfig: enabledJudge(300),
	}
	result, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.JudgeKilled {
		t.Error("JudgeKilled should be false when no intervention fires")
	}
	if result.JudgeIntervention != nil {
		t.Errorf("JudgeIntervention should be nil, got %+v", result.JudgeIntervention)
	}
	if result.SessionID != "s2" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "s2")
	}
}

// TestRunSandbox_JudgeEnabled_LogCallbackWired verifies that the LogCallback
// is set when judge is enabled.
func TestRunSandbox_JudgeEnabled_LogCallbackWired(t *testing.T) {
	var capturedOpts sandbox.RunOpts
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedOpts = opts
		return sandboxResult(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`, 0, false), nil
	})

	opts := RunOpts{
		Prompt:      "test",
		Role:        "implementer",
		Repo:        "owner/repo",
		JudgeConfig: enabledJudge(300),
	}
	_, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if capturedOpts.LogCallback == nil {
		t.Error("LogCallback should be set when judge is enabled")
	}
}

// TestRunSandbox_KillStopsContainer verifies that a Kill intervention cancels
// the context (stopping the container) and sets JudgeKilled=true.
func TestRunSandbox_KillStopsContainer(t *testing.T) {
	var contextCancelled bool
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		// Simulate idle timeout by invoking the callback with a line, then
		// manipulating time by injecting an intervention directly via the callback.
		// We create a fake idle-timeout line that the judge would see after enough
		// idle time. Since we cannot control wall time in unit tests, we directly
		// verify that the context is cancelled by the cancel() call inside the
		// LogCallback when it receives an intervention.
		//
		// Instead, we bypass the idle rule and inject a kill intervention via the
		// tool-thrash rule: send the same ToolSearch query more than the threshold.
		if opts.LogCallback != nil {
			for i := 0; i < 5; i++ {
				opts.LogCallback(`ToolSearch("identical query")`)
			}
		}
		// Check if context was cancelled.
		select {
		case <-ctx.Done():
			contextCancelled = true
		default:
		}
		return sandboxResult("partial output", 1, false), nil
	})

	enabled := true
	opts := RunOpts{
		Prompt: "test",
		Role:   "implementer",
		Repo:   "owner/repo",
		JudgeConfig: &config.Judge{
			Enabled:              &enabled,
			ToolThrashThreshold:  3,
			ToolThrashWindowSecs: 60,
		},
	}
	result, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.JudgeKilled {
		t.Error("JudgeKilled should be true after Kill intervention")
	}
	if !contextCancelled {
		t.Error("context should be cancelled after Kill intervention")
	}
	if result.TimedOut {
		t.Error("TimedOut should be false when judge kills (not a timeout)")
	}
}

// TestRunSandbox_InterventionRecordPopulated verifies that JudgeIntervention
// contains rule name, detail, and timing after a kill.
func TestRunSandbox_InterventionRecordPopulated(t *testing.T) {
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		if opts.LogCallback != nil {
			// Trigger tool thrash: same query repeated.
			for i := 0; i < 5; i++ {
				opts.LogCallback(`ToolSearch("repeated query")`)
			}
		}
		return sandboxResult("", 1, false), nil
	})

	enabled := true
	opts := RunOpts{
		Prompt: "test",
		Role:   "implementer",
		Repo:   "owner/repo",
		JudgeConfig: &config.Judge{
			Enabled:              &enabled,
			ToolThrashThreshold:  3,
			ToolThrashWindowSecs: 60,
		},
	}
	result, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.JudgeIntervention == nil {
		t.Fatal("JudgeIntervention should be non-nil after kill")
	}
	iv := result.JudgeIntervention
	if iv.Rule == "" {
		t.Error("JudgeIntervention.Rule should be non-empty")
	}
	if iv.Judgment != judge.Kill {
		t.Errorf("JudgeIntervention.Judgment = %q, want %q", iv.Judgment, judge.Kill)
	}
	if iv.DetectedAt.IsZero() {
		t.Error("JudgeIntervention.DetectedAt should be non-zero")
	}
}

// TestRunSandbox_KillWithUsableResult verifies that when the agent prints its
// final result JSON before going idle and the judge kills the container, both
// JudgeKilled=true and the parsed result fields are populated.
func TestRunSandbox_KillWithUsableResult(t *testing.T) {
	const finalJSON = `{"session_id":"sess-xyz","result":"Implementation complete","cost_usd":0.55,"is_error":false,"verdict":"","tool_trace":["Read foo.go"]}`

	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		if opts.LogCallback != nil {
			// Trigger tool thrash kill.
			for i := 0; i < 5; i++ {
				opts.LogCallback(`ToolSearch("same search again")`)
			}
		}
		// Agent printed result JSON before going idle; it appears in stdout.
		return sandboxResult(finalJSON, 1, false), nil
	})

	enabled := true
	opts := RunOpts{
		Prompt: "test",
		Role:   "implementer",
		Repo:   "owner/repo",
		JudgeConfig: &config.Judge{
			Enabled:              &enabled,
			ToolThrashThreshold:  3,
			ToolThrashWindowSecs: 60,
		},
	}
	result, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.JudgeKilled {
		t.Error("JudgeKilled should be true")
	}
	if result.SessionID != "sess-xyz" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "sess-xyz")
	}
	if result.ResultText != "Implementation complete" {
		t.Errorf("ResultText = %q, want %q", result.ResultText, "Implementation complete")
	}
	if result.CostUSD != 0.55 {
		t.Errorf("CostUSD = %v, want 0.55", result.CostUSD)
	}
	if len(result.ToolTrace) != 1 || result.ToolTrace[0] != "Read foo.go" {
		t.Errorf("ToolTrace = %v, want [\"Read foo.go\"]", result.ToolTrace)
	}
}

// TestRunSandbox_JudgeEnabled_DefaultTimeout verifies that a nil Enabled field
// in config.Judge is treated as enabled (default-on behaviour).
func TestRunSandbox_JudgeEnabled_DefaultOn(t *testing.T) {
	var capturedOpts sandbox.RunOpts
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedOpts = opts
		return sandboxResult(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`, 0, false), nil
	})

	// Enabled field is nil — should default to enabled.
	opts := RunOpts{
		Prompt:      "test",
		Role:        "implementer",
		Repo:        "owner/repo",
		JudgeConfig: &config.Judge{DefaultIdleTimeout: 300},
	}
	_, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if capturedOpts.LogCallback == nil {
		t.Error("LogCallback should be set when Enabled is nil (default-on)")
	}
}

// TestRunSandbox_TimingFields verifies StartedAt and FinishedAt are set for
// sandbox runs.
func TestRunSandbox_TimingFields(t *testing.T) {
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		return sandboxResult(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`, 0, false), nil
	})

	before := time.Now()
	result, err := Run(context.Background(), RunOpts{Prompt: "test", Repo: "owner/repo"}, false, testLogger(t))
	after := time.Now()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.StartedAt.Before(before) || result.StartedAt.After(after) {
		t.Errorf("StartedAt %v not in [%v, %v]", result.StartedAt, before, after)
	}
	if result.FinishedAt.Before(result.StartedAt) {
		t.Errorf("FinishedAt %v is before StartedAt %v", result.FinishedAt, result.StartedAt)
	}
}

// --- Container retry tests ---

// TestRunSandbox_RetrySucceeds verifies that a transport failure on the first
// container triggers a retry, and the second container's result is returned.
func TestRunSandbox_RetrySucceeds(t *testing.T) {
	callCount := 0
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		callCount++
		if callCount == 1 {
			// First container: feed stream errors to trigger transport failure.
			if opts.LogCallback != nil {
				for i := 0; i < 12; i++ {
					opts.LogCallback("stream closed: EOF")
				}
			}
			return sandboxResult("", 1, false), nil
		}
		// Second container: success.
		if opts.LogCallback != nil {
			opts.LogCallback(`{"tool":"Read","path":"foo.go"}`)
		}
		return sandboxResult(`{"session_id":"s-ok","result":"done","cost_usd":0.2,"is_error":false}`, 0, false), nil
	})

	enabled := true
	opts := RunOpts{
		Prompt: "test",
		Role:   "implementer",
		Repo:   "owner/repo",
		JudgeConfig: &config.Judge{
			Enabled:                   &enabled,
			TransportFailureThreshold: 10,
			ContainerRetryLimit:       2,
		},
	}
	result, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if callCount != 2 {
		t.Errorf("SandboxRunner called %d times, want 2", callCount)
	}
	if result.JudgeKilled {
		t.Error("JudgeKilled should be false after successful retry")
	}
	if result.SessionID != "s-ok" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "s-ok")
	}
}

// TestRunSandbox_RetryLimitExhausted verifies that when all container retries
// hit transport failures, the final result has JudgeKilled=true.
func TestRunSandbox_RetryLimitExhausted(t *testing.T) {
	callCount := 0
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		callCount++
		// Every container hits transport failure.
		if opts.LogCallback != nil {
			for i := 0; i < 12; i++ {
				opts.LogCallback("stream closed: EOF")
			}
		}
		return sandboxResult("", 1, false), nil
	})

	enabled := true
	opts := RunOpts{
		Prompt: "test",
		Role:   "implementer",
		Repo:   "owner/repo",
		JudgeConfig: &config.Judge{
			Enabled:                   &enabled,
			TransportFailureThreshold: 10,
			ContainerRetryLimit:       2,
		},
	}
	result, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// 1 initial + 2 retries = 3 total calls.
	if callCount != 3 {
		t.Errorf("SandboxRunner called %d times, want 3", callCount)
	}
	if !result.JudgeKilled {
		t.Error("JudgeKilled should be true after exhausting retries")
	}
	if result.JudgeIntervention == nil {
		t.Fatal("JudgeIntervention should be non-nil")
	}
	if result.JudgeIntervention.Rule != "transport_failure" {
		t.Errorf("Rule = %q, want %q", result.JudgeIntervention.Rule, "transport_failure")
	}
}

// TestRunSandbox_KillDoesNotRetry verifies that a Kill judgment (not
// RetryContainer) does not trigger container retries.
func TestRunSandbox_KillDoesNotRetry(t *testing.T) {
	callCount := 0
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		callCount++
		// Trigger tool thrash → Kill (not RetryContainer).
		if opts.LogCallback != nil {
			for i := 0; i < 5; i++ {
				opts.LogCallback(`ToolSearch("same query")`)
			}
		}
		return sandboxResult("", 1, false), nil
	})

	enabled := true
	opts := RunOpts{
		Prompt: "test",
		Role:   "implementer",
		Repo:   "owner/repo",
		JudgeConfig: &config.Judge{
			Enabled:              &enabled,
			ToolThrashThreshold:  3,
			ToolThrashWindowSecs: 60,
			ContainerRetryLimit:  2,
		},
	}
	result, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("SandboxRunner called %d times, want 1 (no retry for Kill)", callCount)
	}
	if !result.JudgeKilled {
		t.Error("JudgeKilled should be true")
	}
}

// TestRunSandbox_RetryRunsTwiceOnTransportFailure is a simpler verification
// that the retry loop executes exactly N+1 times (1 initial + N retries)
// when transport failures persist, and the final result reflects the last
// container's state.
func TestRunSandbox_RetryRunsTwiceOnTransportFailure(t *testing.T) {
	callCount := 0
	stubSandboxRunner(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		callCount++
		if callCount <= 2 {
			// First two containers: transport failure.
			if opts.LogCallback != nil {
				for i := 0; i < 12; i++ {
					opts.LogCallback("stream error: connection reset")
				}
			}
			return sandboxResult("", 1, false), nil
		}
		// Third container: success.
		return sandboxResult(`{"session_id":"s3","result":"ok","cost_usd":0,"is_error":false}`, 0, false), nil
	})

	enabled := true
	opts := RunOpts{
		Prompt: "test",
		Role:   "implementer",
		Repo:   "owner/repo",
		JudgeConfig: &config.Judge{
			Enabled:                   &enabled,
			TransportFailureThreshold: 10,
			ContainerRetryLimit:       2,
		},
	}
	result, err := Run(context.Background(), opts, false, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if callCount != 3 {
		t.Errorf("SandboxRunner called %d times, want 3 (1 initial + 2 retries)", callCount)
	}
	if result.JudgeKilled {
		t.Error("JudgeKilled should be false after successful retry")
	}
	if result.SessionID != "s3" {
		t.Errorf("SessionID = %q, want %q (from third container)", result.SessionID, "s3")
	}
}
