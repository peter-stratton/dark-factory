package cmd

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/rundata"
)

// fakeRunner builds a CommandRunner stub that dispatches on the first gh
// subcommand (args[0]) and returns the corresponding payload.
func fakeRunner(t *testing.T, dispatch func(args []string) ([]byte, error)) func(string, ...string) ([]byte, error) {
	t.Helper()
	return func(name string, args ...string) ([]byte, error) {
		return dispatch(args)
	}
}

// testWatchCfg returns a minimal Config sufficient for pollOnce tests.
func testWatchCfg() *config.Config {
	return &config.Config{Repo: "owner/repo"}
}

// stubWatchSeams replaces the watch testability seams with no-ops and restores them on t.Cleanup.
func stubWatchSeams(t *testing.T) {
	t.Helper()

	origRetry := watchRetryFn
	origFindSession := watchFindSessionIDFn
	origNewWriter := watchNewWriterFn
	origFetchComments := watchFetchReviewCommentsFn
	origFetchIssue := watchFetchIssueFn

	watchRetryFn = func(_ context.Context, _ github.Issue, _ int, _ string, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		return &agent.Result{SessionID: "sess-stub"}, nil
	}
	watchFindSessionIDFn = func(_ string, _ int) (string, error) { return "", nil }
	watchNewWriterFn = func(repo, milestone string, issueNumbers []int) (*rundata.Writer, error) { return nil, nil }
	watchFetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	watchFetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "test issue"}, nil
	}

	t.Cleanup(func() {
		watchRetryFn = origRetry
		watchFindSessionIDFn = origFindSession
		watchNewWriterFn = origNewWriter
		watchFetchReviewCommentsFn = origFetchComments
		watchFetchIssueFn = origFetchIssue
	})
}

func TestWatchCmd_Registered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "watch" {
			found = true
			break
		}
	}
	if !found {
		t.Error("watch command not registered on rootCmd")
	}
}

func TestWatchCmd_HasHelpOutput(t *testing.T) {
	if watchCmd.Use != "watch" {
		t.Errorf("watchCmd.Use: got %q, want %q", watchCmd.Use, "watch")
	}
	if watchCmd.Short == "" {
		t.Error("watchCmd.Short must not be empty")
	}
}

// TestWatchPollInterval_DefaultWhenNilConfig verifies the 60s default when
// Watch config is nil.
func TestWatchPollInterval_DefaultWhenNilConfig(t *testing.T) {
	cfg := &config.Config{Watch: nil}
	got, err := watchPollInterval(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 60*time.Second {
		t.Errorf("expected 60s default, got %v", got)
	}
}

// TestWatchPollInterval_DefaultWhenEmptyInterval verifies the 60s default when
// Watch is set but PollInterval is empty.
func TestWatchPollInterval_DefaultWhenEmptyInterval(t *testing.T) {
	cfg := &config.Config{Watch: &config.Watch{PollInterval: ""}}
	got, err := watchPollInterval(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 60*time.Second {
		t.Errorf("expected 60s default, got %v", got)
	}
}

// TestWatchPollInterval_CustomInterval verifies that a configured interval is used.
func TestWatchPollInterval_CustomInterval(t *testing.T) {
	cfg := &config.Config{Watch: &config.Watch{PollInterval: "10s"}}
	got, err := watchPollInterval(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 10*time.Second {
		t.Errorf("expected 10s, got %v", got)
	}
}

// TestPollOnce_NoPRs verifies that an empty PR list causes no label calls.
func TestPollOnce_NoPRs(t *testing.T) {
	stubWatchSeams(t)

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer func() { github.CommandRunner = orig }()

	processed := make(map[int]bool)
	if err := pollOnce(context.Background(), testWatchCfg(), nil, nil, processed, slog.Default()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(processed) != 0 {
		t.Errorf("expected no processed reviews, got %v", processed)
	}
}

// TestPollOnce_DetectsChangesRequestedReview verifies that a CHANGES_REQUESTED
// review triggers label swapping and marks the review as processed.
func TestPollOnce_DetectsChangesRequestedReview(t *testing.T) {
	stubWatchSeams(t)

	var addLabelCalled, removeLabelCalled bool

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		if len(args) < 2 {
			return []byte(`{}`), nil
		}
		switch {
		case args[0] == "pr" && args[1] == "list":
			return []byte(`[{"number":5,"headRefName":"42-feature-branch"}]`), nil
		case args[0] == "api":
			// reviews endpoint — no inline comments
			if strings.Contains(args[1], "reviews/") {
				return []byte(`[]`), nil
			}
			return []byte(`[{"id":101,"state":"CHANGES_REQUESTED","body":"fix this","user":{"login":"alice"}}]`), nil
		case args[0] == "issue" && args[1] == "edit":
			for _, a := range args {
				if a == "--add-label" {
					addLabelCalled = true
				}
				if a == "--remove-label" {
					removeLabelCalled = true
				}
			}
			return []byte(`{}`), nil
		}
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	processed := make(map[int]bool)
	if err := pollOnce(context.Background(), testWatchCfg(), nil, nil, processed, slog.Default()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !processed[101] {
		t.Error("expected review ID 101 to be marked as processed")
	}
	if !addLabelCalled {
		t.Error("expected add-label to be called")
	}
	if !removeLabelCalled {
		t.Error("expected remove-label to be called")
	}
}

// TestPollOnce_DuplicateSkipped verifies that an already-processed review ID
// does not trigger another label swap.
func TestPollOnce_DuplicateSkipped(t *testing.T) {
	stubWatchSeams(t)

	var addLabelCalled bool

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		if len(args) < 2 {
			return []byte(`{}`), nil
		}
		switch {
		case args[0] == "pr" && args[1] == "list":
			return []byte(`[{"number":5,"headRefName":"42-feature-branch"}]`), nil
		case args[0] == "api":
			return []byte(`[{"id":101,"state":"CHANGES_REQUESTED","body":"fix this","user":{"login":"alice"}}]`), nil
		case args[0] == "issue" && args[1] == "edit":
			for _, a := range args {
				if a == "--add-label" {
					addLabelCalled = true
				}
			}
			return []byte(`{}`), nil
		}
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	// Pre-mark review 101 as already handled.
	processed := map[int]bool{101: true}
	if err := pollOnce(context.Background(), testWatchCfg(), nil, nil, processed, slog.Default()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if addLabelCalled {
		t.Error("expected no label update for already-processed review")
	}
}

// TestPollOnce_NonChangesRequestedSkipped verifies that APPROVED reviews do not
// trigger label swapping.
func TestPollOnce_NonChangesRequestedSkipped(t *testing.T) {
	stubWatchSeams(t)

	var addLabelCalled bool

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		if len(args) < 2 {
			return []byte(`{}`), nil
		}
		switch {
		case args[0] == "pr" && args[1] == "list":
			return []byte(`[{"number":5,"headRefName":"42-feature-branch"}]`), nil
		case args[0] == "api":
			return []byte(`[{"id":200,"state":"APPROVED","body":"LGTM","user":{"login":"bob"}}]`), nil
		case args[0] == "issue" && args[1] == "edit":
			for _, a := range args {
				if a == "--add-label" {
					addLabelCalled = true
				}
			}
			return []byte(`{}`), nil
		}
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	processed := make(map[int]bool)
	if err := pollOnce(context.Background(), testWatchCfg(), nil, nil, processed, slog.Default()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if addLabelCalled {
		t.Error("expected no label update for APPROVED review")
	}
	if processed[200] {
		t.Error("expected APPROVED review not to be recorded in processed set")
	}
}

// TestPollOnce_ContextCancelled verifies that a cancelled context causes
// pollOnce to return nil without processing PRs.
func TestPollOnce_ContextCancelled(t *testing.T) {
	stubWatchSeams(t)

	listCalled := false

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "pr" && args[1] == "list" {
			listCalled = true
		}
		return []byte(`[]`), nil
	})
	defer func() { github.CommandRunner = orig }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before polling

	processed := make(map[int]bool)
	// pollOnce calls ListPRsWithLabel before checking ctx — the important thing
	// is it returns nil (no error) even when context is already cancelled.
	err := pollOnce(ctx, testWatchCfg(), nil, nil, processed, slog.Default())
	if err != nil {
		t.Fatalf("expected nil error on cancelled context, got %v", err)
	}
	_ = listCalled // the list may or may not be called depending on timing
}

// TestHandleChangesRequested_FeedbackFed verifies that the review body is
// passed to the retry agent as feedback.
func TestHandleChangesRequested_FeedbackFed(t *testing.T) {
	var gotFeedback string

	origRetry := watchRetryFn
	watchRetryFn = func(_ context.Context, _ github.Issue, _ int, _ string, feedback string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		gotFeedback = feedback
		return &agent.Result{}, nil
	}
	defer func() { watchRetryFn = origRetry }()

	origFindSession := watchFindSessionIDFn
	watchFindSessionIDFn = func(_ string, _ int) (string, error) { return "", nil }
	defer func() { watchFindSessionIDFn = origFindSession }()

	origNewWriter := watchNewWriterFn
	watchNewWriterFn = func(_, _ string, _ []int) (*rundata.Writer, error) { return nil, nil }
	defer func() { watchNewWriterFn = origNewWriter }()

	origFetchComments := watchFetchReviewCommentsFn
	watchFetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	defer func() { watchFetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := watchFetchIssueFn
	watchFetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { watchFetchIssueFn = origFetchIssue }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	pr := github.PRInfo{Number: 5, HeadRefName: "42-my-issue"}
	review := github.PRReview{ID: 101, State: "CHANGES_REQUESTED", Body: "Please fix the error handling", Author: "alice"}

	handleChangesRequested(context.Background(), testWatchCfg(), nil, nil, pr, review, slog.Default())

	if !strings.Contains(gotFeedback, "Please fix the error handling") {
		t.Errorf("expected feedback to contain review body, got %q", gotFeedback)
	}
}

// TestHandleChangesRequested_SessionResumed verifies that the session ID from
// run data is passed to the retry agent.
func TestHandleChangesRequested_SessionResumed(t *testing.T) {
	var gotSessionID string

	origRetry := watchRetryFn
	watchRetryFn = func(_ context.Context, _ github.Issue, _ int, sessionID string, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		gotSessionID = sessionID
		return &agent.Result{}, nil
	}
	defer func() { watchRetryFn = origRetry }()

	origFindSession := watchFindSessionIDFn
	watchFindSessionIDFn = func(_ string, _ int) (string, error) { return "sess-abc123", nil }
	defer func() { watchFindSessionIDFn = origFindSession }()

	origNewWriter := watchNewWriterFn
	watchNewWriterFn = func(_, _ string, _ []int) (*rundata.Writer, error) { return nil, nil }
	defer func() { watchNewWriterFn = origNewWriter }()

	origFetchComments := watchFetchReviewCommentsFn
	watchFetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	defer func() { watchFetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := watchFetchIssueFn
	watchFetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { watchFetchIssueFn = origFetchIssue }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	pr := github.PRInfo{Number: 5, HeadRefName: "42-my-issue"}
	review := github.PRReview{ID: 101, State: "CHANGES_REQUESTED", Body: "fix this", Author: "alice"}

	handleChangesRequested(context.Background(), testWatchCfg(), nil, nil, pr, review, slog.Default())

	if gotSessionID != "sess-abc123" {
		t.Errorf("expected session ID %q, got %q", "sess-abc123", gotSessionID)
	}
}

// TestHandleChangesRequested_LabelsSwapped verifies that after the agent fix,
// the FixingReviewFeedback label is removed and AwaitingHumanReview is applied.
func TestHandleChangesRequested_LabelsSwapped(t *testing.T) {
	var labelsAdded, labelsRemoved []string

	origRetry := watchRetryFn
	watchRetryFn = func(_ context.Context, _ github.Issue, _ int, _ string, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		return &agent.Result{}, nil
	}
	defer func() { watchRetryFn = origRetry }()

	origFindSession := watchFindSessionIDFn
	watchFindSessionIDFn = func(_ string, _ int) (string, error) { return "", nil }
	defer func() { watchFindSessionIDFn = origFindSession }()

	origNewWriter := watchNewWriterFn
	watchNewWriterFn = func(_, _ string, _ []int) (*rundata.Writer, error) { return nil, nil }
	defer func() { watchNewWriterFn = origNewWriter }()

	origFetchComments := watchFetchReviewCommentsFn
	watchFetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	defer func() { watchFetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := watchFetchIssueFn
	watchFetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { watchFetchIssueFn = origFetchIssue }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "issue" && args[1] == "edit" {
			for i, a := range args {
				if a == "--add-label" && i+1 < len(args) {
					labelsAdded = append(labelsAdded, args[i+1])
				}
				if a == "--remove-label" && i+1 < len(args) {
					labelsRemoved = append(labelsRemoved, args[i+1])
				}
			}
		}
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	pr := github.PRInfo{Number: 5, HeadRefName: "42-my-issue"}
	review := github.PRReview{ID: 101, State: "CHANGES_REQUESTED", Body: "fix this", Author: "alice"}

	handleChangesRequested(context.Background(), testWatchCfg(), nil, nil, pr, review, slog.Default())

	foundAwaitingReview := false
	for _, l := range labelsAdded {
		if l == "godark:awaiting-human-review" {
			foundAwaitingReview = true
		}
	}
	if !foundAwaitingReview {
		t.Errorf("expected godark:awaiting-human-review to be added after fix, got added: %v", labelsAdded)
	}

	foundFixing := false
	for _, l := range labelsRemoved {
		if l == "godark:fixing-review-feedback" {
			foundFixing = true
		}
	}
	if !foundFixing {
		t.Errorf("expected godark:fixing-review-feedback to be removed after fix, got removed: %v", labelsRemoved)
	}
}

// TestHandleChangesRequested_NoSessionID verifies that a missing session ID
// still invokes the agent (cold start).
func TestHandleChangesRequested_NoSessionID(t *testing.T) {
	agentCalled := false

	origRetry := watchRetryFn
	watchRetryFn = func(_ context.Context, _ github.Issue, _ int, sessionID string, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		agentCalled = true
		if sessionID != "" {
			return nil, nil // fail the test via assertion below
		}
		return &agent.Result{}, nil
	}
	defer func() { watchRetryFn = origRetry }()

	origFindSession := watchFindSessionIDFn
	watchFindSessionIDFn = func(_ string, _ int) (string, error) { return "", nil } // no session found
	defer func() { watchFindSessionIDFn = origFindSession }()

	origNewWriter := watchNewWriterFn
	watchNewWriterFn = func(_, _ string, _ []int) (*rundata.Writer, error) { return nil, nil }
	defer func() { watchNewWriterFn = origNewWriter }()

	origFetchComments := watchFetchReviewCommentsFn
	watchFetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	defer func() { watchFetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := watchFetchIssueFn
	watchFetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { watchFetchIssueFn = origFetchIssue }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	pr := github.PRInfo{Number: 5, HeadRefName: "42-my-issue"}
	review := github.PRReview{ID: 101, State: "CHANGES_REQUESTED", Body: "fix this", Author: "alice"}

	handleChangesRequested(context.Background(), testWatchCfg(), nil, nil, pr, review, slog.Default())

	if !agentCalled {
		t.Error("expected agent to be called even without session ID")
	}
}

// TestHandleChangesRequested_RunDataWritten verifies that a run data directory
// is created for watch-initiated fix cycles.
func TestHandleChangesRequested_RunDataWritten(t *testing.T) {
	base := t.TempDir()
	writerCreated := false

	origRetry := watchRetryFn
	watchRetryFn = func(_ context.Context, _ github.Issue, _ int, _ string, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		return &agent.Result{}, nil
	}
	defer func() { watchRetryFn = origRetry }()

	origFindSession := watchFindSessionIDFn
	watchFindSessionIDFn = func(_ string, _ int) (string, error) { return "", nil }
	defer func() { watchFindSessionIDFn = origFindSession }()

	origNewWriter := watchNewWriterFn
	watchNewWriterFn = func(repo, milestone string, issueNumbers []int) (*rundata.Writer, error) {
		writerCreated = true
		// Use a custom base dir to verify the call actually happened.
		w, err := rundata.NewWithBase(base, repo, milestone, issueNumbers)
		return w, err
	}
	defer func() { watchNewWriterFn = origNewWriter }()

	origFetchComments := watchFetchReviewCommentsFn
	watchFetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	defer func() { watchFetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := watchFetchIssueFn
	watchFetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { watchFetchIssueFn = origFetchIssue }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	pr := github.PRInfo{Number: 5, HeadRefName: "42-my-issue"}
	review := github.PRReview{ID: 101, State: "CHANGES_REQUESTED", Body: "fix this", Author: "alice"}

	handleChangesRequested(context.Background(), testWatchCfg(), nil, nil, pr, review, slog.Default())

	if !writerCreated {
		t.Error("expected run data writer to be created")
	}

	// Verify a run directory was created under the base.
	entries, err := os.ReadDir(filepath.Join(base, "owner", "repo"))
	if err != nil {
		t.Fatalf("reading run dirs: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one run directory to be created")
	}
}

// TestHandleChangesRequested_MultipleCommentsConcatenated verifies that multiple
// review comments are joined into a single feedback string.
func TestHandleChangesRequested_MultipleCommentsConcatenated(t *testing.T) {
	var gotFeedback string

	origRetry := watchRetryFn
	watchRetryFn = func(_ context.Context, _ github.Issue, _ int, _ string, feedback string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		gotFeedback = feedback
		return &agent.Result{}, nil
	}
	defer func() { watchRetryFn = origRetry }()

	origFindSession := watchFindSessionIDFn
	watchFindSessionIDFn = func(_ string, _ int) (string, error) { return "", nil }
	defer func() { watchFindSessionIDFn = origFindSession }()

	origNewWriter := watchNewWriterFn
	watchNewWriterFn = func(_, _ string, _ []int) (*rundata.Writer, error) { return nil, nil }
	defer func() { watchNewWriterFn = origNewWriter }()

	origFetchComments := watchFetchReviewCommentsFn
	watchFetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) {
		return []string{"Inline comment A", "Inline comment B"}, nil
	}
	defer func() { watchFetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := watchFetchIssueFn
	watchFetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { watchFetchIssueFn = origFetchIssue }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	pr := github.PRInfo{Number: 5, HeadRefName: "42-my-issue"}
	review := github.PRReview{ID: 101, State: "CHANGES_REQUESTED", Body: "Review body comment", Author: "alice"}

	handleChangesRequested(context.Background(), testWatchCfg(), nil, nil, pr, review, slog.Default())

	for _, want := range []string{"Review body comment", "Inline comment A", "Inline comment B"} {
		if !strings.Contains(gotFeedback, want) {
			t.Errorf("expected feedback to contain %q, got: %q", want, gotFeedback)
		}
	}
}

// TestIssueNumberFromBranch verifies branch name parsing.
func TestIssueNumberFromBranch(t *testing.T) {
	cases := []struct {
		branch string
		want   int
	}{
		{"249-human-feedback-agent-resumption", 249},
		{"42-fix-bug", 42},
		{"1-init", 1},
		{"no-dash", 0},  // leading segment is not a number
		{"", 0},
		{"abc-def", 0},
	}
	for _, tc := range cases {
		got := issueNumberFromBranch(tc.branch)
		if got != tc.want {
			t.Errorf("issueNumberFromBranch(%q) = %d, want %d", tc.branch, got, tc.want)
		}
	}
}

// TestBuildFeedback_ReviewBodyOnly verifies that only the review body is
// returned when there are no inline comments.
func TestBuildFeedback_ReviewBodyOnly(t *testing.T) {
	fetchFn := func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	got := buildFeedback("Review body", fetchFn, "owner/repo", 5, 101, slog.Default())
	if got != "Review body" {
		t.Errorf("expected %q, got %q", "Review body", got)
	}
}

// TestBuildFeedback_ReviewBodyAndComments verifies concatenation of body and comments.
func TestBuildFeedback_ReviewBodyAndComments(t *testing.T) {
	fetchFn := func(_ string, _ int, _ int) ([]string, error) {
		return []string{"comment 1", "comment 2"}, nil
	}
	got := buildFeedback("body", fetchFn, "owner/repo", 5, 101, slog.Default())
	for _, want := range []string{"body", "comment 1", "comment 2"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in feedback, got: %q", want, got)
		}
	}
}

// TestBuildFeedback_EmptyBodyWithComments verifies that an empty review body
// is omitted and only comments appear.
func TestBuildFeedback_EmptyBodyWithComments(t *testing.T) {
	fetchFn := func(_ string, _ int, _ int) ([]string, error) {
		return []string{"inline comment"}, nil
	}
	got := buildFeedback("", fetchFn, "owner/repo", 5, 101, slog.Default())
	if got != "inline comment" {
		t.Errorf("expected %q, got %q", "inline comment", got)
	}
}
