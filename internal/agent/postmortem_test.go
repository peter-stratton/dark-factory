package agent

import (
	"strings"
	"testing"
)

// TestAnalyzeFailure_StreamClosedDetected verifies that stream_closed pattern is
// detected and severity is set correctly based on count.
func TestAnalyzeFailure_StreamClosedDetected(t *testing.T) {
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, "2026-03-08T10:00:00 Stream closed by remote")
	}
	log := strings.Join(lines, "\n")

	result := AnalyzeFailure(log, 0)

	found := false
	for _, p := range result.Patterns {
		if p.Code == "stream_closed" {
			found = true
			if p.Count != 50 {
				t.Errorf("stream_closed count = %d, want 50", p.Count)
			}
			if p.Severity != "high" {
				t.Errorf("stream_closed severity = %q, want %q", p.Severity, "high")
			}
		}
	}
	if !found {
		t.Error("expected stream_closed pattern, not found")
	}
}

// TestAnalyzeFailure_StreamClosedMediumSeverity checks that fewer than 50
// stream_closed lines produce "medium" severity.
func TestAnalyzeFailure_StreamClosedMediumSeverity(t *testing.T) {
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "stream closed")
	}
	log := strings.Join(lines, "\n")

	result := AnalyzeFailure(log, 0)

	for _, p := range result.Patterns {
		if p.Code == "stream_closed" {
			if p.Severity != "medium" {
				t.Errorf("stream_closed severity = %q, want %q", p.Severity, "medium")
			}
			return
		}
	}
	t.Error("expected stream_closed pattern, not found")
}

// TestAnalyzeFailure_APIOverloaded verifies that the api_overloaded pattern is
// detected when a 529 status or "overloaded" text appears.
func TestAnalyzeFailure_APIOverloaded(t *testing.T) {
	log := `starting agent
error: HTTP 529 Service Temporarily Unavailable
retrying...`

	result := AnalyzeFailure(log, 1)

	found := false
	for _, p := range result.Patterns {
		if p.Code == "api_overloaded" {
			found = true
			if p.Severity != "high" {
				t.Errorf("api_overloaded severity = %q, want %q", p.Severity, "high")
			}
		}
	}
	if !found {
		t.Error("expected api_overloaded pattern, not found")
	}
}

// TestAnalyzeFailure_HighSubagentCount verifies the high_subagent_count pattern
// fires correctly at different thresholds.
func TestAnalyzeFailure_HighSubagentCount(t *testing.T) {
	tests := []struct {
		name         string
		count        int
		wantSeverity string
	}{
		{"low (50)", 50, "low"},
		{"medium (200)", 200, "medium"},
		{"high (400)", 400, "high"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var lines []string
			for i := 0; i < tc.count; i++ {
				lines = append(lines, `{"type":"tool_use","parent_tool_use_id":"toolu_abc"}`)
			}
			log := strings.Join(lines, "\n")

			result := AnalyzeFailure(log, 0)

			for _, p := range result.Patterns {
				if p.Code == "high_subagent_count" {
					if p.Count != tc.count {
						t.Errorf("high_subagent_count count = %d, want %d", p.Count, tc.count)
					}
					if p.Severity != tc.wantSeverity {
						t.Errorf("high_subagent_count severity = %q, want %q", p.Severity, tc.wantSeverity)
					}
					return
				}
			}
			t.Errorf("expected high_subagent_count pattern, not found (count=%d)", tc.count)
		})
	}
}

// TestAnalyzeFailure_RateLimited verifies the rate_limited pattern.
func TestAnalyzeFailure_RateLimited(t *testing.T) {
	log := "Error: rate_limit exceeded, please slow down"

	result := AnalyzeFailure(log, 0)

	for _, p := range result.Patterns {
		if p.Code == "rate_limited" {
			return
		}
	}
	t.Error("expected rate_limited pattern, not found")
}

// TestAnalyzeFailure_NoPatterns verifies that a clean log produces an empty patterns list.
func TestAnalyzeFailure_NoPatterns(t *testing.T) {
	log := `starting agent
cloning repository
running implementation
done`

	result := AnalyzeFailure(log, 0)

	if len(result.Patterns) != 0 {
		t.Errorf("expected no patterns for clean log, got %d: %v", len(result.Patterns), result.Patterns)
	}
}

// TestAnalyzeFailure_NonzeroExitCode verifies that a non-zero exit code produces
// the nonzero_exit pattern.
func TestAnalyzeFailure_NonzeroExitCode(t *testing.T) {
	result := AnalyzeFailure("some log output", 2)

	for _, p := range result.Patterns {
		if p.Code == "nonzero_exit" {
			if p.Count != 2 {
				t.Errorf("nonzero_exit count = %d, want 2 (the exit code)", p.Count)
			}
			return
		}
	}
	t.Error("expected nonzero_exit pattern, not found")
}

// TestAnalyzeFailure_ContextExhausted verifies the context_exhausted pattern.
func TestAnalyzeFailure_ContextExhausted(t *testing.T) {
	log := `APIError: context length exceeded, please reduce your input`

	result := AnalyzeFailure(log, 1)

	for _, p := range result.Patterns {
		if p.Code == "context_exhausted" {
			if p.Severity != "high" {
				t.Errorf("context_exhausted severity = %q, want %q", p.Severity, "high")
			}
			return
		}
	}
	t.Error("expected context_exhausted pattern, not found")
}

// TestAnalyzeFailure_LogLinesCaptured verifies that the reported line count
// matches what was actually analyzed.
func TestAnalyzeFailure_LogLinesCaptured(t *testing.T) {
	log := "line1\nline2\nline3\n"

	result := AnalyzeFailure(log, 0)

	if result.LogLinesCaptured != 3 {
		t.Errorf("LogLinesCaptured = %d, want 3", result.LogLinesCaptured)
	}
}

// TestAnalyzeFailure_ContainerExitCode verifies that the exit code is passed through.
func TestAnalyzeFailure_ContainerExitCode(t *testing.T) {
	result := AnalyzeFailure("log", 137)

	if result.ContainerExitCode != 137 {
		t.Errorf("ContainerExitCode = %d, want 137", result.ContainerExitCode)
	}
}

// TestBoundLog_TruncatesToMaxLines verifies that boundLog keeps only the last N lines.
func TestBoundLog_TruncatesToMaxLines(t *testing.T) {
	var lines []string
	for i := 0; i < 600; i++ {
		lines = append(lines, "line")
	}
	log := strings.Join(lines, "\n")

	bounded := boundLog(log, 500, 10*1024*1024)

	got := len(strings.Split(bounded, "\n"))
	if got != 500 {
		t.Errorf("boundLog returned %d lines, want 500", got)
	}
}

// TestBoundLog_TruncatesToMaxBytes verifies that boundLog caps to the byte limit.
func TestBoundLog_TruncatesToMaxBytes(t *testing.T) {
	// Build a log that is 200KB of data.
	line := strings.Repeat("x", 100) + "\n"
	var sb strings.Builder
	for i := 0; i < 2000; i++ {
		sb.WriteString(line)
	}
	log := sb.String()

	bounded := boundLog(log, 10000, 100*1024)

	if len(bounded) > 100*1024 {
		t.Errorf("boundLog returned %d bytes, want <= 100KB", len(bounded))
	}
}

// TestBoundLog_ShortLogUnchanged verifies that a short log is not modified.
func TestBoundLog_ShortLogUnchanged(t *testing.T) {
	log := "line1\nline2\nline3"

	bounded := boundLog(log, 500, 100*1024)

	if bounded != log {
		t.Errorf("boundLog changed short log: got %q, want %q", bounded, log)
	}
}

// TestBoundLog_EmptyLogReturnsEmpty verifies that empty input returns empty output.
func TestBoundLog_EmptyLogReturnsEmpty(t *testing.T) {
	bounded := boundLog("", 500, 100*1024)

	if bounded != "" {
		t.Errorf("boundLog(%q) = %q, want %q", "", bounded, "")
	}
}
