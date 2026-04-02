## Phase 33: Semi-Structured Review

**Goal**: The functional reviewer produces auditable, structured reasoning (premises → traces → conclusion) that is machine-verifiable for consistency, reducing false approvals on subtle bugs.

**Milestone**: `Phase 33: Semi-Structured Review` | **Label**: `phase-33` | **Issues**: #728–#731

- Semi-formal reviewer prompt — new `prompts/reviewer_semiformal.txt` with PREMISES, ACCEPTANCE TRACE, REGRESSION TRACE, UNCOVERED PATHS, and FORMAL CONCLUSION sections
- Config toggle — add `review.semiformal` and `review.semiformal_on_retry` to config struct and godark.yaml, default both to `false`
- Prompt selection in orchestrator — select semiformal prompt in `loop.go` based on config and retry cycle
- Consistency quality gate — `CheckSemiformalConsistency` in `internal/quality/` that parses structured output and flags verdict/trace contradictions
- Wire consistency gate into review flags — add `semiformal_inconsistency` flag to `computeReviewFlags()`, trigger automatic re-run on inconsistency (same pattern as `no_review_tests_written`)
- Dashboard: render semi-formal analysis — display structured analysis sections in the review chain view
