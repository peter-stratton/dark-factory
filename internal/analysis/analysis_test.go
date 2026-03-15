package analysis

import (
	"math"
	"testing"

	"github.com/phs/dark-factory/internal/rundata"
)

// nearlyEqual returns true if a and b differ by less than epsilon.
func nearlyEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestAggregateNilRuns(t *testing.T) {
	got := Aggregate(nil)
	if got.RunCount != 0 || got.IssueCount != 0 || got.Outcomes != nil ||
		got.FlagFrequencies != nil || got.VerifyStats != nil {
		t.Errorf("Aggregate(nil) = %+v, want zero Report", got)
	}
}

func TestAggregateEmptyRuns(t *testing.T) {
	got := Aggregate([]rundata.RunDetail{})
	if got.RunCount != 0 || got.IssueCount != 0 || got.Outcomes != nil ||
		got.FlagFrequencies != nil || got.VerifyStats != nil {
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
