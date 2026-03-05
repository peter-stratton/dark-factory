# Scenario: Agent result timing

Relates to: Issue #95

## Setup
- The `agent.Result` struct in `internal/agent/launcher.go` is the subject
- Timing is tested by running `Run()` with a stubbed command runner that takes
  a known amount of time (or returns immediately)
- No real Claude API, Docker, or GitHub access required

## Cases

### StartedAt populated before execution
Call `Run()` with a stubbed runner.
- `Result.StartedAt` is non-zero
- `Result.StartedAt` is close to `time.Now()` at the time of the call

### FinishedAt populated after execution
Call `Run()` with a stubbed runner.
- `Result.FinishedAt` is non-zero
- `Result.FinishedAt` is after `Result.StartedAt`

### Duration is positive
Call `Run()` with a stubbed runner.
- `Result.FinishedAt.Sub(Result.StartedAt)` is positive

### Timed-out run still has timing
Call `Run()` with a stubbed runner that exceeds the timeout.
- `Result.TimedOut` is true
- `Result.StartedAt` and `Result.FinishedAt` are both non-zero
- Duration reflects the timeout period, not the command's full run time

### ResultToStep uses timing fields
Call `ResultToStep()` with a `Result` that has populated timing fields.
- `StepResult.StartedAt` is formatted as ISO 8601
- `StepResult.FinishedAt` is formatted as ISO 8601
- `StepResult.DurationSeconds` equals `FinishedAt - StartedAt` in seconds

### Host and sandbox both populate timing
Call `Run()` in both host mode and sandbox mode (with stubbed runners).
- Both modes set `StartedAt` and `FinishedAt`
- Sandbox timing includes container startup overhead
