# Scenario: Delete Python runner package and runHost function

Relates to: Issue #668

## Setup
- The `internal/agent/runner/` package currently contains `agent_runner.py`,
  `embed.go`, `embed_test.go`, and `test_hooks.py`
- `runHost()` in `internal/agent/launcher.go` is the only consumer of
  `runner.FS`
- `Run()` currently accepts a `noSandbox bool` parameter and dispatches to
  `runHost()` or `runSandbox()`
- 8 call sites across 6 agent files pass `cfg.NoSandbox` to `Run()`
- `internal/agent/launcher_test.go` contains 22 `TestRun_NoSandbox_*` tests

## Cases

### Runner package deleted
After the change:
- `internal/agent/runner/` directory does not exist
- No Go file in the repository imports `internal/agent/runner`

### runHost function removed
After the change:
- `runHost` does not appear in `internal/agent/launcher.go`
- `Runner` var, `writerFunc` type, and `goosForRusage` var do not exist in
  `internal/agent/launcher.go`

### Run() signature simplified
Inspect `Run()` in `internal/agent/launcher.go`.
- The function signature is `Run(ctx context.Context, opts RunOpts, logger *slog.Logger)`
- There is no `noSandbox` or `bool` parameter
- The function body calls `runSandbox()` unconditionally

### Agent callers updated
Inspect call sites in `implementer.go`, `punchlist.go`, `quality_reviewer.go`,
`recon.go`, `reviewer.go`, `specgen.go`.
- Each calls `Run(ctx, opts, logger)` with 3 arguments (no `cfg.NoSandbox`)

### Launcher tests removed
After the change:
- `internal/agent/launcher_test.go` does not exist
- No test function named `TestRun_NoSandbox_` exists in the codebase

### Architecture JSON updated
Read `docs/architecture.json`.
- The `infrastructure` layer paths do not include `internal/agent/runner/`

### Build and vet pass
Run `go build ./...` and `go vet ./...`.
- Both complete with exit code 0
