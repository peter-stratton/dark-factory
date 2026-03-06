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
