package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// --- PRUpdateMsg tests ---

func TestWatchModel_PRUpdateMsg_AddsPRs(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)
	prs := []WatchPRRow{
		{Number: 42, Title: "Fix the bug", Labels: []string{"godark:awaiting-human-review"}},
		{Number: 99, Title: "Add feature", Labels: []string{}},
	}

	next, _ := m.Update(PRUpdateMsg{PRs: prs})
	updated := next.(WatchModel)

	if len(updated.prs) != 2 {
		t.Fatalf("prs: got %d, want 2", len(updated.prs))
	}
	if updated.prs[0].Number != 42 {
		t.Errorf("prs[0].Number: got %d, want 42", updated.prs[0].Number)
	}
	if updated.prs[1].Number != 99 {
		t.Errorf("prs[1].Number: got %d, want 99", updated.prs[1].Number)
	}
}

func TestWatchModel_PRUpdateMsg_ReplacesExistingPRs(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)

	next, _ := m.Update(PRUpdateMsg{PRs: []WatchPRRow{{Number: 1, Title: "Old"}}})
	next, _ = next.(WatchModel).Update(PRUpdateMsg{PRs: []WatchPRRow{{Number: 2, Title: "New"}}})
	updated := next.(WatchModel)

	if len(updated.prs) != 1 {
		t.Fatalf("prs: got %d, want 1", len(updated.prs))
	}
	if updated.prs[0].Number != 2 {
		t.Errorf("prs[0].Number: got %d, want 2", updated.prs[0].Number)
	}
}

// --- ActivityMsg tests ---

func TestWatchModel_ActivityMsg_Appends(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)

	next, _ := m.Update(ActivityMsg{Text: "PR #42: APPROVED — merging"})
	updated := next.(WatchModel)

	if len(updated.activity) != 1 {
		t.Fatalf("activity: got %d, want 1", len(updated.activity))
	}
	if updated.activity[0] != "PR #42: APPROVED — merging" {
		t.Errorf("activity[0]: got %q", updated.activity[0])
	}
}

func TestWatchModel_ActivityMsg_TrimsAt10(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)

	var current tea.Model = m
	for i := 0; i < 12; i++ {
		next, _ := current.Update(ActivityMsg{Text: strings.Repeat("x", i+1)})
		current = next
	}
	final := current.(WatchModel)

	if len(final.activity) != 10 {
		t.Errorf("activity len: got %d, want 10", len(final.activity))
	}
	// Oldest two entries (i=0 "x", i=1 "xx") should have been dropped.
	// Last entry should be "xxxxxxxxxxxx" (12 x's).
	last := final.activity[9]
	if last != strings.Repeat("x", 12) {
		t.Errorf("activity last entry: got %q, want 12 x's", last)
	}
}

// --- WatchDoneMsg tests ---

func TestWatchModel_WatchDoneMsg_SetsDone(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)

	next, _ := m.Update(WatchDoneMsg{})
	updated := next.(WatchModel)

	if !updated.done {
		t.Error("done should be true after WatchDoneMsg")
	}
}

func TestWatchModel_WatchDoneMsg_QuitOnCtrlC(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)

	next, _ := m.Update(WatchDoneMsg{})
	done := next.(WatchModel)

	_, cmd := done.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected quit cmd after ctrl+c when done, got nil")
	}
}

func TestWatchModel_WatchDoneMsg_QuitOnQ(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)

	next, _ := m.Update(WatchDoneMsg{})
	done := next.(WatchModel)

	_, cmd := done.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("expected quit cmd after q when done, got nil")
	}
}

// --- Empty state tests ---

func TestWatchModel_View_EmptyState(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)
	got := stripANSI(m.View())

	if !strings.Contains(got, "no PRs awaiting review") {
		t.Errorf("View empty state: expected 'no PRs awaiting review', got:\n%s", got)
	}
}

// --- View tests ---

func TestWatchModel_View_ShowsRepoName(t *testing.T) {
	m := NewWatchModel("peter-stratton/dark-factory", time.Minute, nil)
	got := stripANSI(m.View())

	if !strings.Contains(got, "peter-stratton/dark-factory") {
		t.Errorf("View: repo name not found in output:\n%s", got)
	}
}

func TestWatchModel_View_ShowsHeader(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)
	got := stripANSI(m.View())

	if !strings.Contains(got, "godark watch") {
		t.Errorf("View: 'godark watch' not found in output:\n%s", got)
	}
}

func TestWatchModel_View_ShowsPRTable(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)
	next, _ := m.Update(PRUpdateMsg{PRs: []WatchPRRow{
		{Number: 101, Title: "My test PR", Labels: []string{"godark:awaiting-human-review"}},
	}})
	got := stripANSI(next.(WatchModel).View())

	if !strings.Contains(got, "#101") {
		t.Errorf("View: '#101' not found in output:\n%s", got)
	}
	if !strings.Contains(got, "My test PR") {
		t.Errorf("View: PR title not found in output:\n%s", got)
	}
}

func TestWatchModel_View_ShowsActivityLog(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)
	next, _ := m.Update(ActivityMsg{Text: "PR #5: CHANGES_REQUESTED"})
	got := stripANSI(next.(WatchModel).View())

	if !strings.Contains(got, "PR #5: CHANGES_REQUESTED") {
		t.Errorf("View: activity entry not found in output:\n%s", got)
	}
}

func TestWatchModel_View_ShowsFooter(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)
	got := stripANSI(m.View())

	if !strings.Contains(got, "ctrl+c") {
		t.Errorf("View footer: ctrl+c hint not found in output:\n%s", got)
	}
}

func TestWatchModel_View_DoneFooter(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)
	next, _ := m.Update(WatchDoneMsg{})
	got := stripANSI(next.(WatchModel).View())

	if !strings.Contains(got, "q") {
		t.Errorf("View done footer: quit hint not found in output:\n%s", got)
	}
}

// --- Window size test ---

func TestWatchModel_WindowSize(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated := next.(WatchModel)

	if updated.width != 120 {
		t.Errorf("width: got %d, want 120", updated.width)
	}
	if updated.height != 40 {
		t.Errorf("height: got %d, want 40", updated.height)
	}
}

// --- Cancellation test ---

func TestWatchModel_CtrlC_CancelsCancelFn(t *testing.T) {
	cancelled := false
	m := NewWatchModel("owner/repo", time.Minute, func() { cancelled = true })

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	updated := next.(WatchModel)

	if !cancelled {
		t.Error("cancelFn should have been called on ctrl+c")
	}
	if !updated.cancelling {
		t.Error("cancelling should be true after ctrl+c")
	}
}

func TestWatchModel_PRUpdateMsg_SetsLastPoll(t *testing.T) {
	m := NewWatchModel("owner/repo", time.Minute, nil)
	if !m.lastPoll.IsZero() {
		t.Error("lastPoll should be zero before first PRUpdateMsg")
	}

	next, _ := m.Update(PRUpdateMsg{PRs: nil})
	updated := next.(WatchModel)

	if updated.lastPoll.IsZero() {
		t.Error("lastPoll should be non-zero after PRUpdateMsg")
	}
}
