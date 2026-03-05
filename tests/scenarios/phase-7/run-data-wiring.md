# Scenario: Wire RunDataWriter into agent loop

Relates to: Issue #96

## Setup
- The `RunDataHook` interface is defined in `internal/agent/runhook.go`
- Tests use a mock implementation of `RunDataHook` to verify call sequences
- `ProcessIssue` is tested with a stubbed `Runner` (no real agent invocation)
- The orchestrator and implement commands are tested for writer creation and
  `FinalizeRun` calls
- No Docker, GitHub API, or real Claude invocations required

## Cases

### RunDataHook interface defined
Import the `agent` package and verify the `RunDataHook` interface exists.
- Interface has `WriteImplementResult(issueNumber int, step StepResult) error`
- Interface has `WriteReviewResult(issueNumber int, kind string, step StepResult) error`
- Interface has `WriteRetryResult(issueNumber int, attempt int, step StepResult) error`
- Interface has `WriteRetryReviewResult(issueNumber int, attempt int, step StepResult) error`
- Interface has `WriteOutcome(outcome Outcome) error`

### Hook called after implement step
Run `ProcessIssue` with a mock hook and a stubbed runner that returns success.
- `WriteImplementResult` is called with the correct issue number
- The `StepResult` contains non-empty fields

### Hook called after review step
Run `ProcessIssue` through to the review phase with a mock hook.
- `WriteReviewResult` is called with the correct issue number
- The `kind` parameter is either `"quality"` or `"functional"`

### Hook called with outcome
Run `ProcessIssue` to completion with a mock hook.
- `WriteOutcome` is called with the correct issue number and status

### Nil hook does not panic
Run `ProcessIssue` with `Hook` set to nil.
- No panic occurs
- The function completes normally

### FinalizeRun called by orchestrator
Run the orchestrator loop with a mock writer.
- `FinalizeRun` is called after all issues are processed
- The summary contains correct counts for implemented, failed, and skipped

### ResultToStep conversion
Call `ResultToStep` with a populated `agent.Result`.
- Returns a `StepResult` with matching `session_id`, `cost_usd`, `exit_code`
- `tool_trace` is copied from the result
- `verdict` is set if present
