# Scenario: TUI watch-mode transition

Relates to: Issue #523

## Setup
- `internal/tui/model.go` with `watching` state and `WatchingMsg` message type
- `godark run --watch` command wiring sends `WatchingMsg` when entering watch mode
- TUI already showing completed run results (issue table with outcomes)

## Cases

### Transition to watching state
Send `WatchingMsg` after `RunFinishedMsg`.
- `watching` is set to true
- Hint text changes to "watching for merges · press ctrl+c to cancel"
- Spinner continues animating (indicating active polling)

### Issue table preserved during watch
Send `WatchingMsg` after a run with 5 completed issues.
- All 5 issue rows remain visible with their final statuses
- Table is not cleared

### New issue added during watch mode
During watch mode, re-resolution processes issue B. Send `IssueStartedMsg` for B.
- New row appears in the issue table for B
- B shows spinner and "starting" stage

### New issue completes during watch mode
Send `IssueCompletedMsg` for B with status "implemented".
- B's row updates to show green marker and MERGED badge
- Merged count in summary bar increments

### Cancel during watch mode
Press ctrl+c while `watching == true`.
- `cancelling` set to true
- Hint changes to "cancelling... waiting for current issue to finish"
- cancelFn is called

### Watch mode completes
All PRs merged, re-resolution finds nothing new. Send `RunDoneMsg`.
- `done` set to true, `watching` set to false
- Hint changes to "press q to exit"
- User can exit with q/esc/ctrl+c

### Summary bar updates during watch
Issues processed during watch mode update the summary bar counts.
- Merged count reflects both initial run and watch-mode completions
- Queued count reflects newly discovered issues from re-resolution
