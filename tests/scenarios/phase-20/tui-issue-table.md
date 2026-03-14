# Scenario: TUI issue table component

Relates to: Issue #443

## Setup
- `internal/tui/table.go` defines `issueRow` struct and `renderTable()`
- `internal/tui/messages.go` defines custom Bubble Tea message types
- `Model` in `model.go` maintains `issues []issueRow` and `issueIndex map[int]int`
- Model handles `IssueStartedMsg`, `IssueStageChangedMsg`, `IssueCompletedMsg` in `Update()`

## Cases

### Queued issue renders circle marker
Render a row with status `""` (no stage yet).
- Output contains the `○` character
- The marker is styled with muted/dim color

### In-progress issue renders spinner
Render a row with stage `"implement"`.
- Output contains the spinner character (animated in live TUI)
- Output contains `"implement"` as the stage label

### Completed issue renders filled square in green
Render a row with status `"implemented"`.
- Output contains the `■` character
- The marker is styled green

### In-review issue renders filled circle in yellow
Render a row with status `"ready-to-merge"`.
- Output contains the `●` character
- The marker is styled yellow

### Failed issue renders cross in red
Render a row with status `"failed"` and errMsg `"sandbox timeout"`.
- Output contains the `✕` character
- The marker is styled red
- Output contains `"sandbox timeout"`

### IssueStartedMsg adds a new row
Send `IssueStartedMsg{Number: 42, Title: "add endpoint"}` to `Update()`.
- `Model.issues` has one entry with number 42
- `Model.issueIndex[42]` equals 0

### IssueStartedMsg for multiple issues preserves order
Send `IssueStartedMsg` for issues 42, 43, 44 in sequence.
- `Model.issues` has 3 entries in order: 42, 43, 44

### IssueStageChangedMsg updates correct row
Add issue 42, then send `IssueStageChangedMsg{Number: 42, Stage: "verify"}`.
- Issue 42's stage is `"verify"`
- Other issues (if any) are unchanged

### IssueCompletedMsg updates status and stage
Add issue 42, then send `IssueCompletedMsg{Number: 42, Status: "implemented", PRNumber: 87}`.
- Issue 42's status is `"implemented"`
- Issue 42's prNumber is 87

### WaveStartedMsg is handled without error
Send `WaveStartedMsg{Wave: 2, Count: 3}` to `Update()`.
- No panic or error
- Model continues to render normally

### RunFinishedMsg updates summary counts
Send `RunFinishedMsg{Implemented: 3, ReadyToMerge: 1, NeedsHumanReview: 0, Failed: 1, Blocked: 2}` to `Update()`.
- Summary bar reflects the new counts

### Long title truncated to terminal width
Set terminal width to 80 columns. Add an issue with a 120-character title.
- The rendered row does not exceed 80 columns
- The title ends with `…`

### Stage labels match expected values
Process an issue through all stages by sending sequential `IssueStageChangedMsg` messages.
- Stages appear in order: `recon`, `implement`, `verify`, `review`
- Final `IssueCompletedMsg` with status `"implemented"` shows `merged` stage
