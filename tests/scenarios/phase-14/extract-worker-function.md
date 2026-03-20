# Scenario: extract per-issue processing into worker function

Relates to: Issue #595

## Setup
- `internal/orchestrator/orchestrator.go` with extracted worker function
- Stubbed `ProcessIssue` returning controlled outcomes
- `waveResult` struct definition

## Cases

### Successful issue returns implemented result
Stub `ProcessIssue` to return `StatusImplemented` with PR #42.
- `waveResult.Status` equals `StatusImplemented`
- `waveResult.PRNumber` equals `42`
- `waveResult.Err` is nil

### Failed issue returns error result
Stub `ProcessIssue` to return `StatusFailed` with an error.
- `waveResult.Status` equals `StatusFailed`
- `waveResult.Err` is non-nil

### Function does not mutate shared state
Inspect the worker function signature.
- No package-level variables referenced inside the function body
- Parameters are all value types or immutable references (issue, config, prompts, auth env, logger, hook, reporter)
- Return type is `waveResult` struct (no pointer to shared state)

### Existing test suite passes unchanged
Run `go test ./internal/orchestrator/...` after the refactor.
- All existing tests pass without modification
- No new test failures introduced
