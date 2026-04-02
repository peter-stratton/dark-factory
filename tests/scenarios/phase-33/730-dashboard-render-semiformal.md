# Scenario: Dashboard renders semi-formal review analysis

Relates to: Issue #730

## Setup
- The review chain view in the dashboard renders `StepResult.Output` as part of the timeline
- A `StepResult` contains reviewer output with semi-formal analysis sections (PREMISES, ACCEPTANCE TRACE, REGRESSION TRACE, UNCOVERED PATHS, FORMAL CONCLUSION)
- The existing template in `issue-detail.html` renders step output and quality flags

## Cases

### Semiformal output renders without errors
- GIVEN a `StepResult` with `Output` containing all five semi-formal sections (PREMISES, ACCEPTANCE TRACE, REGRESSION TRACE, UNCOVERED PATHS, FORMAL CONCLUSION) with sample trace data and `AGENT_RESULT=APPROVED`
- WHEN the review chain template renders this step
- THEN no template errors occur
- THEN the section headers "PREMISES", "ACCEPTANCE TRACE", "REGRESSION TRACE", "UNCOVERED PATHS", and "FORMAL CONCLUSION" appear in the rendered HTML output

### Semiformal output with quality flags renders correctly
- GIVEN a `StepResult` with semiformal output AND quality flags including one with code `semiformal_inconsistency`
- WHEN the review chain template renders this step
- THEN both the quality flags and the semi-formal analysis sections are visible in the rendered output
- THEN the `semiformal_inconsistency` flag displays its code and message in the flags area
