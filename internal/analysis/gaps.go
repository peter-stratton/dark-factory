package analysis

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/phs/dark-factory/internal/rundata"
)

// PromptGap describes a detected correlation between a condition and
// higher failure rates.
type PromptGap struct {
	Finding         string  `json:"finding"`
	FailRateWith    float64 `json:"fail_rate_with"`
	FailRateWithout float64 `json:"fail_rate_without"`
	SamplesWith     int     `json:"samples_with"`
	SamplesWithout  int     `json:"samples_without"`
}

const minGapSamples = 3

// DetectGaps compares failure rates across runs grouped by various
// conditions. Returns findings sorted by the absolute difference in
// failure rates (largest gap first). Conditions with fewer than 3
// samples on either side are excluded to avoid noise.
func DetectGaps(runs []rundata.RunDetail) []PromptGap {
	if len(runs) == 0 {
		return nil
	}

	var allIssues []rundata.IssueDetail
	for _, run := range runs {
		allIssues = append(allIssues, run.Issues...)
	}

	if len(allIssues) == 0 {
		return nil
	}

	var gaps []PromptGap

	// Quality reviewer step present vs absent.
	if gap := gapByCondition(allIssues, "quality reviewer", func(issue rundata.IssueDetail) bool {
		return isStepRecorded(issue.QualityReview)
	}); gap != nil {
		gaps = append(gaps, *gap)
	}

	// Scenario specs present vs absent.
	if gap := gapByCondition(allIssues, "scenario specs", func(issue rundata.IssueDetail) bool {
		return isStepRecorded(issue.SpecGenerator)
	}); gap != nil {
		gaps = append(gaps, *gap)
	}

	// Exhausted retries — list issue numbers and titles as a separate finding.
	var exhausted []rundata.IssueDetail
	for _, issue := range allIssues {
		if issue.Outcome.Status == "needs-human-review" {
			exhausted = append(exhausted, issue)
		}
	}
	if len(exhausted) > 0 {
		parts := make([]string, 0, len(exhausted))
		for _, issue := range exhausted {
			parts = append(parts, fmt.Sprintf("#%d %q", issue.IssueNumber, issue.Outcome.Title))
		}
		gaps = append(gaps, PromptGap{
			Finding:     "exhausted retries: " + strings.Join(parts, ", "),
			FailRateWith: 1.0,
			SamplesWith:  len(exhausted),
		})
	}

	if len(gaps) == 0 {
		return nil
	}

	sort.Slice(gaps, func(i, j int) bool {
		di := math.Abs(gaps[i].FailRateWith - gaps[i].FailRateWithout)
		dj := math.Abs(gaps[j].FailRateWith - gaps[j].FailRateWithout)
		return di > dj
	})

	return gaps
}

// gapByCondition splits issues into "with" and "without" groups based on cond,
// then compares failure rates. Returns nil if either group has fewer than minGapSamples.
func gapByCondition(
	issues []rundata.IssueDetail,
	finding string,
	cond func(rundata.IssueDetail) bool,
) *PromptGap {
	var with, without []rundata.IssueDetail
	for _, issue := range issues {
		if cond(issue) {
			with = append(with, issue)
		} else {
			without = append(without, issue)
		}
	}

	if len(with) < minGapSamples || len(without) < minGapSamples {
		return nil
	}

	return &PromptGap{
		Finding:         finding,
		FailRateWith:    issueFailureRate(with),
		FailRateWithout: issueFailureRate(without),
		SamplesWith:     len(with),
		SamplesWithout:  len(without),
	}
}

// issueFailureRate returns the fraction of issues with "failed" or
// "needs-human-review" status.
func issueFailureRate(issues []rundata.IssueDetail) float64 {
	if len(issues) == 0 {
		return 0
	}
	var count int
	for _, issue := range issues {
		if issue.Outcome.Status == "failed" || issue.Outcome.Status == "needs-human-review" {
			count++
		}
	}
	return float64(count) / float64(len(issues))
}

// isStepRecorded reports whether a StepResult contains any recorded data.
func isStepRecorded(step rundata.StepResult) bool {
	return step.StartedAt != nil || step.Output != ""
}
