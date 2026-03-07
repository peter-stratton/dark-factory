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

func TestSpecGeneratorWritten(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{Output: "spec generator output"}
	if err := w.WriteSpecGeneratorResult(42, step); err != nil {
		t.Fatalf("WriteSpecGeneratorResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "spec-generator.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s, got: %v", path, err)
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

func TestRetryFunctionalReviewWritten(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{Output: "pre-retry functional review output"}
	if err := w.WriteRetryFunctionalReviewResult(42, 0, step); err != nil {
		t.Fatalf("WriteRetryFunctionalReviewResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "retries", "0", "functional-review.json")
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

func TestFlagsWrittenToReviewJSON(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	flags := []Flag{
		{Code: "low_cost", Message: "review cost $0.0001 is below threshold $0.1000"},
		{Code: "short_duration", Message: "review duration 1s is below threshold 30s"},
	}
	step := StepResult{Output: "review output", Flags: flags}
	if err := w.WriteReviewResult(42, "quality", step); err != nil {
		t.Fatalf("WriteReviewResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "quality-review.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading quality-review.json: %v", err)
	}

	var written StepResult
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("parsing quality-review.json: %v", err)
	}

	if len(written.Flags) != 2 {
		t.Fatalf("Flags: got %d, want 2", len(written.Flags))
	}
	if written.Flags[0].Code != "low_cost" {
		t.Errorf("Flags[0].Code = %q, want %q", written.Flags[0].Code, "low_cost")
	}
	if written.Flags[1].Code != "short_duration" {
		t.Errorf("Flags[1].Code = %q, want %q", written.Flags[1].Code, "short_duration")
	}
}

func TestToolTraceWrittenToJSON(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	trace := []string{"Read main.go", "Write tests/review/test.go", "go test ./..."}
	step := StepResult{Output: "review output", ToolTrace: trace}
	if err := w.WriteReviewResult(42, "functional", step); err != nil {
		t.Fatalf("WriteReviewResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "functional-review.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading functional-review.json: %v", err)
	}

	var written StepResult
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("parsing functional-review.json: %v", err)
	}

	if len(written.ToolTrace) != len(trace) {
		t.Fatalf("ToolTrace length: got %d, want %d", len(written.ToolTrace), len(trace))
	}
	for i, want := range trace {
		if written.ToolTrace[i] != want {
			t.Errorf("ToolTrace[%d] = %q, want %q", i, written.ToolTrace[i], want)
		}
	}
}

func TestToolTraceOmittedFromJSONWhenEmpty(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{Output: "review output"}
	if err := w.WriteReviewResult(42, "functional", step); err != nil {
		t.Fatalf("WriteReviewResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "functional-review.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading functional-review.json: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	if _, ok := raw["tool_trace"]; ok {
		t.Error("tool_trace key should be omitted from JSON when empty")
	}
}

func TestNoFlagsOmittedFromJSON(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{Output: "review output"}
	if err := w.WriteReviewResult(42, "functional", step); err != nil {
		t.Fatalf("WriteReviewResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "functional-review.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading functional-review.json: %v", err)
	}

	// flags field should be omitted (omitempty) when empty
	if string(data) != "" {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("parsing JSON: %v", err)
		}
		if _, ok := raw["flags"]; ok {
			t.Error("flags key should be omitted from JSON when empty")
		}
	}
}

func TestWriteDialogue(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entries := []DialogueEntry{
		{Role: "implementer", Round: 1, Body: "## Implementation Notes\nApproach..."},
		{Role: "reviewer", Round: 1, Body: "## Review Notes\nApproved..."},
	}
	if err := w.WriteDialogue(42, entries); err != nil {
		t.Fatalf("WriteDialogue() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "dialogue.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading dialogue.json: %v", err)
	}

	var written []DialogueEntry
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("parsing dialogue.json: %v", err)
	}

	if len(written) != 2 {
		t.Fatalf("entries: got %d, want 2", len(written))
	}
	if written[0].Role != "implementer" || written[0].Round != 1 {
		t.Errorf("entry[0]: got {%s, %d}, want {implementer, 1}", written[0].Role, written[0].Round)
	}
	if written[1].Role != "reviewer" || written[1].Round != 1 {
		t.Errorf("entry[1]: got {%s, %d}, want {reviewer, 1}", written[1].Role, written[1].Round)
	}
}

func TestWriteDialogueMultipleRounds(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{7})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entries := []DialogueEntry{
		{Role: "implementer", Round: 1, Body: "round 1 impl"},
		{Role: "reviewer", Round: 1, Body: "round 1 review"},
		{Role: "implementer", Round: 2, Body: "round 2 impl"},
		{Role: "reviewer", Round: 2, Body: "round 2 review"},
	}
	if err := w.WriteDialogue(7, entries); err != nil {
		t.Fatalf("WriteDialogue() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "7", "dialogue.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading dialogue.json: %v", err)
	}

	var written []DialogueEntry
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("parsing dialogue.json: %v", err)
	}

	if len(written) != 4 {
		t.Fatalf("entries: got %d, want 4", len(written))
	}
	for i, want := range entries {
		if written[i].Role != want.Role || written[i].Round != want.Round || written[i].Body != want.Body {
			t.Errorf("entry[%d]: got {%s, %d, %q}, want {%s, %d, %q}",
				i, written[i].Role, written[i].Round, written[i].Body,
				want.Role, want.Round, want.Body)
		}
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
