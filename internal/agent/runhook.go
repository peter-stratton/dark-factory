package agent

import "github.com/phs/dark-factory/internal/rundata"

// RunDataHook is an optional sink for per-step run data. ProcessIssue calls
// these methods after each agent step. Callers should check for nil before
// passing; individual methods are not required to be nil-safe themselves.
type RunDataHook interface {
	WriteSpecGeneratorResult(issueNumber int, step rundata.StepResult) error
	WriteImplementResult(issueNumber int, step rundata.StepResult) error
	WriteReviewResult(issueNumber int, kind string, step rundata.StepResult) error
	WriteRetryResult(issueNumber int, attempt int, step rundata.StepResult) error
	WriteRetryReviewResult(issueNumber int, attempt int, step rundata.StepResult) error
	WriteRetryFunctionalReviewResult(issueNumber int, attempt int, step rundata.StepResult) error
	WriteVerifyResult(issueNumber int, step rundata.VerifyStepResult) error
	WriteOutcome(outcome rundata.Outcome) error
	WriteIssueStatus(issueNumber int, status rundata.IssueStatus) error
	WriteRiskAssessment(issueNumber int, assessment rundata.RiskAssessment) error
	WriteFailureAnalysis(issueNumber int, analysis rundata.FailureAnalysis) error
	WriteContainerLog(issueNumber int, log string) error
}
