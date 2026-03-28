# Scenario: godark trace CLI command

Relates to: Issue #702

## Setup
- A temporary SQLite stats.db seeded with:
  - Two runs (run-1 older, run-2 newer) for repo "org/repo"
  - Issue #42 processed in both runs with trace IDs "trace-old" (run-1) and "trace-new" (run-2)
  - Issue #99 processed in run-2 only with trace ID "trace-99"
  - Step results for each trace: recon, planner, implement, quality-review, functional-review with varying durations and costs
  - Issue outcomes with status "implemented" for #42 and "failed" for #99

## Cases

### Trace by issue number resolves most recent
- GIVEN stats.db contains two runs processing issue #42
- WHEN `godark trace 42` is executed
- THEN the output header shows trace ID "trace-new" (from the newer run)
- THEN the timeline lists all steps from run-2 in chronological order

### Trace by trace ID directly
- GIVEN stats.db contains trace "trace-old"
- WHEN `godark trace trace-old` is executed
- THEN the output shows the timeline for trace "trace-old" regardless of recency

### Timeline shows step details
- GIVEN stats.db contains trace "trace-new" with 5 steps
- WHEN `godark trace trace-new` is executed
- THEN each step row shows step name, duration, cost, and started_at
- THEN steps are ordered chronologically by started_at

### Repo filter narrows results
- GIVEN stats.db contains runs for "org/repo" and "org/other"
- WHEN `godark trace 42 --repo org/repo` is executed
- THEN only results from "org/repo" are shown

### Run filter selects specific run
- GIVEN issue #42 exists in both run-1 and run-2
- WHEN `godark trace 42 --run run-1` is executed
- THEN the output shows trace "trace-old" from run-1

### JSON output format
- GIVEN stats.db contains trace "trace-new"
- WHEN `godark trace 42 --json` is executed
- THEN the output is valid JSON containing a steps array and issue outcome

### No results for unknown issue
- GIVEN stats.db does not contain issue #999
- WHEN `godark trace 999` is executed
- THEN the error message is "No trace found for issue #999"

### Missing stats database
- GIVEN no stats.db file exists at the expected path
- WHEN `godark trace 42` is executed
- THEN the error message contains "No stats database found"
