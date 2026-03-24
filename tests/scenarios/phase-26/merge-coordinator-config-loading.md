# Scenario: Merge coordinator config and prompt loading

Relates to: Issue #606

## Setup
- `godark.yaml` exists with a `prompts:` section
- `prompts/merge_coordinator.txt` exists as the embedded default template
- `internal/agent/prompt.go` contains `LoadPrompts()` and `PromptData`

## Cases

### Config accepts merge_coordinator prompt path
Parse a `godark.yaml` containing:
```yaml
prompts:
  merge_coordinator: custom/merge.txt
```
- `cfg.Prompts.MergeCoordinator` equals `"custom/merge.txt"`

### LoadPrompts loads merge coordinator template
Call `LoadPrompts()` with default config (no custom path set).
- The returned `Prompts.MergeCoordinator` is a non-empty string
- No error is returned

### ConflictInfo field renders in template
Create a `PromptData` with `ConflictInfo: "CONFLICT (content): merge conflict in file.go"`.
Render a template containing `{{.ConflictInfo}}`.
- The rendered output contains "CONFLICT (content): merge conflict in file.go"

### ConflictInfo defaults to empty
Call `newPromptData()` with standard issue and config inputs.
- `data.ConflictInfo` is an empty string
