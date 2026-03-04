# Scenario: Role-scoped permissions

Relates to: Issue #37

## Setup
- The agent package (`internal/agent`) is imported directly
- The `Runner` variable in `launcher.go` is stubbed to capture environment variables passed to subprocess invocations
- `agent_runner.py` role-to-permission mapping is tested via Python unit tests (no real SDK or Claude API calls)
- No real Docker daemon or agent execution required

## Cases

### Implement sets implementer role
Call `Implement()` with a stubbed runner.
- `GODARK_ROLE` is set to `implementer` in the subprocess environment

### Retry sets implementer_retry role
Call `Retry()` with a stubbed runner.
- `GODARK_ROLE` is set to `implementer_retry` in the subprocess environment

### Review sets reviewer role
Call `Review()` with a stubbed runner.
- `GODARK_ROLE` is set to `reviewer` in the subprocess environment

### GenerateSpec sets spec_generator role
Call `GenerateSpec()` with a stubbed runner.
- `GODARK_ROLE` is set to `spec_generator` in the subprocess environment

### Implementer has full read-write-execute tools
Inspect the permission config for the `implementer` role in `agent_runner.py`.
- `allowed_tools` includes `Read`, `Write`, `Edit`, `Bash`, `Glob`, `Grep`
- `disallowed_tools` is empty

### Implementer retry has same tools as implementer
Inspect the permission config for the `implementer_retry` role in `agent_runner.py`.
- `allowed_tools` matches the implementer role exactly
- `disallowed_tools` is empty

### Reviewer cannot modify files
Inspect the permission config for the `reviewer` role in `agent_runner.py`.
- `allowed_tools` includes `Read`, `Glob`, `Grep`, `Bash`
- `disallowed_tools` includes `Write` and `Edit`
- `Write` and `Edit` are hard-denied regardless of other settings

### Reviewer can run commands
Inspect the permission config for the `reviewer` role in `agent_runner.py`.
- `allowed_tools` includes `Bash` (needed for `go test`, `gh pr diff`, etc.)

### Spec generator cannot run commands
Inspect the permission config for the `spec_generator` role in `agent_runner.py`.
- `allowed_tools` includes `Read`, `Write`, `Glob`, `Grep`
- `disallowed_tools` includes `Bash`
- `Bash` is hard-denied regardless of other settings

### Unknown role causes exit with error
Set `GODARK_ROLE` to `bogus` and invoke `agent_runner.py`.
- The process exits with a non-zero exit code
- Stderr or stdout contains a descriptive error message mentioning the invalid role
