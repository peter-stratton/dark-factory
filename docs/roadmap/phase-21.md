## Phase 21: Analytics Persistence ✅

**Goal**: Run statistics are persisted to a SQLite database (`~/.godark/stats.db`)
at run finalization, surviving run directory deletion. `godark analyze` and the
dashboard read from the database instead of scanning run directories, enabling
improved metrics: retry recovery rate, cost breakdown by step, duration trends,
and success rate by repo.

**Milestone**: `Phase 21` | **Label**: `phase-21`

### SQLite stats store
- SQLite dependency — add `modernc.org/sqlite` (pure Go, no CGO) to `go.mod`
- Schema and migrations — `internal/stats/` package; tables for runs, issue
  outcomes, step results (per-step cost, duration, flags); migration framework
  for schema evolution
- Write on finalize — `FinalizeRun()` writes aggregate stats to the database;
  one row per issue outcome, one row per step result; idempotent (re-finalizing
  a run updates rather than duplicates)
- Backfill command — `godark analyze --backfill` scans existing
  `~/.godark/runs/` directories and populates the database from historical
  run data
- Update `architecture.json` — add `internal/stats/` to the appropriate layer

### Improved analytics
- Retry recovery rate — of issues that retried, what percentage eventually
  succeeded vs exhausted retries
- Cost breakdown by step — implement vs quality review vs functional review vs
  retries as percentages of total cost
- Duration trends — per-step duration over time; helps identify when
  `agent_timeout` (default 30m) needs adjusting for specific repos
- Success rate by repo — pass/fail breakdown when running against multiple repos
- Surface verify stats — expose the verify check failure data that's currently
  computed but never displayed
- Rework prompt gaps — replace confusing "with/without quality reviewer"
  comparison with flag-to-outcome correlation (e.g., "issues with `no_diff_read`
  fail at 75% vs 20% baseline")
- Update `godark analyze` — read from SQLite instead of scanning run directories
- Update dashboard analysis page — same data source switch, add new metric
  cards and trend charts

**Issues**: #458–#468

**Planning doc**: `docs/planning/phase-21-analytics-persistence.md`

