package rundata

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// newReaderWithBase creates a Reader that uses base as its root directory.
func newReaderWithBase(base string) *Reader {
	return &Reader{
		logger:  slog.Default(),
		baseDir: base,
	}
}

// makeRunDir creates a run directory and writes run.json. Returns the run dir path.
func makeRunDir(t *testing.T, base, owner, repo, timestamp string, meta RunMeta) string {
	t.Helper()
	runDir := filepath.Join(base, owner, repo, timestamp)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("creating run dir: %v", err)
	}
	if err := writeJSON(filepath.Join(runDir, "run.json"), meta); err != nil {
		t.Fatalf("writing run.json: %v", err)
	}
	return runDir
}

func TestListRunsReturnsSortedMostRecentFirst(t *testing.T) {
	base := t.TempDir()

	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)

	makeRunDir(t, base, "owner", "repo", "20260101-100000", RunMeta{Repo: "owner/repo", StartedAt: t1})
	makeRunDir(t, base, "owner", "repo", "20260103-100000", RunMeta{Repo: "owner/repo", StartedAt: t3})
	makeRunDir(t, base, "owner", "repo", "20260102-100000", RunMeta{Repo: "owner/repo", StartedAt: t2})

	r := newReaderWithBase(base)
	runs, err := r.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns() error: %v", err)
	}

	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}
	if !runs[0].StartedAt.Equal(t3) {
		t.Errorf("run[0] started_at: got %v, want %v", runs[0].StartedAt, t3)
	}
	if !runs[1].StartedAt.Equal(t2) {
		t.Errorf("run[1] started_at: got %v, want %v", runs[1].StartedAt, t2)
	}
	if !runs[2].StartedAt.Equal(t1) {
		t.Errorf("run[2] started_at: got %v, want %v", runs[2].StartedAt, t1)
	}
}

func TestListRunsEmptyDirReturnsEmptySlice(t *testing.T) {
	base := t.TempDir()
	r := newReaderWithBase(filepath.Join(base, "no-such-dir"))
	runs, err := r.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns() error: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
}

func TestListRunsSkipsCorruptRunJSON(t *testing.T) {
	base := t.TempDir()

	goodTime := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	makeRunDir(t, base, "owner", "repo", "20260201-000000", RunMeta{Repo: "owner/repo", StartedAt: goodTime})

	// Create a corrupt run.json for a second run
	corruptDir := filepath.Join(base, "owner", "repo", "20260101-000000")
	if err := os.MkdirAll(corruptDir, 0o755); err != nil {
		t.Fatalf("creating corrupt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(corruptDir, "run.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("writing corrupt run.json: %v", err)
	}

	r := newReaderWithBase(base)
	runs, err := r.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns() error: %v", err)
	}

	if len(runs) != 1 {
		t.Fatalf("expected 1 run (corrupt skipped), got %d", len(runs))
	}
	if !runs[0].StartedAt.Equal(goodTime) {
		t.Errorf("remaining run started_at: got %v, want %v", runs[0].StartedAt, goodTime)
	}
}

func TestLoadRunFullDetail(t *testing.T) {
	base := t.TempDir()
	startedAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	runDir := makeRunDir(t, base, "owner", "repo", "20260301-120000", RunMeta{
		Repo:         "owner/repo",
		Milestone:    "Phase 7",
		IssueNumbers: []int{42},
		StartedAt:    startedAt,
	})

	// Write issue 42 files
	issueDir := filepath.Join(runDir, "issues", "42")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "implement.json"), StepResult{Output: "impl output"}); err != nil {
		t.Fatalf("writing implement.json: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "quality-review.json"), StepResult{Output: "quality output"}); err != nil {
		t.Fatalf("writing quality-review.json: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "functional-review.json"), StepResult{Output: "functional output"}); err != nil {
		t.Fatalf("writing functional-review.json: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "outcome.json"), Outcome{IssueNumber: 42, Status: "implemented", PRNumber: 99}); err != nil {
		t.Fatalf("writing outcome.json: %v", err)
	}

	// Write retries
	retry1Dir := filepath.Join(issueDir, "retries", "1")
	if err := os.MkdirAll(retry1Dir, 0o755); err != nil {
		t.Fatalf("creating retry dir: %v", err)
	}
	if err := writeJSON(filepath.Join(retry1Dir, "retry.json"), StepResult{Output: "retry 1 output"}); err != nil {
		t.Fatalf("writing retry.json: %v", err)
	}
	if err := writeJSON(filepath.Join(retry1Dir, "quality-review.json"), StepResult{Output: "retry 1 review"}); err != nil {
		t.Fatalf("writing retry quality-review.json: %v", err)
	}

	r := newReaderWithBase(base)
	detail, err := r.LoadRun("owner", "repo", "20260301-120000")
	if err != nil {
		t.Fatalf("LoadRun() error: %v", err)
	}

	if detail.Repo != "owner/repo" {
		t.Errorf("repo: got %q, want %q", detail.Repo, "owner/repo")
	}
	if detail.Milestone != "Phase 7" {
		t.Errorf("milestone: got %q, want %q", detail.Milestone, "Phase 7")
	}
	if len(detail.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(detail.Issues))
	}

	issue := detail.Issues[0]
	if issue.IssueNumber != 42 {
		t.Errorf("issue number: got %d, want 42", issue.IssueNumber)
	}
	if issue.Implement.Output != "impl output" {
		t.Errorf("implement output: got %q, want %q", issue.Implement.Output, "impl output")
	}
	if issue.QualityReview.Output != "quality output" {
		t.Errorf("quality review output: got %q, want %q", issue.QualityReview.Output, "quality output")
	}
	if issue.FunctionalReview.Output != "functional output" {
		t.Errorf("functional review output: got %q, want %q", issue.FunctionalReview.Output, "functional output")
	}
	if issue.Outcome.Status != "implemented" {
		t.Errorf("outcome status: got %q, want %q", issue.Outcome.Status, "implemented")
	}
	if issue.Outcome.PRNumber != 99 {
		t.Errorf("outcome PR number: got %d, want 99", issue.Outcome.PRNumber)
	}
	if len(issue.Retries) != 1 {
		t.Fatalf("expected 1 retry, got %d", len(issue.Retries))
	}
	if issue.Retries[0].Attempt != 1 {
		t.Errorf("retry attempt: got %d, want 1", issue.Retries[0].Attempt)
	}
	if issue.Retries[0].Retry.Output != "retry 1 output" {
		t.Errorf("retry output: got %q, want %q", issue.Retries[0].Retry.Output, "retry 1 output")
	}
	if issue.Retries[0].QualityReview.Output != "retry 1 review" {
		t.Errorf("retry review output: got %q, want %q", issue.Retries[0].QualityReview.Output, "retry 1 review")
	}
}

func TestLoadRunMissingOutcomeUsesZeroValue(t *testing.T) {
	base := t.TempDir()
	runDir := makeRunDir(t, base, "owner", "repo", "20260301-120000", RunMeta{
		Repo:      "owner/repo",
		StartedAt: time.Now().UTC(),
	})

	issueDir := filepath.Join(runDir, "issues", "42")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	// outcome.json intentionally not written

	r := newReaderWithBase(base)
	detail, err := r.LoadRun("owner", "repo", "20260301-120000")
	if err != nil {
		t.Fatalf("LoadRun() error: %v", err)
	}

	if len(detail.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(detail.Issues))
	}
	issue := detail.Issues[0]
	if issue.IssueNumber != 42 {
		t.Errorf("issue number: got %d, want 42", issue.IssueNumber)
	}
	// Outcome should be zero value
	if issue.Outcome.IssueNumber != 0 || issue.Outcome.Status != "" || issue.Outcome.PRNumber != 0 {
		t.Errorf("expected zero-value outcome, got %+v", issue.Outcome)
	}
}

func TestLoadRunNoIssuesDir(t *testing.T) {
	base := t.TempDir()
	makeRunDir(t, base, "owner", "repo", "20260301-120000", RunMeta{
		Repo:      "owner/repo",
		StartedAt: time.Now().UTC(),
	})

	r := newReaderWithBase(base)
	detail, err := r.LoadRun("owner", "repo", "20260301-120000")
	if err != nil {
		t.Fatalf("LoadRun() error: %v", err)
	}
	if len(detail.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(detail.Issues))
	}
}

func TestLoadRunPlaceholderIssuesFromMeta(t *testing.T) {
	base := t.TempDir()
	makeRunDir(t, base, "owner", "repo", "20260301-120000", RunMeta{
		Repo:         "owner/repo",
		Milestone:    "Phase 7",
		IssueNumbers: []int{10, 20, 30},
		StartedAt:    time.Now().UTC(),
	})

	// No issues/ directory exists — simulates a run in progress before any
	// hook data has been written.

	r := newReaderWithBase(base)
	detail, err := r.LoadRun("owner", "repo", "20260301-120000")
	if err != nil {
		t.Fatalf("LoadRun() error: %v", err)
	}

	if len(detail.Issues) != 3 {
		t.Fatalf("expected 3 placeholder issues, got %d", len(detail.Issues))
	}

	nums := map[int]bool{}
	for _, issue := range detail.Issues {
		nums[issue.IssueNumber] = true
		if issue.Outcome.Status != "" {
			t.Errorf("issue #%d: expected zero-value status, got %q", issue.IssueNumber, issue.Outcome.Status)
		}
	}
	for _, want := range []int{10, 20, 30} {
		if !nums[want] {
			t.Errorf("missing placeholder issue #%d", want)
		}
	}
}

func TestLoadRunPlaceholderSkipsExistingIssueDirs(t *testing.T) {
	base := t.TempDir()
	runDir := makeRunDir(t, base, "owner", "repo", "20260301-120000", RunMeta{
		Repo:         "owner/repo",
		IssueNumbers: []int{10, 20},
		StartedAt:    time.Now().UTC(),
	})

	// Issue 10 has a directory with an outcome; issue 20 does not.
	issueDir := filepath.Join(runDir, "issues", "10")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "outcome.json"), Outcome{
		IssueNumber: 10, Status: "implemented", PRNumber: 55,
	}); err != nil {
		t.Fatalf("writing outcome.json: %v", err)
	}

	r := newReaderWithBase(base)
	detail, err := r.LoadRun("owner", "repo", "20260301-120000")
	if err != nil {
		t.Fatalf("LoadRun() error: %v", err)
	}

	if len(detail.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(detail.Issues))
	}

	issueMap := map[int]IssueDetail{}
	for _, issue := range detail.Issues {
		issueMap[issue.IssueNumber] = issue
	}

	if issueMap[10].Outcome.Status != "implemented" {
		t.Errorf("issue #10: expected status %q, got %q", "implemented", issueMap[10].Outcome.Status)
	}
	if issueMap[20].Outcome.Status != "" {
		t.Errorf("issue #20: expected zero-value status, got %q", issueMap[20].Outcome.Status)
	}
}

func TestLoadRunReturnsErrorForMissingRunJSON(t *testing.T) {
	base := t.TempDir()
	r := newReaderWithBase(base)
	_, err := r.LoadRun("owner", "repo", "no-such-timestamp")
	if err == nil {
		t.Fatal("expected error for missing run.json, got nil")
	}
}

func TestListRunsMultipleOwnerRepo(t *testing.T) {
	base := t.TempDir()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	makeRunDir(t, base, "ownerA", "repoX", "20260101-000000", RunMeta{Repo: "ownerA/repoX", StartedAt: t1})
	makeRunDir(t, base, "ownerB", "repoY", "20260102-000000", RunMeta{Repo: "ownerB/repoY", StartedAt: t2})

	r := newReaderWithBase(base)
	runs, err := r.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns() error: %v", err)
	}

	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	// Most recent first
	if runs[0].Repo != "ownerB/repoY" {
		t.Errorf("runs[0] repo: got %q, want %q", runs[0].Repo, "ownerB/repoY")
	}
	if runs[1].Repo != "ownerA/repoX" {
		t.Errorf("runs[1] repo: got %q, want %q", runs[1].Repo, "ownerA/repoX")
	}
}

func TestListRunsAcrossMultipleReposSameOwner(t *testing.T) {
	base := t.TempDir()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	makeRunDir(t, base, "owner", "repo-a", "20260101-000000", RunMeta{Repo: "owner/repo-a", StartedAt: t1})
	makeRunDir(t, base, "owner", "repo-b", "20260102-000000", RunMeta{Repo: "owner/repo-b", StartedAt: t2})

	r := newReaderWithBase(base)
	runs, err := r.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns() error: %v", err)
	}

	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	// Most recent first (interleaved by timestamp)
	if runs[0].Repo != "owner/repo-b" {
		t.Errorf("runs[0] repo: got %q, want %q", runs[0].Repo, "owner/repo-b")
	}
	if runs[1].Repo != "owner/repo-a" {
		t.Errorf("runs[1] repo: got %q, want %q", runs[1].Repo, "owner/repo-a")
	}
}

func TestLoadRunRetriesSortedByAttempt(t *testing.T) {
	base := t.TempDir()
	runDir := makeRunDir(t, base, "owner", "repo", "20260301-120000", RunMeta{
		Repo:      "owner/repo",
		StartedAt: time.Now().UTC(),
	})

	issueDir := filepath.Join(runDir, "issues", "10")
	for _, n := range []int{3, 1, 2} {
		retryDir := filepath.Join(issueDir, "retries", itoa(n))
		if err := os.MkdirAll(retryDir, 0o755); err != nil {
			t.Fatalf("creating retry dir: %v", err)
		}
	}

	r := newReaderWithBase(base)
	detail, err := r.LoadRun("owner", "repo", "20260301-120000")
	if err != nil {
		t.Fatalf("LoadRun() error: %v", err)
	}

	if len(detail.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(detail.Issues))
	}
	retries := detail.Issues[0].Retries
	if len(retries) != 3 {
		t.Fatalf("expected 3 retries, got %d", len(retries))
	}
	for i, want := range []int{1, 2, 3} {
		if retries[i].Attempt != want {
			t.Errorf("retries[%d].Attempt: got %d, want %d", i, retries[i].Attempt, want)
		}
	}
}

func TestReadDialogue(t *testing.T) {
	base := t.TempDir()
	runDir := makeRunDir(t, base, "owner", "repo", "20260301-120000", RunMeta{
		Repo:         "owner/repo",
		IssueNumbers: []int{42},
		StartedAt:    time.Now().UTC(),
	})

	issueDir := filepath.Join(runDir, "issues", "42")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}

	entries := []DialogueEntry{
		{Role: "implementer", Round: 1, Body: "impl body"},
		{Role: "reviewer", Round: 1, Body: "review body"},
	}
	if err := writeJSON(filepath.Join(issueDir, "dialogue.json"), entries); err != nil {
		t.Fatalf("writing dialogue.json: %v", err)
	}

	r := newReaderWithBase(base)
	detail, err := r.LoadRun("owner", "repo", "20260301-120000")
	if err != nil {
		t.Fatalf("LoadRun() error: %v", err)
	}

	if len(detail.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(detail.Issues))
	}

	d := detail.Issues[0].Dialogue
	if len(d) != 2 {
		t.Fatalf("Dialogue: got %d entries, want 2", len(d))
	}
	if d[0].Role != "implementer" || d[0].Round != 1 || d[0].Body != "impl body" {
		t.Errorf("Dialogue[0]: got {%s, %d, %q}, want {implementer, 1, impl body}", d[0].Role, d[0].Round, d[0].Body)
	}
	if d[1].Role != "reviewer" || d[1].Round != 1 || d[1].Body != "review body" {
		t.Errorf("Dialogue[1]: got {%s, %d, %q}, want {reviewer, 1, review body}", d[1].Role, d[1].Round, d[1].Body)
	}
}

func TestReadDialogueMissingFile(t *testing.T) {
	base := t.TempDir()
	runDir := makeRunDir(t, base, "owner", "repo", "20260301-120000", RunMeta{
		Repo:      "owner/repo",
		StartedAt: time.Now().UTC(),
	})

	issueDir := filepath.Join(runDir, "issues", "42")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	// dialogue.json intentionally not written

	r := newReaderWithBase(base)
	detail, err := r.LoadRun("owner", "repo", "20260301-120000")
	if err != nil {
		t.Fatalf("LoadRun() error: %v", err)
	}

	if len(detail.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(detail.Issues))
	}
	if detail.Issues[0].Dialogue != nil {
		t.Errorf("Dialogue: expected nil when file absent, got %v", detail.Issues[0].Dialogue)
	}
}

func TestReadDialogueRoundTrip(t *testing.T) {
	base := t.TempDir()
	t.Setenv("HOME", base)
	w, err := New("owner/repo", "Phase 7", []int{5})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	want := []DialogueEntry{
		{Role: "implementer", Round: 1, Body: "first impl"},
		{Role: "reviewer", Round: 1, Body: "first review"},
		{Role: "implementer", Round: 2, Body: "second impl"},
		{Role: "reviewer", Round: 2, Body: "second review"},
	}
	if err := w.WriteDialogue(5, want); err != nil {
		t.Fatalf("WriteDialogue() error: %v", err)
	}

	r := &Reader{logger: slog.Default(), baseDir: filepath.Join(base, ".godark", "runs")}
	runs, err := r.ListRuns()
	if err != nil || len(runs) != 1 {
		t.Fatalf("ListRuns() error: %v, len: %d", err, len(runs))
	}

	parts := strings.SplitN(runs[0].Repo, "/", 2)
	owner, repo := parts[0], parts[1]
	timestamp := filepath.Base(w.Dir())

	detail, err := r.LoadRun(owner, repo, timestamp)
	if err != nil {
		t.Fatalf("LoadRun() error: %v", err)
	}

	if len(detail.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(detail.Issues))
	}

	got := detail.Issues[0].Dialogue
	if len(got) != len(want) {
		t.Fatalf("Dialogue: got %d entries, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Role != w.Role || got[i].Round != w.Round || got[i].Body != w.Body {
			t.Errorf("Dialogue[%d]: got {%s, %d, %q}, want {%s, %d, %q}",
				i, got[i].Role, got[i].Round, got[i].Body,
				w.Role, w.Round, w.Body)
		}
	}
}

func TestLoadRunRetryFunctionalReview(t *testing.T) {
	base := t.TempDir()
	runDir := makeRunDir(t, base, "owner", "repo", "20260301-120000", RunMeta{
		Repo:      "owner/repo",
		StartedAt: time.Now().UTC(),
	})

	issueDir := filepath.Join(runDir, "issues", "10")
	retryDir := filepath.Join(issueDir, "retries", "0")
	if err := os.MkdirAll(retryDir, 0o755); err != nil {
		t.Fatalf("creating retry dir: %v", err)
	}
	if err := writeJSON(filepath.Join(retryDir, "retry.json"), StepResult{Output: "retry output"}); err != nil {
		t.Fatalf("writing retry.json: %v", err)
	}
	if err := writeJSON(filepath.Join(retryDir, "functional-review.json"), StepResult{Output: "pre-retry functional review"}); err != nil {
		t.Fatalf("writing functional-review.json: %v", err)
	}

	r := newReaderWithBase(base)
	detail, err := r.LoadRun("owner", "repo", "20260301-120000")
	if err != nil {
		t.Fatalf("LoadRun() error: %v", err)
	}

	if len(detail.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(detail.Issues))
	}
	retries := detail.Issues[0].Retries
	if len(retries) != 1 {
		t.Fatalf("expected 1 retry, got %d", len(retries))
	}
	if retries[0].FunctionalReview.Output != "pre-retry functional review" {
		t.Errorf("FunctionalReview.Output = %q, want %q", retries[0].FunctionalReview.Output, "pre-retry functional review")
	}
}

func TestLoadRunRetryFunctionalReviewAbsent(t *testing.T) {
	base := t.TempDir()
	runDir := makeRunDir(t, base, "owner", "repo", "20260301-120000", RunMeta{
		Repo:      "owner/repo",
		StartedAt: time.Now().UTC(),
	})

	issueDir := filepath.Join(runDir, "issues", "10")
	retryDir := filepath.Join(issueDir, "retries", "0")
	if err := os.MkdirAll(retryDir, 0o755); err != nil {
		t.Fatalf("creating retry dir: %v", err)
	}
	// No functional-review.json written in retry dir.
	if err := writeJSON(filepath.Join(retryDir, "retry.json"), StepResult{Output: "retry output"}); err != nil {
		t.Fatalf("writing retry.json: %v", err)
	}

	r := newReaderWithBase(base)
	detail, err := r.LoadRun("owner", "repo", "20260301-120000")
	if err != nil {
		t.Fatalf("LoadRun() error: %v", err)
	}

	retries := detail.Issues[0].Retries
	if len(retries) != 1 {
		t.Fatalf("expected 1 retry, got %d", len(retries))
	}
	// FunctionalReview should be zero value when file is absent.
	if retries[0].FunctionalReview.Output != "" {
		t.Errorf("FunctionalReview.Output = %q, want empty string when file absent", retries[0].FunctionalReview.Output)
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
