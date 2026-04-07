# Scenario: post-wave merge serializer and continuation

Relates to: Issue #751

## Setup
- An orchestrator run with `max_workers > 1`
- Multiple issues completing a wave with mixed outcomes
- Stubbed merge and dependency resolution functions

## Cases

### All succeed and merge in order
- GIVEN a wave of 3 issues all returning `StatusImplemented`
- WHEN the wave completes
- THEN all 3 merge in ascending issue number order and dependency re-resolution runs

### Mixed results continue the run
- GIVEN a wave of 3 issues where 1 fails and 2 succeed
- WHEN the wave completes
- THEN the 2 successes merge, the failure is counted, and the run continues to the next wave

### All fail exits wave loop
- GIVEN a wave of 3 issues all returning `StatusFailed`
- WHEN the wave completes
- THEN no merges are attempted and the wave loop exits

### Merge order is deterministic
- GIVEN issues completing in order #3, #1, #2
- WHEN the post-wave merge runs
- THEN merges execute in order #1, #2, #3

### Blocked by failure reported
- GIVEN issue B depends on issue A, and issue A fails in wave 1
- WHEN dependency re-resolution runs after wave 1
- THEN issue B does not appear in the next wave's processable list and is counted as blocked in the final summary

### Rebase runs between consecutive merges
- GIVEN a wave with 3 successful issues
- WHEN the post-wave merge serializes them
- THEN `runPreMergeRebasePhase` executes before each merge after the first
