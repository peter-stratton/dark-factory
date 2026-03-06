# Scenario: Populate harness template variables in launcher

Relates to: Issue #147

## Setup
- `internal/config/config.go` provides the `Config` struct
- `internal/agent/implementer.go` provides `newPromptData()`
- Test fixtures: a temporary directory with optional `docs/architecture.md` and
  `docs/conventions.md` files containing known content

## Cases

### Config defaults are set
Create a new `Config` with no YAML overrides.
- `ArchitectureDoc` field equals `"docs/architecture.md"`
- `ConventionsDoc` field equals `"docs/conventions.md"`

### Config override from YAML
Parse a YAML config with `architecture_doc: custom/arch.md`.
- `ArchitectureDoc` field equals `"custom/arch.md"`

### File contents populated when files exist
Create temp files at the configured paths with known content. Call
`newPromptData()` with a config pointing to those paths.
- `PromptData.ArchitectureDoc` contains the file contents
- `PromptData.ConventionsDoc` contains the file contents

### Empty string when files are missing
Call `newPromptData()` with a config pointing to non-existent paths.
- `PromptData.ArchitectureDoc` is an empty string
- `PromptData.ConventionsDoc` is an empty string
- No error is returned or logged

### Both fields populated together
Create both files with distinct content. Call `newPromptData()`.
- Both `ArchitectureDoc` and `ConventionsDoc` are non-empty
- Each contains its respective file's content
