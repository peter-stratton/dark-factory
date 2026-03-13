# Scenario: Shared critical rules template variable

Relates to: Issue #391

## Setup
- `PromptData` has a `SharedRules` field populated by `newPromptData()`
- All 5 prompt templates use `{{.SharedRules}}` for the shared subset

## Cases

### SharedRules includes protected paths
Create `PromptData` with `ProtectedPaths: "CLAUDE.md, tests/scenarios/"`.
- `SharedRules` contains `Do NOT modify any protected paths: CLAUDE.md, tests/scenarios/`

### SharedRules includes scenario dir
Create `PromptData` with `ScenarioDir: "tests/scenarios/"`.
- `SharedRules` contains `Do NOT modify files in tests/scenarios/`

### SharedRules empty when no paths
Create `PromptData` with empty `ProtectedPaths` and `ScenarioDir`.
- `SharedRules` is an empty string

### Implementer prompt has shared plus specific rules
Render `implementer.txt` with `SharedRules` populated and `GeneratedPaths` set.
- Output contains the shared rules text
- Output contains branch creation logic
- Output contains generated paths warning

### Spec generator keeps narrow ScenarioDir rule
Render `spec_generator.txt` with `SharedRules` populated.
- Output contains shared protected paths rule
- Output contains "existing files" ScenarioDir wording (agent-specific, not from SharedRules)

### Verify fix prompt uses shared rules
Render `verify_fix.txt` with `SharedRules` populated.
- Output contains the shared rules text
- Output contains "Push to the existing PR branch"
