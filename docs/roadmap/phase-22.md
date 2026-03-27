## Phase 22: Analytics Overhaul ✅

**Goal**: The `godark analyze` command and dashboard analytics page surface
actionable metrics that answer five operator questions: is the system improving,
where is money going, where is time going, what's failing and why, and what did
we ship. First-pass success rate, wasted cost, failure reason breakdown, and
per-repo efficiency replace low-value metrics.

**Milestone**: `Phase 22` | **Label**: `phase-22`

### Overview cards
- First-pass success rate — percentage of issues that succeed without any
  retries
- Avg cost per successful issue — total cost / implemented count
- Overview card row in dashboard (total runs, total issues, success rate,
  first-pass rate, total cost, avg cost per success)

### Trends
- First-pass success rate trend over time (new trend line alongside existing
  success rate)

### Cost analysis
- Cost per successful issue vs cost per failed issue
- Wasted cost — total cost on issues that ultimately failed

### Duration analysis
- Avg time to merge — end-to-end cycle time from issue start to implemented
- Timeout rate — percentage of steps that hit the agent_timeout

### Quality and failure analysis
- Failure reason breakdown — categorize failures into verify failure, review
  exhaustion, timeout, error
- Drop scenario spec gap condition (always present now)
- Drop exhausted retries listing (redundant with failure breakdown)

### Per-repo enrichment
- Avg cost per issue by repo
- First-pass success rate by repo

### Output
- Update `godark analyze` CLI output with all new metrics
- Update dashboard analysis page with new cards, charts, and tables
- Update `godark analyze --json` output to include all new fields

### Sprint summary report
- `godark report` command — new Cobra subcommand with `--since` (duration like
  `2w`, `30d`) and `--until` flags for date range, `--repo` filter, and
  `--format` flag (terminal default, markdown, html)
- Report content — sprint-scoped metrics from SQLite: issues closed, PRs
  merged, success rate, first-pass rate, cost breakdown, failure reasons,
  notable failures (with issue numbers and error messages)
- Markdown/HTML output suitable for pasting into Slack, email, or a wiki page

**Issues**: #489–#497

**Planning doc**: `docs/planning/phase-22-analytics-overhaul.md`

