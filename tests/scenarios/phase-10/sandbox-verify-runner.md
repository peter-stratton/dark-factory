# Scenario: Sandbox-mode verify runner

Relates to: Issue #201

## Setup
- The `internal/agent/` and `internal/sandbox/` packages
- Mock `sandbox.RunContainer` or `CommandRunner` variable for testing
- Config with `NoSandbox: false`, `Docker.Image` set, and verify commands
- No real Docker containers

## Cases

### Sandbox runner executes in container
Process a verify check with `NoSandbox: false`.
- `sandbox.RunContainer` is called with the verify command
- Container uses the same image as agent containers (`cfg.Docker.Image`)
- Stdout, stderr, and exit code are captured from container result

### Container clones and checks out PR branch
Verify that the sandbox runner's container script includes repo clone and
branch checkout.
- Container command includes `git clone` of the repo
- Container command includes `git checkout` of the PR branch
- Verify command runs inside the cloned workspace

### Host mode unchanged
Process a verify check with `NoSandbox: true`.
- The host-mode runner is used (not sandbox)
- No container is created

### Container failure captured
Process a verify check where the command exits non-zero inside the container.
- Check result has `Passed: false`
- Exit code from the container is captured correctly
- Error output is captured in the check result
