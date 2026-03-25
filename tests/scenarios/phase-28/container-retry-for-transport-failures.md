# Scenario: Container retry for transport failures

Relates to: Issue #647

## Setup
- `runSandbox()` in `internal/agent/launcher.go` handles `RetryContainer` judgments
- `sandbox.RunContainer` is stubbed to control judge behavior per attempt
- `refreshGHToken` is stubbed to track calls
- Judge config has `ContainerRetryLimit: 2` (default)

## Cases

### Retry succeeds on second container
First `RunContainer` call triggers transport failure (judge returns `RetryContainer`).
Second `RunContainer` call succeeds normally.
- Final result is from the successful second container
- `Result.JudgeKilled` is false
- `Result.ExitCode` reflects the successful run

### Retry limit exhausted
All `RunContainer` calls trigger transport failure (3 total: 1 original + 2 retries).
- Final result has `JudgeKilled: true`
- `Result.JudgeIntervention` contains the transport failure rule
- No more than `ContainerRetryLimit` retries attempted

### Kill judgment does not trigger container retry
Stub judge to return `Kill` (not `RetryContainer`).
- No container retry attempted
- Result returned immediately with `JudgeKilled: true`

### Token refreshed before each retry
First container triggers transport failure, second succeeds.
- `refreshGHToken` called before the retry attempt
- Each retry gets a fresh token
