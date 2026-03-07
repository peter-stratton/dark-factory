package analysis

import (
	"sort"

	"github.com/phs/dark-factory/internal/rundata"
)

// Report holds aggregate statistics computed across multiple runs.
type Report struct {
	RunCount        int              `json:"run_count"`
	IssueCount      int              `json:"issue_count"`
	Outcomes        map[string]int   `json:"outcomes"`
	FlagFrequencies []FlagFrequency  `json:"flag_frequencies"`
	RetryStats      RetryStats       `json:"retry_stats"`
	VerifyStats     map[string]int   `json:"verify_stats"`
	CostStats       CostStats        `json:"cost_stats"`
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
}

// CostStats holds aggregate cost statistics.
type CostStats struct {
	TotalUSD       float64 `json:"total_usd"`
	AvgPerIssueUSD float64 `json:"avg_per_issue_usd"`
	AvgPerRunUSD   float64 `json:"avg_per_run_usd"`
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

	flagCounts := make(map[string]int)
	var totalRetries int
	var maxRetries int
	var totalCost float64

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
			if issue.Outcome.Status == "needs-human-review" {
				report.RetryStats.ExhaustedCount++
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
			totalCost += issue.SpecGenerator.CostUSD
			totalCost += issue.Implement.CostUSD
			totalCost += issue.QualityReview.CostUSD
			totalCost += issue.FunctionalReview.CostUSD
			for _, retry := range issue.Retries {
				totalCost += retry.Retry.CostUSD
				totalCost += retry.QualityReview.CostUSD
				totalCost += retry.FunctionalReview.CostUSD
			}

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

	// Populate cost stats.
	report.CostStats.TotalUSD = totalCost
	if report.IssueCount > 0 {
		report.CostStats.AvgPerIssueUSD = totalCost / float64(report.IssueCount)
	}
	report.CostStats.AvgPerRunUSD = totalCost / float64(report.RunCount)

	// Build flag frequencies sorted by count descending, then code ascending for stability.
	for code, count := range flagCounts {
		var pct float64
		if report.IssueCount > 0 {
			pct = float64(count) / float64(report.IssueCount) * 100
		}
		report.FlagFrequencies = append(report.FlagFrequencies, FlagFrequency{
			Code:    code,
			Count:   count,
			Percent: pct,
		})
	}
	sort.Slice(report.FlagFrequencies, func(i, j int) bool {
		if report.FlagFrequencies[i].Count != report.FlagFrequencies[j].Count {
			return report.FlagFrequencies[i].Count > report.FlagFrequencies[j].Count
		}
		return report.FlagFrequencies[i].Code < report.FlagFrequencies[j].Code
	})

	return report
}

// collectFlags increments the count for each flag code in flagCounts.
func collectFlags(flagCounts map[string]int, flags []rundata.Flag) {
	for _, f := range flags {
		flagCounts[f.Code]++
	}
}
