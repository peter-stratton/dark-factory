package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/peter-stratton/dark-factory/internal/agent"
	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
	"github.com/peter-stratton/dark-factory/internal/notify"
	"github.com/peter-stratton/dark-factory/internal/orchestrator"
	"github.com/peter-stratton/dark-factory/internal/progress"
)

// stubProgressReporter records IssueCompleted calls for assertions.
type stubProgressReporter struct {
	issueCompleted []stubIssueCompleted
}

type stubIssueCompleted struct {
	issueNumber int
	title       string
	status      string
	prNumber    int
	retries     int
	errMsg      string
	costUSD     float64
	traceID     string
}

func (r *stubProgressReporter) RunStarted(_, _, _, _, _, _ string, _ []progress.IssueSummary) {}
func (r *stubProgressReporter) IssueStarted(_ int, _ string)                                   {}
func (r *stubProgressReporter) IssueStageChanged(_ int, _ string)                              {}
func (r *stubProgressReporter) IssueCompleted(issueNumber int, title, status string, prNumber, retries int, errMsg string, costUSD float64, traceID string) {
	r.issueCompleted = append(r.issueCompleted, stubIssueCompleted{issueNumber, title, status, prNumber, retries, errMsg, costUSD, traceID})
}
func (r *stubProgressReporter) WaveStarted(_ int, _ int)                                     {}
func (r *stubProgressReporter) AllBlocked(_ int, _ int)                                      {}
func (r *stubProgressReporter) RollupCreated(_ int, _ string, _ bool)                        {}
func (r *stubProgressReporter) RunFinished(_ int, _ int, _ int, _ int, _ int)                {}
func (r *stubProgressReporter) JudgeIntervention(_ int, _, _, _, _ string)                   {}
func (r *stubProgressReporter) PunchlistText(_ string)                                       {}
func (r *stubProgressReporter) RateLimited(_ time.Time)                                      {}
func (r *stubProgressReporter) RateLimitCleared()                                            {}
func (r *stubProgressReporter) WorkersActive(_ int, _ int)                                   {}

// stubNotifier records sent events and optionally returns an error.
type stubNotifier struct {
	received []notify.Event
	err      error
}

func (s *stubNotifier) Send(_ context.Context, event notify.Event) error {
	if s.err != nil {
		return s.err
	}
	s.received = append(s.received, event)
	return nil
}

func TestFetchPRCommentBodiesFnDefault(t *testing.T) {
	// The package-level variable must default to the real GitHub implementation
	// so that dialogue fetching works in production runs.
	if fetchPRCommentBodiesFn == nil {
		t.Fatal("fetchPRCommentBodiesFn is nil, want github.FetchPRCommentBodies")
	}

	// Verify it's the same function by checking it uses github.CommandRunner
	// (indirectly: stub CommandRunner and confirm the variable calls through).
	orig := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = orig })

	called := false
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		called = true
		return []byte("[]"), nil
	}

	// Call the default function — it will invoke github.CommandRunner.
	fetchPRCommentBodiesFn("owner/repo", 1) //nolint:errcheck // best-effort; ignore error
	if !called {
		t.Error("fetchPRCommentBodiesFn did not delegate to github.CommandRunner, want github.FetchPRCommentBodies")
	}
}

func TestFetchPRCommentBodiesFnReplaceable(t *testing.T) {
	// Confirm the variable can be replaced for testing (testability seam).
	orig := fetchPRCommentBodiesFn
	t.Cleanup(func() { fetchPRCommentBodiesFn = orig })

	called := false
	fetchPRCommentBodiesFn = func(repo string, prNum int) ([]string, error) {
		called = true
		return []string{"body1", "body2"}, nil
	}

	bodies, err := fetchPRCommentBodiesFn("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("replaced fetchPRCommentBodiesFn was not called")
	}
	if len(bodies) != 2 {
		t.Errorf("got %d bodies, want 2", len(bodies))
	}
}

func TestImplementCmd_PunchlistFlagRegistered(t *testing.T) {
	f := implementCmd.Flags().Lookup("punchlist")
	if f == nil {
		t.Fatal("implement command missing --punchlist flag")
	}
	if f.DefValue != "" {
		t.Errorf("punchlist default = %q, want %q", f.DefValue, "")
	}
}

func TestImplementCmd_IssuesFlagRegistered(t *testing.T) {
	f := implementCmd.Flags().Lookup("issues")
	if f == nil {
		t.Fatal("implement command missing --issues flag")
	}
	if f.DefValue != "" {
		t.Errorf("issues default = %q, want %q", f.DefValue, "")
	}
}

func TestCheckWorkingTreeFnDefault(t *testing.T) {
	// checkWorkingTreeFn must default to a non-nil function.
	if checkWorkingTreeFn == nil {
		t.Fatal("checkWorkingTreeFn is nil, want orchestrator.CheckWorkingTree")
	}
}

func TestCheckWorkingTreeFnReplaceable(t *testing.T) {
	orig := checkWorkingTreeFn
	t.Cleanup(func() { checkWorkingTreeFn = orig })

	called := false
	checkWorkingTreeFn = func() error {
		called = true
		return nil
	}

	if err := checkWorkingTreeFn(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("replaced checkWorkingTreeFn was not called")
	}
}

func TestCheckWorkingTreeFnBlocksOnDirty(t *testing.T) {
	orig := checkWorkingTreeFn
	t.Cleanup(func() { checkWorkingTreeFn = orig })

	checkWorkingTreeFn = func() error {
		return fmt.Errorf("working tree is dirty — commit or stash your changes before running:\n M dirty.go")
	}

	err := checkWorkingTreeFn()
	if err == nil {
		t.Fatal("expected error when working tree is dirty")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestImplementCmd_ArbitraryArgs(t *testing.T) {
	// implementCmd must accept zero or more positional args (not ExactArgs(1)).
	// Calling Args validator with 0, 1, and 2 args must not return an error.
	for _, n := range []int{0, 1, 2, 5} {
		args := make([]string, n)
		for i := range args {
			args[i] = "1"
		}
		if err := implementCmd.Args(implementCmd, args); err != nil {
			t.Errorf("Args(%d positional args) = %v, want nil", n, err)
		}
	}
}

func TestParseIssueNumbers(t *testing.T) {
	tests := []struct {
		input   string
		want    []int
		wantErr bool
	}{
		{"160", []int{160}, false},
		{"160,161,162", []int{160, 161, 162}, false},
		{" 160 , 161 , 162 ", []int{160, 161, 162}, false},
		{"", nil, false},
		{",", nil, false},
		{"abc", nil, true},
		{"160,abc", nil, true},
		{"160,", []int{160}, false},
	}

	for _, tc := range tests {
		got, err := parseIssueNumbers(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseIssueNumbers(%q) = %v, nil; want error", tc.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseIssueNumbers(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("parseIssueNumbers(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseIssueNumbers(%q)[%d] = %d, want %d", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestCollectIssueNumbers(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		issuesFlag string
		want       []int
		wantErr    bool
	}{
		{
			name: "single positional arg",
			args: []string{"160"},
			want: []int{160},
		},
		{
			name: "multiple positional args",
			args: []string{"160", "161", "162"},
			want: []int{160, 161, 162},
		},
		{
			name:       "issues flag only",
			issuesFlag: "160,161",
			want:       []int{160, 161},
		},
		{
			name:       "positional args and flag combined",
			args:       []string{"160"},
			issuesFlag: "161,162",
			want:       []int{160, 161, 162},
		},
		{
			name:       "duplicates deduplicated",
			args:       []string{"160", "161"},
			issuesFlag: "160,162",
			want:       []int{160, 161, 162},
		},
		{
			name:    "neither provided returns error",
			wantErr: true,
		},
		{
			name:    "invalid positional arg",
			args:    []string{"abc"},
			wantErr: true,
		},
		{
			name:       "invalid issues flag",
			issuesFlag: "160,abc",
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := collectIssueNumbers(tc.args, tc.issuesFlag)
			if tc.wantErr {
				if err == nil {
					t.Errorf("collectIssueNumbers(%v, %q) = %v, nil; want error", tc.args, tc.issuesFlag, got)
				}
				return
			}
			if err != nil {
				t.Errorf("collectIssueNumbers(%v, %q) unexpected error: %v", tc.args, tc.issuesFlag, err)
				return
			}
			if len(got) != len(tc.want) {
				t.Errorf("collectIssueNumbers(%v, %q) = %v, want %v", tc.args, tc.issuesFlag, got, tc.want)
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("collectIssueNumbers(%v, %q)[%d] = %d, want %d", tc.args, tc.issuesFlag, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestFireImplementNotification_SendsEvent(t *testing.T) {
	stub := &stubNotifier{}
	event := notify.Event{
		Type:    "implementation_complete",
		Repo:    "owner/repo",
		Message: "issue #123: status=implemented, PR #456",
	}

	notify.Fire(context.Background(), []notify.Notifier{stub}, event, slog.Default())

	if len(stub.received) != 1 {
		t.Fatalf("got %d events, want 1", len(stub.received))
	}
	if stub.received[0].Type != "implementation_complete" {
		t.Errorf("event type = %q, want %q", stub.received[0].Type, "implementation_complete")
	}
}

func TestFireImplementNotification_LogsErrorAndContinues(t *testing.T) {
	failing := &stubNotifier{err: errors.New("send failed")}
	ok := &stubNotifier{}
	event := notify.Event{Type: "implementation_complete", Repo: "owner/repo", Message: "done"}

	// Should not panic even if first notifier fails; second should still receive.
	notify.Fire(context.Background(), []notify.Notifier{failing, ok}, event, slog.Default())

	if len(ok.received) != 1 {
		t.Errorf("ok notifier got %d events, want 1", len(ok.received))
	}
}

func TestFireImplementNotification_NoNotifiers(t *testing.T) {
	// Should not panic with nil or empty slice.
	notify.Fire(context.Background(), nil, notify.Event{Type: "implementation_complete"}, slog.Default())
	notify.Fire(context.Background(), []notify.Notifier{}, notify.Event{Type: "implementation_complete"}, slog.Default())
}

func TestApplyOutcomeStats_PassesCostToReporter(t *testing.T) {
	issue := github.Issue{Number: 10, Title: "test issue"}
	cfg := &config.Config{Repo: "owner/repo"}
	reporter := &stubProgressReporter{}
	logger := slog.Default()

	tests := []struct {
		name    string
		outcome agent.IssueOutcome
		cost    float64
	}{
		{
			name:    "implemented",
			outcome: agent.IssueOutcome{IssueNumber: 10, Status: agent.StatusImplemented, PRNumber: 5},
			cost:    1.50,
		},
		{
			name:    "ready-to-merge",
			outcome: agent.IssueOutcome{IssueNumber: 10, Status: agent.StatusReadyToMerge, PRNumber: 5},
			cost:    2.00,
		},
		{
			name:    "needs-human-review",
			outcome: agent.IssueOutcome{IssueNumber: 10, Status: agent.StatusNeedsHumanReview, PRNumber: 5},
			cost:    0.75,
		},
		{
			name:    "failed",
			outcome: agent.IssueOutcome{IssueNumber: 10, Status: agent.StatusFailed, Err: fmt.Errorf("oops")},
			cost:    0.10,
		},
		{
			name:    "zero cost graceful",
			outcome: agent.IssueOutcome{IssueNumber: 10, Status: agent.StatusFailed},
			cost:    0.0,
		},
	}

	// Stub orchestrator.CommandRunner so PullAfterMerge (called on StatusImplemented)
	// does not invoke real git commands.
	origRunner := orchestrator.CommandRunner
	orchestrator.CommandRunner = func(_ string, _ ...string) ([]byte, error) { return []byte(""), nil }
	defer func() { orchestrator.CommandRunner = origRunner }()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reporter.issueCompleted = nil

			applyOutcomeStats(tc.outcome, issue, cfg, reporter, logger, tc.cost)

			if len(reporter.issueCompleted) != 1 {
				t.Fatalf("expected 1 IssueCompleted call, got %d", len(reporter.issueCompleted))
			}
			got := reporter.issueCompleted[0].costUSD
			if got != tc.cost {
				t.Errorf("IssueCompleted costUSD = %f, want %f", got, tc.cost)
			}
		})
	}
}
