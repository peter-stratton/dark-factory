# Scenario: Merge coordinator prompt template and role permissions

Relates to: Issue #605

## Setup
- The `internal/agent/runner/agent_runner.py` file contains `_ROLE_PERMISSIONS`
- The `prompts/merge_coordinator.txt` template is loaded via `RenderPrompt()`
- Standard `PromptData` with `IssueNumber`, `IssueTitle`, `IssueBody`,
  `BaseBranch`, `BuildCommand`, `TestCommand`, and `ConflictInfo` populated

## Cases

### Merge coordinator role allows edit and bash tools
Look up `merge_coordinator` in `_ROLE_PERMISSIONS`.
- `allowed_tools` contains `Read`, `Edit`, `Bash`, `Glob`, `Grep`
- `disallowed_tools` contains `Write`

### Merge coordinator role rejects file creation
Invoke the runner with role `merge_coordinator` and attempt a `Write` tool call.
- The tool call is blocked by the permission check
- An error message is returned to the agent explaining the restriction

### Prompt renders with conflict context
Render `merge_coordinator.txt` with a `PromptData` containing
`ConflictInfo: "CONFLICT in app/lib/main.dart"` and
`BaseBranch: "phase-5/base"`.
- The rendered prompt contains "CONFLICT in app/lib/main.dart"
- The rendered prompt contains "phase-5/base"

### Prompt renders with issue context
Render `merge_coordinator.txt` with `IssueTitle: "Add expense repo"` and
`IssueBody: "Create the expense repository interface"`.
- The rendered prompt contains "Add expense repo"
- The rendered prompt contains "Create the expense repository interface"

### Prompt renders with build and test commands
Render `merge_coordinator.txt` with `BuildCommand: "go build ./..."` and
`TestCommand: "go test ./..."`.
- The rendered prompt contains the build command
- The rendered prompt contains the test command
