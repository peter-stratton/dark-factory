# Scenario: Hybrid output mode and --no-tui flag

Relates to: Issue #445

## Setup
- `cmd/run.go` and `cmd/implement.go` have a `--no-tui` flag (bool, default false)
- Terminal detection uses `term.IsTerminal(int(os.Stdout.Fd()))`
- `logging.NewLoggerFileOnly()` creates a logger that writes JSON to file only
- The Bubble Tea program runs with `tea.WithAltScreen()`

## Cases

### Interactive terminal launches TUI mode
Run `godark run --milestone "Phase 20"` with stdout connected to a terminal and `--no-tui` not set.
- A Bubble Tea program is created with `tea.WithAltScreen()`
- A `TUIReporter` is created and passed to the orchestrator
- The logger uses file-only mode (no stdout text output)

### Piped stdout uses text reporter
Run `godark run --milestone "Phase 20" | cat` (stdout is not a terminal).
- A `TextReporter` is created and passed to the orchestrator
- No Bubble Tea program is created
- The logger writes to both file and stdout (current behavior)

### --no-tui flag forces text mode
Run `godark run --milestone "Phase 20" --no-tui` with stdout connected to a terminal.
- A `TextReporter` is created and passed to the orchestrator
- No Bubble Tea program is created
- The logger writes to both file and stdout (current behavior)

### Implement command has same --no-tui flag
Run `godark implement 42 --no-tui`.
- The `--no-tui` flag is recognized
- Text reporter is used

### Implement command detects terminal for TUI
Run `godark implement 42` with stdout connected to a terminal.
- A Bubble Tea program is created
- A `TUIReporter` is used

### TUI exits when orchestrator finishes
The orchestrator completes processing all issues.
- A quit message is sent to the Bubble Tea program
- `program.Run()` returns without error
- The terminal is restored to its normal state (alt screen exited)

### SIGINT shuts down gracefully
Send SIGINT (Ctrl-C) while the TUI is running.
- The orchestrator's context is cancelled
- The Bubble Tea program receives a quit signal
- Both shut down without panic
- The terminal is restored to its normal state

### Logger file-only mode writes no stdout
Create a logger with `NewLoggerFileOnly()` and emit a log message.
- `debug.log` in the specified directory contains the JSON log entry
- No text output appears on stdout

### Logger file-only mode still writes JSON
Create a logger with `NewLoggerFileOnly()` and emit an info-level message.
- `debug.log` contains a JSON line with the message and structured fields

### Orchestrator error propagated after TUI exit
The orchestrator returns an error (e.g., lock acquisition failure).
- The Bubble Tea program exits
- The error is returned to the caller (Cobra command RunE)
- The error message is displayed after the TUI exits
