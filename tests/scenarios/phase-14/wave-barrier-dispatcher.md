# Scenario: wave barrier dispatcher with concurrent workers

Relates to: Issue #598

## Setup
- `internal/orchestrator/orchestrator.go` with wave barrier dispatch logic
- Stubbed `ProcessIssue` with configurable delays and outcomes
- Config with `concurrency.max_workers` set

## Cases

### Serial mode identical to current behavior
Run with `max_workers: 1` and 3 independent issues.
- Issues process one at a time in order
- Output matches pre-concurrency behavior exactly
- Each issue completes before the next starts

### Concurrent dispatch within wave
Run with `max_workers: 3` and 3 independent issues.
- All three issues start processing before any completes
- All three results are collected after the wave

### Worker cap respected
Run with `max_workers: 2` and 5 independent issues.
- At most 2 issues process simultaneously at any point
- All 5 issues complete across multiple waves

### Context cancellation stops workers
Start a wave with 3 concurrent workers and cancel the context mid-wave.
- All workers receive cancellation and exit
- No new waves are dispatched after cancellation

### Results collected with correct issue numbers
Run with `max_workers: 3` and 3 issues with different outcomes (pass, fail, needs-review).
- Each result maps to the correct issue number
- Statuses match the stubbed outcomes
- No result is lost or duplicated

### Shared state updated after wave only
Run with `max_workers: 2` and 4 independent issues.
- `runStats` counters are updated only after each wave completes
- No concurrent writes to shared state during wave execution
