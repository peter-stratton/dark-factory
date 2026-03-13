# Scenario: Doctor.go CommandRunnerFunc with external timeout

Relates to: Issue #403

## Setup
- The `internal/doctor` package with `CommandRunner`, `Check`, and `Run()`
- A fake `CommandRunner` that returns immediately or blocks indefinitely

## Cases

### CommandRunner uses shared type
Read `internal/doctor/doctor.go`.
- `var CommandRunner` is typed as `exec.CommandRunnerFunc` (no `context.Context` parameter)

### Check.run has no context parameter
Read `internal/doctor/doctor.go`.
- The `Check.run` field is `func() bool`, not `func(ctx context.Context) bool`

### All checks pass
Set `CommandRunner` to a fake that always succeeds. Call `Run()` with Docker, gh, and auth checks.
- Returns true
- Output contains `[PASS]` for each check

### Failed check reported
Set `CommandRunner` to a fake that fails for `docker info`. Call `Run()`.
- Returns false
- Output contains `[FAIL] Docker daemon running`
- Output contains `Fix:` hint

### Timeout reported as failure
Set `CommandRunner` to a fake that blocks for 30 seconds. Call `Run()`.
- The check is reported as `[FAIL]` within ~15 seconds (not 30)
- `Run()` does not hang

### Test fakes assignable without context
Assign a `func(string, ...string) ([]byte, error)` literal to `doctor.CommandRunner`.
- The assignment compiles
