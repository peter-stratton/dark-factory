# Scenario: Single-source scenario spec format

Relates to: Issue #392

## Setup
- The `internal/skills/godark-create-scenarios/SKILL.md` file has been updated
- The `prompts/spec_generator.txt` file is unchanged

## Cases

### SKILL.md references prompt as source
Read `internal/skills/godark-create-scenarios/SKILL.md`.
- Contains a reference to `prompts/spec_generator.txt`
- Does not contain the full duplicated format example block

### SKILL.md still self-contained
Read `internal/skills/godark-create-scenarios/SKILL.md`.
- Still describes the general structure (Scenario title, Setup, Cases)
- A user can understand the format without reading the prompt file

### Spec generator prompt unchanged
Read `prompts/spec_generator.txt`.
- Contains the full format specification with markdown example block
- Content is identical to the version before this change

### Existing skills tests pass
Run `go test ./internal/skills/...`.
- All tests pass
