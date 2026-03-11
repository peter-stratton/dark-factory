# Scenario: Planner run data and dashboard

Relates to: Issue #348

## Setup
- The `internal/rundata/` package with a temp directory for writing run data
- The `internal/agent/` package with a mock `RunDataHook` implementation
- A `StepResult` populated with planner output, cost, and duration
- The dashboard templates with an `IssueDetail` containing `PlanResult`

## Cases

### Write plan result
Call `WritePlanResult(42, step)` on the run data writer.
- File `issues/42/plan.json` is created in the run directory
- File contains valid JSON with session_id, cost_usd, and result fields

### Read plan result when present
Write a `plan.json` file to `issues/42/` and call `ReadIssueDetail`.
- `IssueDetail.PlanResult` is non-nil
- `PlanResult.CostUSD` matches the written value
- `PlanResult.Result` contains the plan text

### Read issue detail without plan
Call `ReadIssueDetail` on an issue directory with no `plan.json`.
- `IssueDetail.PlanResult` is nil
- No error is returned
- Other fields (implement result, review result) load normally

### Hook called after planner step
Process an issue with planning enabled using a mock hook.
- `hook.WritePlanResult` is called once with the correct issue number
- The step result contains the planner's `ResultText` and cost

### Hook not called when planner skipped
Process an issue with `Planning.Enabled: false` using a mock hook.
- `hook.WritePlanResult` is never called

### Dashboard renders plan section
Render the issue detail template with `PlanResult` set.
- Output contains "Implementation Plan" text
- Output contains the plan result text content
- Output contains the cost and duration values

### Dashboard hides plan section when absent
Render the issue detail template with `PlanResult` as nil.
- Output does not contain "Implementation Plan" text
