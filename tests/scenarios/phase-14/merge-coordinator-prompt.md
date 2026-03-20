# Scenario: merge coordinator prompt template

Relates to: Issue #596

## Setup
- `prompts/merge_coordinator.txt` template file
- `MergeCoordinator` field in config `Prompts` struct
- `LoadPrompts()` function in `internal/agent/prompt.go`

## Cases

### Template loads with default path
Call `LoadPrompts` with default config paths.
- Merge coordinator prompt is loaded without error
- Prompt content is non-empty

### Template renders with branch context
Render the template with `PromptData` containing branch name, base branch, and conflict description.
- Rendered output contains the branch name
- Rendered output contains the base branch name
- Rendered output contains conflict resolution instructions

### Custom path overrides default
Set `prompts.merge_coordinator: "custom/merge.txt"` in config with a valid file at that path.
- `LoadPrompts` loads from the custom path
- Prompt content matches the custom file

### Embedded default used when no custom path
Leave `prompts.merge_coordinator` empty in config.
- `LoadPrompts` loads the embedded default from `prompts/merge_coordinator.txt`
- No error returned
