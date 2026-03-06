# Scenario: Configurable architecture enforcement

Relates to: Issue #152

## Setup
- `internal/config/config.go` provides the `Config` struct
- `internal/agent/prompt.go` provides the `PromptData` struct
- `prompts/reviewer.txt` is the reviewer template with architecture
  enforcement conditionals
- No external services required

## Cases

### Default enforcement is off
Create a new `Config` with no YAML overrides.
- `EnforceArchitecture` field is `false`

### Config override enables enforcement
Parse a YAML config with `enforce_architecture: true`.
- `EnforceArchitecture` field is `true`

### Blocking directive when enforcement on and JSON present
Render the reviewer prompt with `EnforceArchitecture: true` and
`ArchitectureJSON` set to valid JSON.
- The rendered prompt contains "MUST result in CHANGES_REQUESTED"
- The rendered prompt contains instruction to not approve PRs with layer
  violations

### Informational directive when enforcement off and JSON present
Render the reviewer prompt with `EnforceArchitecture: false` and
`ArchitectureJSON` set to valid JSON.
- The rendered prompt contains "do not block approval"
- The rendered prompt contains instruction to flag violations in Review Notes

### No directive when JSON absent
Render the reviewer prompt with `ArchitectureJSON` set to empty string
(regardless of `EnforceArchitecture` value).
- The rendered prompt does not contain any layer enforcement directive
- Neither "MUST result in CHANGES_REQUESTED" nor "do not block approval"
  appears
