# Scenario: Wire judge into launcher — kill and partial result

Relates to: Issue #645

## Setup
- `runSandbox()` in `internal/agent/launcher.go` creates a judge from config and wires it to `LogCallback`
- `sandbox.RunContainer` is stubbed to control log streaming behavior
- `agent.Result` has `JudgeKilled bool` and `JudgeIntervention *judge.Intervention` fields

## Cases

### Kill stops container and returns partial result
Stub `RunContainer` to stream log lines that trigger the idle timeout rule (no tool calls for longer than threshold).
Call `Run` in sandbox mode with judge enabled.
- Result has `JudgeKilled: true`
- Result has `TimedOut: false`
- Result has partial stdout/stderr content
- `Result.JudgeIntervention` is non-nil

### Judge disabled skips callback
Set judge config to `enabled: false`.
Call `Run` in sandbox mode.
- Container runs to completion without judge interference
- `Result.JudgeKilled` is false
- No `LogCallback` was set on `sandbox.RunOpts`

### No intervention on normal execution
Stub `RunContainer` to stream lines that include regular tool call audit lines.
Call `Run` in sandbox mode with judge enabled.
- Result has `JudgeKilled: false`
- `Result.JudgeIntervention` is nil
- Container ran to normal completion

### Intervention record fully populated
Trigger a judge kill via idle timeout.
- `Result.JudgeIntervention.Rule` is `"idle_timeout"`
- `Result.JudgeIntervention.Judgment` is `Kill`
- `Result.JudgeIntervention.Detail` is non-empty
- `Result.JudgeIntervention.DetectedAt` is non-zero

### Kill with usable result preserves parsed output
Stub `RunContainer` so the agent prints a valid final result JSON line (with session_id, cost, result text), then goes idle past the threshold. Judge kills the container.
- `Result.JudgeKilled` is true
- `Result.ResultText` is non-empty (parsed from final JSON)
- `Result.SessionID` is non-empty
- `Result.CostUSD` is greater than zero
