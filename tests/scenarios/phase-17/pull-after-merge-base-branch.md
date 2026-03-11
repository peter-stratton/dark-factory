# Scenario: PullAfterMerge uses configured base branch

Relates to: Issue #314

## Setup
- The `internal/orchestrator/` package with stubbed `CommandRunner`
- `PullAfterMerge` function accepting a branch parameter

## Cases

### Pull from custom branch
Call `PullAfterMerge("feature/foo", logger)`.
- `CommandRunner` is called with `"git", "pull", "--rebase", "origin", "feature/foo"`

### Pull from main
Call `PullAfterMerge("main", logger)`.
- `CommandRunner` is called with `"git", "pull", "--rebase", "origin", "main"`

### Dirty repo warning references configured branch
Call `PullAfterMerge("feature/foo", logger)` when the working tree is dirty.
- Warning message contains "feature/foo", not hardcoded "main"

### Caller passes main when config is empty
In `implement.go`, when `cfg.BaseBranch` is `""`, the caller passes `"main"` to `PullAfterMerge`.
- `PullAfterMerge` receives `"main"` as the branch argument
