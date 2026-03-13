# Scenario: Adopt CommandRunnerFunc in infrastructure and foundation

Relates to: Issue #401

## Setup
- The `internal/exec` package provides `CommandRunnerFunc`
- The infrastructure packages have been updated: `github/issues.go`,
  `sandbox/build.go`, `config/config.go`

## Cases

### GitHub CommandRunner typed
Read `internal/github/issues.go`.
- `var CommandRunner` declaration includes `exec.CommandRunnerFunc` type

### Sandbox CommandRunner typed
Read `internal/sandbox/build.go`.
- `var CommandRunner` declaration includes `exec.CommandRunnerFunc` type

### Config CommandRunner typed
Read `internal/config/config.go`.
- `var CommandRunner` declaration includes `exec.CommandRunnerFunc` type

### Existing test fakes still assignable
Run `go test ./internal/github/... ./internal/sandbox/... ./internal/config/...`.
- All tests pass without modification to test fake assignments
