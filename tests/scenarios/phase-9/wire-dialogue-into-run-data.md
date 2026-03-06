# Scenario: Wire agent dialogue into run data

Relates to: Issue #150

## Setup
- `internal/rundata/` package provides `Writer` and reader functions
- Test fixtures: a temporary run directory at
  `<tmpdir>/runs/owner/repo/timestamp/`
- No external services required for rundata tests
- Orchestrator integration requires stubbed GitHub client for PR comment
  fetching

## Cases

### WriteDialogue creates valid JSON
Call `Writer.WriteDialogue(42, entries)` with two dialogue entries.
- File `issues/42/dialogue.json` is created in the run directory
- File content is valid JSON containing both entries
- Each entry has `role`, `round`, and `body` fields

### ReadIssueDetail includes dialogue
Write a `dialogue.json` file to an issue directory, then call
`ReadIssueDetail`.
- `IssueDetail.Dialogue` contains the entries from the file
- Entry fields match what was written

### Missing dialogue file returns nil
Call `ReadIssueDetail` for an issue directory that has no `dialogue.json`.
- `IssueDetail.Dialogue` is nil
- No error is returned

### Round trip preserves data
Write entries with `WriteDialogue`, then read with `ReadIssueDetail`.
- Returned entries are identical to what was written (role, round, body all
  match)

### Multiple retry rounds
Write four entries representing two retry rounds (implementer round 1,
reviewer round 1, implementer round 2, reviewer round 2).
- All four entries are stored and returned in order
- Round numbers are 1, 1, 2, 2 respectively
