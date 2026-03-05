# Scenario: Run data reader

Relates to: Issue #98

## Setup
- The reader functions live in `internal/rundata/reader.go`
- Tests create a temporary `~/.godark/runs/` directory structure with
  pre-written JSON files to simulate completed runs
- No external services required

## Cases

### ListRuns returns all runs sorted
Create 3 run directories with different timestamps under `owner/repo/`.
- `ListRuns()` returns 3 entries
- Entries are sorted most-recent-first by `started_at`

### ListRuns with empty state
Call `ListRuns()` with no runs directory.
- Returns an empty slice
- No error is returned

### ListRuns across multiple repos
Create run directories under `owner/repo-a/` and `owner/repo-b/`.
- `ListRuns()` returns runs from both repos
- All runs are interleaved by timestamp

### LoadRun returns full detail
Create a run directory with `run.json`, and an issue subdirectory containing
`outcome.json`, `implement.json`, `quality-review.json`, and `functional-review.json`.
- `LoadRun()` returns a `RunDetail` with the run metadata
- `RunDetail.Issues` contains one `IssueDetail`
- The `IssueDetail` has populated outcome, implement, and review fields

### LoadRun with retries
Create an issue subdirectory with `retries/1/retry.json` and `retries/1/quality-review.json`.
- The `IssueDetail` includes retry data for attempt 1

### Missing outcome file handled gracefully
Create an issue subdirectory with `implement.json` but no `outcome.json`.
- The `IssueDetail` is still returned
- The outcome fields are zero-valued

### Corrupt JSON skipped with warning
Create a `run.json` with invalid JSON content alongside valid run directories.
- The corrupt run is skipped
- Other valid runs are still returned
- A warning is logged
