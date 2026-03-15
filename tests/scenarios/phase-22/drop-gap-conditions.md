# Scenario: Drop scenario spec gap and exhausted retries listing

Relates to: Issue #491

## Setup
- `internal/analysis/gaps.go` with `DetectGaps()` function
- Test data with issues that have and lack scenario specs
- Test data with issues in `needs-human-review` status
- Flag-based correlation data preserved for comparison

## Cases

### Scenario spec gap no longer reported
10 issues with spec generator steps (5 succeed, 5 fail). 10 issues without (3 succeed, 7 fail).
- No gap entry mentions "scenario" or "spec"
- Previously this would have produced a gap; now it does not

### Exhausted retries no longer listed
3 issues with status `needs-human-review`.
- No gap entry lists issue numbers for exhausted retries
- Previously this would have produced a listing; now it does not

### Flag-based correlations still work
10 issues with `no_diff_read` flag (7 failed), 40 without (8 failed).
- A gap entry exists for `no_diff_read`
- `FailRateWith` is 0.70
- `FailRateWithout` is 0.20

### Empty gaps when only removed conditions apply
All issues have scenario specs. Some have exhausted retries. No quality flags.
- `DetectGaps()` returns an empty slice

### Multiple flag correlations preserved
Both `no_diff_read` and `low_cost` flags appear with sufficient samples.
- Two gap entries exist, one for each flag
- Sorted by failure rate delta descending
