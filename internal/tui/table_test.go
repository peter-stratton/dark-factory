package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
)

// newTestSpinner returns a spinner.Model suitable for testing (no animation needed).
func newTestSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return s
}

// --- truncate tests ---

func TestTruncateShortString(t *testing.T) {
	t.Helper()
	got := truncate("hello", 10)
	if got != "hello" {
		t.Errorf("truncate short: got %q, want %q", got, "hello")
	}
}

func TestTruncateExactLength(t *testing.T) {
	got := truncate("hello", 5)
	if got != "hello" {
		t.Errorf("truncate exact: got %q, want %q", got, "hello")
	}
}

func TestTruncateLongTitle(t *testing.T) {
	// 120-character title, 80-column terminal.
	// titleWidth = 80 - 22 = 58 runes, so truncation applies.
	title := strings.Repeat("a", 120)
	width := 80
	titleWidth := width - 22

	got := truncate(title, titleWidth)
	runes := []rune(got)

	if len(runes) > titleWidth {
		t.Errorf("truncate long: result has %d runes, want ≤%d", len(runes), titleWidth)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncate long: result %q does not end with ellipsis", got)
	}
}

func TestTruncateZeroWidth(t *testing.T) {
	got := truncate("hello", 0)
	if got != "…" {
		t.Errorf("truncate zero width: got %q, want %q", got, "…")
	}
}

// --- markerFor tests ---

func TestMarkerQueued(t *testing.T) {
	row := issueRow{number: 1, title: "fix bug"}
	spin := newTestSpinner()
	got := stripANSI(markerFor(row, spin))
	if got != markerQueued {
		t.Errorf("markerFor queued: got %q, want %q", got, markerQueued)
	}
}

func TestMarkerInProgress(t *testing.T) {
	row := issueRow{number: 1, title: "fix bug", stage: "implement"}
	spin := newTestSpinner()
	// The spinner view is not the queued marker.
	got := stripANSI(markerFor(row, spin))
	if got == markerQueued {
		t.Errorf("markerFor in-progress: should not be queued marker %q", markerQueued)
	}
}

func TestMarkerCompleted(t *testing.T) {
	row := issueRow{number: 1, title: "fix bug", status: "implemented", stage: "merged"}
	spin := newTestSpinner()
	got := stripANSI(markerFor(row, spin))
	if got != markerCompleted {
		t.Errorf("markerFor completed: got %q, want %q", got, markerCompleted)
	}
}

func TestMarkerReadyToMerge(t *testing.T) {
	row := issueRow{number: 1, title: "fix bug", status: "ready-to-merge", stage: "review"}
	spin := newTestSpinner()
	got := stripANSI(markerFor(row, spin))
	if got != markerReview {
		t.Errorf("markerFor ready-to-merge: got %q, want %q", got, markerReview)
	}
}

func TestMarkerNeedsHumanReview(t *testing.T) {
	row := issueRow{number: 1, title: "fix bug", status: "needs-human-review"}
	spin := newTestSpinner()
	got := stripANSI(markerFor(row, spin))
	if got != markerReview {
		t.Errorf("markerFor needs-human-review: got %q, want %q", got, markerReview)
	}
}

func TestMarkerFailed(t *testing.T) {
	row := issueRow{number: 1, title: "fix bug", status: "failed", stage: "failed"}
	spin := newTestSpinner()
	got := stripANSI(markerFor(row, spin))
	if got != markerFailed {
		t.Errorf("markerFor failed: got %q, want %q", got, markerFailed)
	}
}

// --- renderTable tests ---

func TestRenderTableEmpty(t *testing.T) {
	spin := newTestSpinner()
	got := renderTable(nil, spin, 80)
	if got != "" {
		t.Errorf("renderTable empty: got %q, want empty string", got)
	}
}

func TestRenderTableQueuedRow(t *testing.T) {
	rows := []issueRow{
		{number: 42, title: "fix the bug"},
	}
	spin := newTestSpinner()
	got := stripANSI(renderTable(rows, spin, 80))

	if !strings.Contains(got, markerQueued) {
		t.Errorf("renderTable queued: marker %q not found in %q", markerQueued, got)
	}
	if !strings.Contains(got, "#42") {
		t.Errorf("renderTable queued: issue number not found in %q", got)
	}
	if !strings.Contains(got, "fix the bug") {
		t.Errorf("renderTable queued: title not found in %q", got)
	}
}

func TestRenderTableInProgressRowShowsStage(t *testing.T) {
	rows := []issueRow{
		{number: 42, title: "fix the bug", stage: "implement"},
	}
	spin := newTestSpinner()
	got := stripANSI(renderTable(rows, spin, 80))

	// In-progress: should contain the uppercased stage label in a badge.
	if !strings.Contains(got, "IMPLEMENT") {
		t.Errorf("renderTable in-progress: stage badge not found in %q", got)
	}
	// Should not show queued marker.
	if strings.Contains(got, markerQueued) {
		t.Errorf("renderTable in-progress: queued marker %q should not appear in %q", markerQueued, got)
	}
}

func TestRenderTableCompletedRow(t *testing.T) {
	rows := []issueRow{
		{number: 42, title: "fix the bug", status: "implemented", stage: "merged"},
	}
	spin := newTestSpinner()
	got := stripANSI(renderTable(rows, spin, 80))

	if !strings.Contains(got, markerCompleted) {
		t.Errorf("renderTable completed: marker %q not found in %q", markerCompleted, got)
	}
}

func TestRenderTableFailedRow(t *testing.T) {
	rows := []issueRow{
		{number: 42, title: "fix the bug", status: "failed", stage: "failed", errMsg: "timeout"},
	}
	spin := newTestSpinner()
	got := stripANSI(renderTable(rows, spin, 80))

	if !strings.Contains(got, markerFailed) {
		t.Errorf("renderTable failed: marker %q not found in %q", markerFailed, got)
	}
	if !strings.Contains(got, "timeout") {
		t.Errorf("renderTable failed: errMsg not found in %q", got)
	}
}

func TestRenderTableFailedRowErrMsgTruncated(t *testing.T) {
	longErr := strings.Repeat("e", 200)
	rows := []issueRow{
		{number: 42, title: "fix the bug", status: "failed", stage: "failed", errMsg: longErr},
	}
	spin := newTestSpinner()
	got := stripANSI(renderTable(rows, spin, 80))

	if !strings.Contains(got, "…") {
		t.Errorf("renderTable long errMsg: expected ellipsis in %q", got)
	}
	if strings.Contains(got, longErr) {
		t.Errorf("renderTable long errMsg: full 200-char error should be truncated")
	}
}

func TestRenderTableSuccessHasNoError(t *testing.T) {
	rows := []issueRow{
		{number: 42, title: "fix the bug", status: "implemented", stage: "merged", errMsg: ""},
	}
	spin := newTestSpinner()
	got := stripANSI(renderTable(rows, spin, 80))

	if !strings.Contains(got, markerCompleted) {
		t.Errorf("renderTable success: marker %q not found in %q", markerCompleted, got)
	}
	// No error text should appear for a successful issue.
	if strings.Contains(got, "error") || strings.Contains(got, "…") {
		t.Errorf("renderTable success: unexpected error text in %q", got)
	}
}

func TestRenderTableTitleTruncation(t *testing.T) {
	title := strings.Repeat("x", 120)
	rows := []issueRow{
		{number: 1, title: title},
	}
	spin := newTestSpinner()
	got := stripANSI(renderTable(rows, spin, 80))

	if !strings.Contains(got, "…") {
		t.Errorf("renderTable truncation: expected ellipsis in %q", got)
	}
	if strings.Contains(got, strings.Repeat("x", 120)) {
		t.Errorf("renderTable truncation: full 120-char title should be truncated")
	}
}

// --- Model Update message handler tests ---

func TestModelIssueStartedMsg(t *testing.T) {
	m := New("repo", "ms", "ts", "", "", "", nil)
	next, _ := m.Update(IssueStartedMsg{Number: 42, Title: "fix the bug"})
	updated := next.(Model)

	if len(updated.issues) != 1 {
		t.Fatalf("IssueStartedMsg: expected 1 issue, got %d", len(updated.issues))
	}
	if updated.issues[0].number != 42 {
		t.Errorf("IssueStartedMsg: issue number: got %d, want 42", updated.issues[0].number)
	}
	if updated.issues[0].title != "fix the bug" {
		t.Errorf("IssueStartedMsg: title: got %q, want %q", updated.issues[0].title, "fix the bug")
	}
	if updated.issueIndex[42] != 0 {
		t.Errorf("IssueStartedMsg: issueIndex[42]: got %d, want 0", updated.issueIndex[42])
	}
}

func TestModelIssueStageChangedMsg(t *testing.T) {
	m := New("repo", "ms", "ts", "", "", "", nil)

	// First start an issue.
	next, _ := m.Update(IssueStartedMsg{Number: 42, Title: "fix the bug"})
	m = next.(Model)

	// Then change its stage.
	next, _ = m.Update(IssueStageChangedMsg{Number: 42, Stage: "verify"})
	updated := next.(Model)

	idx := updated.issueIndex[42]
	if updated.issues[idx].stage != "verify" {
		t.Errorf("IssueStageChangedMsg: stage: got %q, want %q", updated.issues[idx].stage, "verify")
	}
}

func TestModelIssueCompletedMsg(t *testing.T) {
	m := New("repo", "ms", "ts", "", "", "", nil)

	next, _ := m.Update(IssueStartedMsg{Number: 42, Title: "fix the bug"})
	m = next.(Model)
	next, _ = m.Update(IssueStageChangedMsg{Number: 42, Stage: "implement"})
	m = next.(Model)
	next, _ = m.Update(IssueCompletedMsg{
		Number:   42,
		Title:    "fix the bug",
		Status:   "implemented",
		PRNumber: 100,
		Retries:  1,
		ErrMsg:   "",
	})
	updated := next.(Model)

	idx := updated.issueIndex[42]
	if updated.issues[idx].status != "implemented" {
		t.Errorf("IssueCompletedMsg: status: got %q, want %q", updated.issues[idx].status, "implemented")
	}
	if updated.issues[idx].prNumber != 100 {
		t.Errorf("IssueCompletedMsg: prNumber: got %d, want 100", updated.issues[idx].prNumber)
	}
}

func TestModelStageChangedUnknownIssue(t *testing.T) {
	// Should not panic or add rows for unknown issue numbers.
	m := New("repo", "ms", "ts", "", "", "", nil)
	next, _ := m.Update(IssueStageChangedMsg{Number: 999, Stage: "verify"})
	updated := next.(Model)
	if len(updated.issues) != 0 {
		t.Errorf("IssueStageChangedMsg unknown: expected 0 issues, got %d", len(updated.issues))
	}
}

func TestModelMultipleIssues(t *testing.T) {
	m := New("repo", "ms", "ts", "", "", "", nil)

	next, _ := m.Update(IssueStartedMsg{Number: 10, Title: "issue ten"})
	m = next.(Model)
	next, _ = m.Update(IssueStartedMsg{Number: 20, Title: "issue twenty"})
	m = next.(Model)

	if len(m.issues) != 2 {
		t.Fatalf("multiple issues: expected 2, got %d", len(m.issues))
	}
	if m.issueIndex[10] != 0 {
		t.Errorf("issueIndex[10]: got %d, want 0", m.issueIndex[10])
	}
	if m.issueIndex[20] != 1 {
		t.Errorf("issueIndex[20]: got %d, want 1", m.issueIndex[20])
	}
}

// --- Model View table integration ---

func TestModelViewContainsTable(t *testing.T) {
	m := New("repo", "ms", "ts", "", "", "", nil)
	next, _ := m.Update(IssueStartedMsg{Number: 42, Title: "fix the bug"})
	m = next.(Model)
	m.width = 80

	got := stripANSI(m.View())
	if !strings.Contains(got, "#42") {
		t.Errorf("View with issue: #42 not found in %q", got)
	}
	if !strings.Contains(got, "fix the bug") {
		t.Errorf("View with issue: title not found in %q", got)
	}
}
