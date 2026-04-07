# Scenario: rate-limit handling in concurrent waves

Relates to: Issue #752

## Setup
- An orchestrator run with `max_workers > 1`
- Stubbed `processIssueFn` that can return `UsageLimited: true` with a `ResetsAt` time

## Cases

### Rate-limited issue retried after hold
- GIVEN a wave of 3 issues where 1 returns `UsageLimited` and 2 succeed
- WHEN the wave completes
- THEN the 2 successes merge, the orchestrator sleeps until `ResetsAt + 30s`, and the rate-limited issue is retried in the next wave

### All rate-limited sleeps and retries
- GIVEN a wave of 3 issues all returning `UsageLimited`
- WHEN the wave completes
- THEN no merges occur, the orchestrator sleeps until the latest `ResetsAt + 30s`, and all 3 are retried

### Mixed rate limit and failure
- GIVEN a wave where 1 is rate-limited, 1 fails, and 1 succeeds
- WHEN the wave completes
- THEN the success merges, the failure is counted, the orchestrator sleeps, and the rate-limited issue is retried

### Max hold exceeded fails the issue
- GIVEN an issue returning `UsageLimited` with `ResetsAt` more than 6 hours in the future
- WHEN the wave results are processed
- THEN the issue is failed instead of holding

### Context cancelled during hold exits cleanly
- GIVEN the orchestrator sleeping during a rate-limit hold
- WHEN the context is cancelled
- THEN the sleep is interrupted and the function exits without deadlock
