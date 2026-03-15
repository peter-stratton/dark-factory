# Scenario: Retry recovery rate metric

Relates to: Issue #464

## Setup
- `internal/analysis/` package with `Aggregate()` function
- `RetryStats` struct extended with `RecoveryRate` and `RetriedCount`
- Test data with a mix of issues: some succeed on first attempt, some retry and succeed, some retry and exhaust

## Cases

### No retries produces zero recovery rate
Aggregate 5 issues that all succeed on first attempt (no retry steps).
- `RetryStats.RetriedCount` is 0
- `RetryStats.RecoveryRate` is 0.0

### All retries succeed
Aggregate 3 issues that each have retry steps and end with status "implemented".
- `RetryStats.RetriedCount` is 3
- `RetryStats.RecoveryRate` is 1.0

### Mixed retry outcomes
Aggregate 4 issues: 2 retry and succeed ("implemented"), 1 retries and fails ("failed"), 1 succeeds without retrying.
- `RetryStats.RetriedCount` is 3 (excludes the non-retried issue)
- `RetryStats.RecoveryRate` is approximately 0.67 (2/3)

### Needs-human-review counts as not recovered
An issue with retries that ends with status "needs-human-review".
- Counts toward `RetriedCount` but not toward recovered
- Reduces `RecoveryRate`

### CLI output includes recovery rate
Run `godark analyze` with retry data.
- Retry stats table includes a "Recovery rate" row
- Value formatted as percentage (e.g., "66.7%")

### Dashboard shows recovery rate
View `/analysis` with retry data.
- Retry statistics card includes recovery rate
