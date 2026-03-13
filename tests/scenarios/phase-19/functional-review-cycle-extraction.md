# Scenario: Extract functional review cycle function

Relates to: Issue #408

## Setup
- The `internal/agent/loop.go` file contains `runFunctionalReviewCycle()`
- Stub agent functions simulate review verdicts and merge outcomes

## Cases

### Approved and merged
Stub `Review` to return `APPROVED` and merge to succeed. Call `runFunctionalReviewCycle()`.
- Returns `(StatusImplemented, true, nil)`

### Approved but auto-merge disabled
Stub `Review` to return `APPROVED`. Set `auto_merge.feature: "none"`. Call `runFunctionalReviewCycle()`.
- Returns `(StatusReadyToMerge, false, nil)`

### Changes requested then approved
Stub `Review` to return `CHANGES_REQUESTED` then `APPROVED`. Call `runFunctionalReviewCycle()`.
- Returns success after retry

### Max retries exhausted
Stub `Review` to always return `CHANGES_REQUESTED`. Set max attempts to 2. Call `runFunctionalReviewCycle()`.
- Returns `(StatusNeedsHumanReview, false, nil)`

### Drift during review
Stub `checkDriftAndClose` to return an error after review. Call `runFunctionalReviewCycle()`.
- Returns `(StatusFailed, false, driftErr)`

### ProcessIssue is shorter
Read `ProcessIssue` in `loop.go`.
- The main function body is under 100 lines
- Functional review logic is a single call to `runFunctionalReviewCycle`
