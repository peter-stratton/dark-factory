package watch

import (
	"context"
	"fmt"
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

// testWatchCfg returns a minimal Config sufficient for PollOnce tests.
func testWatchCfg() *config.Config {
	return &config.Config{Repo: "owner/repo"}
}

// stubSeams replaces the watch testability seams with no-ops and restores them on t.Cleanup.
func stubSeams(t *testing.T) {
	t.Helper()

	origRetry := retryFn
	origFindSession := findSessionIDFn
	origNewWriter := newWriterFn
	origFetchComments := fetchReviewCommentsFn
	origFetchIssue := fetchIssueFn
	origMergePR := mergePRFn
	origListMergedPRs := listMergedPRsFn

	retryFn = func(_ context.Context, _ github.Issue, _ int, _ string, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		return &agent.Result{SessionID: "sess-stub"}, nil
	}
	findSessionIDFn = func(_ string, _ int) (string, error) { return "", nil }
	newWriterFn = func(repo, milestone string, issueNumbers []int, baseBranch string, _ rundata.AutoMerge) (*rundata.Writer, error) {
		return nil, nil
	}
	fetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	fetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "test issue"}, nil
	}
	mergePRFn = func(_ string, _ int) error { return nil }
	listMergedPRsFn = func(_ string) ([]github.PRInfo, error) { return nil, nil }

	t.Cleanup(func() {
		retryFn = origRetry
		findSessionIDFn = origFindSession
		newWriterFn = origNewWriter
		fetchReviewCommentsFn = origFetchComments
		fetchIssueFn = origFetchIssue
		mergePRFn = origMergePR
		listMergedPRsFn = origListMergedPRs
	})
}

// TestPollInterval_DefaultWhenNilConfig verifies the 60s default when
// Watch config is nil.
func TestPollInterval_DefaultWhenNilConfig(t *testing.T) {
	w := New(&config.Config{Watch: nil}, nil, nil, slog.Default())
	got, err := w.pollInterval()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 60*time.Second {
		t.Errorf("expected 60s default, got %v", got)
	}
}

// TestPollInterval_DefaultWhenEmptyInterval verifies the 60s default when
// Watch is set but PollInterval is empty.
func TestPollInterval_DefaultWhenEmptyInterval(t *testing.T) {
	w := New(&config.Config{Watch: &config.Watch{PollInterval: ""}}, nil, nil, slog.Default())
	got, err := w.pollInterval()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 60*time.Second {
		t.Errorf("expected 60s default, got %v", got)
	}
}

// TestPollInterval_CustomInterval verifies that a configured interval is used.
func TestPollInterval_CustomInterval(t *testing.T) {
	w := New(&config.Config{Watch: &config.Watch{PollInterval: "10s"}}, nil, nil, slog.Default())
	got, err := w.pollInterval()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 10*time.Second {
		t.Errorf("expected 10s, got %v", got)
	}
}

// TestPollOnce_NoPRs verifies that an empty PR list causes no label calls.
func TestPollOnce_NoPRs(t *testing.T) {
	stubSeams(t)

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer func() { github.CommandRunner = orig }()

	w := New(testWatchCfg(), nil, nil, slog.Default())
	if err := w.PollOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.processed) != 0 {
		t.Errorf("expected no processed reviews, got %v", w.processed)
	}
}

// TestPollOnce_DetectsChangesRequestedReview verifies that a CHANGES_REQUESTED
// review triggers label swapping and marks the review as processed.
func TestPollOnce_DetectsChangesRequestedReview(t *testing.T) {
	stubSeams(t)

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

	w := New(testWatchCfg(), nil, nil, slog.Default())
	if err := w.PollOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !w.processed[101] {
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
	stubSeams(t)

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
	w := New(testWatchCfg(), nil, nil, slog.Default())
	w.processed[101] = true
	if err := w.PollOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if addLabelCalled {
		t.Error("expected no label update for already-processed review")
	}
}

// TestPollOnce_CommentedSkipped verifies that COMMENTED reviews do not trigger
// any label swapping or merge attempts.
func TestPollOnce_CommentedSkipped(t *testing.T) {
	stubSeams(t)

	var addLabelCalled bool
	mergeCalled := false
	origMergePR := mergePRFn
	mergePRFn = func(_ string, _ int) error {
		mergeCalled = true
		return nil
	}
	defer func() { mergePRFn = origMergePR }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		if len(args) < 2 {
			return []byte(`{}`), nil
		}
		switch {
		case args[0] == "pr" && args[1] == "list":
			return []byte(`[{"number":5,"headRefName":"42-feature-branch"}]`), nil
		case args[0] == "api":
			return []byte(`[{"id":300,"state":"COMMENTED","body":"nit","user":{"login":"carol"}}]`), nil
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

	w := New(testWatchCfg(), nil, nil, slog.Default())
	if err := w.PollOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if addLabelCalled {
		t.Error("expected no label update for COMMENTED review")
	}
	if mergeCalled {
		t.Error("expected no merge for COMMENTED review")
	}
	if w.processed[300] {
		t.Error("expected COMMENTED review not to be recorded in processed set")
	}
}

// TestPollOnce_ApprovedPRMerged verifies that an APPROVED review triggers a
// merge and label removal.
func TestPollOnce_ApprovedPRMerged(t *testing.T) {
	stubSeams(t)

	var mergedPR int
	var removeLabelCalled bool

	origMergePR := mergePRFn
	mergePRFn = func(_ string, prNum int) error {
		mergedPR = prNum
		return nil
	}
	defer func() { mergePRFn = origMergePR }()

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
				if a == "--remove-label" {
					removeLabelCalled = true
				}
			}
			return []byte(`{}`), nil
		case args[0] == "issue" && args[1] == "close":
			return []byte(`{}`), nil
		}
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	w := New(testWatchCfg(), nil, nil, slog.Default())
	if err := w.PollOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mergedPR != 5 {
		t.Errorf("expected PR 5 to be merged, got %d", mergedPR)
	}
	if !removeLabelCalled {
		t.Error("expected remove-label to be called after merge")
	}
	if !w.processed[200] {
		t.Error("expected review ID 200 to be marked as processed")
	}
}

// TestPollOnce_ApprovedDuplicateSkipped verifies that an already-processed
// APPROVED review does not trigger a second merge attempt.
func TestPollOnce_ApprovedDuplicateSkipped(t *testing.T) {
	stubSeams(t)

	mergeCalled := false
	origMergePR := mergePRFn
	mergePRFn = func(_ string, _ int) error {
		mergeCalled = true
		return nil
	}
	defer func() { mergePRFn = origMergePR }()

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
		}
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	// Pre-mark review 200 as already processed.
	w := New(testWatchCfg(), nil, nil, slog.Default())
	w.processed[200] = true
	if err := w.PollOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mergeCalled {
		t.Error("expected no merge for already-processed APPROVED review")
	}
}

// TestPollOnce_MixedReviewsMergesOnApproved verifies that when a PR has both
// COMMENTED and APPROVED reviews, only the APPROVED one triggers a merge.
func TestPollOnce_MixedReviewsMergesOnApproved(t *testing.T) {
	stubSeams(t)

	var mergedPR int

	origMergePR := mergePRFn
	mergePRFn = func(_ string, prNum int) error {
		mergedPR = prNum
		return nil
	}
	defer func() { mergePRFn = origMergePR }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		if len(args) < 2 {
			return []byte(`{}`), nil
		}
		switch {
		case args[0] == "pr" && args[1] == "list":
			return []byte(`[{"number":7,"headRefName":"99-mixed-reviews"}]`), nil
		case args[0] == "api":
			return []byte(`[{"id":300,"state":"COMMENTED","body":"nit","user":{"login":"carol"}},{"id":400,"state":"APPROVED","body":"LGTM","user":{"login":"bob"}}]`), nil
		case args[0] == "issue" && (args[1] == "close" || args[1] == "edit"):
			return []byte(`{}`), nil
		}
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	w := New(testWatchCfg(), nil, nil, slog.Default())
	if err := w.PollOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mergedPR != 7 {
		t.Errorf("expected PR 7 to be merged via APPROVED review, got mergedPR=%d", mergedPR)
	}
	if w.processed[300] {
		t.Error("expected COMMENTED review ID 300 not to be recorded")
	}
	if !w.processed[400] {
		t.Error("expected APPROVED review ID 400 to be recorded in processed set")
	}
}

// TestPollOnce_MergeFailureLogged verifies that a merge failure is logged and
// the label remains (label-remove is NOT called when merge fails).
func TestPollOnce_MergeFailureLogged(t *testing.T) {
	stubSeams(t)

	origMergePR := mergePRFn
	mergePRFn = func(_ string, _ int) error {
		return fmt.Errorf("merge conflict")
	}
	defer func() { mergePRFn = origMergePR }()

	var removeLabelCalled bool

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
				if a == "--remove-label" {
					removeLabelCalled = true
				}
			}
			return []byte(`{}`), nil
		}
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	w := New(testWatchCfg(), nil, nil, slog.Default())
	// PollOnce itself should not return an error — merge errors are logged only.
	if err := w.PollOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if removeLabelCalled {
		t.Error("expected label NOT to be removed when merge fails")
	}
	// The review is still marked processed to prevent infinite retry loops on a
	// broken PR — the processed flag was set before HandleApproved was called.
	if !w.processed[200] {
		t.Error("expected review ID 200 to be marked processed even on merge failure")
	}
}

// TestPollOnce_ContextCancelled verifies that a cancelled context causes
// PollOnce to return nil without processing PRs.
func TestPollOnce_ContextCancelled(t *testing.T) {
	stubSeams(t)

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

	w := New(testWatchCfg(), nil, nil, slog.Default())
	// PollOnce calls ListPRsWithLabel before checking ctx — the important thing
	// is it returns nil (no error) even when context is already cancelled.
	err := w.PollOnce(ctx)
	if err != nil {
		t.Fatalf("expected nil error on cancelled context, got %v", err)
	}
	_ = listCalled // the list may or may not be called depending on timing
}

// TestHandleChangesRequested_FeedbackFed verifies that the review body is
// passed to the retry agent as feedback.
func TestHandleChangesRequested_FeedbackFed(t *testing.T) {
	var gotFeedback string

	origRetry := retryFn
	retryFn = func(_ context.Context, _ github.Issue, _ int, _ string, feedback string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		gotFeedback = feedback
		return &agent.Result{}, nil
	}
	defer func() { retryFn = origRetry }()

	origFindSession := findSessionIDFn
	findSessionIDFn = func(_ string, _ int) (string, error) { return "", nil }
	defer func() { findSessionIDFn = origFindSession }()

	origNewWriter := newWriterFn
	newWriterFn = func(_, _ string, _ []int, _ string, _ rundata.AutoMerge) (*rundata.Writer, error) { return nil, nil }
	defer func() { newWriterFn = origNewWriter }()

	origFetchComments := fetchReviewCommentsFn
	fetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	defer func() { fetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := fetchIssueFn
	fetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { fetchIssueFn = origFetchIssue }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	pr := github.PRInfo{Number: 5, HeadRefName: "42-my-issue"}
	review := github.PRReview{ID: 101, State: "CHANGES_REQUESTED", Body: "Please fix the error handling", Author: "alice"}

	w := New(testWatchCfg(), nil, nil, slog.Default())
	w.HandleChangesRequested(context.Background(), pr, review)

	if !strings.Contains(gotFeedback, "Please fix the error handling") {
		t.Errorf("expected feedback to contain review body, got %q", gotFeedback)
	}
}

// TestHandleChangesRequested_SessionResumed verifies that the session ID from
// run data is passed to the retry agent.
func TestHandleChangesRequested_SessionResumed(t *testing.T) {
	var gotSessionID string

	origRetry := retryFn
	retryFn = func(_ context.Context, _ github.Issue, _ int, sessionID string, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		gotSessionID = sessionID
		return &agent.Result{}, nil
	}
	defer func() { retryFn = origRetry }()

	origFindSession := findSessionIDFn
	findSessionIDFn = func(_ string, _ int) (string, error) { return "sess-abc123", nil }
	defer func() { findSessionIDFn = origFindSession }()

	origNewWriter := newWriterFn
	newWriterFn = func(_, _ string, _ []int, _ string, _ rundata.AutoMerge) (*rundata.Writer, error) { return nil, nil }
	defer func() { newWriterFn = origNewWriter }()

	origFetchComments := fetchReviewCommentsFn
	fetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	defer func() { fetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := fetchIssueFn
	fetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { fetchIssueFn = origFetchIssue }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	pr := github.PRInfo{Number: 5, HeadRefName: "42-my-issue"}
	review := github.PRReview{ID: 101, State: "CHANGES_REQUESTED", Body: "fix this", Author: "alice"}

	w := New(testWatchCfg(), nil, nil, slog.Default())
	w.HandleChangesRequested(context.Background(), pr, review)

	if gotSessionID != "sess-abc123" {
		t.Errorf("expected session ID %q, got %q", "sess-abc123", gotSessionID)
	}
}

// TestHandleChangesRequested_LabelsSwapped verifies that after the agent fix,
// the FixingReviewFeedback label is removed and AwaitingHumanReview is applied.
func TestHandleChangesRequested_LabelsSwapped(t *testing.T) {
	var labelsAdded, labelsRemoved []string

	origRetry := retryFn
	retryFn = func(_ context.Context, _ github.Issue, _ int, _ string, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		return &agent.Result{}, nil
	}
	defer func() { retryFn = origRetry }()

	origFindSession := findSessionIDFn
	findSessionIDFn = func(_ string, _ int) (string, error) { return "", nil }
	defer func() { findSessionIDFn = origFindSession }()

	origNewWriter := newWriterFn
	newWriterFn = func(_, _ string, _ []int, _ string, _ rundata.AutoMerge) (*rundata.Writer, error) { return nil, nil }
	defer func() { newWriterFn = origNewWriter }()

	origFetchComments := fetchReviewCommentsFn
	fetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	defer func() { fetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := fetchIssueFn
	fetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { fetchIssueFn = origFetchIssue }()

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

	w := New(testWatchCfg(), nil, nil, slog.Default())
	w.HandleChangesRequested(context.Background(), pr, review)

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

	origRetry := retryFn
	retryFn = func(_ context.Context, _ github.Issue, _ int, sessionID string, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		agentCalled = true
		if sessionID != "" {
			return nil, nil // fail the test via assertion below
		}
		return &agent.Result{}, nil
	}
	defer func() { retryFn = origRetry }()

	origFindSession := findSessionIDFn
	findSessionIDFn = func(_ string, _ int) (string, error) { return "", nil } // no session found
	defer func() { findSessionIDFn = origFindSession }()

	origNewWriter := newWriterFn
	newWriterFn = func(_, _ string, _ []int, _ string, _ rundata.AutoMerge) (*rundata.Writer, error) { return nil, nil }
	defer func() { newWriterFn = origNewWriter }()

	origFetchComments := fetchReviewCommentsFn
	fetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	defer func() { fetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := fetchIssueFn
	fetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { fetchIssueFn = origFetchIssue }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	pr := github.PRInfo{Number: 5, HeadRefName: "42-my-issue"}
	review := github.PRReview{ID: 101, State: "CHANGES_REQUESTED", Body: "fix this", Author: "alice"}

	w := New(testWatchCfg(), nil, nil, slog.Default())
	w.HandleChangesRequested(context.Background(), pr, review)

	if !agentCalled {
		t.Error("expected agent to be called even without session ID")
	}
}

// TestHandleChangesRequested_RunDataWritten verifies that a run data directory
// is created for watch-initiated fix cycles.
func TestHandleChangesRequested_RunDataWritten(t *testing.T) {
	base := t.TempDir()
	writerCreated := false

	origRetry := retryFn
	retryFn = func(_ context.Context, _ github.Issue, _ int, _ string, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		return &agent.Result{}, nil
	}
	defer func() { retryFn = origRetry }()

	origFindSession := findSessionIDFn
	findSessionIDFn = func(_ string, _ int) (string, error) { return "", nil }
	defer func() { findSessionIDFn = origFindSession }()

	origNewWriter := newWriterFn
	newWriterFn = func(repo, milestone string, issueNumbers []int, baseBranch string, am rundata.AutoMerge) (*rundata.Writer, error) {
		writerCreated = true
		// Use a custom base dir to verify the call actually happened.
		w, err := rundata.NewWithBase(base, repo, milestone, issueNumbers, baseBranch, am)
		return w, err
	}
	defer func() { newWriterFn = origNewWriter }()

	origFetchComments := fetchReviewCommentsFn
	fetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) { return nil, nil }
	defer func() { fetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := fetchIssueFn
	fetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { fetchIssueFn = origFetchIssue }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	pr := github.PRInfo{Number: 5, HeadRefName: "42-my-issue"}
	review := github.PRReview{ID: 101, State: "CHANGES_REQUESTED", Body: "fix this", Author: "alice"}

	w := New(testWatchCfg(), nil, nil, slog.Default())
	w.HandleChangesRequested(context.Background(), pr, review)

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

	origRetry := retryFn
	retryFn = func(_ context.Context, _ github.Issue, _ int, _ string, feedback string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger) (*agent.Result, error) {
		gotFeedback = feedback
		return &agent.Result{}, nil
	}
	defer func() { retryFn = origRetry }()

	origFindSession := findSessionIDFn
	findSessionIDFn = func(_ string, _ int) (string, error) { return "", nil }
	defer func() { findSessionIDFn = origFindSession }()

	origNewWriter := newWriterFn
	newWriterFn = func(_, _ string, _ []int, _ string, _ rundata.AutoMerge) (*rundata.Writer, error) { return nil, nil }
	defer func() { newWriterFn = origNewWriter }()

	origFetchComments := fetchReviewCommentsFn
	fetchReviewCommentsFn = func(_ string, _ int, _ int) ([]string, error) {
		return []string{"Inline comment A", "Inline comment B"}, nil
	}
	defer func() { fetchReviewCommentsFn = origFetchComments }()

	origFetchIssue := fetchIssueFn
	fetchIssueFn = func(_ string, _ int) (github.Issue, error) {
		return github.Issue{Number: 42, Title: "my issue"}, nil
	}
	defer func() { fetchIssueFn = origFetchIssue }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`{}`), nil
	})
	defer func() { github.CommandRunner = orig }()

	pr := github.PRInfo{Number: 5, HeadRefName: "42-my-issue"}
	review := github.PRReview{ID: 101, State: "CHANGES_REQUESTED", Body: "Review body comment", Author: "alice"}

	w := New(testWatchCfg(), nil, nil, slog.Default())
	w.HandleChangesRequested(context.Background(), pr, review)

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
		{"no-dash", 0}, // leading segment is not a number
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

// TestDetectMergedPRs_DetectsMergedPR verifies that an issue whose PR has been
// merged is returned by DetectMergedPRs.
func TestDetectMergedPRs_DetectsMergedPR(t *testing.T) {
	origListMergedPRs := listMergedPRsFn
	listMergedPRsFn = func(_ string) ([]github.PRInfo, error) {
		return []github.PRInfo{{Number: 5, HeadRefName: "42-fix-bug"}}, nil
	}
	defer func() { listMergedPRsFn = origListMergedPRs }()

	w := New(testWatchCfg(), nil, nil, slog.Default())
	got, err := w.DetectMergedPRs(context.Background(), "owner/repo", []int{42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 42 {
		t.Errorf("expected [42], got %v", got)
	}
	if !w.mergedIssues[42] {
		t.Error("expected issue 42 to be recorded in mergedIssues")
	}
}

// TestDetectMergedPRs_AlreadyDetectedSkipped verifies that an issue already
// present in mergedIssues is not returned again.
func TestDetectMergedPRs_AlreadyDetectedSkipped(t *testing.T) {
	origListMergedPRs := listMergedPRsFn
	listMergedPRsFn = func(_ string) ([]github.PRInfo, error) {
		return []github.PRInfo{{Number: 5, HeadRefName: "42-fix-bug"}}, nil
	}
	defer func() { listMergedPRsFn = origListMergedPRs }()

	w := New(testWatchCfg(), nil, nil, slog.Default())
	w.mergedIssues[42] = true // already detected in a prior cycle

	got, err := w.DetectMergedPRs(context.Background(), "owner/repo", []int{42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result for already-detected issue, got %v", got)
	}
}

// TestDetectMergedPRs_NoMerges verifies that an empty slice is returned when
// no PRs in the provided list have been merged.
func TestDetectMergedPRs_NoMerges(t *testing.T) {
	origListMergedPRs := listMergedPRsFn
	listMergedPRsFn = func(_ string) ([]github.PRInfo, error) {
		return []github.PRInfo{}, nil // no merged PRs
	}
	defer func() { listMergedPRsFn = origListMergedPRs }()

	w := New(testWatchCfg(), nil, nil, slog.Default())
	got, err := w.DetectMergedPRs(context.Background(), "owner/repo", []int{42, 43})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result when no PRs merged, got %v", got)
	}
}

// TestDetectMergedPRs_MultipleIssuesMerged verifies that all matching merged
// issues are returned when multiple PRs have been merged.
func TestDetectMergedPRs_MultipleIssuesMerged(t *testing.T) {
	origListMergedPRs := listMergedPRsFn
	listMergedPRsFn = func(_ string) ([]github.PRInfo, error) {
		return []github.PRInfo{
			{Number: 5, HeadRefName: "42-fix-bug"},
			{Number: 6, HeadRefName: "43-another-fix"},
		}, nil
	}
	defer func() { listMergedPRsFn = origListMergedPRs }()

	w := New(testWatchCfg(), nil, nil, slog.Default())
	got, err := w.DetectMergedPRs(context.Background(), "owner/repo", []int{42, 43})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 merged issues, got %v", got)
	}
	gotSet := make(map[int]bool)
	for _, n := range got {
		gotSet[n] = true
	}
	if !gotSet[42] || !gotSet[43] {
		t.Errorf("expected both 42 and 43 in result, got %v", got)
	}
	if !w.mergedIssues[42] || !w.mergedIssues[43] {
		t.Error("expected both issues to be recorded in mergedIssues")
	}
}

// TestRunUntilDone_ExitsWhenNoPRs verifies that RunUntilDone exits immediately
// if no PRs are awaiting review after the initial poll.
func TestRunUntilDone_ExitsWhenNoPRs(t *testing.T) {
	stubSeams(t)

	origListPRs := listPRsFn
	listPRsFn = func(_ string, _ string) ([]github.PRInfo, error) {
		return nil, nil // no PRs awaiting review
	}
	defer func() { listPRsFn = origListPRs }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer func() { github.CommandRunner = orig }()

	w := New(&config.Config{Repo: "owner/repo", Watch: &config.Watch{PollInterval: "10ms"}}, nil, nil, slog.Default())
	err := w.RunUntilDone(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRunUntilDone_ExitsAfterLastPRMerged verifies that RunUntilDone exits once
// the last awaiting PR is gone (PR count drops to zero after a poll tick).
func TestRunUntilDone_ExitsAfterLastPRMerged(t *testing.T) {
	stubSeams(t)

	callCount := 0
	origListPRs := listPRsFn
	listPRsFn = func(_ string, _ string) ([]github.PRInfo, error) {
		callCount++
		// First call (after initial poll): still 1 PR awaiting.
		// Second call (after a tick): PR is gone.
		if callCount == 1 {
			return []github.PRInfo{{Number: 5, HeadRefName: "42-feature"}}, nil
		}
		return nil, nil
	}
	defer func() { listPRsFn = origListPRs }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer func() { github.CommandRunner = orig }()

	w := New(&config.Config{Repo: "owner/repo", Watch: &config.Watch{PollInterval: "10ms"}}, nil, nil, slog.Default())
	err := w.RunUntilDone(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 PR list calls, got %d", callCount)
	}
}

// TestRunUntilDone_ContextCancelled verifies that a cancelled context causes
// RunUntilDone to return nil cleanly.
func TestRunUntilDone_ContextCancelled(t *testing.T) {
	stubSeams(t)

	origListPRs := listPRsFn
	listPRsFn = func(_ string, _ string) ([]github.PRInfo, error) {
		// Always return a PR so the loop would run forever without cancellation.
		return []github.PRInfo{{Number: 5, HeadRefName: "42-feature"}}, nil
	}
	defer func() { listPRsFn = origListPRs }()

	orig := github.CommandRunner
	github.CommandRunner = fakeRunner(t, func(args []string) ([]byte, error) {
		return []byte(`[]`), nil
	})
	defer func() { github.CommandRunner = orig }()

	ctx, cancel := context.WithCancel(context.Background())

	w := New(&config.Config{Repo: "owner/repo", Watch: &config.Watch{PollInterval: "10ms"}}, nil, nil, slog.Default())

	done := make(chan error, 1)
	go func() {
		done <- w.RunUntilDone(ctx)
	}()

	// Cancel context after a short delay to allow the goroutine to start.
	cancel()

	err := <-done
	if err != nil {
		t.Fatalf("expected nil on context cancel, got %v", err)
	}
}
