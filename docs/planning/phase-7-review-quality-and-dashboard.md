# Phase 7: Review Quality & Dashboard

> **Goal:** Capture review telemetry, report on review quality metrics, and
> surface it all in a local web dashboard for human spot-checking.

## Milestone

`Phase 7`

---

## Issue 94: RunDataWriter package

### Description

Create the `internal/rundata/` package with types, directory creation, and all
write methods. This is pure new code — no existing files are modified.

The package manages a per-run directory under
`~/.godark/runs/<owner>/<repo>/<YYYYMMDD-HHMMSS>/` and provides methods to
write JSON files for each step of the agent loop.

### Key constraints

- New package: `internal/rundata/`
- Exported types:
  ```go
  type Writer struct { ... }

  type RunMeta struct {
      StartedAt      time.Time         `json:"started_at"`
      FinishedAt     *time.Time        `json:"finished_at"`
      Repo           string            `json:"repo"`
      Milestone      string            `json:"milestone"`
      IssueNumbers   []int             `json:"issue_numbers"`
      ConfigSnapshot map[string]any    `json:"config_snapshot"`
      Summary        *RunSummary       `json:"summary"`
  }

  type RunSummary struct {
      Total        int     `json:"total"`
      Implemented  int     `json:"implemented"`
      Failed       int     `json:"failed"`
      Skipped      int     `json:"skipped"`
      TotalCostUSD float64 `json:"total_cost_usd"`
  }

  type StepResult struct {
      StartedAt       string   `json:"started_at"`
      FinishedAt      string   `json:"finished_at"`
      DurationSeconds float64  `json:"duration_seconds"`
      CostUSD         float64  `json:"cost_usd"`
      SessionID       string   `json:"session_id"`
      Verdict         string   `json:"verdict,omitempty"`
      ToolTrace       []string `json:"tool_trace,omitempty"`
      TimedOut        bool     `json:"timed_out"`
      ExitCode        int      `json:"exit_code"`
  }

  type Outcome struct {
      IssueNumber  int     `json:"issue_number"`
      IssueTitle   string  `json:"issue_title"`
      Status       string  `json:"status"`
      PRNumber     int     `json:"pr_number"`
      Retries      int     `json:"retries"`
      TotalCostUSD float64 `json:"total_cost_usd"`
      Error        string  `json:"error,omitempty"`
  }
  ```
- `New(repo, milestone string, issueNumbers []int) (*Writer, error)` —
  creates the run directory, writes initial `run.json`, returns the writer.
  Uses `os.UserHomeDir()` for the base path.
- Repo string is validated: reject components containing `..` or path
  separators to prevent directory traversal
- `Dir() string` — returns the run directory path (needed by logger)
- `WriteImplementResult(issueNumber int, step StepResult) error`
- `WriteReviewResult(issueNumber int, kind string, step StepResult) error` —
  `kind` must be `"quality"` or `"functional"`, returns error otherwise
- `WriteRetryResult(issueNumber int, attempt int, step StepResult) error`
- `WriteRetryReviewResult(issueNumber int, attempt int, step StepResult) error`
- `WriteOutcome(outcome Outcome) error`
- `FinalizeRun(summary RunSummary) error` — updates `run.json` with
  `finished_at` and `summary`
- Directory structure:
  ```
  ~/.godark/runs/<owner>/<repo>/<YYYYMMDD-HHMMSS>/
    run.json
    issues/
      <issue-number>/
        outcome.json
        implement.json
        quality-review.json
        functional-review.json
        retries/
          <n>/
            retry.json
            quality-review.json
  ```
- `StepResult` does not include timing fields until the telemetry issue adds
  `StartedAt`/`FinishedAt` to `agent.Result`. For now, the caller populates
  `StepResult` manually — no dependency on `agent.Result` in this package.

### Acceptance criteria

- [ ] `New()` creates `~/.godark/runs/<owner>/<repo>/<timestamp>/` directory
- [ ] `run.json` is written at creation with `started_at`, `repo`, `milestone`
- [ ] `FinalizeRun()` updates `run.json` with `finished_at` and `summary`
- [ ] All write methods create the correct file paths
- [ ] Repo components with `..` are rejected

### Test cases

- **Run directory created**: `New()` creates the expected directory under a temp base
- **Run.json at start**: After `New()`, `run.json` has `started_at`, `repo`, `issue_numbers`
- **Run.json finalized**: After `FinalizeRun()`, `run.json` has `finished_at` and `summary`
- **Implement written**: `WriteImplementResult(42, step)` creates `issues/42/implement.json`
- **Review written**: `WriteReviewResult(42, "quality", step)` creates `issues/42/quality-review.json`
- **Review kind validated**: `WriteReviewResult(42, "bad", step)` returns error
- **Retry written**: `WriteRetryResult(42, 1, step)` creates `issues/42/retries/1/retry.json`
- **Outcome written**: `WriteOutcome(outcome)` creates `issues/42/outcome.json`
- **Path traversal rejected**: `New("../evil/../../path", ...)` returns error
- **Timestamp format**: Directory name matches `YYYYMMDD-HHMMSS` pattern

---

## Issue 96: Wire RunDataWriter into agent loop

**Blocked by**: #94

### Description

Integrate the `RunDataWriter` into the orchestrator and implement commands.
Define a `RunDataHook` interface in the agent package so `ProcessIssue` can
optionally record step results without a hard dependency on `rundata`.

### Key constraints

- New interface in `internal/agent/runhook.go`:
  ```go
  type RunDataHook interface {
      WriteImplementResult(issueNumber int, step rundata.StepResult) error
      WriteReviewResult(issueNumber int, kind string, step rundata.StepResult) error
      WriteRetryResult(issueNumber int, attempt int, step rundata.StepResult) error
      WriteRetryReviewResult(issueNumber int, attempt int, step rundata.StepResult) error
      WriteOutcome(outcome rundata.Outcome) error
  }
  ```
- Add `Hook RunDataHook` field to the arguments passed to `ProcessIssue`
  (nil-safe — all call sites check `if hook != nil` before writing)
- In `ProcessIssue` (`loop.go`): after each agent step, build a `StepResult`
  from the `agent.Result` and call the appropriate hook method
- In `orchestrator.go`: create `RunDataWriter` before the processing loop,
  pass it as the hook, call `FinalizeRun` at the end
- In `implement.go`: same pattern for single-issue runs
- Helper function `ResultToStep(r *Result) StepResult` converts an
  `agent.Result` to a `rundata.StepResult` — timing fields are zero until
  the telemetry issue adds them

### Acceptance criteria

- [ ] `RunDataHook` interface defined
- [ ] `ProcessIssue` calls hook methods after each step (nil-safe)
- [ ] `orchestrator.go` creates writer and passes it through
- [ ] `implement.go` creates writer and passes it through
- [ ] `FinalizeRun` is called at end of run

### Test cases

- **Hook called on implement**: Mock hook verifies `WriteImplementResult` called after implementer
- **Hook called on review**: Mock hook verifies `WriteReviewResult` called after reviewer
- **Hook called on outcome**: Mock hook verifies `WriteOutcome` called with correct status
- **Nil hook safe**: `ProcessIssue` with nil hook does not panic
- **FinalizeRun called**: Orchestrator calls `FinalizeRun` with correct summary counts

---

## Issue 97: Migrate debug log to run directory

**Blocked by**: #96

### Description

Move the slog debug log from `logs/` into the run directory as `debug.log`.
Remove the `LogDir` config field. Handle the dry-run case where no run
directory exists.

### Key constraints

- Update `logging.NewLogger` to accept a directory path and create
  `debug.log` there (instead of using `LogDir`)
- In orchestrator and implement commands: pass `writer.Dir()` to `NewLogger`
  after creating the `RunDataWriter`
- Dry-run mode: use `os.MkdirTemp` for a private temp directory since no
  `RunDataWriter` is created. Do not write to `/tmp/debug.log` (shared
  path, race condition, security issue).
- Remove `LogDir` field from `config.Config`
- Remove `log_dir` from YAML schema and `defaults()`
- Delete the `logs/` entry from `.gitignore` if present

### Acceptance criteria

- [ ] Debug log writes to `<run-dir>/debug.log`
- [ ] `LogDir` removed from config
- [ ] Dry-run uses a private temp directory for logging
- [ ] `go test ./...` passes

### Test cases

- **Log in run dir**: After creating writer + logger, `debug.log` exists in `writer.Dir()`
- **Dry-run isolation**: Two concurrent dry-runs don't share a log path
- **Config without log_dir**: YAML without `log_dir` key loads without error
- **Logger writes**: Log entries appear in the debug.log file

---

## Issue 95: Agent result timing

### Description

Add wall-clock timing to `agent.Result` so run data files contain accurate
timestamps and durations. Currently `Result` has no timing fields.

### Key constraints

- Add `StartedAt time.Time` and `FinishedAt time.Time` fields to
  `agent.Result` in `launcher.go`
- Populate `StartedAt` with `time.Now()` before the agent invocation and
  `FinishedAt` with `time.Now()` after, in both `runHost` and `runSandbox`
- Update `ResultToStep` (from the wiring issue) to use these fields for
  accurate `started_at`, `finished_at`, and `duration_seconds` in the JSON
- No changes to `agent_runner.py` — timing is measured on the Go side
  (includes container startup, giving a truer wall-clock picture)

### Acceptance criteria

- [ ] `Result` struct includes `StartedAt` and `FinishedAt` fields
- [ ] `runHost` populates timing fields around execution
- [ ] `runSandbox` populates timing fields around execution
- [ ] `ResultToStep` computes accurate duration from timing fields

### Test cases

- **Timing populated**: After `Run()`, `Result.StartedAt` and `Result.FinishedAt` are non-zero
- **Duration positive**: `FinishedAt.Sub(StartedAt)` is positive
- **Timed-out runs**: `Result.TimedOut == true` still has valid timing fields
- **ResultToStep conversion**: `ResultToStep` with populated timing produces correct `duration_seconds`

---

## Issue 99: Quality flag analysis package

**Blocked by**: #95

### Description

Create the `internal/quality/` package with analysis functions that inspect
an `agent.Result` and return quality flags. Pure new code with no existing
file modifications.

### Key constraints

- New package: `internal/quality/`
- Types:
  ```go
  type Flag struct {
      Code    string `json:"code"`
      Message string `json:"message"`
  }
  ```
- Analysis functions:
  - `CheckCostFloor(costUSD, threshold float64) *Flag` — returns `low_cost`
    flag if cost is below threshold. Returns nil if threshold is 0 (disabled).
  - `CheckDuration(duration time.Duration, threshold time.Duration) *Flag` —
    returns `short_duration` flag. Returns nil if threshold is 0.
  - `CheckToolTrace(toolTrace []string) []Flag` — returns `no_diff_read`
    and/or `no_tests_run` flags based on trace contents
  - `CheckReviewTestExecution(toolTrace []string, reviewDir, testCommand string) []Flag`
    — returns `no_review_tests_written` and/or `no_review_tests_run` flags
- Functions take primitive values (not `*agent.Result`) to avoid a dependency
  on the agent package from the quality package

### Acceptance criteria

- [ ] `CheckCostFloor` flags reviews below threshold
- [ ] `CheckDuration` flags reviews below threshold
- [ ] `CheckToolTrace` flags missing diff reads and test runs
- [ ] `CheckReviewTestExecution` flags missing test creation and execution
- [ ] Zero thresholds disable the corresponding check

### Test cases

- **Low cost flagged**: `cost=0.02, threshold=0.10` produces `low_cost` flag
- **Cost above threshold**: `cost=0.50, threshold=0.10` produces no flag
- **Cost check disabled**: `threshold=0.0` returns nil
- **No diff read**: Trace without `Read` or `gh pr diff` produces `no_diff_read`
- **No tests run**: Trace without `go test` or `npm test` produces `no_tests_run`
- **Normal trace**: Trace with diff reads and test runs produces no flags
- **Short duration**: 30s with 60s threshold produces `short_duration`
- **Duration check disabled**: `threshold=0` returns nil
- **Tests written and run**: Trace with Write to review dir and test command produces no flags
- **Neither written nor run**: Empty trace produces both flags

---

## Issue 100: Wire quality flags into config and agent loop

**Blocked by**: #99

### Description

Add the `quality:` config block and call the quality analysis functions after
each review step in `ProcessIssue`. Log flags as warnings and include them
in the run data.

### Key constraints

- Add to `config.Config`:
  ```go
  type Quality struct {
      MinReviewCostUSD         float64 `yaml:"min_review_cost_usd"`
      MinReviewDurationSeconds int     `yaml:"min_review_duration_seconds"`
  }
  ```
- Defaults: both 0 (disabled)
- In `ProcessIssue` after each review step: call the quality analysis
  functions, log any flags as `slog.Warn`, and pass them to the
  `RunDataHook` (extend the review write methods to accept `[]quality.Flag`)
- Quality reviewer is exempt from `CheckReviewTestExecution` — only the
  functional reviewer is checked
- Flags do NOT affect run outcomes — a flagged review still counts as
  approved

### Acceptance criteria

- [ ] `quality:` config block parsed from YAML with correct defaults
- [ ] Quality flags computed after each review step
- [ ] Flags logged as warnings
- [ ] Flags included in run data review files
- [ ] Quality reviewer exempt from test execution check

### Test cases

- **Config parsing**: `quality: {min_review_cost_usd: 0.10}` parsed correctly
- **Config defaults**: Missing `quality:` block uses zero defaults
- **Flags logged**: Review below cost threshold produces warning log
- **Flags in run data**: Review flags appear in written JSON
- **QA reviewer exempt**: Quality reviewer skips test execution check

---

## Issue 98: Run data reader

**Blocked by**: #96

### Description

Create a reader that loads run data from `~/.godark/runs/` for the dashboard
to consume. Walks the directory tree and deserializes JSON files into Go
structs.

### Key constraints

- New file: `internal/rundata/reader.go`
- `ListRuns() ([]RunMeta, error)` — walks `~/.godark/runs/`, reads each
  `run.json`, returns sorted most-recent-first by `started_at`
- `LoadRun(owner, repo, timestamp string) (*RunDetail, error)` — reads
  `run.json` plus all issue subdirectories
- `RunDetail` struct includes `RunMeta` plus `[]IssueDetail`
- `IssueDetail` aggregates outcome, implement, reviews, retries for one issue
- Graceful handling of missing or corrupt files (skip with warning, don't crash)

### Acceptance criteria

- [ ] `ListRuns` returns all runs sorted most-recent-first
- [ ] `LoadRun` returns full run detail with all issue data
- [ ] Missing files are skipped gracefully
- [ ] Corrupt JSON is skipped with a logged warning

### Test cases

- **List runs**: Given 3 run directories, `ListRuns` returns 3 entries sorted by time
- **Empty state**: No runs directory returns empty slice, no error
- **Load run**: `LoadRun` with a complete run directory returns all issue data
- **Missing outcome**: Issue directory without `outcome.json` is included with zero-value outcome
- **Corrupt JSON**: Malformed `run.json` is skipped, other runs still returned

---

## Issue 101: Dashboard server and run list page

**Blocked by**: #98

### Description

`godark status` starts a local web server that serves a dashboard UI. The
homepage shows a list of all runs, most recent first.

### Key constraints

- New package: `internal/dashboard/` — HTTP server, handlers, templates
- Update existing `internal/cmd/status.go` — change from log-parsing to
  starting the web server
- Templates and static assets embedded via `//go:embed`
- htmx and Alpine.js vendored as static files (no CDN)
- Server binds to `localhost:8374`, configurable via `--port`
- Opens browser on start (best-effort)
- Routes: `GET /` (run list), static asset routes
- Run list displays: timestamp, repo, milestone, issue count, pass/fail,
  cost, quality flag count
- Sorting: most recent first
- Filtering by repo (htmx, no full page reload)
- Graceful shutdown on SIGINT/SIGTERM

### Acceptance criteria

- [ ] `godark status` starts HTTP server
- [ ] Homepage lists runs from `~/.godark/runs/`
- [ ] Runs sorted most recent first
- [ ] Each run shows timestamp, repo, milestone, issue count, pass/fail, cost
- [ ] Static assets embedded in binary

### Test cases

- **Server starts**: `godark status` binds and responds to HTTP requests
- **Run list populated**: GET `/` returns HTML with run entries
- **Empty state**: No runs directory returns helpful empty state
- **Run sorting**: Runs appear most recent first
- **Static assets served**: CSS and JS return correct content types

---

## Issue 102: Run detail and issue detail pages

**Blocked by**: #101

### Description

Add drill-down views: run detail shows per-issue outcomes, issue detail shows
the full review chain with telemetry.

### Key constraints

- Run detail page (`/runs/<owner>/<repo>/<timestamp>`):
  - Per-issue list with status, PR number, retry count, cost
  - Color-coded status (green/red/yellow)
  - Links to GitHub PR
  - Quality flag indicators
  - Click through to issue detail
- Issue detail page (`/runs/<owner>/<repo>/<timestamp>/issues/<number>`):
  - Timeline: implement → QA review cycles → retries → functional review
  - Each step shows duration, cost, verdict, quality flags, tool trace
  - Expandable tool trace (Alpine.js toggle)
  - Links to GitHub PR and issue
- Breadcrumb navigation

### Acceptance criteria

- [ ] Run detail page shows per-issue outcomes
- [ ] Issues link to GitHub PRs
- [ ] Issue detail shows review chain timeline
- [ ] Tool trace is expandable/collapsible
- [ ] Breadcrumb navigation works

### Test cases

- **Run detail populated**: GET run detail returns HTML with issue entries
- **Status color coding**: Implemented = success style, failed = error style
- **PR links**: Issue with PR #57 links to correct GitHub URL
- **Issue timeline**: All steps shown in order
- **Tool trace toggle**: Alpine.js toggle shows/hides trace

---

## Issue 103: Log viewer

**Blocked by**: #101

### Description

Add a log viewer page that displays parsed `debug.log` entries with filtering
and search.

### Key constraints

- Route: `GET /runs/<owner>/<repo>/<timestamp>/logs`
- Parse `debug.log` JSON lines into a table
- Columns: timestamp, level, message, structured fields
- Level filtering (htmx-driven)
- Search within log messages
- Paginated (load more via htmx)
- Breadcrumb navigation

### Acceptance criteria

- [ ] Log viewer parses and displays debug.log entries
- [ ] Log level filtering works
- [ ] Log search works
- [ ] Pagination loads more entries without full reload

### Test cases

- **Log parsing**: 100 JSON lines render as table with correct columns
- **Level filter**: Filtering to "error" shows only error entries
- **Search**: Searching "issue_number" filters to matching lines
- **Pagination**: Initial load shows first page, "load more" fetches next batch
- **Breadcrumbs**: Page shows correct breadcrumb trail
