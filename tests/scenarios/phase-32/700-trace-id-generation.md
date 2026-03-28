# Scenario: Trace ID generation and stamping in ProcessIssue

Relates to: Issue #700

## Setup
- A stubbed `SandboxRunner` that returns successful agent results for all roles
- A `captureRunDataHook` that records all `StepResult`, `VerifyStepResult`, and `Outcome` values passed to `Write*` methods
- A `ProcessIssue` call with valid config, prompts (including planner and spec generator), and the capture hook

## Cases

### TraceID stamped on all step results
- GIVEN a `captureRunDataHook` is provided to `ProcessIssue`
- WHEN `ProcessIssue` completes successfully
- THEN every captured `StepResult` has a non-empty `TraceID` field
- THEN all captured `StepResult` values share the same `TraceID`

### TraceID on outcome matches steps
- GIVEN a `captureRunDataHook` is provided to `ProcessIssue`
- WHEN `ProcessIssue` completes and the hook captures the `Outcome`
- THEN `Outcome.TraceID` equals the `TraceID` on the captured step results

### TraceID is valid UUID v4 format
- GIVEN `ProcessIssue` generates a trace ID at startup
- WHEN the trace ID is inspected
- THEN it matches the UUID v4 pattern `xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`

### TraceID on IssueOutcome return value
- GIVEN `ProcessIssue` is called
- WHEN it returns an `IssueOutcome`
- THEN `IssueOutcome.TraceID` is non-empty and matches the trace ID on the captured hook data

### TraceID on verify results
- GIVEN config has verify enabled with lint and test checks
- WHEN `ProcessIssue` runs through the verify phase
- THEN all captured `VerifyStepResult` values have `TraceID` set to the same value as the step results

### Each ProcessIssue call gets a unique TraceID
- GIVEN two separate `ProcessIssue` calls for different issues
- WHEN both complete
- THEN the `IssueOutcome.TraceID` values are different

### TraceID field serialized to JSON
- GIVEN a `StepResult` with `TraceID` set to "abc-123"
- WHEN the struct is marshaled to JSON
- THEN the output contains `"trace_id":"abc-123"`

### TraceID omitted from JSON when empty
- GIVEN a `StepResult` with `TraceID` set to ""
- WHEN the struct is marshaled to JSON
- THEN the output does not contain a `trace_id` key (omitempty)
