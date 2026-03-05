package agent

// RunDataHook is implemented by callers that want to record per-step results
// to a run data directory. Pass nil to ProcessIssue to disable recording.
type RunDataHook interface {
	WriteImplementResult(issueNumber int, result *Result) error
	WriteReviewResult(issueNumber int, kind string, retry int, result *Result) error
	WriteRetryResult(issueNumber int, n int, result *Result) error
}
