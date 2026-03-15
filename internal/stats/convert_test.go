package stats

import (
	"testing"
	"time"
)

// TestToRunDetails_Empty: empty runs slice returns nil.
func TestToRunDetails_Empty(t *testing.T) {
	got := ToRunDetails(nil, nil, nil)
	if got != nil {
		t.Errorf("ToRunDetails(nil, nil, nil) = %v, want nil", got)
	}
}

// TestToRunDetails_BasicConversion: single run with one outcome round-trips correctly.
func TestToRunDetails_BasicConversion(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	finished := time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC)

	runs := []RunRecord{
		{
			ID:          "run-1",
			Repo:        "org/repo",
			Milestone:   "Phase 21",
			BaseBranch:  "main",
			StartedAt:   ts,
			FinishedAt:  finished,
			Total:       2,
			Implemented: 1,
			Failed:      1,
		},
	}
	outcomes := []IssueOutcomeRecord{
		{RunID: "run-1", IssueNumber: 10, Title: "Fix bug", Status: "implemented", PRNumber: 99},
		{RunID: "run-1", IssueNumber: 11, Title: "Add feature", Status: "failed"},
	}

	got := ToRunDetails(runs, outcomes, nil)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	rd := got[0]
	if rd.Repo != "org/repo" {
		t.Errorf("Repo = %q, want %q", rd.Repo, "org/repo")
	}
	if rd.Milestone != "Phase 21" {
		t.Errorf("Milestone = %q, want %q", rd.Milestone, "Phase 21")
	}
	if rd.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want %q", rd.BaseBranch, "main")
	}
	if !rd.StartedAt.Equal(ts) {
		t.Errorf("StartedAt = %v, want %v", rd.StartedAt, ts)
	}
	if rd.FinishedAt == nil || !rd.FinishedAt.Equal(finished) {
		t.Errorf("FinishedAt = %v, want %v", rd.FinishedAt, finished)
	}
	if rd.Summary == nil {
		t.Fatal("Summary is nil, want non-nil")
	}
	if rd.Summary.Total != 2 {
		t.Errorf("Summary.Total = %d, want 2", rd.Summary.Total)
	}

	if len(rd.Issues) != 2 {
		t.Fatalf("len(rd.Issues) = %d, want 2", len(rd.Issues))
	}
	// Issues are sorted by issue number.
	if rd.Issues[0].IssueNumber != 10 {
		t.Errorf("Issues[0].IssueNumber = %d, want 10", rd.Issues[0].IssueNumber)
	}
	if rd.Issues[0].Outcome.Status != "implemented" {
		t.Errorf("Issues[0].Outcome.Status = %q, want %q", rd.Issues[0].Outcome.Status, "implemented")
	}
	if rd.Issues[0].Outcome.PRNumber != 99 {
		t.Errorf("Issues[0].Outcome.PRNumber = %d, want 99", rd.Issues[0].Outcome.PRNumber)
	}
	if rd.Issues[1].IssueNumber != 11 {
		t.Errorf("Issues[1].IssueNumber = %d, want 11", rd.Issues[1].IssueNumber)
	}
	if rd.Issues[1].Outcome.Status != "failed" {
		t.Errorf("Issues[1].Outcome.Status = %q, want %q", rd.Issues[1].Outcome.Status, "failed")
	}
}

// TestToRunDetails_StepMapping: step names map to the correct IssueDetail fields.
func TestToRunDetails_StepMapping(t *testing.T) {
	ts := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	stepStarted := time.Date(2026, 2, 1, 10, 1, 0, 0, time.UTC)

	runs := []RunRecord{
		{ID: "run-1", Repo: "org/repo", StartedAt: ts},
	}
	outcomes := []IssueOutcomeRecord{
		{RunID: "run-1", IssueNumber: 1, Status: "implemented"},
	}
	steps := []StepResultRecord{
		{RunID: "run-1", IssueNumber: 1, StepName: "recon", CostUSD: 0.01, StartedAt: stepStarted},
		{RunID: "run-1", IssueNumber: 1, StepName: "spec-generator", CostUSD: 0.02, StartedAt: stepStarted},
		{RunID: "run-1", IssueNumber: 1, StepName: "implement", CostUSD: 0.10, StartedAt: stepStarted},
		{RunID: "run-1", IssueNumber: 1, StepName: "quality-review", CostUSD: 0.05, StartedAt: stepStarted},
		{RunID: "run-1", IssueNumber: 1, StepName: "functional-review", CostUSD: 0.03, StartedAt: stepStarted},
	}

	got := ToRunDetails(runs, outcomes, steps)
	if len(got) != 1 || len(got[0].Issues) != 1 {
		t.Fatalf("unexpected structure: %d runs, %v issues", len(got), len(got[0].Issues))
	}

	issue := got[0].Issues[0]

	if issue.Recon.CostUSD != 0.01 {
		t.Errorf("Recon.CostUSD = %v, want 0.01", issue.Recon.CostUSD)
	}
	if issue.Recon.StartedAt == nil {
		t.Error("Recon.StartedAt is nil, want non-nil")
	}
	if issue.SpecGenerator.CostUSD != 0.02 {
		t.Errorf("SpecGenerator.CostUSD = %v, want 0.02", issue.SpecGenerator.CostUSD)
	}
	if issue.SpecGenerator.StartedAt == nil {
		t.Error("SpecGenerator.StartedAt is nil (needed for DetectGaps)")
	}
	if issue.Implement.CostUSD != 0.10 {
		t.Errorf("Implement.CostUSD = %v, want 0.10", issue.Implement.CostUSD)
	}
	if issue.QualityReview.CostUSD != 0.05 {
		t.Errorf("QualityReview.CostUSD = %v, want 0.05", issue.QualityReview.CostUSD)
	}
	if issue.QualityReview.StartedAt == nil {
		t.Error("QualityReview.StartedAt is nil (needed for DetectGaps)")
	}
	if issue.FunctionalReview.CostUSD != 0.03 {
		t.Errorf("FunctionalReview.CostUSD = %v, want 0.03", issue.FunctionalReview.CostUSD)
	}
}

// TestToRunDetails_RetryMapping: retry step names map to IssueDetail.Retries.
func TestToRunDetails_RetryMapping(t *testing.T) {
	ts := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)

	runs := []RunRecord{{ID: "run-1", Repo: "org/repo", StartedAt: ts}}
	outcomes := []IssueOutcomeRecord{{RunID: "run-1", IssueNumber: 1, Status: "needs-human-review"}}
	steps := []StepResultRecord{
		{RunID: "run-1", IssueNumber: 1, StepName: "implement", CostUSD: 0.10},
		{RunID: "run-1", IssueNumber: 1, StepName: "retry-1", CostUSD: 0.08},
		{RunID: "run-1", IssueNumber: 1, StepName: "retry-1-quality-review", CostUSD: 0.04},
		{RunID: "run-1", IssueNumber: 1, StepName: "retry-1-functional-review", CostUSD: 0.03},
		{RunID: "run-1", IssueNumber: 1, StepName: "retry-2", CostUSD: 0.07},
		{RunID: "run-1", IssueNumber: 1, StepName: "retry-2-quality-review", CostUSD: 0.04},
	}

	got := ToRunDetails(runs, outcomes, steps)
	if len(got) != 1 || len(got[0].Issues) != 1 {
		t.Fatalf("unexpected structure")
	}

	issue := got[0].Issues[0]
	if len(issue.Retries) != 2 {
		t.Fatalf("len(Retries) = %d, want 2", len(issue.Retries))
	}

	r1 := issue.Retries[0]
	if r1.Attempt != 1 {
		t.Errorf("Retries[0].Attempt = %d, want 1", r1.Attempt)
	}
	if r1.Retry.CostUSD != 0.08 {
		t.Errorf("Retries[0].Retry.CostUSD = %v, want 0.08", r1.Retry.CostUSD)
	}
	if r1.QualityReview.CostUSD != 0.04 {
		t.Errorf("Retries[0].QualityReview.CostUSD = %v, want 0.04", r1.QualityReview.CostUSD)
	}
	if r1.FunctionalReview.CostUSD != 0.03 {
		t.Errorf("Retries[0].FunctionalReview.CostUSD = %v, want 0.03", r1.FunctionalReview.CostUSD)
	}

	r2 := issue.Retries[1]
	if r2.Attempt != 2 {
		t.Errorf("Retries[1].Attempt = %d, want 2", r2.Attempt)
	}
	if r2.Retry.CostUSD != 0.07 {
		t.Errorf("Retries[1].Retry.CostUSD = %v, want 0.07", r2.Retry.CostUSD)
	}
	if r2.QualityReview.CostUSD != 0.04 {
		t.Errorf("Retries[1].QualityReview.CostUSD = %v, want 0.04", r2.QualityReview.CostUSD)
	}
}

// TestToRunDetails_FlagsConversion: flag codes convert to rundata.Flag structs.
func TestToRunDetails_FlagsConversion(t *testing.T) {
	ts := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)

	runs := []RunRecord{{ID: "run-1", Repo: "org/repo", StartedAt: ts}}
	outcomes := []IssueOutcomeRecord{{RunID: "run-1", IssueNumber: 1, Status: "implemented"}}
	steps := []StepResultRecord{
		{
			RunID:       "run-1",
			IssueNumber: 1,
			StepName:    "quality-review",
			Flags:       []string{"low_cost", "no_diff_read"},
		},
	}

	got := ToRunDetails(runs, outcomes, steps)
	issue := got[0].Issues[0]

	if len(issue.QualityReview.Flags) != 2 {
		t.Fatalf("len(QualityReview.Flags) = %d, want 2", len(issue.QualityReview.Flags))
	}
	if issue.QualityReview.Flags[0].Code != "low_cost" {
		t.Errorf("Flags[0].Code = %q, want %q", issue.QualityReview.Flags[0].Code, "low_cost")
	}
	if issue.QualityReview.Flags[1].Code != "no_diff_read" {
		t.Errorf("Flags[1].Code = %q, want %q", issue.QualityReview.Flags[1].Code, "no_diff_read")
	}
}

// TestToRunDetails_MultipleRuns: multiple runs each get their own RunDetail.
func TestToRunDetails_MultipleRuns(t *testing.T) {
	ts1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)

	runs := []RunRecord{
		{ID: "run-1", Repo: "org/repo-a", StartedAt: ts1},
		{ID: "run-2", Repo: "org/repo-b", StartedAt: ts2},
	}
	outcomes := []IssueOutcomeRecord{
		{RunID: "run-1", IssueNumber: 1, Status: "implemented"},
		{RunID: "run-2", IssueNumber: 2, Status: "failed"},
	}

	got := ToRunDetails(runs, outcomes, nil)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Repo != "org/repo-a" {
		t.Errorf("got[0].Repo = %q, want %q", got[0].Repo, "org/repo-a")
	}
	if len(got[0].Issues) != 1 || got[0].Issues[0].IssueNumber != 1 {
		t.Errorf("got[0] issues unexpected: %+v", got[0].Issues)
	}
	if got[1].Repo != "org/repo-b" {
		t.Errorf("got[1].Repo = %q, want %q", got[1].Repo, "org/repo-b")
	}
	if len(got[1].Issues) != 1 || got[1].Issues[0].IssueNumber != 2 {
		t.Errorf("got[1] issues unexpected: %+v", got[1].Issues)
	}
}

// TestToRunDetails_ZeroFinishedAt: zero FinishedAt maps to nil RunMeta.FinishedAt.
func TestToRunDetails_ZeroFinishedAt(t *testing.T) {
	ts := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	runs := []RunRecord{{ID: "run-1", Repo: "org/repo", StartedAt: ts}}
	got := ToRunDetails(runs, nil, nil)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].FinishedAt != nil {
		t.Errorf("FinishedAt = %v, want nil", got[0].FinishedAt)
	}
}

// TestParseRetryStepName covers valid and invalid inputs.
func TestParseRetryStepName(t *testing.T) {
	tests := []struct {
		name    string
		wantN   int
		wantSub string
		wantOK  bool
	}{
		{"retry-1", 1, "", true},
		{"retry-2", 2, "", true},
		{"retry-1-quality-review", 1, "quality-review", true},
		{"retry-3-functional-review", 3, "functional-review", true},
		{"implement", 0, "", false},
		{"quality-review", 0, "", false},
		{"retry-", 0, "", false},
		{"retry-abc", 0, "", false},
		{"retry-1-unknown", 0, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, sub, ok := parseRetryStepName(tt.name)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if !ok {
				return
			}
			if n != tt.wantN {
				t.Errorf("n = %d, want %d", n, tt.wantN)
			}
			if sub != tt.wantSub {
				t.Errorf("sub = %q, want %q", sub, tt.wantSub)
			}
		})
	}
}
