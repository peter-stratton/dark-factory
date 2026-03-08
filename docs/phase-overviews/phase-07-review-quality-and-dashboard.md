# Phase 7: Review Quality & Dashboard

Phase 7 gave Dark Factory a memory and a face. Before this phase, every run was fire-and-forget -- the orchestrator logged to a flat file and moved on. Now every agent invocation writes structured telemetry to disk, quality checks flag suspicious reviews in real time, and `godark status` launches a local web dashboard where you can drill into any run, any issue, any tool call. The result: human operators can spot-check agent work without reading raw logs or grepping through JSON.

---

## Run Data Writer

Every `godark run` and `godark implement` invocation writes structured JSON files to a timestamped directory under `~/.godark/runs/`. The directory layout mirrors the agent loop: one subdirectory per issue, with separate files for each step (implementation, quality review, functional review, retries, outcome).

**What you see on disk after a run:**

```
~/.godark/runs/acme/widget-service/20260307-143022/
  run.json                          # repo, milestone, issue list, start/finish times, summary
  debug.log                         # structured JSON log (moved here from logs/)
  issues/
    42/
      implement.json                # timing, cost, tool trace
      quality-review.json           # QA reviewer result + quality flags
      functional-review.json        # functional reviewer result + flags
      outcome.json                  # final status, PR number
      punchlist.json                # verification steps, scenario cases
      dialogue.json                 # agent PR comment dialogue
      retries/
        1/
          retry.json                # implementer retry after changes requested
          quality-review.json       # QA review of retry
          functional-review.json    # functional review that triggered retry
```

When the run finishes, `run.json` is updated with a summary:

```json
{
  "repo": "acme/widget-service",
  "milestone": "Phase 3",
  "issue_numbers": [42, 43, 44],
  "started_at": "2026-03-07T14:30:22Z",
  "finished_at": "2026-03-07T15:12:08Z",
  "summary": {
    "total": 3,
    "implemented": 2,
    "failed": 1
  }
}
```

---

## Agent Result Timing

Every agent invocation -- implementer, reviewer, spec generator -- records wall-clock start and finish times on the Go side, including container startup overhead. Duration is computed as `finished_at - started_at` and written to the step's JSON file.

**Example step result in `implement.json`:**

```json
{
  "started_at": "2026-03-07T14:30:24Z",
  "finished_at": "2026-03-07T14:35:51Z",
  "duration_seconds": 327.4,
  "cost_usd": 0.0847,
  "tool_trace": ["Read: internal/config/config.go", "Edit: internal/config/config.go", "Bash: go test ./..."]
}
```

This matters because cost alone does not tell you whether an agent was productive. A $0.08 review that lasted 5 minutes and read 30 files is different from a $0.08 review that lasted 12 seconds and read nothing.

---

## Quality Flag Analysis

The `internal/quality/` package runs four checks on every review step and flags suspicious behavior. Flags are informational -- they never block a merge -- but they show up prominently in the dashboard and run data.

**The checks:**

| Flag Code | Trigger |
|---|---|
| `low_cost` | Review cost below configured threshold |
| `short_duration` | Review completed faster than configured threshold |
| `no_diff_read` | Tool trace contains no `Read` or `gh pr diff` call -- the reviewer never looked at the changes |
| `no_tests_run` | Tool trace contains no test command execution |
| `no_review_tests_written` | Functional reviewer did not write test files to the review directory |
| `no_review_tests_run` | Functional reviewer did not run the review test suite |

**Configuring thresholds in `godark.yaml`:**

```yaml
quality:
  min_review_cost_usd: 0.05       # flag reviews cheaper than $0.05
  min_review_duration_seconds: 30  # flag reviews shorter than 30 seconds
```

Both default to 0 (disabled). The quality reviewer is exempt from the review-test-execution checks since only the functional reviewer is expected to write and run tests.

**What a flag looks like in the run data:**

```json
{
  "duration_seconds": 8.2,
  "cost_usd": 0.0031,
  "flags": [
    {"code": "short_duration", "message": "review duration 8s is below threshold 30s"},
    {"code": "low_cost", "message": "review cost $0.0031 is below threshold $0.0500"},
    {"code": "no_diff_read", "message": "no diff read detected in tool trace (expected Read or gh pr diff)"}
  ]
}
```

When flags fire, they also appear as `slog.Warn` entries in `debug.log`, so you see them in real time during a run.

---

## Dashboard: Run List

`godark status` starts a local web server on `localhost:8374` and opens your browser. The homepage shows every run across all repos, most recent first, with pass/fail counts and a repo filter dropdown.

**Launching it:**

```
$ godark status
2026/03/07 14:42:01 INFO dashboard server started url=http://127.0.0.1:8374
```

The run list shows each run as a row with: repo name, milestone (or issue numbers for single-issue runs), issue count, pass/fail breakdown with a progress bar, status badge (Passed/Failed/Running), and relative timestamp ("3 hours ago").

The repo filter uses htmx to swap the table body without a full page reload -- select a repo from the dropdown and only that repo's runs appear.

You can change the port with `--port`:

```
$ godark status --port 9090
```

---

## Dashboard: Run Detail

Click any run to see per-issue outcomes. Each issue row shows its status (color-coded green/red/blue), PR number (linked to GitHub), retry count, total quality flag count, and total cost.

**URL pattern:** `/runs/acme/widget-service/20260307-143022`

From here you can see at a glance which issues succeeded on the first try, which needed retries, and which had quality flags raised. The issues table auto-refreshes via htmx polling, so if you open the detail page while a run is still in progress, you see issues flip from "Running" to "Implemented" or "Failed" in real time.

---

## Dashboard: Issue Detail

Click an issue to see its full review chain as a vertical timeline. Each step -- Spec Generator, Implement, Quality Review, Retry 1, Quality Review (Retry 1), Functional Review -- appears as a card showing duration, cost, verdict, and any quality flags.

**What you see for a typical issue that needed one retry:**

```
Spec Generator         12s   $0.0023   Passed
Implement             4m32s  $0.0847   Passed
Quality Review          45s  $0.0156   Passed
Functional Review       52s  $0.0203   Changes Requested
Retry 1               2m18s  $0.0512   Passed
Quality Review (R1)     38s  $0.0134   Passed
Functional Review     1m04s  $0.0241   Passed
```

Each step has an expandable tool trace (toggled via Alpine.js) showing every tool call the agent made. This is where you answer questions like "did the reviewer actually read the changed files?" without searching through logs.

The page also shows the issue's punchlist (verification steps, scenario cases, changed files) and agent dialogue entries (implementation notes and review notes posted as PR comments).

---

## Dashboard: Log Viewer

Each run's `debug.log` is viewable in the browser at `/runs/<owner>/<repo>/<timestamp>/logs`. The viewer parses the JSON-lines log file and renders it as a filterable, searchable table.

**Features:**

- **Level filtering:** Click a level button (DEBUG, INFO, WARN, ERROR) to show only entries at that level. Driven by htmx, no page reload.
- **Search:** Type a query to filter entries whose message or structured fields contain the term. Useful for narrowing to a specific issue number or agent step.
- **Pagination:** Loads 50 entries per page (newest first), with a "Load more" button that appends the next batch via htmx.
- **Notable highlighting:** Key milestone messages -- "starting implementer agent", "functional reviewer finished", "quality review requested changes" -- are visually highlighted so you can scan a long log for the important transitions.

---

## Debug Log Migration

Before Phase 7, the debug log lived in a shared `logs/` directory configured via `log_dir` in `godark.yaml`. Now it is co-located in the run directory as `debug.log`, making each run self-contained. The `log_dir` config field was removed.

In dry-run mode (where no run directory is created), the logger writes to a private temporary directory to avoid path collisions between concurrent dry-runs.

---

## Tech Stack

The dashboard is a single-binary web app with no external dependencies at runtime:

- **Go templates** for server-side HTML rendering
- **htmx** (vendored) for partial page updates (table filtering, log pagination, auto-refresh)
- **Alpine.js** (vendored) for client-side interactivity (tool trace toggles, expandable sections)
- **Chart.js** (vendored) for trend charts on the analysis page
- **Embedded filesystem** (`//go:embed`) for all templates, CSS, and JS -- no file paths to configure
- **Goldmark** for rendering Markdown content (issue descriptions) in templates

The server binds to localhost only and shuts down gracefully on Ctrl-C.
