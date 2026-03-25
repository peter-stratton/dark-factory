# Scenario: Real-time log streaming from RunContainer

Relates to: Issue #640

## Setup
- `RunContainer` in `internal/sandbox/container.go` accepts `RunOpts` with optional `LogCallback func(line string)`
- `CommandRunner` and `CommandRunnerWithContext` are stubbed to simulate Docker commands
- `SplitRunner` is stubbed to return post-exit log content

## Cases

### Callback receives lines during execution
Set `LogCallback` on `RunOpts` to a function that appends each line to a slice.
Stub Docker commands to simulate a container that prints several lines to stdout.
Call `RunContainer`.
- The callback slice contains one or more lines
- Lines are complete (no partial line fragments)
- `RunResult.Stdout` is also populated after exit

### Nil callback skips streaming goroutine
Set `LogCallback` to nil on `RunOpts`.
Call `RunContainer`.
- No panic occurs
- `RunResult.Stdout` and `RunResult.Stderr` are populated normally
- No `docker logs --follow` command was issued (only the post-exit `docker logs`)

### Streaming goroutine exits on context cancellation
Set `LogCallback` on `RunOpts`.
Cancel the context shortly after `docker start`.
- The streaming goroutine exits without leaking
- `RunResult.TimedOut` is true
- No goroutine leak detected after test completes

### RunResult still populated with callback set
Set `LogCallback` on `RunOpts`.
Stub Docker commands for a container that runs normally and exits 0.
Call `RunContainer`.
- `RunResult.Stdout` contains the full container output
- `RunResult.Stderr` contains the full stderr output
- `RunResult.ExitCode` is 0
