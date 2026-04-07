# Scenario: extract per-issue processing into worker function

Relates to: Issue #749

## Setup
- The `processIssues` function in `internal/orchestrator/orchestrator.go`
- Stubbed `processIssueFn` for controlling issue outcomes

## Cases

### Successful issue returns merged result
- GIVEN a stub that returns `StatusImplemented` with PR number 42
- WHEN `runOneIssue` is called
- THEN the result has `Merged: true`, `IssueNumber` matching the input, and `Outcome.PRNumber == 42`

### Failed issue returns non-merged result
- GIVEN a stub that returns `StatusFailed` with an error
- WHEN `runOneIssue` is called
- THEN the result has `Merged: false` and `Outcome.Err` is non-nil

### No shared state mutation
- GIVEN the `runOneIssue` function signature
- WHEN inspecting its parameters
- THEN it takes only values and pointers to immutable or per-issue data (no runStats, seen, implementedIssues)

### Existing test suite passes
- GIVEN the existing orchestrator test suite
- WHEN `go test ./internal/orchestrator/...` is run
- THEN all tests pass without modification
