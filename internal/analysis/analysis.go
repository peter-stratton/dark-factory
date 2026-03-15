package analysis

import (
	"sort"

	"github.com/phs/dark-factory/internal/rundata"
)

// Report holds aggregate statistics computed across multiple runs.
type Report struct {
	RunCount        int             `json:"run_count"`
	IssueCount      int             `json:"issue_count"`
	Outcomes        map[string]int  `json:"outcomes"`
	FlagFrequencies []FlagFrequency `json:"flag_frequencies"`
	RetryStats      RetryStats      `json:"retry_stats"`
	VerifyStats     map[string]int  `json:"verify_stats"`
	CostStats       CostStats       `json:"cost_stats"`
	DurationStats   DurationStats   `json:"duration_stats"`
}

// DurationStats holds aggregate step duration statistics.
type DurationStats struct {
	AvgImplementSeconds float64 `json:"avg_implement_seconds"` // average across all issues
	AvgReviewSeconds    float64 `json:"avg_review_seconds"`    // average across all issues
}

// FlagFrequency records how often a quality flag code appears across all issues.
type FlagFrequency struct {
	Code    string  `json:"code"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"` // of total issues
}

// RetryStats holds aggregate retry statistics.
type RetryStats struct {
	TotalRetries   int     `json:"total_retries"`
	AvgPerIssue    float64 `json:"avg_per_issue"`
	MaxRetries     int     `json:"max_retries"`
	ExhaustedCount int     `json:"exhausted_count"` // issues with status "needs-human-review"
	RetriedCount   int     `json:"retried_count"`   // issues that had at least one retry
	RecoveryRate   float64 `json:"recovery_rate"`   // fraction of retried issues that eventually succeeded
}

// CostStats holds aggregate cost statistics.
type CostStats struct {
	TotalUSD       float64            `json:"total_usd"`
	AvgPerIssueUSD float64            `json:"avg_per_issue_usd"`
	AvgPerRunUSD   float64            `json:"avg_per_run_usd"`
	CostByStep     map[string]float64 `json:"cost_by_step"`
}

// Aggregate computes statistics across the provided runs.
// Returns a zero Report if runs is empty.
func Aggregate(runs []rundata.RunDetail) Report {
	if len(runs) == 0 {
		return Report{}
	}

	report := Report{
		RunCount:    len(runs),
		Outcomes:    make(map[string]int),
		VerifyStats: make(map[string]int),
	}
	report.CostStats.CostByStep = make(map[string]float64)

	flagCounts := make(map[string]int)
	var totalRetries int
	var maxRetries int
	var totalCost float64
	var retriedSucceeded int
	var totalImplementDuration, totalReviewDuration float64

	for _, run := range runs {
		for _, issue := range run.Issues {
			report.IssueCount++

			// Outcome distribution.
			if issue.Outcome.Status != "" {
				report.Outcomes[issue.Outcome.Status]++
			}

			// Retry statistics.
			retries := len(issue.Retries)
			totalRetries += retries
			if retries > maxRetries {
				maxRetries = retries
			}
			if issue.Outcome.Status == rundata.OutcomeStatusNeedsHumanReview {
				report.RetryStats.ExhaustedCount++
			}
			if retries > 0 {
				report.RetryStats.RetriedCount++
				if issue.Outcome.Status == rundata.OutcomeStatusImplemented ||
					issue.Outcome.Status == rundata.OutcomeStatusReadyToMerge {
					retriedSucceeded++
				}
			}

			// Collect flags from all step results for this issue.
			collectFlags(flagCounts, issue.Implement.Flags)
			collectFlags(flagCounts, issue.QualityReview.Flags)
			collectFlags(flagCounts, issue.FunctionalReview.Flags)
			for _, retry := range issue.Retries {
				collectFlags(flagCounts, retry.Retry.Flags)
				collectFlags(flagCounts, retry.QualityReview.Flags)
				collectFlags(flagCounts, retry.FunctionalReview.Flags)
			}

			// Cost statistics from all step results.
			totalCost += accumulateIssueCosts(report.CostStats.CostByStep, issue)

			// Duration statistics from implement and quality-review steps.
			totalImplementDuration += issue.Implement.DurationSeconds
			totalReviewDuration += issue.QualityReview.DurationSeconds

			// Verify step failure counts by check name.
			for _, vr := range issue.VerifyResults {
				for _, check := range vr.Checks {
					if !check.Passed {
						report.VerifyStats[check.Name]++
					}
				}
			}
		}
	}

	// Populate retry stats.
	report.RetryStats.TotalRetries = totalRetries
	report.RetryStats.MaxRetries = maxRetries
	if report.IssueCount > 0 {
		report.RetryStats.AvgPerIssue = float64(totalRetries) / float64(report.IssueCount)
	}
	if report.RetryStats.RetriedCount > 0 {
		report.RetryStats.RecoveryRate = float64(retriedSucceeded) / float64(report.RetryStats.RetriedCount)
	}

	// Populate cost stats.
	report.CostStats.TotalUSD = totalCost
	if report.IssueCount > 0 {
		report.CostStats.AvgPerIssueUSD = totalCost / float64(report.IssueCount)
	}
	report.CostStats.AvgPerRunUSD = totalCost / float64(report.RunCount)

	// Populate duration stats.
	if report.IssueCount > 0 {
		report.DurationStats.AvgImplementSeconds = totalImplementDuration / float64(report.IssueCount)
		report.DurationStats.AvgReviewSeconds = totalReviewDuration / float64(report.IssueCount)
	}

	// Build flag frequencies sorted by count descending, then code ascending for stability.
	report.FlagFrequencies = buildFlagFrequencies(flagCounts, report.IssueCount)

	return report
}

// buildFlagFrequencies converts raw flag counts into a sorted FlagFrequency slice.
func buildFlagFrequencies(flagCounts map[string]int, issueCount int) []FlagFrequency {
	var freqs []FlagFrequency
	for code, count := range flagCounts {
		var pct float64
		if issueCount > 0 {
			pct = float64(count) / float64(issueCount) * 100
		}
		freqs = append(freqs, FlagFrequency{
			Code:    code,
			Count:   count,
			Percent: pct,
		})
	}
	sort.Slice(freqs, func(i, j int) bool {
		if freqs[i].Count != freqs[j].Count {
			return freqs[i].Count > freqs[j].Count
		}
		return freqs[i].Code < freqs[j].Code
	})
	return freqs
}

// collectFlags increments the count for each flag code in flagCounts.
func collectFlags(flagCounts map[string]int, flags []rundata.Flag) {
	for _, f := range flags {
		flagCounts[f.Code]++
	}
}

// accumulateStepCost adds cost to the named step's entry in costByStep.
// Zero cost values are ignored so the map only contains steps that incurred cost.
func accumulateStepCost(costByStep map[string]float64, step string, cost float64) {
	if cost > 0 {
		costByStep[step] += cost
	}
}

// accumulateIssueCosts records per-step costs for a single issue into costByStep
// and returns the total cost across all steps including retries.
func accumulateIssueCosts(costByStep map[string]float64, issue rundata.IssueDetail) float64 {
	total := issue.Recon.CostUSD + issue.SpecGenerator.CostUSD +
		issue.Implement.CostUSD + issue.QualityReview.CostUSD + issue.FunctionalReview.CostUSD
	accumulateStepCost(costByStep, "recon", issue.Recon.CostUSD)
	accumulateStepCost(costByStep, "spec-generator", issue.SpecGenerator.CostUSD)
	accumulateStepCost(costByStep, "implement", issue.Implement.CostUSD)
	accumulateStepCost(costByStep, "quality-review", issue.QualityReview.CostUSD)
	accumulateStepCost(costByStep, "functional-review", issue.FunctionalReview.CostUSD)
	for _, retry := range issue.Retries {
		total += retry.Retry.CostUSD + retry.QualityReview.CostUSD + retry.FunctionalReview.CostUSD
		accumulateStepCost(costByStep, "retries", retry.Retry.CostUSD)
		accumulateStepCost(costByStep, "retries", retry.QualityReview.CostUSD)
		accumulateStepCost(costByStep, "retries", retry.FunctionalReview.CostUSD)
	}
	return total
}
