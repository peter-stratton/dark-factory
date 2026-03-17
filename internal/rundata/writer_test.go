package rundata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// newWithBase creates a Writer using a temp directory as the base instead of ~/.godark.
// It temporarily overrides os.UserHomeDir by swapping the home dir via env.
func newWithBase(t *testing.T, base, repo, milestone string, issueNumbers []int) (*Writer, error) {
	t.Helper()
	t.Setenv("HOME", base)
	return New(repo, milestone, issueNumbers, "", AutoMerge{})
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

func TestReconResultWritten(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{
		Output:          "recon brief text",
		SessionID:       "sess-abc",
		CostUSD:         0.0012,
		DurationSeconds: 15.0,
	}
	if err := w.WriteReconResult(42, step); err != nil {
		t.Fatalf("WriteReconResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "recon.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file at %s, got: %v", path, err)
	}

	var written StepResult
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("parsing recon.json: %v", err)
	}

	if written.Output != "recon brief text" {
		t.Errorf("Output = %q, want %q", written.Output, "recon brief text")
	}
	if written.SessionID != "sess-abc" {
		t.Errorf("SessionID = %q, want %q", written.SessionID, "sess-abc")
	}
	if written.CostUSD != 0.0012 {
		t.Errorf("CostUSD = %v, want 0.0012", written.CostUSD)
	}
	if written.DurationSeconds != 15.0 {
		t.Errorf("DurationSeconds = %v, want 15.0", written.DurationSeconds)
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

func TestOutcomeDescriptionWritten(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	outcome := Outcome{
		IssueNumber: 42,
		Title:       "Some issue title",
		Description: "## Problem\nThis is the issue body.",
		Status:      "implemented",
		PRNumber:    57,
	}
	if err := w.WriteOutcome(outcome); err != nil {
		t.Fatalf("WriteOutcome() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "outcome.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading outcome.json: %v", err)
	}

	var written Outcome
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("parsing outcome.json: %v", err)
	}

	if written.Description != outcome.Description {
		t.Errorf("Description = %q, want %q", written.Description, outcome.Description)
	}
}

func TestOutcomeDescriptionOmittedWhenEmpty(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	outcome := Outcome{IssueNumber: 42, Status: "implemented"}
	if err := w.WriteOutcome(outcome); err != nil {
		t.Fatalf("WriteOutcome() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "outcome.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading outcome.json: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	if _, ok := raw["description"]; ok {
		t.Error("description key should be omitted from JSON when empty")
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

func TestWriteVerifyResult(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := VerifyStepResult{
		Attempt:   0,
		AllPassed: true,
		Checks: []CheckResult{
			{Name: "build", Passed: true, ExitCode: 0},
		},
	}
	if err := w.WriteVerifyResult(42, step); err != nil {
		t.Fatalf("WriteVerifyResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "verify-0.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s, got: %v", path, err)
	}
}

func TestWriteVerifyResultMultipleAttempts(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step0 := VerifyStepResult{Attempt: 0, AllPassed: false, FixAttempted: false}
	if err := w.WriteVerifyResult(42, step0); err != nil {
		t.Fatalf("WriteVerifyResult(attempt=0) error: %v", err)
	}

	step1 := VerifyStepResult{Attempt: 1, AllPassed: true, FixAttempted: true}
	if err := w.WriteVerifyResult(42, step1); err != nil {
		t.Fatalf("WriteVerifyResult(attempt=1) error: %v", err)
	}

	for _, name := range []string{"verify-0.json", "verify-1.json"} {
		path := filepath.Join(w.Dir(), "issues", "42", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file at %s, got: %v", path, err)
		}
	}
}

func TestWriteVerifyResultContents(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := VerifyStepResult{
		Attempt:      1,
		AllPassed:    false,
		FixAttempted: true,
		Checks: []CheckResult{
			{Name: "build", Passed: false, Output: "error: build failed", ExitCode: 1},
		},
	}
	if err := w.WriteVerifyResult(42, step); err != nil {
		t.Fatalf("WriteVerifyResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "verify-1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading verify-1.json: %v", err)
	}

	var written VerifyStepResult
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("parsing verify-1.json: %v", err)
	}

	if written.Attempt != 1 {
		t.Errorf("Attempt = %d, want 1", written.Attempt)
	}
	if written.AllPassed {
		t.Errorf("AllPassed = true, want false")
	}
	if !written.FixAttempted {
		t.Errorf("FixAttempted = false, want true")
	}
	if len(written.Checks) != 1 {
		t.Fatalf("Checks: got %d, want 1", len(written.Checks))
	}
	if written.Checks[0].Name != "build" {
		t.Errorf("Checks[0].Name = %q, want %q", written.Checks[0].Name, "build")
	}
	if written.Checks[0].Passed {
		t.Errorf("Checks[0].Passed = true, want false")
	}
	if written.Checks[0].ExitCode != 1 {
		t.Errorf("Checks[0].ExitCode = %d, want 1", written.Checks[0].ExitCode)
	}
}

func TestWriteIssueStatus(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	status := IssueStatus{Status: "implementing"}
	if err := w.WriteIssueStatus(42, status); err != nil {
		t.Fatalf("WriteIssueStatus() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "status.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading status.json: %v", err)
	}

	var written IssueStatus
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("parsing status.json: %v", err)
	}

	if written.Status != "implementing" {
		t.Errorf("Status: got %q, want %q", written.Status, "implementing")
	}
}

func TestWriteIssueDepsUpdatesRunJSON(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{1, 2, 3})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	deps := []IssueDep{
		{IssueNumber: 3, DependsOn: []int{1, 2}},
	}
	if err := w.WriteIssueDeps(deps); err != nil {
		t.Fatalf("WriteIssueDeps() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(w.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var meta RunMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}

	if len(meta.IssueDeps) != 1 {
		t.Fatalf("IssueDeps: got %d entries, want 1", len(meta.IssueDeps))
	}
	if meta.IssueDeps[0].IssueNumber != 3 {
		t.Errorf("IssueDeps[0].IssueNumber: got %d, want 3", meta.IssueDeps[0].IssueNumber)
	}
	if len(meta.IssueDeps[0].DependsOn) != 2 {
		t.Fatalf("IssueDeps[0].DependsOn: got %d entries, want 2", len(meta.IssueDeps[0].DependsOn))
	}
	if meta.IssueDeps[0].DependsOn[0] != 1 || meta.IssueDeps[0].DependsOn[1] != 2 {
		t.Errorf("IssueDeps[0].DependsOn: got %v, want [1 2]", meta.IssueDeps[0].DependsOn)
	}
}

func TestWriteIssueDepsPreservesExistingRunJSONFields(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{1, 2})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	deps := []IssueDep{{IssueNumber: 2, DependsOn: []int{1}}}
	if err := w.WriteIssueDeps(deps); err != nil {
		t.Fatalf("WriteIssueDeps() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(w.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var meta RunMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}

	// Ensure pre-existing fields are unchanged.
	if meta.Repo != "owner/repo" {
		t.Errorf("Repo: got %q, want %q", meta.Repo, "owner/repo")
	}
	if meta.Milestone != "Phase 7" {
		t.Errorf("Milestone: got %q, want %q", meta.Milestone, "Phase 7")
	}
	if len(meta.IssueNumbers) != 2 {
		t.Errorf("IssueNumbers: got %v, want [1 2]", meta.IssueNumbers)
	}
}

func TestWriteIssueTitlesUpdatesRunJSON(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{10, 20})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	titles := map[string]string{
		"10": "Fix the login bug",
		"20": "Add dark mode toggle",
	}
	if err := w.WriteIssueTitles(titles); err != nil {
		t.Fatalf("WriteIssueTitles() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(w.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var meta RunMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}

	if len(meta.IssueTitles) != 2 {
		t.Fatalf("IssueTitles: got %d entries, want 2", len(meta.IssueTitles))
	}
	if meta.IssueTitles["10"] != "Fix the login bug" {
		t.Errorf("IssueTitles[10]: got %q, want %q", meta.IssueTitles["10"], "Fix the login bug")
	}
	if meta.IssueTitles["20"] != "Add dark mode toggle" {
		t.Errorf("IssueTitles[20]: got %q, want %q", meta.IssueTitles["20"], "Add dark mode toggle")
	}
}

func TestWriteIssueTitlesPreservesExistingRunJSONFields(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{1, 2})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	titles := map[string]string{"1": "First issue", "2": "Second issue"}
	if err := w.WriteIssueTitles(titles); err != nil {
		t.Fatalf("WriteIssueTitles() error: %v", err)
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
		t.Errorf("Repo: got %q, want %q", meta.Repo, "owner/repo")
	}
	if meta.Milestone != "Phase 7" {
		t.Errorf("Milestone: got %q, want %q", meta.Milestone, "Phase 7")
	}
	if len(meta.IssueNumbers) != 2 {
		t.Errorf("IssueNumbers: got %v, want [1 2]", meta.IssueNumbers)
	}
}

func TestIssueTitlesOmittedWhenNeverWritten(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{1})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	// Do not call WriteIssueTitles.
	_ = w

	data, err := os.ReadFile(filepath.Join(w.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	if _, ok := raw["issue_titles"]; ok {
		t.Error("issue_titles key should be omitted from JSON when WriteIssueTitles was never called")
	}
}

func TestIssueTitlesMissingInOldDataDeserializesCleanly(t *testing.T) {
	// Simulate old run.json without issue_titles field.
	oldJSON := []byte(`{"repo":"owner/repo","milestone":"Phase 7","issue_numbers":[1],"started_at":"2024-01-01T00:00:00Z"}`)

	var meta RunMeta
	if err := json.Unmarshal(oldJSON, &meta); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if meta.IssueTitles != nil {
		t.Errorf("IssueTitles = %v, want nil for old data without issue_titles", meta.IssueTitles)
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
		_, err := New(repo, "Phase 7", []int{1}, "", AutoMerge{})
		if err == nil {
			t.Errorf("New(%q) should return error for path traversal, got nil", repo)
		}
	}
}

func TestWriteRiskAssessmentCreatesFile(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{42})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	assessment := RiskAssessment{
		IsLowRisk: true,
		Gates: []RiskGate{
			{Name: "max_lines", Passed: true, Detail: "10 lines changed (max 200)"},
			{Name: "protected_paths", Passed: true, Detail: "no protected paths touched"},
		},
	}
	if err := w.WriteRiskAssessment(42, assessment); err != nil {
		t.Fatalf("WriteRiskAssessment() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "42", "risk-assessment.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file at %s, got: %v", path, err)
	}

	var written RiskAssessment
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	if written.IsLowRisk != true {
		t.Errorf("IsLowRisk = %v, want true", written.IsLowRisk)
	}
	if len(written.Gates) != 2 {
		t.Errorf("Gates length = %d, want 2", len(written.Gates))
	}
}

func TestBaseBranchWrittenToRunJSON(t *testing.T) {
	base := t.TempDir()
	t.Setenv("HOME", base)
	w, err := New("owner/repo", "Phase 7", []int{1}, "feature/foo", AutoMerge{})
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

	if meta.BaseBranch != "feature/foo" {
		t.Errorf("BaseBranch = %q, want %q", meta.BaseBranch, "feature/foo")
	}
}

func TestBaseBranchOmittedWhenEmpty(t *testing.T) {
	base := t.TempDir()
	t.Setenv("HOME", base)
	w, err := New("owner/repo", "Phase 7", []int{1}, "", AutoMerge{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(w.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	if _, ok := raw["base_branch"]; ok {
		t.Error("base_branch key should be omitted from JSON when empty")
	}
}

func TestAutoMergeWrittenToRunJSON(t *testing.T) {
	base := t.TempDir()
	t.Setenv("HOME", base)
	w, err := New("owner/repo", "Phase 7", []int{1}, "", AutoMerge{Feature: "low_risk", Rollup: "auto"})
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

	if meta.AutoMerge == nil {
		t.Fatal("AutoMerge should be set in run.json")
	}
	if meta.AutoMerge.Feature != "low_risk" {
		t.Errorf("AutoMerge.Feature = %q, want %q", meta.AutoMerge.Feature, "low_risk")
	}
	if meta.AutoMerge.Rollup != "auto" {
		t.Errorf("AutoMerge.Rollup = %q, want %q", meta.AutoMerge.Rollup, "auto")
	}
}

func TestAutoMergeOmittedWhenEmpty(t *testing.T) {
	base := t.TempDir()
	t.Setenv("HOME", base)
	w, err := New("owner/repo", "Phase 7", []int{1}, "", AutoMerge{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(w.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	if _, ok := raw["auto_merge"]; ok {
		t.Error("auto_merge key should be omitted from JSON when AutoMerge is empty")
	}
}

func TestAutoMergeMissingInOldDataDeserializesCleanly(t *testing.T) {
	// Simulate old run.json without auto_merge field.
	oldJSON := []byte(`{"repo":"owner/repo","milestone":"Phase 7","issue_numbers":[1],"started_at":"2024-01-01T00:00:00Z"}`)

	var meta RunMeta
	if err := json.Unmarshal(oldJSON, &meta); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if meta.AutoMerge != nil {
		t.Errorf("AutoMerge = %+v, want nil for old data without auto_merge", meta.AutoMerge)
	}
}

func TestBaseBranchMissingInOldDataDeserializesCleanly(t *testing.T) {
	// Simulate old run.json without base_branch field.
	oldJSON := []byte(`{"repo":"owner/repo","milestone":"Phase 7","issue_numbers":[1],"started_at":"2024-01-01T00:00:00Z"}`)

	var meta RunMeta
	if err := json.Unmarshal(oldJSON, &meta); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if meta.BaseBranch != "" {
		t.Errorf("BaseBranch = %q, want empty string for old data", meta.BaseBranch)
	}
	if meta.Repo != "owner/repo" {
		t.Errorf("Repo = %q, want %q", meta.Repo, "owner/repo")
	}
}

func TestWriteRollupVerifyResult(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{1})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := VerifyStepResult{
		Attempt:   0,
		AllPassed: true,
		Checks: []CheckResult{
			{Name: "build", Passed: true, ExitCode: 0},
		},
	}
	if err := w.WriteRollupVerifyResult(step); err != nil {
		t.Fatalf("WriteRollupVerifyResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "rollup", "verify-0.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s, got: %v", path, err)
	}
}

func TestWriteRollupVerifyResultMultipleAttempts(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 7", []int{1})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step0 := VerifyStepResult{Attempt: 0, AllPassed: false, FixAttempted: false}
	if err := w.WriteRollupVerifyResult(step0); err != nil {
		t.Fatalf("WriteRollupVerifyResult(attempt=0) error: %v", err)
	}

	step1 := VerifyStepResult{Attempt: 1, AllPassed: true, FixAttempted: true}
	if err := w.WriteRollupVerifyResult(step1); err != nil {
		t.Fatalf("WriteRollupVerifyResult(attempt=1) error: %v", err)
	}

	for _, name := range []string{"verify-0.json", "verify-1.json"} {
		path := filepath.Join(w.Dir(), "rollup", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file at %s, got: %v", path, err)
		}
	}
}

func TestIssueCostUSD_NoFiles(t *testing.T) {
	// When no step files exist, IssueCostUSD must return 0.0.
	got := IssueCostUSD(t.TempDir())
	if got != 0.0 {
		t.Errorf("IssueCostUSD with no files = %f, want 0.0", got)
	}
}

func TestIssueCostUSD_SingleStep(t *testing.T) {
	base := t.TempDir()
	w, err := NewWithBase(base, "owner/repo", "m1", []int{1}, "", AutoMerge{})
	if err != nil {
		t.Fatalf("NewWithBase() error: %v", err)
	}

	if err := w.WriteImplementResult(1, StepResult{CostUSD: 1.50}); err != nil {
		t.Fatalf("WriteImplementResult() error: %v", err)
	}

	got := IssueCostUSD(w.IssueDir(1))
	if got != 1.50 {
		t.Errorf("IssueCostUSD = %f, want 1.50", got)
	}
}

func TestIssueCostUSD_MultipleSteps(t *testing.T) {
	base := t.TempDir()
	w, err := NewWithBase(base, "owner/repo", "m1", []int{7}, "", AutoMerge{})
	if err != nil {
		t.Fatalf("NewWithBase() error: %v", err)
	}

	if err := w.WriteReconResult(7, StepResult{CostUSD: 0.10}); err != nil {
		t.Fatalf("WriteReconResult() error: %v", err)
	}
	if err := w.WriteImplementResult(7, StepResult{CostUSD: 1.00}); err != nil {
		t.Fatalf("WriteImplementResult() error: %v", err)
	}
	if err := w.WriteReviewResult(7, "quality", StepResult{CostUSD: 0.20}); err != nil {
		t.Fatalf("WriteReviewResult(quality) error: %v", err)
	}
	if err := w.WriteReviewResult(7, "functional", StepResult{CostUSD: 0.30}); err != nil {
		t.Fatalf("WriteReviewResult(functional) error: %v", err)
	}
	// Retry steps.
	if err := w.WriteRetryResult(7, 1, StepResult{CostUSD: 0.80}); err != nil {
		t.Fatalf("WriteRetryResult() error: %v", err)
	}
	if err := w.WriteRetryReviewResult(7, 1, StepResult{CostUSD: 0.15}); err != nil {
		t.Fatalf("WriteRetryReviewResult() error: %v", err)
	}
	if err := w.WriteRetryFunctionalReviewResult(7, 1, StepResult{CostUSD: 0.25}); err != nil {
		t.Fatalf("WriteRetryFunctionalReviewResult() error: %v", err)
	}

	const want = 0.10 + 1.00 + 0.20 + 0.30 + 0.80 + 0.15 + 0.25
	got := IssueCostUSD(w.IssueDir(7))
	// Use a small epsilon for floating-point comparison.
	const eps = 1e-9
	if got < want-eps || got > want+eps {
		t.Errorf("IssueCostUSD = %.10f, want %.10f", got, want)
	}
}

func TestIssueCostUSD_ZeroCostSteps(t *testing.T) {
	// Steps written without CostUSD (zero value) should not inflate the total.
	base := t.TempDir()
	w, err := NewWithBase(base, "owner/repo", "m1", []int{3}, "", AutoMerge{})
	if err != nil {
		t.Fatalf("NewWithBase() error: %v", err)
	}

	if err := w.WriteImplementResult(3, StepResult{}); err != nil {
		t.Fatalf("WriteImplementResult() error: %v", err)
	}

	got := IssueCostUSD(w.IssueDir(3))
	if got != 0.0 {
		t.Errorf("IssueCostUSD with zero-cost step = %f, want 0.0", got)
	}
}

func TestIssueDir(t *testing.T) {
	base := t.TempDir()
	w, err := NewWithBase(base, "owner/repo", "m1", []int{5}, "", AutoMerge{})
	if err != nil {
		t.Fatalf("NewWithBase() error: %v", err)
	}

	got := w.IssueDir(5)
	want := filepath.Join(w.Dir(), "issues", "5")
	if got != want {
		t.Errorf("IssueDir(5) = %q, want %q", got, want)
	}
}

func TestStepResult_ResourceFieldsSerialization(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 24", []int{544})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	step := StepResult{
		CostUSD:         0.05,
		PeakMemoryBytes: 256 * 1024 * 1024,
		CPUNanoseconds:  1_500_000_000,
	}
	if err := w.WriteImplementResult(544, step); err != nil {
		t.Fatalf("WriteImplementResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "544", "implement.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading implement.json: %v", err)
	}

	var written StepResult
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("parsing implement.json: %v", err)
	}

	if written.PeakMemoryBytes != step.PeakMemoryBytes {
		t.Errorf("PeakMemoryBytes = %d, want %d", written.PeakMemoryBytes, step.PeakMemoryBytes)
	}
	if written.CPUNanoseconds != step.CPUNanoseconds {
		t.Errorf("CPUNanoseconds = %d, want %d", written.CPUNanoseconds, step.CPUNanoseconds)
	}
}

func TestStepResult_ResourceFieldsOmittedWhenZero(t *testing.T) {
	base := t.TempDir()
	w, err := newWithBase(t, base, "owner/repo", "Phase 24", []int{544})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// StepResult with zero resource fields.
	step := StepResult{CostUSD: 0.01}
	if err := w.WriteImplementResult(544, step); err != nil {
		t.Fatalf("WriteImplementResult() error: %v", err)
	}

	path := filepath.Join(w.Dir(), "issues", "544", "implement.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading implement.json: %v", err)
	}

	// Zero int64 fields with omitempty must not appear in the JSON.
	raw := string(data)
	if strings.Contains(raw, "peak_memory_bytes") {
		t.Errorf("peak_memory_bytes should be omitted when zero, but appears in JSON: %s", raw)
	}
	if strings.Contains(raw, "cpu_nanoseconds") {
		t.Errorf("cpu_nanoseconds should be omitted when zero, but appears in JSON: %s", raw)
	}
}
