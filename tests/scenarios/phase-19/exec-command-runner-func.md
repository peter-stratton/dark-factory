# Scenario: CommandRunnerFunc shared type

Relates to: Issue #385

## Setup
- The `internal/exec` package exists with `CommandRunnerFunc` type and `Default` var

## Cases

### Default runs a command
Call `exec.Default("echo", "hello")`.
- Returns output containing "hello"
- Returns nil error

### Custom function assignable
Assign a function literal `func(name string, args ...string) ([]byte, error) { return nil, nil }` to a variable of type `exec.CommandRunnerFunc`.
- The assignment compiles without error

### Architecture updated
Read `docs/architecture.json`.
- The `foundation` layer's `paths` array includes `internal/exec/`
