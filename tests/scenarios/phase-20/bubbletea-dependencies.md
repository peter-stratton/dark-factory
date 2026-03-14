# Scenario: Bubble Tea dependencies and architecture registration

Relates to: Issue #439

## Setup
- The project `go.mod` is at the repository root
- `docs/architecture.json` defines the presentation layer with paths array
- `godark vet architecture` validates layer definitions and import rules

## Cases

### Go module includes Bubble Tea ecosystem
Read `go.mod` after dependency addition.
- `github.com/charmbracelet/bubbletea` appears in the require block
- `github.com/charmbracelet/lipgloss` appears in the require block
- `github.com/charmbracelet/bubbles` appears in the require block

### Project builds successfully
Run `go build ./...` from the repository root.
- The build exits with code 0
- No compilation errors related to the new dependencies

### Architecture JSON includes TUI path
Read `docs/architecture.json` and find the presentation layer entry.
- The `paths` array contains `"internal/tui/"`
- The `paths` array still contains `"internal/dashboard/"`

### TUI layer has same dependency rules as dashboard
Read the presentation layer entry in `docs/architecture.json`.
- `may_depend_on` includes `"domain"` and `"foundation"`
- `must_not_depend_on` includes `"cmd"`, `"orchestration"`, `"service"`, `"infrastructure"`, and `"content"`

### Architecture vet passes
Run `godark vet architecture`.
- No cycle violations reported
- No layer path violations reported
- Exit code 0
