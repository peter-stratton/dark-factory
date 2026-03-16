# Scenario: Re-resolve dependencies and process unblocked issues

Relates to: Issue #522

## Setup
- `internal/orchestrator/orchestrator.go` with extracted `reResolveAndProcess()` function
- Watch polling loop detects merged PRs and triggers re-resolution
- Test issues with dependency chains: A blocks B, C blocks D
- Stubbed `processIssueFn` and GitHub calls

## Cases

### Re-resolve after single merge
Issue A was `needs-human-review`, blocking issue B. A's PR is merged externally.
- Re-resolution detects B is now unblocked
- B is processed through the full agent loop

### Multiple issues unblocked
Issues A and C both merged. B (blocked by A) and D (blocked by C) are both blocked.
- Re-resolution finds both B and D unblocked
- Both are processed

### No new issues unblocked
A merge occurs but no blocked issues depend on it.
- Re-resolution finds no new processable issues
- Polling continues without processing

### Stats written for daemon-mode issues
Issue B is processed during daemon mode re-resolution.
- Stats DB contains a run record and outcome for B
- Step results written with cost and duration

### Existing wave loop unchanged
Run `processIssues()` without `--watch` (normal run mode).
- Behavior is identical to before the refactor
- Wave re-resolution still works for internal merges

### Reporter receives messages for new issues
Issue B is processed during daemon mode.
- TUI receives `IssueStartedMsg` and `IssueCompletedMsg` for B
- Issue appears in the TUI table

### Exit when all complete
All issues implemented and no PRs awaiting review.
- Daemon mode exits the polling loop
- Run completes cleanly
