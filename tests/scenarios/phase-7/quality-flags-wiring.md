# Scenario: Wire quality flags into config and agent loop

Relates to: Issue #100

## Setup
- Config loading is tested with YAML strings containing the `quality:` block
- `ProcessIssue` is tested with stubbed runners and a mock `RunDataHook`
- A test logger captures warning logs for flag assertions
- No Docker, GitHub API, or real Claude invocations required

## Cases

### Quality config parsed from YAML
Parse a YAML config with `quality: {min_review_cost_usd: 0.10, min_review_duration_seconds: 60}`.
- `Config.Quality.MinReviewCostUSD` equals `0.10`
- `Config.Quality.MinReviewDurationSeconds` equals `60`

### Quality config defaults to zero
Parse a YAML config with no `quality:` block.
- `Config.Quality.MinReviewCostUSD` equals `0.0`
- `Config.Quality.MinReviewDurationSeconds` equals `0`

### Flags computed after review step
Run `ProcessIssue` with a review result that has low cost (below threshold).
- Quality analysis functions are called after the review step
- A `low_cost` flag is detected

### Flags logged as warnings
Run `ProcessIssue` with a review that triggers a quality flag.
- A warning log entry is emitted containing the flag code
- The log includes the issue number

### Flags included in run data
Run `ProcessIssue` with a mock `RunDataHook` and a review that triggers flags.
- `WriteReviewResult` is called with the quality flags included
- The flags appear in the written JSON

### Quality reviewer exempt from test execution check
Run `ProcessIssue` where the quality reviewer produces a trace with no test execution.
- `CheckReviewTestExecution` is NOT called for the quality reviewer
- No `no_review_tests_written` or `no_review_tests_run` flags are raised

### Flags do not affect outcome
Run `ProcessIssue` with a review that is APPROVED but has quality flags.
- The issue proceeds as approved (not blocked by flags)
- Flags are recorded but do not change the review verdict
