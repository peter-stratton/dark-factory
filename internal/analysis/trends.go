package analysis

import (
	"sort"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
)

// TrendPoint represents one data point in a time series, corresponding
// to a single run.
type TrendPoint struct {
	Timestamp            time.Time `json:"timestamp"`
	Repo                 string    `json:"repo"`
	Milestone            string    `json:"milestone"`
	IssueCount           int       `json:"issue_count"`
	SuccessRate          float64   `json:"success_rate"` // 0.0–1.0
	AvgRetries           float64   `json:"avg_retries"`
	TotalCostUSD         float64   `json:"total_cost_usd"`
	AvgCostPerIssueUSD   float64   `json:"avg_cost_per_issue_usd"`
	AvgImplementDuration float64   `json:"avg_implement_duration"` // seconds
	AvgReviewDuration    float64   `json:"avg_review_duration"`    // seconds
}

// ComputeTrends returns one TrendPoint per run, sorted chronologically
// (oldest first). Unfinished runs (no FinishedAt) are excluded.
func ComputeTrends(runs []rundata.RunDetail) []TrendPoint {
	if len(runs) == 0 {
		return nil
	}

	var points []TrendPoint
	for _, run := range runs {
		if run.FinishedAt == nil {
			continue
		}

		issueCount := len(run.Issues)
		var implemented, totalRetries int
		var totalCost, totalImplementDuration, totalReviewDuration float64

		for _, issue := range run.Issues {
			if issue.Outcome.Status == "implemented" {
				implemented++
			}
			totalRetries += len(issue.Retries)

			totalCost += issue.SpecGenerator.CostUSD
			totalCost += issue.Implement.CostUSD
			totalCost += issue.QualityReview.CostUSD
			totalCost += issue.FunctionalReview.CostUSD
			for _, retry := range issue.Retries {
				totalCost += retry.Retry.CostUSD
				totalCost += retry.QualityReview.CostUSD
				totalCost += retry.FunctionalReview.CostUSD
			}

			totalImplementDuration += issue.Implement.DurationSeconds
			totalReviewDuration += issue.QualityReview.DurationSeconds
		}

		var successRate, avgRetries, avgCostPerIssue, avgImplementDuration, avgReviewDuration float64
		if issueCount > 0 {
			successRate = float64(implemented) / float64(issueCount)
			avgRetries = float64(totalRetries) / float64(issueCount)
			avgCostPerIssue = totalCost / float64(issueCount)
			avgImplementDuration = totalImplementDuration / float64(issueCount)
			avgReviewDuration = totalReviewDuration / float64(issueCount)
		}

		points = append(points, TrendPoint{
			Timestamp:            run.StartedAt,
			Repo:                 run.Repo,
			Milestone:            run.Milestone,
			IssueCount:           issueCount,
			SuccessRate:          successRate,
			AvgRetries:           avgRetries,
			TotalCostUSD:         totalCost,
			AvgCostPerIssueUSD:   avgCostPerIssue,
			AvgImplementDuration: avgImplementDuration,
			AvgReviewDuration:    avgReviewDuration,
		})
	}

	if len(points) == 0 {
		return nil
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})

	return points
}
