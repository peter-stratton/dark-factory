# Scenario: Stats query functions

Relates to: Issue #460

## Setup
- `internal/stats/` package with write and query functions
- An in-memory SQLite database populated with test data:
  - 3 runs across 2 repos ("org/repo-a" with 2 runs, "org/repo-b" with 1 run)
  - Each run has 2-3 issue outcomes and step results
  - Runs have different timestamps for date range testing
- `RunFilter` struct with optional Repo, Milestone, Since, Until fields

## Cases

### Query all runs with no filter
Call `QueryRuns()` with an empty `RunFilter`.
- Returns all 3 runs
- Results are sorted by `started_at` ascending

### Filter runs by repo
Call `QueryRuns()` with `RunFilter{Repo: "org/repo-a"}`.
- Returns exactly 2 runs (both repo-a)
- No repo-b runs included

### Filter runs by date range
Call `QueryRuns()` with `Since` set to a timestamp between run 1 and run 2.
- Returns only runs with `started_at` >= Since
- Earlier runs excluded

### Filter with Since and Until
Call `QueryRuns()` with both `Since` and `Until` set to bracket the middle run.
- Returns only the middle run

### Filter by milestone
Call `QueryRuns()` with `RunFilter{Milestone: "Phase 19"}`.
- Returns only runs with matching milestone

### Query outcomes joined to filtered runs
Call `QueryIssueOutcomes()` with `RunFilter{Repo: "org/repo-a"}`.
- Returns only outcomes belonging to repo-a runs
- No outcomes from repo-b runs

### Query step results joined to filtered runs
Call `QueryStepResults()` with `RunFilter{Repo: "org/repo-a"}`.
- Returns only step results belonging to repo-a runs

### Empty result returns empty slice
Call `QueryRuns()` with `RunFilter{Repo: "nonexistent/repo"}`.
- Returns an empty slice (not nil)
- No error

### Parameterized queries prevent injection
Call `QueryRuns()` with `RunFilter{Repo: "'; DROP TABLE runs; --"}`.
- No error
- Tables still exist and contain original data
