## Phase 11: Run Analysis & Prompt Feedback ✅

**Goal**: `godark analyze` reads run data across multiple runs to surface
failure patterns, common quality flags, and prompt gaps — closing the feedback
loop between agent execution and prompt engineering.

**Milestone**: `Phase 11` | **Label**: `phase-11`

### Analyze command
- `godark analyze` command — reads `~/.godark/runs/` across all repos and runs
- Filterable by repo, milestone, date range
- Outputs a structured report to stdout (human-readable, optionally JSON)

### Failure mode aggregation
- Common quality flag frequencies (e.g. "30% of reviews flagged
  `no_review_tests_written` on first pass")
- Retry reason distribution — why implementations needed retries
- Verdict distributions per phase/milestone
- Verify step failure rates by check type (build vs lint vs test)

### Prompt gap detection
- Correlate issue characteristics (title patterns, body length, label set)
  with failure rates
- Identify which template variables are empty on failing runs vs passing runs
- Surface issues that consistently exhaust retries

### Dashboard integration
- Analysis views in `godark status` alongside existing run/issue views
- Trend charts: success rate, average retries, cost per issue over time
- Drill-down from aggregate patterns to specific failing runs

**Issues**: #183–#190

**Planning doc**: `docs/planning/phase-11-run-analysis-and-prompt-feedback.md`

