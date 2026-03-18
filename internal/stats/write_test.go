package stats

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestWriteRun_ReadBack(t *testing.T) {
	db := openTestDB(t)

	started := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	finished := time.Date(2026, 3, 14, 11, 0, 0, 0, time.UTC)
	run := RunRecord{
		ID:               "20260314-100000",
		Repo:             "org/repo",
		Milestone:        "Phase 21",
		BaseBranch:       "main",
		AutoMergeFeature: "low_risk",
		AutoMergeRollup:  "auto",
		StartedAt:        started,
		FinishedAt:       finished,
		Total:            10,
		Implemented:      8,
		Failed:           2,
		AbortReason:      "",
	}

	if err := WriteRun(context.Background(), db, run); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}

	var (
		gotID, gotRepo, gotMilestone, gotBaseBranch   string
		gotAMFeature, gotAMRollup, gotAbortReason     string
		gotStartedAt, gotFinishedAt                   string
		gotTotal, gotImplemented, gotFailed            int
	)
	err := db.db.QueryRow(
		`SELECT id, repo, milestone, base_branch, auto_merge_feature, auto_merge_rollup,
		        started_at, finished_at, total, implemented, failed, abort_reason
		 FROM runs WHERE id = ?`, run.ID,
	).Scan(&gotID, &gotRepo, &gotMilestone, &gotBaseBranch,
		&gotAMFeature, &gotAMRollup,
		&gotStartedAt, &gotFinishedAt,
		&gotTotal, &gotImplemented, &gotFailed, &gotAbortReason)
	if err != nil {
		t.Fatalf("SELECT run: %v", err)
	}

	if gotID != run.ID {
		t.Errorf("id: got %q, want %q", gotID, run.ID)
	}
	if gotRepo != run.Repo {
		t.Errorf("repo: got %q, want %q", gotRepo, run.Repo)
	}
	if gotMilestone != run.Milestone {
		t.Errorf("milestone: got %q, want %q", gotMilestone, run.Milestone)
	}
	if gotBaseBranch != run.BaseBranch {
		t.Errorf("base_branch: got %q, want %q", gotBaseBranch, run.BaseBranch)
	}
	if gotAMFeature != run.AutoMergeFeature {
		t.Errorf("auto_merge_feature: got %q, want %q", gotAMFeature, run.AutoMergeFeature)
	}
	if gotAMRollup != run.AutoMergeRollup {
		t.Errorf("auto_merge_rollup: got %q, want %q", gotAMRollup, run.AutoMergeRollup)
	}
	if gotTotal != run.Total {
		t.Errorf("total: got %d, want %d", gotTotal, run.Total)
	}
	if gotImplemented != run.Implemented {
		t.Errorf("implemented: got %d, want %d", gotImplemented, run.Implemented)
	}
	if gotFailed != run.Failed {
		t.Errorf("failed: got %d, want %d", gotFailed, run.Failed)
	}
	if gotStartedAt != started.Format("2006-01-02T15:04:05Z") {
		t.Errorf("started_at: got %q, want %q", gotStartedAt, started.Format("2006-01-02T15:04:05Z"))
	}
	if gotFinishedAt != finished.Format("2006-01-02T15:04:05Z") {
		t.Errorf("finished_at: got %q, want %q", gotFinishedAt, finished.Format("2006-01-02T15:04:05Z"))
	}
}

func TestWriteIssueOutcome_ReadBack(t *testing.T) {
	db := openTestDB(t)

	outcome := IssueOutcomeRecord{
		RunID:       "run-abc",
		IssueNumber: 42,
		Title:       "Fix the bug",
		Status:      "implemented",
		PRNumber:    87,
		Error:       "",
	}

	if err := WriteIssueOutcome(context.Background(), db, outcome); err != nil {
		t.Fatalf("WriteIssueOutcome: %v", err)
	}

	var (
		gotRunID, gotTitle, gotStatus, gotError string
		gotIssueNumber, gotPRNumber             int
	)
	err := db.db.QueryRow(
		`SELECT run_id, issue_number, title, status, pr_number, error
		 FROM issue_outcomes WHERE run_id = ? AND issue_number = ?`,
		outcome.RunID, outcome.IssueNumber,
	).Scan(&gotRunID, &gotIssueNumber, &gotTitle, &gotStatus, &gotPRNumber, &gotError)
	if err != nil {
		t.Fatalf("SELECT outcome: %v", err)
	}

	if gotRunID != outcome.RunID {
		t.Errorf("run_id: got %q, want %q", gotRunID, outcome.RunID)
	}
	if gotIssueNumber != outcome.IssueNumber {
		t.Errorf("issue_number: got %d, want %d", gotIssueNumber, outcome.IssueNumber)
	}
	if gotTitle != outcome.Title {
		t.Errorf("title: got %q, want %q", gotTitle, outcome.Title)
	}
	if gotStatus != outcome.Status {
		t.Errorf("status: got %q, want %q", gotStatus, outcome.Status)
	}
	if gotPRNumber != outcome.PRNumber {
		t.Errorf("pr_number: got %d, want %d", gotPRNumber, outcome.PRNumber)
	}
}

func TestWriteStepResult_WithFlags(t *testing.T) {
	db := openTestDB(t)

	started := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	finished := time.Date(2026, 3, 14, 10, 0, 45, 0, time.UTC)
	step := StepResultRecord{
		RunID:           "run-abc",
		IssueNumber:     42,
		StepName:        "quality-review",
		CostUSD:         0.15,
		DurationSeconds: 45.2,
		Flags:           []string{"low_cost", "no_diff_read"},
		StartedAt:       started,
		FinishedAt:      finished,
	}

	if err := WriteStepResult(context.Background(), db, step); err != nil {
		t.Fatalf("WriteStepResult: %v", err)
	}

	var (
		gotRunID, gotStepName, gotFlagsJSON string
		gotIssueNumber                      int
		gotCostUSD, gotDurationSeconds      float64
	)
	err := db.db.QueryRow(
		`SELECT run_id, issue_number, step_name, cost_usd, duration_seconds, flags
		 FROM step_results WHERE run_id = ? AND issue_number = ? AND step_name = ?`,
		step.RunID, step.IssueNumber, step.StepName,
	).Scan(&gotRunID, &gotIssueNumber, &gotStepName, &gotCostUSD, &gotDurationSeconds, &gotFlagsJSON)
	if err != nil {
		t.Fatalf("SELECT step: %v", err)
	}

	if gotRunID != step.RunID {
		t.Errorf("run_id: got %q, want %q", gotRunID, step.RunID)
	}
	if gotStepName != step.StepName {
		t.Errorf("step_name: got %q, want %q", gotStepName, step.StepName)
	}
	if gotCostUSD != step.CostUSD {
		t.Errorf("cost_usd: got %v, want %v", gotCostUSD, step.CostUSD)
	}
	if gotDurationSeconds != step.DurationSeconds {
		t.Errorf("duration_seconds: got %v, want %v", gotDurationSeconds, step.DurationSeconds)
	}

	// Verify JSON round-trip
	var gotFlags []string
	if err := json.Unmarshal([]byte(gotFlagsJSON), &gotFlags); err != nil {
		t.Fatalf("unmarshal flags JSON %q: %v", gotFlagsJSON, err)
	}
	if len(gotFlags) != len(step.Flags) {
		t.Fatalf("flags length: got %d, want %d", len(gotFlags), len(step.Flags))
	}
	for i, f := range step.Flags {
		if gotFlags[i] != f {
			t.Errorf("flags[%d]: got %q, want %q", i, gotFlags[i], f)
		}
	}
}

func TestWriteRun_Idempotent(t *testing.T) {
	db := openTestDB(t)

	run := RunRecord{
		ID:          "20260314-142305",
		Repo:        "org/repo",
		Implemented: 3,
	}
	if err := WriteRun(context.Background(), db, run); err != nil {
		t.Fatalf("first WriteRun: %v", err)
	}

	run.Implemented = 5
	if err := WriteRun(context.Background(), db, run); err != nil {
		t.Fatalf("second WriteRun: %v", err)
	}

	var count int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM runs WHERE id = ?`, run.ID).Scan(&count); err != nil {
		t.Fatalf("COUNT runs: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}

	var implemented int
	if err := db.db.QueryRow(`SELECT implemented FROM runs WHERE id = ?`, run.ID).Scan(&implemented); err != nil {
		t.Fatalf("SELECT implemented: %v", err)
	}
	if implemented != 5 {
		t.Errorf("implemented: got %d, want 5", implemented)
	}
}

func TestWriteIssueOutcome_Idempotent(t *testing.T) {
	db := openTestDB(t)

	outcome := IssueOutcomeRecord{
		RunID:       "run-1",
		IssueNumber: 42,
		Status:      "failed",
	}
	if err := WriteIssueOutcome(context.Background(), db, outcome); err != nil {
		t.Fatalf("first WriteIssueOutcome: %v", err)
	}

	outcome.Status = "implemented"
	if err := WriteIssueOutcome(context.Background(), db, outcome); err != nil {
		t.Fatalf("second WriteIssueOutcome: %v", err)
	}

	var count int
	if err := db.db.QueryRow(
		`SELECT COUNT(*) FROM issue_outcomes WHERE run_id = ? AND issue_number = ?`,
		outcome.RunID, outcome.IssueNumber,
	).Scan(&count); err != nil {
		t.Fatalf("COUNT issue_outcomes: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}

	var status string
	if err := db.db.QueryRow(
		`SELECT status FROM issue_outcomes WHERE run_id = ? AND issue_number = ?`,
		outcome.RunID, outcome.IssueNumber,
	).Scan(&status); err != nil {
		t.Fatalf("SELECT status: %v", err)
	}
	if status != "implemented" {
		t.Errorf("status: got %q, want %q", status, "implemented")
	}
}

func TestWriteIssueOutcome_MultipleIssuesPerRun(t *testing.T) {
	db := openTestDB(t)

	runID := "run-multi"
	issues := []IssueOutcomeRecord{
		{RunID: runID, IssueNumber: 1, Status: "implemented"},
		{RunID: runID, IssueNumber: 2, Status: "failed"},
		{RunID: runID, IssueNumber: 3, Status: "implemented"},
	}
	for _, o := range issues {
		if err := WriteIssueOutcome(context.Background(), db, o); err != nil {
			t.Fatalf("WriteIssueOutcome issue %d: %v", o.IssueNumber, err)
		}
	}

	var count int
	if err := db.db.QueryRow(
		`SELECT COUNT(*) FROM issue_outcomes WHERE run_id = ?`, runID,
	).Scan(&count); err != nil {
		t.Fatalf("COUNT: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 rows, got %d", count)
	}

	for _, o := range issues {
		var issueNumber int
		if err := db.db.QueryRow(
			`SELECT issue_number FROM issue_outcomes WHERE run_id = ? AND issue_number = ?`,
			runID, o.IssueNumber,
		).Scan(&issueNumber); err != nil {
			t.Errorf("issue %d not found: %v", o.IssueNumber, err)
		}
	}
}

func TestWriteStepResult_MultipleStepsPerIssue(t *testing.T) {
	db := openTestDB(t)

	runID := "run-steps"
	steps := []StepResultRecord{
		{RunID: runID, IssueNumber: 10, StepName: "implement"},
		{RunID: runID, IssueNumber: 10, StepName: "quality-review"},
		{RunID: runID, IssueNumber: 10, StepName: "retry-1"},
	}
	for _, s := range steps {
		if err := WriteStepResult(context.Background(), db, s); err != nil {
			t.Fatalf("WriteStepResult %q: %v", s.StepName, err)
		}
	}

	var count int
	if err := db.db.QueryRow(
		`SELECT COUNT(*) FROM step_results WHERE run_id = ? AND issue_number = ?`,
		runID, 10,
	).Scan(&count); err != nil {
		t.Fatalf("COUNT: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 rows, got %d", count)
	}

	for _, s := range steps {
		var stepName string
		if err := db.db.QueryRow(
			`SELECT step_name FROM step_results WHERE run_id = ? AND issue_number = ? AND step_name = ?`,
			runID, s.IssueNumber, s.StepName,
		).Scan(&stepName); err != nil {
			t.Errorf("step %q not found: %v", s.StepName, err)
		}
	}
}

func TestWriteStepResult_EmptyFlags(t *testing.T) {
	db := openTestDB(t)

	// Test with nil flags
	step := StepResultRecord{
		RunID:       "run-1",
		IssueNumber: 1,
		StepName:    "implement",
		Flags:       nil,
	}
	if err := WriteStepResult(context.Background(), db, step); err != nil {
		t.Fatalf("WriteStepResult with nil flags: %v", err)
	}

	var flagsJSON string
	if err := db.db.QueryRow(
		`SELECT flags FROM step_results WHERE run_id = ? AND issue_number = ? AND step_name = ?`,
		step.RunID, step.IssueNumber, step.StepName,
	).Scan(&flagsJSON); err != nil {
		t.Fatalf("SELECT flags: %v", err)
	}

	if flagsJSON != "[]" {
		t.Errorf("flags JSON: got %q, want %q", flagsJSON, "[]")
	}

	var gotFlags []string
	if err := json.Unmarshal([]byte(flagsJSON), &gotFlags); err != nil {
		t.Fatalf("unmarshal flags: %v", err)
	}
	if len(gotFlags) != 0 {
		t.Errorf("expected empty flags slice, got %v", gotFlags)
	}
}

func TestWriteStepResult_ResourceFields(t *testing.T) {
	db := openTestDB(t)

	step := StepResultRecord{
		RunID:           "run-res",
		IssueNumber:     10,
		StepName:        "implement",
		PeakMemoryBytes: 512 * 1024 * 1024,
		CPUNanoseconds:  3_000_000_000,
	}

	if err := WriteStepResult(context.Background(), db, step); err != nil {
		t.Fatalf("WriteStepResult: %v", err)
	}

	var gotPeakMem, gotCPUNano int64
	err := db.db.QueryRow(
		`SELECT peak_memory_bytes, cpu_nanoseconds FROM step_results
		 WHERE run_id = ? AND issue_number = ? AND step_name = ?`,
		step.RunID, step.IssueNumber, step.StepName,
	).Scan(&gotPeakMem, &gotCPUNano)
	if err != nil {
		t.Fatalf("SELECT resource fields: %v", err)
	}
	if gotPeakMem != step.PeakMemoryBytes {
		t.Errorf("peak_memory_bytes: got %d, want %d", gotPeakMem, step.PeakMemoryBytes)
	}
	if gotCPUNano != step.CPUNanoseconds {
		t.Errorf("cpu_nanoseconds: got %d, want %d", gotCPUNano, step.CPUNanoseconds)
	}
}
