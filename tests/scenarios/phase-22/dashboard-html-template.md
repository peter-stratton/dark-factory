# Scenario: Dashboard analysis HTML template

Relates to: Issue #496

## Setup
- `internal/dashboard/templates/analysis.html` with updated cards
- Dashboard server running with populated stats data
- `AnalysisData` contains all new view models from Phase 22

## Cases

### Overview metrics card renders
Request `GET /analysis` with populated data.
- Page contains an overview card with total runs, total issues, success rate, first-pass rate, total cost, avg cost per success
- All 6 metric values are visible

### Failure reason breakdown card renders
Request `GET /analysis` with failures in the data.
- Page contains a failure reason section
- Shows entries for verify, exhaustion, timeout, error (non-zero only)

### First-pass trend line on success rate chart
Request `GET /analysis` with 3+ completed runs.
- Success rate chart renders two lines: overall success rate and first-pass success rate
- Both lines have data points

### Repo table shows new columns
Request `GET /analysis` with multi-repo data.
- Repo table has columns for first-pass rate and avg cost per issue
- Values correspond to the RepoRow data

### Wasted cost and timeout rate in cost card
Request `GET /analysis` with failures and timeouts.
- Cost statistics card includes wasted cost value
- Cost statistics card includes timeout rate percentage

### Time to merge displayed
Request `GET /analysis` with implemented issues.
- Duration section includes average time to merge

### Exhausted retries row removed
Request `GET /analysis`.
- Retry statistics card does not contain an "Exhausted" row

### Empty state renders without error
Request `GET /analysis` with empty stats database.
- Page returns 200
- Cards show zero-state values (0, 0%, $0.00)
- No JavaScript errors in rendered charts
