# Scenario: Judge tool thrash and transport failure rules

Relates to: Issue #644

## Setup
- `internal/agent/judge/` package contains the tool thrash and transport failure rules
- Both rules implement the `Rule` interface and are included in `NewJudge`
- Config fields: `ToolThrashThreshold` (default 3), `ToolThrashWindowSecs` (default 60), `TransportFailureThreshold` (default 10)

## Cases

### Tool thrash fires on repeated queries
Create a judge with default config.
Feed 3 log lines containing ToolSearch with the same query within 60 seconds.
- `ProcessLine` returns an intervention with `Judgment: Kill`
- `Intervention.Rule` is `"tool_thrash"`

### Tool thrash ignores different queries
Feed 3 ToolSearch lines each with a different query string within 60 seconds.
- No intervention returned

### Tool thrash outside window does not fire
Feed 3 ToolSearch lines with the same query, but spread across 120 seconds (each 60s apart).
- No intervention returned (sliding window expired)

### Transport failure fires on stream errors with zero tool calls
Feed 10 lines containing "stream closed" with no `"tool":` lines present.
- `ProcessLine` returns an intervention with `Judgment: RetryContainer`
- `Intervention.Rule` is `"transport_failure"`

### Transport failure does not fire when tool calls present
Feed 10 "stream closed" lines but also feed 1 line containing `"tool":`.
- No intervention returned (transport recovered — agent made progress)
