# Scenario: Per-issue pre-merge integration

Relates to: Issue #608

## Setup
- `runPreMergeRebasePhase()` in `internal/agent/loop.go` is the function under test
- `github.CheckMergeable()`, `github.UpdateBranch()`, and `MergeCoordinate()`
  are stubbable
- `cfg.MaxRebaseAttempts` set to 2 for multi-attempt tests
- `runVerifyPhase()` is stubbable

## Cases

### No conflict proceeds to merge
Stub `CheckMergeable` to return `"MERGEABLE"`.
- Function returns `(false, nil)` immediately
- `MergeCoordinate` is not called
- `UpdateBranch` is not called

### Automatic rebase succeeds without merge coordinator
Stub `CheckMergeable` to return `"CONFLICTING"` on first call, then `"MERGEABLE"`.
Stub `UpdateBranch` to succeed.
- `MergeCoordinate` is not called
- `runVerifyPhase` is called once (after the successful rebase)
- Function returns `(false, nil)`

### Merge coordinator invoked when automatic rebase fails
Stub `CheckMergeable` to return `"CONFLICTING"` then `"MERGEABLE"`.
Stub `UpdateBranch` to fail with an error.
Stub `MergeCoordinate` to succeed.
- `MergeCoordinate` is called with the error message as `conflictInfo`
- `runVerifyPhase` is called after merge coordinator succeeds
- Function returns `(false, nil)`

### Max rebase attempts exhausted labels for human review
Stub `CheckMergeable` to always return `"CONFLICTING"`.
Stub `UpdateBranch` to always fail.
Stub `MergeCoordinate` to succeed (but conflicts persist).
Set `MaxRebaseAttempts: 2`.
- `MergeCoordinate` is called exactly 2 times
- Function returns `(true, nil)` signaling needs-human-review

### Merge coordinator error counts as failed attempt
Stub `CheckMergeable` to return `"CONFLICTING"`.
Stub `UpdateBranch` to fail.
Stub `MergeCoordinate` to return an error.
Set `MaxRebaseAttempts: 2`.
- The failed attempt is counted
- The loop continues to the next attempt
- After 2 failed attempts, function returns `(true, nil)`

### Session ID is not updated by merge coordinator
Stub a conflict resolution flow where `MergeCoordinate` succeeds.
- The `sessionID` pointer value is unchanged after the merge coordinator runs
- The merge coordinator runs as an independent agent, not a session continuation
