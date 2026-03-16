# Scenario: Add --watch flag to godark run

Relates to: Issue #519

## Setup
- `internal/cmd/run.go` with `--watch` flag registered
- `internal/watch/` package available for polling
- Stubbed orchestrator and GitHub calls

## Cases

### Watch mode entered after unmerged PRs
Run `godark run --tag phase-X --watch` where the run completes with 1 issue in `needs-human-review`.
- After first pass completes, the command enters the watch polling loop
- Does not exit immediately

### Watch mode skipped when all implemented
Run `godark run --tag phase-X --watch` where all issues are implemented and merged.
- Run completes and exits normally
- No watch polling loop entered

### Watch exits when no more PRs awaiting
During watch mode, the last `awaiting-human-review` PR is merged.
- Watch loop detects no more labeled PRs
- Command exits cleanly

### Cancel during watch mode
Press ctrl+c during the watch polling loop.
- Context is cancelled
- Watch loop exits
- Command returns without error (context cancellation is expected)

### Flag not set exits as before
Run `godark run --tag phase-X` without `--watch`.
- Run completes and exits immediately (no watch loop)
- Behavior identical to pre-Phase 23

### Watch handles CHANGES_REQUESTED during polling
During watch mode, a human submits CHANGES_REQUESTED on an awaiting PR.
- Watch detects the review
- Implementer agent is invoked with the feedback
- PR is re-labeled for human review after fix

### Watch handles APPROVED during polling
During watch mode, a human approves an awaiting PR.
- Watch detects the approval
- PR is merged
