package agent

import "github.com/phs/dark-factory/internal/rundata"

// ResultToStep converts an agent.Result to a rundata.StepResult, computing
// DurationSeconds from the timing fields captured during execution.
func ResultToStep(r *Result) rundata.StepResult {
	step := rundata.StepResult{
		Output:    r.ResultText,
		CostUSD:   r.CostUSD,
		ToolTrace: r.ToolTrace,
	}
	if !r.StartedAt.IsZero() {
		t := r.StartedAt
		step.StartedAt = &t
	}
	if !r.FinishedAt.IsZero() {
		t := r.FinishedAt
		step.FinishedAt = &t
	}
	if step.StartedAt != nil && step.FinishedAt != nil {
		step.DurationSeconds = r.FinishedAt.Sub(r.StartedAt).Seconds()
	}
	return step
}

// verifyToRundata converts a VerifyResult to a rundata.VerifyStepResult.
// attempt is 0-indexed; fixAttempted indicates whether a fix was run before this check.
func verifyToRundata(vr VerifyResult, attempt int, fixAttempted bool) rundata.VerifyStepResult {
	checks := make([]rundata.CheckResult, len(vr.Checks))
	for i, cr := range vr.Checks {
		checks[i] = rundata.CheckResult{
			Name:     cr.Name,
			Passed:   cr.Passed,
			Output:   cr.Output,
			ExitCode: cr.ExitCode,
		}
	}
	return rundata.VerifyStepResult{
		Attempt:      attempt,
		Checks:       checks,
		AllPassed:    vr.AllPassed,
		FixAttempted: fixAttempted,
	}
}
