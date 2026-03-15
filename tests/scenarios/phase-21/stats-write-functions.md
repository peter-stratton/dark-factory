# Scenario: Stats write functions

Relates to: Issue #459

## Setup
- `internal/stats/` package with `Open()`, schema migrations, and write functions
- An in-memory SQLite database opened via `stats.Open(":memory:")`
- `RunRecord`, `IssueOutcomeRecord`, and `StepResultRecord` types defined

## Cases

### Write and read back a run record
Call `WriteRun()` with a `RunRecord` containing repo, milestone, timestamps, and summary counts. Query the `runs` table.
- One row exists with matching repo, milestone, total, implemented, failed values
- Timestamps are stored and retrieved correctly

### Write and read back an issue outcome
Call `WriteIssueOutcome()` with run_id, issue_number 42, title, status "implemented", pr_number 87.
- One row exists in `issue_outcomes` with matching fields

### Write and read back a step result with flags
Call `WriteStepResult()` with step_name "quality-review", cost_usd 0.15, duration_seconds 45.2, flags `["low_cost", "no_diff_read"]`.
- One row exists in `step_results` with matching fields
- Flags are stored as JSON text `["low_cost","no_diff_read"]`
- Reading the flags back produces the original `[]string{"low_cost", "no_diff_read"}`

### Idempotent run write
Write a `RunRecord` with id "20260314-142305" and implemented=3. Write again with implemented=5.
- Only one row exists in `runs` with id "20260314-142305"
- The `implemented` value is 5 (updated, not duplicated)

### Idempotent outcome write
Write an `IssueOutcomeRecord` with run_id "run-1", issue_number 42, status "failed". Write again with status "implemented".
- Only one row exists for (run-1, 42)
- Status is "implemented"

### Multiple issues per run
Write 3 `IssueOutcomeRecord` entries with the same run_id but different issue_numbers.
- All 3 rows exist in `issue_outcomes`
- Each has the correct issue_number

### Multiple steps per issue
Write step results for "implement", "quality-review", and "retry-1" for the same run_id and issue_number.
- All 3 rows exist in `step_results`
- Each has the correct step_name

### Empty flags stored as empty array
Write a `StepResultRecord` with nil/empty flags.
- Flags field is stored as `[]` (empty JSON array)
- Reading back produces an empty `[]string`
