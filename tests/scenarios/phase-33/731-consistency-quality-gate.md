# Scenario: Semi-formal consistency quality gate

Relates to: Issue #731

## Setup
- `CheckSemiformalConsistency` exists in `internal/quality/quality.go`
- The function takes a reviewer output string and returns `*Flag` or nil
- `computeReviewFlags` in `loop.go` calls the check for semiformal reviews
- `runFunctionalReviewCycle` checks for the `semiformal_inconsistency` flag and triggers a re-run when found

## Cases

### No formal conclusion in output
- GIVEN reviewer output that does not contain "FORMAL CONCLUSION"
- WHEN `CheckSemiformalConsistency` is called
- THEN it returns nil (not a semiformal review, nothing to check)

### Clean approval with no contradictions
- GIVEN reviewer output containing FORMAL CONCLUSION with all ACs marked SATISFIED, no RTs marked BROKEN, no HIGH-risk uncovered paths, and `AGENT_RESULT=APPROVED`
- WHEN `CheckSemiformalConsistency` is called
- THEN it returns nil

### NOT SATISFIED contradicts APPROVED verdict
- GIVEN reviewer output containing an acceptance trace with "Verdict: NOT SATISFIED" and `AGENT_RESULT=APPROVED`
- WHEN `CheckSemiformalConsistency` is called
- THEN it returns a `Flag` with code `semiformal_inconsistency`
- THEN the flag message mentions "NOT SATISFIED"

### BROKEN regression contradicts APPROVED verdict
- GIVEN reviewer output containing a regression trace with "Status: BROKEN" and `AGENT_RESULT=APPROVED`
- WHEN `CheckSemiformalConsistency` is called
- THEN it returns a `Flag` with code `semiformal_inconsistency`

### HIGH risk uncovered path contradicts APPROVED verdict
- GIVEN reviewer output containing an uncovered path with "Risk: HIGH" and `AGENT_RESULT=APPROVED`
- WHEN `CheckSemiformalConsistency` is called
- THEN it returns a `Flag` with code `semiformal_inconsistency`

### NOT SATISFIED with CHANGES_REQUESTED is consistent
- GIVEN reviewer output containing "NOT SATISFIED" and `AGENT_RESULT=CHANGES_REQUESTED`
- WHEN `CheckSemiformalConsistency` is called
- THEN it returns nil (verdict matches traces, no contradiction)

### Inconsistency triggers re-run in functional review cycle
- GIVEN a functional review result with the `semiformal_inconsistency` flag
- WHEN `runFunctionalReviewCycle` evaluates the flags
- THEN it logs a warning, deletes the stale review comment, and continues the retry loop instead of returning approved
