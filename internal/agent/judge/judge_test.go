package judge

import (
	"testing"
	"time"
)

var t0 = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func TestIdleTimeoutFires(t *testing.T) {
	j := NewJudge("recon", Config{DefaultIdleTimeout: 300})

	// First line starts the clock at t0.
	got := j.ProcessLine("some assistant text", t0)
	if got != nil {
		t.Fatalf("expected nil at t0, got %+v", got)
	}

	// Advance just under threshold — should still be nil.
	got = j.ProcessLine("more text", t0.Add(299*time.Second))
	if got != nil {
		t.Fatalf("expected nil before threshold, got %+v", got)
	}

	// Advance past threshold — Kill should fire.
	got = j.ProcessLine("more text", t0.Add(301*time.Second))
	if got == nil {
		t.Fatal("expected Kill intervention, got nil")
	}
	if got.Judgment != Kill {
		t.Errorf("got judgment %q, want %q", got.Judgment, Kill)
	}
	if got.Rule != "idle_timeout" {
		t.Errorf("got rule %q, want %q", got.Rule, "idle_timeout")
	}
	if got.Detail == "" {
		t.Error("expected non-empty detail")
	}
	if got.Counts["idle_seconds"] == 0 {
		t.Error("expected idle_seconds > 0 in Counts")
	}
}

func TestIdleTimeoutResetsOnToolCall(t *testing.T) {
	j := NewJudge("recon", Config{DefaultIdleTimeout: 300})

	// Start clock.
	j.ProcessLine("assistant text", t0)

	// Send non-tool lines up to just before threshold.
	j.ProcessLine("more text", t0.Add(250*time.Second))

	// Tool call line resets the clock.
	j.ProcessLine(`{"tool": "bash", "input": {}}`, t0.Add(290*time.Second))

	// Advance 290s past the tool call — still under threshold.
	got := j.ProcessLine("text after tool", t0.Add(290*time.Second+289*time.Second))
	if got != nil {
		t.Fatalf("expected nil within threshold after reset, got %+v", got)
	}

	// Advance past threshold from the tool call.
	got = j.ProcessLine("text after tool", t0.Add(290*time.Second+301*time.Second))
	if got == nil {
		t.Fatal("expected Kill intervention after threshold exceeded post-reset, got nil")
	}
	if got.Judgment != Kill {
		t.Errorf("got judgment %q, want %q", got.Judgment, Kill)
	}
}

func TestPerRoleThreshold(t *testing.T) {
	tests := []struct {
		name      string
		role      string
		idleAfter time.Duration
		wantKill  bool
	}{
		{name: "recon under threshold", role: "recon", idleAfter: 179 * time.Second, wantKill: false},
		{name: "recon over threshold", role: "recon", idleAfter: 181 * time.Second, wantKill: true},
		{name: "implementer under threshold", role: "implementer", idleAfter: 299 * time.Second, wantKill: false},
		{name: "implementer over threshold", role: "implementer", idleAfter: 301 * time.Second, wantKill: true},
	}

	cfg := Config{
		IdleTimeoutByRole: map[string]int{
			"recon":       180,
			"implementer": 300,
		},
		DefaultIdleTimeout: 600,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := NewJudge(tt.role, cfg)
			j.ProcessLine("start", t0)
			got := j.ProcessLine("idle line", t0.Add(tt.idleAfter))
			if tt.wantKill && got == nil {
				t.Errorf("expected Kill intervention, got nil")
			}
			if !tt.wantKill && got != nil {
				t.Errorf("expected nil, got intervention %+v", got)
			}
		})
	}
}

func TestDefaultThresholdFallback(t *testing.T) {
	cfg := Config{
		IdleTimeoutByRole: map[string]int{
			"recon": 180,
		},
		DefaultIdleTimeout: 300,
	}

	j := NewJudge("unknown-role", cfg)
	j.ProcessLine("start", t0)

	// Under default threshold.
	got := j.ProcessLine("text", t0.Add(299*time.Second))
	if got != nil {
		t.Errorf("expected nil before default threshold, got %+v", got)
	}

	// Over default threshold.
	got = j.ProcessLine("text", t0.Add(301*time.Second))
	if got == nil {
		t.Fatal("expected Kill using default threshold, got nil")
	}
	if got.Judgment != Kill {
		t.Errorf("got judgment %q, want %q", got.Judgment, Kill)
	}
}

func TestDefaultIdleTimeoutAppliedWhenZero(t *testing.T) {
	// When Config.DefaultIdleTimeout is 0, NewJudge should apply 300s default.
	j := NewJudge("recon", Config{})
	j.ProcessLine("start", t0)

	got := j.ProcessLine("text", t0.Add(299*time.Second))
	if got != nil {
		t.Errorf("expected nil before 300s default, got %+v", got)
	}

	got = j.ProcessLine("text", t0.Add(301*time.Second))
	if got == nil {
		t.Fatal("expected Kill at 301s with default 300s threshold, got nil")
	}
}

func TestFirstLineStartsTheClock(t *testing.T) {
	j := NewJudge("recon", Config{DefaultIdleTimeout: 300})

	// Do NOT call ProcessLine before the "start" time — the judge was just
	// constructed. Verify that idle is measured from the first ProcessLine
	// call, not from construction time.
	start := t0.Add(1000 * time.Second) // construction was "long ago"
	j.ProcessLine("first line", start)

	// Only 10s after first line — should not fire.
	got := j.ProcessLine("second line", start.Add(10*time.Second))
	if got != nil {
		t.Errorf("expected nil (idle measured from first line), got %+v", got)
	}
}
