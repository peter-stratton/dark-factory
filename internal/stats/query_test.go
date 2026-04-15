package stats

import (
	"context"
	"testing"
	"time"
)

// Shared test timestamps anchored to dates that make date-range assertions clear.
var (
	ts1 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ts2 = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	ts3 = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
)

// writeRun is a thin wrapper for WriteRun that fails the test on error.
func writeRun(t *testing.T, db *DB, r RunRecord) {
	t.Helper()
	if err := WriteRun(context.Background(), db, r); err != nil {
		t.Fatalf("WriteRun %q: %v", r.ID, err)
	}
}

// writeOutcome is a thin wrapper for WriteIssueOutcome that fails the test on error.
func writeOutcome(t *testing.T, db *DB, o IssueOutcomeRecord) {
	t.Helper()
	if err := WriteIssueOutcome(context.Background(), db, o); err != nil {
		t.Fatalf("WriteIssueOutcome run=%q issue=%d: %v", o.RunID, o.IssueNumber, err)
	}
}

// writeStep is a thin wrapper for WriteStepResult that fails the test on error.
func writeStep(t *testing.T, db *DB, s StepResultRecord) {
	t.Helper()
	if err := WriteStepResult(context.Background(), db, s); err != nil {
		t.Fatalf("WriteStepResult run=%q issue=%d step=%q: %v", s.RunID, s.IssueNumber, s.StepName, err)
	}
}

// TestQueryRuns_NoFilter: write 3 runs across 2 repos, empty filter returns all 3.
func TestQueryRuns_NoFilter(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "org/repo-a", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-2", Repo: "org/repo-a", StartedAt: ts2})
	writeRun(t, db, RunRecord{ID: "run-3", Repo: "org/repo-b", StartedAt: ts3})

	got, err := QueryRuns(context.Background(), db, RunFilter{})
	if err != nil {
		t.Fatalf("QueryRuns: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d runs, want 3", len(got))
	}

	// Verify ascending order.
	for i := 1; i < len(got); i++ {
		if got[i].StartedAt.Before(got[i-1].StartedAt) {
			t.Errorf("run[%d].StartedAt %v before run[%d].StartedAt %v — not sorted ascending",
				i, got[i].StartedAt, i-1, got[i-1].StartedAt)
		}
	}
}

// TestQueryRuns_RepoFilter: filter by "org/repo-a" returns only those runs.
func TestQueryRuns_RepoFilter(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "org/repo-a", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-2", Repo: "org/repo-a", StartedAt: ts2})
	writeRun(t, db, RunRecord{ID: "run-3", Repo: "org/repo-b", StartedAt: ts3})

	got, err := QueryRuns(context.Background(), db, RunFilter{Repo: "org/repo-a"})
	if err != nil {
		t.Fatalf("QueryRuns: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d runs, want 2", len(got))
	}
	for _, r := range got {
		if r.Repo != "org/repo-a" {
			t.Errorf("unexpected repo %q in result", r.Repo)
		}
	}
}

// TestQueryRuns_MilestoneFilter: filter by milestone returns only matching runs.
func TestQueryRuns_MilestoneFilter(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "org/repo", Milestone: "Phase 21", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-2", Repo: "org/repo", Milestone: "Phase 22", StartedAt: ts2})

	got, err := QueryRuns(context.Background(), db, RunFilter{Milestone: "Phase 21"})
	if err != nil {
		t.Fatalf("QueryRuns: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d runs, want 1", len(got))
	}
	if len(got) > 0 && got[0].ID != "run-1" {
		t.Errorf("got run %q, want %q", got[0].ID, "run-1")
	}
}

// TestQueryRuns_DateRange: Since and Until filter to the correct window.
func TestQueryRuns_DateRange(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-jan", Repo: "org/repo", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-feb", Repo: "org/repo", StartedAt: ts2})
	writeRun(t, db, RunRecord{ID: "run-mar", Repo: "org/repo", StartedAt: ts3})

	tests := []struct {
		name      string
		filter    RunFilter
		wantCount int
		wantIDs   []string
	}{
		{
			name:      "Since Feb",
			filter:    RunFilter{Since: ts2},
			wantCount: 2,
			wantIDs:   []string{"run-feb", "run-mar"},
		},
		{
			name:      "Until Feb",
			filter:    RunFilter{Until: ts2},
			wantCount: 2,
			wantIDs:   []string{"run-jan", "run-feb"},
		},
		{
			name:      "Since Feb Until Feb",
			filter:    RunFilter{Since: ts2, Until: ts2},
			wantCount: 1,
			wantIDs:   []string{"run-feb"},
		},
		{
			name:      "Since Jan Until Mar",
			filter:    RunFilter{Since: ts1, Until: ts3},
			wantCount: 3,
			wantIDs:   []string{"run-jan", "run-feb", "run-mar"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := QueryRuns(context.Background(), db, tc.filter)
			if err != nil {
				t.Fatalf("QueryRuns: %v", err)
			}
			if len(got) != tc.wantCount {
				t.Errorf("got %d runs, want %d", len(got), tc.wantCount)
				return
			}
			for i, id := range tc.wantIDs {
				if got[i].ID != id {
					t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}

// TestQueryRuns_EmptyResult: querying with a non-existent repo returns empty slice, not error.
func TestQueryRuns_EmptyResult(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "org/repo-a", StartedAt: ts1})

	got, err := QueryRuns(context.Background(), db, RunFilter{Repo: "org/nonexistent"})
	if err != nil {
		t.Fatalf("QueryRuns: %v", err)
	}
	if got == nil {
		t.Error("got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("got %d runs, want 0", len(got))
	}
}

// TestQueryIssueOutcomes_Join: write 2 runs with 3 outcomes each; filter by repo
// returns only matching outcomes.
func TestQueryIssueOutcomes_Join(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-a", Repo: "org/repo-a", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-b", Repo: "org/repo-b", StartedAt: ts2})

	// 3 outcomes for repo-a.
	for i := 1; i <= 3; i++ {
		writeOutcome(t, db, IssueOutcomeRecord{RunID: "run-a", IssueNumber: i, Status: "implemented"})
	}
	// 3 outcomes for repo-b.
	for i := 4; i <= 6; i++ {
		writeOutcome(t, db, IssueOutcomeRecord{RunID: "run-b", IssueNumber: i, Status: "failed"})
	}

	got, err := QueryIssueOutcomes(context.Background(), db, RunFilter{Repo: "org/repo-a"})
	if err != nil {
		t.Fatalf("QueryIssueOutcomes: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d outcomes, want 3", len(got))
	}
	for _, o := range got {
		if o.RunID != "run-a" {
			t.Errorf("unexpected run_id %q in result", o.RunID)
		}
	}
}

// TestQueryIssueOutcomes_NoFilter: empty filter returns all outcomes.
func TestQueryIssueOutcomes_NoFilter(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-a", Repo: "org/repo-a", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-b", Repo: "org/repo-b", StartedAt: ts2})

	for i := 1; i <= 3; i++ {
		writeOutcome(t, db, IssueOutcomeRecord{RunID: "run-a", IssueNumber: i})
		writeOutcome(t, db, IssueOutcomeRecord{RunID: "run-b", IssueNumber: i})
	}

	got, err := QueryIssueOutcomes(context.Background(), db, RunFilter{})
	if err != nil {
		t.Fatalf("QueryIssueOutcomes: %v", err)
	}
	if len(got) != 6 {
		t.Errorf("got %d outcomes, want 6", len(got))
	}
}

// TestQueryIssueOutcomes_EmptyResult: non-matching repo returns empty slice.
func TestQueryIssueOutcomes_EmptyResult(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-a", Repo: "org/repo-a", StartedAt: ts1})
	writeOutcome(t, db, IssueOutcomeRecord{RunID: "run-a", IssueNumber: 1})

	got, err := QueryIssueOutcomes(context.Background(), db, RunFilter{Repo: "org/nonexistent"})
	if err != nil {
		t.Fatalf("QueryIssueOutcomes: %v", err)
	}
	if got == nil {
		t.Error("got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("got %d outcomes, want 0", len(got))
	}
}

// TestQueryStepResults_Join: write step results for 2 runs; filter by repo returns
// only matching steps.
func TestQueryStepResults_Join(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-a", Repo: "org/repo-a", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-b", Repo: "org/repo-b", StartedAt: ts2})

	steps := []string{"implement", "quality-review", "retry-1"}
	for _, s := range steps {
		writeStep(t, db, StepResultRecord{RunID: "run-a", IssueNumber: 1, StepName: s})
		writeStep(t, db, StepResultRecord{RunID: "run-b", IssueNumber: 1, StepName: s})
	}

	got, err := QueryStepResults(context.Background(), db, RunFilter{Repo: "org/repo-a"})
	if err != nil {
		t.Fatalf("QueryStepResults: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d step results, want 3", len(got))
	}
	for _, s := range got {
		if s.RunID != "run-a" {
			t.Errorf("unexpected run_id %q in result", s.RunID)
		}
	}
}

// TestQueryStepResults_NoFilter: empty filter returns all step results.
func TestQueryStepResults_NoFilter(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-a", Repo: "org/repo-a", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-b", Repo: "org/repo-b", StartedAt: ts2})

	writeStep(t, db, StepResultRecord{RunID: "run-a", IssueNumber: 1, StepName: "implement"})
	writeStep(t, db, StepResultRecord{RunID: "run-a", IssueNumber: 1, StepName: "quality-review"})
	writeStep(t, db, StepResultRecord{RunID: "run-b", IssueNumber: 1, StepName: "implement"})

	got, err := QueryStepResults(context.Background(), db, RunFilter{})
	if err != nil {
		t.Fatalf("QueryStepResults: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d step results, want 3", len(got))
	}
}

// TestQueryStepResults_EmptyResult: non-matching repo returns empty slice.
func TestQueryStepResults_EmptyResult(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-a", Repo: "org/repo-a", StartedAt: ts1})
	writeStep(t, db, StepResultRecord{RunID: "run-a", IssueNumber: 1, StepName: "implement"})

	got, err := QueryStepResults(context.Background(), db, RunFilter{Repo: "org/nonexistent"})
	if err != nil {
		t.Fatalf("QueryStepResults: %v", err)
	}
	if got == nil {
		t.Error("got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("got %d step results, want 0", len(got))
	}
}

// TestQueryStepResults_FlagsRoundTrip: flags survive write-then-query.
func TestQueryStepResults_FlagsRoundTrip(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "org/repo", StartedAt: ts1})
	writeStep(t, db, StepResultRecord{
		RunID:       "run-1",
		IssueNumber: 10,
		StepName:    "quality-review",
		Flags:       []string{"low_cost", "no_diff_read"},
	})

	got, err := QueryStepResults(context.Background(), db, RunFilter{})
	if err != nil {
		t.Fatalf("QueryStepResults: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d step results, want 1", len(got))
	}

	want := []string{"low_cost", "no_diff_read"}
	if len(got[0].Flags) != len(want) {
		t.Fatalf("flags length: got %d, want %d", len(got[0].Flags), len(want))
	}
	for i, f := range want {
		if got[0].Flags[i] != f {
			t.Errorf("flags[%d]: got %q, want %q", i, got[0].Flags[i], f)
		}
	}
}

// TestQueryRuns_FieldsRoundTrip: all RunRecord fields survive write-then-query.
func TestQueryRuns_FieldsRoundTrip(t *testing.T) {
	db := openTestDB(t)

	startedAt := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 3, 14, 11, 30, 0, 0, time.UTC)
	want := RunRecord{
		ID:               "20260314-100000",
		Repo:             "org/repo",
		Milestone:        "Phase 21",
		BaseBranch:       "main",
		AutoMergeFeature: "low_risk",
		AutoMergeRollup:  "auto",
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
		Total:            5,
		Implemented:      4,
		Failed:           1,
		AbortReason:      "timeout",
	}
	writeRun(t, db, want)

	got, err := QueryRuns(context.Background(), db, RunFilter{})
	if err != nil {
		t.Fatalf("QueryRuns: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d runs, want 1", len(got))
	}
	r := got[0]
	if r.ID != want.ID {
		t.Errorf("ID: got %q, want %q", r.ID, want.ID)
	}
	if r.Repo != want.Repo {
		t.Errorf("Repo: got %q, want %q", r.Repo, want.Repo)
	}
	if r.Milestone != want.Milestone {
		t.Errorf("Milestone: got %q, want %q", r.Milestone, want.Milestone)
	}
	if r.BaseBranch != want.BaseBranch {
		t.Errorf("BaseBranch: got %q, want %q", r.BaseBranch, want.BaseBranch)
	}
	if r.AutoMergeFeature != want.AutoMergeFeature {
		t.Errorf("AutoMergeFeature: got %q, want %q", r.AutoMergeFeature, want.AutoMergeFeature)
	}
	if r.AutoMergeRollup != want.AutoMergeRollup {
		t.Errorf("AutoMergeRollup: got %q, want %q", r.AutoMergeRollup, want.AutoMergeRollup)
	}
	if !r.StartedAt.Equal(want.StartedAt) {
		t.Errorf("StartedAt: got %v, want %v", r.StartedAt, want.StartedAt)
	}
	if !r.FinishedAt.Equal(want.FinishedAt) {
		t.Errorf("FinishedAt: got %v, want %v", r.FinishedAt, want.FinishedAt)
	}
	if r.Total != want.Total {
		t.Errorf("Total: got %d, want %d", r.Total, want.Total)
	}
	if r.Implemented != want.Implemented {
		t.Errorf("Implemented: got %d, want %d", r.Implemented, want.Implemented)
	}
	if r.Failed != want.Failed {
		t.Errorf("Failed: got %d, want %d", r.Failed, want.Failed)
	}
	if r.AbortReason != want.AbortReason {
		t.Errorf("AbortReason: got %q, want %q", r.AbortReason, want.AbortReason)
	}
}

// TestQueryStepResults_ResourceFieldsRoundTrip: PeakMemoryBytes and CPUNanoseconds
// survive write-then-query.
func TestQueryStepResults_ResourceFieldsRoundTrip(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "org/repo", StartedAt: ts1})
	writeStep(t, db, StepResultRecord{
		RunID:           "run-1",
		IssueNumber:     42,
		StepName:        "implement",
		PeakMemoryBytes: 256 * 1024 * 1024,
		CPUNanoseconds:  2_500_000_000,
	})

	got, err := QueryStepResults(context.Background(), db, RunFilter{})
	if err != nil {
		t.Fatalf("QueryStepResults: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d step results, want 1", len(got))
	}

	s := got[0]
	if s.PeakMemoryBytes != 256*1024*1024 {
		t.Errorf("PeakMemoryBytes: got %d, want %d", s.PeakMemoryBytes, 256*1024*1024)
	}
	if s.CPUNanoseconds != 2_500_000_000 {
		t.Errorf("CPUNanoseconds: got %d, want %d", s.CPUNanoseconds, 2_500_000_000)
	}
}

// TestQueryStepsByTraceID: seed steps with trace_id, verify correct filtering.
func TestQueryStepsByTraceID(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "org/repo", StartedAt: ts1})
	writeStep(t, db, StepResultRecord{
		RunID: "run-1", IssueNumber: 42, StepName: "implement",
		TraceID: "t-1", StartedAt: ts1, FinishedAt: ts1,
	})
	writeStep(t, db, StepResultRecord{
		RunID: "run-1", IssueNumber: 42, StepName: "quality-review",
		TraceID: "t-1", StartedAt: ts2, FinishedAt: ts2,
	})
	writeStep(t, db, StepResultRecord{
		RunID: "run-1", IssueNumber: 99, StepName: "implement",
		TraceID: "t-other", StartedAt: ts1, FinishedAt: ts1,
	})

	got, err := QueryStepsByTraceID(context.Background(), db, "t-1")
	if err != nil {
		t.Fatalf("QueryStepsByTraceID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d steps, want 2", len(got))
	}
	if got[0].StepName != "implement" {
		t.Errorf("steps[0].StepName = %q, want %q", got[0].StepName, "implement")
	}
	if got[1].StepName != "quality-review" {
		t.Errorf("steps[1].StepName = %q, want %q", got[1].StepName, "quality-review")
	}
}

// TestQueryStepsByTraceID_NotFound: non-existent trace ID returns empty slice.
func TestQueryStepsByTraceID_NotFound(t *testing.T) {
	db := openTestDB(t)

	got, err := QueryStepsByTraceID(context.Background(), db, "nonexistent")
	if err != nil {
		t.Fatalf("QueryStepsByTraceID: %v", err)
	}
	if got == nil {
		t.Error("got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("got %d steps, want 0", len(got))
	}
}

// TestQueryOutcomeByTraceID: found and not-found cases.
func TestQueryOutcomeByTraceID(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "org/repo", StartedAt: ts1})
	writeOutcome(t, db, IssueOutcomeRecord{
		RunID: "run-1", IssueNumber: 42, Status: "implemented", TraceID: "t-1",
	})

	got, err := QueryOutcomeByTraceID(context.Background(), db, "t-1")
	if err != nil {
		t.Fatalf("QueryOutcomeByTraceID: %v", err)
	}
	if got == nil {
		t.Fatal("got nil, want non-nil outcome")
	}
	if got.IssueNumber != 42 {
		t.Errorf("IssueNumber = %d, want 42", got.IssueNumber)
	}
	if got.Status != "implemented" {
		t.Errorf("Status = %q, want %q", got.Status, "implemented")
	}
}

// TestQueryOutcomeByTraceID_NotFound: returns nil for non-existent trace ID.
func TestQueryOutcomeByTraceID_NotFound(t *testing.T) {
	db := openTestDB(t)

	got, err := QueryOutcomeByTraceID(context.Background(), db, "nonexistent")
	if err != nil {
		t.Fatalf("QueryOutcomeByTraceID: %v", err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

// TestQueryLatestTraceForIssue: multiple runs, verify most recent returned.
func TestQueryLatestTraceForIssue(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-old", Repo: "org/repo", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-new", Repo: "org/repo", StartedAt: ts3})

	writeOutcome(t, db, IssueOutcomeRecord{
		RunID: "run-old", IssueNumber: 42, Status: "failed", TraceID: "t-old",
	})
	writeOutcome(t, db, IssueOutcomeRecord{
		RunID: "run-new", IssueNumber: 42, Status: "implemented", TraceID: "t-new",
	})

	got, err := QueryLatestTraceForIssue(context.Background(), db, 42, "", "")
	if err != nil {
		t.Fatalf("QueryLatestTraceForIssue: %v", err)
	}
	if got != "t-new" {
		t.Errorf("got %q, want %q", got, "t-new")
	}
}

// TestQueryLatestTraceForIssue_RepoFilter: repo filter narrows results.
func TestQueryLatestTraceForIssue_RepoFilter(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-a", Repo: "org/repo-a", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-b", Repo: "org/repo-b", StartedAt: ts3})

	writeOutcome(t, db, IssueOutcomeRecord{
		RunID: "run-a", IssueNumber: 42, Status: "implemented", TraceID: "t-a",
	})
	writeOutcome(t, db, IssueOutcomeRecord{
		RunID: "run-b", IssueNumber: 42, Status: "implemented", TraceID: "t-b",
	})

	got, err := QueryLatestTraceForIssue(context.Background(), db, 42, "org/repo-a", "")
	if err != nil {
		t.Fatalf("QueryLatestTraceForIssue: %v", err)
	}
	if got != "t-a" {
		t.Errorf("got %q, want %q", got, "t-a")
	}
}

// TestQueryLatestTraceForIssue_RunIDFilter: run_id filter narrows results.
func TestQueryLatestTraceForIssue_RunIDFilter(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-old", Repo: "org/repo", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-new", Repo: "org/repo", StartedAt: ts3})

	writeOutcome(t, db, IssueOutcomeRecord{
		RunID: "run-old", IssueNumber: 42, Status: "failed", TraceID: "t-old",
	})
	writeOutcome(t, db, IssueOutcomeRecord{
		RunID: "run-new", IssueNumber: 42, Status: "implemented", TraceID: "t-new",
	})

	got, err := QueryLatestTraceForIssue(context.Background(), db, 42, "", "run-old")
	if err != nil {
		t.Fatalf("QueryLatestTraceForIssue: %v", err)
	}
	if got != "t-old" {
		t.Errorf("got %q, want %q", got, "t-old")
	}
}

// TestQueryLatestTraceForIssue_NotFound: returns empty string for non-existent issue.
func TestQueryLatestTraceForIssue_NotFound(t *testing.T) {
	db := openTestDB(t)

	got, err := QueryLatestTraceForIssue(context.Background(), db, 999, "", "")
	if err != nil {
		t.Fatalf("QueryLatestTraceForIssue: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

// TestQueryLatestTraceForIssue_SkipsEmptyTraceID: rows with empty trace_id are ignored.
func TestQueryLatestTraceForIssue_SkipsEmptyTraceID(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "org/repo", StartedAt: ts1})
	writeOutcome(t, db, IssueOutcomeRecord{
		RunID: "run-1", IssueNumber: 42, Status: "implemented", TraceID: "",
	})

	got, err := QueryLatestTraceForIssue(context.Background(), db, 42, "", "")
	if err != nil {
		t.Fatalf("QueryLatestTraceForIssue: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string (should skip empty trace_id)", got)
	}
}

// TestQueryRunByID_Found: insert a run, query by ID, verify fields match.
func TestQueryRunByID_Found(t *testing.T) {
	db := openTestDB(t)

	want := RunRecord{
		ID:               "run-1",
		Repo:             "org/repo",
		Milestone:        "Phase 1",
		BaseBranch:       "main",
		AutoMergeFeature: "true",
		AutoMergeRollup:  "false",
		StartedAt:        ts1,
		FinishedAt:       ts2,
		Total:            5,
		Implemented:      3,
		Failed:           2,
		AbortReason:      "timeout",
	}
	writeRun(t, db, want)

	got, err := QueryRunByID(context.Background(), db, "run-1")
	if err != nil {
		t.Fatalf("QueryRunByID: %v", err)
	}
	if got == nil {
		t.Fatal("got nil, want non-nil RunRecord")
	}
	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
	if got.Repo != want.Repo {
		t.Errorf("Repo = %q, want %q", got.Repo, want.Repo)
	}
	if got.Milestone != want.Milestone {
		t.Errorf("Milestone = %q, want %q", got.Milestone, want.Milestone)
	}
	if got.Total != want.Total {
		t.Errorf("Total = %d, want %d", got.Total, want.Total)
	}
	if got.Implemented != want.Implemented {
		t.Errorf("Implemented = %d, want %d", got.Implemented, want.Implemented)
	}
	if got.Failed != want.Failed {
		t.Errorf("Failed = %d, want %d", got.Failed, want.Failed)
	}
	if got.AbortReason != want.AbortReason {
		t.Errorf("AbortReason = %q, want %q", got.AbortReason, want.AbortReason)
	}
	if !got.StartedAt.Equal(want.StartedAt) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, want.StartedAt)
	}
	if !got.FinishedAt.Equal(want.FinishedAt) {
		t.Errorf("FinishedAt = %v, want %v", got.FinishedAt, want.FinishedAt)
	}
}

// TestQueryRunByID_NotFound: query a nonexistent run ID, verify nil return.
func TestQueryRunByID_NotFound(t *testing.T) {
	db := openTestDB(t)

	got, err := QueryRunByID(context.Background(), db, "nonexistent")
	if err != nil {
		t.Fatalf("QueryRunByID: %v", err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

// TestQueryRuns_ExcludeRepo: ExcludeRepo filters out matching repos.
func TestQueryRuns_ExcludeRepo(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "org/repo-a", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-2", Repo: "org/repo-b", StartedAt: ts2})
	writeRun(t, db, RunRecord{ID: "run-3", Repo: "org/repo-c", StartedAt: ts3})

	got, err := QueryRuns(context.Background(), db, RunFilter{
		ExcludeRepo: []string{"org/repo-a", "org/repo-b"},
	})
	if err != nil {
		t.Fatalf("QueryRuns: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d runs, want 1", len(got))
	}
	if got[0].ID != "run-3" {
		t.Errorf("got run %q, want %q", got[0].ID, "run-3")
	}
}

// TestQueryRuns_ExcludeMilestone: ExcludeMilestone filters out matching milestones.
func TestQueryRuns_ExcludeMilestone(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "org/repo", Milestone: "test-milestone", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-2", Repo: "org/repo", Milestone: "Phase 21", StartedAt: ts2})

	got, err := QueryRuns(context.Background(), db, RunFilter{
		ExcludeMilestone: []string{"test-milestone"},
	})
	if err != nil {
		t.Fatalf("QueryRuns: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d runs, want 1", len(got))
	}
	if got[0].ID != "run-2" {
		t.Errorf("got run %q, want %q", got[0].ID, "run-2")
	}
}

// TestQueryRuns_ExcludeRepoAndMilestone: both exclusions are AND'd together.
func TestQueryRuns_ExcludeRepoAndMilestone(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-1", Repo: "owner/repo", Milestone: "Phase 21", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-2", Repo: "org/real", Milestone: "test-milestone", StartedAt: ts2})
	writeRun(t, db, RunRecord{ID: "run-3", Repo: "org/real", Milestone: "Phase 21", StartedAt: ts3})

	got, err := QueryRuns(context.Background(), db, RunFilter{
		ExcludeRepo:      []string{"owner/repo"},
		ExcludeMilestone: []string{"test-milestone"},
	})
	if err != nil {
		t.Fatalf("QueryRuns: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d runs, want 1", len(got))
	}
	if got[0].ID != "run-3" {
		t.Errorf("got run %q, want %q", got[0].ID, "run-3")
	}
}

// TestQueryIssueOutcomes_ExcludeRepo: exclusion works through the join query.
func TestQueryIssueOutcomes_ExcludeRepo(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-a", Repo: "owner/repo", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-b", Repo: "org/real", StartedAt: ts2})

	writeOutcome(t, db, IssueOutcomeRecord{RunID: "run-a", IssueNumber: 1, Status: "implemented"})
	writeOutcome(t, db, IssueOutcomeRecord{RunID: "run-b", IssueNumber: 2, Status: "implemented"})

	got, err := QueryIssueOutcomes(context.Background(), db, RunFilter{
		ExcludeRepo: []string{"owner/repo"},
	})
	if err != nil {
		t.Fatalf("QueryIssueOutcomes: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d outcomes, want 1", len(got))
	}
	if got[0].RunID != "run-b" {
		t.Errorf("got run_id %q, want %q", got[0].RunID, "run-b")
	}
}

// TestQueryStepResults_ExcludeMilestone: exclusion works through the join query.
func TestQueryStepResults_ExcludeMilestone(t *testing.T) {
	db := openTestDB(t)

	writeRun(t, db, RunRecord{ID: "run-a", Repo: "org/repo", Milestone: "test-milestone", StartedAt: ts1})
	writeRun(t, db, RunRecord{ID: "run-b", Repo: "org/repo", Milestone: "Phase 21", StartedAt: ts2})

	writeStep(t, db, StepResultRecord{RunID: "run-a", IssueNumber: 1, StepName: "implement"})
	writeStep(t, db, StepResultRecord{RunID: "run-b", IssueNumber: 2, StepName: "implement"})

	got, err := QueryStepResults(context.Background(), db, RunFilter{
		ExcludeMilestone: []string{"test-milestone"},
	})
	if err != nil {
		t.Fatalf("QueryStepResults: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d step results, want 1", len(got))
	}
	if got[0].RunID != "run-b" {
		t.Errorf("got run_id %q, want %q", got[0].RunID, "run-b")
	}
}
