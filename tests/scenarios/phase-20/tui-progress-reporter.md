# Scenario: TUI progress reporter implementation

Relates to: Issue #444

## Setup
- `internal/tui/reporter.go` defines `TUIReporter` struct
- `TUIReporter` holds a `*tea.Program` reference
- `TUIReporter` implements `progress.ProgressReporter`
- A test helper captures messages sent via `p.Send()`

## Cases

### TUIReporter satisfies ProgressReporter interface
Assign a `*TUIReporter` to a variable of type `progress.ProgressReporter`.
- The assignment compiles without error

### IssueStarted sends IssueStartedMsg
Call `IssueStarted(42, "add endpoint")`.
- `p.Send()` is called with `IssueStartedMsg{Number: 42, Title: "add endpoint"}`

### IssueStageChanged sends IssueStageChangedMsg
Call `IssueStageChanged(42, "verify")`.
- `p.Send()` is called with `IssueStageChangedMsg{Number: 42, Stage: "verify"}`

### IssueCompleted sends IssueCompletedMsg
Call `IssueCompleted(42, "add endpoint", "implemented", 87, 0, "")`.
- `p.Send()` is called with `IssueCompletedMsg{Number: 42, Title: "add endpoint", Status: "implemented", PRNumber: 87, Retries: 0, ErrMsg: ""}`

### IssueCompleted sends failed status with error
Call `IssueCompleted(42, "add endpoint", "failed", 0, 0, "sandbox timeout")`.
- `p.Send()` is called with `IssueCompletedMsg` where `ErrMsg` is `"sandbox timeout"`

### WaveStarted sends WaveStartedMsg
Call `WaveStarted(2, 3)`.
- `p.Send()` is called with `WaveStartedMsg{Wave: 2, Count: 3}`

### RunStarted sends RunStartedMsg
Call `RunStarted("phs/dark-factory", "Phase 20", "20260314-142305", "phase-20", "low_risk", "manual", 5)`.
- `p.Send()` is called with a `RunStartedMsg` containing all metadata fields

### RunFinished sends RunFinishedMsg
Call `RunFinished(3, 1, 0, 1, 2)`.
- `p.Send()` is called with `RunFinishedMsg{Implemented: 3, ReadyToMerge: 1, NeedsHumanReview: 0, Failed: 1, Blocked: 2}`

### RollupCreated sends RollupCreatedMsg
Call `RollupCreated(99, "https://github.com/peter-stratton/dark-factory/pull/99", true)`.
- `p.Send()` is called with `RollupCreatedMsg{PRNumber: 99, PRURL: "...", Merged: true}`

### AllBlocked sends RunFinishedMsg with blocked counts
Call `AllBlocked(5, 5)`.
- `p.Send()` is called with a `RunFinishedMsg` where `Blocked` is 5

### PunchlistText is a no-op
Call `PunchlistText("some punchlist text")`.
- `p.Send()` is NOT called
- No panic or error occurs

### TUI package has no forbidden imports
Inspect import statements in `internal/tui/reporter.go`.
- No imports from `internal/orchestrator/`, `internal/service/`, or `internal/infrastructure/`
- Imports `internal/progress/` (foundation layer — allowed)
