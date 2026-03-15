# Scenario: SQLite dependency and stats package scaffold

Relates to: Issue #458

## Setup
- `internal/stats/` package exists with `db.go` and `schema.go`
- `modernc.org/sqlite` is in `go.mod`
- `docs/architecture.json` domain layer includes `"internal/stats/"`

## Cases

### Open creates new database with all tables
Call `stats.Open(":memory:")`.
- No error returned
- Querying `sqlite_master` for table names returns `runs`, `issue_outcomes`, and `step_results`

### Runs table has expected columns
Insert a row into `runs` with all columns: id, repo, milestone, base_branch, auto_merge_feature, auto_merge_rollup, started_at, finished_at, total, implemented, failed, abort_reason.
- INSERT succeeds without error
- SELECT returns the inserted values

### Issue outcomes table has expected columns
Insert a row into `issue_outcomes` with all columns: run_id, issue_number, title, status, pr_number, error.
- INSERT succeeds without error

### Step results table has expected columns
Insert a row into `step_results` with all columns: run_id, issue_number, step_name, cost_usd, duration_seconds, flags, started_at, finished_at.
- INSERT succeeds without error

### Unique constraint on issue outcomes
Insert two rows into `issue_outcomes` with the same `(run_id, issue_number)`.
- The second INSERT fails with a constraint violation (or replaces, depending on mode)

### Unique constraint on step results
Insert two rows into `step_results` with the same `(run_id, issue_number, step_name)`.
- The second INSERT fails with a constraint violation (or replaces, depending on mode)

### Idempotent open on existing database
Call `Open()` on a database that already has tables and data. Then query the data.
- No error on open
- Existing data is preserved

### Close succeeds
Call `Open(":memory:")` then `Close()`.
- Close returns nil

### Architecture vet passes
Run `godark vet architecture` after adding `internal/stats/` to domain layer.
- No violations reported

### Build succeeds
Run `go build ./...` after adding `modernc.org/sqlite`.
- Exit code 0
