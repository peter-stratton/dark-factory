# Scenario: Update docs and config schemas for sandbox-only mode

Relates to: Issue #669

## Setup
- `internal/harness/templates/godark.md` contains a config reference table
  with a `no_sandbox` row
- `.claude/godark.md` is the installed copy of the template
- `internal/skills/godark-configure-project/godark-config-schema.json` defines
  a `no_sandbox` property
- `.claude/skills/godark-configure-project/godark-config-schema.json` is the
  installed copy

## Cases

### godark.md template no_sandbox removed
Read `internal/harness/templates/godark.md`.
- The file does not contain `no_sandbox`
- The "Agent behavior" config table does not have a row for sandbox mode

### Installed godark.md updated
Read `.claude/godark.md`.
- The file does not contain `no_sandbox`

### Config schema no_sandbox removed
Read `internal/skills/godark-configure-project/godark-config-schema.json`.
- The JSON does not contain a `no_sandbox` property
- The JSON is valid (parses without errors)

### Installed schema updated
Read `.claude/skills/godark-configure-project/godark-config-schema.json`.
- The JSON does not contain a `no_sandbox` property

### Skills tests pass
Run `go test ./internal/skills/...`.
- Exits with code 0
