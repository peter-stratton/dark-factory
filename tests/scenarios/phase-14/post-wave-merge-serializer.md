# Scenario: post-wave merge serializer and failure abort

Relates to: Issue #599

## Setup
- Wave barrier dispatcher producing wave results
- Stubbed merge and rebase functions
- Config with `max_rebase_attempts` set

## Cases

### All issues succeed and merge in order
Wave of 3 issues (#10, #5, #8) all return StatusImplemented.
- Issues merge in ascending order: #5, #8, #10
- Rebase phase runs before merging #8 and #10
- Dependency re-resolution runs after all merges

### Mixed results merge successes then abort
Wave of 3 issues: #1 succeeds, #2 fails, #3 succeeds.
- #1 and #3 merge successfully
- Run aborts after wave — no further waves dispatched
- Summary reports #2 as failed

### All issues fail skips merge
Wave of 3 issues all return StatusFailed.
- No merge attempts are made
- Run aborts with failure summary
- All 3 issues reported as failed

### Rebase failure labels PR for human review
During post-wave merge, PR for issue #2 fails rebase after `max_rebase_attempts`.
- Issue #2 PR is labeled `needs-human-review`
- Other successful issues in the wave still merge
- Run continues to next wave if no failures otherwise

### Blocked issues counted in summary
Wave of 2 issues: #1 fails. Issues #3 and #4 are blocked by #1.
- #3 and #4 are reported as blocked in the run summary
- Blocked count in summary equals 2

### Re-resolution only on all-success wave
Wave of 3 issues: all succeed. Second wave available after re-resolution.
- `refreshAndCategorize()` is called after merging
- Newly unblocked issues are dispatched in the next wave
