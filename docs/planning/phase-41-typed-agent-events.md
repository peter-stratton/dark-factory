# Phase 41: Typed Agent Events

> **Goal:** The judge and downstream consumers operate on typed `Event` values produced by a dedicated parser package, not raw stream-json strings. Claude stream parsing lives in one place. No behavior changes; this is a correctness and maintainability cleanup.

## Milestone

`Phase 41: Typed Agent Events`

---

## Issue 808: Define Event types and ParseEvent in parser package

### Description

Create a new `internal/agent/parser/` package that defines the typed `Event`, `EventKind`, and `Usage` types, and implements `ParseEvent(line []byte) Event` for converting a Claude stream-json line into a typed `Event`. This is the foundation that the judge migration consumes.

This issue is pure new code with no modifications to existing files. The `parser` package sits in the orchestration layer (matching `internal/agent/`) but must have no upward dependencies so it can be imported freely by the judge, launcher, and any future consumer.

`Event.Tool` is a plain string carrying Claude's native tool name (`Bash`, `Edit`, `ToolSearch`, etc.). No normalization layer - that was a multi-harness hedge and godark is Claude-Code-native by design (see `docs/philosophy/claude-code-native.md`).

This issue would touch 1 file across 1 layer: `internal/agent/parser/parser.go` (orchestration). The main complexity is exhaustively mapping Claude's stream-json line shapes onto the `EventKind` enum.

### Key constraints

- New package path: `internal/agent/parser/`. Lives in the orchestration layer per `docs/architecture.json` (paths include `internal/agent/`).
- `Event` struct exactly:
  ```go
  type Event struct {
      Kind   EventKind
      Tool   string          // Claude native name: "Bash", "Edit", "ToolSearch", etc. Empty for non-tool events.
      Path   string          // For tool events that target a file path. Empty otherwise.
      Text   string          // Text content for assistant text events. Empty otherwise.
      Tokens Usage           // Populated for usage events. Zero value otherwise.
      Raw    json.RawMessage // Unmodified provider line for rules that need structured tool input.
  }
  ```
- `EventKind` enum, in this order so `EventUnknown` is the zero value:
  ```go
  type EventKind uint8
  const (
      EventUnknown EventKind = iota
      EventText
      EventToolCall
      EventToolResult
      EventRateLimit
      EventUsage
      EventDone
      EventError
  )
  ```
- `Usage` struct: `InputTokens`, `OutputTokens`, `CacheReadTokens`, `CacheCreationTokens` (all `int64`).
- `ParseEvent(line []byte) Event` - never returns an error. Unrecognized or malformed lines return `Event{Kind: EventUnknown, Raw: line}`. This preserves the launcher's heartbeat semantics and lets the caller stream uninterrupted.
- Stream-json shapes to recognize:
  - `{"type":"assistant","message":{"content":[{"type":"text","text":"..."}]}}` -> `EventText`
  - `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"...","input":{...}}]}}` -> `EventToolCall` (Tool=name, Path=input.file_path if present)
  - `{"type":"user","message":{"content":[{"type":"tool_result",...}]}}` -> `EventToolResult`
  - `{"type":"result"...}` -> `EventDone`
  - Lines containing `rate_limit_event` -> `EventRateLimit`
  - Lines containing usage data on result events -> `EventUsage` (populate `Tokens`)
  - Lines that fail JSON unmarshal entirely -> `EventUnknown`
- No imports from `internal/agent/`, `internal/sandbox/`, `internal/judge/`, or any other `internal/` package outside foundation/content. Standard library only.

### Acceptance criteria

- [ ] Package `internal/agent/parser/` exists and exports `Event`, `EventKind`, `Usage`, `ParseEvent`, plus the `EventKind` constants
- [ ] `EventUnknown` is the zero value of `EventKind`
- [ ] `ParseEvent` returns `Event{Kind: EventUnknown}` (never an error) for unparseable input
- [ ] `ParseEvent` correctly classifies the documented stream-json shapes
- [ ] Package compiles with only standard-library imports (no internal dependencies above foundation/content)

### Test cases

- **Zero value is unknown**: `var e Event; assert e.Kind == EventUnknown && e.Tool == ""`
- **Event JSON round-trip**: Marshal an `Event` populated with each `EventKind` variant; unmarshal; verify deep equality
- **Raw bytes preserved**: Construct an `Event` with arbitrary JSON in `Raw`; marshal then unmarshal; verify `Raw` bytes are byte-for-byte identical
- **ParseEvent on tool_use**: Feed `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`; assert `Kind == EventToolCall`, `Tool == "Bash"`, `Raw` equals the input bytes
- **ParseEvent on text**: Feed a text-only assistant message; assert `Kind == EventText` and `Text` carries the message body
- **ParseEvent on rate_limit_event**: Feed a captured `rate_limit_event` line; assert `Kind == EventRateLimit`
- **ParseEvent on garbage**: Feed `not json at all`; assert `Kind == EventUnknown` with no panic and no error

---

## Issue 809: Extract Claude stream parsing functions from launcher.go

**Blocked by**: #808

### Description

Move the existing Claude-specific parsing helpers out of `internal/agent/launcher.go` into the `internal/agent/parser/` package. This consolidates all Claude stream-parsing concerns in one location and trims `launcher.go` from 580 lines to roughly 450. Pure refactor - no behavior changes, no signature changes for the launcher callers beyond the new package qualifier.

Functions to move: `parseRunnerOutput`, `parseRateLimitEvent`, `extractToolTrace`, `extractCloneSHA`, `extractVerdict`. Supporting types (`runnerFinalResult`, `rateLimitEvent`), regexes (`textResetRe`, `verdictRe`, `cloneSHARe`), and the helper `parseTextResetTime` move with them.

The `claude` argv construction at `launcher.go:189-197` stays in `launcher.go`. Argv construction is a separate concern from parsing, and without a Provider abstraction there is no value in collocating them.

This issue would touch 3 files across 1 layer: `internal/agent/launcher.go`, `internal/agent/parser/parser.go` (or a new file like `parser/legacy.go` in the same package), and `internal/agent/clone_sha_test.go` (orchestration). The main complexity is making sure each of the 5 call sites in `launcher.go` switches to the package-qualified name without behavior drift.

### Key constraints

- Move into the `internal/agent/parser/` package (created in the prior issue):
  - Functions: `parseRunnerOutput` -> `ParseRunnerOutput`, `parseRateLimitEvent` -> `ParseRateLimitEvent`, `extractToolTrace` -> `ExtractToolTrace`, `extractCloneSHA` -> `ExtractCloneSHA`, `extractVerdict` -> `ExtractVerdict`
  - Types: `runnerFinalResult` -> `RunnerFinalResult`, `rateLimitEvent` -> `RateLimitEvent`
  - Helpers: `parseTextResetTime` (can stay unexported as `parseTextResetTime` since it is only used internally)
  - Regexes: `textResetRe`, `verdictRe`, `cloneSHARe` (unexported within parser package)
- Update the 5 call sites in `internal/agent/launcher.go`:
  - line ~298 (`parseRateLimitEvent` call)
  - line ~309 (`parseRunnerOutput` call)
  - line ~321 (`extractVerdict` call)
  - line ~324 (`extractToolTrace` call)
  - line ~339 (`extractCloneSHA` call)
- Update `internal/agent/clone_sha_test.go` to call `parser.ExtractCloneSHA` instead of the package-local `extractCloneSHA`
- Do NOT touch the `claude` argv construction at `launcher.go:189-197`
- Do NOT modify `ParseEvent` or any of the types from the prior issue
- Behavior must be byte-for-byte identical: same captured stream-json input must produce the same `RunnerFinalResult`, the same rate-limit event tuple, the same tool trace, the same clone SHA, and the same verdict before and after the move

### Acceptance criteria

- [ ] All 5 functions and their supporting types/regexes/helpers exist in the `internal/agent/parser/` package
- [ ] `internal/agent/launcher.go` no longer defines these functions, types, regexes, or `parseTextResetTime`
- [ ] `launcher.go` calls the parser-qualified versions at the 5 documented call sites
- [ ] `internal/agent/clone_sha_test.go` calls `parser.ExtractCloneSHA` and passes
- [ ] All existing tests in `internal/agent/` pass with no test source changes (other than the `clone_sha_test.go` import update)

### Test cases

- **Build clean**: `go build ./...` succeeds after the move
- **Existing agent tests pass**: `go test ./internal/agent/...` passes
- **Behavior equivalence on captured transcript**: Feed a captured stream-json transcript through `parser.ParseRunnerOutput`; verify aggregated `SessionID`, `CostUSD`, `Verdict`, `ToolTrace` match the values produced by the pre-refactor function on the same input
- **Argv stays in launcher**: `grep -n "claude -p" internal/agent/launcher.go` still finds the argv construction at the original location range (189-197)

---

## Issue 810: Migrate judge to consume typed events

**Blocked by**: #808

### Description

Change `Judge.ProcessLine(line string, now time.Time) *Intervention` to `Judge.ProcessEvent(event parser.Event, now time.Time) *Intervention`. Update the three existing rules (`no_progress`, `tool_thrash`, `transport_failure`) to branch on `event.Kind` and `event.Tool` rather than regex-matching raw stream-json. Update the launcher to call `parser.ParseEvent` on each stdout line before dispatching to the judge.

This is the payoff issue. The judge stops parsing JSON with regex - which is brittle and silently breaks when Claude's output format shifts - and instead consumes typed values that have a contract.

Heartbeat behavior is preserved: empty heartbeat ticks become `Event{Kind: EventUnknown}` so the no_progress rule still sees the time-keeping signal it needs.

This issue would touch 5 files across 1 layer (orchestration): `internal/agent/judge/judge.go`, `internal/agent/judge/no_progress.go`, `internal/agent/judge/tool_thrash.go`, `internal/agent/judge/transport_failure.go`, `internal/agent/launcher.go`. The main complexity is keeping the rule semantics identical - especially `tool_thrash`, which extracts the search query from tool input JSON.

### Key constraints

- `internal/agent/judge/judge.go` (around line 79):
  - Rename `ProcessLine(line string, now time.Time) *Intervention` to `ProcessEvent(event parser.Event, now time.Time) *Intervention`
  - Update the `Rule` interface (or equivalent) to take `parser.Event` instead of `string`
  - The judge package gains an import of `internal/agent/parser`
- `internal/agent/judge/no_progress.go`:
  - Replace `strings.Contains(line, "\"tool\":") || strings.Contains(line, "\"tool_use\"")` with `event.Kind == parser.EventToolCall`
  - Heartbeat ticks (now `Event{Kind: EventUnknown}`) must still advance the timer the same way the empty string did before
- `internal/agent/judge/tool_thrash.go`:
  - Replace `strings.Contains(line, "ToolSearch")` with `event.Tool == "ToolSearch"`
  - To extract the query: `json.Unmarshal(event.Raw, &shape)` where `shape` is a small struct mirroring the stream-json tool_use envelope, then read `shape.Message.Content[i].Input.Query` (or equivalent path). The existing `queryRe` regex can be retired.
  - The 60-second window and 3-repeat threshold are unchanged
- `internal/agent/judge/transport_failure.go`:
  - Replace `strings.Contains(lower, "stream closed") || strings.Contains(lower, "stream error")` with `event.Kind == parser.EventError`
  - The 10-error threshold and "no tool calls between errors" condition are unchanged
- `internal/agent/launcher.go` (around the `buildJudgeCallback` block at lines 100-162):
  - For active stdout lines (line ~160): `event := parser.ParseEvent([]byte(line)); judge.ProcessEvent(event, now)`
  - For heartbeat ticks (line ~154): dispatch `parser.Event{Kind: parser.EventUnknown}` (or an explicit synthetic event) to keep timers advancing
- All existing judge tests in `internal/agent/judge/` must be updated to construct `parser.Event` values instead of passing raw strings. This is the only category of test source changes allowed in this issue.
- `ProcessLine` is removed from the public surface; no compatibility shim

### Acceptance criteria

- [ ] `Judge.ProcessEvent(parser.Event, time.Time) *Intervention` exists; `Judge.ProcessLine` no longer exists
- [ ] All three rules branch on `event.Kind` and `event.Tool`, not regex on raw lines
- [ ] `tool_thrash` extracts the search query by unmarshaling `event.Raw`, not by regex on the raw line
- [ ] Launcher's `buildJudgeCallback` calls `parser.ParseEvent` for active lines and dispatches an `EventUnknown` for heartbeat ticks
- [ ] All existing judge tests pass with their inputs converted to constructed `parser.Event` values

### Test cases

- **no_progress fires on tool gap**: Send a sequence of `EventText` events spanning the threshold without any `EventToolCall`; assert intervention fires
- **no_progress resets on tool call**: Send `EventText` then `EventToolCall` before threshold; assert no intervention; advance time past threshold; assert intervention now fires only after the new gap
- **tool_thrash fires on repeated query**: Send 3 `EventToolCall` events with `Tool == "ToolSearch"` and the same query in `Raw` within 60s; assert intervention fires
- **tool_thrash query extracted from Raw**: Send a single `EventToolCall` with `Tool == "ToolSearch"` and a query in `Raw`; assert the rule unmarshals successfully and records the query
- **transport_failure fires on repeated errors**: Send 10 `EventError` events with no `EventToolCall` between; assert intervention fires
- **EventUnknown is harmless**: Send `Event{Kind: EventUnknown}` heartbeat tick; assert no rule crashes and no spurious intervention is produced
- **End-to-end via launcher**: Feed a captured stream-json transcript through the launcher's updated `processAndHandle` path; assert the same intervention sequence is produced as the pre-refactor judge produced on the same transcript
