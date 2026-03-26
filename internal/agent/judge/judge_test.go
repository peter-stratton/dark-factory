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

	// Empty-line heartbeat ticks don't reset the timer.
	got = j.ProcessLine("", t0.Add(299*time.Second))
	if got != nil {
		t.Fatalf("expected nil before threshold, got %+v", got)
	}

	// Past threshold with no non-empty lines — Kill fires.
	got = j.ProcessLine("", t0.Add(301*time.Second))
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

func TestIdleTimeoutResetsOnAnyOutput(t *testing.T) {
	j := NewJudge("recon", Config{DefaultIdleTimeout: 300})

	// Start clock.
	j.ProcessLine("assistant text", t0)

	// Any non-empty line resets the timer — not just tool lines.
	j.ProcessLine("just thinking out loud...", t0.Add(290*time.Second))

	// 289s after the last non-empty line — still under threshold.
	got := j.ProcessLine("still going", t0.Add(290*time.Second+289*time.Second))
	if got != nil {
		t.Fatalf("expected nil within threshold after reset, got %+v", got)
	}

	// Now go silent (only heartbeat ticks). Past threshold from last output.
	got = j.ProcessLine("", t0.Add(290*time.Second+289*time.Second+301*time.Second))
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
			// Empty line = heartbeat tick, does not reset the timer.
			got := j.ProcessLine("", t0.Add(tt.idleAfter))
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

	// Under default threshold (heartbeat tick, no output).
	got := j.ProcessLine("", t0.Add(299*time.Second))
	if got != nil {
		t.Errorf("expected nil before default threshold, got %+v", got)
	}

	// Over default threshold.
	got = j.ProcessLine("", t0.Add(301*time.Second))
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

	got := j.ProcessLine("", t0.Add(299*time.Second))
	if got != nil {
		t.Errorf("expected nil before 300s default, got %+v", got)
	}

	got = j.ProcessLine("", t0.Add(301*time.Second))
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

// --- No-progress rule tests ---

func TestNoProgressFiresWithoutToolCalls(t *testing.T) {
	j := NewJudge("recon", Config{DefaultNoProgressTimeout: 600})

	// First line starts the clock.
	j.ProcessLine("some text", t0)

	// Non-empty lines keep flowing but no tool calls — timer should NOT reset.
	got := j.ProcessLine("still thinking...", t0.Add(599*time.Second))
	if got != nil {
		t.Fatalf("expected nil before threshold, got %+v", got)
	}

	// Past threshold — Kill fires even though output is still flowing.
	got = j.ProcessLine("more thinking", t0.Add(601*time.Second))
	if got == nil {
		t.Fatal("expected Kill intervention, got nil")
	}
	if got.Rule != "no_progress" {
		t.Errorf("got rule %q, want %q", got.Rule, "no_progress")
	}
	if got.Judgment != Kill {
		t.Errorf("got judgment %q, want %q", got.Judgment, Kill)
	}
}

func TestNoProgressResetsOnToolCall(t *testing.T) {
	j := NewJudge("recon", Config{DefaultNoProgressTimeout: 600})

	j.ProcessLine("start", t0)

	// Lots of text, no tools — close to threshold.
	j.ProcessLine("thinking...", t0.Add(590*time.Second))

	// Tool call resets the clock.
	j.ProcessLine(`{"tool": "Read", "input_summary": "foo.go"}`, t0.Add(595*time.Second))

	// 599s after the tool call — still under threshold.
	got := j.ProcessLine("more text", t0.Add(595*time.Second+599*time.Second))
	if got != nil {
		t.Fatalf("expected nil after tool call reset, got %+v", got)
	}

	// Past threshold from last tool call.
	got = j.ProcessLine("still going", t0.Add(595*time.Second+601*time.Second))
	if got == nil {
		t.Fatal("expected Kill after threshold exceeded post-reset, got nil")
	}
	if got.Rule != "no_progress" {
		t.Errorf("got rule %q, want %q", got.Rule, "no_progress")
	}
}

func TestNoProgressPerRoleThreshold(t *testing.T) {
	cfg := Config{
		NoProgressTimeoutByRole: map[string]int{
			"recon":       900,
			"implementer": 600,
		},
		DefaultNoProgressTimeout: 300,
	}

	tests := []struct {
		name     string
		role     string
		after    time.Duration
		wantKill bool
	}{
		{name: "recon under", role: "recon", after: 899 * time.Second, wantKill: false},
		{name: "recon over", role: "recon", after: 901 * time.Second, wantKill: true},
		{name: "implementer under", role: "implementer", after: 599 * time.Second, wantKill: false},
		{name: "implementer over", role: "implementer", after: 601 * time.Second, wantKill: true},
		{name: "unknown uses default under", role: "reviewer", after: 299 * time.Second, wantKill: false},
		{name: "unknown uses default over", role: "reviewer", after: 301 * time.Second, wantKill: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := NewJudge(tt.role, cfg)
			j.ProcessLine("start", t0)
			// Non-empty line (not a tool call) — only no_progress should care.
			got := j.ProcessLine("still thinking", t0.Add(tt.after))
			var fired bool
			if got != nil && got.Rule == "no_progress" {
				fired = true
			}
			if tt.wantKill && !fired {
				t.Error("expected no_progress Kill, did not fire")
			}
			if !tt.wantKill && fired {
				t.Errorf("did not expect no_progress Kill, got %+v", got)
			}
		})
	}
}

// --- Tool thrash rule tests ---

func toolThrashConfig() Config {
	return Config{
		DefaultIdleTimeout:   300,
		ToolThrashThreshold:  3,
		ToolThrashWindowSecs: 60,
	}
}

func toolSearchLine(query string) string {
	return `ToolSearch {"query":"` + query + `","max_results":5}`
}

func TestToolThrashFires(t *testing.T) {
	r := newToolThrashRule(toolThrashConfig())

	// Three identical queries within 60s should fire Kill.
	r.ProcessLine(toolSearchLine("my query"), t0)
	r.ProcessLine(toolSearchLine("my query"), t0.Add(10*time.Second))
	got := r.ProcessLine(toolSearchLine("my query"), t0.Add(20*time.Second))

	if got == nil {
		t.Fatal("expected Kill intervention, got nil")
	}
	if got.Judgment != Kill {
		t.Errorf("got judgment %q, want Kill", got.Judgment)
	}
	if got.Rule != "tool_thrash" {
		t.Errorf("got rule %q, want tool_thrash", got.Rule)
	}
	if got.Counts["repeated_searches"] < 3 {
		t.Errorf("expected repeated_searches >= 3, got %d", got.Counts["repeated_searches"])
	}
}

func TestToolThrashDifferentQueriesNoFire(t *testing.T) {
	r := newToolThrashRule(toolThrashConfig())

	// Three different queries — no thrash.
	r.ProcessLine(toolSearchLine("query one"), t0)
	r.ProcessLine(toolSearchLine("query two"), t0.Add(10*time.Second))
	got := r.ProcessLine(toolSearchLine("query three"), t0.Add(20*time.Second))

	if got != nil {
		t.Errorf("expected nil for different queries, got %+v", got)
	}
}

func TestToolThrashOutsideWindowNoFire(t *testing.T) {
	r := newToolThrashRule(toolThrashConfig())

	// Three same queries but spread over 120s — outside the 60s window.
	r.ProcessLine(toolSearchLine("stuck query"), t0)
	r.ProcessLine(toolSearchLine("stuck query"), t0.Add(65*time.Second))
	got := r.ProcessLine(toolSearchLine("stuck query"), t0.Add(130*time.Second))

	if got != nil {
		t.Errorf("expected nil (queries outside window), got %+v", got)
	}
}

// --- Transport failure rule tests ---

func transportConfig() Config {
	return Config{
		DefaultIdleTimeout:        300,
		TransportFailureThreshold: 10,
	}
}

func TestTransportFailureFires(t *testing.T) {
	r := newTransportFailureRule(transportConfig())

	var got *Intervention
	for i := 0; i < 10; i++ {
		got = r.ProcessLine("stream closed: EOF", t0.Add(time.Duration(i)*time.Second))
	}

	if got == nil {
		t.Fatal("expected RetryContainer intervention, got nil")
	}
	if got.Judgment != RetryContainer {
		t.Errorf("got judgment %q, want RetryContainer", got.Judgment)
	}
	if got.Rule != "transport_failure" {
		t.Errorf("got rule %q, want transport_failure", got.Rule)
	}
	if got.Counts["stream_errors"] < 10 {
		t.Errorf("expected stream_errors >= 10, got %d", got.Counts["stream_errors"])
	}
}

func TestTransportFailureWithToolCallsNoFire(t *testing.T) {
	r := newTransportFailureRule(transportConfig())

	// One successful tool call mixed in — transport recovered.
	r.ProcessLine(`{"tool": "bash", "input": {}}`, t0)

	var got *Intervention
	for i := 0; i < 10; i++ {
		got = r.ProcessLine("stream error: connection reset", t0.Add(time.Duration(i+1)*time.Second))
	}

	if got != nil {
		t.Errorf("expected nil when tool calls present, got %+v", got)
	}
}
