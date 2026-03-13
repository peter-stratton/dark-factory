# Scenario: Recon config and prompt data wiring

Relates to: Issue #368

## Setup
- The `internal/config/` package is tested via Go unit tests
- The `internal/agent/prompt.go` file contains `PromptData` and `LoadPrompts()`
- The `prompts/implementer.txt` template includes a conditional `ReconBrief` block

## Cases

### Config parses recon prompt path
Parse a `godark.yaml` with `prompts: { recon: "custom/recon.txt" }`.
- `Config.Prompts.Recon` is `"custom/recon.txt"`

### Config default is empty
Parse a minimal `godark.yaml` without a `prompts.recon` field.
- `Config.Prompts.Recon` is an empty string

### LoadPrompts loads recon template
Call `LoadPrompts()` with a config where `Prompts.Recon` is empty and `prompts/recon.txt` exists as an embedded default.
- `Prompts.Recon` contains the embedded template text
- No error is returned

### LoadPrompts skips missing recon gracefully
Call `LoadPrompts()` with a config where `Prompts.Recon` points to a nonexistent file and no embedded default exists.
- `Prompts.Recon` is an empty string
- No error is returned (optional prompt pattern)

### Implementer prompt includes ReconBrief when set
Render `implementer.txt` with `ReconBrief` set to `"Files to change: foo.go, bar.go"`.
- The rendered prompt contains `"Files to change: foo.go, bar.go"`

### Implementer prompt omits ReconBrief when empty
Render `implementer.txt` with `ReconBrief` set to `""`.
- The rendered prompt does not contain a recon section or recon-related header
