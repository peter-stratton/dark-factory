package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/phs/dark-factory/internal/progress"
)

// mockSender records every message passed to Send so tests can assert on them.
type mockSender struct {
	msgs []tea.Msg
}

func (m *mockSender) Send(msg tea.Msg) {
	m.msgs = append(m.msgs, msg)
}

func newTestReporter() (*TUIReporter, *mockSender) {
	s := &mockSender{}
	return &TUIReporter{p: s}, s
}

func TestIssueStartedSendsMessage(t *testing.T) {
	r, s := newTestReporter()
	r.IssueStarted(42, "add endpoint")

	if len(s.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.msgs))
	}
	msg, ok := s.msgs[0].(IssueStartedMsg)
	if !ok {
		t.Fatalf("expected IssueStartedMsg, got %T", s.msgs[0])
	}
	if msg.Number != 42 {
		t.Errorf("Number: got %d, want 42", msg.Number)
	}
	if msg.Title != "add endpoint" {
		t.Errorf("Title: got %q, want %q", msg.Title, "add endpoint")
	}
}

func TestIssueStageChangedSendsMessage(t *testing.T) {
	r, s := newTestReporter()
	r.IssueStageChanged(7, "reviewing")

	if len(s.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.msgs))
	}
	msg, ok := s.msgs[0].(IssueStageChangedMsg)
	if !ok {
		t.Fatalf("expected IssueStageChangedMsg, got %T", s.msgs[0])
	}
	if msg.Number != 7 {
		t.Errorf("Number: got %d, want 7", msg.Number)
	}
	if msg.Stage != "reviewing" {
		t.Errorf("Stage: got %q, want %q", msg.Stage, "reviewing")
	}
}

func TestIssueCompletedSendsMessage(t *testing.T) {
	r, s := newTestReporter()
	r.IssueCompleted(42, "add endpoint", "implemented", 87, 0, "", 0.0)

	if len(s.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.msgs))
	}
	msg, ok := s.msgs[0].(IssueCompletedMsg)
	if !ok {
		t.Fatalf("expected IssueCompletedMsg, got %T", s.msgs[0])
	}
	if msg.Number != 42 {
		t.Errorf("Number: got %d, want 42", msg.Number)
	}
	if msg.Title != "add endpoint" {
		t.Errorf("Title: got %q, want %q", msg.Title, "add endpoint")
	}
	if msg.Status != "implemented" {
		t.Errorf("Status: got %q, want %q", msg.Status, "implemented")
	}
	if msg.PRNumber != 87 {
		t.Errorf("PRNumber: got %d, want 87", msg.PRNumber)
	}
	if msg.Retries != 0 {
		t.Errorf("Retries: got %d, want 0", msg.Retries)
	}
	if msg.ErrMsg != "" {
		t.Errorf("ErrMsg: got %q, want %q", msg.ErrMsg, "")
	}
	if msg.CostUSD != 0.0 {
		t.Errorf("CostUSD: got %f, want 0.0", msg.CostUSD)
	}
}

func TestWaveStartedSendsMessage(t *testing.T) {
	r, s := newTestReporter()
	r.WaveStarted(2, 5)

	if len(s.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.msgs))
	}
	msg, ok := s.msgs[0].(WaveStartedMsg)
	if !ok {
		t.Fatalf("expected WaveStartedMsg, got %T", s.msgs[0])
	}
	if msg.Wave != 2 {
		t.Errorf("Wave: got %d, want 2", msg.Wave)
	}
	if msg.Count != 5 {
		t.Errorf("Count: got %d, want 5", msg.Count)
	}
}

func TestRunFinishedSendsMessage(t *testing.T) {
	r, s := newTestReporter()
	r.RunFinished(3, 1, 0, 1, 2)

	if len(s.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.msgs))
	}
	msg, ok := s.msgs[0].(RunFinishedMsg)
	if !ok {
		t.Fatalf("expected RunFinishedMsg, got %T", s.msgs[0])
	}
	if msg.Implemented != 3 {
		t.Errorf("Implemented: got %d, want 3", msg.Implemented)
	}
	if msg.ReadyToMerge != 1 {
		t.Errorf("ReadyToMerge: got %d, want 1", msg.ReadyToMerge)
	}
	if msg.NeedsHumanReview != 0 {
		t.Errorf("NeedsHumanReview: got %d, want 0", msg.NeedsHumanReview)
	}
	if msg.Failed != 1 {
		t.Errorf("Failed: got %d, want 1", msg.Failed)
	}
	if msg.Blocked != 2 {
		t.Errorf("Blocked: got %d, want 2", msg.Blocked)
	}
}

func TestAllBlockedSendsRunFinishedMsg(t *testing.T) {
	r, s := newTestReporter()
	r.AllBlocked(10, 10)

	if len(s.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.msgs))
	}
	msg, ok := s.msgs[0].(RunFinishedMsg)
	if !ok {
		t.Fatalf("expected RunFinishedMsg, got %T", s.msgs[0])
	}
	if msg.Blocked != 10 {
		t.Errorf("Blocked: got %d, want 10", msg.Blocked)
	}
	if msg.Implemented != 0 || msg.ReadyToMerge != 0 || msg.NeedsHumanReview != 0 || msg.Failed != 0 {
		t.Errorf("expected all non-Blocked counts to be 0, got %+v", msg)
	}
}

func TestRunStartedSendsMessage(t *testing.T) {
	r, s := newTestReporter()
	issues := []progress.IssueSummary{
		{Number: 1, Title: "first"},
		{Number: 2, Title: "second"},
	}
	r.RunStarted("owner/repo", "v1.0", "2026-03-14T10:00:00Z", "main", "feat", "rollup", issues)

	if len(s.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.msgs))
	}
	msg, ok := s.msgs[0].(RunStartedMsg)
	if !ok {
		t.Fatalf("expected RunStartedMsg, got %T", s.msgs[0])
	}
	if msg.Repo != "owner/repo" {
		t.Errorf("Repo: got %q, want %q", msg.Repo, "owner/repo")
	}
	if msg.Milestone != "v1.0" {
		t.Errorf("Milestone: got %q, want %q", msg.Milestone, "v1.0")
	}
	if len(msg.Issues) != 2 {
		t.Errorf("Issues: got %d, want 2", len(msg.Issues))
	}
	if msg.MergeFeature != "feat" {
		t.Errorf("MergeFeature: got %q, want %q", msg.MergeFeature, "feat")
	}
	if msg.MergeRollup != "rollup" {
		t.Errorf("MergeRollup: got %q, want %q", msg.MergeRollup, "rollup")
	}
}

func TestRollupCreatedSendsMessage(t *testing.T) {
	r, s := newTestReporter()
	r.RollupCreated(99, "https://github.com/owner/repo/pull/99", true)

	if len(s.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.msgs))
	}
	msg, ok := s.msgs[0].(RollupCreatedMsg)
	if !ok {
		t.Fatalf("expected RollupCreatedMsg, got %T", s.msgs[0])
	}
	if msg.PRNumber != 99 {
		t.Errorf("PRNumber: got %d, want 99", msg.PRNumber)
	}
	if msg.PRURL != "https://github.com/owner/repo/pull/99" {
		t.Errorf("PRURL: got %q", msg.PRURL)
	}
	if !msg.Merged {
		t.Errorf("Merged: got false, want true")
	}
}

func TestPunchlistTextIsNoOp(t *testing.T) {
	r, s := newTestReporter()
	r.PunchlistText("some punchlist content")

	if len(s.msgs) != 0 {
		t.Errorf("PunchlistText should send no messages, got %d", len(s.msgs))
	}
}

func TestRunAbortedSendsMessage(t *testing.T) {
	r, s := newTestReporter()
	r.RunAborted("API rate limit reached, resets 5pm (UTC)")

	if len(s.msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.msgs))
	}
	msg, ok := s.msgs[0].(RunAbortedMsg)
	if !ok {
		t.Fatalf("expected RunAbortedMsg, got %T", s.msgs[0])
	}
	if msg.Reason != "API rate limit reached, resets 5pm (UTC)" {
		t.Errorf("Reason: got %q, want %q", msg.Reason, "API rate limit reached, resets 5pm (UTC)")
	}
}
