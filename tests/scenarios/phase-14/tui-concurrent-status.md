# Scenario: TUI concurrent status display

Relates to: Issue #753

## Setup
- A TUI `Model` initialized with issue rows
- Messages dispatched via Bubble Tea's `Update` loop

## Cases

### Multiple issues show concurrent spinners
- GIVEN two `IssueStartedMsg` sent without completing the first
- WHEN the TUI renders
- THEN both rows display in-progress spinners simultaneously

### Worker count displayed in summary bar
- GIVEN a `WorkersActiveMsg{Active: 3, Total: 5}` sent to the model
- WHEN the TUI renders the summary bar
- THEN the output contains "3/5 workers"

### Worker count hidden in serial mode
- GIVEN a `WorkersActiveMsg{Active: 1, Total: 1}` sent to the model
- WHEN the TUI renders the summary bar
- THEN no worker count is displayed

### Worker count updates on new message
- GIVEN a `WorkersActiveMsg{Active: 3, Total: 5}` followed by `WorkersActiveMsg{Active: 1, Total: 5}`
- WHEN the TUI renders after each message
- THEN the summary bar updates from "3/5 workers" to "1/5 workers"
