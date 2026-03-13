# Scenario: Recon agent prompt template and role permissions

Relates to: Issue #367

## Setup
- The `internal/agent/runner/agent_runner.py` file contains `_ROLE_PERMISSIONS`
- The `prompts/recon.txt` template is loaded via `RenderPrompt()`
- Standard `PromptData` with `IssueNumber`, `IssueTitle`, `IssueBody` populated

## Cases

### Recon role allows read-only tools
Look up `recon` in `_ROLE_PERMISSIONS`.
- `allowed_tools` contains `Read`, `Glob`, `Grep`
- `disallowed_tools` contains `Write`, `Edit`, `Bash`

### Recon role rejects write operations
Invoke the runner with role `recon` and attempt a `Write` tool call.
- The tool call is blocked by the permission check
- An error message is returned to the agent explaining the restriction

### Recon role rejects bash operations
Invoke the runner with role `recon` and attempt a `Bash` tool call.
- The tool call is blocked by the permission check

### Prompt renders with issue context
Render `recon.txt` with a `PromptData` containing `IssueTitle: "Add widget"` and `IssueBody: "Build a widget package"`.
- The rendered prompt contains "Add widget"
- The rendered prompt contains "Build a widget package"

### Prompt renders with empty optional fields
Render `recon.txt` with a `PromptData` where `ArchitectureDocContent` and `ConventionsDocContent` are empty strings.
- The rendered prompt does not error
- Architecture and conventions sections are omitted or empty
