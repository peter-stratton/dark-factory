## Phase 41: Typed Agent Events

**Goal**: The judge and downstream consumers operate on typed `Event` values produced by a dedicated parser package, not raw stream-json strings. Claude stream parsing lives in one place. No behavior changes; this is a correctness and maintainability cleanup.

**Milestone**: `Phase 41: Typed Agent Events` | **Label**: `phase-41`

**Issues**: #808-#810

- `define-event-types` — new `internal/agent/parser/` package with typed `Event`, `EventKind`, and `Usage`. `Event.Tool` is a plain string carrying Claude's native tool name (`Bash`, `Edit`, `ToolSearch`, etc.). JSON round-trip tests. Package has no upward dependencies.
- `extract-claude-stream-parser` — move `parseRunnerOutput`, `parseRateLimitEvent`, `extractToolTrace`, `extractCloneSHA`, and `extractVerdict` out of `internal/agent/launcher.go` into the parser package. `launcher.go` no longer constructs the `claude` argv inline; it calls into the parser to convert each stdout line into an `Event`. Pure refactor — existing tests pass without modification.
- `migrate-judge-to-events` — `Judge.ProcessLine(line string, now time.Time)` becomes `Judge.ProcessEvent(event parser.Event, now time.Time)`. Existing rules (`no_progress`, `tool_thrash`, `transport_failure`) branch on `event.Kind` and `event.Tool` instead of regex-matching raw JSON. Lines that don't parse cleanly produce an `EventUnknown` event so heartbeat behavior is preserved.
