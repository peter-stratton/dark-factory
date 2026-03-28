# Phase 32: Decision Flow Tracing

> **Goal:** Every issue processed by Dark Factory gets a unique trace ID that
> threads through all stages, making the full lifecycle — from recon through
> merge — queryable and reconstructible as a single unit.

## Milestone

`Phase 32: Decision Flow Tracing`

---

## Issue 700: Add TraceID to rundata types and generate in ProcessIssue

### Description

Add a `TraceID` field to `StepResult`, `VerifyStepResult`, `Outcome`, and
`IssueOutcome`. Generate a UUID at the top of `ProcessIssue()` and stamp it
on every result before passing to the hook. This is the foundational change
that makes every artifact traceable.

The key design decision: TraceID lives on the data structs, not on the
`RunDataHook` interface methods. This avoids changing all 15 interface methods
and their 3 test stub implementations. The writer serializes TraceID
automatically via JSON tags.

### Key constraints

- Add `github.com/google/uuid` dependency (or use `crypto/rand` to generate
  a v4 UUID without a dependency — implementer's choice, both are fine)
- In `internal/rundata/writer.go`:
  - Add `TraceID string \`json:"trace_id,omitempty"\`` to `StepResult` struct
    (after `CPUNanoseconds`)
  - Add `TraceID string \`json:"trace_id,omitempty"\`` to `VerifyStepResult`
    struct
  - Add `TraceID string \`json:"trace_id,omitempty"\`` to `Outcome` struct
- In `internal/agent/loop.go`:
  - At the top of `ProcessIssue()` (after `outcome := IssueOutcome{...}`),
    generate a trace ID: `traceID := generateTraceID()`
  - Set `outcome.TraceID = traceID`
  - Before every `hook.Write*Result(...)` call, set `step.TraceID = traceID`
    on the `StepResult` being passed. There are ~15 such call sites in
    `ProcessIssue()` — each needs the stamp
  - In the deferred `hook.WriteOutcome(o)` block, set `o.TraceID = traceID`
  - Add a package-level `generateTraceID()` function (unexported) that returns
    a UUID string. Use a package-level variable for testability:
    ```go
    var generateTraceID = func() string { return uuid.New().String() }
    ```
- In `internal/agent/loop.go` — `IssueOutcome` struct:
  - Add `TraceID string` field
- In `internal/agent/rundata.go`:
  - `ResultToStep` does not need to copy TraceID — it is set by ProcessIssue
    after conversion. No change needed here
- The `RunDataHook` interface in `runhook.go` does NOT change
- Test stubs (`testRunDataHook`, `captureRunDataHook`,
  `captureRetryFunctionalHook`) do NOT need updates

### Acceptance criteria

- [ ] `StepResult` has `TraceID string` field with `json:"trace_id,omitempty"` tag
- [ ] `VerifyStepResult` has `TraceID string` field with `json:"trace_id,omitempty"` tag
- [ ] `Outcome` has `TraceID string` field with `json:"trace_id,omitempty"` tag
- [ ] `IssueOutcome` has `TraceID string` field
- [ ] `ProcessIssue()` generates a UUID at the start and stamps it on all results
- [ ] All JSON artifacts written by the hook contain `trace_id`
- [ ] `go build ./...` passes
- [ ] `go test ./internal/agent/...` passes

### Test cases

- **TraceID generated**: Call `ProcessIssue()` with a `captureRunDataHook` —
  verify all captured `StepResult` values have a non-empty, identical `TraceID`
- **TraceID on outcome**: Call `ProcessIssue()` with a `captureRunDataHook` —
  verify the `Outcome.TraceID` matches the step `TraceID`
- **TraceID is UUID format**: Verify the generated ID matches UUID v4 format
  (`xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`)
- **TraceID on IssueOutcome**: Call `ProcessIssue()` — verify
  `IssueOutcome.TraceID` is non-empty
- **TraceID on verify results**: Process an issue with verify enabled — verify
  `VerifyStepResult.TraceID` is set

---

## Issue 701: Persist trace_id to stats.db

**Blocked by**: #700

### Description

Extend the SQLite schema to store `trace_id` on `step_results` and
`issue_outcomes` tables. Update the record types, conversion functions, and
write operations to persist the trace ID. Update query functions to return it.

### Key constraints

- In `internal/stats/schema.go`:
  - Add two ALTER TABLE migrations (following the existing `peak_memory_bytes`
    pattern):
    ```sql
    ALTER TABLE step_results ADD COLUMN trace_id TEXT DEFAULT ''
    ALTER TABLE issue_outcomes ADD COLUMN trace_id TEXT DEFAULT ''
    ```
- In `internal/stats/schema.go` — record structs (if defined here) or wherever
  `StepResultRecord` and `IssueOutcomeRecord` are defined:
  - Add `TraceID string` field to `StepResultRecord`
  - Add `TraceID string` field to `IssueOutcomeRecord`
- In `internal/stats/convert.go` (or wherever rundata→stats conversion lives):
  - Copy `TraceID` from `rundata.StepResult` to `StepResultRecord`
  - Copy `TraceID` from `rundata.Outcome` to `IssueOutcomeRecord`
- In `internal/stats/write.go`:
  - Include `trace_id` in the INSERT statements for both `doWriteStepResult`
    and `doWriteIssueOutcome`
- In `internal/stats/query.go`:
  - Include `trace_id` in SELECT for `QueryStepResults` and
    `QueryIssueOutcomes`
  - Scan the column into the record struct

### Acceptance criteria

- [ ] `step_results` table has `trace_id` column after migration
- [ ] `issue_outcomes` table has `trace_id` column after migration
- [ ] `StepResultRecord.TraceID` populated from rundata conversion
- [ ] `IssueOutcomeRecord.TraceID` populated from rundata conversion
- [ ] `trace_id` written to both tables on INSERT
- [ ] `trace_id` returned by query functions
- [ ] `go build ./...` passes
- [ ] `go test ./internal/stats/...` passes

### Test cases

- **Migration idempotent**: Run `EnsureSchema` twice — no error on second run
  (column already exists)
- **Step result round-trip**: Write a `StepResultRecord` with TraceID "abc-123",
  query it back — TraceID matches
- **Issue outcome round-trip**: Write an `IssueOutcomeRecord` with TraceID
  "abc-123", query it back — TraceID matches
- **Empty TraceID**: Write a record with empty TraceID (backwards compat with
  old runs) — no error, queries return empty string
- **Convert copies TraceID**: Convert a `rundata.StepResult` with TraceID set —
  verify `StepResultRecord.TraceID` matches

---

## Issue 702: `godark trace` CLI command

**Blocked by**: #701

### Description

Add a new `godark trace` command that queries `stats.db` and renders the full
decision flow for an issue. Accepts either an issue number (resolves to the
most recent trace) or a trace ID directly. Outputs a structured timeline
showing every stage, its duration, cost, outcome, and flags.

### Key constraints

- New file `internal/cmd/trace.go`:
  - `godark trace <issue-number|trace-id>` — positional argument
  - Flags: `--repo` (filter by repo, optional), `--run` (filter by run ID,
    optional), `--json` (output as JSON)
  - Detect whether argument is a number (issue) or UUID (trace ID)
  - If issue number: query `issue_outcomes` for the most recent row with that
    `issue_number`, extract `trace_id`. If `--run` provided, filter by run_id
  - If trace ID: use directly
  - Query `step_results WHERE trace_id = ?` and `issue_outcomes WHERE
    trace_id = ?`
  - Render timeline to stdout using tabwriter (matching `godark analyze` style)
  - Show: step name, duration, cost, started_at, flags, and final outcome
  - Show trace ID and issue number as header
  - Error if no results found: "No trace found for issue #N"
  - Error if stats.db missing: "No stats database found. Run `godark run`
    first."
- In `internal/stats/query.go`:
  - Add `QueryStepsByTraceID(ctx, db, traceID string) ([]StepResultRecord, error)`
  - Add `QueryOutcomeByTraceID(ctx, db, traceID string) (*IssueOutcomeRecord, error)`
  - Add `QueryLatestTraceForIssue(ctx, db, issueNumber int, repo string) (string, error)`
    — returns the trace_id from the most recent issue_outcome for that issue
    number, joined with runs for ordering by started_at

### Acceptance criteria

- [ ] `godark trace 42` resolves the most recent trace for issue #42
- [ ] `godark trace <uuid>` queries by trace ID directly
- [ ] Timeline shows all steps in chronological order with duration and cost
- [ ] `--repo` filters to a specific repository
- [ ] `--run` filters to a specific run
- [ ] `--json` outputs structured JSON
- [ ] Missing trace shows clear error message

### Test cases

- **By issue number**: Seed stats.db with a run containing issue #42 with
  trace_id "t-1" — `godark trace 42` outputs timeline with correct steps
- **By trace ID**: `godark trace t-1` outputs the same timeline
- **Multiple runs same issue**: Two runs process issue #42 with different
  trace IDs — without `--run`, returns the most recent
- **With --run filter**: Specify a run ID — returns that run's trace even if
  not the most recent
- **No results**: `godark trace 999` — error "No trace found for issue #999"
- **JSON output**: `godark trace 42 --json` — valid JSON with steps array
- **Missing database**: No stats.db — clear error message

---

## Issue 703: Dashboard trace view

**Blocked by**: #700

### Description

Surface the trace ID on the dashboard issue detail page. Add it as a header
field with a copy-to-clipboard button. The trace ID becomes the anchor
operators use to reference specific issue runs in debugging conversations.

### Key constraints

- In `internal/dashboard/handlers.go`:
  - Add `TraceID string` field to `IssueDetailData` struct
  - In the issue detail handler, extract the trace ID from
    `IssueDetail.Outcome.TraceID` and set it on the view model
  - Add `TraceID string` field to `TimelineStepView` struct
  - In `stepToView()`, copy `step.TraceID` to the view
- In `internal/dashboard/templates/issue_detail.html` (or equivalent template):
  - Add a "Trace ID" row in the issue metadata section (near status, PR link)
  - Style as a monospace code block with a copy button (small JS onclick that
    copies to clipboard)
  - If TraceID is empty (old runs), omit the row

### Acceptance criteria

- [ ] Issue detail page shows trace ID when present
- [ ] Trace ID displayed in monospace with copy-to-clipboard
- [ ] Old runs without trace ID show no trace ID row (no empty field)
- [ ] `go build ./...` passes
- [ ] `go test ./internal/dashboard/...` passes

### Test cases

- **Trace ID displayed**: Load issue detail for a run with trace ID — page
  contains the trace ID string
- **Copy button present**: Page contains a clickable element that copies the
  trace ID
- **Missing trace ID**: Load issue detail for an old run (no trace ID) —
  no trace ID row in the metadata section
- **Trace ID in timeline steps**: Each timeline step shows its trace ID (all
  identical for the same issue)

---

## Issue 704: TUI trace column

**Blocked by**: #700

### Description

Display the trace ID in the TUI so operators can grab it during or after a
run. Since the trace ID is generated inside `ProcessIssue()` and the TUI
receives data via the `ProgressReporter` interface, the trace ID flows through
`IssueCompleted` — it appears on the row once the issue finishes.

### Key constraints

- In `internal/progress/reporter.go`:
  - Add `traceID string` parameter to `IssueCompleted` method signature:
    ```go
    IssueCompleted(issueNumber int, title, status string, prNumber, retries int, errMsg string, costUSD float64, traceID string)
    ```
- In `internal/tui/messages.go`:
  - Add `TraceID string` to `IssueCompletedMsg`
- In `internal/tui/reporter.go`:
  - Update `IssueCompleted` to accept and pass through `traceID`
- In `internal/tui/table.go`:
  - Add `traceID string` to `issueRow` struct
  - Display truncated trace ID (first 8 chars) in the row, after the status
    badge. Only show when non-empty
- In `internal/tui/model.go`:
  - In `handleIssueCompleted`, copy `msg.TraceID` to `issueRow.traceID`
- In `internal/progress/text.go`:
  - Update `TextReporter.IssueCompleted` signature to accept `traceID`
    (log it if non-empty)
- Callers of `reporter.IssueCompleted` (3 production sites):
  - `internal/orchestrator/orchestrator.go:~391` — pass `outcome.TraceID`
  - `internal/orchestrator/orchestrator.go:~756` — pass `outcome.TraceID`
  - `internal/cmd/implement.go:~285` — pass `outcome.TraceID`
- Test stubs (3 stubs with `IssueCompleted` method):
  - `internal/orchestrator/orchestrator_test.go` — `fakeReporter`
  - `internal/cmd/implement_test.go` — `stubProgressReporter`
  - `internal/agent/loop_test.go` — `mockReporter`

### Acceptance criteria

- [ ] `ProgressReporter.IssueCompleted` accepts `traceID` parameter
- [ ] `IssueCompletedMsg` carries `TraceID`
- [ ] Completed issues in TUI show truncated trace ID (first 8 chars)
- [ ] In-progress issues show no trace ID (not yet available)
- [ ] All 3 production callers pass `outcome.TraceID`
- [ ] All 3 test stubs compile with updated signature
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

### Test cases

- **Trace ID in completed row**: Complete an issue with TraceID "abcd1234-..."
  — TUI row displays "abcd1234"
- **No trace ID while in progress**: Issue in "implement" stage — no trace ID
  shown in row
- **Empty trace ID**: Complete an issue with empty TraceID (old path) — no
  trace ID fragment shown
- **Text reporter logs trace ID**: Complete an issue via TextReporter — log
  output includes the trace ID

---

## Integration chain audit

```
generateTraceID() defined in loop.go
  -> called by ProcessIssue() in loop.go                         <- Issue 1
  -> stored in local traceID variable                            <- Issue 1

traceID stamped on StepResult.TraceID
  -> before every hook.Write*Result() call in loop.go            <- Issue 1
  -> serialized to JSON by Writer methods in writer.go           <- automatic (JSON tags)
  -> deserialized by Reader in reader.go                         <- automatic (JSON tags)
  -> copied to StepResultRecord by convert.go                    <- Issue 2
  -> written to step_results.trace_id by write.go                <- Issue 2
  -> queried by query.go                                         <- Issue 2
  -> queried by QueryStepsByTraceID in query.go                  <- Issue 3
  -> rendered by cmd/trace.go                                    <- Issue 3

traceID stamped on Outcome.TraceID
  -> in deferred WriteOutcome block in loop.go                   <- Issue 1
  -> serialized to JSON by Writer.WriteOutcome in writer.go      <- automatic
  -> deserialized by Reader into IssueDetail.Outcome.TraceID     <- automatic
  -> copied to IssueOutcomeRecord by convert.go                  <- Issue 2
  -> written to issue_outcomes.trace_id by write.go              <- Issue 2
  -> queried by QueryLatestTraceForIssue in query.go             <- Issue 3
  -> read by dashboard handler from IssueDetail.Outcome.TraceID  <- Issue 4
  -> rendered in issue_detail.html template                      <- Issue 4

traceID stored in IssueOutcome.TraceID
  -> set by ProcessIssue() in loop.go                            <- Issue 1
  -> returned to orchestrator/implement caller                   <- automatic (return value)
  -> passed to reporter.IssueCompleted() by orchestrator.go      <- Issue 5
  -> passed to reporter.IssueCompleted() by implement.go         <- Issue 5
  -> sent as IssueCompletedMsg.TraceID to TUI                    <- Issue 5
  -> stored in issueRow.traceID by model.go                      <- Issue 5
  -> rendered (truncated) by table.go                            <- Issue 5

ProgressReporter.IssueCompleted signature change
  -> interface in progress/reporter.go                           <- Issue 5
  -> TUIReporter in tui/reporter.go                              <- Issue 5
  -> TextReporter in progress/text.go                            <- Issue 5
  -> fakeReporter in orchestrator/orchestrator_test.go           <- Issue 5
  -> stubProgressReporter in cmd/implement_test.go               <- Issue 5
  -> mockReporter in agent/loop_test.go                          <- Issue 5
```

All hops covered. No gaps.
