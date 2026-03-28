package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

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
	for _, name := range []string{"repo", "run", "json"} {
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
	if err := runTrace(&buf, db, "42", "", "", false); err != nil {
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
	if err := runTrace(&buf, db, "t-1", "", "", false); err != nil {
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
	if err := runTrace(&buf, db, "42", "", "", false); err != nil {
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
	if err := runTrace(&buf, db, "42", "", "run-old", false); err != nil {
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
	err := runTrace(&buf, db, "999", "", "", false)
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
	err := runTrace(&buf, db, "nonexistent-trace", "", "", false)
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
	if err := runTrace(&buf, db, "42", "", "", true); err != nil {
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
	if err := runTrace(&buf, db, "42", "org/repo-a", "", false); err != nil {
		t.Fatalf("runTrace: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "t-a") {
		t.Errorf("expected trace 't-a' for org/repo-a; got:\n%s", out)
	}
}
