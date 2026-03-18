package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/stats"
)

func TestStepToRecord_Basic(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := now.Add(30 * time.Second)
	step := rundata.StepResult{
		StartedAt:       &now,
		FinishedAt:      &end,
		DurationSeconds: 30,
		CostUSD:         0.05,
		Flags: []rundata.Flag{
			{Code: "A", Message: "alpha"},
			{Code: "B", Message: "beta"},
		},
	}

	rec := stepToRecord("run-1", 42, "implement", step)

	if rec.RunID != "run-1" {
		t.Errorf("RunID: got %q, want %q", rec.RunID, "run-1")
	}
	if rec.IssueNumber != 42 {
		t.Errorf("IssueNumber: got %d, want 42", rec.IssueNumber)
	}
	if rec.StepName != "implement" {
		t.Errorf("StepName: got %q, want implement", rec.StepName)
	}
	if rec.CostUSD != 0.05 {
		t.Errorf("CostUSD: got %f, want 0.05", rec.CostUSD)
	}
	if rec.DurationSeconds != 30 {
		t.Errorf("DurationSeconds: got %f, want 30", rec.DurationSeconds)
	}
	if len(rec.Flags) != 2 || rec.Flags[0] != "A" || rec.Flags[1] != "B" {
		t.Errorf("Flags: got %v, want [A B]", rec.Flags)
	}
	if !rec.StartedAt.Equal(now) {
		t.Errorf("StartedAt: got %v, want %v", rec.StartedAt, now)
	}
	if !rec.FinishedAt.Equal(end) {
		t.Errorf("FinishedAt: got %v, want %v", rec.FinishedAt, end)
	}
}

func TestStepToRecord_NilTimes(t *testing.T) {
	step := rundata.StepResult{
		DurationSeconds: 5,
		CostUSD:         0.01,
	}
	rec := stepToRecord("run-x", 7, "recon", step)
	if !rec.StartedAt.IsZero() {
		t.Errorf("StartedAt: expected zero, got %v", rec.StartedAt)
	}
	if !rec.FinishedAt.IsZero() {
		t.Errorf("FinishedAt: expected zero, got %v", rec.FinishedAt)
	}
}

func TestStepToRecord_NilFlags(t *testing.T) {
	now := time.Now()
	step := rundata.StepResult{
		StartedAt:  &now,
		FinishedAt: &now,
	}
	rec := stepToRecord("r", 1, "step", step)
	if rec.Flags == nil {
		t.Error("Flags: expected empty slice, got nil")
	}
	if len(rec.Flags) != 0 {
		t.Errorf("Flags: expected empty, got %v", rec.Flags)
	}
}

func TestBuildStepRecords_MainSteps(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := now.Add(10 * time.Second)

	issue := rundata.IssueDetail{
		IssueNumber: 10,
		Recon:       rundata.StepResult{StartedAt: &now, FinishedAt: &end},
		Implement:   rundata.StepResult{StartedAt: &now, FinishedAt: &end, CostUSD: 0.1},
		// SpecGenerator, QualityReview, FunctionalReview are zero — skipped
	}

	records := buildStepRecords("ts-001", issue)

	if len(records) != 2 {
		t.Fatalf("expected 2 records (recon + implement), got %d", len(records))
	}
	names := map[string]bool{}
	for _, r := range records {
		names[r.StepName] = true
	}
	if !names["recon"] {
		t.Error("expected step 'recon' in records")
	}
	if !names["implement"] {
		t.Error("expected step 'implement' in records")
	}
}

func TestBuildStepRecords_WithRetries(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := now.Add(5 * time.Second)

	issue := rundata.IssueDetail{
		IssueNumber: 20,
		Implement:   rundata.StepResult{StartedAt: &now, FinishedAt: &end},
		Retries: []rundata.RetryDetail{
			{
				Attempt:       1,
				Retry:         rundata.StepResult{StartedAt: &now, FinishedAt: &end},
				QualityReview: rundata.StepResult{StartedAt: &now, FinishedAt: &end},
				// FunctionalReview is zero — skipped
			},
		},
	}

	records := buildStepRecords("ts-002", issue)

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d: implement, retry-1, retry-1-quality-review", len(records))
	}
	names := map[string]bool{}
	for _, r := range records {
		names[r.StepName] = true
	}
	if !names["implement"] {
		t.Error("expected step 'implement'")
	}
	if !names["retry-1"] {
		t.Error("expected step 'retry-1'")
	}
	if !names["retry-1-quality-review"] {
		t.Error("expected step 'retry-1-quality-review'")
	}
}

func TestBuildStepRecords_EmptyIssue(t *testing.T) {
	issue := rundata.IssueDetail{IssueNumber: 99}
	records := buildStepRecords("ts-empty", issue)
	if len(records) != 0 {
		t.Errorf("expected no records for empty issue, got %d", len(records))
	}
}

// TestWriteRunStats_EndToEnd verifies that WriteRunStats writes a run record,
// issue outcome, and step result to SQLite, using a temp directory for run data.
func TestWriteRunStats_EndToEnd(t *testing.T) {
	baseDir := t.TempDir()
	owner := "org"
	repoName := "myrepo"
	timestamp := "20260101-000000"
	issueDir := filepath.Join(baseDir, owner, repoName, timestamp, "issues", "42")
	if err := os.MkdirAll(issueDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	startedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Minute)

	// Write run.json.
	runMeta := rundata.RunMeta{
		Repo:         "org/myrepo",
		Milestone:    "v1.0",
		BaseBranch:   "main",
		IssueNumbers: []int{42},
		StartedAt:    startedAt,
		FinishedAt:   &finishedAt,
		AutoMerge:    &rundata.AutoMerge{Feature: "low_risk", Rollup: "none"},
		Summary:      &rundata.RunSummary{Total: 1, Implemented: 1},
	}
	writeTestJSON(t, filepath.Join(baseDir, owner, repoName, timestamp, "run.json"), runMeta)

	// Write outcome.json and implement.json for issue 42.
	writeTestJSON(t, filepath.Join(issueDir, "outcome.json"), rundata.Outcome{
		IssueNumber: 42,
		Title:       "Fix the bug",
		Status:      "implemented",
		PRNumber:    99,
	})
	writeTestJSON(t, filepath.Join(issueDir, "implement.json"), rundata.StepResult{
		StartedAt:       &startedAt,
		FinishedAt:      &finishedAt,
		DurationSeconds: 120,
		CostUSD:         0.05,
	})

	// Inject a reader seam pointing at our base dir — overridden below once fixedBaseDir is known.
	origFn := writeRunStatsNewReaderFn
	defer func() { writeRunStatsNewReaderFn = origFn }()

	fixedBaseDir := t.TempDir()
	fixedWriter, err := rundata.NewWithBase(fixedBaseDir, "org/myrepo", "v1.0", []int{42}, "main", rundata.AutoMerge{})
	if err != nil {
		t.Fatalf("NewWithBase fixed: %v", err)
	}

	// Override reader to use our pre-built data dir, mapped by the writer's actual timestamp.
	// We'll rename the writer's dir to match our expected timestamp.
	// Actually, the simplest approach: just copy the run data into the writer's actual dir.
	actualTimestamp := filepath.Base(fixedWriter.Dir())
	actualIssueDir := filepath.Join(fixedBaseDir, "org", "myrepo", actualTimestamp, "issues", "42")
	if err := os.MkdirAll(actualIssueDir, 0o750); err != nil {
		t.Fatalf("mkdir actual issue dir: %v", err)
	}
	// Write files at the real writer path.
	writeTestJSON(t, filepath.Join(actualIssueDir, "outcome.json"), rundata.Outcome{
		IssueNumber: 42,
		Title:       "Fix the bug",
		Status:      "implemented",
		PRNumber:    99,
	})
	writeTestJSON(t, filepath.Join(actualIssueDir, "implement.json"), rundata.StepResult{
		StartedAt:       &startedAt,
		FinishedAt:      &finishedAt,
		DurationSeconds: 120,
		CostUSD:         0.05,
	})

	// Override reader to use fixedBaseDir.
	writeRunStatsNewReaderFn = func(logger *slog.Logger) (*rundata.Reader, error) {
		return rundata.NewReaderWithBase(fixedBaseDir, logger), nil
	}

	// Finalize the writer so run.json has a FinishedAt.
	summary := rundata.RunSummary{Total: 1, Implemented: 1}
	if err := fixedWriter.FinalizeRun(summary); err != nil {
		t.Fatalf("FinalizeRun: %v", err)
	}

	// Open in-memory stats DB.
	db, err := stats.Open(":memory:")
	if err != nil {
		t.Fatalf("stats.Open: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{Repo: "org/myrepo"}
	WriteRunStats(context.Background(), db, cfg, fixedWriter, summary, slog.Default())

	// Verify run record.
	runs, err := stats.QueryRuns(context.Background(), db, stats.RunFilter{})
	if err != nil {
		t.Fatalf("QueryRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run record, got %d", len(runs))
	}
	if runs[0].ID != actualTimestamp {
		t.Errorf("run.ID: got %q, want %q", runs[0].ID, actualTimestamp)
	}
	if runs[0].Implemented != 1 {
		t.Errorf("run.Implemented: got %d, want 1", runs[0].Implemented)
	}

	// Verify issue outcome.
	outcomes, err := stats.QueryIssueOutcomes(context.Background(), db, stats.RunFilter{})
	if err != nil {
		t.Fatalf("QueryIssueOutcomes: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected 1 outcome record, got %d", len(outcomes))
	}
	if outcomes[0].IssueNumber != 42 {
		t.Errorf("outcome.IssueNumber: got %d, want 42", outcomes[0].IssueNumber)
	}
	if outcomes[0].Status != "implemented" {
		t.Errorf("outcome.Status: got %q, want implemented", outcomes[0].Status)
	}

	// Verify step result.
	steps, err := stats.QueryStepResults(context.Background(), db, stats.RunFilter{})
	if err != nil {
		t.Fatalf("QueryStepResults: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step result, got %d", len(steps))
	}
	if steps[0].StepName != "implement" {
		t.Errorf("step.StepName: got %q, want implement", steps[0].StepName)
	}
}

// TestWriteRunStats_NilDB verifies that a nil DB is a no-op (no panic).
func TestWriteRunStats_NilDB(t *testing.T) {
	cfg := &config.Config{Repo: "org/repo"}
	// Should not panic with nil DB.
	WriteRunStats(context.Background(), nil, cfg, nil, rundata.RunSummary{}, slog.Default())
}

// TestWriteRunStats_NilWriter verifies that a nil writer is a no-op (no panic).
func TestWriteRunStats_NilWriter(t *testing.T) {
	db, err := stats.Open(":memory:")
	if err != nil {
		t.Fatalf("stats.Open: %v", err)
	}
	defer db.Close()
	cfg := &config.Config{Repo: "org/repo"}
	// Should not panic with nil writer.
	WriteRunStats(context.Background(), db, cfg, nil, rundata.RunSummary{}, slog.Default())
}

// writeTestJSON marshals v to JSON and writes it to path. Fails the test on error.
func writeTestJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
