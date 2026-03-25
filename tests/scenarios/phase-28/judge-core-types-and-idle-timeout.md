# Scenario: Judge core types and idle timeout rule

Relates to: Issue #641

## Setup
- `internal/agent/judge/` package exists with `Judge`, `Judgment`, `Intervention`, `Rule`, and `Config` types
- `NewJudge(role string, cfg Config) *Judge` constructs a judge with the idle timeout rule
- Time is controlled via the `now` parameter on `ProcessLine`

## Cases

### Idle timeout fires after threshold
Create a judge with `DefaultIdleTimeout: 10` (seconds).
Feed lines with no `"tool":` content, advancing time by 11 seconds between first and last call.
- `ProcessLine` returns a non-nil `*Intervention`
- `Intervention.Judgment` is `Kill`
- `Intervention.Rule` is `"idle_timeout"`

### Idle timeout resets on tool call
Create a judge with `DefaultIdleTimeout: 10`.
Feed non-tool lines for 8 seconds, then feed a line containing `"tool":`.
Advance time 8 more seconds and feed another non-tool line.
- No intervention returned (tool call reset the clock)

Feed non-tool lines for 11 more seconds after the tool call.
- Intervention returned with `Kill` judgment

### Per-role threshold respected
Create a judge with `IdleTimeoutByRole: {"recon": 5, "implementer": 20}` and role `"recon"`.
Feed non-tool lines for 6 seconds.
- Intervention fires at 6 seconds for recon

Create a judge with same config but role `"implementer"`.
Feed non-tool lines for 6 seconds.
- No intervention (implementer threshold is 20s)

### Default threshold fallback
Create a judge with `DefaultIdleTimeout: 15` and empty `IdleTimeoutByRole`.
Use role `"reviewer"`.
Feed non-tool lines for 16 seconds.
- Intervention fires using the 15-second default

### First ProcessLine call starts the clock
Create a judge with `DefaultIdleTimeout: 10`.
Construct the judge at T=0 but call `ProcessLine` for the first time at T=100.
Call `ProcessLine` again at T=109.
- No intervention (only 9 seconds since first call)

Call `ProcessLine` at T=111.
- Intervention fires (11 seconds since first call)

### Judgment constants exist
- `Kill`, `RetryContainer`, `Warn`, `Ignore` are distinct `Judgment` values

### Intervention struct populated
Trigger an idle timeout intervention.
- `Intervention.Rule` is non-empty
- `Intervention.Detail` is non-empty and human-readable
- `Intervention.DetectedAt` matches the `now` parameter of the triggering call
