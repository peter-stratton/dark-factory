# Scenario: Watch TUI view

Relates to: Issue #520

## Setup
- `internal/tui/watch_model.go` with `WatchModel` implementing `tea.Model`
- Message types: `PollTickMsg`, `PRUpdateMsg`, `ActivityMsg`, `WatchDoneMsg`
- `internal/cmd/watch.go` wires TUI when in interactive terminal mode

## Cases

### Header renders with repo name
Create a `WatchModel` with repo "org/my-repo".
- View output contains "godark watch" and "org/my-repo"

### PR table shows labeled PRs
Send `PRUpdateMsg` with PR #42 title "add endpoint", label "awaiting-human-review".
- PR table contains "#42" and "add endpoint"
- Status shows awaiting review

### PR table updates on label change
Send `PRUpdateMsg` for PR #42 with label "fixing-review-feedback".
- PR #42 row updates to show fixing feedback status

### Activity log shows recent events
Send 3 `ActivityMsg` entries: "CHANGES_REQUESTED on #42", "fix pushed for #42", "APPROVED on #42".
- Activity log shows all 3 entries in order

### Activity log trims at 10 entries
Send 12 `ActivityMsg` entries.
- Only the 10 most recent entries are displayed
- Oldest 2 are trimmed

### Done state allows exit
Send `WatchDoneMsg`.
- `done` flag is set
- Hint shows "press q to exit"
- Pressing q exits the TUI

### Ctrl+c exits cleanly
Press ctrl+c during active watch.
- cancelFn is called
- TUI shows "cancelling..."
- After WatchDoneMsg, user can exit with q

### Empty state
No PRUpdateMsg sent.
- PR table area shows "no PRs awaiting review"
