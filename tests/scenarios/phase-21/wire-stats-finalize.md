# Scenario: Wire stats writes into FinalizeRun callers

Relates to: Issue #461

## Setup
- `internal/stats/` package with write functions
- Stats DB path defaults to `~/.godark/stats.db`
- Stub `processIssueFn` and agent runners for testing
- Run data writer creates run directories under a temp dir

## Cases

### Orchestrator writes run stats after FinalizeRun
Run `orchestrator.Run()` with 2 stubbed issues that return StatusImplemented.
- `~/.godark/stats.db` exists after the run
- The `runs` table contains 1 row with the correct repo, milestone, total=2, implemented=2
- The `issue_outcomes` table contains 2 rows with status "implemented"

### Orchestrator writes step results
Run `orchestrator.Run()` with a stubbed issue that has implement and quality-review steps.
- The `step_results` table contains rows for each step with cost and duration

### Implement command writes stats
Run `implementIssues()` with 1 stubbed issue.
- The `runs` table contains 1 row
- The `issue_outcomes` table contains 1 row

### Watch command writes stats
Simulate a watch-initiated fix cycle via `handleChangesRequested()`.
- The `runs` table contains 1 row with total=1
- The `issue_outcomes` table contains 1 row

### Stats write failure is non-blocking
Set the stats DB path to an unwritable location (e.g., `/nonexistent/dir/stats.db`).
- The run completes normally
- A warning is logged about the stats write failure
- `run.json` is still written correctly

### Database created on first run
Delete any existing `~/.godark/stats.db`. Run the orchestrator.
- The database file is created
- Schema migrations run automatically
- Run data is persisted

### Idempotent on re-run
Process the same issue twice (two separate runs).
- The `runs` table contains 2 rows (different run IDs)
- No duplicate key errors
