# Scenario: Failure reason breakdown and timeout rate

Relates to: Issue #490

## Setup
- `internal/analysis/analysis.go` with `Aggregate()` function
- `Report` extended with `FailureReasons map[string]int`, `TimeoutRate float64`
- `DurationStats` extended with `AvgTimeToMergeSeconds float64`
- Test data with issues failing for different reasons: verify failures, review exhaustion, timeouts, and errors

## Cases

### Verify failure categorized
An issue fails with a verify step where `AllPassed == false`.
- `FailureReasons["verify"]` is 1

### Review exhaustion categorized
An issue with status `needs-human-review` (retries exhausted).
- `FailureReasons["exhaustion"]` is 1

### Timeout categorized
An issue has a step with `TimedOut == true`.
- `FailureReasons["timeout"]` is 1

### Error fallback categorized
An issue fails with no verify failure, no exhaustion, no timeout.
- `FailureReasons["error"]` is 1

### Categories are mutually exclusive
4 failed issues, one of each type.
- `FailureReasons` has 4 entries, each with count 1
- Total of all values equals 4

### No failures produces empty map
All 5 issues succeed.
- `FailureReasons` is an empty map (not nil)

### Timeout rate calculation
20 total steps across all issues, 2 have `TimedOut == true`.
- `TimeoutRate` is 0.10

### Timeout rate zero when no timeouts
15 steps, none timed out.
- `TimeoutRate` is 0.0

### Average time to merge for implemented issues
3 implemented issues: first step started at T, last step finished at T+300s, T+600s, T+900s respectively.
- `AvgTimeToMergeSeconds` is 600.0

### Time to merge excludes failed issues
2 implemented issues (300s, 600s each) and 1 failed issue (1200s).
- `AvgTimeToMergeSeconds` is 450.0 (only implemented counted)

### Time to merge zero when none implemented
All issues fail.
- `AvgTimeToMergeSeconds` is 0.0
