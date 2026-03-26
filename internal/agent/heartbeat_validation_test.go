package agent

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

func TestHeartbeatFiresIdleTimeout(t *testing.T) {
	// Use a very short idle timeout and heartbeat interval.
	origInterval := judgeHeartbeatInterval
	judgeHeartbeatInterval = 100 * time.Millisecond
	t.Cleanup(func() { judgeHeartbeatInterval = origInterval })

	enabled := true
	var containerRanFor time.Duration

	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		// Send one tool call, then go silent.
		if opts.LogCallback != nil {
			opts.LogCallback(`{"tool": "Read", "path": "foo.go"}`)
		}
		// Block until the judge kills us via context cancellation.
		start := time.Now()
		<-ctx.Done()
		containerRanFor = time.Since(start)
		return sandboxResult("", 1, false), nil
	})

	opts := RunOpts{
		Prompt: "test",
		Role:   "recon",
		Repo:   "owner/repo",
		JudgeConfig: &config.Judge{
			Enabled:            &enabled,
			DefaultIdleTimeout: 1, // 1 second idle timeout
		},
	}

	result, err := Run(context.Background(), opts, testLogger(t))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.JudgeKilled {
		t.Fatal("expected JudgeKilled=true, heartbeat did not fire")
	}
	if result.JudgeIntervention == nil {
		t.Fatal("expected JudgeIntervention to be non-nil")
	}
	if result.JudgeIntervention.Rule != "idle_timeout" {
		t.Errorf("Rule = %q, want idle_timeout", result.JudgeIntervention.Rule)
	}
	// Should have been killed within ~1.2s (1s threshold + heartbeat tick)
	if containerRanFor > 5*time.Second {
		t.Errorf("container ran for %s, expected <5s with 1s idle timeout", containerRanFor)
	}
	t.Logf("container killed after %s (idle timeout 1s, heartbeat 100ms)", containerRanFor)
}
