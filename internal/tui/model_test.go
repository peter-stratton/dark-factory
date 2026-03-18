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

func TestRenderHeaderShowsIdentity(t *testing.T) {
	m := Model{
		repo:      "owner/repo",
		milestone: "Phase 10",
		identity:  "godark-runner[bot]",
	}
	got := stripANSI(renderHeader(m))

	if !strings.Contains(got, "as:") {
		t.Errorf("renderHeader identity: label %q not found in output\ngot: %q", "as:", got)
	}
	if !strings.Contains(got, "godark-runner[bot]") {
		t.Errorf("renderHeader identity: value %q not found in output\ngot: %q", "godark-runner[bot]", got)
	}
}

func TestRenderHeaderOmitsIdentityWhenEmpty(t *testing.T) {
	m := Model{
		repo:      "owner/repo",
		milestone: "Phase 10",
		identity:  "",
	}
	got := stripANSI(renderHeader(m))

	if strings.Contains(got, "as:") {
		t.Errorf("renderHeader: %q should be absent when identity is empty\ngot: %q", "as:", got)
	}
}

func TestHandleRunStartedSetsIdentity(t *testing.T) {
	m := Model{}
	m.handleRunStarted(RunStartedMsg{
		Repo:      "owner/repo",
		Milestone: "Phase 10",
		Identity:  "octocat",
	})

	if m.identity != "octocat" {
		t.Errorf("identity: got %q, want %q", m.identity, "octocat")
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

// --- Cost accumulation tests ---

func TestHandleIssueCompletedAccumulatesCost(t *testing.T) {
	m := Model{
		issueIndex: map[int]int{1: 0},
		issues:     []issueRow{{number: 1, title: "test"}},
	}

	m.handleIssueCompleted(IssueCompletedMsg{Number: 1, Status: "implemented", CostUSD: 1.50})
	m.handleIssueCompleted(IssueCompletedMsg{Number: 1, Status: "implemented", CostUSD: 1.50})

	if m.totalCost != 3.00 {
		t.Errorf("totalCost: got %f, want 3.00", m.totalCost)
	}
}

func TestHandleIssueCompletedZeroCostUnchanged(t *testing.T) {
	m := Model{
		issueIndex: map[int]int{1: 0},
		issues:     []issueRow{{number: 1, title: "test"}},
	}

	m.handleIssueCompleted(IssueCompletedMsg{Number: 1, Status: "implemented", CostUSD: 0.0})

	if m.totalCost != 0.0 {
		t.Errorf("totalCost: got %f, want 0.0", m.totalCost)
	}
}

// --- Watch-mode tests ---

func TestWatchingMsgSetsWatchingTrue(t *testing.T) {
	m := Model{}
	next, cmd := m.Update(WatchingMsg{})
	if cmd != nil {
		t.Errorf("Update(WatchingMsg): expected nil cmd, got %v", cmd)
	}
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned wrong type: %T", next)
	}
	if !updated.watching {
		t.Error("watching should be true after WatchingMsg")
	}
	if updated.done {
		t.Error("done should remain false after WatchingMsg")
	}
}

func TestWatchingMsgHintText(t *testing.T) {
	m := Model{watching: true}
	got := stripANSI(m.View())
	if !strings.Contains(got, "watching for merges") {
		t.Errorf("View during watch mode: hint should contain 'watching for merges'\ngot: %q", got)
	}
}

func TestWatchingCancellingHintOverridesWatch(t *testing.T) {
	m := Model{watching: true, cancelling: true}
	got := stripANSI(m.View())
	if !strings.Contains(got, "cancelling") {
		t.Errorf("View during cancelling: hint should contain 'cancelling'\ngot: %q", got)
	}
	if strings.Contains(got, "watching for merges") {
		t.Errorf("View during cancelling: 'watching for merges' should not appear when cancelling\ngot: %q", got)
	}
}

func TestIssueStartedDuringWatchAddsRow(t *testing.T) {
	m := Model{watching: true, issueIndex: map[int]int{}}
	next, _ := m.Update(IssueStartedMsg{Number: 99, Title: "new issue in watch mode"})
	updated := next.(Model)
	if _, exists := updated.issueIndex[99]; !exists {
		t.Error("IssueStartedMsg during watch mode should add a new row")
	}
	if len(updated.issues) != 1 {
		t.Errorf("expected 1 issue row, got %d", len(updated.issues))
	}
}

func TestCtrlCDuringWatchSetsCancelling(t *testing.T) {
	cancelled := false
	m := Model{
		watching: true,
		cancelFn: func() { cancelled = true },
	}
	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	updated := next.(Model)
	if !updated.cancelling {
		t.Error("ctrl+c during watch mode should set cancelling = true")
	}
	if !cancelled {
		t.Error("ctrl+c during watch mode should invoke cancelFn")
	}
}

func TestRunDoneDuringWatchSetsDone(t *testing.T) {
	m := Model{watching: true}
	next, _ := m.Update(RunDoneMsg{})
	updated := next.(Model)
	if !updated.done {
		t.Error("RunDoneMsg during watch mode should set done = true")
	}
}

func TestRunDoneDuringWatchHintShowsQuit(t *testing.T) {
	m := Model{watching: true, done: true}
	got := stripANSI(m.View())
	if !strings.Contains(got, "press q to exit") {
		t.Errorf("View after RunDone: should show 'press q to exit'\ngot: %q", got)
	}
}

func TestModelUpdateAccumulatesCostViaTUIUpdate(t *testing.T) {
	m := Model{
		issueIndex: map[int]int{42: 0},
		issues:     []issueRow{{number: 42, title: "some issue"}},
	}

	next, _ := m.Update(IssueCompletedMsg{Number: 42, Status: "implemented", CostUSD: 2.25})
	updated := next.(Model)
	next2, _ := updated.Update(IssueCompletedMsg{Number: 42, Status: "implemented", CostUSD: 0.75})
	final := next2.(Model)

	if final.totalCost != 3.00 {
		t.Errorf("totalCost after two updates: got %f, want 3.00", final.totalCost)
	}
}
