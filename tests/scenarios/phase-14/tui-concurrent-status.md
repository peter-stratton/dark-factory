# Scenario: TUI concurrent status display

Relates to: Issue #601

## Setup
- `internal/tui/` model, messages, and view components
- Bubble Tea test program for sending messages

## Cases

### Multiple in-progress spinners
Send `IssueStartedMsg` for issues #1 and #2 without sending `IssueCompletedMsg` for either.
- Both rows display in-progress spinner state
- Both issues visible in the table simultaneously

### Worker count in summary bar
Send `WorkersActiveMsg{Active: 3, Total: 5}`.
- Summary bar displays "3/5 workers active" (or equivalent)

### Wave transition resets worker count
Complete a wave (send `IssueCompletedMsg` for all issues), then send `WorkersActiveMsg` for the new wave.
- Worker count reflects the new wave's active count
- Completed issues from prior wave show final status

### Serial mode display
Run with `max_workers: 1`.
- No worker count indicator displayed (or shows "1/1")
- Display is identical to pre-concurrency TUI behavior
