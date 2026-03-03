# Scenario: Dry-run execution plan output

Relates to: Issue #6

## Setup
- The orchestrator package (`internal/orchestrator`) with its `Run` function
- A valid `Config` struct with `Repo` and `Milestone` set
- The `github.CommandRunner` variable stubbed to return controlled JSON responses (no real GitHub API calls)
- A `slog.Logger` instance (can discard output or write to a buffer)
- No external services or network access required

## Cases

### Dry-run with processable and blocked issues
Stub `CommandRunner` to return:
- Open issues: #1 (no deps), #2 (blocked by #99), #3 (no deps)
- Closed issues: none
Call `Run(cfg, logger, true)`.
- Output contains `Execution Plan (dry-run)`
- Output lists #1 and #3 under processable issues
- Output lists #2 under blocked issues with `blocked by: #99`
- Output contains a summary line with counts: `3 total, 1 blocked, 2 processable`

### Dry-run with all issues blocked
Stub `CommandRunner` to return:
- Open issues: #5 (blocked by #1), #6 (blocked by #2)
- Closed issues: none
Call `Run(cfg, logger, true)`.
- No processable issues section appears (or it is empty)
- Both issues appear under blocked issues
- Summary shows `2 total, 2 blocked, 0 processable`

### Dry-run with no issues in milestone
Stub `CommandRunner` to return an empty issue list.
Call `Run(cfg, logger, true)`.
- Output contains `No issues found`
- No error is returned

### Dry-run shows priority labels
Stub `CommandRunner` to return:
- Issue #1 with label `p1`, #2 with label `p2`, #3 with no priority label
- No dependencies, all processable
Call `Run(cfg, logger, true)`.
- Issues are listed in priority order: #1 (p1), #2 (p2), #3 (none)
- Each issue line shows its priority

### Non-dry-run logs placeholder messages
Stub `CommandRunner` to return processable issues.
Call `Run(cfg, logger, false)`.
- Output contains processing messages for each issue
- Output includes `not implemented yet` (since agent execution is Phase 2)
- Summary line is still printed
