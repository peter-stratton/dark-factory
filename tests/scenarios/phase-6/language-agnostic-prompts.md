# Scenario: Language-agnostic prompts

Relates to: Issue #51

## Setup
- The agent package (`internal/agent`) is imported directly
- Prompt templates are loaded via `LoadPrompts` with default (embedded) paths
- `RenderPrompt` is called with a populated `PromptData` struct
- No external services or network access required

## Cases

### Implementer prompt has no Go-specific conventions
Load and render the implementer prompt with sample `PromptData`.
- Rendered output does NOT contain `foo_test.go`
- Rendered output does NOT contain `foo.go`
- Rendered output contains `Write unit tests`
- Rendered output contains the configured `BuildCommand` value
- Rendered output contains the configured `TestCommand` value

### Reviewer prompt has no Go test references
Load and render the reviewer prompt with sample `PromptData`.
- Rendered output does NOT contain `go test`
- Rendered output does NOT contain `Go integration tests`
- Rendered output contains `Generate integration tests`
- Rendered output contains the configured `ReviewDir` value

### Reviewer prompt still uses template variables
Load and render the reviewer prompt with `BuildCommand: "flutter build"`,
`TestCommand: "flutter test"`, `ReviewDir: "tests/review/"`.
- Rendered output contains `flutter build`
- Rendered output contains `flutter test`
- Rendered output contains `tests/review/`

### Implementer retry prompt has no Go references
Load and render the implementer retry prompt with sample `PromptData`.
- Rendered output does NOT contain `go test`
- Rendered output does NOT contain `go build`
- Rendered output contains the configured `TestCommand` value
- Rendered output contains the configured `BuildCommand` value

### All prompts preserve critical template variables
Load and render each prompt (implementer, reviewer, retry) with `ProtectedPaths: "CLAUDE.md,tests/scenarios/"`.
- Each rendered output contains `CLAUDE.md,tests/scenarios/`
