# Scenario: thread-safe run data writer

Relates to: Issue #746

## Setup
- A `rundata.Writer` instance created via `NewWithBase` pointing at a temp directory
- Multiple goroutines calling Writer methods concurrently

## Cases

### Concurrent run.json writes do not corrupt data
- GIVEN a Writer with an initialized run.json
- WHEN 10 goroutines call SetRateLimit and ClearRateLimit concurrently
- THEN run.json is valid JSON after all goroutines complete

### Per-issue writes to different issues are unblocked
- GIVEN a Writer with an initialized run directory
- WHEN two goroutines call WriteImplementResult for different issue numbers concurrently
- THEN both `issues/<num>/implement.json` files are written correctly

### Existing writer tests pass unchanged
- GIVEN the existing writer test suite
- WHEN `go test ./internal/rundata/...` is run
- THEN all tests pass without modification
