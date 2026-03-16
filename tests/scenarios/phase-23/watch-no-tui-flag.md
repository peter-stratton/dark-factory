# Scenario: Add --no-tui flag to godark watch

Relates to: Issue #517

## Setup
- `internal/cmd/watch.go` with `--no-tui` flag registered
- Terminal detection via `term.IsTerminal()`
- `logging.NewLoggerFileOnly()` for TUI mode, `logging.NewLogger()` for text mode

## Cases

### Flag is registered
Run `godark watch --help`.
- Output includes `--no-tui` flag description

### Interactive terminal selects file-only logger
Run `godark watch` with stdout connected to a terminal and `--no-tui` not set.
- Logger created via `NewLoggerFileOnly()` (file only, no stdout)

### No-tui forces standard logger
Run `godark watch --no-tui` with stdout connected to a terminal.
- Logger created via `NewLogger()` (file + stdout)

### Piped stdout uses standard logger
Run `godark watch | cat` (stdout is not a terminal).
- Logger created via `NewLogger()` regardless of `--no-tui`

### Watch functionality unchanged
Run `godark watch --no-tui` with stubbed GitHub responses.
- Polling loop works identically to before the flag was added
