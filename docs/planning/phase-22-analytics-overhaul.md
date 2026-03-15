# Phase 22: Analytics Overhaul

> **Goal:** The `godark analyze` command and dashboard analytics page surface
> actionable metrics that answer five operator questions: is the system
> improving, where is money going, where is time going, what's failing and why,
> and what did we ship. First-pass success rate, wasted cost, failure reason
> breakdown, and per-repo efficiency replace low-value metrics. A new
> `godark report` command generates sprint-scoped summaries for engineering
> managers.

## Milestone

`Phase 22`

---

## Issue 489: First-pass success rate and overview metrics in analysis

### Description

Add first-pass success rate, average cost per successful issue, and wasted cost
to the `Report` struct in `internal/analysis/analysis.go`. First-pass success
rate measures the percentage of issues that succeed without any retries — the
purest signal of harness quality. Wasted cost is the total spend on issues that
ultimately failed. Avg cost per success normalizes cost against actual output.

Also enrich `RepoSummary` with first-pass rate and avg cost per issue for
per-repo breakdown.

### Key constraints

- Modify `internal/analysis/analysis.go`:
  - Add to `Report`: `FirstPassSuccessRate float64`, `FirstPassCount int`,
    `WastedCostUSD float64`, `AvgCostPerSuccessUSD float64`
  - Add to `RepoSummary`: `FirstPassRate float64`, `AvgCostPerIssueUSD float64`
  - In `Aggregate()`: an issue is "first-pass" if it has zero retry steps and
    status is `implemented` or `ready-to-merge`
  - `WastedCostUSD` = sum of all step costs for issues with status `failed` or
    `needs-human-review`
  - `AvgCostPerSuccessUSD` = total cost / count of implemented issues (0.0 if
    none implemented)
  - Per-repo first-pass rate and avg cost computed in `accumulateRepoStat()`

### Acceptance criteria

- [ ] `Report.FirstPassSuccessRate` computed correctly
- [ ] `Report.FirstPassCount` counts issues with zero retries and successful
  outcome
- [ ] `Report.WastedCostUSD` sums cost of failed/needs-human-review issues
- [ ] `Report.AvgCostPerSuccessUSD` divides total cost by implemented count
- [ ] `RepoSummary` includes `FirstPassRate` and `AvgCostPerIssueUSD`

### Test cases

- **All first-pass**: 5 issues, 0 retries each, all implemented —
  `FirstPassSuccessRate` is 1.0, `FirstPassCount` is 5
- **Mixed retries**: 3 issues succeed first-pass, 2 retry and succeed —
  `FirstPassSuccessRate` is 0.6 (3/5)
- **Wasted cost**: 2 failed issues with $1.50 total cost — `WastedCostUSD`
  is 1.50
- **No successes**: All issues fail — `AvgCostPerSuccessUSD` is 0.0 (no
  division by zero)
- **Per-repo first-pass**: Repo-a has 3/4 first-pass, repo-b has 1/3 —
  rates computed independently

---

## Issue 490: Failure reason breakdown and timeout rate in analysis

### Description

Add failure categorization and timeout rate to the `Report` struct.
Failures are categorized into four buckets: verify failure (failed during
build/lint/test), review exhaustion (exhausted max retries), timeout (agent
hit the timeout), and error (other failures). Timeout rate is the percentage
of all steps across all issues that timed out.

Also add average time-to-merge: the end-to-end duration from first step start
to final outcome for implemented issues.

### Key constraints

- Modify `internal/analysis/analysis.go`:
  - Add to `Report`: `FailureReasons map[string]int` with keys `"verify"`,
    `"exhaustion"`, `"timeout"`, `"error"`
  - Add to `Report`: `TimeoutRate float64` (timed-out steps / total steps)
  - Add to `DurationStats`: `AvgTimeToMergeSeconds float64`
  - Failure categorization logic in `Aggregate()`:
    - `"verify"` — issue has a verify step with `AllPassed == false` as the
      last step before failure
    - `"exhaustion"` — issue status is `needs-human-review` (retries exhausted)
    - `"timeout"` — any step result has `TimedOut == true` or duration >=
      agent timeout threshold
    - `"error"` — all other failures
  - Time-to-merge: for implemented issues, compute duration from earliest
    step `StartedAt` to latest step `FinishedAt`
  - Timeout detection: check step results for timeout signals; count as
    fraction of total steps

### Acceptance criteria

- [ ] `FailureReasons` populated with correct counts per category
- [ ] Categories are mutually exclusive (each failed issue counted once)
- [ ] `TimeoutRate` is 0.0 when no steps timed out
- [ ] `AvgTimeToMergeSeconds` computed from implemented issues only
- [ ] `AvgTimeToMergeSeconds` is 0.0 when no issues are implemented

### Test cases

- **Verify failure**: Issue fails with verify step `AllPassed == false` —
  counted as `"verify"`
- **Exhaustion**: Issue status `needs-human-review` — counted as `"exhaustion"`
- **Timeout**: Issue has a step with `TimedOut == true` — counted as
  `"timeout"`
- **Error fallback**: Issue fails with none of the above — counted as `"error"`
- **No failures**: All issues succeed — `FailureReasons` is empty map
- **Time to merge**: 3 implemented issues with durations 300s, 600s, 900s —
  `AvgTimeToMergeSeconds` is 600.0
- **Timeout rate**: 2 of 20 total steps timed out — `TimeoutRate` is 0.10

---

## Issue 493: First-pass success rate trend

**Blocked by**: #489

### Description

Add `FirstPassSuccessRate` to `TrendPoint` so the dashboard can render a
first-pass success rate trend line alongside the existing overall success rate.

### Key constraints

- Modify `internal/analysis/trends.go`:
  - Add `FirstPassSuccessRate float64` to `TrendPoint`
  - In `ComputeTrends()`: for each run, count issues with zero retries and
    successful outcome, divide by total issues in that run

### Acceptance criteria

- [ ] `TrendPoint.FirstPassSuccessRate` computed per run
- [ ] Value is 0.0 when no issues in the run are first-pass successes
- [ ] Value is 1.0 when all issues succeed without retries

### Test cases

- **All first-pass run**: 5 issues, 0 retries, all implemented —
  `FirstPassSuccessRate` is 1.0
- **Mixed run**: 3 first-pass, 2 retried — `FirstPassSuccessRate` is 0.6
- **Multiple runs**: Each run has its own independent rate
- **Empty run**: Run with 0 finished issues — rate is 0.0

---

## Issue 491: Drop scenario spec gap and exhausted retries listing

### Description

Remove the scenario spec presence/absence condition and the exhausted retries
listing from `DetectGaps()`. Scenario specs are now expected on every issue, so
the "with/without" comparison is noise. Exhausted retries are better served by
the new failure reason breakdown in `Report.FailureReasons`.

### Key constraints

- Modify `internal/analysis/gaps.go`:
  - Remove the scenario spec gap condition (the block that checks for
    `SpecGenerator` step presence)
  - Remove the exhausted retries listing (the block that lists issue numbers
    with `needs-human-review` status)
  - Keep the flag-based correlation detection (unchanged)
- Update `internal/analysis/gaps_test.go`:
  - Remove tests for scenario spec gap
  - Remove tests for exhausted retries listing
  - Existing flag correlation tests unchanged

### Acceptance criteria

- [ ] `DetectGaps()` no longer returns a gap for scenario spec presence
- [ ] `DetectGaps()` no longer returns an exhausted retries listing
- [ ] Flag-based correlation gaps still work correctly
- [ ] Tests updated to reflect removed conditions

### Test cases

- **No scenario gap**: Issues with and without spec generator steps — no gap
  referencing "scenario" in output
- **No exhausted listing**: Issues with `needs-human-review` status — no gap
  listing their issue numbers
- **Flags still work**: Issues with `no_diff_read` flag — gap correctly shows
  failure rate correlation

---

## Issue 494: Update analyze CLI output with new metrics

**Blocked by**: #489, #490, #491

### Description

Update `godark analyze` to display all new metrics: overview section with
first-pass rate and wasted cost, failure reason breakdown table, time-to-merge
summary, and enriched per-repo table with first-pass rate and avg cost columns.
Update the JSON output struct to include all new fields.

### Key constraints

- Modify `internal/cmd/analyze.go`:
  - Add overview section at the top: first-pass rate, avg cost per success,
    wasted cost, timeout rate
  - Add failure reasons table: reason, count, percentage
  - Add time-to-merge line in duration section
  - Update repo table columns: add first-pass rate, avg cost per issue
  - Update `analyzeOutput` struct for `--json` to include new Report fields
  - Remove exhausted count from retry stats display (now in failure reasons)

### Acceptance criteria

- [ ] Overview section displays first-pass rate, wasted cost, avg cost per
  success, timeout rate
- [ ] Failure reasons table displays all four categories with counts and
  percentages
- [ ] Time-to-merge average displayed in duration section
- [ ] Per-repo table includes first-pass rate and avg cost columns
- [ ] `--json` output includes all new Report fields

### Test cases

- **Overview renders**: `godark analyze` output contains "First-pass success
  rate:" line
- **Failure reasons render**: Output contains a failure reasons table with
  "verify", "exhaustion", "timeout", "error" rows (only non-zero rows shown)
- **Time to merge renders**: Output contains "Avg time to merge:" line
- **Repo table enriched**: Repo table has first-pass rate and avg cost columns
- **JSON includes new fields**: `--json` output deserializes with
  `FirstPassSuccessRate`, `WastedCostUSD`, `FailureReasons` fields

---

## Issue 495: Update dashboard analysis view models and handlers

**Blocked by**: #489, #490, #491

### Description

Add new view model types and extend `AnalysisData` to carry the new metrics
to the dashboard template. Build helper functions that transform `Report`
fields into template-ready view models.

### Key constraints

- Modify `internal/dashboard/handlers.go`:
  - Add `OverviewMetrics` struct: `TotalRuns int`, `TotalIssues int`,
    `SuccessRate float64`, `FirstPassRate float64`, `TotalCostUSD float64`,
    `AvgCostPerSuccessUSD float64`, `WastedCostUSD float64`,
    `TimeoutRate float64`
  - Add `FailureReasonRow` struct: `Reason string`, `Count int`,
    `Percent float64`
  - Extend `RepoRow`: add `FirstPassRate float64`,
    `AvgCostPerIssueUSD float64`
  - Extend `AnalysisData`: add `Overview OverviewMetrics`,
    `FailureReasons []FailureReasonRow`, `AvgTimeToMerge string`
    (human-readable duration)
  - Add builder functions: `buildOverviewMetrics(report)`,
    `buildFailureReasonRows(report)`
  - Update `buildAnalysisDataFromDB()` and `buildAnalysisDataFromFS()` to
    populate new fields

### Acceptance criteria

- [ ] `OverviewMetrics` populated from Report
- [ ] `FailureReasonRow` slice built from `Report.FailureReasons`
- [ ] `RepoRow` includes first-pass rate and avg cost
- [ ] `AnalysisData` carries all new view models to the template
- [ ] Both DB and FS paths populate the new fields

### Test cases

- **Overview metrics built**: Given a Report with 10 runs and 50 issues,
  `buildOverviewMetrics()` returns correct totals and rates
- **Failure reasons sorted**: Rows sorted by count descending
- **Empty failure reasons**: No failures — empty slice, not nil
- **Repo rows enriched**: RepoRow for "org/repo" includes first-pass rate
  matching RepoSummary.FirstPassRate

---

## Issue 496: Update dashboard analysis HTML template

**Blocked by**: #495

### Description

Update the dashboard analysis page with new cards for overview metrics, failure
reason breakdown, and first-pass success rate trend chart. Modify existing cards
to show enriched per-repo data.

### Key constraints

- Modify `internal/dashboard/templates/analysis.html`:
  - Add overview metrics card at top of page: 6 metric tiles (total runs,
    total issues, success rate, first-pass rate, total cost, avg cost per
    success)
  - Add wasted cost and timeout rate to the cost statistics card
  - Add failure reason breakdown card with horizontal bar chart or table
  - Add time-to-merge to the duration section
  - Update repo table to include first-pass rate and avg cost columns
  - Add first-pass success rate line to the success rate trend chart
    (second dataset on existing chart)
  - Remove exhausted retries row from retry stats card

### Acceptance criteria

- [ ] Overview metrics card renders with 6 tiles
- [ ] Failure reason breakdown card visible with non-zero data
- [ ] First-pass success rate trend line appears on success rate chart
- [ ] Repo table shows first-pass rate and avg cost columns
- [ ] Page renders without error when all metrics are zero (empty state)

### Test cases

- **Overview card renders**: `/analysis` page contains overview metrics with
  total runs, first-pass rate
- **Failure reasons render**: Page contains failure reason entries when
  failures exist
- **Empty state**: Page with no run data shows zero-state for all cards
- **Trend chart has two lines**: Success rate chart renders both overall and
  first-pass lines

---

## Issue 492: godark report command scaffold

### Description

Add a new `godark report` Cobra subcommand that generates sprint-scoped
summaries from the SQLite stats database. This issue creates the command
skeleton, flag parsing, and date range resolution. The actual report content
and formatting is a separate issue.

### Key constraints

- New file `internal/cmd/report.go`:
  - `godark report` subcommand registered on root
  - Flags: `--since` (duration string like `2w`, `30d`, `7d`; default `2w`),
    `--until` (date string, default now), `--repo` (filter by repo),
    `--format` (terminal, markdown, html; default terminal)
  - Parse `--since` into a `time.Time` by subtracting the duration from
    `--until`
  - Open `~/.godark/stats.db`, query with the date range and repo filter
  - Pass results to a report renderer (stubbed in this issue)
  - Error if stats.db doesn't exist: "No stats database found. Run
    `godark run` or `godark implement` first."
- Duration parsing: support `Nd` (days) and `Nw` (weeks) suffixes

### Acceptance criteria

- [ ] `godark report` command exists and is registered
- [ ] `--since 2w` resolves to 14 days before now
- [ ] `--since 30d` resolves to 30 days before now
- [ ] `--repo` filters results to matching repository
- [ ] `--format` accepts `terminal`, `markdown`, `html`
- [ ] Missing stats.db produces a clear error message

### Test cases

- **Duration parsing**: `"2w"` → 14 days, `"30d"` → 30 days, `"7d"` → 7 days
- **Invalid duration**: `"abc"` → error
- **Repo filter**: Passed through to stats query
- **Missing database**: Error message mentions running godark first
- **Default format**: No `--format` flag uses terminal

---

## Issue 497: Report content and formatting

**Blocked by**: #492, #489, #490

### Description

Implement the report content generation and three output formats (terminal,
markdown, HTML). The report summarizes a sprint period with metrics designed
for engineering managers: what was accomplished, at what cost, and what needs
attention.

### Key constraints

- New file `internal/report/report.go` (or extend `internal/cmd/report.go`):
  - `SprintReport` struct: `Since time.Time`, `Until time.Time`,
    `Repo string` (empty = all), `TotalRuns int`, `IssuesProcessed int`,
    `IssuesImplemented int`, `IssuesFailed int`, `SuccessRate float64`,
    `FirstPassRate float64`, `TotalCostUSD float64`,
    `AvgCostPerSuccessUSD float64`, `WastedCostUSD float64`,
    `FailureReasons map[string]int`,
    `NotableFailures []NotableFailure` (issue number, title, error)
  - `Generate(runs, outcomes, steps) SprintReport` — computes metrics from
    the queried data
  - `RenderTerminal(report) string` — tabular output for the terminal
  - `RenderMarkdown(report) string` — markdown suitable for Slack/wiki
  - `RenderHTML(report) string` — styled HTML for email
- Terminal format: same tabwriter style as `godark analyze`
- Markdown format: headers, bullet lists, bold metrics, table for failures
- HTML format: simple inline-styled HTML (no external CSS dependencies)
- Notable failures: issues with status `failed` that had the highest cost
  (top 5)

### Acceptance criteria

- [ ] `SprintReport` computed from stats database queries
- [ ] Terminal format renders readable summary to stdout
- [ ] Markdown format produces valid markdown with metrics and tables
- [ ] HTML format produces self-contained styled HTML
- [ ] Notable failures list top 5 most expensive failures with issue numbers

### Test cases

- **Terminal output**: Report with 3 runs, 15 issues renders a readable table
- **Markdown output**: Contains `##` headers, `**bold**` metrics, failure
  table in markdown format
- **HTML output**: Contains `<html>`, inline styles, metric values
- **No failures**: Notable failures section omitted when all issues succeed
- **Empty period**: Report with 0 runs shows "No runs found in this period"
