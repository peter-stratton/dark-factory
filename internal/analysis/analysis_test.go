package analysis

import (
	"math"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
)

// nearlyEqual returns true if a and b differ by less than epsilon.
func nearlyEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestAggregateNilRuns(t *testing.T) {
	got := Aggregate(nil)
	if got.RunCount != 0 || got.IssueCount != 0 || got.Outcomes != nil ||
		got.FlagFrequencies != nil || got.VerifyStats != nil || got.RepoStats != nil {
		t.Errorf("Aggregate(nil) = %+v, want zero Report", got)
	}
}

func TestAggregateEmptyRuns(t *testing.T) {
	got := Aggregate([]rundata.RunDetail{})
	if got.RunCount != 0 || got.IssueCount != 0 || got.Outcomes != nil ||
		got.FlagFrequencies != nil || got.VerifyStats != nil || got.RepoStats != nil {
		t.Errorf("Aggregate([]) = %+v, want zero Report", got)
	}
}

func TestAggregateOutcomeCounts(t *testing.T) {
	// 3 issues across runs: 2 implemented, 1 failed.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: "implemented"}},
				{Outcome: rundata.Outcome{Status: "implemented"}},
			},
		},
		{
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: "failed"}},
			},
		},
	}

	got := Aggregate(runs)

	if got.RunCount != 2 {
		t.Errorf("RunCount = %d, want 2", got.RunCount)
	}
	if got.IssueCount != 3 {
		t.Errorf("IssueCount = %d, want 3", got.IssueCount)
	}
	if got.Outcomes["implemented"] != 2 {
		t.Errorf("Outcomes[implemented] = %d, want 2", got.Outcomes["implemented"])
	}
	if got.Outcomes["failed"] != 1 {
		t.Errorf("Outcomes[failed] = %d, want 1", got.Outcomes["failed"])
	}
	if got.Outcomes["needs-human-review"] != 0 {
		t.Errorf("Outcomes[needs-human-review] = %d, want 0", got.Outcomes["needs-human-review"])
	}
}

func TestAggregateFlagFrequencies(t *testing.T) {
	// 5 issues: 3 with no_diff_read flag, 1 with low_cost flag.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Implement: rundata.StepResult{Flags: []rundata.Flag{{Code: "no_diff_read"}}}},
				{Implement: rundata.StepResult{Flags: []rundata.Flag{{Code: "no_diff_read"}}}},
				{Implement: rundata.StepResult{Flags: []rundata.Flag{{Code: "no_diff_read"}}}},
				{QualityReview: rundata.StepResult{Flags: []rundata.Flag{{Code: "low_cost"}}}},
				{}, // no flags
			},
		},
	}

	got := Aggregate(runs)

	if got.IssueCount != 5 {
		t.Errorf("IssueCount = %d, want 5", got.IssueCount)
	}
	if len(got.FlagFrequencies) != 2 {
		t.Fatalf("len(FlagFrequencies) = %d, want 2", len(got.FlagFrequencies))
	}

	// Sorted by count descending: no_diff_read (3), low_cost (1).
	if got.FlagFrequencies[0].Code != "no_diff_read" {
		t.Errorf("FlagFrequencies[0].Code = %q, want %q", got.FlagFrequencies[0].Code, "no_diff_read")
	}
	if got.FlagFrequencies[0].Count != 3 {
		t.Errorf("FlagFrequencies[0].Count = %d, want 3", got.FlagFrequencies[0].Count)
	}
	if !nearlyEqual(got.FlagFrequencies[0].Percent, 60.0, 0.001) {
		t.Errorf("FlagFrequencies[0].Percent = %f, want 60.0", got.FlagFrequencies[0].Percent)
	}

	if got.FlagFrequencies[1].Code != "low_cost" {
		t.Errorf("FlagFrequencies[1].Code = %q, want %q", got.FlagFrequencies[1].Code, "low_cost")
	}
	if got.FlagFrequencies[1].Count != 1 {
		t.Errorf("FlagFrequencies[1].Count = %d, want 1", got.FlagFrequencies[1].Count)
	}
	if !nearlyEqual(got.FlagFrequencies[1].Percent, 20.0, 0.001) {
		t.Errorf("FlagFrequencies[1].Percent = %f, want 20.0", got.FlagFrequencies[1].Percent)
	}
}

func TestAggregateFlagsFromRetries(t *testing.T) {
	// Flags should be collected from retry step results too.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Retries: []rundata.RetryDetail{
						{
							QualityReview: rundata.StepResult{Flags: []rundata.Flag{{Code: "retry_flag"}}},
						},
					},
				},
			},
		},
	}

	got := Aggregate(runs)

	if len(got.FlagFrequencies) != 1 {
		t.Fatalf("len(FlagFrequencies) = %d, want 1", len(got.FlagFrequencies))
	}
	if got.FlagFrequencies[0].Code != "retry_flag" {
		t.Errorf("FlagFrequencies[0].Code = %q, want %q", got.FlagFrequencies[0].Code, "retry_flag")
	}
}

func TestAggregateRetryStats(t *testing.T) {
	// Issues with 0, 2, and 3 retries — avg 1.67, max 3.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{}, // 0 retries
				{
					Retries: []rundata.RetryDetail{{Attempt: 0}, {Attempt: 1}}, // 2 retries
				},
				{
					Retries: []rundata.RetryDetail{{Attempt: 0}, {Attempt: 1}, {Attempt: 2}}, // 3 retries
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.RetryStats.TotalRetries != 5 {
		t.Errorf("RetryStats.TotalRetries = %d, want 5", got.RetryStats.TotalRetries)
	}
	if got.RetryStats.MaxRetries != 3 {
		t.Errorf("RetryStats.MaxRetries = %d, want 3", got.RetryStats.MaxRetries)
	}
	if !nearlyEqual(got.RetryStats.AvgPerIssue, 5.0/3.0, 0.001) {
		t.Errorf("RetryStats.AvgPerIssue = %f, want ~1.667", got.RetryStats.AvgPerIssue)
	}
}

func TestAggregateExhaustedCount(t *testing.T) {
	// 2 issues with needs-human-review, 1 implemented.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: "needs-human-review"}},
				{Outcome: rundata.Outcome{Status: "needs-human-review"}},
				{Outcome: rundata.Outcome{Status: "implemented"}},
			},
		},
	}

	got := Aggregate(runs)

	if got.RetryStats.ExhaustedCount != 2 {
		t.Errorf("RetryStats.ExhaustedCount = %d, want 2", got.RetryStats.ExhaustedCount)
	}
}

func TestAggregateCostStats(t *testing.T) {
	// Total cost summed across all step results for all issues in all runs.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Implement:     rundata.StepResult{CostUSD: 1.0},
					QualityReview: rundata.StepResult{CostUSD: 0.5},
				},
				{
					Implement: rundata.StepResult{CostUSD: 2.0},
					Retries: []rundata.RetryDetail{
						{
							Retry:         rundata.StepResult{CostUSD: 0.3},
							QualityReview: rundata.StepResult{CostUSD: 0.2},
						},
					},
				},
			},
		},
		{
			Issues: []rundata.IssueDetail{
				{
					Implement: rundata.StepResult{CostUSD: 1.5},
				},
			},
		},
	}
	// Total = 1.0 + 0.5 + 2.0 + 0.3 + 0.2 + 1.5 = 5.5
	// IssueCount = 3, RunCount = 2
	// AvgPerIssue = 5.5 / 3
	// AvgPerRun = 5.5 / 2

	got := Aggregate(runs)

	if !nearlyEqual(got.CostStats.TotalUSD, 5.5, 0.0001) {
		t.Errorf("CostStats.TotalUSD = %f, want 5.5", got.CostStats.TotalUSD)
	}
	if !nearlyEqual(got.CostStats.AvgPerIssueUSD, 5.5/3.0, 0.0001) {
		t.Errorf("CostStats.AvgPerIssueUSD = %f, want %f", got.CostStats.AvgPerIssueUSD, 5.5/3.0)
	}
	if !nearlyEqual(got.CostStats.AvgPerRunUSD, 5.5/2.0, 0.0001) {
		t.Errorf("CostStats.AvgPerRunUSD = %f, want %f", got.CostStats.AvgPerRunUSD, 5.5/2.0)
	}
}

func TestAggregateVerifyStats(t *testing.T) {
	// Verify failure counts by check name.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					VerifyResults: []rundata.VerifyStepResult{
						{
							Checks: []rundata.CheckResult{
								{Name: "build", Passed: false},
								{Name: "lint", Passed: true},
								{Name: "test", Passed: false},
							},
						},
						{
							Checks: []rundata.CheckResult{
								{Name: "build", Passed: false},
								{Name: "lint", Passed: false},
								{Name: "test", Passed: true},
							},
						},
					},
				},
			},
		},
	}
	// build failed: 2, lint failed: 1, test failed: 1

	got := Aggregate(runs)

	if got.VerifyStats["build"] != 2 {
		t.Errorf("VerifyStats[build] = %d, want 2", got.VerifyStats["build"])
	}
	if got.VerifyStats["lint"] != 1 {
		t.Errorf("VerifyStats[lint] = %d, want 1", got.VerifyStats["lint"])
	}
	if got.VerifyStats["test"] != 1 {
		t.Errorf("VerifyStats[test] = %d, want 1", got.VerifyStats["test"])
	}
}

func TestAggregateSingleRunSingleIssue(t *testing.T) {
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome:   rundata.Outcome{Status: "implemented"},
					Implement: rundata.StepResult{CostUSD: 3.0},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", got.RunCount)
	}
	if got.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", got.IssueCount)
	}
	if got.Outcomes["implemented"] != 1 {
		t.Errorf("Outcomes[implemented] = %d, want 1", got.Outcomes["implemented"])
	}
	if got.RetryStats.TotalRetries != 0 {
		t.Errorf("RetryStats.TotalRetries = %d, want 0", got.RetryStats.TotalRetries)
	}
	if !nearlyEqual(got.CostStats.TotalUSD, 3.0, 0.0001) {
		t.Errorf("CostStats.TotalUSD = %f, want 3.0", got.CostStats.TotalUSD)
	}
	if !nearlyEqual(got.CostStats.AvgPerIssueUSD, 3.0, 0.0001) {
		t.Errorf("CostStats.AvgPerIssueUSD = %f, want 3.0", got.CostStats.AvgPerIssueUSD)
	}
	if !nearlyEqual(got.CostStats.AvgPerRunUSD, 3.0, 0.0001) {
		t.Errorf("CostStats.AvgPerRunUSD = %f, want 3.0", got.CostStats.AvgPerRunUSD)
	}
}

func TestAggregateRecoveryRateNoRetries(t *testing.T) {
	// All issues succeed on first attempt — no retries.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusReadyToMerge}},
			},
		},
	}

	got := Aggregate(runs)

	if got.RetryStats.RetriedCount != 0 {
		t.Errorf("RetryStats.RetriedCount = %d, want 0", got.RetryStats.RetriedCount)
	}
	if got.RetryStats.RecoveryRate != 0.0 {
		t.Errorf("RetryStats.RecoveryRate = %f, want 0.0", got.RetryStats.RecoveryRate)
	}
}

func TestAggregateRecoveryRateAllSucceed(t *testing.T) {
	// 3 issues all retry and all eventually succeed.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusReadyToMerge},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.RetryStats.RetriedCount != 3 {
		t.Errorf("RetryStats.RetriedCount = %d, want 3", got.RetryStats.RetriedCount)
	}
	if !nearlyEqual(got.RetryStats.RecoveryRate, 1.0, 0.001) {
		t.Errorf("RetryStats.RecoveryRate = %f, want 1.0", got.RetryStats.RecoveryRate)
	}
}

func TestAggregateRecoveryRateMixed(t *testing.T) {
	// 2 retried issues succeed, 1 exhausts — RecoveryRate ~0.667.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusReadyToMerge},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusNeedsHumanReview},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.RetryStats.RetriedCount != 3 {
		t.Errorf("RetryStats.RetriedCount = %d, want 3", got.RetryStats.RetriedCount)
	}
	if !nearlyEqual(got.RetryStats.RecoveryRate, 2.0/3.0, 0.001) {
		t.Errorf("RetryStats.RecoveryRate = %f, want ~0.667", got.RetryStats.RecoveryRate)
	}
}

func TestAggregateRecoveryRateFailedStatus(t *testing.T) {
	// Retried issue with "failed" status is not a success.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusFailed},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.RetryStats.RetriedCount != 1 {
		t.Errorf("RetryStats.RetriedCount = %d, want 1", got.RetryStats.RetriedCount)
	}
	if got.RetryStats.RecoveryRate != 0.0 {
		t.Errorf("RetryStats.RecoveryRate = %f, want 0.0", got.RetryStats.RecoveryRate)
	}
}

func TestAggregateFlagFrequenciesSortStability(t *testing.T) {
	// Two flags with same count should be sorted alphabetically.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Implement: rundata.StepResult{Flags: []rundata.Flag{{Code: "zebra"}, {Code: "alpha"}}}},
			},
		},
	}

	got := Aggregate(runs)

	if len(got.FlagFrequencies) != 2 {
		t.Fatalf("len(FlagFrequencies) = %d, want 2", len(got.FlagFrequencies))
	}
	if got.FlagFrequencies[0].Code != "alpha" {
		t.Errorf("FlagFrequencies[0].Code = %q, want %q (alphabetical for equal counts)", got.FlagFrequencies[0].Code, "alpha")
	}
	if got.FlagFrequencies[1].Code != "zebra" {
		t.Errorf("FlagFrequencies[1].Code = %q, want %q", got.FlagFrequencies[1].Code, "zebra")
	}
}

func TestAggregateCostByStepZeroCost(t *testing.T) {
	// Zero cost: no cost data — CostByStep should be an empty (non-nil) map.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: "implemented"}},
			},
		},
	}

	got := Aggregate(runs)

	if got.CostStats.CostByStep == nil {
		t.Error("CostStats.CostByStep is nil, want empty map")
	}
	if len(got.CostStats.CostByStep) != 0 {
		t.Errorf("CostStats.CostByStep = %v, want empty map", got.CostStats.CostByStep)
	}
}

func TestAggregateCostByStepSingleStep(t *testing.T) {
	// Only implement steps present — implement should be 100%.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Implement: rundata.StepResult{CostUSD: 3.0}},
			},
		},
	}

	got := Aggregate(runs)

	if !nearlyEqual(got.CostStats.CostByStep["implement"], 3.0, 0.0001) {
		t.Errorf("CostByStep[implement] = %f, want 3.0", got.CostStats.CostByStep["implement"])
	}
	// Other steps should not appear.
	if v, ok := got.CostStats.CostByStep["quality-review"]; ok && v != 0 {
		t.Errorf("CostByStep[quality-review] = %f, want 0 or absent", v)
	}
	// Total should equal sum of all CostByStep values.
	if !nearlyEqual(got.CostStats.TotalUSD, 3.0, 0.0001) {
		t.Errorf("TotalUSD = %f, want 3.0", got.CostStats.TotalUSD)
	}
}

func TestAggregateCostByStepMultipleSteps(t *testing.T) {
	// Implement $3.00, quality-review $1.00, retry $0.50 — percentages 66.7%, 22.2%, 11.1%.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Implement:     rundata.StepResult{CostUSD: 3.0},
					QualityReview: rundata.StepResult{CostUSD: 1.0},
					Retries: []rundata.RetryDetail{
						{
							Retry: rundata.StepResult{CostUSD: 0.50},
						},
					},
				},
			},
		},
	}

	got := Aggregate(runs)

	total := got.CostStats.TotalUSD
	if !nearlyEqual(total, 4.5, 0.0001) {
		t.Errorf("TotalUSD = %f, want 4.5", total)
	}
	if !nearlyEqual(got.CostStats.CostByStep["implement"], 3.0, 0.0001) {
		t.Errorf("CostByStep[implement] = %f, want 3.0", got.CostStats.CostByStep["implement"])
	}
	if !nearlyEqual(got.CostStats.CostByStep["quality-review"], 1.0, 0.0001) {
		t.Errorf("CostByStep[quality-review] = %f, want 1.0", got.CostStats.CostByStep["quality-review"])
	}
	if !nearlyEqual(got.CostStats.CostByStep["retries"], 0.50, 0.0001) {
		t.Errorf("CostByStep[retries] = %f, want 0.50", got.CostStats.CostByStep["retries"])
	}

	// Percentages should sum to ~100%.
	var sumPct float64
	for _, cost := range got.CostStats.CostByStep {
		sumPct += cost / total * 100
	}
	if !nearlyEqual(sumPct, 100.0, 0.1) {
		t.Errorf("sum of percentages = %f, want ~100", sumPct)
	}
}

func TestAggregateDurationStats(t *testing.T) {
	// Two issues with known durations across two runs.
	// Issue 1: implement 300s, review 120s
	// Issue 2: implement 600s, review 0s (missing)
	// Total implement: 900s, total review: 120s, issue count: 2
	// AvgImplementSeconds = 450.0, AvgReviewSeconds = 60.0
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Implement:     rundata.StepResult{DurationSeconds: 300.0},
					QualityReview: rundata.StepResult{DurationSeconds: 120.0},
				},
			},
		},
		{
			Issues: []rundata.IssueDetail{
				{
					Implement: rundata.StepResult{DurationSeconds: 600.0},
					// QualityReview has no duration — defaults to 0.0
				},
			},
		},
	}

	got := Aggregate(runs)

	if !nearlyEqual(got.DurationStats.AvgImplementSeconds, 450.0, 0.001) {
		t.Errorf("DurationStats.AvgImplementSeconds = %f, want 450.0", got.DurationStats.AvgImplementSeconds)
	}
	if !nearlyEqual(got.DurationStats.AvgReviewSeconds, 60.0, 0.001) {
		t.Errorf("DurationStats.AvgReviewSeconds = %f, want 60.0", got.DurationStats.AvgReviewSeconds)
	}
}

func TestAggregateDurationStatsNoData(t *testing.T) {
	// No duration data — both averages are 0.0, not NaN.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{}, // no step data
			},
		},
	}

	got := Aggregate(runs)

	if got.DurationStats.AvgImplementSeconds != 0.0 {
		t.Errorf("DurationStats.AvgImplementSeconds = %f, want 0.0", got.DurationStats.AvgImplementSeconds)
	}
	if got.DurationStats.AvgReviewSeconds != 0.0 {
		t.Errorf("DurationStats.AvgReviewSeconds = %f, want 0.0", got.DurationStats.AvgReviewSeconds)
	}
}

func TestAggregateCostByStepRecon(t *testing.T) {
	// Recon cost should be tracked and included in total.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Recon:     rundata.StepResult{CostUSD: 0.25},
					Implement: rundata.StepResult{CostUSD: 1.0},
				},
			},
		},
	}

	got := Aggregate(runs)

	if !nearlyEqual(got.CostStats.CostByStep["recon"], 0.25, 0.0001) {
		t.Errorf("CostByStep[recon] = %f, want 0.25", got.CostStats.CostByStep["recon"])
	}
	if !nearlyEqual(got.CostStats.TotalUSD, 1.25, 0.0001) {
		t.Errorf("TotalUSD = %f, want 1.25 (recon included)", got.CostStats.TotalUSD)
	}
}

func TestAggregateRepoStatsSingle(t *testing.T) {
	// One repo with 8 implemented, 2 failed — success rate 80%.
	runs := []rundata.RunDetail{
		{
			RunMeta: rundata.RunMeta{Repo: "owner/repo"},
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusFailed}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusFailed}},
			},
		},
	}

	got := Aggregate(runs)

	if len(got.RepoStats) != 1 {
		t.Fatalf("len(RepoStats) = %d, want 1", len(got.RepoStats))
	}
	rs := got.RepoStats["owner/repo"]
	if rs.Total != 10 {
		t.Errorf("RepoStats[owner/repo].Total = %d, want 10", rs.Total)
	}
	if rs.Implemented != 8 {
		t.Errorf("RepoStats[owner/repo].Implemented = %d, want 8", rs.Implemented)
	}
	if rs.Failed != 2 {
		t.Errorf("RepoStats[owner/repo].Failed = %d, want 2", rs.Failed)
	}
	if !nearlyEqual(rs.SuccessRate, 0.80, 0.001) {
		t.Errorf("RepoStats[owner/repo].SuccessRate = %f, want 0.80", rs.SuccessRate)
	}
}

func TestAggregateRepoStatsMultiple(t *testing.T) {
	// Two repos with different success rates shown separately.
	runs := []rundata.RunDetail{
		{
			RunMeta: rundata.RunMeta{Repo: "owner/alpha"},
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusFailed}},
			},
		},
		{
			RunMeta: rundata.RunMeta{Repo: "owner/beta"},
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusNeedsHumanReview}},
			},
		},
	}

	got := Aggregate(runs)

	if len(got.RepoStats) != 2 {
		t.Fatalf("len(RepoStats) = %d, want 2", len(got.RepoStats))
	}

	alpha := got.RepoStats["owner/alpha"]
	if alpha.Total != 3 {
		t.Errorf("alpha.Total = %d, want 3", alpha.Total)
	}
	if alpha.Implemented != 2 {
		t.Errorf("alpha.Implemented = %d, want 2", alpha.Implemented)
	}
	if alpha.Failed != 1 {
		t.Errorf("alpha.Failed = %d, want 1", alpha.Failed)
	}
	if !nearlyEqual(alpha.SuccessRate, 2.0/3.0, 0.001) {
		t.Errorf("alpha.SuccessRate = %f, want ~0.667", alpha.SuccessRate)
	}

	beta := got.RepoStats["owner/beta"]
	if beta.Total != 2 {
		t.Errorf("beta.Total = %d, want 2", beta.Total)
	}
	if beta.Implemented != 1 {
		t.Errorf("beta.Implemented = %d, want 1", beta.Implemented)
	}
	if beta.Failed != 1 {
		t.Errorf("beta.Failed = %d, want 1", beta.Failed)
	}
	if !nearlyEqual(beta.SuccessRate, 0.5, 0.001) {
		t.Errorf("beta.SuccessRate = %f, want 0.5", beta.SuccessRate)
	}
}

func TestAggregateDurationByStepZeroDuration(t *testing.T) {
	// No duration data — DurationByStep should be an empty (non-nil) map.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: "implemented"}},
			},
		},
	}

	got := Aggregate(runs)

	if got.DurationStats.DurationByStep == nil {
		t.Error("DurationStats.DurationByStep is nil, want empty map")
	}
	if len(got.DurationStats.DurationByStep) != 0 {
		t.Errorf("DurationStats.DurationByStep = %v, want empty map", got.DurationStats.DurationByStep)
	}
}

func TestAggregateDurationByStepSingleStep(t *testing.T) {
	// Only implement steps present — implement should be 100%.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Implement: rundata.StepResult{DurationSeconds: 420.0}},
			},
		},
	}

	got := Aggregate(runs)

	if !nearlyEqual(got.DurationStats.DurationByStep["implement"], 420.0, 0.0001) {
		t.Errorf("DurationByStep[implement] = %f, want 420.0", got.DurationStats.DurationByStep["implement"])
	}
	// Other steps should not appear.
	if v, ok := got.DurationStats.DurationByStep["quality-review"]; ok && v != 0 {
		t.Errorf("DurationByStep[quality-review] = %f, want 0 or absent", v)
	}
	// With one step, that step is 100% of total.
	var total float64
	for _, secs := range got.DurationStats.DurationByStep {
		total += secs
	}
	if !nearlyEqual(total, 420.0, 0.0001) {
		t.Errorf("total DurationByStep = %f, want 420.0", total)
	}
}

func TestAggregateDurationByStepMultipleSteps(t *testing.T) {
	// Implement 420s, quality-review 180s — percentages ~70%, ~30%.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Implement:     rundata.StepResult{DurationSeconds: 420.0},
					QualityReview: rundata.StepResult{DurationSeconds: 180.0},
				},
			},
		},
	}

	got := Aggregate(runs)

	if !nearlyEqual(got.DurationStats.DurationByStep["implement"], 420.0, 0.0001) {
		t.Errorf("DurationByStep[implement] = %f, want 420.0", got.DurationStats.DurationByStep["implement"])
	}
	if !nearlyEqual(got.DurationStats.DurationByStep["quality-review"], 180.0, 0.0001) {
		t.Errorf("DurationByStep[quality-review] = %f, want 180.0", got.DurationStats.DurationByStep["quality-review"])
	}

	// Percentages should sum to ~100%.
	var total float64
	for _, secs := range got.DurationStats.DurationByStep {
		total += secs
	}
	if !nearlyEqual(total, 600.0, 0.0001) {
		t.Errorf("total DurationByStep = %f, want 600.0", total)
	}
	var sumPct float64
	for _, secs := range got.DurationStats.DurationByStep {
		sumPct += secs / total * 100
	}
	if !nearlyEqual(sumPct, 100.0, 0.1) {
		t.Errorf("sum of percentages = %f, want ~100", sumPct)
	}
}

func TestAggregateDurationByStepWithRetries(t *testing.T) {
	// Retry durations should accumulate under the "retries" key.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Implement: rundata.StepResult{DurationSeconds: 300.0},
					Retries: []rundata.RetryDetail{
						{
							Retry:         rundata.StepResult{DurationSeconds: 60.0},
							QualityReview: rundata.StepResult{DurationSeconds: 30.0},
						},
					},
				},
			},
		},
	}

	got := Aggregate(runs)

	if !nearlyEqual(got.DurationStats.DurationByStep["implement"], 300.0, 0.0001) {
		t.Errorf("DurationByStep[implement] = %f, want 300.0", got.DurationStats.DurationByStep["implement"])
	}
	if !nearlyEqual(got.DurationStats.DurationByStep["retries"], 90.0, 0.0001) {
		t.Errorf("DurationByStep[retries] = %f, want 90.0 (retry 60s + qr 30s)", got.DurationStats.DurationByStep["retries"])
	}
}

func TestAggregateFirstPassAllFirstPass(t *testing.T) {
	// 5 issues, 0 retries each, all implemented — FirstPassSuccessRate=1.0, FirstPassCount=5.
	issues := make([]rundata.IssueDetail, 5)
	for i := range issues {
		issues[i] = rundata.IssueDetail{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}}
	}
	runs := []rundata.RunDetail{{Issues: issues}}

	got := Aggregate(runs)

	if got.FirstPassCount != 5 {
		t.Errorf("FirstPassCount = %d, want 5", got.FirstPassCount)
	}
	if !nearlyEqual(got.FirstPassSuccessRate, 1.0, 0.001) {
		t.Errorf("FirstPassSuccessRate = %f, want 1.0", got.FirstPassSuccessRate)
	}
}

func TestAggregateFirstPassMixed(t *testing.T) {
	// 3 issues succeed first-pass, 2 retry and succeed — FirstPassSuccessRate=0.6.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.FirstPassCount != 3 {
		t.Errorf("FirstPassCount = %d, want 3", got.FirstPassCount)
	}
	if !nearlyEqual(got.FirstPassSuccessRate, 0.6, 0.001) {
		t.Errorf("FirstPassSuccessRate = %f, want 0.6", got.FirstPassSuccessRate)
	}
}

func TestAggregateWastedCost(t *testing.T) {
	// 2 failed issues with $1.50 total cost — WastedCostUSD=1.50.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusFailed},
					Implement: rundata.StepResult{CostUSD: 1.00},
				},
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusFailed},
					Implement: rundata.StepResult{CostUSD: 0.50},
				},
			},
		},
	}

	got := Aggregate(runs)

	if !nearlyEqual(got.WastedCostUSD, 1.50, 0.0001) {
		t.Errorf("WastedCostUSD = %f, want 1.50", got.WastedCostUSD)
	}
}

func TestAggregateAvgCostPerSuccessNoSuccesses(t *testing.T) {
	// All issues fail — AvgCostPerSuccessUSD=0.0 (no division by zero).
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusFailed},
					Implement: rundata.StepResult{CostUSD: 2.00},
				},
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusNeedsHumanReview},
					Implement: rundata.StepResult{CostUSD: 1.00},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.AvgCostPerSuccessUSD != 0.0 {
		t.Errorf("AvgCostPerSuccessUSD = %f, want 0.0", got.AvgCostPerSuccessUSD)
	}
}

func TestAggregateRepoFirstPassRate(t *testing.T) {
	// repo-a: 3 first-pass out of 4, repo-b: 1 first-pass out of 3.
	runs := []rundata.RunDetail{
		{
			RunMeta: rundata.RunMeta{Repo: "owner/repo-a"},
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
			},
		},
		{
			RunMeta: rundata.RunMeta{Repo: "owner/repo-b"},
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Retries: []rundata.RetryDetail{{Attempt: 0}},
				},
			},
		},
	}

	got := Aggregate(runs)

	repoA := got.RepoStats["owner/repo-a"]
	if repoA.FirstPassCount != 3 {
		t.Errorf("repo-a FirstPassCount = %d, want 3", repoA.FirstPassCount)
	}
	if !nearlyEqual(repoA.FirstPassRate, 3.0/4.0, 0.001) {
		t.Errorf("repo-a FirstPassRate = %f, want 0.75", repoA.FirstPassRate)
	}

	repoB := got.RepoStats["owner/repo-b"]
	if repoB.FirstPassCount != 1 {
		t.Errorf("repo-b FirstPassCount = %d, want 1", repoB.FirstPassCount)
	}
	if !nearlyEqual(repoB.FirstPassRate, 1.0/3.0, 0.001) {
		t.Errorf("repo-b FirstPassRate = %f, want ~0.333", repoB.FirstPassRate)
	}
}

func TestAggregateAvgCostPerSuccessUSD(t *testing.T) {
	// 2 implemented issues at $2.00 each, 1 failed at $1.00 — AvgCostPerSuccessUSD = 5.0/2 = 2.5.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Implement: rundata.StepResult{CostUSD: 2.00},
				},
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Implement: rundata.StepResult{CostUSD: 2.00},
				},
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusFailed},
					Implement: rundata.StepResult{CostUSD: 1.00},
				},
			},
		},
	}

	got := Aggregate(runs)

	// total cost = 5.0, implemented count = 2 → AvgCostPerSuccessUSD = 2.5
	if !nearlyEqual(got.AvgCostPerSuccessUSD, 2.5, 0.0001) {
		t.Errorf("AvgCostPerSuccessUSD = %f, want 2.5", got.AvgCostPerSuccessUSD)
	}
}

func TestAggregateRepoAvgCostPerIssueUSD(t *testing.T) {
	// Verify per-repo AvgCostPerIssueUSD is computed correctly.
	runs := []rundata.RunDetail{
		{
			RunMeta: rundata.RunMeta{Repo: "owner/repo"},
			Issues: []rundata.IssueDetail{
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Implement: rundata.StepResult{CostUSD: 1.00},
				},
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Implement: rundata.StepResult{CostUSD: 3.00},
				},
			},
		},
	}

	got := Aggregate(runs)

	rs := got.RepoStats["owner/repo"]
	if !nearlyEqual(rs.TotalCostUSD, 4.00, 0.0001) {
		t.Errorf("RepoStats TotalCostUSD = %f, want 4.00", rs.TotalCostUSD)
	}
	if !nearlyEqual(rs.AvgCostPerIssueUSD, 2.00, 0.0001) {
		t.Errorf("RepoStats AvgCostPerIssueUSD = %f, want 2.00", rs.AvgCostPerIssueUSD)
	}
}

func TestAggregateFirstPassReadyToMergeCountsAsSuccess(t *testing.T) {
	// ready-to-merge with no retries should count as first-pass.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusReadyToMerge}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
			},
		},
	}

	got := Aggregate(runs)

	if got.FirstPassCount != 2 {
		t.Errorf("FirstPassCount = %d, want 2 (both implemented and ready-to-merge count)", got.FirstPassCount)
	}
}

func TestAggregateWastedCostNeedsHumanReview(t *testing.T) {
	// needs-human-review issues also contribute to wasted cost.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusNeedsHumanReview},
					Implement: rundata.StepResult{CostUSD: 0.75},
				},
			},
		},
	}

	got := Aggregate(runs)

	if !nearlyEqual(got.WastedCostUSD, 0.75, 0.0001) {
		t.Errorf("WastedCostUSD = %f, want 0.75", got.WastedCostUSD)
	}
}

func TestAggregateDurationByStepRecon(t *testing.T) {
	// Recon and spec-generator durations should be tracked.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Recon:         rundata.StepResult{DurationSeconds: 20.0},
					SpecGenerator: rundata.StepResult{DurationSeconds: 10.0},
					Implement:     rundata.StepResult{DurationSeconds: 300.0},
				},
			},
		},
	}

	got := Aggregate(runs)

	if !nearlyEqual(got.DurationStats.DurationByStep["recon"], 20.0, 0.0001) {
		t.Errorf("DurationByStep[recon] = %f, want 20.0", got.DurationStats.DurationByStep["recon"])
	}
	if !nearlyEqual(got.DurationStats.DurationByStep["spec-generator"], 10.0, 0.0001) {
		t.Errorf("DurationByStep[spec-generator] = %f, want 10.0", got.DurationStats.DurationByStep["spec-generator"])
	}
	if !nearlyEqual(got.DurationStats.DurationByStep["implement"], 300.0, 0.0001) {
		t.Errorf("DurationByStep[implement] = %f, want 300.0", got.DurationStats.DurationByStep["implement"])
	}
}

// --- Phase 22 / Issue #490: failure reason breakdown and timeout rate ---

func TestAggregateFailureReasonVerify(t *testing.T) {
	// Issue fails with a verify step where AllPassed == false.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusFailed},
					VerifyResults: []rundata.VerifyStepResult{
						{Attempt: 0, AllPassed: false},
					},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.FailureReasons["verify"] != 1 {
		t.Errorf("FailureReasons[verify] = %d, want 1", got.FailureReasons["verify"])
	}
	if len(got.FailureReasons) != 1 {
		t.Errorf("len(FailureReasons) = %d, want 1", len(got.FailureReasons))
	}
}

func TestAggregateFailureReasonExhaustion(t *testing.T) {
	// Issue with status needs-human-review counts as exhaustion.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusNeedsHumanReview},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.FailureReasons["exhaustion"] != 1 {
		t.Errorf("FailureReasons[exhaustion] = %d, want 1", got.FailureReasons["exhaustion"])
	}
}

func TestAggregateFailureReasonTimeout(t *testing.T) {
	// Issue fails and a step has TimedOut == true.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusFailed},
					Implement: rundata.StepResult{TimedOut: true},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.FailureReasons["timeout"] != 1 {
		t.Errorf("FailureReasons[timeout] = %d, want 1", got.FailureReasons["timeout"])
	}
}

func TestAggregateFailureReasonError(t *testing.T) {
	// Issue fails with no verify failure, no exhaustion, no timeout — counts as error.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusFailed},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.FailureReasons["error"] != 1 {
		t.Errorf("FailureReasons[error] = %d, want 1", got.FailureReasons["error"])
	}
}

func TestAggregateFailureReasonsMutuallyExclusive(t *testing.T) {
	// 4 failed issues, one of each type — each bucket should be 1, total 4.
	base := time.Now()
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				// verify failure
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusFailed},
					VerifyResults: []rundata.VerifyStepResult{
						{Attempt: 0, AllPassed: false},
					},
				},
				// exhaustion
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusNeedsHumanReview},
				},
				// timeout
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusFailed},
					Implement: rundata.StepResult{TimedOut: true, StartedAt: &base},
				},
				// error fallback
				{
					Outcome: rundata.Outcome{Status: rundata.OutcomeStatusFailed},
				},
			},
		},
	}

	got := Aggregate(runs)

	if len(got.FailureReasons) != 4 {
		t.Errorf("len(FailureReasons) = %d, want 4", len(got.FailureReasons))
	}
	total := 0
	for _, v := range got.FailureReasons {
		total += v
	}
	if total != 4 {
		t.Errorf("sum of FailureReasons = %d, want 4", total)
	}
	for _, key := range []string{"verify", "exhaustion", "timeout", "error"} {
		if got.FailureReasons[key] != 1 {
			t.Errorf("FailureReasons[%s] = %d, want 1", key, got.FailureReasons[key])
		}
	}
}

func TestAggregateFailureReasonsNoFailures(t *testing.T) {
	// All issues succeed — FailureReasons should be an empty (non-nil) map.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusReadyToMerge}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusImplemented}},
			},
		},
	}

	got := Aggregate(runs)

	if got.FailureReasons == nil {
		t.Error("FailureReasons is nil, want empty map")
	}
	if len(got.FailureReasons) != 0 {
		t.Errorf("FailureReasons = %v, want empty map", got.FailureReasons)
	}
}

func TestAggregateTimeoutRate(t *testing.T) {
	// 20 total steps across all issues; 2 have TimedOut == true → rate 0.10.
	// Each issue has 5 steps (recon, spec-generator, implement, quality-review, functional-review).
	// 4 issues = 20 steps; set TimedOut on 2 implement steps.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Implement: rundata.StepResult{TimedOut: true}},
				{Implement: rundata.StepResult{TimedOut: true}},
				{},
				{},
			},
		},
	}

	got := Aggregate(runs)

	// 4 issues × 5 steps = 20 total; 2 timed out.
	if !nearlyEqual(got.TimeoutRate, 0.10, 0.001) {
		t.Errorf("TimeoutRate = %f, want 0.10", got.TimeoutRate)
	}
}

func TestAggregateTimeoutRateZero(t *testing.T) {
	// 15 steps, none timed out — TimeoutRate is 0.0.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{},
				{},
				{},
			},
		},
	}

	got := Aggregate(runs)

	if got.TimeoutRate != 0.0 {
		t.Errorf("TimeoutRate = %f, want 0.0", got.TimeoutRate)
	}
}

func TestAggregateAvgTimeToMerge(t *testing.T) {
	// 3 implemented issues with durations 300s, 600s, 900s → avg 600.0.
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mkTimes := func(durationSecs float64) (start, finish *time.Time) {
		s := base
		f := base.Add(time.Duration(durationSecs) * time.Second)
		return &s, &f
	}

	s1, f1 := mkTimes(300)
	s2, f2 := mkTimes(600)
	s3, f3 := mkTimes(900)

	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Implement: rundata.StepResult{StartedAt: s1, FinishedAt: f1},
				},
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Implement: rundata.StepResult{StartedAt: s2, FinishedAt: f2},
				},
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Implement: rundata.StepResult{StartedAt: s3, FinishedAt: f3},
				},
			},
		},
	}

	got := Aggregate(runs)

	if !nearlyEqual(got.DurationStats.AvgTimeToMergeSeconds, 600.0, 0.001) {
		t.Errorf("AvgTimeToMergeSeconds = %f, want 600.0", got.DurationStats.AvgTimeToMergeSeconds)
	}
}

func TestAggregateAvgTimeToMergeExcludesFailed(t *testing.T) {
	// 2 implemented issues (300s, 600s) and 1 failed issue (1200s) — avg = 450.0.
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mkTimes := func(durationSecs float64) (start, finish *time.Time) {
		s := base
		f := base.Add(time.Duration(durationSecs) * time.Second)
		return &s, &f
	}

	s1, f1 := mkTimes(300)
	s2, f2 := mkTimes(600)
	s3, f3 := mkTimes(1200)

	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Implement: rundata.StepResult{StartedAt: s1, FinishedAt: f1},
				},
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusImplemented},
					Implement: rundata.StepResult{StartedAt: s2, FinishedAt: f2},
				},
				{
					Outcome:   rundata.Outcome{Status: rundata.OutcomeStatusFailed},
					Implement: rundata.StepResult{StartedAt: s3, FinishedAt: f3},
				},
			},
		},
	}

	got := Aggregate(runs)

	if !nearlyEqual(got.DurationStats.AvgTimeToMergeSeconds, 450.0, 0.001) {
		t.Errorf("AvgTimeToMergeSeconds = %f, want 450.0", got.DurationStats.AvgTimeToMergeSeconds)
	}
}

func TestAggregateAvgTimeToMergeZeroWhenNoImplemented(t *testing.T) {
	// All issues fail — AvgTimeToMergeSeconds is 0.0.
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusFailed}},
				{Outcome: rundata.Outcome{Status: rundata.OutcomeStatusFailed}},
			},
		},
	}

	got := Aggregate(runs)

	if got.DurationStats.AvgTimeToMergeSeconds != 0.0 {
		t.Errorf("AvgTimeToMergeSeconds = %f, want 0.0", got.DurationStats.AvgTimeToMergeSeconds)
	}
}

// TestAggregateResourceStats_NoData: runs without resource data produce nil ResourceStats.
func TestAggregateResourceStats_NoData(t *testing.T) {
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Implement:     rundata.StepResult{CostUSD: 1.0},
					QualityReview: rundata.StepResult{CostUSD: 0.5},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.ResourceStats != nil {
		t.Errorf("ResourceStats = %+v, want nil when no resource data", got.ResourceStats)
	}
}

// TestAggregateResourceStats_WithData: resource stats are computed correctly from step results.
func TestAggregateResourceStats_WithData(t *testing.T) {
	// issue 1: implement 200MB / 2s CPU, quality-review 100MB / 1s CPU
	// issue 2: implement 300MB / 3s CPU, no quality-review resource data
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Implement:     rundata.StepResult{PeakMemoryBytes: 200 << 20, CPUNanoseconds: 2_000_000_000},
					QualityReview: rundata.StepResult{PeakMemoryBytes: 100 << 20, CPUNanoseconds: 1_000_000_000},
				},
				{
					Implement: rundata.StepResult{PeakMemoryBytes: 300 << 20, CPUNanoseconds: 3_000_000_000},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.ResourceStats == nil {
		t.Fatal("ResourceStats = nil, want non-nil")
	}
	rs := got.ResourceStats

	// MaxPeakMemoryBytes: 300MB
	want300MB := int64(300 << 20)
	if rs.MaxPeakMemoryBytes != want300MB {
		t.Errorf("MaxPeakMemoryBytes = %d, want %d", rs.MaxPeakMemoryBytes, want300MB)
	}

	// AvgPeakMemoryBytes: avg of per-issue peaks = (200MB + 300MB) / 2 = 250MB
	want250MB := int64(250 << 20)
	if rs.AvgPeakMemoryBytes != want250MB {
		t.Errorf("AvgPeakMemoryBytes = %d, want %d", rs.AvgPeakMemoryBytes, want250MB)
	}

	// TotalCPUNanoseconds: 2s + 1s + 3s = 6s
	wantTotalCPU := int64(6_000_000_000)
	if rs.TotalCPUNanoseconds != wantTotalCPU {
		t.Errorf("TotalCPUNanoseconds = %d, want %d", rs.TotalCPUNanoseconds, wantTotalCPU)
	}

	// AvgCPUNanosecondsPerIssue: 6s / 2 = 3s
	wantAvgCPU := int64(3_000_000_000)
	if rs.AvgCPUNanosecondsPerIssue != wantAvgCPU {
		t.Errorf("AvgCPUNanosecondsPerIssue = %d, want %d", rs.AvgCPUNanosecondsPerIssue, wantAvgCPU)
	}

	// Per-step: implement — max 300MB, avg (200+300)/2=250MB, CPU 5s
	implStep, ok := rs.ResourceByStep["implement"]
	if !ok {
		t.Fatal("ResourceByStep missing 'implement'")
	}
	// want250MB already declared above for AvgPeakMemoryBytes check
	if implStep.MaxMemoryBytes != want300MB {
		t.Errorf("implement MaxMemoryBytes = %d, want %d", implStep.MaxMemoryBytes, want300MB)
	}
	if implStep.AvgMemoryBytes != want250MB {
		t.Errorf("implement AvgMemoryBytes = %d, want %d", implStep.AvgMemoryBytes, want250MB)
	}
	wantImplCPU := int64(5_000_000_000)
	if implStep.TotalCPUNanoseconds != wantImplCPU {
		t.Errorf("implement TotalCPUNanoseconds = %d, want %d", implStep.TotalCPUNanoseconds, wantImplCPU)
	}

	// Per-step: quality-review — max 100MB, avg 100MB, CPU 1s
	qrStep, ok := rs.ResourceByStep["quality-review"]
	if !ok {
		t.Fatal("ResourceByStep missing 'quality-review'")
	}
	want100MB := int64(100 << 20)
	if qrStep.MaxMemoryBytes != want100MB {
		t.Errorf("quality-review MaxMemoryBytes = %d, want %d", qrStep.MaxMemoryBytes, want100MB)
	}
	if qrStep.AvgMemoryBytes != want100MB {
		t.Errorf("quality-review AvgMemoryBytes = %d, want %d", qrStep.AvgMemoryBytes, want100MB)
	}
	wantQRCPU := int64(1_000_000_000)
	if qrStep.TotalCPUNanoseconds != wantQRCPU {
		t.Errorf("quality-review TotalCPUNanoseconds = %d, want %d", qrStep.TotalCPUNanoseconds, wantQRCPU)
	}
}

// TestAggregateResourceStats_MixedData: some runs have resource data, some don't;
// aggregation only counts non-zero samples.
func TestAggregateResourceStats_MixedData(t *testing.T) {
	// issue 1: has resource data; issue 2: no resource data
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Implement: rundata.StepResult{PeakMemoryBytes: 400 << 20, CPUNanoseconds: 4_000_000_000},
				},
				{
					Implement: rundata.StepResult{CostUSD: 1.0}, // no resource data
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.ResourceStats == nil {
		t.Fatal("ResourceStats = nil, want non-nil")
	}
	rs := got.ResourceStats

	// Max should be 400MB (only one non-zero sample).
	want400MB := int64(400 << 20)
	if rs.MaxPeakMemoryBytes != want400MB {
		t.Errorf("MaxPeakMemoryBytes = %d, want %d", rs.MaxPeakMemoryBytes, want400MB)
	}
	// Avg should also be 400MB (only one non-zero sample, not divided by 2).
	if rs.AvgPeakMemoryBytes != want400MB {
		t.Errorf("AvgPeakMemoryBytes = %d, want %d (only one non-zero sample)", rs.AvgPeakMemoryBytes, want400MB)
	}
	// AvgCPU: 4s / 2 issues = 2s
	wantAvgCPU := int64(2_000_000_000)
	if rs.AvgCPUNanosecondsPerIssue != wantAvgCPU {
		t.Errorf("AvgCPUNanosecondsPerIssue = %d, want %d", rs.AvgCPUNanosecondsPerIssue, wantAvgCPU)
	}
}

// TestAggregateResourceStats_RetrySteps: resource data from retry steps is aggregated.
func TestAggregateResourceStats_RetrySteps(t *testing.T) {
	runs := []rundata.RunDetail{
		{
			Issues: []rundata.IssueDetail{
				{
					Implement: rundata.StepResult{PeakMemoryBytes: 100 << 20, CPUNanoseconds: 1_000_000_000},
					Retries: []rundata.RetryDetail{
						{
							Retry:         rundata.StepResult{PeakMemoryBytes: 50 << 20, CPUNanoseconds: 500_000_000},
							QualityReview: rundata.StepResult{CPUNanoseconds: 200_000_000},
						},
					},
				},
			},
		},
	}

	got := Aggregate(runs)

	if got.ResourceStats == nil {
		t.Fatal("ResourceStats = nil, want non-nil")
	}
	rs := got.ResourceStats

	// TotalCPU: 1s + 0.5s + 0.2s = 1.7s
	wantCPU := int64(1_700_000_000)
	if rs.TotalCPUNanoseconds != wantCPU {
		t.Errorf("TotalCPUNanoseconds = %d, want %d", rs.TotalCPUNanoseconds, wantCPU)
	}

	// "retries" step should appear in ResourceByStep.
	retriesStep, ok := rs.ResourceByStep["retries"]
	if !ok {
		t.Fatal("ResourceByStep missing 'retries'")
	}
	// Retry step max memory = 50MB (quality-review had 0 memory).
	want50MB := int64(50 << 20)
	if retriesStep.MaxMemoryBytes != want50MB {
		t.Errorf("retries MaxMemoryBytes = %d, want %d", retriesStep.MaxMemoryBytes, want50MB)
	}
	// Retry CPU = 0.5s + 0.2s = 0.7s
	wantRetryCPU := int64(700_000_000)
	if retriesStep.TotalCPUNanoseconds != wantRetryCPU {
		t.Errorf("retries TotalCPUNanoseconds = %d, want %d", retriesStep.TotalCPUNanoseconds, wantRetryCPU)
	}
}
