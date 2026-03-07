# Scenario: Wire verify step into agent loop (host mode)

Relates to: Issue #199

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

### No commands configured skips verify
Process an issue with empty `BuildCommand`, `LintCommand`, and `TestCommand`.
- Verify step is skipped entirely
- Quality review is invoked directly after guard rails

### Host mode verify runs via sh -c
Process an issue with `NoSandbox: true` and `BuildCommand: "go build ./..."`.
- `CommandRunner` executes `sh -c "go build ./..."` via `GuardRunner`
- Stdout, stderr, and exit code are captured

### Blocking failure stops processing
Process an issue with `Verify.Blocking: true` where build fails.
- Issue outcome status is "failed"
- Quality review is never invoked

### Non-blocking failure proceeds to review
Process an issue with `Verify.Blocking: false` where build fails.
- A warning is logged
- Quality review is still invoked

### Verify runs between guard rails and quality review
Process an issue through the full pipeline.
- Guard rails (PR exists, Closes #N, protected paths) run before verify
- Verify runs after guard rails
- Quality review runs after verify passes
