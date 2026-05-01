# Scenario: Typed Event types and ParseEvent in parser package

Relates to: Issue #808

## Setup
- A clean checkout of the repository with no `internal/agent/parser/` package present
- Go toolchain available; standard `testing` package only
- A few captured Claude stream-json lines available as inline test fixtures (assistant text message, tool_use for Bash, rate_limit_event, malformed garbage)

## Cases

### Package compiles with declared types
- GIVEN no `internal/agent/parser/` package exists
- WHEN the issue is implemented and `go build ./...` runs
- THEN the package compiles and exports `Event`, `EventKind`, `Usage`, `ParseEvent`, and the `EventKind` constants (`EventUnknown`, `EventText`, `EventToolCall`, `EventToolResult`, `EventRateLimit`, `EventUsage`, `EventDone`, `EventError`)

### Package has no upward dependencies
- GIVEN the new `internal/agent/parser/` package
- WHEN its imports are inspected
- THEN it imports only standard-library packages and nothing from `internal/agent/`, `internal/sandbox/`, `internal/judge/`, or any other internal package above the foundation/content layers

### Zero value of EventKind is Unknown
- GIVEN a freshly declared `var e Event`
- WHEN `e.Kind` and `e.Tool` are read without assignment
- THEN `e.Kind` equals `EventUnknown` and `e.Tool` is the empty string

### Event JSON round-trips for every kind
- GIVEN an `Event` value populated for each variant of `EventKind` (Unknown, Text, ToolCall, ToolResult, RateLimit, Usage, Done, Error)
- WHEN the event is marshaled to JSON and unmarshaled back into a new `Event`
- THEN the resulting value is deeply equal to the original

### Raw JSON survives round-trip unchanged
- GIVEN an `Event` whose `Raw` field contains arbitrary provider-specific JSON bytes
- WHEN the event is marshaled and then unmarshaled
- THEN the `Raw` field is byte-for-byte identical to the original

### ParseEvent classifies a tool_use line
- GIVEN the line `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`
- WHEN `ParseEvent(line)` is called
- THEN the returned event has `Kind == EventToolCall`, `Tool == "Bash"`, and `Raw` equal to the input bytes

### ParseEvent classifies an assistant text line
- GIVEN a Claude stream-json line carrying a single assistant `text` content block with body `"hello"`
- WHEN `ParseEvent(line)` is called
- THEN the returned event has `Kind == EventText` and `Text == "hello"`

### ParseEvent classifies a rate_limit_event line
- GIVEN a captured `rate_limit_event` stream-json line
- WHEN `ParseEvent(line)` is called
- THEN the returned event has `Kind == EventRateLimit`

### ParseEvent on garbage returns Unknown
- GIVEN the input bytes `not json at all`
- WHEN `ParseEvent(line)` is called
- THEN it returns `Event{Kind: EventUnknown}` without error and without panicking

### ParseEvent records tool path when present
- GIVEN a tool_use line whose `input` contains a `file_path` field (e.g. an `Edit` invocation)
- WHEN `ParseEvent(line)` is called
- THEN the returned event's `Path` field equals the value of `input.file_path`
