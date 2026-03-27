# Scenario: Planner run data, dashboard timeline, and TUI stage

Relates to: Issue #695

## Setup
- `ProcessIssue()` is called with planner enabled and a `testRunDataHook`
- Run data is written to a temporary directory
- Dashboard `buildTimeline()` is called with an `IssueDetail` struct

## Cases

### WritePlannerResult on RunDataHook interface
- GIVEN the `RunDataHook` interface definition
- WHEN inspected
- THEN it contains a `WritePlannerResult(issueNumber int, step rundata.StepResult) error` method

### Planner result written to run data
- GIVEN a successful planner run for issue 42
- WHEN `ProcessIssue()` completes
- THEN `issues/42/planner.json` exists in the run data directory

### Planner result contains expected fields
- GIVEN a successful planner run that returns a brief
- WHEN `planner.json` is read
- THEN it contains `session_id`, `duration_ms`, `cost_usd`, and `result_text` fields

### Dashboard timeline includes planner
- GIVEN an `IssueDetail` with a populated `Planner` field
- WHEN `buildTimeline()` is called
- THEN the timeline contains a step labeled "Planner" positioned after "Recon" and before "Implement"

### Dashboard timeline skips absent planner
- GIVEN an `IssueDetail` with an empty `Planner` field
- WHEN `buildTimeline()` is called
- THEN the timeline does not contain a "Planner" step and there is no gap between Recon and Implement

### TUI displays plan stage
- GIVEN a TUI model receiving stage change messages
- WHEN an `IssueStageChangedMsg` with stage "plan" is received
- THEN the issue row displays the "plan" stage

### Hook called with correct issue number
- GIVEN a `testRunDataHook` that records calls
- WHEN `ProcessIssue()` runs for issue 42 with planner enabled
- THEN `WritePlannerResult` was called with issue number 42
