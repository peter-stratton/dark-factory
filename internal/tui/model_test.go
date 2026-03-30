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

// --- Detail panel tests ---

func TestDetailPanelErrorAppearsOnCompletion(t *testing.T) {
	m := Model{
		issueIndex: map[int]int{43: 0},
		issues:     []issueRow{{number: 43, title: "fix auth"}},
	}
	m.handleIssueCompleted(IssueCompletedMsg{Number: 43, Status: "failed", ErrMsg: "go test exit 1"})

	if len(m.detailMessages) != 1 {
		t.Fatalf("expected 1 detail entry, got %d", len(m.detailMessages))
	}
	e := m.detailMessages[0]
	if e.issueNumber != 43 {
		t.Errorf("detailMessages[0].issueNumber: got %d, want 43", e.issueNumber)
	}
	if e.kind != detailKindError {
		t.Errorf("detailMessages[0].kind: got %v, want detailKindError", e.kind)
	}
	if e.message != "go test exit 1" {
		t.Errorf("detailMessages[0].message: got %q, want %q", e.message, "go test exit 1")
	}
}

func TestDetailPanelNoEntryWhenNoError(t *testing.T) {
	m := Model{
		issueIndex: map[int]int{1: 0},
		issues:     []issueRow{{number: 1, title: "success"}},
	}
	m.handleIssueCompleted(IssueCompletedMsg{Number: 1, Status: "implemented", ErrMsg: ""})

	if len(m.detailMessages) != 0 {
		t.Errorf("expected no detail entries for successful completion, got %d", len(m.detailMessages))
	}
}

func TestDetailPanelJudgeEntry(t *testing.T) {
	m := Model{
		issueIndex: map[int]int{42: 0},
		issues:     []issueRow{{number: 42, title: "widget"}},
	}
	next, _ := m.Update(JudgeInterventionMsg{
		IssueNumber: 42,
		Rule:        "idle_timeout",
		Judgment:    "killed",
		Step:        "recon",
	})
	updated := next.(Model)

	if len(updated.detailMessages) != 1 {
		t.Fatalf("expected 1 detail entry, got %d", len(updated.detailMessages))
	}
	e := updated.detailMessages[0]
	if e.issueNumber != 42 {
		t.Errorf("judge entry issueNumber: got %d, want 42", e.issueNumber)
	}
	if e.kind != detailKindJudge {
		t.Errorf("judge entry kind: got %v, want detailKindJudge", e.kind)
	}
	if !strings.Contains(e.message, "idle_timeout") {
		t.Errorf("judge entry message: %q should contain %q", e.message, "idle_timeout")
	}
}

func TestDetailPanelCappedAtFive(t *testing.T) {
	m := Model{
		issueIndex: map[int]int{},
		issues:     []issueRow{},
	}
	// Add 7 issues and complete them all with errors.
	for i := 1; i <= 7; i++ {
		m.issueIndex[i] = len(m.issues)
		m.issues = append(m.issues, issueRow{number: i, title: "issue"})
	}
	for i := 1; i <= 7; i++ {
		m.handleIssueCompleted(IssueCompletedMsg{Number: i, Status: "failed", ErrMsg: "err"})
	}

	if len(m.detailMessages) != 5 {
		t.Errorf("detailMessages capped at 5: got %d entries", len(m.detailMessages))
	}
	// The last 5 issues (3-7) should be retained.
	for j, e := range m.detailMessages {
		wantNum := j + 3
		if e.issueNumber != wantNum {
			t.Errorf("detailMessages[%d].issueNumber: got %d, want %d", j, e.issueNumber, wantNum)
		}
	}
}

func TestDetailPanelEmptyHidesPanel(t *testing.T) {
	m := New("repo", "ms", "ts", "", "", "", nil)
	m.width = 80
	got := stripANSI(m.View())

	// With no errors, there should be only one divider line (between table/detail and summary).
	// Count how many times the divider character repeats — should appear once.
	dividerRune := "─"
	lines := strings.Split(got, "\n")
	dividerCount := 0
	for _, line := range lines {
		if strings.Contains(line, dividerRune) {
			dividerCount++
		}
	}
	if dividerCount != 1 {
		t.Errorf("View with no errors: expected 1 divider line, got %d\noutput: %q", dividerCount, got)
	}
}

func TestDetailPanelAppearsInView(t *testing.T) {
	m := Model{
		issueIndex: map[int]int{43: 0},
		issues:     []issueRow{{number: 43, title: "fix auth"}},
		width:      80,
		detailMessages: []detailEntry{
			{issueNumber: 43, kind: detailKindError, message: "verify: go test exit code 1"},
		},
	}
	got := stripANSI(m.View())

	if !strings.Contains(got, "#43") {
		t.Errorf("detail panel: #43 not found in view\ngot: %q", got)
	}
	if !strings.Contains(got, "verify: go test exit code 1") {
		t.Errorf("detail panel: error message not found in view\ngot: %q", got)
	}
}

func TestDetailPanelErrorNotInTableRow(t *testing.T) {
	m := Model{
		issueIndex: map[int]int{43: 0},
		issues:     []issueRow{{number: 43, title: "fix auth", status: "failed", errMsg: "timeout"}},
		width:      80,
	}
	tableOut := stripANSI(renderTable(m.issues, newTestSpinner(), 80, nil))

	if strings.Contains(tableOut, "timeout") {
		t.Errorf("renderTable: error message should not appear inline in row\ngot: %q", tableOut)
	}
}

func TestRenderDetailPanelEmpty(t *testing.T) {
	got := renderDetailPanel(nil, 40)
	if got != "" {
		t.Errorf("renderDetailPanel empty: expected empty string, got %q", got)
	}
}

func TestRenderDetailPanelContent(t *testing.T) {
	entries := []detailEntry{
		{issueNumber: 42, kind: detailKindJudge, message: "Killed: idle_timeout (recon)"},
		{issueNumber: 43, kind: detailKindError, message: "verify: exit code 1"},
	}
	got := stripANSI(renderDetailPanel(entries, 80))

	for _, want := range []string{"#42", "judge:", "Killed: idle_timeout (recon)", "#43", "error:", "verify: exit code 1"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderDetailPanel: %q not found in output\ngot: %q", want, got)
		}
	}
}
