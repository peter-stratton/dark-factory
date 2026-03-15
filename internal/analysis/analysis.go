package analysis

import (
	"sort"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
)

// Report holds aggregate statistics computed across multiple runs.
type Report struct {
	RunCount             int                    `json:"run_count"`
	IssueCount           int                    `json:"issue_count"`
	Outcomes             map[string]int         `json:"outcomes"`
	FlagFrequencies      []FlagFrequency        `json:"flag_frequencies"`
	RetryStats           RetryStats             `json:"retry_stats"`
	VerifyStats          map[string]int         `json:"verify_stats"`
	CostStats            CostStats              `json:"cost_stats"`
	DurationStats        DurationStats          `json:"duration_stats"`
	RepoStats            map[string]RepoSummary `json:"repo_stats"`
	FirstPassCount       int                    `json:"first_pass_count"`
	FirstPassSuccessRate float64                `json:"first_pass_success_rate"`
	WastedCostUSD        float64                `json:"wasted_cost_usd"`
	AvgCostPerSuccessUSD float64                `json:"avg_cost_per_success_usd"`
	FailureReasons       map[string]int         `json:"failure_reasons"`
	TimeoutRate          float64                `json:"timeout_rate"`
}

// RepoSummary holds per-repository outcome statistics.
type RepoSummary struct {
	Total              int     `json:"total"`
	Implemented        int     `json:"implemented"`
	Failed             int     `json:"failed"`
	SuccessRate        float64 `json:"success_rate"`
	FirstPassCount     int     `json:"first_pass_count"`
	FirstPassRate      float64 `json:"first_pass_rate"`
	TotalCostUSD       float64 `json:"total_cost_usd"`
	AvgCostPerIssueUSD float64 `json:"avg_cost_per_issue_usd"`
}

// DurationStats holds aggregate step duration statistics.
type DurationStats struct {
	AvgImplementSeconds    float64            `json:"avg_implement_seconds"`     // average across all issues
	AvgReviewSeconds       float64            `json:"avg_review_seconds"`        // average across all issues
	DurationByStep         map[string]float64 `json:"duration_by_step"`          // step name to total seconds
	AvgTimeToMergeSeconds  float64            `json:"avg_time_to_merge_seconds"` // end-to-end for implemented issues
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

// issueAcc holds mutable per-issue counters accumulated during Aggregate.
type issueAcc struct {
	flagCounts             map[string]int
	totalRetries           int
	maxRetries             int
	retriedSucceeded       int
	totalCost              float64
	totalImplementDuration float64
	totalReviewDuration    float64
	firstPassCount         int
	wastedCost             float64
	timedOutSteps          int
	totalSteps             int
	totalTimeToMerge       float64
	implementedCount       int
}

// add incorporates a single issue's data into the accumulator and the report.
func (a *issueAcc) add(report *Report, repo string, issue rundata.IssueDetail) {
	report.IssueCount++

	if issue.Outcome.Status != "" {
		report.Outcomes[issue.Outcome.Status]++
	}

	retries := len(issue.Retries)
	a.totalRetries += retries
	if retries > a.maxRetries {
		a.maxRetries = retries
	}
	if issue.Outcome.Status == rundata.OutcomeStatusNeedsHumanReview {
		report.RetryStats.ExhaustedCount++
	}
	if retries > 0 {
		report.RetryStats.RetriedCount++
		if issue.Outcome.Status == rundata.OutcomeStatusImplemented ||
			issue.Outcome.Status == rundata.OutcomeStatusReadyToMerge {
			a.retriedSucceeded++
		}
	}

	collectFlags(a.flagCounts, issue.Implement.Flags)
	collectFlags(a.flagCounts, issue.QualityReview.Flags)
	collectFlags(a.flagCounts, issue.FunctionalReview.Flags)
	for _, retry := range issue.Retries {
		collectFlags(a.flagCounts, retry.Retry.Flags)
		collectFlags(a.flagCounts, retry.QualityReview.Flags)
		collectFlags(a.flagCounts, retry.FunctionalReview.Flags)
	}

	// Compute cost before accumulateRepoStat so the per-issue cost is available.
	issueCost := accumulateIssueCosts(report.CostStats.CostByStep, issue)
	a.totalCost += issueCost

	// First-pass: succeeded on the initial attempt (no retries).
	isFirstPass := retries == 0 &&
		(issue.Outcome.Status == rundata.OutcomeStatusImplemented ||
			issue.Outcome.Status == rundata.OutcomeStatusReadyToMerge)
	if isFirstPass {
		a.firstPassCount++
	}

	// Wasted cost: issues that ultimately failed or exhausted retries.
	if issue.Outcome.Status == rundata.OutcomeStatusFailed ||
		issue.Outcome.Status == rundata.OutcomeStatusNeedsHumanReview {
		a.wastedCost += issueCost
	}

	accumulateRepoStat(report.RepoStats, repo, issue.Outcome.Status, isFirstPass, issueCost)

	accumulateIssueDurations(report.DurationStats.DurationByStep, issue)
	a.totalImplementDuration += issue.Implement.DurationSeconds
	a.totalReviewDuration += issue.QualityReview.DurationSeconds

	accumulateVerifyStats(report.VerifyStats, issue.VerifyResults)

	// Accumulate timeout counters across all step results for this issue.
	allSteps := issueStepResults(issue)
	a.totalSteps += len(allSteps)
	for _, s := range allSteps {
		if s.TimedOut {
			a.timedOutSteps++
		}
	}

	// Categorize failed issues into mutually exclusive failure reason buckets.
	categorizeFailureReason(report.FailureReasons, issue, allSteps)

	// Time-to-merge: accumulate for implemented/ready-to-merge issues.
	if issue.Outcome.Status == rundata.OutcomeStatusImplemented ||
		issue.Outcome.Status == rundata.OutcomeStatusReadyToMerge {
		if d := issueElapsedSeconds(allSteps); d > 0 {
			a.totalTimeToMerge += d
			a.implementedCount++
		}
	}
}

// accumulateRepoStat updates per-repository counters for a single issue outcome.
func accumulateRepoStat(repoStats map[string]RepoSummary, repo, status string, isFirstPass bool, cost float64) {
	if repo == "" {
		return
	}
	rs := repoStats[repo]
	rs.Total++
	switch status {
	case rundata.OutcomeStatusImplemented, rundata.OutcomeStatusReadyToMerge:
		rs.Implemented++
	case rundata.OutcomeStatusFailed, rundata.OutcomeStatusNeedsHumanReview:
		rs.Failed++
	}
	if isFirstPass {
		rs.FirstPassCount++
	}
	rs.TotalCostUSD += cost
	repoStats[repo] = rs
}

// Aggregate computes statistics across the provided runs.
// Returns a zero Report if runs is empty.
func Aggregate(runs []rundata.RunDetail) Report {
	if len(runs) == 0 {
		return Report{}
	}

	report := Report{
		RunCount:       len(runs),
		Outcomes:       make(map[string]int),
		VerifyStats:    make(map[string]int),
		RepoStats:      make(map[string]RepoSummary),
		FailureReasons: make(map[string]int),
	}
	report.CostStats.CostByStep = make(map[string]float64)
	report.DurationStats.DurationByStep = make(map[string]float64)

	acc := issueAcc{flagCounts: make(map[string]int)}

	for _, run := range runs {
		for _, issue := range run.Issues {
			acc.add(&report, run.Repo, issue)
		}
	}

	// Populate retry stats.
	report.RetryStats.TotalRetries = acc.totalRetries
	report.RetryStats.MaxRetries = acc.maxRetries
	if report.IssueCount > 0 {
		report.RetryStats.AvgPerIssue = float64(acc.totalRetries) / float64(report.IssueCount)
	}
	if report.RetryStats.RetriedCount > 0 {
		report.RetryStats.RecoveryRate = float64(acc.retriedSucceeded) / float64(report.RetryStats.RetriedCount)
	}

	// Populate cost stats.
	report.CostStats.TotalUSD = acc.totalCost
	if report.IssueCount > 0 {
		report.CostStats.AvgPerIssueUSD = acc.totalCost / float64(report.IssueCount)
	}
	report.CostStats.AvgPerRunUSD = acc.totalCost / float64(report.RunCount)

	// Populate duration stats.
	if report.IssueCount > 0 {
		report.DurationStats.AvgImplementSeconds = acc.totalImplementDuration / float64(report.IssueCount)
		report.DurationStats.AvgReviewSeconds = acc.totalReviewDuration / float64(report.IssueCount)
	}

	// Populate first-pass and wasted-cost stats.
	report.FirstPassCount = acc.firstPassCount
	if report.IssueCount > 0 {
		report.FirstPassSuccessRate = float64(acc.firstPassCount) / float64(report.IssueCount)
	}
	report.WastedCostUSD = acc.wastedCost
	if implementedCount := report.Outcomes["implemented"]; implementedCount > 0 {
		report.AvgCostPerSuccessUSD = acc.totalCost / float64(implementedCount)
	}

	// Compute per-repo rates.
	for repo, rs := range report.RepoStats {
		if rs.Total > 0 {
			rs.SuccessRate = float64(rs.Implemented) / float64(rs.Total)
			rs.FirstPassRate = float64(rs.FirstPassCount) / float64(rs.Total)
			rs.AvgCostPerIssueUSD = rs.TotalCostUSD / float64(rs.Total)
		}
		report.RepoStats[repo] = rs
	}

	// Populate timeout rate.
	if acc.totalSteps > 0 {
		report.TimeoutRate = float64(acc.timedOutSteps) / float64(acc.totalSteps)
	}

	// Populate average time-to-merge.
	if acc.implementedCount > 0 {
		report.DurationStats.AvgTimeToMergeSeconds = acc.totalTimeToMerge / float64(acc.implementedCount)
	}

	// Build flag frequencies sorted by count descending, then code ascending for stability.
	report.FlagFrequencies = buildFlagFrequencies(acc.flagCounts, report.IssueCount)

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

// accumulateVerifyStats increments the failure count for each failed verify check.
func accumulateVerifyStats(verifyStats map[string]int, verifyResults []rundata.VerifyStepResult) {
	for _, vr := range verifyResults {
		for _, check := range vr.Checks {
			if !check.Passed {
				verifyStats[check.Name]++
			}
		}
	}
}

// accumulateStepCost adds cost to the named step's entry in costByStep.
// Zero cost values are ignored so the map only contains steps that incurred cost.
func accumulateStepCost(costByStep map[string]float64, step string, cost float64) {
	if cost > 0 {
		costByStep[step] += cost
	}
}

// accumulateStepDuration adds duration to the named step's entry in durationByStep.
// Zero duration values are ignored so the map only contains steps that ran.
func accumulateStepDuration(durationByStep map[string]float64, step string, duration float64) {
	if duration > 0 {
		durationByStep[step] += duration
	}
}

// accumulateIssueDurations records per-step durations for a single issue into durationByStep.
// Note: VerifyStepResult does not have a DurationSeconds field so verify duration is not tracked.
func accumulateIssueDurations(durationByStep map[string]float64, issue rundata.IssueDetail) {
	accumulateStepDuration(durationByStep, "recon", issue.Recon.DurationSeconds)
	accumulateStepDuration(durationByStep, "spec-generator", issue.SpecGenerator.DurationSeconds)
	accumulateStepDuration(durationByStep, "implement", issue.Implement.DurationSeconds)
	accumulateStepDuration(durationByStep, "quality-review", issue.QualityReview.DurationSeconds)
	accumulateStepDuration(durationByStep, "functional-review", issue.FunctionalReview.DurationSeconds)
	for _, retry := range issue.Retries {
		accumulateStepDuration(durationByStep, "retries", retry.Retry.DurationSeconds)
		accumulateStepDuration(durationByStep, "retries", retry.QualityReview.DurationSeconds)
		accumulateStepDuration(durationByStep, "retries", retry.FunctionalReview.DurationSeconds)
	}
}

// issueStepResults returns all StepResult values for an issue in a flat slice.
// This includes the main steps plus all retry steps.
func issueStepResults(issue rundata.IssueDetail) []rundata.StepResult {
	steps := []rundata.StepResult{
		issue.Recon,
		issue.SpecGenerator,
		issue.Implement,
		issue.QualityReview,
		issue.FunctionalReview,
	}
	for _, retry := range issue.Retries {
		steps = append(steps, retry.Retry, retry.QualityReview, retry.FunctionalReview)
	}
	return steps
}

// categorizeFailureReason assigns the issue's failure reason to one of four
// mutually exclusive buckets: "exhaustion", "verify", "timeout", or "error".
// Only failed and needs-human-review issues are categorized.
func categorizeFailureReason(failureReasons map[string]int, issue rundata.IssueDetail, allSteps []rundata.StepResult) {
	switch issue.Outcome.Status {
	case rundata.OutcomeStatusNeedsHumanReview:
		failureReasons["exhaustion"]++
	case rundata.OutcomeStatusFailed:
		// Check verify failure: last verify step has AllPassed == false.
		if len(issue.VerifyResults) > 0 {
			last := issue.VerifyResults[len(issue.VerifyResults)-1]
			if !last.AllPassed {
				failureReasons["verify"]++
				return
			}
		}
		// Check timeout: any step timed out.
		for _, s := range allSteps {
			if s.TimedOut {
				failureReasons["timeout"]++
				return
			}
		}
		// Fallback.
		failureReasons["error"]++
	}
}

// issueElapsedSeconds returns the wall-clock duration of an issue in seconds
// by finding the earliest StartedAt and latest FinishedAt across all step results.
// Returns 0 if no timing data is available.
func issueElapsedSeconds(allSteps []rundata.StepResult) float64 {
	var earliest, latest *time.Time
	for _, s := range allSteps {
		if s.StartedAt != nil {
			if earliest == nil || s.StartedAt.Before(*earliest) {
				t := *s.StartedAt
				earliest = &t
			}
		}
		if s.FinishedAt != nil {
			if latest == nil || s.FinishedAt.After(*latest) {
				t := *s.FinishedAt
				latest = &t
			}
		}
	}
	if earliest == nil || latest == nil {
		return 0
	}
	return latest.Sub(*earliest).Seconds()
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
