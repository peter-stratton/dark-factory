# Phase 11: Run Analysis & Prompt Feedback

> **Goal:** `godark analyze` reads run data across multiple runs to surface
> failure patterns, common quality flags, and prompt gaps — closing the
> feedback loop between agent execution and prompt engineering.

## Milestone

`Phase 11`

---

## Issue 183: Update architecture.json for analysis package

### Description

Add `internal/analysis/` to the domain layer paths in `docs/architecture.json`
and update the corresponding entry in `docs/architecture.md`. The analysis
package contains pure business logic (aggregation and statistics over run data)
with no external service dependencies.

### Key constraints

- Modify `docs/architecture.json` — add `"internal/analysis/"` to the domain
  layer's `paths` array
- Modify `docs/architecture.md` — add `internal/analysis/` to the domain
  layer's **Paths** list

### Acceptance criteria

- [ ] `docs/architecture.json` domain layer includes `internal/analysis/`
- [ ] `docs/architecture.md` domain layer lists `internal/analysis/`
- [ ] `godark vet architecture` still passes (no cycles introduced)

### Test cases

- **JSON valid**: `docs/architecture.json` parses without errors
- **Domain paths updated**: Domain layer `paths` array contains
  `internal/analysis/`
- **Vet passes**: `godark vet architecture` produces no findings

---

## Issue 184: Analysis aggregation package

**Blocked by**: #183

### Description

New `internal/analysis/` package containing types and functions for computing
aggregate statistics across multiple runs. The package takes pre-loaded
`[]rundata.RunDetail` slices as input and returns structured reports — pure
domain logic with no I/O or external dependencies.

Statistics computed:
- Outcome distribution (implemented, failed, needs-human-review counts)
- Quality flag frequencies (how often each flag code appears, sorted by count)
- Retry statistics (average retries, max retries, retry-exhaustion count)
- Verify step failure rates by check type (build, lint, test)
- Cost statistics (total cost, average cost per issue)

This is pure new code — no existing files are modified.

### Key constraints

- New package: `internal/analysis/`
- Architecture layer: domain (depends only on `internal/rundata/` types)
- Exported types:
  ```go
  // Report holds aggregate statistics computed across multiple runs.
  type Report struct {
      RunCount       int                `json:"run_count"`
      IssueCount     int                `json:"issue_count"`
      Outcomes       map[string]int     `json:"outcomes"`
      FlagFrequencies []FlagFrequency   `json:"flag_frequencies"`
      RetryStats     RetryStats         `json:"retry_stats"`
      VerifyStats    map[string]int     `json:"verify_stats"`
      CostStats      CostStats          `json:"cost_stats"`
  }

  type FlagFrequency struct {
      Code    string  `json:"code"`
      Count   int     `json:"count"`
      Percent float64 `json:"percent"` // of total issues
  }

  type RetryStats struct {
      TotalRetries    int     `json:"total_retries"`
      AvgPerIssue     float64 `json:"avg_per_issue"`
      MaxRetries      int     `json:"max_retries"`
      ExhaustedCount  int     `json:"exhausted_count"` // issues with status "needs-human-review"
  }

  type CostStats struct {
      TotalUSD       float64 `json:"total_usd"`
      AvgPerIssueUSD float64 `json:"avg_per_issue_usd"`
      AvgPerRunUSD   float64 `json:"avg_per_run_usd"`
  }
  ```
- Exported function:
  ```go
  // Aggregate computes statistics across the provided runs.
  // Returns a zero Report if runs is empty.
  func Aggregate(runs []rundata.RunDetail) Report
  ```
- `FlagFrequencies` sorted by `Count` descending
- Outcome counts use the same status strings as `rundata.Outcome.Status`
  (`"implemented"`, `"failed"`, `"needs-human-review"`, `"ready-to-merge"`)
- `VerifyStats` keys are check names (`"build"`, `"lint"`, `"test"`) with
  values being failure counts — this depends on Phase 10's
  `rundata.VerifyStepResult`; if that type doesn't exist yet, the verify
  stats section returns an empty map (graceful degradation)
- Flags are collected from all `StepResult.Flags` across implement, reviews,
  and retries for each issue

### Acceptance criteria

- [ ] `Aggregate` computes outcome distribution from issue outcomes
- [ ] `Aggregate` computes flag frequencies sorted by count
- [ ] `Aggregate` computes retry statistics including exhaustion count
- [ ] `Aggregate` computes cost statistics (total, per-issue, per-run)
- [ ] Empty input returns zero Report (no panics)

### Test cases

- **Outcome counts**: 3 runs with 2 implemented, 1 failed → outcomes map
  has correct counts
- **Flag frequencies**: 5 issues with 3 `no_diff_read` flags and 1
  `low_cost` flag → sorted output is `no_diff_read` (3), `low_cost` (1)
  with correct percentages
- **Retry stats**: Issues with 0, 2, and 3 retries → avg 1.67, max 3
- **Exhausted count**: 2 issues with status `needs-human-review` → count is 2
- **Cost stats**: Total cost summed across all issues in all runs
- **Empty runs**: `Aggregate(nil)` returns zero Report
- **Single run**: One run with one issue produces correct singleton stats

---

## Issue 185: Prompt gap detection

**Blocked by**: #184

### Description

Add a `DetectGaps` function to the `internal/analysis/` package that
identifies correlations between run configuration and failure rates.
This helps users discover which harness files or configuration options
improve agent success.

Gap detection is based entirely on data available in run data — no
external API calls. The function compares failure rates for runs that
have a given condition vs runs that don't.

### Key constraints

- Add to `internal/analysis/`:
  ```go
  // PromptGap describes a detected correlation between a condition and
  // higher failure rates.
  type PromptGap struct {
      Finding       string  `json:"finding"`
      FailRateWith  float64 `json:"fail_rate_with"`
      FailRateWithout float64 `json:"fail_rate_without"`
      SamplesWith   int     `json:"samples_with"`
      SamplesWithout int    `json:"samples_without"`
  }

  // DetectGaps compares failure rates across runs grouped by various
  // conditions. Returns findings sorted by the absolute difference in
  // failure rates (largest gap first). Conditions with fewer than 3
  // samples on either side are excluded to avoid noise.
  func DetectGaps(runs []rundata.RunDetail) []PromptGap
  ```
- Conditions to compare (each produces one potential finding):
  - Runs with vs without a quality reviewer step recorded
  - Runs with vs without scenario specs (issues that have spec gen results)
  - Issues that exhausted retries — list their issue numbers and titles
    as a separate finding type
- A "failure" for gap detection purposes is any issue with status `"failed"`
  or `"needs-human-review"`
- Minimum sample size of 3 on each side of a comparison; below that, the
  finding is excluded (not enough data to be meaningful)

### Acceptance criteria

- [ ] `DetectGaps` compares failure rates for runs with/without quality review
- [ ] `DetectGaps` compares failure rates for issues with/without scenario specs
- [ ] `DetectGaps` surfaces issues that exhausted retries
- [ ] Findings with fewer than 3 samples per side are excluded
- [ ] Findings sorted by gap magnitude (largest difference first)

### Test cases

- **Quality review gap**: 5 runs with quality review (20% failure) and 5
  without (60% failure) → finding with correct rates
- **Scenario spec gap**: Issues with specs fail 10%, without fail 50% →
  finding with correct rates
- **Exhausted retries**: 2 issues with `needs-human-review` → finding lists
  both
- **Insufficient samples**: Condition with 2 runs on one side → excluded
  from results
- **No gaps**: All runs have identical conditions → empty results
- **Empty input**: `DetectGaps(nil)` returns nil

---

## Issue 187: Analyze command

**Blocked by**: #184, #185

### Description

New `godark analyze` Cobra command that reads run data, filters by
user-specified criteria, runs the analysis functions, and prints a
structured report to stdout. Supports human-readable (default) and JSON
output formats.

### Key constraints

- New file: `internal/cmd/analyze.go`
- Modify `internal/cmd/root.go`: add `analyzeCmd` to root command
- Flags:
  - `--repo` (string): filter to runs for this repo (optional)
  - `--milestone` (string): filter to runs for this milestone (optional)
  - `--since` (string): only include runs started after this date
    (RFC 3339 or `YYYY-MM-DD`, optional)
  - `--until` (string): only include runs started before this date (optional)
  - `--json` (bool): output as JSON instead of human-readable table
- Command logic:
  1. Create `rundata.Reader`
  2. Call `ListRuns()` to get all `RunMeta`
  3. Filter by `--repo`, `--milestone`, `--since`, `--until`
  4. Call `LoadRun()` for each matching run
  5. Call `analysis.Aggregate(runs)`
  6. Call `analysis.DetectGaps(runs)`
  7. Format and print report
- Human-readable output format:
  - Summary line: "Analyzed N runs, M issues"
  - Outcome table (status, count, percentage)
  - Flag frequency table (code, count, percentage)
  - Retry stats (avg, max, exhausted)
  - Cost stats (total, per-issue, per-run)
  - Prompt gaps section (finding, failure rates, sample sizes)
- JSON output: marshal the `Report` and `[]PromptGap` into a combined
  JSON object

### Acceptance criteria

- [ ] `godark analyze` prints aggregate report to stdout
- [ ] `--repo` filters runs by repository
- [ ] `--since` and `--until` filter by date range
- [ ] `--json` outputs machine-readable JSON
- [ ] No runs matching filters prints "No matching runs found"

### Test cases

- **Full report**: Given run data, command prints all report sections
- **Repo filter**: `--repo owner/other` excludes non-matching runs
- **Milestone filter**: `--milestone "Phase 7"` includes only Phase 7 runs
- **Date filter**: `--since 2026-01-01 --until 2026-02-01` includes only
  January runs
- **JSON output**: `--json` produces valid JSON with report and gaps fields
- **No matches**: Filters that match no runs print message and exit 0
- **Empty state**: No run data directory prints message and exit 0

---

## Issue 186: Trend computation function

**Blocked by**: #184

### Description

Add a `ComputeTrends` function to the `internal/analysis/` package that
produces per-run data points suitable for charting. Each data point
represents one run and includes success rate, average retries, and cost.
Runs are sorted chronologically.

This is pure new code added to the existing analysis package.

### Key constraints

- Add to `internal/analysis/`:
  ```go
  // TrendPoint represents one data point in a time series, corresponding
  // to a single run.
  type TrendPoint struct {
      Timestamp   time.Time `json:"timestamp"`
      Repo        string    `json:"repo"`
      Milestone   string    `json:"milestone"`
      IssueCount  int       `json:"issue_count"`
      SuccessRate float64   `json:"success_rate"` // 0.0–1.0
      AvgRetries  float64   `json:"avg_retries"`
      TotalCostUSD float64  `json:"total_cost_usd"`
  }

  // ComputeTrends returns one TrendPoint per run, sorted chronologically
  // (oldest first). Unfinished runs (no FinishedAt) are excluded.
  func ComputeTrends(runs []rundata.RunDetail) []TrendPoint
  ```
- `SuccessRate` = count of `"implemented"` outcomes / total issues in run
- Unfinished runs (where `RunMeta.FinishedAt` is nil) are excluded since
  their stats are incomplete
- Cost is summed from all issue step results within the run

### Acceptance criteria

- [ ] `ComputeTrends` returns one point per finished run, sorted by time
- [ ] Each point has correct success rate, avg retries, and cost
- [ ] Unfinished runs are excluded
- [ ] Empty input returns nil

### Test cases

- **Chronological order**: 3 runs started at different times → sorted
  oldest first
- **Success rate**: Run with 3 implemented, 1 failed → rate 0.75
- **Avg retries**: Run with issues having 0, 2, 4 retries → avg 2.0
- **Cost summed**: Run with 3 issues costing $0.10 each → total $0.30
- **Unfinished excluded**: Run with nil `FinishedAt` is omitted
- **Empty input**: `ComputeTrends(nil)` returns nil

---

## Issue 188: Dashboard analysis page

**Blocked by**: #184

### Description

Add an `/analysis` page to the dashboard that displays aggregate statistics
across all runs in a table format. The page reuses the existing dashboard
template infrastructure (Go templates + htmx + Alpine.js) and reads data
via the existing `rundata.Reader`.

### Key constraints

- New template file: `internal/dashboard/templates/analysis.html`
- Modify `internal/dashboard/server.go`:
  - Add route: `GET /analysis` → `handleAnalysis`
- Modify `internal/dashboard/handlers.go`:
  - New `AnalysisData` view model struct containing the `analysis.Report`
    and `[]analysis.PromptGap`
  - New `handleAnalysis` handler: loads all runs, calls `Aggregate` and
    `DetectGaps`, renders template
  - Add repo filter support via `?repo=` query param (same pattern as
    index page)
- Template content:
  - Navigation link in header (add to all templates or shared nav partial)
  - Outcome distribution table (status, count, percentage bar)
  - Flag frequency table (code, count, percentage)
  - Retry stats summary (avg, max, exhausted count)
  - Cost stats summary (total, per-issue, per-run)
  - Prompt gaps section (collapsible, showing finding + rates + sample sizes)
  - Repo filter dropdown (htmx, same pattern as index page)
- The `internal/dashboard/` package adds an import of `internal/analysis/`
  — this is allowed: presentation may depend on domain

### Acceptance criteria

- [ ] `/analysis` route serves the analysis page
- [ ] Page displays outcome distribution, flag frequencies, retry and cost stats
- [ ] Prompt gaps section renders when gaps are detected
- [ ] Repo filter narrows analysis to a single repo
- [ ] Page renders correctly with no run data (empty state message)

### Test cases

- **Page renders**: GET `/analysis` returns 200 with HTML content
- **Stats displayed**: Page contains outcome counts matching test data
- **Flag table**: Page lists flag codes with counts
- **Gaps section**: Prompt gap finding text appears in rendered HTML
- **Repo filter**: `?repo=owner/name` limits stats to that repo
- **Empty state**: No run data shows "No run data available" message

---

## Issue 189: Dashboard trend charts

**Blocked by**: #186, #188

### Description

Add trend charts to the analysis page showing success rate, average retries,
and cost per run over time. Charts are rendered client-side using Chart.js
(vendored as a static asset). Trend data is computed server-side and embedded
in the template as JSON.

### Key constraints

- New static asset: `internal/dashboard/static/chart.min.js` — vendored
  Chart.js (lightweight, ~70 KB minified)
- Modify `internal/dashboard/templates/analysis.html`:
  - Add `<canvas>` elements for three charts: success rate, avg retries,
    cost over time
  - Add `<script>` block that reads trend data from a JSON `<script>` tag
    and initializes Chart.js line charts
  - Charts use timestamp as x-axis (time scale), metric as y-axis
  - Minimal styling: line charts, point markers, tooltips showing run
    details (repo, milestone, timestamp)
- Modify `internal/dashboard/handlers.go`:
  - In `handleAnalysis`, also call `analysis.ComputeTrends(runs)` and
    include trend data in `AnalysisData`
  - Add a `template.FuncMap` entry `toJSON` that marshals data for
    embedding in templates (or use existing patterns)
- Charts should degrade gracefully: if no trend data (< 2 points), hide
  the charts section and show "Not enough data for trends"

### Acceptance criteria

- [ ] Chart.js vendored as static asset and served at `/static/chart.min.js`
- [ ] Analysis page renders three trend charts when data is available
- [ ] Charts display correct data points matching the run history
- [ ] Fewer than 2 data points hides charts with explanatory message
- [ ] Charts are responsive (resize with container)

### Test cases

- **Chart.js served**: GET `/static/chart.min.js` returns JavaScript content
- **Charts rendered**: Analysis page with 5 runs contains `<canvas>` elements
- **Trend data embedded**: Page source contains JSON array with trend points
- **No data fallback**: Analysis page with 0 runs shows "Not enough data"
- **Single run fallback**: 1 finished run (< 2 points) hides charts

---

## Issue 190: Homepage analytics summary

**Blocked by**: #186, #188

### Description

Upgrade the dashboard homepage (`/`) to show a lightweight analytics summary
above the existing run list. This gives users an at-a-glance view of system
health without navigating to the full analysis page. The summary reuses the
existing `analysis.Aggregate` and `analysis.ComputeTrends` functions.

### Key constraints

- Modify `internal/dashboard/handlers.go`:
  - In `buildIndexData`, load all run details (not just metas), call
    `analysis.Aggregate` and `analysis.ComputeTrends`
  - Add `Summary *analysis.Report` and `Trends []analysis.TrendPoint`
    fields to `IndexData`
  - If loading full details is too slow, limit to the most recent 50 runs
- Modify `internal/dashboard/templates/index.html`:
  - Add summary cards above the run list: total runs, total issues,
    success rate (percentage), average cost per issue
  - Add a small success-rate sparkline or mini line chart using Chart.js
    (already vendored by the trend charts issue)
  - Cards link to `/analysis` for full details
  - If no run data exists, cards section is hidden (existing empty state
    is sufficient)

### Acceptance criteria

- [ ] Homepage shows summary cards with aggregate stats
- [ ] Success rate percentage is displayed prominently
- [ ] Mini trend chart shows success rate over recent runs
- [ ] Cards link to `/analysis` page
- [ ] Empty state (no runs) hides the summary section gracefully

### Test cases

- **Cards displayed**: Homepage with run data shows summary cards
- **Success rate correct**: 8 implemented, 2 failed → "80% success rate"
- **Cost displayed**: Average cost per issue shown in card
- **Link to analysis**: Cards section contains link to `/analysis`
- **Empty state**: No run data hides summary cards, shows existing empty state
- **Mini chart**: Homepage contains a `<canvas>` element for the sparkline
