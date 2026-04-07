# Phase 32: Decision Flow Tracing

Before this phase, correlating artifacts across an issue's lifecycle required manually navigating run directories and matching timestamps. Phase 32 threads a single UUID through every stage of `ProcessIssue()` - from recon through merge - so the entire decision flow is queryable as one unit. The trace ID propagates to JSON artifacts on disk, the SQLite stats database, the `godark trace` CLI command, the web dashboard, and the TUI.

---

## Trace ID Generation and Propagation

**What it does:** At the top of every `ProcessIssue()` call, a v4 UUID is generated and stamped on every `StepResult`, `VerifyStepResult`, and `Outcome` before they're written to disk. This is the foundational plumbing that makes everything else in the phase work.

**Example:** When a run starts processing issue #340, the agent loop in `internal/agent/loop.go` generates and propagates the trace:

```go
var generateTraceID = func() string { return uuid.New().String() }

func ProcessIssue(ctx context.Context, issue github.Issue, ...) IssueOutcome {
    outcome := IssueOutcome{IssueNumber: issue.Number}
    traceID := generateTraceID()
    outcome.TraceID = traceID

    // Before every hook.Write*Result() call:
    step.TraceID = traceID
    // ...
}
```

The design decision was to put `TraceID` on the data structs rather than changing the `RunDataHook` interface methods. This avoided touching all 15 interface methods and their 3 test stub implementations. The writer serializes it automatically via JSON tags:

```go
// In internal/rundata/writer.go
type StepResult struct {
    // ...
    TraceID          string     `json:"trace_id,omitempty"`
}

type Outcome struct {
    // ...
    TraceID      string        `json:"trace_id,omitempty"`
}

type VerifyStepResult struct {
    // ...
    TraceID     string `json:"trace_id,omitempty"`
}
```

Every JSON artifact in `~/.godark/runs/` now carries `trace_id`. An `implement.json` for issue #340 might look like:

```json
{
  "output": "AGENT_RESULT=CHANGES_MADE ...",
  "cost_usd": 1.23,
  "duration_seconds": 180,
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "tool_trace": ["Edit cmd/root.go", "Bash go test ./..."]
}
```

The `generateTraceID` function is a package-level variable for testability - tests can replace it to return predictable IDs.

---

## SQLite Schema Extension

**What it does:** The `step_results` and `issue_outcomes` tables in `stats.db` gain a `trace_id` column, persisted on every INSERT and returned by every query. This makes trace data available for analytics and the `godark trace` command.

**Example:** Two new ALTER TABLE migrations in `internal/stats/schema.go` follow the existing pattern used for `peak_memory_bytes` and `cpu_nanoseconds`:

```sql
ALTER TABLE step_results ADD COLUMN trace_id TEXT DEFAULT ''
ALTER TABLE issue_outcomes ADD COLUMN trace_id TEXT DEFAULT ''
```

The migration is idempotent - `EnsureSchema` checks whether each column already exists before adding it, so upgrading from any prior version works without error. The conversion layer in `internal/stats/convert.go` copies `TraceID` from the `rundata` structs into the `StepResultRecord` and `IssueOutcomeRecord` types:

```go
type StepResultRecord struct {
    // ...
    TraceID          string
}

type IssueOutcomeRecord struct {
    // ...
    TraceID string
}
```

Old runs written before this phase have an empty `trace_id` column, which queries handle gracefully.

---

## `godark trace` CLI Command

**What it does:** A new command queries `stats.db` and renders the full decision flow for an issue. It accepts either an issue number (resolves to the most recent trace) or a trace ID directly. Supports `--repo`, `--run`, and `--json` flags.

**Example:** After a run processes issue #42, an operator can inspect the full lifecycle:

```
$ godark trace 42
Trace: 550e8400-e29b-41d4-a716-446655440000
Issue: #42
Status: implemented
PR: #87

Step              Duration  Cost     Started              Flags
recon             1m23s     $0.0845  2026-04-01 14:00:12
spec_generator    0m45s     $0.0312  2026-04-01 14:01:35
planner           1m02s     $0.0523  2026-04-01 14:02:20
implement         8m15s     $1.2340  2026-04-01 14:03:22
verify            0m38s     $0.0000  2026-04-01 14:11:37
quality_review    2m10s     $0.1890  2026-04-01 14:12:15
functional_review 1m55s     $0.1650  2026-04-01 14:14:25
```

If the same issue was processed in multiple runs, `godark trace 42` returns the most recent. Use `--run` to target a specific run:

```
$ godark trace 42 --run run-20260401-140000
```

For programmatic consumption, `--json` outputs structured data:

```
$ godark trace 42 --json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "issue_number": 42,
  "outcome": {
    "status": "implemented",
    "pr_number": 87
  },
  "steps": [
    {
      "step_name": "recon",
      "duration_seconds": 83,
      "cost_usd": 0.0845,
      "started_at": "2026-04-01T14:00:12Z",
      "flags": []
    }
  ]
}
```

The command is backed by three dedicated query functions in `internal/stats/query.go`:

- `QueryLatestTraceForIssue(ctx, db, issueNumber, repo, runID)` - resolves an issue number to its most recent trace ID, joining `issue_outcomes` with `runs` for ordering
- `QueryStepsByTraceID(ctx, db, traceID)` - returns all steps for a trace, sorted by `started_at`
- `QueryOutcomeByTraceID(ctx, db, traceID)` - returns the outcome record for a trace

The argument is auto-detected: if it parses as an integer, it's treated as an issue number; otherwise, as a trace ID.

---

## Dashboard Trace View

**What it does:** The issue detail page in the web dashboard displays the trace ID as a monospace header field with a copy-to-clipboard button. This gives operators a quick reference ID for debugging conversations and cross-referencing with `godark trace`.

**Example:** When viewing issue #42's detail page, the metadata section shows:

```
#42 - Add rate limiting to API endpoints
Status: implemented  |  PR: #87  |  View on GitHub
Trace: 550e8400-e29b-41d4-a716-446655440000 [Copy]
Clone SHA: a1b2c3d4e5f6 [Copy]
```

The template in `internal/dashboard/templates/issue-detail.html` conditionally renders the trace:

```html
{{if .TraceID}}
<span class="trace-id">Trace: <code>{{.TraceID}}</code>
  <button onclick="navigator.clipboard.writeText('{{.TraceID}}')"
          title="Copy trace ID" class="trace-copy-btn">Copy</button>
</span>
{{end}}
```

Old runs without a trace ID show no trace row - no empty fields or placeholder text. The `IssueDetailData` struct in `internal/dashboard/handlers.go` carries the trace through:

```go
type IssueDetailData struct {
    // ...
    TraceID  string
    CloneSHA string
}
```

Each timeline step also carries its `TraceID` in the `TimelineStepView` struct, populated by `stepToView()`. For a single issue, all steps share the same trace ID.

---

## TUI Trace Column

**What it does:** Completed issues in the TUI display a truncated trace ID (first 8 characters) after their status badge, giving operators an at-a-glance reference without cluttering the table.

**Example:** During a run, an issue transitions through stages. Once complete, the trace ID appears:

```
#42  Add rate limiting to API endpoints    [implemented]  550e8400
#43  Fix null pointer in auth middleware    [implement]
```

Issue #43 is still in progress - no trace ID yet, since it's generated inside `ProcessIssue()` and only flows to the TUI via `IssueCompleted`. The `issueRow` struct in `internal/tui/table.go` holds it:

```go
type issueRow struct {
    number      int
    title       string
    status      string
    // ...
    traceID     string
}
```

The rendering truncates to 8 characters for readability:

```go
if row.traceID != "" {
    short := row.traceID
    if len(short) > 8 {
        short = short[:8]
    }
    // render short trace ID after the badge
}
```

The trace ID flows through the `ProgressReporter` interface - `IssueCompleted` gained a `traceID string` parameter:

```go
func (r *TUIReporter) IssueCompleted(issueNumber int, title, status string,
    prNumber, retries int, errMsg string, costUSD float64, traceID string)
```

The `TextReporter` also accepts and logs the trace ID, so non-TUI output modes get it too.

---

## End-to-End Data Flow

**What it does:** The trace ID flows from generation through every persistence and display layer without gaps. Every hop is covered: loop to hook, hook to disk, disk to stats DB, stats DB to CLI/dashboard, and loop to TUI via the reporter.

**Example:** Here is the complete propagation path for a single trace:

```
generateTraceID() in loop.go
  |
  +-> outcome.TraceID                    (returned to orchestrator)
  |     +-> reporter.IssueCompleted()    (TUI displays truncated ID)
  |     +-> hook.WriteOutcome()          (outcome.json on disk)
  |           +-> stats.WriteOutcome()   (issue_outcomes.trace_id in SQLite)
  |
  +-> step.TraceID = traceID            (set before every hook.Write*Result)
        +-> hook.WriteStepResult()       (implement.json, recon.json, etc.)
              +-> stats.WriteStepResult() (step_results.trace_id in SQLite)

Query paths:
  godark trace 42
    -> QueryLatestTraceForIssue() -> trace_id
    -> QueryStepsByTraceID()     -> all steps
    -> QueryOutcomeByTraceID()   -> outcome

  Dashboard issue detail
    -> reader.LoadRun() -> IssueDetail.Outcome.TraceID -> template

  TUI
    -> IssueCompletedMsg.TraceID -> issueRow.traceID -> renderRow()
```

All artifacts for an issue share the same trace ID. Searching for `"trace_id": "550e8400-..."` across JSON files in a run directory returns every artifact for that issue's processing, making debugging and auditing straightforward.
