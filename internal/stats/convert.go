package stats

import (
	"sort"
	"strconv"
	"strings"

	"github.com/peter-stratton/dark-factory/internal/rundata"
)

// ToRunDetails converts flat stats DB records into []rundata.RunDetail suitable
// for the analysis functions (Aggregate, DetectGaps, ComputeTrends).
//
// The conversion is lossy: step Output, Error, ToolTrace, and SessionID fields
// are not stored in the stats DB and will be empty in the returned structs.
// The fields used by the analysis package — Outcome.Status, Retries length,
// CostUSD, Flags, and step StartedAt — are all populated.
func ToRunDetails(runs []RunRecord, outcomes []IssueOutcomeRecord, steps []StepResultRecord) []rundata.RunDetail {
	if len(runs) == 0 {
		return nil
	}

	// Index outcomes by run_id → issue_number → record.
	outcomesByRun := make(map[string]map[int]IssueOutcomeRecord, len(runs))
	for _, o := range outcomes {
		if outcomesByRun[o.RunID] == nil {
			outcomesByRun[o.RunID] = make(map[int]IssueOutcomeRecord)
		}
		outcomesByRun[o.RunID][o.IssueNumber] = o
	}

	// Index steps by run_id → issue_number → step_name → record.
	stepsByRun := make(map[string]map[int]map[string]StepResultRecord, len(runs))
	for _, s := range steps {
		if stepsByRun[s.RunID] == nil {
			stepsByRun[s.RunID] = make(map[int]map[string]StepResultRecord)
		}
		if stepsByRun[s.RunID][s.IssueNumber] == nil {
			stepsByRun[s.RunID][s.IssueNumber] = make(map[string]StepResultRecord)
		}
		stepsByRun[s.RunID][s.IssueNumber][s.StepName] = s
	}

	details := make([]rundata.RunDetail, 0, len(runs))
	for _, run := range runs {
		rd := rundata.RunDetail{
			RunMeta: toRunMeta(run),
		}

		runOutcomes := outcomesByRun[run.ID]
		runSteps := stepsByRun[run.ID]

		// Collect all issue numbers that appear in outcomes or steps.
		issueNums := collectIssueNumbers(runOutcomes, runSteps)

		for _, issueNum := range issueNums {
			issueSteps := runSteps[issueNum]

			issue := rundata.IssueDetail{
				IssueNumber: issueNum,
			}

			if o, ok := runOutcomes[issueNum]; ok {
				issue.Outcome = rundata.Outcome{
					IssueNumber: o.IssueNumber,
					Title:       o.Title,
					Status:      o.Status,
					PRNumber:    o.PRNumber,
					Error:       o.Error,
					TraceID:     o.TraceID,
				}
			}

			// Populate named step fields.
			if s, ok := issueSteps["recon"]; ok {
				issue.Recon = toStepResult(s)
			}
			if s, ok := issueSteps["spec-generator"]; ok {
				issue.SpecGenerator = toStepResult(s)
			}
			if s, ok := issueSteps["implement"]; ok {
				issue.Implement = toStepResult(s)
			}
			if s, ok := issueSteps["quality-review"]; ok {
				issue.QualityReview = toStepResult(s)
			}
			if s, ok := issueSteps["functional-review"]; ok {
				issue.FunctionalReview = toStepResult(s)
			}

			// Reconstruct retries from step names of the form "retry-N",
			// "retry-N-quality-review", and "retry-N-functional-review".
			issue.Retries = buildRetries(issueSteps)

			rd.Issues = append(rd.Issues, issue)
		}

		details = append(details, rd)
	}

	return details
}

// toRunMeta converts a RunRecord into a rundata.RunMeta.
func toRunMeta(r RunRecord) rundata.RunMeta {
	meta := rundata.RunMeta{
		Repo:       r.Repo,
		Milestone:  r.Milestone,
		BaseBranch: r.BaseBranch,
		StartedAt:  r.StartedAt,
	}

	if r.AutoMergeFeature != "" || r.AutoMergeRollup != "" {
		meta.AutoMerge = &rundata.AutoMerge{
			Feature: r.AutoMergeFeature,
			Rollup:  r.AutoMergeRollup,
		}
	}

	if !r.FinishedAt.IsZero() {
		t := r.FinishedAt
		meta.FinishedAt = &t
	}

	if r.Total > 0 || r.Implemented > 0 || r.Failed > 0 || r.AbortReason != "" {
		meta.Summary = &rundata.RunSummary{
			Total:       r.Total,
			Implemented: r.Implemented,
			Failed:      r.Failed,
			AbortReason: r.AbortReason,
		}
	}

	return meta
}

// toStepResult converts a StepResultRecord into a rundata.StepResult.
// Output, Error, ToolTrace, and SessionID are not stored in the DB and remain empty.
func toStepResult(s StepResultRecord) rundata.StepResult {
	step := rundata.StepResult{
		CostUSD:         s.CostUSD,
		DurationSeconds: s.DurationSeconds,
		PeakMemoryBytes: s.PeakMemoryBytes,
		CPUNanoseconds:  s.CPUNanoseconds,
		TraceID:         s.TraceID,
		Prompt:          s.Prompt,
	}

	if !s.StartedAt.IsZero() {
		t := s.StartedAt
		step.StartedAt = &t
	}
	if !s.FinishedAt.IsZero() {
		t := s.FinishedAt
		step.FinishedAt = &t
	}

	if len(s.Flags) > 0 {
		step.Flags = make([]rundata.Flag, 0, len(s.Flags))
		for _, code := range s.Flags {
			step.Flags = append(step.Flags, rundata.Flag{Code: code})
		}
	}

	return step
}

// buildRetries reconstructs []rundata.RetryDetail from a map of step_name → record.
// Step names matching "retry-N", "retry-N-quality-review", and
// "retry-N-functional-review" are grouped by attempt number N.
func buildRetries(steps map[string]StepResultRecord) []rundata.RetryDetail {
	if len(steps) == 0 {
		return nil
	}

	// Group step records by retry attempt number.
	type retryGroup struct {
		retry            *StepResultRecord
		qualityReview    *StepResultRecord
		functionalReview *StepResultRecord
	}
	groups := make(map[int]*retryGroup)

	for name, s := range steps {
		n, sub, ok := parseRetryStepName(name)
		if !ok {
			continue
		}
		if groups[n] == nil {
			groups[n] = &retryGroup{}
		}
		cp := s
		switch sub {
		case "":
			groups[n].retry = &cp
		case "quality-review":
			groups[n].qualityReview = &cp
		case "functional-review":
			groups[n].functionalReview = &cp
		}
	}

	if len(groups) == 0 {
		return nil
	}

	// Sort by attempt number for deterministic output.
	attempts := make([]int, 0, len(groups))
	for n := range groups {
		attempts = append(attempts, n)
	}
	sort.Ints(attempts)

	retries := make([]rundata.RetryDetail, 0, len(attempts))
	for _, n := range attempts {
		g := groups[n]
		rd := rundata.RetryDetail{Attempt: n}
		if g.retry != nil {
			rd.Retry = toStepResult(*g.retry)
		}
		if g.qualityReview != nil {
			rd.QualityReview = toStepResult(*g.qualityReview)
		}
		if g.functionalReview != nil {
			rd.FunctionalReview = toStepResult(*g.functionalReview)
		}
		retries = append(retries, rd)
	}

	return retries
}

// parseRetryStepName parses a step name of the form "retry-N" or
// "retry-N-quality-review" or "retry-N-functional-review".
// Returns (attemptNumber, subStep, true) on success.
// subStep is "" for the base retry step, "quality-review" or "functional-review" otherwise.
func parseRetryStepName(name string) (int, string, bool) {
	if !strings.HasPrefix(name, "retry-") {
		return 0, "", false
	}
	rest := strings.TrimPrefix(name, "retry-")

	// rest may be "N", "N-quality-review", or "N-functional-review".
	parts := strings.SplitN(rest, "-", 2)
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}

	var sub string
	if len(parts) == 2 {
		sub = parts[1]
		if sub != "quality-review" && sub != "functional-review" {
			return 0, "", false
		}
	}

	return n, sub, true
}

// collectIssueNumbers returns a sorted, deduplicated list of issue numbers
// appearing in either outcomes or steps for a single run.
func collectIssueNumbers(outcomes map[int]IssueOutcomeRecord, steps map[int]map[string]StepResultRecord) []int {
	seen := make(map[int]bool)
	for num := range outcomes {
		seen[num] = true
	}
	for num := range steps {
		seen[num] = true
	}

	nums := make([]int, 0, len(seen))
	for num := range seen {
		nums = append(nums, num)
	}
	sort.Ints(nums)
	return nums
}

