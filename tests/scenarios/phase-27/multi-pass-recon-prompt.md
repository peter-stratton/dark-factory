# Scenario: Multi-pass recon prompt with generalized language

Relates to: Issue #631

## Setup
- `prompts/recon.txt` is the prompt template under test
- `RenderPrompt()` in `internal/agent/prompt.go` renders the template
- Standard `PromptData` with `IssueNumber`, `IssueTitle`, `IssueBody` populated

## Cases

### Prompt contains three priority sections
Render `recon.txt` with standard `PromptData`.
- The rendered output contains a Priority 1 section header (file list and drift)
- The rendered output contains a Priority 2 section header (key signatures)
- The rendered output contains a Priority 3 section header (pattern example)

### Each section instructs agent to write before proceeding
Render `recon.txt` with standard `PromptData`.
- Priority 1 section includes an instruction to write findings before moving on
- Priority 2 section includes an instruction to write findings before moving on

### No Flutter or UI-specific language
Render `recon.txt` with standard `PromptData`.
- Output does not contain "list screen"
- Output does not contain "form screen"
- Output does not contain "app shell"
- Output does not contain "nav structure"

### Issue context is present in rendered output
Render `recon.txt` with `IssueTitle: "Add expense repo"` and
`IssueBody: "Create the expense repository"`.
- The rendered output contains "Add expense repo"
- The rendered output contains "Create the expense repository"

### Prompt instructs scoped verbatim code not full files
Render `recon.txt` with standard `PromptData`.
- Output contains instruction to quote relevant functions/types, not entire files
