# Scenario: dashboard concurrent status display

Relates to: Issue #602

## Setup
- `internal/dashboard/` templates and handlers
- `internal/rundata/writer.go` with `WriteWaveResult()` method
- Run data from a concurrent run with multiple waves

## Cases

### Wave grouping visible
Run with 2 waves (wave 1: issues #1, #2, #3; wave 2: issues #4, #5).
- Dashboard run detail shows wave 1 and wave 2 sections
- Each wave lists its issues

### Wave timing displayed
Run with a wave that took 45 seconds.
- Wave section shows start time, end time, and duration
- Duration is human-readable (e.g., "45s")

### Wall-clock savings shown
Run with 3 concurrent issues (individual durations: 30s, 25s, 35s; wave duration: 35s).
- Dashboard shows serial estimate (90s) vs actual (35s)
- Savings percentage or absolute time displayed

### Serial run no wave grouping
Run with `max_workers: 1` (all issues in separate single-issue waves).
- Dashboard does not show wave grouping
- Issues display in standard list format without wave headers
