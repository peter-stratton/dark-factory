# Scenario: First-pass success rate and overview metrics

Relates to: Issue #489

## Setup
- `internal/analysis/analysis.go` with `Aggregate()` function
- `Report` struct extended with `FirstPassSuccessRate`, `FirstPassCount`, `WastedCostUSD`, `AvgCostPerSuccessUSD`
- `RepoSummary` extended with `FirstPassRate`, `AvgCostPerIssueUSD`
- Test data with a mix of first-pass successes, retried successes, and failures

## Cases

### All issues succeed on first pass
Aggregate 5 issues, each with 0 retry steps, all with status "implemented".
- `FirstPassSuccessRate` is 1.0
- `FirstPassCount` is 5

### Mixed first-pass and retried successes
Aggregate 5 issues: 3 succeed with 0 retries, 2 succeed after retries.
- `FirstPassSuccessRate` is 0.6 (3/5)
- `FirstPassCount` is 3

### No first-pass successes
Aggregate 4 issues: 2 retry and succeed, 2 fail.
- `FirstPassSuccessRate` is 0.0
- `FirstPassCount` is 0

### Wasted cost on failed issues
2 issues fail with step costs totaling $1.50. 3 issues succeed with step costs totaling $3.00.
- `WastedCostUSD` is 1.50
- Total cost is 4.50

### Wasted cost zero when all succeed
5 issues, all implemented.
- `WastedCostUSD` is 0.0

### Average cost per success
Total cost $6.00 across 4 issues, 3 implemented.
- `AvgCostPerSuccessUSD` is 2.00 (6.00 / 3)

### No successes avoids division by zero
All issues fail. Total cost $4.00.
- `AvgCostPerSuccessUSD` is 0.0
- No panic or NaN

### Per-repo first-pass rate
Repo-a: 3 of 4 issues first-pass. Repo-b: 1 of 3 issues first-pass.
- `RepoStats["org/repo-a"].FirstPassRate` is 0.75
- `RepoStats["org/repo-b"].FirstPassRate` is approximately 0.33

### Per-repo avg cost per issue
Repo-a: 4 issues, $8.00 total cost. Repo-b: 3 issues, $6.00 total cost.
- `RepoStats["org/repo-a"].AvgCostPerIssueUSD` is 2.00
- `RepoStats["org/repo-b"].AvgCostPerIssueUSD` is 2.00
