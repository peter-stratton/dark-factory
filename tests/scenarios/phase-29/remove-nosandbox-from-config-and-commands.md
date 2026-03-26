# Scenario: Remove NoSandbox from config, commands, orchestrator, and doctor

Relates to: Issue #670

## Setup
- Issue #668 is complete: `Run()` no longer accepts `noSandbox` parameter
- `NoSandbox` field exists in `Config` and `CLIFlags` structs
- `--no-sandbox` flag is registered on `run`, `implement`, and `doctor` commands
- Orchestrator guards `BuildImage` with `if !cfg.NoSandbox`
- Orchestrator guards `PullAfterMerge` with `if cfg.NoSandbox`
- `newHostRunner()` exists in `loop.go`
- Doctor has `NoSandbox` in `Opts` with conditional host toolchain checks

## Cases

### Config struct field removed
Parse a `Config` from YAML containing `no_sandbox: true`.
- No error (yaml.v3 ignores unknown fields in non-strict mode)
- The `Config` struct has no `NoSandbox` field

### CLIFlags field removed
Inspect `CLIFlags` struct in `internal/config/config.go`.
- No `NoSandbox` field exists

### Flag not registered on run command
Run `godark run --help` (or inspect flag definitions).
- `--no-sandbox` does not appear in the output

### Flag not registered on implement command
Run `godark implement --help` (or inspect flag definitions).
- `--no-sandbox` does not appear in the output

### Flag not registered on doctor command
Run `godark doctor --help` (or inspect flag definitions).
- `--no-sandbox` does not appear in the output

### BuildImage unconditional in orchestrator
Read `internal/orchestrator/orchestrator.go`.
- `BuildImage` calls are not wrapped in `if !cfg.NoSandbox` guards
- `BuildImage` is called unconditionally before processing issues

### PullAfterMerge removed from wave loop
Read `internal/orchestrator/orchestrator.go`.
- `PullAfterMerge` is not called from the `processIssues` wave loop
- The `if cfg.NoSandbox { PullAfterMerge(...) }` block does not exist

### newHostRunner deleted
Search `internal/agent/loop.go` for `newHostRunner`.
- The function does not exist
- No `if cfg.NoSandbox` blocks remain in `loop.go`

### Verify always uses sandbox runner
Read `internal/agent/verify.go`.
- No `if cfg.NoSandbox` block exists
- `sandboxCommandRunner()` is called unconditionally

### Doctor NoSandbox removed
Read `internal/doctor/doctor.go`.
- `Opts` struct has no `NoSandbox` field
- Docker daemon check is always included (no `if !opts.NoSandbox` guard)
- No host toolchain checks (golangci-lint, runtime) exist

### Build and targeted tests pass
Run `go build ./...` and
`go test ./internal/config/... ./internal/cmd/... ./internal/doctor/...`.
- Both complete with exit code 0
