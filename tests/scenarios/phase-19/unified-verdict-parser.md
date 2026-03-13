# Scenario: Unified verdict parser

Relates to: Issue #390

## Setup
- The `internal/agent/verdict.go` file contains `ParseVerdict(stdout, keyword)`
- `ParseReviewResult` and `ParseQualityResult` are thin wrappers

## Cases

### Approved with REVIEW keyword
Call `ParseVerdict("REVIEW_RESULT=APPROVED\n", "REVIEW")`.
- Returns `"APPROVED"`

### Changes requested with QUALITY keyword
Call `ParseVerdict("QUALITY_RESULT=CHANGES_REQUESTED\n", "QUALITY")`.
- Returns `"CHANGES_REQUESTED"`

### No match returns empty
Call `ParseVerdict("some unrelated output", "REVIEW")`.
- Returns `""`

### First match wins
Call `ParseVerdict("REVIEW_RESULT=APPROVED\nREVIEW_RESULT=CHANGES_REQUESTED\n", "REVIEW")`.
- Returns `"APPROVED"`

### Case insensitive
Call `ParseVerdict("review_result=approved\n", "REVIEW")`.
- Returns `"APPROVED"`

### Colon format accepted
Call `ParseVerdict("REVIEW_RESULT: APPROVED\n", "REVIEW")`.
- Returns `"APPROVED"`

### ParseReviewResult delegates
Call `ParseReviewResult("REVIEW_RESULT=APPROVED\n")`.
- Returns `"APPROVED"` (delegates to `ParseVerdict` with `"REVIEW"`)

### ParseQualityResult delegates
Call `ParseQualityResult("QUALITY_RESULT=CHANGES_REQUESTED\n")`.
- Returns `"CHANGES_REQUESTED"` (delegates to `ParseVerdict` with `"QUALITY"`)
