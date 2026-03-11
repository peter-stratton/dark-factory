# Scenario: Wire planner into ProcessIssue

Relates to: Issue #347

## Setup
- The `internal/agent/` package with mock agent functions (`Plan`, `Implement`,
  `Review`) replaced via function variables or test seams
- Config with `Planning.Enabled: true` and verify/review configured
- Mock `Plan()` returns a result with `ResultText: "## Files to change\n- foo.go"`
- Mock `Implement()` captures the `planOutput` parameter it receives
- No real containers, GitHub API calls, or agent invocations

## Cases

### Plan runs before implement
Process an issue with planning enabled and a planner prompt loaded.
- `Plan()` is called before `Implement()`
- `Implement()` receives the plan text from `Plan().ResultText` as its
  `planOutput` parameter

### Plan output appears in implementer prompt
Render the implementer prompt with `PlanOutput` set to a plan string.
- Rendered prompt contains "## Implementation Plan" header
- Rendered prompt contains the plan text

### Plan output absent when empty
Render the implementer prompt with `PlanOutput` set to empty string.
- Rendered prompt does not contain "## Implementation Plan" header

### Planning disabled skips planner
Process an issue with `Planning.Enabled: false`.
- `Plan()` is never called
- `Implement()` receives empty string for `planOutput`
- Implementation proceeds normally

### Empty planner prompt skips planner
Process an issue with `Planning.Enabled: true` but `Prompts.Planner` is empty.
- `Plan()` is never called
- `Implement()` receives empty string for `planOutput`

### Plan failure is non-fatal
Process an issue where `Plan()` returns an error.
- A warning is logged mentioning the plan failure
- `Implement()` is still called with empty `planOutput`
- The issue is not marked as failed

### Plan timeout is non-fatal
Process an issue where `Plan()` returns `Result{TimedOut: true}`.
- A warning is logged mentioning the plan timeout
- `Implement()` is still called with empty `planOutput`
- The issue is not marked as failed
