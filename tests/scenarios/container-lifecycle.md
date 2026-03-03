# Scenario: Container lifecycle management

Relates to: Issue #22

## Setup
- The sandbox package (`internal/sandbox`) is imported directly
- The `CommandRunner` variable is stubbed to simulate `docker create`, `docker start`, `docker wait`, and `docker rm` commands
- Stub responses return controlled container IDs, exit codes, and stdout/stderr content
- No real Docker daemon or containers required

## Cases

### Successful command run
Call `RunContainer` with a stubbed runner that simulates a successful `echo hello` command.
- `RunResult.ExitCode` is 0
- `RunResult.Stdout` contains `hello`
- `docker rm` is called to clean up the container

### Failed command returns exit code
Call `RunContainer` with a stubbed runner where `docker wait` returns exit code 1.
- `RunResult.ExitCode` is 1
- `docker rm` is still called to clean up

### Stderr is captured
Call `RunContainer` with a stubbed runner that produces stderr output.
- `RunResult.Stderr` contains the expected error output

### Environment variables are passed
Call `RunContainer` with `RunOpts.Env` containing `{"FOO": "bar"}`.
- The stubbed runner receives `-e FOO=bar` (or equivalent) in the `docker create` arguments

### Timeout stops the container
Call `RunContainer` with a short timeout and a stubbed runner that simulates a long-running command.
- `docker stop` is called on the container
- `docker rm` is called to clean up
- The result indicates a timeout occurred

### Cleanup on docker start failure
Stub the runner so `docker create` succeeds but `docker start` fails.
- `docker rm` is still called with the created container ID
- An error is returned from `RunContainer`

### Cleanup on context cancellation
Call `RunContainer` with a pre-cancelled context.
- `docker stop` and `docker rm` are called
- The container is not left running
