# Scenario: wave barrier dispatcher with concurrent workers

Relates to: Issue #750

## Setup
- An orchestrator run with `concurrency.max_workers` configured
- Multiple independent (non-blocked) issues in the milestone
- Stubbed `processIssueFn` with configurable delays

## Cases

### Serial mode unchanged
- GIVEN `max_workers: 1` and 3 independent issues
- WHEN `processIssues` runs
- THEN issues are processed one at a time in sequence, identical to pre-concurrency behavior

### Concurrent dispatch
- GIVEN `max_workers: 3` and 3 independent issues
- WHEN `processIssues` runs
- THEN all three workers run concurrently (overlapping execution times)

### Worker cap respected
- GIVEN `max_workers: 2` and 5 independent issues
- WHEN `processIssues` runs
- THEN at most 2 issues execute simultaneously at any point

### Context cancellation stops workers
- GIVEN `max_workers: 3` and 3 independent issues
- WHEN the context is cancelled mid-wave
- THEN all workers exit and results are collected without deadlock

### All batch issues marked seen before dispatch
- GIVEN `max_workers: 3` and 3 independent issues
- WHEN the wave dispatches
- THEN all 3 issues are in the `seen` map before any worker starts
