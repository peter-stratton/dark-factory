# Scenario: Switch analyze command to read from SQLite

Relates to: Issue #462

## Setup
- `internal/stats/` package with query functions and `convert.go`
- `~/.godark/stats.db` populated with test run data
- `internal/analysis/` functions (Aggregate, DetectGaps, ComputeTrends) unchanged
- `godark analyze` command with `--legacy` flag

## Cases

### Default reads from SQLite
Run `godark analyze` with a populated `stats.db` and no `--legacy` flag.
- Output contains outcome distribution, flag frequencies, retry stats, and cost stats
- Data matches what was written to the database

### Repo filter works against database
Run `godark analyze --repo org/repo-a` with runs from multiple repos in the database.
- Output only includes data from org/repo-a

### Milestone filter works against database
Run `godark analyze --milestone "Phase 19"`.
- Output only includes data from Phase 19 runs

### Date range filter works
Run `godark analyze --since 2026-03-10 --until 2026-03-12`.
- Output only includes runs within the date range

### JSON output unchanged
Run `godark analyze --json` with SQLite data.
- Output is valid JSON
- Structure matches the pre-Phase 21 JSON format

### Legacy flag falls back to filesystem
Run `godark analyze --legacy` with run data in `~/.godark/runs/`.
- Output is produced from filesystem scan, not SQLite
- Behavior matches pre-Phase 21 analyze

### Empty database shows no runs
Run `godark analyze` with an empty (newly created) `stats.db`.
- Output shows "No runs found" or equivalent
- No error or stack trace

### Conversion produces identical results
Populate both `stats.db` and `~/.godark/runs/` with the same data. Run `godark analyze` and `godark analyze --legacy`.
- Both produce identical output

### ToRunDetails groups by run
Call `ToRunDetails()` with 2 runs, 4 outcomes, and 6 step results.
- Returns 2 `RunDetail` structs
- Each contains the correct issues and step data
