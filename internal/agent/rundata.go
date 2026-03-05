package agent

import "github.com/phs/dark-factory/internal/rundata"

// ResultToStep converts an agent.Result to a rundata.StepResult, computing
// DurationSeconds from the timing fields captured during execution.
func ResultToStep(r *Result) rundata.StepResult {
	step := rundata.StepResult{
		Output:     r.ResultText,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
	}
	if !r.StartedAt.IsZero() && !r.FinishedAt.IsZero() {
		step.DurationSeconds = r.FinishedAt.Sub(r.StartedAt).Seconds()
	}
	return step
}
