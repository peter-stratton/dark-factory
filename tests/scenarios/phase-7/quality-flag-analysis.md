# Scenario: Quality flag analysis package

Relates to: Issue #99

## Setup
- The `internal/quality` package is tested directly via Go unit tests
- Functions accept primitive values (floats, durations, string slices), not
  `agent.Result` — no dependency on the agent package
- No external services required

## Cases

### Low cost flagged
Call `CheckCostFloor(0.02, 0.10)`.
- Returns a non-nil `*Flag`
- `Flag.Code` equals `"low_cost"`
- `Flag.Message` contains the cost value

### Cost above threshold passes
Call `CheckCostFloor(0.50, 0.10)`.
- Returns nil

### Cost check disabled with zero threshold
Call `CheckCostFloor(0.02, 0.0)`.
- Returns nil (check is disabled)

### Short duration flagged
Call `CheckDuration(30*time.Second, 60*time.Second)`.
- Returns a non-nil `*Flag`
- `Flag.Code` equals `"short_duration"`

### Duration above threshold passes
Call `CheckDuration(120*time.Second, 60*time.Second)`.
- Returns nil

### Duration check disabled with zero threshold
Call `CheckDuration(30*time.Second, 0)`.
- Returns nil

### No diff read flagged
Call `CheckToolTrace([]string{"Glob", "Bash: go test ./..."})`.
- Returns a slice containing a flag with `Code` equal to `"no_diff_read"`

### No tests run flagged
Call `CheckToolTrace([]string{"Read /workspace/main.go", "Glob"})`.
- Returns a slice containing a flag with `Code` equal to `"no_tests_run"`

### Normal trace produces no flags
Call `CheckToolTrace([]string{"Read /workspace/main.go", "Bash: gh pr diff", "Bash: go test ./..."})`.
- Returns an empty slice

### Review tests neither written nor run
Call `CheckReviewTestExecution([]string{"Read /workspace/main.go"}, "tests/review/", "go test")`.
- Returns a slice containing flags for both `"no_review_tests_written"` and `"no_review_tests_run"`

### Review tests written and run
Call `CheckReviewTestExecution([]string{"Write tests/review/foo_test.go", "Bash: go test ./tests/review/..."}, "tests/review/", "go test")`.
- Returns an empty slice
