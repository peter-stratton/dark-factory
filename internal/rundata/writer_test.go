package rundata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
)

func testCfg() *config.Config {
	return &config.Config{
		Repo:       "owner/repo",
		MaxRetries: 3,
	}
}

func testResult() *agent.Result {
	return &agent.Result{
		ExitCode:  0,
		CostUSD:   0.42,
		SessionID: "sess-abc123",
		Verdict:   "APPROVED",
		ToolTrace: []string{"Read foo.go", "Edit bar.go"},
		TimedOut:  false,
	}
}

func TestNew_CreatesRunDirectory(t *testing.T) {
	// Override home dir to avoid writing to ~/.godark in tests.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rdw, err := New("owner/repo", "Phase 1", []int{42, 43}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if _, err := os.Stat(rdw.Dir()); os.IsNotExist(err) {
		t.Fatal("expected run directory to be created")
	}

	// Verify path structure: ~/.godark/runs/owner/repo/<timestamp>/
	expected := filepath.Join(tmpHome, ".godark", "runs", "owner", "repo")
	if !strings.HasPrefix(rdw.Dir(), expected) {
		t.Errorf("run dir %q does not start with %q", rdw.Dir(), expected)
	}
}

func TestNew_RunJSONWrittenAtStart(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	issueNumbers := []int{42, 43}
	rdw, err := New("owner/repo", "Phase 1", issueNumbers, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rdw.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var r runData
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}

	if r.StartedAt == "" {
		t.Error("run.json missing started_at")
	}
	if r.Repo != "owner/repo" {
		t.Errorf("run.json repo = %q, want %q", r.Repo, "owner/repo")
	}
	if len(r.IssueNumbers) != 2 || r.IssueNumbers[0] != 42 || r.IssueNumbers[1] != 43 {
		t.Errorf("run.json issue_numbers = %v, want [42 43]", r.IssueNumbers)
	}
	if r.FinishedAt != nil {
		t.Error("run.json finished_at should be null at start")
	}
}

func TestFinalizeRun_UpdatesRunJSON(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rdw, err := New("owner/repo", "Phase 1", []int{42}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	summary := RunSummary{
		Total:        1,
		Implemented:  1,
		Failed:       0,
		Skipped:      0,
		TotalCostUSD: 1.23,
	}
	if err := rdw.FinalizeRun(summary); err != nil {
		t.Fatalf("FinalizeRun() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rdw.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var r runData
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}

	if r.FinishedAt == nil {
		t.Error("run.json finished_at should be set after FinalizeRun")
	}
	if r.Summary == nil {
		t.Fatal("run.json summary should be set after FinalizeRun")
	}
	if r.Summary.Total != 1 {
		t.Errorf("summary.total = %d, want 1", r.Summary.Total)
	}
	if r.Summary.Implemented != 1 {
		t.Errorf("summary.implemented = %d, want 1", r.Summary.Implemented)
	}
	if r.Summary.TotalCostUSD != 1.23 {
		t.Errorf("summary.total_cost_usd = %f, want 1.23", r.Summary.TotalCostUSD)
	}
}

func TestWriteImplementResult_CreatesFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rdw, err := New("owner/repo", "", []int{42}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := rdw.WriteImplementResult(42, testResult()); err != nil {
		t.Fatalf("WriteImplementResult() error = %v", err)
	}

	path := filepath.Join(rdw.Dir(), "issues", "42", "implement.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading implement.json: %v", err)
	}

	var s StepResult
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("parsing implement.json: %v", err)
	}
	if s.CostUSD != 0.42 {
		t.Errorf("cost_usd = %f, want 0.42", s.CostUSD)
	}
	if s.SessionID != "sess-abc123" {
		t.Errorf("session_id = %q, want sess-abc123", s.SessionID)
	}
}

func TestWriteReviewResult_QualityTopLevel(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rdw, err := New("owner/repo", "", []int{42}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := rdw.WriteReviewResult(42, "quality", 0, testResult()); err != nil {
		t.Fatalf("WriteReviewResult() error = %v", err)
	}

	path := filepath.Join(rdw.Dir(), "issues", "42", "quality-review.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist", path)
	}
}

func TestWriteReviewResult_FunctionalTopLevel(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rdw, err := New("owner/repo", "", []int{42}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := rdw.WriteReviewResult(42, "functional", 0, testResult()); err != nil {
		t.Fatalf("WriteReviewResult() error = %v", err)
	}

	path := filepath.Join(rdw.Dir(), "issues", "42", "functional-review.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist", path)
	}
}

func TestWriteReviewResult_QualityUnderRetry(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rdw, err := New("owner/repo", "", []int{42}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := rdw.WriteReviewResult(42, "quality", 1, testResult()); err != nil {
		t.Fatalf("WriteReviewResult() error = %v", err)
	}

	path := filepath.Join(rdw.Dir(), "issues", "42", "retries", "1", "quality-review.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist", path)
	}
}

func TestWriteRetryResult_CreatesFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rdw, err := New("owner/repo", "", []int{42}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := rdw.WriteRetryResult(42, 1, testResult()); err != nil {
		t.Fatalf("WriteRetryResult() error = %v", err)
	}

	path := filepath.Join(rdw.Dir(), "issues", "42", "retries", "1", "retry.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist", path)
	}
}

func TestWriteOutcome_CreatesFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rdw, err := New("owner/repo", "", []int{42}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	outcome := Outcome{
		IssueNumber:  42,
		IssueTitle:   "Add widget support",
		Status:       "implemented",
		PRNumber:     57,
		Retries:      1,
		TotalCostUSD: 1.85,
		Error:        nil,
	}
	if err := rdw.WriteOutcome(outcome); err != nil {
		t.Fatalf("WriteOutcome() error = %v", err)
	}

	path := filepath.Join(rdw.Dir(), "issues", "42", "outcome.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading outcome.json: %v", err)
	}

	var o Outcome
	if err := json.Unmarshal(data, &o); err != nil {
		t.Fatalf("parsing outcome.json: %v", err)
	}
	if o.IssueNumber != 42 {
		t.Errorf("issue_number = %d, want 42", o.IssueNumber)
	}
	if o.Status != "implemented" {
		t.Errorf("status = %q, want implemented", o.Status)
	}
	if o.PRNumber != 57 {
		t.Errorf("pr_number = %d, want 57", o.PRNumber)
	}
}

func TestTimestampFormat(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	fixedTime := time.Date(2025, 6, 15, 14, 30, 45, 0, time.UTC)
	timeNow = func() time.Time { return fixedTime }
	t.Cleanup(func() { timeNow = time.Now })

	rdw, err := New("owner/repo", "", []int{}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Directory name should be YYYYMMDD-HHMMSS
	base := filepath.Base(rdw.Dir())
	matched, _ := regexp.MatchString(`^\d{8}-\d{6}$`, base)
	if !matched {
		t.Errorf("timestamp directory %q does not match YYYYMMDD-HHMMSS", base)
	}
	if base != "20250615-143045" {
		t.Errorf("timestamp = %q, want 20250615-143045", base)
	}
}

func TestOwnerRepoParsing(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rdw, err := New("myorg/myrepo", "Phase 2", []int{1}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Path should contain owner/repo components
	if !strings.Contains(rdw.Dir(), filepath.Join("myorg", "myrepo")) {
		t.Errorf("run dir %q does not contain owner/repo path", rdw.Dir())
	}
}

func TestNew_InvalidRepoFormat(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	_, err := New("invalidrepo", "", []int{}, testCfg())
	if err == nil {
		t.Fatal("expected error for invalid repo format, got nil")
	}
}

func TestNew_RejectsDotDotInRepo(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cases := []string{
		"../evil/repo",
		"owner/../../../etc",
		"owner/repo/../../../etc",
	}
	for _, repo := range cases {
		_, err := New(repo, "", []int{}, testCfg())
		if err == nil {
			t.Errorf("New(%q) expected error for path traversal, got nil", repo)
		}
	}
}

func TestWriteReviewResult_InvalidKind(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rdw, err := New("owner/repo", "", []int{42}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = rdw.WriteReviewResult(42, "invalid", 0, testResult())
	if err == nil {
		t.Fatal("WriteReviewResult with invalid kind expected error, got nil")
	}
}

func TestResultToStep_UsesRealTimestamps(t *testing.T) {
	start := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	finish := start.Add(2*time.Minute + 30*time.Second)
	result := &agent.Result{
		CostUSD:    0.42,
		SessionID:  "sess-xyz",
		StartedAt:  start,
		FinishedAt: finish,
	}
	step := resultToStep(result)

	if step.StartedAt == nil {
		t.Fatal("expected started_at to be set")
	}
	if step.FinishedAt == nil {
		t.Fatal("expected finished_at to be set")
	}
	if *step.StartedAt != "2025-06-15T10:00:00Z" {
		t.Errorf("started_at = %q, want 2025-06-15T10:00:00Z", *step.StartedAt)
	}
	if *step.FinishedAt != "2025-06-15T10:02:30Z" {
		t.Errorf("finished_at = %q, want 2025-06-15T10:02:30Z", *step.FinishedAt)
	}
	if step.DurationSeconds != 150.0 {
		t.Errorf("duration_seconds = %f, want 150.0", step.DurationSeconds)
	}
}

func TestResultToStep_OmitsTimestampsWhenZero(t *testing.T) {
	result := &agent.Result{CostUSD: 0.10}
	step := resultToStep(result)
	if step.StartedAt != nil {
		t.Error("expected started_at to be nil when agent.Result has zero StartedAt")
	}
	if step.FinishedAt != nil {
		t.Error("expected finished_at to be nil when agent.Result has zero FinishedAt")
	}
	if step.DurationSeconds != 0 {
		t.Errorf("duration_seconds = %f, want 0 when timestamps are zero", step.DurationSeconds)
	}
}

func TestUpdateIssueNumbers(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	rdw, err := New("owner/repo", "Phase 1", []int{}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := rdw.UpdateIssueNumbers([]int{10, 20, 30}); err != nil {
		t.Fatalf("UpdateIssueNumbers() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rdw.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var r runData
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}

	if len(r.IssueNumbers) != 3 {
		t.Fatalf("issue_numbers len = %d, want 3", len(r.IssueNumbers))
	}
	if r.IssueNumbers[0] != 10 || r.IssueNumbers[1] != 20 || r.IssueNumbers[2] != 30 {
		t.Errorf("issue_numbers = %v, want [10 20 30]", r.IssueNumbers)
	}
}

func TestNew_HandlesMissingGodarkDir(t *testing.T) {
	// Start with a home dir that has no .godark directory.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Confirm .godark doesn't exist.
	if _, err := os.Stat(filepath.Join(tmpHome, ".godark")); !os.IsNotExist(err) {
		t.Skip(".godark already exists in temp dir")
	}

	rdw, err := New("owner/repo", "", []int{}, testCfg())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if _, err := os.Stat(rdw.Dir()); os.IsNotExist(err) {
		t.Fatal("expected run directory to be created even when ~/.godark doesn't exist")
	}
}

func TestConfigSnapshot(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg := &config.Config{
		Repo:       "owner/repo",
		MaxRetries: 5,
		NoSandbox:  true,
		NoMerge:    true,
	}
	rdw, err := New("owner/repo", "", []int{}, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rdw.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var r runData
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}

	if r.ConfigSnapshot.MaxRetries != 5 {
		t.Errorf("config_snapshot.max_retries = %d, want 5", r.ConfigSnapshot.MaxRetries)
	}
	if !r.ConfigSnapshot.NoSandbox {
		t.Error("config_snapshot.no_sandbox = false, want true")
	}
	if !r.ConfigSnapshot.NoMerge {
		t.Error("config_snapshot.no_merge = false, want true")
	}
}
