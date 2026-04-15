package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/stats"
)

// TestTraceCommandRegistered checks the trace command is wired to root.
func TestTraceCommandRegistered(t *testing.T) {
	names := make(map[string]bool)
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
	}
	if !names["trace"] {
		t.Error("trace subcommand not registered on rootCmd")
	}
}

// TestTraceCommandFlags verifies all flags are present with correct defaults.
func TestTraceCommandFlags(t *testing.T) {
	for _, name := range []string{"repo", "run", "json", "detail"} {
		if traceCmd.Flags().Lookup(name) == nil {
			t.Errorf("trace command missing flag --%s", name)
		}
	}

	jsonFlag := traceCmd.Flags().Lookup("json")
	if jsonFlag.DefValue != "false" {
		t.Errorf("--json default = %q, want %q", jsonFlag.DefValue, "false")
	}
}

func openTraceTestDB(t *testing.T) *stats.DB {
	t.Helper()
	db, err := stats.Open(":memory:")
	if err != nil {
		t.Fatalf("stats.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func seedTraceData(t *testing.T, db *stats.DB) {
	t.Helper()
	ctx := context.Background()

	run := stats.RunRecord{
		ID:        "run-1",
		Repo:      "org/repo",
		StartedAt: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
	}
	if err := stats.WriteRun(ctx, db, run); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}

	outcome := stats.IssueOutcomeRecord{
		RunID:       "run-1",
		IssueNumber: 42,
		Title:       "Add feature",
		Status:      "implemented",
		PRNumber:    100,
		TraceID:     "t-1",
	}
	if err := stats.WriteIssueOutcome(ctx, db, outcome); err != nil {
		t.Fatalf("WriteIssueOutcome: %v", err)
	}

	steps := []stats.StepResultRecord{
		{
			RunID:           "run-1",
			IssueNumber:     42,
			StepName:        "implement",
			CostUSD:         0.05,
			DurationSeconds: 120,
			Flags:           []string{"low_cost"},
			StartedAt:       time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
			FinishedAt:      time.Date(2026, 3, 1, 10, 2, 0, 0, time.UTC),
			TraceID:         "t-1",
		},
		{
			RunID:           "run-1",
			IssueNumber:     42,
			StepName:        "quality-review",
			CostUSD:         0.03,
			DurationSeconds: 60,
			Flags:           []string{},
			StartedAt:       time.Date(2026, 3, 1, 10, 2, 0, 0, time.UTC),
			FinishedAt:      time.Date(2026, 3, 1, 10, 3, 0, 0, time.UTC),
			TraceID:         "t-1",
		},
	}
	for _, s := range steps {
		if err := stats.WriteStepResult(ctx, db, s); err != nil {
			t.Fatalf("WriteStepResult: %v", err)
		}
	}
}

// TestTraceByIssueNumber: seed stats.db with issue #42 with trace_id "t-1",
// verify timeline output with correct steps.
func TestTraceByIssueNumber(t *testing.T) {
	db := openTraceTestDB(t)
	seedTraceData(t, db)

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "", "", false, false); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "t-1") {
		t.Errorf("output missing trace ID; got:\n%s", out)
	}
	if !strings.Contains(out, "#42") {
		t.Errorf("output missing issue number; got:\n%s", out)
	}
	if !strings.Contains(out, "implement") {
		t.Errorf("output missing step 'implement'; got:\n%s", out)
	}
	if !strings.Contains(out, "quality-review") {
		t.Errorf("output missing step 'quality-review'; got:\n%s", out)
	}
	if !strings.Contains(out, "implemented") {
		t.Errorf("output missing status 'implemented'; got:\n%s", out)
	}
}

// TestTraceByTraceID: query by trace ID directly outputs the same timeline.
func TestTraceByTraceID(t *testing.T) {
	db := openTraceTestDB(t)
	seedTraceData(t, db)

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "t-1", "", "", false, false); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "t-1") {
		t.Errorf("output missing trace ID; got:\n%s", out)
	}
	if !strings.Contains(out, "#42") {
		t.Errorf("output missing issue number; got:\n%s", out)
	}
	if !strings.Contains(out, "implement") {
		t.Errorf("output missing step 'implement'; got:\n%s", out)
	}
}

// TestTraceMultipleRunsMostRecent: two runs for issue #42,
// without --run, returns the most recent.
func TestTraceMultipleRunsMostRecent(t *testing.T) {
	db := openTraceTestDB(t)
	ctx := context.Background()

	if err := stats.WriteRun(ctx, db, stats.RunRecord{
		ID: "run-old", Repo: "org/repo",
		StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	if err := stats.WriteRun(ctx, db, stats.RunRecord{
		ID: "run-new", Repo: "org/repo",
		StartedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}

	if err := stats.WriteIssueOutcome(ctx, db, stats.IssueOutcomeRecord{
		RunID: "run-old", IssueNumber: 42, Status: "failed", TraceID: "t-old",
	}); err != nil {
		t.Fatalf("WriteIssueOutcome: %v", err)
	}
	if err := stats.WriteIssueOutcome(ctx, db, stats.IssueOutcomeRecord{
		RunID: "run-new", IssueNumber: 42, Status: "implemented", TraceID: "t-new",
	}); err != nil {
		t.Fatalf("WriteIssueOutcome: %v", err)
	}

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "", "", false, false); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "t-new") {
		t.Errorf("expected most recent trace 't-new'; got:\n%s", out)
	}
	if strings.Contains(out, "t-old") {
		t.Errorf("should not contain older trace 't-old'; got:\n%s", out)
	}
}

// TestTraceWithRunFilter: --run returns that specific run's trace.
func TestTraceWithRunFilter(t *testing.T) {
	db := openTraceTestDB(t)
	ctx := context.Background()

	if err := stats.WriteRun(ctx, db, stats.RunRecord{
		ID: "run-old", Repo: "org/repo",
		StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	if err := stats.WriteRun(ctx, db, stats.RunRecord{
		ID: "run-new", Repo: "org/repo",
		StartedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}

	if err := stats.WriteIssueOutcome(ctx, db, stats.IssueOutcomeRecord{
		RunID: "run-old", IssueNumber: 42, Status: "failed", TraceID: "t-old",
	}); err != nil {
		t.Fatalf("WriteIssueOutcome: %v", err)
	}
	if err := stats.WriteIssueOutcome(ctx, db, stats.IssueOutcomeRecord{
		RunID: "run-new", IssueNumber: 42, Status: "implemented", TraceID: "t-new",
	}); err != nil {
		t.Fatalf("WriteIssueOutcome: %v", err)
	}

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "", "run-old", false, false); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "t-old") {
		t.Errorf("expected trace from run-old 't-old'; got:\n%s", out)
	}
}

// TestTraceNoResults: error message for non-existent issue.
func TestTraceNoResults(t *testing.T) {
	db := openTraceTestDB(t)

	var buf bytes.Buffer
	err := runTrace(&buf, db, "999", "", "", false, false)
	if err == nil {
		t.Fatal("expected error for non-existent issue, got nil")
	}
	if !strings.Contains(err.Error(), "no trace found for issue #999") {
		t.Errorf("error = %q, want to contain 'no trace found for issue #999'", err.Error())
	}
}

// TestTraceNoResultsTraceID: error message for non-existent trace ID.
func TestTraceNoResultsTraceID(t *testing.T) {
	db := openTraceTestDB(t)

	var buf bytes.Buffer
	err := runTrace(&buf, db, "nonexistent-trace", "", "", false, false)
	if err == nil {
		t.Fatal("expected error for non-existent trace ID, got nil")
	}
	if !strings.Contains(err.Error(), "no trace found for trace ID nonexistent-trace") {
		t.Errorf("error = %q, want to contain 'no trace found for trace ID'", err.Error())
	}
}

// TestTraceJSONOutput: valid JSON with correct structure.
func TestTraceJSONOutput(t *testing.T) {
	db := openTraceTestDB(t)
	seedTraceData(t, db)

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "", "", true, false); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	var data traceJSON
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}

	if data.TraceID != "t-1" {
		t.Errorf("trace_id = %q, want %q", data.TraceID, "t-1")
	}
	if data.IssueNumber != 42 {
		t.Errorf("issue_number = %d, want %d", data.IssueNumber, 42)
	}
	if data.Outcome.Status != "implemented" {
		t.Errorf("outcome.status = %q, want %q", data.Outcome.Status, "implemented")
	}
	if data.Outcome.PRNumber != 100 {
		t.Errorf("outcome.pr_number = %d, want %d", data.Outcome.PRNumber, 100)
	}
	if len(data.Steps) != 2 {
		t.Errorf("got %d steps, want 2", len(data.Steps))
	}
	if len(data.Steps) > 0 && data.Steps[0].StepName != "implement" {
		t.Errorf("steps[0].step_name = %q, want %q", data.Steps[0].StepName, "implement")
	}
}

// TestTraceMissingDatabase: error message when stats.db doesn't exist.
func TestTraceMissingDatabase(t *testing.T) {
	orig := newTraceDB
	newTraceDB = func() (*stats.DB, error) {
		return openStatsDBAt("/tmp/this-path-does-not-exist-godark-trace-test.db")
	}
	defer func() { newTraceDB = orig }()

	cmd := traceCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, []string{"42"})
	if err == nil {
		t.Fatal("expected error for missing database, got nil")
	}
	if !strings.Contains(err.Error(), "no stats database found") {
		t.Errorf("error = %q, want to contain 'no stats database found'", err.Error())
	}
}

// TestTraceRepoFilter: --repo filters to a specific repository.
func TestTraceRepoFilter(t *testing.T) {
	db := openTraceTestDB(t)
	ctx := context.Background()

	if err := stats.WriteRun(ctx, db, stats.RunRecord{
		ID: "run-a", Repo: "org/repo-a",
		StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	if err := stats.WriteRun(ctx, db, stats.RunRecord{
		ID: "run-b", Repo: "org/repo-b",
		StartedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}

	if err := stats.WriteIssueOutcome(ctx, db, stats.IssueOutcomeRecord{
		RunID: "run-a", IssueNumber: 42, Status: "implemented", TraceID: "t-a",
	}); err != nil {
		t.Fatalf("WriteIssueOutcome: %v", err)
	}
	if err := stats.WriteIssueOutcome(ctx, db, stats.IssueOutcomeRecord{
		RunID: "run-b", IssueNumber: 42, Status: "failed", TraceID: "t-b",
	}); err != nil {
		t.Fatalf("WriteIssueOutcome: %v", err)
	}

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "org/repo-a", "", false, false); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "t-a") {
		t.Errorf("expected trace 't-a' for org/repo-a; got:\n%s", out)
	}
}

// buildFixtureDir creates a minimal run data directory structure for detail tests.
// Returns the base dir. The run data is at <base>/org/repo/<runID>/.
func buildFixtureDir(t *testing.T, runID string, issueNum int, step rundata.StepResult, verify *rundata.VerifyStepResult, risk *rundata.RiskAssessment, retry *rundata.RetryDetail, dialogue []rundata.DialogueEntry) string {
	t.Helper()
	base := t.TempDir()

	issueDir := filepath.Join(base, "org", "repo", runID, "issues", strconv.Itoa(issueNum))
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write run.json
	runMeta := rundata.RunMeta{
		Repo:         "org/repo",
		IssueNumbers: []int{issueNum},
		StartedAt:    time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
	}
	writeJSON(t, filepath.Join(base, "org", "repo", runID, "run.json"), runMeta)

	// Write step files: recon and implement use the same data for simplicity
	writeJSON(t, filepath.Join(issueDir, "recon.json"), step)
	writeJSON(t, filepath.Join(issueDir, "implement.json"), step)

	// Write outcome
	outcome := rundata.Outcome{Status: "implemented"}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"), outcome)

	if verify != nil {
		writeJSON(t, filepath.Join(issueDir, "verify-0.json"), verify)
	}

	if risk != nil {
		writeJSON(t, filepath.Join(issueDir, "risk-assessment.json"), risk)
	}

	if retry != nil {
		retryDir := filepath.Join(issueDir, "retries", strconv.Itoa(retry.Attempt))
		if err := os.MkdirAll(retryDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		writeJSON(t, filepath.Join(retryDir, "retry.json"), retry.Retry)
		writeJSON(t, filepath.Join(retryDir, "quality-review.json"), retry.QualityReview)
		writeJSON(t, filepath.Join(retryDir, "functional-review.json"), retry.FunctionalReview)
	}

	if dialogue != nil {
		writeJSON(t, filepath.Join(issueDir, "dialogue.json"), dialogue)
	}

	return base
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// seedDetailData seeds stats.db with a run and outcome for detail tests.
func seedDetailData(t *testing.T, db *stats.DB, runID string, issueNum int) {
	t.Helper()
	ctx := context.Background()

	run := stats.RunRecord{
		ID:        runID,
		Repo:      "org/repo",
		StartedAt: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
		FinishedAt: time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC),
	}
	if err := stats.WriteRun(ctx, db, run); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}

	outcome := stats.IssueOutcomeRecord{
		RunID:       runID,
		IssueNumber: issueNum,
		Title:       "Test issue",
		Status:      "implemented",
		PRNumber:    742,
		TraceID:     "t-detail",
	}
	if err := stats.WriteIssueOutcome(ctx, db, outcome); err != nil {
		t.Fatalf("WriteIssueOutcome: %v", err)
	}
}

// TestTraceDetailRendersSteps: verify --detail renders step section headers.
func TestTraceDetailRendersSteps(t *testing.T) {
	db := openTraceTestDB(t)
	seedDetailData(t, db, "run-detail", 42)

	started := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	step := rundata.StepResult{
		Output:          "some output",
		DurationSeconds: 150,
		CostUSD:         0.0412,
		ToolTrace:       []string{"tool1", "tool2"},
		StartedAt:       &started,
	}
	base := buildFixtureDir(t, "run-detail", 42, step, nil, nil, nil, nil)

	orig := newTraceReader
	newTraceReader = func() (*rundata.Reader, error) {
		return rundata.NewReaderWithBase(base, nil), nil
	}
	defer func() { newTraceReader = orig }()

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "", "", false, true); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	for _, section := range []string{"=== RECON", "=== IMPLEMENT", "=== OUTCOME"} {
		if !strings.Contains(out, section) {
			t.Errorf("output missing section %q; got:\n%s", section, out)
		}
	}
	if !strings.Contains(out, "some output") {
		t.Errorf("output missing step output; got:\n%s", out)
	}
	if !strings.Contains(out, "2 calls") {
		t.Errorf("output missing tool trace count; got:\n%s", out)
	}
}

// TestTraceDetailWithPrompt: verify prompt is rendered when present.
func TestTraceDetailWithPrompt(t *testing.T) {
	db := openTraceTestDB(t)
	seedDetailData(t, db, "run-prompt", 42)

	started := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	step := rundata.StepResult{
		Prompt:          "Line one\nLine two\nLine three\nLine four",
		Output:          "output",
		DurationSeconds: 60,
		CostUSD:         0.01,
		StartedAt:       &started,
	}
	base := buildFixtureDir(t, "run-prompt", 42, step, nil, nil, nil, nil)

	orig := newTraceReader
	newTraceReader = func() (*rundata.Reader, error) {
		return rundata.NewReaderWithBase(base, nil), nil
	}
	defer func() { newTraceReader = orig }()

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "", "", false, true); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Line one") {
		t.Errorf("output missing prompt; got:\n%s", out)
	}
	// Verify the prompt appears in the RECON section (prompt is set for recon/implement steps)
	if !strings.Contains(out, "Prompt: Line one") {
		t.Errorf("output missing prompt content in step section; got:\n%s", out)
	}
}

// TestTraceDetailWithoutPrompt: verify [not captured] when prompt is empty.
func TestTraceDetailWithoutPrompt(t *testing.T) {
	db := openTraceTestDB(t)
	seedDetailData(t, db, "run-noprompt", 42)

	started := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	step := rundata.StepResult{
		Output:          "output",
		DurationSeconds: 60,
		CostUSD:         0.01,
		StartedAt:       &started,
	}
	base := buildFixtureDir(t, "run-noprompt", 42, step, nil, nil, nil, nil)

	orig := newTraceReader
	newTraceReader = func() (*rundata.Reader, error) {
		return rundata.NewReaderWithBase(base, nil), nil
	}
	defer func() { newTraceReader = orig }()

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "", "", false, true); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[not captured]") {
		t.Errorf("output missing [not captured]; got:\n%s", out)
	}
}

// TestTraceDetailWithRetries: verify retry appears in output.
func TestTraceDetailWithRetries(t *testing.T) {
	db := openTraceTestDB(t)
	seedDetailData(t, db, "run-retry", 42)

	started := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	step := rundata.StepResult{
		Output:          "output",
		DurationSeconds: 60,
		CostUSD:         0.01,
		StartedAt:       &started,
	}
	retry := &rundata.RetryDetail{
		Attempt: 1,
		Retry: rundata.StepResult{
			Output:          "retry output",
			DurationSeconds: 30,
			CostUSD:         0.005,
		},
		QualityReview: rundata.StepResult{
			Output:          "qr retry",
			DurationSeconds: 15,
			CostUSD:         0.002,
		},
		FunctionalReview: rundata.StepResult{
			Output:          "fr retry",
			DurationSeconds: 10,
			CostUSD:         0.001,
		},
	}
	base := buildFixtureDir(t, "run-retry", 42, step, nil, nil, retry, nil)

	orig := newTraceReader
	newTraceReader = func() (*rundata.Reader, error) {
		return rundata.NewReaderWithBase(base, nil), nil
	}
	defer func() { newTraceReader = orig }()

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "", "", false, true); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "=== RETRY 1") {
		t.Errorf("output missing retry section; got:\n%s", out)
	}
	if !strings.Contains(out, "retry output") {
		t.Errorf("output missing retry content; got:\n%s", out)
	}
}

// TestTraceDetailFallbackMissingDir: falls back to summary when run dir is missing.
func TestTraceDetailFallbackMissingDir(t *testing.T) {
	db := openTraceTestDB(t)
	seedDetailData(t, db, "run-gone", 42)

	// Seed a step so the summary has content
	ctx := context.Background()
	if err := stats.WriteStepResult(ctx, db, stats.StepResultRecord{
		RunID:       "run-gone",
		IssueNumber: 42,
		StepName:    "implement",
		TraceID:     "t-detail",
		StartedAt:   time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
		FinishedAt:  time.Date(2026, 3, 1, 10, 2, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteStepResult: %v", err)
	}

	// Point reader at an empty temp dir (no run data)
	emptyBase := t.TempDir()
	orig := newTraceReader
	newTraceReader = func() (*rundata.Reader, error) {
		return rundata.NewReaderWithBase(emptyBase, nil), nil
	}
	defer func() { newTraceReader = orig }()

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "", "", false, true); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Run data not found on disk") {
		t.Errorf("output missing fallback note; got:\n%s", out)
	}
	if !strings.Contains(out, "implement") {
		t.Errorf("output missing summary step; got:\n%s", out)
	}
}

// TestTraceDetailAndJSONError: --detail and --json together produce an error.
func TestTraceDetailAndJSONError(t *testing.T) {
	db := openTraceTestDB(t)

	var buf bytes.Buffer
	err := runTrace(&buf, db, "42", "", "", true, true)
	if err == nil {
		t.Fatal("expected error for --detail and --json together, got nil")
	}
	if !strings.Contains(err.Error(), "--detail and --json are mutually exclusive") {
		t.Errorf("error = %q, want to contain '--detail and --json are mutually exclusive'", err.Error())
	}
}

// TestTraceDetailVerifyResults: verify results show per-check pass/fail.
func TestTraceDetailVerifyResults(t *testing.T) {
	db := openTraceTestDB(t)
	seedDetailData(t, db, "run-verify", 42)

	started := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	step := rundata.StepResult{
		Output:          "output",
		DurationSeconds: 60,
		CostUSD:         0.01,
		StartedAt:       &started,
	}
	verify := &rundata.VerifyStepResult{
		Attempt:      0,
		AllPassed:    true,
		FixAttempted: false,
		Checks: []rundata.CheckResult{
			{Name: "build", Passed: true, ExitCode: 0},
			{Name: "lint", Passed: true, ExitCode: 0},
			{Name: "test", Passed: false, ExitCode: 1},
		},
	}
	base := buildFixtureDir(t, "run-verify", 42, step, verify, nil, nil, nil)

	orig := newTraceReader
	newTraceReader = func() (*rundata.Reader, error) {
		return rundata.NewReaderWithBase(base, nil), nil
	}
	defer func() { newTraceReader = orig }()

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "", "", false, true); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "=== VERIFY") {
		t.Errorf("output missing verify section; got:\n%s", out)
	}
	if !strings.Contains(out, "build PASS") {
		t.Errorf("output missing 'build PASS'; got:\n%s", out)
	}
	if !strings.Contains(out, "test FAIL") {
		t.Errorf("output missing 'test FAIL'; got:\n%s", out)
	}
}

// TestTraceDetailRiskAssessment: risk assessment shows gate results.
func TestTraceDetailRiskAssessment(t *testing.T) {
	db := openTraceTestDB(t)
	seedDetailData(t, db, "run-risk", 42)

	started := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	step := rundata.StepResult{
		Output:          "output",
		DurationSeconds: 60,
		CostUSD:         0.01,
		StartedAt:       &started,
	}
	risk := &rundata.RiskAssessment{
		IsLowRisk: true,
		Gates: []rundata.RiskGate{
			{Name: "lines_changed", Passed: true, Detail: "ok"},
			{Name: "files_changed", Passed: false, Detail: "too many"},
		},
	}
	base := buildFixtureDir(t, "run-risk", 42, step, nil, risk, nil, nil)

	orig := newTraceReader
	newTraceReader = func() (*rundata.Reader, error) {
		return rundata.NewReaderWithBase(base, nil), nil
	}
	defer func() { newTraceReader = orig }()

	var buf bytes.Buffer
	if err := runTrace(&buf, db, "42", "", "", false, true); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "=== RISK ASSESSMENT") {
		t.Errorf("output missing risk section; got:\n%s", out)
	}
	if !strings.Contains(out, "Low risk: yes") {
		t.Errorf("output missing 'Low risk: yes'; got:\n%s", out)
	}
	if !strings.Contains(out, "lines_changed PASS") {
		t.Errorf("output missing 'lines_changed PASS'; got:\n%s", out)
	}
	if !strings.Contains(out, "files_changed FAIL") {
		t.Errorf("output missing 'files_changed FAIL'; got:\n%s", out)
	}
}
