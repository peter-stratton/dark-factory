# Scenario: Wire planner into ProcessIssue pipeline

Relates to: Issue #694

## Setup
- `SandboxRunner` is stubbed to return canned results per role
- `Prompts.Planner` is set to a valid template
- `ProcessIssue()` is called with a test issue, config, and stubbed runner
- A `testRunDataHook` captures write calls

## Cases

### Planner runs between recon and implement
- GIVEN `Prompts.Recon` and `Prompts.Planner` are both set
- WHEN `ProcessIssue()` is called
- THEN the stage progression is "recon" → "plan" → "implement"

### Planner brief passed to implementer
- GIVEN the planner runner returns "## Approach\n\nUse existing widget base class"
- WHEN the implementer prompt is rendered
- THEN the rendered prompt contains "## Plan" and "Use existing widget base class"

### Planner skipped when prompt empty
- GIVEN `Prompts.Planner` is an empty string
- WHEN `ProcessIssue()` is called
- THEN no "plan" stage is reported and the implementer runs directly after recon

### Planner failure is non-blocking
- GIVEN the stubbed runner returns an error for the "planner" role
- WHEN `ProcessIssue()` is called
- THEN a warning is logged containing "planner" and the implementer still runs with an empty PlannerBrief

### Planner timeout is non-blocking
- GIVEN the stubbed runner returns a result with `TimedOut: true` for the "planner" role
- WHEN `ProcessIssue()` is called
- THEN a warning is logged and the implementer still runs

### Implementer prompt without planner brief
- GIVEN `Prompts.Planner` is set but the planner fails
- WHEN the implementer prompt is rendered
- THEN the prompt does not contain "## Plan" (the conditional section is omitted)
