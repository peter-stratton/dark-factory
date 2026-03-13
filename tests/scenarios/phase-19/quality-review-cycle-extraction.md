# Scenario: Extract quality review cycle function

Relates to: Issue #407

## Setup
- The `internal/agent/loop.go` file contains `runQualityReviewCycle()`
- Stub agent functions simulate quality review verdicts

## Cases

### Quality approved first try
Stub `QualityReview` to return `APPROVED`. Call `runQualityReviewCycle()`.
- Returns `(true, nil)`

### Quality approved after retry
Stub `QualityReview` to return `CHANGES_REQUESTED` then `APPROVED`. Call `runQualityReviewCycle()`.
- Returns `(true, nil)` after two iterations

### Quality exhausts retries
Stub `QualityReview` to always return `CHANGES_REQUESTED`. Set max attempts to 2. Call `runQualityReviewCycle()`.
- Returns `(false, nil)`

### Drift during quality review
Stub `checkDriftAndClose` to return an error. Call `runQualityReviewCycle()`.
- Returns `(false, driftErr)` where `driftErr` is non-nil

### ProcessIssue calls extracted function
Read `ProcessIssue` in `loop.go`.
- The quality review logic is a single call to `runQualityReviewCycle`, not an inline loop
