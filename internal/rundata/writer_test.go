package rundata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// newWithBase creates a Writer using a temp directory as the base instead of ~/.godark.
// It temporarily overrides os.UserHomeDir by swapping the home dir via env.
func newWithBase(t *testing.T, base, repo, milestone string, issueNumbers []int) (*Writer, error) {
	t.Helper()
	t.Setenv("HOME", base)
	return New(repo, milestone, issueNumbers)
}

func TestRunDirectoryCreated(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{1, 2})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	info, err := os.Stat(w.Dir())
	if err != nil {
		t.Fatalf("run directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory, got file")
	}

	// Verify the directory is under ~/.godark/runs/owner/repo/
	wantPrefix := filepath.Join(base, ".godark", "runs", "owner", "repo")
	if len(w.Dir()) <= len(wantPrefix) || w.Dir()[:len(wantPrefix)] != wantPrefix {
		t.Errorf("dir %q does not start with expected prefix %q", w.Dir(), wantPrefix)
	}
}

func TestTimestampFormat(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{1})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// The last path segment must match YYYYMMDD-HHMMSS
	segment := filepath.Base(w.Dir())
	matched, err := regexp.MatchString(`^\d{8}-\d{6}$`, segment)
	if err != nil {
		t.Fatalf("regexp error: %v", err)
	}
	if !matched {
		t.Errorf("timestamp segment %q does not match YYYYMMDD-HHMMSS", segment)
	}
}

func TestRunJSONAtStart(t *testing.T) {
	base := t.TempDir()
	before := time.Now().UTC().Truncate(time.Second)
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42, 43})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(w.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var meta RunMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}

	if meta.Repo != "owner/repo" {
		t.Errorf("repo: got %q, want %q", meta.Repo, "owner/repo")
	}
	if meta.Milestone != "Phase 7" {
		t.Errorf("milestone: got %q, want %q", meta.Milestone, "Phase 7")
	}
	if len(meta.IssueNumbers) != 2 || meta.IssueNumbers[0] != 42 || meta.IssueNumbers[1] != 43 {
		t.Errorf("issue_numbers: got %v, want [42 43]", meta.IssueNumbers)
	}
	if meta.StartedAt.Before(before) {
		t.Errorf("started_at %v is before test start %v", meta.StartedAt, before)
	}
	if meta.FinishedAt != nil {
		t.Errorf("finished_at should be nil at start, got %v", meta.FinishedAt)
	}
}

func TestRunJSONFinalized(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{1})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	summary := RunSummary{Total: 2, Implemented: 1, Failed: 1}
	beforeFinalize := time.Now().UTC().Truncate(time.Second)
	if err := w.FinalizeRun(summary); err != nil {
		t.Fatalf("FinalizeRun() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(w.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var meta RunMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}

	if meta.FinishedAt == nil {
		t.Fatal("finished_at should be set after FinalizeRun")
	}
	if meta.FinishedAt.Before(beforeFinalize) {
		t.Errorf("finished_at %v is before finalize call %v", meta.FinishedAt, beforeFinalize)
	}
	if meta.Summary == nil {
		t.Fatal("summary should be set after FinalizeRun")
	}
	if meta.Summary.Total != 2 {
		t.Errorf("summary.total: got %d, want 2", meta.Summary.Total)
	}
	if meta.Summary.Implemented != 1 {
		t.Errorf("summary.implemented: got %d, want 1", meta.Summary.Implemented)
	}
	if meta.Summary.Failed != 1 {
		t.Errorf("summary.failed: got %d, want 1", meta.Summary.Failed)
	}
}

func TestImplementWritten(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{Output: "implement output"}
	if err := w.WriteImplementResult(42, step); err != nil {
		t.Fatalf("WriteImplementResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "implement.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s, got: %v", path, err)
	}
}

func TestReviewWritten(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{Output: "review output"}
	if err := w.WriteReviewResult(42, "quality", step); err != nil {
		t.Fatalf("WriteReviewResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "quality-review.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s, got: %v", path, err)
	}
}

func TestFunctionalReviewWritten(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{Output: "functional review output"}
	if err := w.WriteReviewResult(42, "functional", step); err != nil {
		t.Fatalf("WriteReviewResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "functional-review.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s, got: %v", path, err)
	}
}

func TestReviewKindValidated(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{Output: "output"}
	err = w.WriteReviewResult(42, "bad", step)
	if err == nil {
		t.Fatal("expected error for invalid kind, got nil")
	}
}

func TestRetryWritten(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{Output: "retry output"}
	if err := w.WriteRetryResult(42, 1, step); err != nil {
		t.Fatalf("WriteRetryResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "retries", "1", "retry.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s, got: %v", path, err)
	}
}

func TestRetryReviewWritten(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{Output: "retry review output"}
	if err := w.WriteRetryReviewResult(42, 2, step); err != nil {
		t.Fatalf("WriteRetryReviewResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "retries", "2", "quality-review.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s, got: %v", path, err)
	}
}

func TestOutcomeWritten(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	outcome := Outcome{IssueNumber: 42, Status: "implemented", PRNumber: 57}
	if err := w.WriteOutcome(outcome); err != nil {
		t.Fatalf("WriteOutcome() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "outcome.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s, got: %v", path, err)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	cases := []string{
		"../evil/../../path",
		"../evil",
		"evil/..",
		"evil/../../path",
	}

	for _, repo := range cases {
		base := t.TempDir()
		t.Setenv("HOME", base)
		_, err := New(repo, "Phase 7", []int{1})
		if err == nil {
			t.Errorf("New(%q) should return error for path traversal, got nil", repo)
		}
	}
}
