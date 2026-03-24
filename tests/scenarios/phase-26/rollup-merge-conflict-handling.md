# Scenario: Rollup merge conflict handling

Relates to: Issue #610

## Setup
- `handleRollupPR()` in `internal/orchestrator/orchestrator.go` is the function
  under test
- `upsertRollupPRFn`, `mergeRollupPRFn`, `runRollupVerifyFn`, and
  `github.CheckMergeable` are stubbable
- `agent.MergeCoordinate` is stubbable
- `cfg.AutoMerge.Rollup` set to `"auto"` for merge tests
- `cfg.MaxRebaseAttempts` set to 2

## Cases

### Clean rollup merges without merge coordinator
Stub `upsertRollupPRFn` to return PR #100.
Stub `CheckMergeable` to return `"MERGEABLE"`.
Stub `runRollupVerifyFn` to pass.
- `MergeCoordinate` is not called
- `mergeRollupPRFn` is called with PR #100
- Function returns the PR number and URL without error

### Conflicting rollup invokes merge coordinator
Stub `upsertRollupPRFn` to return PR #100.
Stub `CheckMergeable` to return `"CONFLICTING"` then `"MERGEABLE"`.
Stub `MergeCoordinate` to succeed.
Stub `runRollupVerifyFn` to pass.
- `MergeCoordinate` is called once
- `runRollupVerifyFn` is called after conflict resolution
- `mergeRollupPRFn` is called and succeeds

### Unresolvable conflict leaves PR open
Stub `CheckMergeable` to always return `"CONFLICTING"`.
Stub `MergeCoordinate` to succeed (but conflicts persist).
Set `MaxRebaseAttempts: 2`.
- `MergeCoordinate` is called exactly 2 times
- `mergeRollupPRFn` is not called
- `reporter.RollupCreated` is called with `merged: false`
- No error is returned

### Rollup verify re-runs after conflict resolution
Stub a conflict-then-clean flow.
- `runRollupVerifyFn` is called after the merge coordinator resolves the conflict
- If verify fails, the PR is left open for human review
