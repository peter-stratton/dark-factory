# Scenario: MergeCoordinate() agent function

Relates to: Issue #607

## Setup
- `internal/agent/merge_coordinator.go` exists with `MergeCoordinate()` function
- `prompts/merge_coordinator.txt` loaded via `LoadPrompts()`
- Standard test config with `MaxRebaseAttempts: 1`

## Cases

### Function follows agent pattern
Read `internal/agent/merge_coordinator.go`.
- Function signature accepts `ctx`, `issue`, `prNum`, `conflictInfo`, `cfg`,
  `prompts`, `authEnv`, `logger`
- Function returns `(*Result, error)`
- Function calls `newRunOpts` with role `"merge_coordinator"`
- Function calls `Run()` with the constructed options

### ConflictInfo injected into prompt data
Call `MergeCoordinate()` with `conflictInfo: "Auto-merge failed due to conflicts"`.
- The rendered prompt passed to `Run()` contains
  "Auto-merge failed due to conflicts"

### Empty conflict info accepted
Call `MergeCoordinate()` with an empty `conflictInfo` string.
- No error is returned from prompt rendering
- The function proceeds to call `Run()`
