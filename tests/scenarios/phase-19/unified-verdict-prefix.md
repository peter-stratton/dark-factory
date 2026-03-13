# Scenario: Unified AGENT_RESULT verdict prefix in prompts

Relates to: Issue #405

## Setup
- Prompt files `reviewer.txt` and `quality_reviewer.txt` have been updated
- Parser wrappers call `ParseVerdict` with `"AGENT"` keyword

## Cases

### Reviewer prompt uses AGENT_RESULT
Read `prompts/reviewer.txt`.
- Contains `AGENT_RESULT=APPROVED`
- Contains `AGENT_RESULT=CHANGES_REQUESTED`
- Does not contain `REVIEW_RESULT=`

### Quality reviewer prompt uses AGENT_RESULT
Read `prompts/quality_reviewer.txt`.
- Contains `AGENT_RESULT=APPROVED`
- Contains `AGENT_RESULT=CHANGES_REQUESTED`
- Does not contain `QUALITY_RESULT=`

### ParseReviewResult parses new format
Call `ParseReviewResult("AGENT_RESULT=APPROVED\n")`.
- Returns `"APPROVED"`

### ParseQualityResult parses new format
Call `ParseQualityResult("AGENT_RESULT=CHANGES_REQUESTED\n")`.
- Returns `"CHANGES_REQUESTED"`

### Old format no longer matched
Call `ParseReviewResult("REVIEW_RESULT=APPROVED\n")`.
- Returns `""` (old prefix is not matched)
