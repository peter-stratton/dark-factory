## Phase 7: Review Quality & Dashboard ✅

**Goal**: Capture review telemetry, report on review quality metrics, and
surface it all in a local web dashboard for human spot-checking.

**Milestone**: `Phase 7` | **Label**: `phase-7`

### Run data
- Structured JSON files written to `~/.godark/runs/<owner>/<repo>/<timestamp>/`
- Per-run metadata (config, milestone, issue list, summary stats)
- Per-issue outcome files with per-step telemetry (implement, QA review
  cycles, retries, functional review)
- Debug log (`debug.log`) co-located in the run directory (replaces `logs/`)
- Both `godark run` and `godark implement` write the same format

### Telemetry
- Wall-clock duration per agent invocation (measured on Go side)
- Cost, tool trace, verdict, and session ID (already captured in `Result`)
- Quality review stores an array of results for multi-cycle reviews

### Quality reporting
- Flag reviews with low cost, short duration, missing diff reads, or missing
  test runs — report only, no enforcement
- Review test execution reporting: flag functional reviews that didn't create
  or run tests in `tests/review/`
- Configurable thresholds in `godark.yaml` (`quality:` block)

### Dashboard
- `godark status` serves a local web UI (Go templates + htmx + Alpine.js)
- Tech: embedded static assets, single binary, localhost only
- Run list: all runs across all repos, filterable, with summary stats and
  quality flag counts
- Run detail: per-issue outcomes with status, PR links, retry count, cost
- Issue detail: review chain timeline with expandable tool traces
- Log viewer: parsed `debug.log` with level filtering and search

**Issues**: #94–#103

**Planning doc**: `docs/planning/phase-7-review-quality-and-dashboard.md`

