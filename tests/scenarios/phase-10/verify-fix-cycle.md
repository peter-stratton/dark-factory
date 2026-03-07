# Scenario: Verify fix cycle

Relates to: Issue #200

## Setup
- The `internal/agent/` package with mock agent runners and hook implementations
- Mock `CommandRunner` that can be configured to fail then pass
- Config with verify commands and `Verify` block (`MaxFixAttempts`, `Blocking`)
- Mock `VerifyFix` function on the implementer
- No real containers or GitHub API calls

## Cases

### Build fails then fix succeeds
Process an issue where initial verify fails on build, then the fix attempt
makes verify pass.
- Implementer `VerifyFix` is invoked with verify fix prompt containing the build error
- Second verify attempt passes
- Quality review is invoked after successful verify

### Exhausted fix attempts with blocking mode
Process an issue with `Verify.Blocking: true` and `Verify.MaxFixAttempts: 2`
where verify fails on every attempt.
- Two fix attempts are made
- Issue outcome status is "failed"
- Quality review is never invoked

### Exhausted fix attempts with non-blocking mode
Process an issue with `Verify.Blocking: false` and `Verify.MaxFixAttempts: 1`
where verify fails on every attempt.
- One fix attempt is made
- A warning is logged
- Quality review is still invoked (verify failure does not block)

### Fix agent resumes session
Process an issue where verify fails and a fix cycle is triggered.
- The fix agent invocation includes the implementer's session ID
- The fix prompt contains the verify error summary

### Protected path drift re-checked after fix
Process an issue where the fix agent modifies a protected path.
- Protected path drift check runs after the fix attempt
- Issue is failed or flagged for the drift violation

### Fix prompt contains error output
Render the verify fix prompt with two failing checks.
- Rendered prompt includes each check name and its truncated output
- Rendered prompt does not instruct agent to re-run checks
