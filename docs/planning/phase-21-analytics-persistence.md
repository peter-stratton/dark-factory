# Phase 21: Analytics Persistence

> **Goal:** Run statistics are persisted to a SQLite database
> (`~/.godark/stats.db`) at run finalization, surviving run directory deletion.
> `godark analyze` and the dashboard read from the database instead of scanning
> run directories, enabling improved metrics: retry recovery rate, cost
> breakdown by step, duration trends, and success rate by repo.

## Milestone

`Phase 21`

---

## Issue 458: SQLite dependency and stats package scaffold

### Description

Add the `modernc.org/sqlite` pure-Go SQLite driver to `go.mod` and create the
`internal/stats/` package with schema definition, migration framework, and
database open/close lifecycle. The schema has three tables: `runs` (one row per
run), `issue_outcomes` (one row per issue), and `step_results` (one row per
agent step like implement, quality-review, retry-1, etc.).

Update `docs/architecture.json` to add `internal/stats/` to the domain layer
paths.

### Key constraints

- Add `modernc.org/sqlite` to `go.mod` — pure Go, no CGO
- New file `internal/stats/db.go`:
  - `Open(path string) (*DB, error)` — opens or creates the SQLite database;
    runs migrations on first open
  - `Close() error` — closes the database connection
  - `DB` struct wrapping `*sql.DB`
- New file `internal/stats/schema.go`:
  - `migrate(db *sql.DB) error` — creates tables if they don't exist
  - Table `runs`: `id` (TEXT, run directory basename as PK), `repo` (TEXT),
    `milestone` (TEXT), `base_branch` (TEXT), `auto_merge_feature` (TEXT),
    `auto_merge_rollup` (TEXT), `started_at` (TIMESTAMP), `finished_at`
    (TIMESTAMP), `total` (INT), `implemented` (INT), `failed` (INT),
    `abort_reason` (TEXT)
  - Table `issue_outcomes`: `run_id` (TEXT FK), `issue_number` (INT),
    `title` (TEXT), `status` (TEXT), `pr_number` (INT), `error` (TEXT),
    UNIQUE on `(run_id, issue_number)`
  - Table `step_results`: `run_id` (TEXT FK), `issue_number` (INT),
    `step_name` (TEXT, e.g. "implement", "quality-review", "retry-1"),
    `cost_usd` (REAL), `duration_seconds` (REAL), `flags` (TEXT, JSON array),
    `started_at` (TIMESTAMP), `finished_at` (TIMESTAMP),
    UNIQUE on `(run_id, issue_number, step_name)`
- New file `internal/stats/schema_test.go`:
  - Tests use `:memory:` SQLite databases (no disk I/O)
- Default database path: `~/.godark/stats.db`
- Update `docs/architecture.json` domain layer paths to include
  `"internal/stats/"`

### Acceptance criteria

- [ ] `go.mod` includes `modernc.org/sqlite`
- [ ] `go build ./...` succeeds
- [ ] `Open()` creates the database file and all three tables
- [ ] `Open()` on an existing database is idempotent (no errors, no data loss)
- [ ] `Close()` closes the database without error
- [ ] `godark vet architecture` passes with `internal/stats/` in domain

### Test cases

- **Create new database**: `Open(":memory:")` succeeds and all three tables
  exist (query `sqlite_master`)
- **Idempotent open**: Calling `Open()` twice on the same database does not
  error or drop data
- **Close**: `Close()` after `Open()` returns nil
- **Tables have expected columns**: INSERT into each table with all columns
  succeeds

---

## Issue 459: Stats write functions

**Blocked by**: #458

### Description

Add write functions to `internal/stats/` that insert run, issue outcome, and
step result rows. Writes are idempotent — re-writing the same run updates
rather than duplicates (INSERT OR REPLACE). These functions are called by the
orchestrator and command layer at run finalization.

### Key constraints

- New file `internal/stats/write.go`:
  - `WriteRun(db *DB, run RunRecord) error` — inserts or replaces a row in
    the `runs` table
  - `WriteIssueOutcome(db *DB, outcome IssueOutcomeRecord) error` — inserts
    or replaces a row in `issue_outcomes`
  - `WriteStepResult(db *DB, step StepResultRecord) error` — inserts or
    replaces a row in `step_results`
- New file `internal/stats/types.go`:
  - `RunRecord` struct matching the `runs` table columns
  - `IssueOutcomeRecord` struct matching `issue_outcomes` columns
  - `StepResultRecord` struct matching `step_results` columns
  - These are simple value types, not domain types — they exist purely for
    the stats write/read boundary
- All writes use `INSERT OR REPLACE` for idempotency
- Flags field on `StepResultRecord` is `[]string`, serialized to JSON text
  for storage

### Acceptance criteria

- [ ] `WriteRun` inserts a row into `runs` with all fields populated
- [ ] `WriteIssueOutcome` inserts a row into `issue_outcomes`
- [ ] `WriteStepResult` inserts a row into `step_results`
- [ ] Re-writing the same `run_id` updates the row (no duplicate key error)
- [ ] Flags are stored as a JSON array string and round-trip correctly

### Test cases

- **Write and read run**: Write a `RunRecord`, query it back, verify all fields
- **Write and read outcome**: Write an `IssueOutcomeRecord`, query it back
- **Write and read step**: Write a `StepResultRecord` with flags
  `["low_cost", "no_diff_read"]`, query back, verify JSON round-trip
- **Idempotent write**: Write a run, update its `implemented` count, write
  again — only one row exists with the updated value
- **Multiple issues per run**: Write 3 outcomes for the same `run_id` — all
  3 rows exist

---

## Issue 460: Stats query functions

**Blocked by**: #459

### Description

Add read/query functions to `internal/stats/` that return aggregated data
suitable for both `godark analyze` and the dashboard. These functions return
the same types that `internal/analysis/` currently computes from run data files,
so the existing analysis pipeline can accept data from either source.

### Key constraints

- New file `internal/stats/query.go`:
  - `QueryRuns(db *DB, filter RunFilter) ([]RunRecord, error)` — list runs
    with optional repo, milestone, and date range filters
  - `QueryIssueOutcomes(db *DB, filter RunFilter) ([]IssueOutcomeRecord, error)`
    — all outcomes matching the filter
  - `QueryStepResults(db *DB, filter RunFilter) ([]StepResultRecord, error)`
    — all step results matching the filter
  - `RunFilter` struct: `Repo string`, `Milestone string`,
    `Since time.Time`, `Until time.Time` (zero values mean no filter)
- Queries use parameterized SQL (no string interpolation)
- Results are sorted by `started_at` ascending (chronological)

### Acceptance criteria

- [ ] `QueryRuns` with empty filter returns all runs
- [ ] `QueryRuns` with `Repo` filter returns only matching runs
- [ ] `QueryRuns` with `Since`/`Until` filters date range correctly
- [ ] `QueryIssueOutcomes` returns outcomes joined to filtered runs
- [ ] `QueryStepResults` returns step data joined to filtered runs

### Test cases

- **No filter**: Write 3 runs across 2 repos, query with empty filter — get
  all 3
- **Repo filter**: Filter by `"org/repo-a"` — get only repo-a runs
- **Date range**: Write runs at different timestamps, filter with Since/Until —
  get only runs in range
- **Outcomes join**: Write 2 runs with 3 outcomes each, filter by repo — get
  only matching outcomes
- **Steps join**: Write step results for 2 runs, filter by repo — get only
  matching steps
- **Empty result**: Query with non-existent repo returns empty slice, not error

---

## Issue 461: Wire stats writes into FinalizeRun callers

**Blocked by**: #459

### Description

Hook the stats write functions into the three places that call
`FinalizeRun()`: `orchestrator.Run()`, `implement.go`'s `finalizeRunData()`,
and `watch.go`'s `handleChangesRequested()`. After writing `run.json`, also
write the run summary, issue outcomes, and step results to the SQLite database.

The database is opened once at run startup and closed on exit. The database
path defaults to `~/.godark/stats.db`.

### Key constraints

- Modify `internal/orchestrator/orchestrator.go`:
  - Open `stats.DB` early in `Run()` (after config is loaded)
  - After `writer.FinalizeRun(summary)` (line 529), call `stats.WriteRun()`
    with the run metadata and summary
  - Iterate issue outcomes and step results from the writer's run directory,
    call `stats.WriteIssueOutcome()` and `stats.WriteStepResult()` for each
  - Close `stats.DB` in a defer
  - Stats write failures are logged as warnings, never abort the run
- Modify `internal/cmd/implement.go`:
  - Same pattern: open DB, write stats after `FinalizeRun()`, close in defer
- Modify `internal/cmd/watch.go`:
  - Same pattern for the watch-initiated fix cycle
- The stats DB path could be derived from `~/.godark/stats.db` (same base as
  runs directory)
- Reading step results from the run directory for stats persistence can reuse
  `rundata.Reader.LoadRun()` or read the files directly

### Acceptance criteria

- [ ] `orchestrator.Run()` writes run, outcome, and step data to SQLite after
  FinalizeRun
- [ ] `implement` command writes stats after FinalizeRun
- [ ] `watch` command writes stats after FinalizeRun
- [ ] Stats write failure is logged as a warning, does not abort the run
- [ ] Database file is created at `~/.godark/stats.db` on first run

### Test cases

- **Orchestrator writes stats**: After a successful run with 2 issues, the
  stats DB contains 1 run row, 2 outcome rows, and step result rows for each
  step
- **Implement writes stats**: After `godark implement 42`, the stats DB
  contains the run and outcome
- **Stats failure non-blocking**: If the DB path is unwritable, the run
  completes normally with a warning log
- **Idempotent on re-run**: Running the same issues twice produces updated
  (not duplicate) rows

---

## Issue 462: Switch analyze command to read from SQLite

**Blocked by**: #460, #461
callers

### Description

Update `godark analyze` to read from the SQLite database instead of scanning
run directories. The existing `analysis.Aggregate()`, `analysis.DetectGaps()`,
and `analysis.ComputeTrends()` functions continue to work — they receive data
loaded from the database instead of from the filesystem.

Add a `--legacy` flag that falls back to the old filesystem scan for cases
where users need to analyze runs that predate the stats database.

### Key constraints

- Modify `internal/cmd/analyze.go`:
  - Default path: open `~/.godark/stats.db`, query runs/outcomes/steps with
    the existing filter flags (repo, milestone, since, until)
  - Convert `stats.RunRecord` + `stats.IssueOutcomeRecord` +
    `stats.StepResultRecord` into `rundata.RunDetail` structs so the existing
    `analysis.Aggregate()` / `DetectGaps()` / `ComputeTrends()` functions
    work unchanged
  - Add `--legacy` flag: when set, use the current filesystem-based loading
    (preserves the old behavior)
- The conversion function lives in `internal/stats/convert.go`:
  - `ToRunDetails(runs []RunRecord, outcomes []IssueOutcomeRecord, steps []StepResultRecord) []rundata.RunDetail`
  - Groups outcomes and steps by run_id, assembles RunDetail structs

### Acceptance criteria

- [ ] `godark analyze` reads from SQLite by default
- [ ] `--repo`, `--milestone`, `--since`, `--until` filters work against
  the database
- [ ] `--json` output format is unchanged
- [ ] `--legacy` flag falls back to filesystem scan
- [ ] Output is identical whether reading from SQLite or filesystem (for the
  same data)

### Test cases

- **Default reads from DB**: With a populated stats.db and no `--legacy` flag,
  analyze produces output from the database
- **Filters work**: `--repo org/repo-a` returns only repo-a data from the DB
- **Legacy fallback**: `--legacy` scans `~/.godark/runs/` as before
- **Empty database**: `godark analyze` with an empty stats.db prints
  "No runs found" (not an error)
- **Conversion round-trip**: `ToRunDetails()` produces `RunDetail` structs
  that `Aggregate()` processes identically to filesystem-loaded data

---

## Issue 463: Switch dashboard analysis to read from SQLite

**Blocked by**: #460, #461
callers

### Description

Update the dashboard's `buildAnalysisData()` handler to read from the SQLite
database instead of scanning run directories. Uses the same conversion function
from `internal/stats/convert.go` to feed data into the existing analysis
functions.

### Key constraints

- Modify `internal/dashboard/handlers.go`:
  - `buildAnalysisData()` (line 818): open `stats.DB`, query with repo filter,
    convert to `RunDetail`, pass to `Aggregate()` / `DetectGaps()` /
    `ComputeTrends()`
  - The `stats.DB` instance should be opened once at server startup and shared
    across requests (read-only for the dashboard)
- Modify `internal/dashboard/server.go`:
  - Add `statsDB *stats.DB` field to the server struct
  - Open the database at server startup, close on shutdown
  - Pass to handlers that need it

### Acceptance criteria

- [ ] Dashboard `/analysis` page reads from SQLite
- [ ] Repo filter dropdown works against database data
- [ ] Trend charts render from database data
- [ ] Dashboard starts without error if stats.db doesn't exist yet (empty
  state)

### Test cases

- **Analysis page loads**: With a populated stats.db, `/analysis` renders
  without error
- **Empty database**: `/analysis` with no stats.db shows "no data" state
- **Repo filter**: Selecting a repo in the dropdown filters the displayed
  metrics
- **Server startup**: Dashboard opens stats.db at startup and closes on
  shutdown

---

## Issue 464: Retry recovery rate metric

**Blocked by**: #462

### Description

Add a "retry recovery rate" metric: of issues that had at least one retry,
what percentage eventually succeeded (status `implemented` or `ready-to-merge`)
vs exhausted retries (status `failed` or `needs-human-review`). Display in
both `godark analyze` and the dashboard.

### Key constraints

- Modify `internal/analysis/analysis.go`:
  - Add `RecoveryRate float64` and `RetriedCount int` to `RetryStats` struct
  - In `Aggregate()`: count issues with retry steps that ended with a
    successful outcome vs failed/needs-human-review
  - `RecoveryRate = successfulRetries / totalRetried` (0.0 if none retried)
- Modify `internal/cmd/analyze.go`:
  - Add recovery rate row to the retry stats table output
- Modify `internal/dashboard/templates/analysis.html`:
  - Add recovery rate to the retry statistics card

### Acceptance criteria

- [ ] `RetryStats` includes `RecoveryRate` and `RetriedCount`
- [ ] Recovery rate is 0.0 when no issues retried
- [ ] Recovery rate correctly reflects successful vs failed retried issues
- [ ] `godark analyze` displays recovery rate in retry stats table
- [ ] Dashboard shows recovery rate in retry statistics card

### Test cases

- **No retries**: All issues succeed on first attempt — `RecoveryRate` is 0.0,
  `RetriedCount` is 0
- **All retries succeed**: 3 issues retry, all end `implemented` —
  `RecoveryRate` is 1.0
- **Mixed**: 2 retried issues succeed, 1 exhausts — `RecoveryRate` is ~0.67
- **CLI output**: `godark analyze` output includes "Recovery rate: 66.7%"

---

## Issue 465: Cost breakdown by step metric

**Blocked by**: #462

### Description

Add a cost breakdown showing what percentage of total cost goes to each step
type (implement, quality-review, functional-review, retries, recon,
spec-generator). Display in both `godark analyze` and the dashboard.

### Key constraints

- Modify `internal/analysis/analysis.go`:
  - Add `CostByStep map[string]float64` to `CostStats` struct (step name →
    total USD)
  - In `Aggregate()`: sum costs by step name from `StepResult` data
- Modify `internal/cmd/analyze.go`:
  - Add a "Cost by step" table to CLI output showing step name, total cost,
    and percentage of total
- Modify `internal/dashboard/templates/analysis.html`:
  - Add a cost breakdown card (table or horizontal bar chart)

### Acceptance criteria

- [ ] `CostStats.CostByStep` populated with per-step cost totals
- [ ] Percentages sum to ~100% (within floating point tolerance)
- [ ] `godark analyze` displays cost breakdown table
- [ ] Dashboard shows cost breakdown visualization

### Test cases

- **Single step**: Only implement steps present — implement is 100%
- **Multiple steps**: Implement $3.00, quality-review $1.00, retry $0.50 —
  percentages are 66.7%, 22.2%, 11.1%
- **Zero cost**: No cost data — `CostByStep` is empty map
- **CLI output**: Table shows step names sorted by cost descending

---

## Issue 466: Duration trends by step metric

**Blocked by**: #462

### Description

Add per-step duration tracking over time to help identify when `agent_timeout`
(default 30m) needs adjusting. Compute average duration per step type per run,
surface as a trend line in the dashboard and summary in the CLI.

### Key constraints

- Modify `internal/analysis/trends.go`:
  - Add `AvgImplementDuration float64` and `AvgReviewDuration float64` to
    `TrendPoint` (seconds)
  - In `ComputeTrends()`: calculate average implement and review step
    durations per run from step result data
- Modify `internal/cmd/analyze.go`:
  - Add duration summary to CLI output: average implement duration, average
    review duration across all filtered runs
- Modify `internal/dashboard/templates/analysis.html`:
  - Add a duration trend chart (implement and review duration over time)
- `TrendPoint` already includes `Timestamp`, `Repo`, `Milestone` — duration
  fields extend the existing struct

### Acceptance criteria

- [ ] `TrendPoint` includes `AvgImplementDuration` and `AvgReviewDuration`
- [ ] Duration trends computed from step_results duration_seconds
- [ ] `godark analyze` displays average durations
- [ ] Dashboard shows duration trend chart

### Test cases

- **Single run**: One run with implement at 300s, review at 120s — trend point
  shows 300.0 and 120.0
- **Multiple runs**: Duration trends show values per run in chronological order
- **No step data**: Missing duration data produces 0.0 (not NaN or error)
- **CLI output**: Shows "Avg implement duration: 5m00s, Avg review duration:
  2m00s"

---

## Issue 467: Success rate by repo and surface verify stats

**Blocked by**: #462

### Description

Add per-repo success rate breakdown for users running godark against multiple
repositories. Also surface the verify check failure data that is already
computed in `Report.VerifyStats` but never displayed in CLI or dashboard.

### Key constraints

- Modify `internal/analysis/analysis.go`:
  - Add `RepoStats map[string]RepoSummary` to `Report` (repo → summary)
  - `RepoSummary` struct: `Total int`, `Implemented int`, `Failed int`,
    `SuccessRate float64`
  - In `Aggregate()`: group outcomes by repo, compute per-repo stats
- Modify `internal/cmd/analyze.go`:
  - Add "Success by repo" table to CLI output
  - Add "Verify check failures" table showing each check name and its failure
    count/rate (reading from existing `Report.VerifyStats`)
- Modify `internal/dashboard/templates/analysis.html`:
  - Add repo success rate card
  - Add verify check failure card (table with check name, failure count,
    failure rate)

### Acceptance criteria

- [ ] `Report.RepoStats` populated with per-repo success rates
- [ ] `godark analyze` shows per-repo success table
- [ ] Dashboard shows repo success rate breakdown
- [ ] Verify stats displayed in CLI output
- [ ] Verify stats displayed in dashboard

### Test cases

- **Single repo**: One repo with 8 implemented, 2 failed — success rate 80%
- **Multiple repos**: Two repos with different rates shown separately
- **Verify stats shown**: Verify check "lint" failed in 3 of 10 issues —
  displayed as "lint: 30% failure rate"
- **No verify data**: No verify results — verify section omitted (not empty
  table)

---

## Issue 468: Rework prompt gaps to flag-based correlation

**Blocked by**: #462

### Description

Replace the confusing "with/without quality reviewer" comparison in prompt gaps
analysis with flag-to-outcome correlation. For each quality flag code that
appears across issues, compute the failure rate of issues that have the flag vs
issues that don't. This directly answers "which quality problems predict
failure?" instead of the less actionable "does having a quality reviewer help?"

### Key constraints

- Modify `internal/analysis/gaps.go`:
  - Remove `qualityReviewGap()` condition
  - Add flag-based correlation: for each unique flag code, split issues into
    "has this flag" vs "doesn't have this flag" groups, compute failure rate
    for each
  - Example output: "Issues with `no_diff_read` fail at 75% (12 samples) vs
    20% baseline (48 samples)"
  - Keep the scenario spec gap (still useful)
  - Keep the exhausted retries listing
  - Same minimum 3-sample threshold per group
- Modify `internal/cmd/analyze.go`:
  - Update prompt gaps output to show flag-based correlations
- Modify `internal/dashboard/templates/analysis.html`:
  - Update prompt gaps card with new format showing flag code, failure rate
    with flag, failure rate without, and sample counts

### Acceptance criteria

- [ ] Flag-to-outcome correlation replaces "with/without quality reviewer" gap
- [ ] Each quality flag code gets its own gap entry when samples are sufficient
- [ ] Scenario spec gap preserved unchanged
- [ ] Exhausted retries listing preserved unchanged
- [ ] `godark analyze` shows updated prompt gaps
- [ ] Dashboard shows updated prompt gaps card

### Test cases

- **Flag correlation**: 10 issues with `no_diff_read` (7 failed), 40 without
  (8 failed) — gap shows 70% vs 20%
- **Multiple flags**: Both `no_diff_read` and `low_cost` appear — each gets
  its own gap entry sorted by failure rate delta
- **No flags**: No quality flags across any issues — no flag gaps reported
  (scenario spec gap may still appear)
- **Below threshold**: A flag appears on only 2 issues — skipped (minimum 3
  samples)
- **Quality reviewer gap removed**: The old "with/without quality reviewer"
  comparison no longer appears in output
