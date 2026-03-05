# Scenario: Migrate debug log to run directory

Relates to: Issue #97

## Setup
- The `logging` package accepts a directory path for the log file
- Tests create a temporary directory to simulate the run directory
- Config loading is tested with YAML strings (no file I/O)
- No Docker, GitHub API, or real agent invocations required

## Cases

### Debug log written to run directory
Create a `RunDataWriter`, then create a logger with `NewLogger(writer.Dir())`.
- A file named `debug.log` exists in the run directory
- Writing a log entry adds content to `debug.log`

### LogDir removed from config
Parse a YAML config that contains no `log_dir` key.
- Config loads without error
- The `Config` struct has no `LogDir` field

### Dry-run uses private temp directory
Run in dry-run mode (no `RunDataWriter` created).
- A logger is still created successfully
- The log path is a unique temporary directory (not shared with other runs)
- Two concurrent dry-runs have different log paths

### Logger writes structured JSON
Create a logger and write an info message with structured fields.
- The `debug.log` file contains valid JSON lines
- Each line has `time`, `level`, `msg` keys

### Old log_dir config key ignored
Parse a YAML config that still contains `log_dir: "logs/"`.
- Config loads without error (field is silently ignored or removed)
