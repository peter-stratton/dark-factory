# Scenario: Wire verify step into agent loop

Relates to: Issue #181

## Setup
- The `internal/agent/` package with mock agent runners and hook implementations
- Mock `CommandRunner` for verify step execution
- Config with `BuildCommand`, `LintCommand`, `TestCommand`, and `Verify` block
- No real containers or GitHub API calls

## Cases

### All checks pass proceeds to review
Process an issue where verify checks (build, test) all pass.
- Verify step completes successfully
- Quality review is invoked after verify
- No fix cycle is triggered

### Build fails then fix succeeds
Process an issue where initial verify fails on build, then the fix attempt
makes verify pass.
- Implementer is invoked with verify fix prompt containing the build error
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

### No commands configured skips verify
Process an issue with empty `BuildCommand`, `LintCommand`, and `TestCommand`.
- Verify step is skipped entirely
- Quality review is invoked directly after guard rails

### Fix agent resumes session
Process an issue where verify fails and a fix cycle is triggered.
- The fix agent invocation includes the implementer's session ID
- The fix prompt contains the verify error summary

### Protected path drift re-checked after fix
Process an issue where the fix agent modifies a protected path.
- Protected path drift check runs after the fix attempt
- Issue is failed or flagged for the drift violation

### Verify runs between guard rails and quality review
Process an issue through the full pipeline.
- Guard rails (PR exists, Closes #N, protected paths) run before verify
- Verify runs after guard rails
- Quality review runs after verify passes
