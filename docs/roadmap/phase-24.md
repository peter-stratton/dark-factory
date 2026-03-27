## Phase 24: Container Resource Tracking ✅

**Goal**: Every agent container execution records peak memory and CPU usage.
These metrics flow through run data, the stats database, the analyze command,
the dashboard, and sprint reports — giving operators the data they need to plan
bounded concurrency.

**Milestone**: `Phase 24` | **Label**: `phase-24`

- Poll Docker Stats API during `RunContainer` and capture peak memory (RSS) and
  CPU-seconds
- Add resource fields to `StepResult` and write to per-step JSON files
- Add `peak_memory_bytes` and `cpu_seconds` columns to `step_results` table
- Include resource stats in `analysis.Report` (per-step and per-issue
  aggregates)
- Add resource metrics to `analyze` CLI output (table + JSON)
- Surface resource stats in dashboard issue detail view
- Include resource summary in `report` sprint output
- Add resource stats to `--no-sandbox` mode (capture from host process instead
  of container)

**Issues**: #543–#548

**Planning doc**: `docs/planning/phase-24-container-resource-tracking.md`

