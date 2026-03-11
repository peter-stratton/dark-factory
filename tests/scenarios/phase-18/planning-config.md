# Scenario: Planning config and prompt template

Relates to: Issue #343

## Setup
- The `internal/config/` package is tested via Go unit tests
- YAML config files with various `planning:` block values
- The `internal/agent/` package for `Prompts` and `PromptData` testing
- A minimal `prompts/planner.txt` embedded via `prompts/embed.go`

## Cases

### Config defaults
Parse a minimal `godark.yaml` with only `repo:` set.
- `Config.Planning.Enabled` is `true`
- `Config.Planning.FreshRestartAfter` is `2`

### Disable planning via YAML
Parse a `godark.yaml` with `planning: {enabled: false}`.
- `Config.Planning.Enabled` is `false`
- `Config.Planning.FreshRestartAfter` is still `2` (default)

### Override FreshRestartAfter
Parse a `godark.yaml` with `planning: {fresh_restart_after: 0}`.
- `Config.Planning.FreshRestartAfter` is `0`
- No validation error is returned (0 means never escalate)

### Planner prompt loads from embedded default
Call `LoadPrompts` with no custom planner path configured.
- `Prompts.Planner` is a non-empty string
- Content matches the embedded `planner.txt`

### Planner prompt loads from custom path
Set `prompts: {planner: "custom/plan.txt"}` in config and call `LoadPrompts`.
- `Prompts.Planner` contains the content of the custom file

### PlanOutput in PromptData
Render the planner template with a populated `PromptData`.
- Rendered output contains the issue title and issue body
- Rendered output contains architecture doc content when provided
- Rendered output is empty-safe when architecture fields are empty

### PromptData PlanOutput field exists
Construct a `PromptData` with `PlanOutput: "some plan text"`.
- The field is accessible and holds the assigned value
