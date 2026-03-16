# Scenario: Detect external merges during watch polling

Relates to: Issue #521

## Setup
- `internal/watch/` package with `DetectMergedPRs()` function
- Watch polling loop running after `godark run --watch`
- Stubbed GitHub API for PR/issue state queries
- Set of issue numbers from the original run that had unmerged PRs

## Cases

### Detect newly merged PR
Issue #42's PR was open on the last poll. Human merges it externally.
- `DetectMergedPRs()` returns `[42]`

### Already detected merge not re-reported
Issue #42 was detected as merged on the previous poll cycle.
- `DetectMergedPRs()` does not return #42 again

### No merges returns empty slice
All tracked PRs are still open.
- `DetectMergedPRs()` returns an empty slice (not nil)
- No error

### Multiple merges detected
Issues #42 and #43 both merged since last poll.
- `DetectMergedPRs()` returns `[42, 43]`

### Detection runs alongside review polling
Watch polling tick fires.
- Both review detection (CHANGES_REQUESTED/APPROVED) and merge detection run on the same cycle
- No interference between the two checks

### APPROVED handler merge is detected
Watch's own APPROVED handler merges PR #42.
- Next poll cycle detects #42 as merged
- Works the same as an externally-merged PR
