# Phase 21: Analytics Persistence

Before Phase 21, deleting old run directories to keep the dashboard clean destroyed trend data. All analytics came from scanning `~/.godark/runs/` on every query -- slow for large histories and impossible for deleted runs. Phase 21 adds a SQLite database (`~/.godark/stats.db`) that captures run, issue, and per-step metrics at finalization time. `godark analyze` and the dashboard read from the database by default, enabling new metrics that answer the questions that matter: where is time being spent, which repos struggle, and do retries actually work?

---

## SQLite Stats Store

**What it does:** A pure-Go SQLite database (`modernc.org/sqlite`, no CGO) persists run statistics at `~/.godark/stats.db`. Three tables capture metrics at different granularities: runs (one row per run), issue outcomes (one row per issue), and step results (one row per agent step).

**Example:** After a `godark run --tag phase-21` completes, the orchestrator writes stats in a single transaction:

```go
statsDB := OpenStatsDB(logger)
// ... run completes ...
WriteRunStats(ctx, statsDB, cfg, writer, summary, logger)
```

`WriteRunStats` loads the completed run directory, builds records for each issue and step, and writes them atomically:

```sql
INSERT OR REPLACE INTO runs (id, repo, milestone, ...) VALUES (?, ?, ?, ...)
INSERT OR REPLACE INTO issue_outcomes (run_id, issue_number, ...) VALUES (?, ?, ...)
INSERT OR REPLACE INTO step_results (run_id, issue_number, step_name, cost_usd, duration_seconds, flags, ...) VALUES (?, ?, ?, ?, ?, ?, ...)
```

The `INSERT OR REPLACE` pattern makes writes idempotent -- re-running the same issues updates rather than duplicates. If the stats database is unavailable (permissions, disk full), the run completes normally with a warning log. Stats are best-effort, never blocking.

---

## Dual-Path Reading

**What it does:** Both `godark analyze` and the dashboard read from SQLite by default, falling back to the filesystem when the database is unavailable. A `--legacy` flag on the analyze command forces the old filesystem scan.

**Example:** Default behavior reads from the database:

```
$ godark analyze --repo peter-stratton/dark-factory --since 2026-03-10
```

The command opens `~/.godark/stats.db`, queries with the filters, converts the results to the same `RunDetail` structs that `analysis.Aggregate()` expects, and produces identical output. To fall back to the filesystem:

```
$ godark analyze --legacy --repo peter-stratton/dark-factory
```

The conversion function `stats.ToRunDetails()` reconstructs the issue hierarchy from flat database rows, including retry step naming (`retry-1`, `retry-1-quality-review`, `retry-1-functional-review`). The conversion is intentionally lossy -- raw agent output, tool traces, and session IDs are not stored in the stats database, only the metrics that drive analytics.

---

## Retry Recovery Rate

**What it does:** Answers "of issues that needed retries, what percentage eventually succeeded?" A recovery rate of 0.80 means retries are working well. A rate near 0.30 means agents are burning tokens on approaches that don't converge.

**Example:** After several runs, `godark analyze` shows:

```
Retry Statistics
  Total retries:    18
  Average retries:  1.8
  Max retries:      3
  Exhausted:        2
  Recovery rate:    77.8%
```

The recovery rate is computed in `analysis.Aggregate()`:

```go
if retryStats.RetriedCount > 0 {
    retryStats.RecoveryRate = float64(retriedSucceeded) / float64(retryStats.RetriedCount)
}
```

An issue counts as "retried" if it has any retry step results. It counts as "recovered" if its final outcome is `implemented` or `ready-to-merge`. Issues that exhaust retries and land in `needs-human-review` or `failed` reduce the rate.

---

## Cost Breakdown by Step

**What it does:** Shows what percentage of total cost goes to each step type -- implement, quality review, functional review, retries, recon, and spec generation. Tells you where to focus optimization efforts.

**Example:** `godark analyze` includes a cost breakdown table:

```
Cost by Step
  STEP                COST      %
  implement           $4.20     58.3%
  quality-review      $1.50     20.8%
  functional-review   $0.90     12.5%
  retry-1             $0.45      6.3%
  recon               $0.15      2.1%
```

The data comes from `CostStats.CostByStep`, a map of step name to total USD accumulated across all issues and runs matching the filter. Each step result in the stats database carries its own `cost_usd`, summed during aggregation.

---

## Duration Breakdown by Step

**What it does:** Shows where wall-clock time is spent across step types. Helps identify when the `agent_timeout` setting (default 30m) needs adjusting, or when mechanical verification is dominating run time.

**Example:** `godark analyze` includes a duration table:

```
Duration by Step
  STEP                DURATION    %
  implement           14m30s      62.5%
  quality-review       4m15s      18.3%
  functional-review    2m50s      12.2%
  verify               1m05s       4.7%
  recon                0m32s       2.3%
```

If implement steps are consistently approaching the 30-minute timeout, that signals a need to increase `agent_timeout` in `godark.yaml` for that project. If verify steps dominate, the build/lint/test pipeline might benefit from optimization.

---

## Duration Trends Over Time

**What it does:** Tracks average implement and review step durations per run as trend points. The dashboard renders these as line charts alongside the existing success rate and cost trends.

**Example:** The `TrendPoint` struct now includes duration fields:

```go
type TrendPoint struct {
    Timestamp              time.Time
    Repo                   string
    Milestone              string
    AvgImplementDuration   float64  // seconds
    AvgReviewDuration      float64  // seconds
    // ... existing fields ...
}
```

`ComputeTrends()` calculates the average implement and review duration across all issues in each run. The dashboard analysis page renders duration trend lines, letting you spot regressions -- if implement times spike on a particular repo, you can investigate whether the codebase grew, the prompts degraded, or the agent is hitting timeouts.

---

## Success Rate by Repository

**What it does:** Breaks down pass/fail rates per repository for teams running godark against multiple projects. Shows which repos have mature harnesses and which need attention.

**Example:** `godark analyze` shows per-repo stats:

```
Success by Repo
  REPO                           TOTAL  IMPL  FAILED  RATE
  peter-stratton/dark-factory       45    38       7  84.4%
  myorg/frontend-app                12     8       4  66.7%
```

The data comes from `Report.RepoStats`, a map of repo name to `RepoSummary`:

```go
type RepoSummary struct {
    Total       int
    Implemented int
    Failed      int
    SuccessRate float64
}
```

A repo with a 66% success rate probably has weaker architecture docs or scenario specs than one at 84%. This metric directly guides where to invest in harness improvement.

---

## Verify Check Failure Rates

**What it does:** Surfaces the verify check failure data that was already computed in `Report.VerifyStats` but never displayed. Shows which checks (build, lint, test) fail most often across all runs.

**Example:** `godark analyze` now includes verify statistics:

```
Verify Check Failures
  CHECK    FAILURES  RATE
  lint          5    12.5%
  test          3     7.5%
  build         1     2.5%
```

If lint failures dominate, agents may be writing code that doesn't follow the project's style rules -- a signal to improve `conventions.md` or the implementer prompt. If test failures are high, scenario specs may be insufficiently detailed.

---

## Flag-Based Prompt Gaps

**What it does:** Replaces the old "with/without quality reviewer" comparison with flag-to-outcome correlation. For each quality flag code that appears, computes the failure rate of issues carrying that flag vs issues without it. Directly answers "which quality problems predict failure?"

**Example:** `godark analyze` shows flag correlations:

```
Prompt Gaps
  Issues with no_diff_read: 70.0% failure rate (10 samples)
    vs 20.0% baseline (40 samples)
  Issues with low_cost: 50.0% failure rate (8 samples)
    vs 15.0% baseline (42 samples)
```

This tells you that when an agent review is flagged for not reading the PR diff (`no_diff_read`), the issue is 3.5x more likely to fail. The fix: adjust the reviewer prompt to emphasize diff reading, or add a verification step that checks for diff reads.

The scenario spec gap and exhausted retries listing are preserved from the previous implementation. The minimum sample threshold of 3 per condition prevents noisy results from appearing.

---

## Architecture

**What it does:** The `internal/stats/` package lives in the domain layer alongside `internal/rundata/` and `internal/analysis/`. The SQLite driver (`modernc.org/sqlite`) is pure Go with no CGO, preserving the zero-CGO build that enables GoReleaser cross-compilation.

**Example:** The stats database is an optional dependency throughout the system. The orchestrator opens it at run start and closes on exit:

```go
statsDB := OpenStatsDB(logger)
if statsDB != nil {
    defer statsDB.Close()
}
```

The dashboard server receives it at startup:

```go
srv := dashboard.New(cfg, reader, statsDB)
```

If `statsDB` is nil (database doesn't exist, permissions error, or first run), everything falls back to filesystem-based analysis. No feature requires the database -- it strictly enhances performance and enables run deletion without losing trends.
