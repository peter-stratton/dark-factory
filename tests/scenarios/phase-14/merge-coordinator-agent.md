# Scenario: merge coordinator agent function and wiring

Relates to: Issue #600

## Setup
- `internal/agent/merge_coordinator.go` with `MergeCoordinate()` function
- `runPreMergeRebasePhase()` in `internal/agent/loop.go` modified to use merge coordinator
- Stubbed agent runner and GitHub API calls

## Cases

### Successful conflict resolution
Stub automatic rebase (`gh pr update-branch`) to fail and merge coordinator agent to succeed.
- Merge coordinator agent is invoked with conflict context
- Verify pipeline re-runs after resolution
- PR proceeds to merge

### Failed resolution after max attempts
Set `max_rebase_attempts: 2`. Stub both automatic rebase and merge coordinator to fail.
- Merge coordinator is called twice
- PR is labeled `needs-human-review` after exhausting attempts

### Max attempts respected
Set `max_rebase_attempts: 1`. Stub automatic rebase to fail and merge coordinator to fail.
- Merge coordinator is called exactly once
- PR is labeled `needs-human-review`

### No conflict skips merge coordinator
Stub automatic rebase (`gh pr update-branch`) to succeed.
- Merge coordinator is not invoked
- PR proceeds directly to merge

### Run data records merge coordinator result
Stub merge coordinator to succeed.
- Step result written to run data includes duration and cost
- Session ID is recorded
