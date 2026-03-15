# Scenario: Update analyze CLI output with new metrics

Relates to: Issue #494

## Setup
- `internal/cmd/analyze.go` with updated output sections
- Stats database populated with test run data
- Report contains all new Phase 22 fields

## Cases

### Overview section displayed
Run `godark analyze` with populated data.
- Output contains "First-pass success rate:" with a percentage
- Output contains "Avg cost per success:" with a dollar amount
- Output contains "Wasted cost:" with a dollar amount
- Output contains "Timeout rate:" with a percentage

### Failure reasons table displayed
Run `godark analyze` with data containing failures of different types.
- Output contains a failure reasons section
- Rows appear for non-zero categories only (e.g., "verify: 3", "exhaustion: 1")
- Zero-count categories are omitted

### Time to merge displayed
Run `godark analyze` with implemented issues.
- Output contains "Avg time to merge:" with a human-readable duration

### Repo table enriched with new columns
Run `godark analyze` with multi-repo data.
- Repo table includes a "First-pass" column with percentages
- Repo table includes an "Avg cost" column with dollar amounts

### Exhausted count removed from retry stats
Run `godark analyze`.
- Retry stats section does not contain "Exhausted" row
- Recovery rate still displayed

### JSON output includes new fields
Run `godark analyze --json`.
- Output contains `"FirstPassSuccessRate"` key
- Output contains `"WastedCostUSD"` key
- Output contains `"FailureReasons"` key
- Output contains `"AvgCostPerSuccessUSD"` key

### No data produces clean output
Run `godark analyze` with empty stats database.
- "No runs found" message displayed
- No errors or stack traces
