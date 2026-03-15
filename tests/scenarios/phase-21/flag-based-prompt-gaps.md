# Scenario: Rework prompt gaps to flag-based correlation

Relates to: Issue #468

## Setup
- `internal/analysis/gaps.go` with `DetectGaps()` function
- Quality flags collected from step results across all issues
- Test data with issues that have various flag combinations and outcomes
- Minimum sample threshold of 3 per group

## Cases

### Flag correlation detected
10 issues have the `no_diff_read` flag (7 failed), 40 issues don't (8 failed).
- A gap entry exists for `no_diff_read`
- `FailRateWith` is 0.70 (7/10)
- `FailRateWithout` is 0.20 (8/40)
- Finding text mentions "no_diff_read"

### Multiple flags each get their own entry
Both `no_diff_read` and `low_cost` flags appear across different issue sets.
- Two separate gap entries exist
- Each has independent failure rate calculations
- Sorted by absolute difference in failure rates (largest gap first)

### No flags produces no flag gaps
No quality flags appear on any issues.
- No flag-based gap entries in the result
- Scenario spec gap may still appear if applicable

### Below threshold flag is skipped
A flag appears on only 2 issues (below minimum 3).
- No gap entry for that flag
- Other flags with sufficient samples still included

### Scenario spec gap preserved
Issues with scenario specs have 10% failure rate, issues without have 40%.
- A gap entry exists for scenario spec presence
- Not affected by the flag correlation changes

### Exhausted retries listing preserved
3 issues exhausted their retries.
- A gap entry lists the issue numbers and titles
- Format unchanged from pre-Phase 21

### Quality reviewer gap removed
The old "with/without quality reviewer" gap no longer appears.
- No gap entry mentions "quality reviewer"
- Flag-based correlations appear instead

### CLI shows updated gaps
Run `godark analyze` with flag data.
- Prompt gaps section shows flag codes with failure rates
- Format: "Issues with `no_diff_read` fail at 70.0% (10 samples) vs 20.0% baseline (40 samples)"

### Dashboard shows updated gaps
View `/analysis` with flag data.
- Prompt gaps card shows flag-based correlations
- Each entry shows flag code, failure rate with/without, and sample counts

### Flag on successful issues doesn't create gap
A flag appears on 5 issues, all of which succeed.
- Gap entry exists but shows 0% failure rate with flag
- Delta is negative (flag correlates with success, not failure)
- Still displayed — the user may find this informative
