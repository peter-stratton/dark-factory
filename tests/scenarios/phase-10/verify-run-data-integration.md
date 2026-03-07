# Scenario: Verify step run data integration

Relates to: Issue #182

## Setup
- The `internal/rundata/` package for writer and reader tests
- The `internal/agent/` package for hook interface and loop integration
- A temporary directory simulating the run data directory structure
- No external services required

## Cases

### Write verify result creates correct file
Call `WriteVerifyResult(42, step)` with attempt 0.
- File `issues/42/verify-0.json` is created
- File contains valid JSON with `checks`, `all_passed`, and `fix_attempted` fields

### Multiple attempts create separate files
Call `WriteVerifyResult` twice for the same issue with attempts 0 and 1.
- File `issues/42/verify-0.json` exists
- File `issues/42/verify-1.json` exists
- Each file contains the correct attempt index

### Read verify results from issue detail
Write two verify result files, then call `ReadIssueDetail`.
- `IssueDetail.VerifyResults` has two entries
- Entries are ordered by attempt index

### Missing verify files returns nil
Call `ReadIssueDetail` on an issue directory with no `verify-*.json` files.
- `IssueDetail.VerifyResults` is nil or empty slice
- No error is returned (backwards compatible)

### Hook called after verify step in loop
Process an issue with a mock `RunDataHook` where verify runs.
- `WriteVerifyResult` is called on the hook with the correct issue number
- The `VerifyStepResult` contains the check results from the verify execution

### Verify result JSON structure
Write a `VerifyStepResult` with two checks (build passed, test failed).
- JSON contains `"attempt": 0`
- JSON contains `"all_passed": false`
- JSON contains two entries in `"checks"` array
- Failed check entry has `"passed": false` and non-empty `"output"`
