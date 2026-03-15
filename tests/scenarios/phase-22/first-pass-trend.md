# Scenario: First-pass success rate trend

Relates to: Issue #493

## Setup
- `internal/analysis/trends.go` with `ComputeTrends()` function
- `TrendPoint` extended with `FirstPassSuccessRate float64`
- Test data with multiple completed runs containing varying first-pass rates

## Cases

### All first-pass run
One run with 5 issues, 0 retries each, all implemented.
- `TrendPoint.FirstPassSuccessRate` is 1.0

### Mixed first-pass run
One run with 5 issues: 3 succeed first-pass, 2 retry and succeed.
- `TrendPoint.FirstPassSuccessRate` is 0.6

### No first-pass successes in run
One run with 4 issues: all retry at least once.
- `TrendPoint.FirstPassSuccessRate` is 0.0

### Multiple runs have independent rates
Run 1: 4/5 first-pass. Run 2: 2/5 first-pass. Run 3: 5/5 first-pass.
- Three trend points with rates 0.8, 0.4, 1.0 respectively
- Points are in chronological order

### Empty run produces zero rate
A run with no finished issues.
- `FirstPassSuccessRate` is 0.0
- No NaN or infinity

### Unfinished runs excluded
Two runs: one finished, one still in progress (no `FinishedAt`).
- Only the finished run produces a trend point
