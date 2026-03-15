package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes ANSI escape sequences from s so tests can compare plain text.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// --- Compile-time interface check ---

// Ensure Model satisfies tea.Model at compile time.
var _ tea.Model = Model{}

// --- Header tests ---

func TestRenderHeaderFullMetadata(t *testing.T) {
	m := New(
		"peter-stratton/dark-factory",
		"Phase 20",
		"20260314-120000",
		"main",
		"feature/phase-20",
		"rollup/phase-20",
		nil,
	)
	got := stripANSI(renderHeader(m))

	checks := []struct {
		label string
		want  string
	}{
		{"repo", "peter-stratton/dark-factory"},
		{"milestone", "Phase 20"},
		{"timestamp", "20260314-120000"},
		{"base branch label", "base:"},
		{"base branch value", "main"},
		{"auto-merge label", "auto-merge:"},
		{"feature value", "feature=feature/phase-20"},
		{"rollup value", "rollup=rollup/phase-20"},
		{"logo", "godark"},
	}

	for _, c := range checks {
		if !strings.Contains(got, c.want) {
			t.Errorf("renderHeader full metadata: %s not found in output\ngot: %q", c.label, got)
		}
	}
}

func TestRenderHeaderMinimal(t *testing.T) {
	m := New(
		"peter-stratton/dark-factory",
		"Phase 20",
		"20260314-120000",
		"", // no base branch
		"", // no auto-merge
		"",
		nil,
	)
	got := stripANSI(renderHeader(m))

	// Required segments.
	for _, want := range []string{"godark", "peter-stratton/dark-factory", "Phase 20", "20260314-120000"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderHeader minimal: %q not found in output\ngot: %q", want, got)
		}
	}

	// Omitted segments.
	for _, absent := range []string{"base:", "auto-merge:"} {
		if strings.Contains(got, absent) {
			t.Errorf("renderHeader minimal: %q should be absent but was found\ngot: %q", absent, got)
		}
	}
}

// --- Summary tests ---

func TestRenderSummaryZeroState(t *testing.T) {
	m := Model{} // all counts and cost zero
	got := stripANSI(renderSummary(m))

	// The spec requires the exact segments to be present.
	for _, want := range []string{
		"0 merged",
		"0 in review",
		"0 queued",
		"0 failed",
		"$0.00 total cost",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderSummary zero state: %q not found in output\ngot: %q", want, got)
		}
	}
}

func TestRenderSummaryWithCounts(t *testing.T) {
	m := Model{
		merged:    3,
		inReview:  2,
		queued:    5,
		failed:    1,
		totalCost: 4.56,
	}
	got := stripANSI(renderSummary(m))

	for _, want := range []string{
		"3 merged",
		"2 in review",
		"5 queued",
		"1 failed",
		"$4.56 total cost",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderSummary with counts: %q not found in output\ngot: %q", want, got)
		}
	}
}

// --- Model Update tests ---

func TestModelUpdateWindowSize(t *testing.T) {
	m := Model{}
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}

	next, cmd := m.Update(msg)
	if cmd != nil {
		t.Errorf("Update(WindowSizeMsg): expected nil cmd, got %v", cmd)
	}

	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned wrong type: %T", next)
	}
	if updated.width != 120 {
		t.Errorf("width: got %d, want 120", updated.width)
	}
	if updated.height != 40 {
		t.Errorf("height: got %d, want 40", updated.height)
	}
}

// --- Model View tests ---

func TestModelViewContainsHeaderAndSummary(t *testing.T) {
	m := New(
		"peter-stratton/dark-factory",
		"Phase 20",
		"20260314-120000",
		"",
		"",
		"",
		nil,
	)
	got := stripANSI(m.View())

	for _, want := range []string{
		"godark",
		"peter-stratton/dark-factory",
		"Phase 20",
		"0 merged",
		"$0.00 total cost",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("View: %q not found in output\ngot: %q", want, got)
		}
	}
}

// --- New constructor tests ---

func TestNewAutoMergePopulated(t *testing.T) {
	m := New("repo", "ms", "ts", "base", "feat-branch", "rollup-branch", nil)
	if m.autoMerge == nil {
		t.Fatal("autoMerge should be non-nil when mergeFeature is set")
	}
	if m.autoMerge.feature != "feat-branch" {
		t.Errorf("autoMerge.feature: got %q, want %q", m.autoMerge.feature, "feat-branch")
	}
	if m.autoMerge.rollup != "rollup-branch" {
		t.Errorf("autoMerge.rollup: got %q, want %q", m.autoMerge.rollup, "rollup-branch")
	}
}

func TestNewAutoMergeNilWhenEmpty(t *testing.T) {
	m := New("repo", "ms", "ts", "", "", "", nil)
	if m.autoMerge != nil {
		t.Errorf("autoMerge should be nil when both merge fields are empty, got %+v", m.autoMerge)
	}
}
