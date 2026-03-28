# Phase 32: Decision Flow Tracing

Every issue processed by Dark Factory now gets a unique trace ID -- a UUID
generated at the top of `ProcessIssue()` and stamped on every artifact produced
during that issue's lifecycle. The trace ID threads through JSON run data,
SQLite analytics, the CLI, the web dashboard, and the TUI. When you need to
reconstruct what happened to issue #42 last Tuesday, you have one identifier
that pulls the entire decision flow together.

---

## Trace ID Generation and Propagation

**What it does:** A v4 UUID is generated at the start of every `ProcessIssue()`
invocation and stamped onto every `StepResult`, `VerifyStepResult`, `Outcome`,
and `IssueOutcome` produced during that issue's lifecycle. The trace ID
propagates through all hook writes automatically -- no changes to the
`RunDataHook` interface were needed.

**Example:** When `godark run` picks up issue #42, `ProcessIssue()` generates a
trace ID before any agent work begins:

```go
traceID := generateTraceID()
outcome.TraceID = traceID
```

The `generateTraceID` function is a package-level variable for testability:

```go
var generateTraceID = func() string { return uuid.New().String() }
```

Before every `hook.Write*Result()` call in `ProcessIssue()` -- and there are
roughly 12 such call sites spanning recon, implementation, verification, quality
review, and functional review -- the trace ID is stamped:

```go
step.TraceID = traceID
```

The deferred outcome write gets the same stamp:

```go
o.TraceID = traceID
```

The result is that every JSON artifact written to the run data directory
contains a `trace_id` field, all sharing the same value for a given issue run.

---

## Schema Extension

**What it does:** Extends the SQLite analytics database (`~/.godark/stats.db`)
with `trace_id` columns on the `step_results` and `issue_outcomes` tables.
Updates all record types, conversion functions, and write/query operations to
handle the new column.

**Example:** The schema migration in `internal/stats/schema.go` adds the columns
via ALTER TABLE statements that run outside the main migration transaction for
idempotency:

```go
alterStmts := []string{
    `ALTER TABLE step_results ADD COLUMN trace_id TEXT DEFAULT ''`,
    `ALTER TABLE issue_outcomes ADD COLUMN trace_id TEXT DEFAULT ''`,
}
for _, stmt := range alterStmts {
    if _, err := db.Exec(stmt); err != nil {
        if !strings.Contains(err.Error(), "duplicate column name") {
            return fmt.Errorf("execute migration: %w", err)
        }
        // column already exists -- idempotent
    }
}
```

Both `StepResultRecord` and `IssueOutcomeRecord` in `internal/stats/types.go`
carry a `TraceID string` field. The stats writer in
`internal/orchestrator/statswriter.go` copies `TraceID` from the rundata structs
into the records, which are then persisted via INSERT statements that include
the `trace_id` column. Old runs without a trace ID store an empty string --
no migration of historical data required.

---

## JSON Artifact Annotation

**What it does:** All JSON run data artifacts include `trace_id` and inherit it
from the propagation in `ProcessIssue()`. This makes raw artifact files
greppable and joinable by trace.

**Example:** The three rundata structs in `internal/rundata/writer.go` each
carry the field with an `omitempty` tag:

```go
type StepResult struct {
    // ... other fields ...
    CPUNanoseconds   int64      `json:"cpu_nanoseconds,omitempty"`
    TraceID          string     `json:"trace_id,omitempty"`
}

type Outcome struct {
    // ... other fields ...
    Error       string `json:"error,omitempty"`
    TraceID     string `json:"trace_id,omitempty"`
}
```

A typical JSON artifact written by the hook now looks like:

```json
{
  "step_name": "implement",
  "duration_seconds": 142.5,
  "cost_usd": 0.2831,
  "trace_id": "a3f1e2d4-7b8c-4e5f-9a1b-2c3d4e5f6a7b",
  "flags": []
}
```

---

## `godark trace` CLI Command

**What it does:** Queries `stats.db` and renders the full decision flow for an
issue as a structured timeline. Accepts either an issue number (resolves to the
most recent trace) or a trace ID directly. Supports `--repo`, `--run`, and
`--json` flags.

**Example:** After a run completes, an operator wants to see what happened to
issue #42:

```
$ godark trace 42

Trace: a3f1e2d4-7b8c-4e5f-9a1b-2c3d4e5f6a7b
Issue: #42
Status: implemented
PR: #87

Step            Duration  Cost     Started              Flags
recon           1m23s     $0.0412  2026-03-27 14:02:15
implement       4m07s     $0.2831  2026-03-27 14:03:38
verify          0m45s     $0.0156  2026-03-27 14:07:45
quality-review  2m12s     $0.1204  2026-03-27 14:08:30
func-review     1m58s     $0.0987  2026-03-27 14:10:42
```

The command detects whether the argument is an integer (issue number) or a UUID
string (trace ID). For issue numbers, it calls `QueryLatestTraceForIssue` which
joins `issue_outcomes` with `runs` and orders by `started_at DESC` to find the
most recent trace. The `--repo` and `--run` flags narrow the lookup when the
same issue number appears across repos or runs:

```
$ godark trace 42 --repo peter-stratton/dark-factory --run run_20260327_140200
```

For programmatic consumption, `--json` outputs a structured object:

```
$ godark trace 42 --json
{
  "trace_id": "a3f1e2d4-7b8c-4e5f-9a1b-2c3d4e5f6a7b",
  "issue_number": 42,
  "outcome": {
    "status": "implemented",
    "pr_number": 87
  },
  "steps": [
    {
      "step_name": "recon",
      "duration_seconds": 83,
      "cost_usd": 0.0412,
      "started_at": "2026-03-27T14:02:15Z",
      "flags": []
    }
  ]
}
```

---

## Dashboard Trace View

**What it does:** Surfaces the trace ID on the web dashboard's issue detail
page. Displayed as a monospace code block with a copy-to-clipboard button in the
issue metadata header, next to the PR link and GitHub issue link.

**Example:** When an operator opens the issue detail page for issue #42 in the
dashboard, the header shows:

```
Issue #42: Add retry backoff to webhook handler
peter-stratton/dark-factory · PR #87 · View on GitHub · Trace: a3f1e2d4-7b8c-... [Copy]
```

The template in `internal/dashboard/templates/issue-detail.html` conditionally
renders the trace ID only when present, so old runs without a trace ID show no
empty field:

```html
{{if .TraceID}}
&nbsp;&middot;&nbsp;<span class="trace-id">Trace: <code>{{.TraceID}}</code>
  <button onclick="navigator.clipboard.writeText('{{.TraceID}}')"
          title="Copy trace ID" class="trace-copy-btn">Copy</button>
</span>
{{end}}
```

The `IssueDetailData` struct in `internal/dashboard/handlers.go` carries a
`TraceID string` field populated from `IssueDetail.Outcome.TraceID`. The
`TimelineStepView` struct also carries `TraceID` for per-step display.

---

## TUI Trace Column

**What it does:** Displays a truncated trace ID (first 8 characters) in the
terminal UI's completed issue rows. The trace ID appears after the status badge,
rendered in a faint style. In-progress issues show no trace ID since it is not
available until `ProcessIssue()` returns.

**Example:** During a `godark run` with the TUI active, completed rows show the
trace fragment:

```
■ #42 Add retry backoff to webhook handler     IMPLEMENTED  a3f1e2d4
■ #43 Fix dashboard timezone display           IMPLEMENTED  b7c8d9e0
○ #44 Add rate limiting to API endpoints
```

The trace ID flows from `ProcessIssue()` through the `IssueOutcome.TraceID`
field, into `reporter.IssueCompleted()` (which accepts `traceID string` as its
final parameter), through `IssueCompletedMsg.TraceID`, and into
`issueRow.traceID` on the TUI model. The `renderRow` function in
`internal/tui/table.go` truncates and appends it:

```go
if row.traceID != "" {
    short := row.traceID
    if len(short) > 8 {
        short = short[:8]
    }
    line += " " + traceStyle.Render(short)
}
```

The `ProgressReporter` interface, `TUIReporter`, and `TextReporter` all updated
their `IssueCompleted` signatures to accept the trace ID. All three production
callers in the orchestrator and implement command pass `outcome.TraceID`.
