# Scenario: Prompt templates reference configured base branch

Relates to: Issue #313

## Setup
- Implementer and spec generator prompt templates
- `PromptData` with `BaseBranch` field (added in prior issue)

## Cases

### Implementer warns against committing to configured branch
Render the implementer prompt with `BaseBranch: "feature/foo"`.
- Rendered output contains "Never commit directly to feature/foo"
- Rendered output does not contain "Never commit directly to main"

### Implementer defaults to main when empty
Render the implementer prompt with `BaseBranch: ""`.
- Rendered output contains "Never commit directly to main"

### Spec generator warns against committing to configured branch
Render the spec generator prompt with `BaseBranch: "feature/foo"`.
- Rendered output contains "Never commit directly to feature/foo"
- Rendered output does not contain "Never commit directly to main"

### Spec generator defaults to main when empty
Render the spec generator prompt with `BaseBranch: ""`.
- Rendered output contains "Never commit directly to main"
