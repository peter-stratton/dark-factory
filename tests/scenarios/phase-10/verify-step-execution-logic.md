# Scenario: Verify step execution logic

Relates to: Issue #177

## Setup
- The `internal/agent/` package is tested via Go unit tests
- A mock `CommandRunner` function simulates shell command execution
- No external services or containers required

## Cases

### All checks pass
Call `RunVerify` with three checks (build, lint, test) where the mock runner
returns exit code 0 for all.
- `VerifyResult.AllPassed` is true
- `VerifyResult.Checks` has three entries, all with `Passed: true`

### Build fails stops execution
Call `RunVerify` with three checks where the build command exits 1.
- `VerifyResult.AllPassed` is false
- `VerifyResult.Checks` has one entry (build only) with `Passed: false`
- Lint and test checks are not executed

### Lint fails after build passes
Call `RunVerify` with three checks where build exits 0 and lint exits 1.
- `VerifyResult.AllPassed` is false
- `VerifyResult.Checks` has two entries: build (`Passed: true`) and lint (`Passed: false`)
- Test check is not executed

### Empty command is skipped
Call `RunVerify` with a check where `Command` is an empty string.
- The check is omitted from `VerifyResult.Checks`
- It is not counted as a failure

### All commands empty
Call `RunVerify` with three checks that all have empty `Command` strings.
- `VerifyResult.AllPassed` is true
- `VerifyResult.Checks` is an empty slice

### Output truncation
Call `RunVerify` with a check that produces 10 KB of combined stdout+stderr.
- The `Output` field in the result contains at most 4096 bytes
- The truncated output is the tail (last 4096 bytes) of the full output

### Context cancellation
Call `RunVerify` with a cancelled context.
- Execution stops
- Results for any checks completed before cancellation are returned
