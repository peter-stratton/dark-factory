package agent

import (
	"github.com/peter-stratton/dark-factory/internal/quality"
	"github.com/peter-stratton/dark-factory/internal/rundata"
)

// ResultToStep converts an agent.Result to a rundata.StepResult, computing
// DurationSeconds from the timing fields captured during execution.
func ResultToStep(r *Result) rundata.StepResult {
	step := rundata.StepResult{
		Output:          r.ResultText,
		TimedOut:        r.TimedOut,
		CostUSD:         r.CostUSD,
		ToolTrace:       r.ToolTrace,
		SessionID:       r.SessionID,
		PeakMemoryBytes: r.PeakMemoryBytes,
		CPUNanoseconds:  r.CPUNanoseconds,
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

// toRundataRiskAssessment converts a quality.RiskAssessment to its rundata counterpart
// for persistence. The quality and rundata packages are both in the domain layer and
// cannot import each other, so conversion happens here in the orchestration layer.
func toRundataRiskAssessment(a quality.RiskAssessment) rundata.RiskAssessment {
	gates := make([]rundata.RiskGate, len(a.Gates))
	for i, g := range a.Gates {
		gates[i] = rundata.RiskGate{
			Name:   g.Name,
			Passed: g.Passed,
			Detail: g.Detail,
		}
	}
	return rundata.RiskAssessment{
		IsLowRisk: a.IsLowRisk,
		Gates:     gates,
	}
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
