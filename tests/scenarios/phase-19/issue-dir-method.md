# Scenario: Extract issueDir method on rundata.Writer

Relates to: Issue #388

## Setup
- The `internal/rundata` package with `Writer` struct
- A temporary base directory for run data output

## Cases

### issueDir returns correct path
Create a Writer with base dir `/tmp/run`. Call `issueDir(42)`.
- Returns `/tmp/run/issues/42`

### No remaining fmt.Sprintf for issue numbers
Search `internal/rundata/writer.go` for `fmt.Sprintf("%d", issueNum)` or `fmt.Sprintf("%d", issue`.
- No matches found

### Write methods produce same paths
Write a recon result for issue 42 using the updated writer.
- The file is created at `<basedir>/issues/42/recon.json`

### Retry dir helper if present
If `issueRetryDir` was added, call it with issue 42, retry 1.
- Returns `<basedir>/issues/42/retries/1/retry.json` parent directory
