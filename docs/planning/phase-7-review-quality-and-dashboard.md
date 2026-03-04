# Phase 7: Review Quality & Dashboard

> **Goal:** Capture review telemetry, report on review quality metrics, enforce
> mandatory review test execution, and surface it all in a local web dashboard
> for human spot-checking.

## Milestone

`Phase 7`

---

## Issue 74: Run data writer

### Description

The orchestrator writes structured JSON files to
`~/.godark/runs/<owner>/<repo>/<timestamp>/` as it processes each issue. This
is the foundation for the dashboard — every subsequent issue reads from this
directory structure.

Both `godark run` and `godark implement` write the same format. A run is a
run regardless of how many issues it contains.

The existing slog debug log moves into the run directory as `debug.log`,
replacing the current `logs/` directory. The `log_dir` config field is
removed.

### Key constraints

- New package: `internal/rundata/` — responsible for creating the run
  directory, writing JSON files, and moving the debug log
- Directory structure:
  ```
  ~/.godark/runs/<owner>/<repo>/<timestamp>/
    run.json              # written at start, updated at end
    debug.log             # slog JSON stream (replaces logs/)
    issues/
      <issue-number>/
        outcome.json      # final status, PR number, retry count
        implement.json    # Result fields: duration, cost, tool trace
        quality-review.json    # per-cycle array of Result data
        functional-review.json # Result data
        retries/
          <n>/
            retry.json           # Result fields for retry invocation
            quality-review.json  # Result data for post-retry QA review
  ```
- `run.json` schema:
  ```json
  {
    "started_at": "RFC3339 timestamp",
    "finished_at": "RFC3339 timestamp or null",
    "repo": "owner/repo",
    "milestone": "Phase N or empty for implement",
    "issue_numbers": [42, 43],
    "config_snapshot": { "max_retries": 3, "...": "..." },
    "summary": {
      "total": 2,
      "implemented": 1,
      "failed": 1,
      "skipped": 0,
      "total_cost_usd": 1.23
    }
  }
  ```
- Per-step JSON files (implement.json, review files, retry files) share a
  common schema based on the existing `Result` struct:
  ```json
  {
    "started_at": "RFC3339",
    "finished_at": "RFC3339",
    "duration_seconds": 120.5,
    "cost_usd": 0.42,
    "session_id": "sess-abc123",
    "verdict": "APPROVED",
    "tool_trace": ["Read foo.go", "Edit bar.go", "Bash: go test ./..."],
    "timed_out": false,
    "exit_code": 0
  }
  ```
- `outcome.json` schema:
  ```json
  {
    "issue_number": 42,
    "issue_title": "Add widget support",
    "status": "implemented",
    "pr_number": 57,
    "retries": 1,
    "total_cost_usd": 1.85,
    "error": null
  }
  ```
- The `<timestamp>` directory name uses `YYYYMMDD-HHMMSS` format (no colons,
  filesystem-safe)
- `<owner>/<repo>` is derived from the `--repo` flag (already required)
- `RunDataWriter` is created at the start of the orchestrator loop and passed
  through to `ProcessIssue`. It exposes methods like `WriteImplementResult`,
  `WriteReviewResult`, `WriteRetryResult`, `WriteOutcome`, `FinalizeRun`
- Update `NewLogger` to accept the run directory path and write `debug.log`
  there instead of `logs/`
- Remove `LogDir` from `Config` and `log_dir` from YAML schema
- The `RunDataWriter` must handle the case where `~/.godark/` doesn't exist
  (create it)

### Acceptance criteria

- [ ] `~/.godark/runs/<owner>/<repo>/<timestamp>/` is created at run start
- [ ] `run.json` is written at start with config snapshot, updated at end with summary
- [ ] Per-issue files are written as each step completes
- [ ] Debug log is written to `debug.log` in the run directory
- [ ] `LogDir` config field removed; `logs/` directory no longer used
- [ ] `godark implement` writes the same directory structure as `godark run`
- [ ] Directory creation handles missing `~/.godark/` gracefully
- [ ] `go test ./...` passes

### Test cases

- **Run directory created**: `RunDataWriter.New()` creates the expected directory path under `~/.godark/runs/`
- **Run.json written at start**: After `New()`, `run.json` exists with `started_at`, `repo`, and `issue_numbers`
- **Run.json updated at end**: After `FinalizeRun()`, `run.json` has `finished_at` and `summary`
- **Implement result written**: `WriteImplementResult(42, result)` creates `issues/42/implement.json`
- **Review result written**: `WriteReviewResult(42, "quality", 0, result)` creates `issues/42/quality-review.json`
- **Retry result written**: `WriteRetryResult(42, 1, result)` creates `issues/42/retries/1/retry.json`
- **Outcome written**: `WriteOutcome(outcome)` creates `issues/42/outcome.json`
- **Debug log location**: Logger writes to `<run-dir>/debug.log`
- **Owner/repo path parsing**: `owner/repo` string is split correctly into directory components
- **Timestamp format**: Directory name matches `YYYYMMDD-HHMMSS` pattern

---

## Issue 75: Review telemetry capture

**Blocked by**: #74

### Description

Enrich the data written to the run directory with timing information that
isn't currently tracked. The `Result` struct already captures cost, tool
trace, verdict, and session ID from the agent runner. What's missing is
wall-clock duration per agent invocation.

Add start/end timestamps and duration tracking around each `agent.Run()` call
in the orchestrator loop, and pass the timing data to the `RunDataWriter`
alongside the `Result`.

### Key constraints

- Add `StartedAt time.Time` and `FinishedAt time.Time` fields to the
  `Result` struct, populated by `Run()` in `launcher.go` (wrap the actual
  execution with `time.Now()` calls)
- Duration is computed as `FinishedAt.Sub(StartedAt)` — not stored
  separately, derived when writing JSON
- The `RunDataWriter` methods accept `*Result` directly and serialize the
  relevant fields
- No changes to `agent_runner.py` — timing is measured on the Go side
  (which includes container startup overhead, giving a truer picture)
- Quality review files store an array of review results (one per cycle) since
  quality review can run multiple times before passing

### Acceptance criteria

- [ ] `Result` struct includes `StartedAt` and `FinishedAt` fields
- [ ] `Run()` populates timing fields around agent execution
- [ ] Duration is computed and written to JSON output files
- [ ] Quality review JSON stores an array for multi-cycle reviews
- [ ] `go test ./...` passes

### Test cases

- **Timing populated**: After `Run()`, `Result.StartedAt` and `Result.FinishedAt` are non-zero
- **Duration positive**: `FinishedAt.Sub(StartedAt)` is positive
- **Quality review array**: Multiple quality review results serialize as a JSON array
- **Timed-out runs**: `Result.TimedOut == true` still has valid timing fields

---

## Issue 76: Review quality reporting

**Blocked by**: #75

### Description

Analyze review telemetry data and produce quality assessments that the
dashboard can display. This is a reporting layer — it flags potentially
shallow reviews but does not reject them or fail the run.

Quality signals to report on:

- **Cost floor**: Flag reviews where `cost_usd` is below a configurable
  threshold. Very cheap reviews may indicate the reviewer didn't do thorough
  work.
- **Tool trace analysis**: Flag reviews where the reviewer didn't read the PR
  diff (no `Bash: gh pr diff` or `Read` calls), or didn't run tests (no
  `Bash: go test` or equivalent).
- **Review body quality**: Flag reviews where the reviewer's PR comment is
  very short or lacks file path references.
- **Duration floor**: Flag reviews that completed unusually fast.

These flags are written to the review JSON files as a `quality_flags` array
so the dashboard can highlight them.

### Key constraints

- New package: `internal/quality/` — contains analysis functions that take a
  `Result` and return `[]QualityFlag`
- `QualityFlag` struct:
  ```go
  type QualityFlag struct {
      Code    string // "low_cost", "no_diff_read", "no_tests_run", "short_duration", "shallow_review"
      Message string // human-readable explanation
  }
  ```
- Analysis functions:
  - `CheckCostFloor(result *agent.Result, threshold float64) *QualityFlag`
  - `CheckToolTrace(result *agent.Result) []QualityFlag`
  - `CheckDuration(result *agent.Result, threshold time.Duration) *QualityFlag`
- Thresholds are configurable in `godark.yaml` under a new `quality:` block:
  ```yaml
  quality:
    min_review_cost_usd: 0.10
    min_review_duration_seconds: 60
  ```
  Defaults: `min_review_cost_usd: 0.0` (disabled), `min_review_duration_seconds: 0` (disabled).
  When set to 0, the corresponding check is skipped.
- Quality flags are computed after each review step in `ProcessIssue` and
  passed to `RunDataWriter` to include in the review JSON files
- Quality flags do NOT affect the run outcome — a flagged review still counts
  as approved if the verdict is `APPROVED`
- Log a warning when quality flags are raised so they appear in stdout

### Acceptance criteria

- [ ] `internal/quality/` package with analysis functions
- [ ] Cost floor check flags reviews below configured threshold
- [ ] Tool trace check flags reviews missing diff reads or test runs
- [ ] Duration check flags reviews below configured threshold
- [ ] Quality flags are written to review JSON files
- [ ] Quality flags are logged as warnings to stdout
- [ ] Flags do not affect run outcomes (no enforcement)
- [ ] `quality:` config block with configurable thresholds
- [ ] Zero thresholds disable the corresponding check
- [ ] `go test ./...` passes

### Test cases

- **Low cost flagged**: Review with `cost_usd: 0.02` and threshold `0.10` produces `low_cost` flag
- **Cost above threshold**: Review with `cost_usd: 0.50` and threshold `0.10` produces no flag
- **No diff read**: Tool trace without any `Read` or `Bash: gh pr diff` produces `no_diff_read` flag
- **No tests run**: Tool trace without any `Bash: go test` or `Bash: npm test` produces `no_tests_run` flag
- **Normal trace**: Tool trace with diff reads and test runs produces no flags
- **Short duration**: 30-second review with 60-second threshold produces `short_duration` flag
- **Zero threshold disabled**: Threshold of `0.0` skips the check entirely
- **Flags in JSON**: Quality flags serialize into review JSON `quality_flags` array
- **Config parsing**: `quality:` block parsed from YAML with correct defaults

---

## Issue 77: Review test execution reporting

**Blocked by**: #75

### Description

Report on whether the functional reviewer created and ran ephemeral tests in
`tests/review/`. This is a quality flag — it highlights reviews that skipped
test creation but does not override the verdict or fail the run.

This is verified by inspecting the `ToolTrace` from the reviewer's `Result`.
The trace is checked for evidence of writing to `tests/review/` and running
the test command.

### Key constraints

- New function in `internal/quality/`:
  `CheckReviewTestExecution(result *agent.Result, reviewDir, testCommand string) []QualityFlag`
  Returns flags for missing test creation or missing test execution
- Check for Write/Edit to a path under `reviewDir` in the tool trace
- Check for Bash with `testCommand` substring in the tool trace
- Called alongside the other quality checks after each review step
- Produces `no_review_tests_written` and/or `no_review_tests_run` flags
- The quality reviewer is exempt — only the functional reviewer is checked
  (the quality reviewer focuses on code style, not behavioral verification)
- Flags are written to the review JSON and logged as warnings, same as other
  quality flags

### Acceptance criteria

- [ ] Tool trace is checked for Write/Edit to `tests/review/` paths
- [ ] Tool trace is checked for Bash with test command execution
- [ ] Missing test creation produces `no_review_tests_written` flag
- [ ] Missing test execution produces `no_review_tests_run` flag
- [ ] Quality flags are written to review JSON and logged as warnings
- [ ] Quality reviewer is exempt from this check
- [ ] Flags do not affect run outcomes (no enforcement)
- [ ] `go test ./...` passes

### Test cases

- **Tests written and run**: Trace with `Write tests/review/foo_test.go` and `Bash: go test ./tests/review/...` produces no flags
- **Tests written but not run**: Trace with `Write tests/review/foo_test.go` but no test command produces `no_review_tests_run` flag
- **Tests run but not written**: Trace with `Bash: go test` but no Write to `tests/review/` produces `no_review_tests_written` flag
- **Neither**: Empty trace produces both flags
- **Quality reviewer exempt**: Quality reviewer with no test trace produces no flags
- **Flags in JSON**: Flags serialize into review JSON `quality_flags` array alongside other quality flags

---

## Issue 78: Dashboard server and run list

**Blocked by**: #74

### Description

`godark status` starts a local web server that serves a dashboard UI. The
homepage shows a list of all runs across all repos, most recent first. Each
run shows summary stats and review quality flags.

The server reads from `~/.godark/runs/` — no database, no separate data
store. It walks the directory tree and renders Go templates with htmx for
interactivity and Alpine.js for client-side state where needed.

### Key constraints

- New package: `internal/dashboard/` — HTTP server, handlers, templates
- New command: `internal/cmd/status.go` — `godark status` Cobra command
- Templates and static assets (CSS, JS) embedded via `//go:embed` in
  `internal/dashboard/assets/`
- Dependencies: htmx and Alpine.js vendored as static files (no CDN, no npm)
- Server binds to `localhost:8374` by default, configurable via
  `--port` flag
- Opens the browser automatically on start (best-effort, non-fatal if it fails)
- Routes:
  - `GET /` — run list (all repos)
  - `GET /runs/<owner>/<repo>/<timestamp>` — run detail (per-issue outcomes)
  - `GET /runs/<owner>/<repo>/<timestamp>/issues/<number>` — issue detail
  - Static asset routes for CSS/JS
- Run list page displays:
  - Timestamp (human-readable, with relative time like "2 hours ago")
  - Repo name
  - Milestone or "single issue"
  - Issue count
  - Pass/fail/skip summary
  - Total cost
  - Quality flag count (if any reviews were flagged)
- Sorting: most recent first (derived from directory name timestamp)
- Filtering: by repo (htmx-driven, no full page reload)
- Graceful shutdown on SIGINT/SIGTERM
- No authentication (localhost only)

### Acceptance criteria

- [ ] `godark status` starts a local HTTP server
- [ ] Homepage lists all runs from `~/.godark/runs/`
- [ ] Runs are sorted most recent first
- [ ] Each run shows timestamp, repo, milestone, issue count, pass/fail, cost
- [ ] Quality flags are surfaced in the run list
- [ ] Filtering by repo works without page reload
- [ ] Server shuts down gracefully on interrupt
- [ ] Static assets are embedded in the binary
- [ ] `go test ./...` passes

### Test cases

- **Server starts**: `godark status` binds to localhost and responds to HTTP requests
- **Run list populated**: Given runs in `~/.godark/runs/`, GET `/` returns HTML with run entries
- **Empty state**: No runs directory returns a helpful empty state message
- **Run sorting**: Runs appear most recent first based on timestamp directory name
- **Repo filter**: htmx request with repo filter returns filtered run list
- **Static assets served**: CSS and JS files return correct content types
- **Graceful shutdown**: SIGINT causes clean server shutdown

---

## Issue 79: Issue detail view and log viewer

**Blocked by**: #78

### Description

Add drill-down views to the dashboard: run detail shows per-issue outcomes,
issue detail shows the full review chain with telemetry, and a log viewer
shows parsed debug log entries with filtering.

### Key constraints

- Run detail page (`/runs/<owner>/<repo>/<timestamp>`):
  - Lists all issues in the run with status, PR number, retry count, cost
  - Color-coded status (green for implemented, red for failed, yellow for
    needs-human-review)
  - Link to GitHub PR for each issue (constructed from repo + PR number)
  - Quality flag indicators per issue
  - Click through to issue detail
- Issue detail page (`/runs/<owner>/<repo>/<timestamp>/issues/<number>`):
  - Timeline view of the review chain: implement → QA review (cycles) →
    retries → functional review
  - Each step shows: duration, cost, verdict, quality flags, tool trace
    summary
  - Expandable tool trace (collapsed by default, Alpine.js toggle)
  - Links to GitHub PR and issue
- Log viewer (`/runs/<owner>/<repo>/<timestamp>/logs`):
  - Parsed `debug.log` JSON lines displayed in a table
  - Columns: timestamp, level, message, structured fields
  - Filtering by log level (htmx-driven)
  - Search within log messages
  - Paginated (load more via htmx, not full page load)
- All pages include breadcrumb navigation (runs → run → issue)

### Acceptance criteria

- [ ] Run detail page shows per-issue outcomes with status, PR, retries, cost
- [ ] Issues link to GitHub PRs
- [ ] Quality flags are visible per issue
- [ ] Issue detail shows the full review chain as a timeline
- [ ] Each step in the timeline shows duration, cost, verdict, tool trace
- [ ] Tool trace is expandable/collapsible
- [ ] Log viewer parses and displays debug.log entries
- [ ] Log level filtering works
- [ ] Log search works
- [ ] Breadcrumb navigation works across all pages
- [ ] `go test ./...` passes

### Test cases

- **Run detail populated**: Given run data with 2 issues, GET run detail returns HTML with both issues
- **Status color coding**: Implemented issues render with success styling, failed with error styling
- **PR links**: Issue with PR #57 in repo `owner/repo` links to `https://github.com/owner/repo/pull/57`
- **Issue timeline**: Issue with implement + 2 QA cycles + 1 retry + functional review shows all steps in order
- **Tool trace toggle**: Alpine.js toggle shows/hides tool trace details
- **Log parsing**: Debug log with 100 JSON lines renders as a table with correct columns
- **Log level filter**: Filtering to "error" shows only error-level entries
- **Log search**: Searching "issue_number" filters to matching log lines
- **Breadcrumbs**: Issue detail page shows `Runs > owner/repo > 20260304-103000 > Issue #42`
