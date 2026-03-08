package analysis

import (
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
)

func ptr(t time.Time) *time.Time { return &t }

func TestComputeTrendsNilInput(t *testing.T) {
	got := ComputeTrends(nil)
	if got != nil {
		t.Errorf("ComputeTrends(nil) = %v, want nil", got)
	}
}

func TestComputeTrendsEmptyInput(t *testing.T) {
	got := ComputeTrends([]rundata.RunDetail{})
	if got != nil {
		t.Errorf("ComputeTrends([]) = %v, want nil", got)
	}
}

func TestComputeTrendsChronologicalOrder(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	runs := []rundata.RunDetail{
		{RunMeta: rundata.RunMeta{StartedAt: t2, FinishedAt: ptr(t2)}},
		{RunMeta: rundata.RunMeta{StartedAt: t1, FinishedAt: ptr(t1)}},
		{RunMeta: rundata.RunMeta{StartedAt: t3, FinishedAt: ptr(t3)}},
	}

	got := ComputeTrends(runs)

	if len(got) != 3 {
		t.Fatalf("len(ComputeTrends) = %d, want 3", len(got))
	}
	if !got[0].Timestamp.Equal(t1) {
		t.Errorf("got[0].Timestamp = %v, want %v (oldest first)", got[0].Timestamp, t1)
	}
	if !got[1].Timestamp.Equal(t3) {
		t.Errorf("got[1].Timestamp = %v, want %v", got[1].Timestamp, t3)
	}
	if !got[2].Timestamp.Equal(t2) {
		t.Errorf("got[2].Timestamp = %v, want %v (newest last)", got[2].Timestamp, t2)
	}
}

func TestComputeTrendsSuccessRate(t *testing.T) {
	// 3 implemented, 1 failed — rate 0.75
	finishedAt := time.Now()
	runs := []rundata.RunDetail{
		{
			RunMeta: rundata.RunMeta{FinishedAt: &finishedAt},
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: "implemented"}},
				{Outcome: rundata.Outcome{Status: "implemented"}},
				{Outcome: rundata.Outcome{Status: "implemented"}},
				{Outcome: rundata.Outcome{Status: "failed"}},
			},
		},
	}

	got := ComputeTrends(runs)

	if len(got) != 1 {
		t.Fatalf("len(ComputeTrends) = %d, want 1", len(got))
	}
	if !nearlyEqual(got[0].SuccessRate, 0.75, 0.0001) {
		t.Errorf("SuccessRate = %f, want 0.75", got[0].SuccessRate)
	}
	if got[0].IssueCount != 4 {
		t.Errorf("IssueCount = %d, want 4", got[0].IssueCount)
	}
}

func TestComputeTrendsAvgRetries(t *testing.T) {
	// Issues with 0, 2, 4 retries — avg 2.0
	finishedAt := time.Now()
	runs := []rundata.RunDetail{
		{
			RunMeta: rundata.RunMeta{FinishedAt: &finishedAt},
			Issues: []rundata.IssueDetail{
				{}, // 0 retries
				{
					Retries: []rundata.RetryDetail{{Attempt: 0}, {Attempt: 1}}, // 2 retries
				},
				{
					Retries: []rundata.RetryDetail{{Attempt: 0}, {Attempt: 1}, {Attempt: 2}, {Attempt: 3}}, // 4 retries
				},
			},
		},
	}

	got := ComputeTrends(runs)

	if len(got) != 1 {
		t.Fatalf("len(ComputeTrends) = %d, want 1", len(got))
	}
	if !nearlyEqual(got[0].AvgRetries, 2.0, 0.0001) {
		t.Errorf("AvgRetries = %f, want 2.0", got[0].AvgRetries)
	}
}

func TestComputeTrendsCostSummed(t *testing.T) {
	// 3 issues each costing $0.10 — total $0.30
	finishedAt := time.Now()
	runs := []rundata.RunDetail{
		{
			RunMeta: rundata.RunMeta{FinishedAt: &finishedAt},
			Issues: []rundata.IssueDetail{
				{Implement: rundata.StepResult{CostUSD: 0.10}},
				{Implement: rundata.StepResult{CostUSD: 0.10}},
				{Implement: rundata.StepResult{CostUSD: 0.10}},
			},
		},
	}

	got := ComputeTrends(runs)

	if len(got) != 1 {
		t.Fatalf("len(ComputeTrends) = %d, want 1", len(got))
	}
	if !nearlyEqual(got[0].TotalCostUSD, 0.30, 0.0001) {
		t.Errorf("TotalCostUSD = %f, want 0.30", got[0].TotalCostUSD)
	}
	if !nearlyEqual(got[0].AvgCostPerIssueUSD, 0.10, 0.0001) {
		t.Errorf("AvgCostPerIssueUSD = %f, want 0.10", got[0].AvgCostPerIssueUSD)
	}
}

func TestComputeTrendsUnfinishedExcluded(t *testing.T) {
	finishedAt := time.Now()
	runs := []rundata.RunDetail{
		{
			RunMeta: rundata.RunMeta{FinishedAt: nil}, // unfinished — excluded
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: "implemented"}},
			},
		},
		{
			RunMeta: rundata.RunMeta{FinishedAt: &finishedAt},
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: "implemented"}},
			},
		},
	}

	got := ComputeTrends(runs)

	if len(got) != 1 {
		t.Fatalf("len(ComputeTrends) = %d, want 1 (unfinished run excluded)", len(got))
	}
}

func TestComputeTrendsAllUnfinishedReturnsNil(t *testing.T) {
	runs := []rundata.RunDetail{
		{RunMeta: rundata.RunMeta{FinishedAt: nil}},
		{RunMeta: rundata.RunMeta{FinishedAt: nil}},
	}

	got := ComputeTrends(runs)

	if got != nil {
		t.Errorf("ComputeTrends(all unfinished) = %v, want nil", got)
	}
}

func TestComputeTrendsMetaFields(t *testing.T) {
	finishedAt := time.Now()
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	runs := []rundata.RunDetail{
		{
			RunMeta: rundata.RunMeta{
				Repo:       "owner/repo",
				Milestone:  "v1.0",
				StartedAt:  ts,
				FinishedAt: &finishedAt,
			},
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: "implemented"}},
			},
		},
	}

	got := ComputeTrends(runs)

	if len(got) != 1 {
		t.Fatalf("len(ComputeTrends) = %d, want 1", len(got))
	}
	if got[0].Repo != "owner/repo" {
		t.Errorf("Repo = %q, want %q", got[0].Repo, "owner/repo")
	}
	if got[0].Milestone != "v1.0" {
		t.Errorf("Milestone = %q, want %q", got[0].Milestone, "v1.0")
	}
	if !got[0].Timestamp.Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", got[0].Timestamp, ts)
	}
}
