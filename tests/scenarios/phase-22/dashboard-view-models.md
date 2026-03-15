# Scenario: Dashboard analysis view models and handlers

Relates to: Issue #495

## Setup
- `internal/dashboard/handlers.go` with extended `AnalysisData` struct
- `OverviewMetrics` and `FailureReasonRow` view model types
- `RepoRow` extended with `FirstPassRate` and `AvgCostPerIssueUSD`
- Stats database populated with test data

## Cases

### Overview metrics built from report
Given a Report with 10 runs, 50 issues, 40 implemented, $120 total cost.
- `Overview.TotalRuns` is 10
- `Overview.TotalIssues` is 50
- `Overview.SuccessRate` is 0.80
- `Overview.TotalCostUSD` is 120.0
- `Overview.AvgCostPerSuccessUSD` is 3.0 (120/40)

### First-pass rate in overview
Given 50 issues, 30 succeed first-pass.
- `Overview.FirstPassRate` is 0.60

### Wasted cost in overview
Given 10 failed issues with $25 total cost.
- `Overview.WastedCostUSD` is 25.0

### Failure reason rows built and sorted
Given `FailureReasons`: verify=5, exhaustion=3, timeout=1, error=1.
- 4 `FailureReasonRow` entries
- Sorted by count descending: verify, exhaustion, timeout, error
- Percentages sum to 100%

### Empty failure reasons
No failures in the data.
- `FailureReasons` slice is empty (not nil)

### Repo rows include new fields
Given RepoSummary with `FirstPassRate: 0.75` and `AvgCostPerIssueUSD: 2.50`.
- `RepoRow.FirstPassRate` is 0.75
- `RepoRow.AvgCostPerIssueUSD` is 2.50

### Both DB and FS paths produce same view models
Populate both stats.db and ~/.godark/runs/ with same data. Build analysis data from each.
- `Overview`, `FailureReasons`, and `RepoRow` fields match between DB and FS paths
