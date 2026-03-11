# Scenario: PR base branch targeting in prompts

Relates to: Issue #312

## Setup
- The `internal/agent/` package prompt rendering with `PromptData`
- Implementer and spec generator prompt templates
- `PromptData` with `BaseBranch` field populated from config

## Cases

### PromptData has BaseBranch field
Construct a `PromptData` with `BaseBranch: "feature/foo"`.
- The struct accepts and stores the value

### Implementer prompt includes --base when set
Render the implementer prompt with `BaseBranch: "feature/foo"`.
- Rendered output contains `--base feature/foo` in the `gh pr create` instruction

### Implementer prompt omits --base when empty
Render the implementer prompt with `BaseBranch: ""`.
- Rendered output does not contain `--base` in the `gh pr create` instruction

### Branch creation bases off configured branch
Render the implementer prompt with `BaseBranch: "feature/foo"` and `BranchExists: false`.
- Rendered output contains `origin/feature/foo` in the branch creation instruction

### Branch creation unchanged when empty
Render the implementer prompt with `BaseBranch: ""` and `BranchExists: false`.
- Rendered output contains `git checkout -b` without an `origin/` base reference

### Spec generator branches off configured branch
Render the spec generator prompt with `BaseBranch: "feature/foo"`.
- Rendered output contains `origin/feature/foo` in the branch creation instruction

### Spec generator unchanged when empty
Render the spec generator prompt with `BaseBranch: ""`.
- Rendered output does not contain `origin/` base reference in branch creation
