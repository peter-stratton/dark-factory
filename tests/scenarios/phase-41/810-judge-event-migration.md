# Scenario: Migrate judge to consume typed events

Relates to: Issue #810

## Setup
- Issue #808 is merged so `internal/agent/parser/` exists with `Event`, `EventKind`, and `ParseEvent`
- The judge package and its three rules (`no_progress`, `tool_thrash`, `transport_failure`) exist on the base branch
- A captured stream-json transcript fixture is available, along with the intervention sequence the pre-refactor judge produced for it

## Cases

### ProcessLine is replaced by ProcessEvent
- GIVEN the refactored `internal/agent/judge/judge.go`
- WHEN the public surface is inspected
- THEN `Judge.ProcessEvent(event parser.Event, now time.Time) *Intervention` exists and `Judge.ProcessLine` has been removed entirely (no compatibility shim)

### Rules branch on event fields, not raw lines
- GIVEN the refactored rule files `no_progress.go`, `tool_thrash.go`, and `transport_failure.go`
- WHEN the source of each rule is inspected
- THEN no rule calls `strings.Contains` against the previous raw-line markers (`"\"tool\":"`, `"\"tool_use\""`, `"ToolSearch"`, `"stream closed"`, `"stream error"`); instead, each branches on `event.Kind` or `event.Tool`

### no_progress fires when no tool calls occur within the threshold
- GIVEN a freshly constructed `Judge` with a configured no-progress threshold
- WHEN a sequence of `Event{Kind: EventText}` values is dispatched at timestamps spanning past the threshold without any `EventToolCall`
- THEN `ProcessEvent` returns a non-nil `*Intervention` from the `no_progress` rule

### no_progress resets when a tool call is observed
- GIVEN a freshly constructed `Judge` and a sequence of `EventText` events that has not yet crossed the threshold
- WHEN an `Event{Kind: EventToolCall}` is dispatched before the threshold elapses
- THEN no intervention is returned at that point and the rule's internal timer has been reset

### tool_thrash fires on three repeated ToolSearch queries within the window
- GIVEN a freshly constructed `Judge`
- WHEN three `Event{Kind: EventToolCall, Tool: "ToolSearch"}` events are dispatched within 60 seconds, each with the same `query` value embedded in `Raw`
- THEN the third dispatch returns a non-nil `*Intervention` from the `tool_thrash` rule

### tool_thrash extracts the query by unmarshaling Raw
- GIVEN a single `Event{Kind: EventToolCall, Tool: "ToolSearch"}` whose `Raw` contains a stream-json tool_use envelope with `input.query == "needle"`
- WHEN the rule processes the event
- THEN it successfully unmarshals `Raw` to recover the query value `"needle"` and records it in its repeat-tracking state without using regex

### transport_failure fires after repeated EventError events with no tool call between
- GIVEN a freshly constructed `Judge`
- WHEN ten `Event{Kind: EventError}` events are dispatched in sequence with no `EventToolCall` interleaved
- THEN the tenth dispatch returns a non-nil `*Intervention` from the `transport_failure` rule

### EventUnknown heartbeat ticks pass through harmlessly
- GIVEN a freshly constructed `Judge` and no prior events
- WHEN an `Event{Kind: EventUnknown}` is dispatched (the heartbeat-tick equivalent of the previous empty-string call)
- THEN no rule panics, no spurious intervention is returned, and the no_progress timer continues to advance just as it did with the empty-string heartbeat

### Launcher dispatches typed events to the judge
- GIVEN the refactored `internal/agent/launcher.go`
- WHEN the `buildJudgeCallback` block is inspected
- THEN active stdout lines are converted via `parser.ParseEvent` before being passed to `Judge.ProcessEvent`, and heartbeat ticks dispatch a `parser.Event{Kind: parser.EventUnknown}` rather than an empty string

### End-to-end intervention sequence matches the pre-refactor behavior
- GIVEN the captured stream-json transcript fixture and the known pre-refactor intervention sequence
- WHEN the transcript is replayed through the launcher's updated `processAndHandle` path against a freshly constructed judge
- THEN the resulting intervention sequence (rules fired, in order, with the same trigger lines) matches the pre-refactor sequence exactly
