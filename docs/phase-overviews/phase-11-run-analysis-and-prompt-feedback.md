# Phase 11: Run Analysis & Prompt Feedback

Phase 11 closes the feedback loop between running agents and improving their
prompts. Before this phase, run data existed but sat inert on disk -- you
could look at individual runs in the dashboard, but there was no way to spot
patterns across runs or answer questions like "are scenario specs actually
helping?" The `godark analyze` command and the new dashboard analysis page
aggregate statistics, detect prompt gaps, and chart trends over time so you
can make evidence-based changes to your harness files and configuration.

---

## The `godark analyze` command

Reads all run data from `~/.godark/runs/`, computes aggregate statistics, and
prints a structured report to stdout. Supports filtering by repo, milestone,
and date range, with both human-readable and JSON output formats.

**Example: Reviewing results after a milestone run**

```
$ godark analyze --repo myorg/backend --milestone "Phase 9"

Analyzed 3 runs, 14 issues

Outcomes
  Status                Count  Percent
  implemented           11     78.6%
  failed                1      7.1%
  needs-human-review    2      14.3%

Flag Frequencies
  Code                  Count  Percent
  no_diff_read          4      28.6%
  low_cost              2      14.3%

Retry Stats
  Avg per issue:  1.21
  Max retries:    3
  Exhausted:      2

Cost Stats
  Total:          $1.84
  Per issue:      $0.13
  Per run:        $0.61

Prompt Gaps
  quality reviewer
    fail rate with: 10.0% (5 samples)
    fail rate without: 55.6% (9 samples)
  exhausted retries: #146 "Wire agent dialogue into run data", #149 "Architecture JSON context for reviewer"
    fail rate with: 100.0% (2 samples)
    fail rate without: 8.3% (12 samples)
```

The `--json` flag produces machine-readable output suitable for piping into
`jq` or feeding into other tools:

```
$ godark analyze --since 2026-02-01 --until 2026-03-01 --json | jq '.report.cost_stats'
{
  "total_usd": 3.42,
  "avg_per_issue_usd": 0.11,
  "avg_per_run_usd": 0.57
}
```

---

## Failure mode aggregation

The `analysis.Aggregate` function (in `internal/analysis/analysis.go`)
computes five categories of statistics from a slice of run data: outcome
distribution, quality flag frequencies, retry statistics, verify step failure
rates, and cost statistics. Everything is computed from the structured run
data already written by the orchestrator -- no additional API calls.

**Example: Spotting a lint problem**

You notice the verify stats section shows `lint` failures dominating:

```
Verify Failures
  build    1
  lint     9
  test     2
```

This tells you agents are consistently writing code that fails the linter.
The fix is upstream: update your conventions doc or implementer prompt to
reference the specific lint rules agents keep tripping over. Without this
aggregation, you would have to click through individual runs in the dashboard
to notice the pattern.

---

## Prompt gap detection

The `analysis.DetectGaps` function (in `internal/analysis/gaps.go`) compares
failure rates across different conditions to surface actionable correlations.
It answers questions like "do issues with scenario specs fail less often than
issues without them?"

Each comparison requires at least 3 samples on each side to avoid noisy
conclusions. Findings are sorted by the magnitude of the gap -- largest
difference first.

**Example: Discovering that quality review matters**

The gap detector splits all issues into two groups -- those where a quality
review step was recorded and those where it was skipped -- then compares
failure rates:

```
Prompt Gaps
  quality reviewer
    fail rate with: 12.5% (8 samples)
    fail rate without: 50.0% (6 samples)
```

This tells you that skipping the quality review step roughly quadruples your
failure rate. You might decide to make the quality reviewer mandatory for all
runs rather than optional.

It also surfaces issues that exhausted their retry budget, listing them by
number and title so you can investigate what made them hard.

---

## Trend computation

The `analysis.ComputeTrends` function (in `internal/analysis/trends.go`)
produces one data point per completed run, sorted chronologically. Each point
includes the success rate, average retries, total cost, and average cost per
issue. Unfinished runs are excluded since their stats are incomplete.

This data powers the trend charts in the dashboard and the homepage sparkline.
It is also embedded as JSON in the analysis page template for client-side
Chart.js rendering.

**Example: Tracking improvement over time**

After updating your implementer prompt to include architecture constraints,
the trend data shows success rate climbing from 60% to 90% over five runs
while average retries drop from 2.1 to 0.8. You can see this at a glance in
the dashboard charts without manually comparing individual runs.

---

## Dashboard analysis page

The `/analysis` route in the dashboard serves a dedicated page with all
aggregate statistics rendered as tables and charts. It reuses the same
`analysis.Aggregate`, `analysis.DetectGaps`, and `analysis.ComputeTrends`
functions that power the CLI command.

**What you see:**

- **Outcome distribution table** with percentage bars showing how issues
  resolved (implemented, failed, needs-human-review)
- **Flag frequency table** listing every quality flag code with its count
  and percentage, so you can spot which review quality problems are most
  common
- **Retry statistics** showing average and max retries, plus how many issues
  exhausted their retry budget
- **Cost statistics** with total, per-issue, and per-run averages
- **Prompt gaps section** with collapsible details showing each finding's
  failure rates and sample sizes
- **Three trend charts** (success rate, average retries, cost per issue)
  rendered with Chart.js, each with tooltips showing the repo and milestone
  for that data point

The page supports filtering by repository via a sidebar dropdown or the
`?repo=owner/name` query parameter. When fewer than 2 completed runs exist,
the trend charts section shows a "Not enough data for trends" message instead
of empty charts.

**Example: Filtering to a single repo**

You manage three repos with godark. Clicking "myorg/backend" in the sidebar
narrows all statistics and charts to just that repo, letting you see whether
backend-specific prompt changes are having an effect without noise from your
other projects.

---

## Homepage analytics summary

The dashboard homepage (`/`) now shows summary cards above the run list:
total runs, total issues, success rate percentage, and average cost per issue.
Each card links to the full `/analysis` page. This gives you an at-a-glance
health check every time you open the dashboard, without requiring navigation
to a separate page.

**Example: Quick health check**

You open `godark status` in your browser and immediately see:

```
Total Runs: 12    Total Issues: 47    Success Rate: 85%    Avg Cost / Issue: $0.12
```

If the success rate looks off, you click through to the analysis page for the
full breakdown. If everything looks healthy, you scroll down to the run list
to check on the latest run.
