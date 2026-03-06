# Scenario: Architecture JSON context for reviewer

Relates to: Issue #149

## Setup
- `internal/config/config.go` provides the `Config` struct
- `internal/agent/prompt.go` provides the `PromptData` struct
- `internal/agent/implementer.go` provides `newPromptData()`
- `prompts/reviewer.txt` and embedded
  `internal/harness/templates/prompts/reviewer.txt` are the reviewer templates
- Test fixtures: a temporary directory with optional `docs/architecture.json`

## Cases

### Config default path
Create a new `Config` with no YAML overrides.
- `ArchitectureJSON` field equals `"docs/architecture.json"`

### PromptData populated when file exists
Create a temp file at the configured `architecture_json` path with known JSON
content. Call `newPromptData()`.
- `PromptData.ArchitectureJSON` contains the file contents

### PromptData empty when file missing
Call `newPromptData()` with a config pointing to a non-existent
`architecture_json` path.
- `PromptData.ArchitectureJSON` is an empty string
- No error is returned

### Reviewer prompt includes compliance block when JSON exists
Render the reviewer prompt template with `ArchitectureJSON` set to valid JSON
layer definitions.
- The rendered prompt contains the JSON layer definitions
- The rendered prompt contains instructions to check imports against
  `may_depend_on` / `must_not_depend_on` rules

### Reviewer prompt omits compliance block when JSON absent
Render the reviewer prompt template with `ArchitectureJSON` set to empty
string.
- The rendered prompt does not contain layer compliance instructions

### Embedded template matches project template
Read `prompts/reviewer.txt` from the project root and the embedded template.
- The content is identical
