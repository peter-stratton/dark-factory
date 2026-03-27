# Phase 22: Analytics Overhaul

Phase 21 gave godark a SQLite stats database. Phase 22 makes it useful. The `godark analyze` command and dashboard analytics page now surface the metrics operators actually care about: is the system improving over time, where is money going, where is time going, what's failing and why, and what shipped this sprint. A new `godark report` command generates sprint-scoped summaries in terminal, markdown, or HTML format -- ready to paste into Slack or a wiki.

---

## Overview Metrics Cards

**What it does:** Six overview cards on the dashboard analytics page give an at-a-glance snapshot of system health. The same metrics appear at the top of `godark analyze` terminal output.

**Example:** After running godark against 12 milestones over the past month, the dashboard shows:

| Total Runs | Total Issues | Success Rate | First-Pass Rate | Total Cost | Avg Cost / Success |
|------------|-------------|-------------|----------------|------------|-------------------|
| 14 | 87 | 82.8% | 64.4% | $23.50 | $0.33 |

First-pass rate -- the percentage of issues that succeed without any retries -- is the most telling metric. A high success rate with a low first-pass rate means agents are getting there eventually but burning cost on retries. The `OverviewMetrics` struct in `internal/dashboard/handlers.go` feeds the template:

```go
type OverviewMetrics struct {
    TotalRuns              int
    TotalIssues            int
    SuccessRate            float64
    FirstPassRate          float64
    TotalCostUSD           float64
    AvgCostPerSuccessUSD   float64
    WastedCostUSD          float64
    TimeoutRate            float64
}
```

---

## Failure Reason Breakdown

**What it does:** Categorizes every failed issue into one of four mutually exclusive reasons: verify failure (build/test didn't pass), review exhaustion (retries burned through), timeout (agent hit the clock), or error (everything else). Replaces the old exhausted-retries listing which was redundant.

**Example:** Running `godark analyze --repo peter-stratton/dark-factory`:

```
Failure Reasons
  Reason       Count  Percent
  verify           5   33.3%
  exhaustion       4   26.7%
  timeout          4   26.7%
  error            2   13.3%
```

The categorization logic in `internal/analysis/analysis.go` is deterministic -- each failed issue maps to exactly one bucket. The `FailureReasons` map uses canonical keys:

```go
FailureReasons map[string]int{
    "verify":      5,  // last verify step failed
    "exhaustion":  4,  // needs-human-review (retries exhausted)
    "timeout":     4,  // any step timed out
    "error":       2,  // other failures
}
```

This breakdown drives targeted improvements. A high verify rate means specs are unclear or test commands are misconfigured. A high exhaustion rate means agents are close but can't converge. A high timeout rate means the `agent_timeout` is too short or issues are too complex for a single pass.

---

## Cost Analysis

**What it does:** Breaks cost down by step (recon, implement, review, retries) and surfaces wasted cost -- the total spend on issues that ultimately failed. Also computes average cost per successful issue, which is the unit economics metric that matters.

**Example:** The `CostStats` struct from `analysis.Report`:

```go
type CostStats struct {
    TotalUSD       float64              // $23.50
    AvgPerIssueUSD float64              // $0.27
    AvgPerRunUSD   float64              // $1.68
    CostByStep     map[string]float64   // step -> total spend
}
```

Terminal output from `godark analyze`:

```
Cost by Step
  Step                Total Cost  Percent
  implement              $14.20    60.4%
  retries                 $4.10    17.4%
  functional-review       $2.80    11.9%
  quality-review          $1.50     6.4%
  recon                   $0.70     3.0%
  spec-generator          $0.20     0.9%
```

Wasted cost appears in the overview section: `Wasted cost: $3.80`. That's the sum of all step costs on issues with `failed` or `needs-human-review` status -- money spent with nothing to show for it.

---

## Duration Analysis and Time to Merge

**What it does:** Tracks average wall-clock time from issue start to merge (the end-to-end cycle time), plus per-step duration breakdowns. Timeout rate shows what percentage of steps hit the agent timeout.

**Example:** The duration section in `godark analyze` output:

```
Duration Stats
  Avg implement duration:  8m12s
  Avg review duration:     3m45s
  Avg time to merge:       14m30s

Duration by Step
  Step                Total Duration  Percent
  implement               11h22m       58.2%
  retries                  3h45m       19.2%
  functional-review        2h10m       11.1%
  quality-review           1h15m        6.4%
  recon                      38m        3.3%
  spec-generator             21m        1.8%
```

`AvgTimeToMergeSeconds` in `DurationStats` measures wall-clock time for implemented issues only -- from the first step's `StartedAt` to the last step's `FinishedAt`. On the dashboard, this renders as a human-readable duration string (e.g., "14m30s") in the `AvgTimeToMerge` field of `AnalysisData`.

---

## Per-Repo Breakdown

**What it does:** When godark runs against multiple repositories, the analytics page and CLI break down success rate, first-pass rate, and cost per issue by repo. This surfaces which codebases are agent-friendly and which need harness work.

**Example:** A team running godark across three repos:

```
Success by Repo
  Repo                    Total  Impl  Failed  Success%  First-pass  Avg cost
  acme/backend               42    38       4     90.5%      71.4%     $0.28
  acme/frontend              30    22       8     73.3%      53.3%     $0.41
  acme/infra                 15    12       3     80.0%      60.0%     $0.35
```

The `RepoStats` map in `analysis.Report` stores a `RepoSummary` per repo:

```go
type RepoSummary struct {
    Total              int
    Implemented        int
    Failed             int
    SuccessRate        float64
    FirstPassCount     int
    FirstPassRate      float64
    TotalCostUSD       float64
    AvgCostPerIssueUSD float64
}
```

On the dashboard, a repo filter dropdown lets you scope all analytics to a single repo. The filter uses HTMX to swap the stats partial without a full page reload.

---

## Sprint Report Command

**What it does:** `godark report` generates a time-scoped summary from the stats database, suitable for sharing with a team. Supports terminal, markdown, and HTML output formats. Optionally includes an LLM-generated executive summary.

**Example:** Generate a two-week sprint report in markdown:

```bash
godark report --since 2w --format markdown --repo peter-stratton/dark-factory
```

Output:

```markdown
# Sprint Report: 2026-03-12 to 2026-03-26

**Repo:** peter-stratton/dark-factory

## Summary

| Metric | Value |
|--------|-------|
| Runs | 6 |
| Issues processed | 34 |
| Implemented | 28 |
| Failed | 6 |
| Success rate | 82.4% |
| First-pass rate | 61.8% |

## Cost

| Metric | Value |
|--------|-------|
| Total cost | $9.20 |
| Avg cost per success | $0.33 |
| Wasted cost | $1.80 |

## Failure Reasons

| Reason | Count | Share |
|--------|-------|-------|
| verify | 3 | 50.0% |
| exhaustion | 2 | 33.3% |
| timeout | 1 | 16.7% |

## What Shipped

- #678 Require --tag on vet scenarios (PR #680) — peter-stratton/dark-factory
- #679 Spec delta package (PR #682) — peter-stratton/dark-factory
...
```

The command flags in `internal/cmd/report.go`:

| Flag | Default | Purpose |
|------|---------|---------|
| `--since` | `2w` | Lookback duration (e.g., `2w`, `30d`, `7d`) |
| `--until` | now | End of window (RFC 3339 or YYYY-MM-DD) |
| `--repo` | all | Filter to specific repo |
| `--format` | `terminal` | `terminal`, `markdown`, or `html` |
| `--no-summary` | false | Skip LLM executive summary |

The `SprintReport` struct in `internal/report/report.go` includes a `PriorPeriod` field -- when present, the report adds a "Compared to Prior Period" section showing deltas for success rate, first-pass rate, issues processed, total cost, and avg cost per success.

---

## Dashboard Analytics Page

**What it does:** The web dashboard's analysis page renders all metrics as interactive cards and tables, with a repo filter dropdown and trend charts when enough data points exist.

**Example:** Navigating to `http://localhost:8080/analysis` shows the full analytics view. The page is assembled from `AnalysisData` in `internal/dashboard/handlers.go`:

```go
type AnalysisData struct {
    Report         analysis.Report
    Outcomes       []OutcomeRow
    CostByStep     []CostByStepRow
    DurationByStep []DurationByStepRow
    FailureReasons []FailureReasonRow
    RepoRows       []RepoRow
    VerifyRows     []VerifyRow
    Gaps           []GapView
    Trends         []analysis.TrendPoint
    Overview       OverviewMetrics
    AvgTimeToMerge string
    HasData        bool
    HasTrends      bool
    Repos          []string
    RepoFilter     string
}
```

Cards render in order: overview metrics row, outcome distribution, flag frequency, retry statistics, failure reason breakdown, cost statistics, cost by step, duration by step, verify check failures, success by repo, and prompt gaps. The repo dropdown triggers `hx-get="/partials/analysis-stats"` to swap the stats section via HTMX without a full page reload.

Trend data (`TrendPoint` per run) includes success rate, first-pass rate, avg cost per issue, and avg implement/review duration -- displayed as time-series charts when 2 or more data points exist.

---

## JSON Output

**What it does:** Both `godark analyze --json` and `godark report --format terminal` (with `--json` on analyze) emit machine-readable output for scripting and CI integration.

**Example:** `godark analyze --json --since 2026-03-01` outputs:

```json
{
  "report": {
    "run_count": 14,
    "issue_count": 87,
    "first_pass_count": 56,
    "first_pass_success_rate": 0.6437,
    "wasted_cost_usd": 3.80,
    "avg_cost_per_success_usd": 0.33,
    "timeout_rate": 0.046,
    "failure_reasons": {
      "verify": 5,
      "exhaustion": 4,
      "timeout": 4,
      "error": 2
    },
    "cost_stats": { "total_usd": 23.50, "avg_per_issue_usd": 0.27 },
    "duration_stats": { "avg_time_to_merge_seconds": 870 },
    "repo_stats": { "peter-stratton/dark-factory": { "success_rate": 0.828 } }
  },
  "gaps": []
}
```

The output struct wraps `analysis.Report` and `[]analysis.PromptGap` directly, so every metric available in the dashboard is also available programmatically.
