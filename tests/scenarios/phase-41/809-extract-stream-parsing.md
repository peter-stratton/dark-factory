# Scenario: Extract Claude stream parsing functions from launcher.go

Relates to: Issue #809

## Setup
- Issue #808 is merged so `internal/agent/parser/` exists with `Event`, `EventKind`, `Usage`, and `ParseEvent`
- A captured Claude stream-json transcript is available as a test fixture - one that previously produced a known `SessionID`, `CostUSD`, `Verdict`, and `ToolTrace` via `parseRunnerOutput`
- All existing `internal/agent/` tests pass on the base branch

## Cases

### Build is clean after the move
- GIVEN the refactored repository with parsing functions moved into `internal/agent/parser/`
- WHEN `go build ./...` runs
- THEN compilation succeeds with no errors

### Existing agent tests still pass
- GIVEN the test suite under `internal/agent/`
- WHEN `go test ./internal/agent/...` runs against the refactored code
- THEN every test passes with no test source changes other than the `clone_sha_test.go` import update

### Parsing functions live in the parser package
- GIVEN the refactored repository
- WHEN `internal/agent/parser/` is inspected
- THEN it exports `ParseRunnerOutput`, `ParseRateLimitEvent`, `ExtractToolTrace`, `ExtractCloneSHA`, `ExtractVerdict`, `RunnerFinalResult`, and `RateLimitEvent`

### launcher.go no longer defines the moved symbols
- GIVEN the refactored `internal/agent/launcher.go`
- WHEN its source is searched for the original definitions of `parseRunnerOutput`, `parseRateLimitEvent`, `extractToolTrace`, `extractCloneSHA`, `extractVerdict`, `runnerFinalResult`, `rateLimitEvent`, `parseTextResetTime`, `textResetRe`, `verdictRe`, and `cloneSHARe`
- THEN none of those definitions remain in the file

### launcher.go calls the parser-qualified versions
- GIVEN the refactored `internal/agent/launcher.go`
- WHEN the 5 prior call sites (around lines 298, 309, 321, 324, 339) are inspected
- THEN each calls `parser.ParseRateLimitEvent`, `parser.ParseRunnerOutput`, `parser.ExtractVerdict`, `parser.ExtractToolTrace`, and `parser.ExtractCloneSHA` respectively

### clone_sha_test.go calls the exported parser function
- GIVEN the refactored `internal/agent/clone_sha_test.go`
- WHEN the test source is inspected
- THEN it calls `parser.ExtractCloneSHA` and the test passes when run

### Behavior is byte-for-byte equivalent on a captured transcript
- GIVEN the captured stream-json transcript fixture and the known pre-refactor outputs
- WHEN the transcript is fed through `parser.ParseRunnerOutput`
- THEN the returned `RunnerFinalResult` has the same `SessionID`, `CostUSD`, `Verdict`, and `ToolTrace` values as the pre-refactor function produced on the same input

### Argv construction stays in launcher.go
- GIVEN the refactored `internal/agent/launcher.go`
- WHEN the file is searched with `grep -n "claude -p"`
- THEN the match is found at the original argv-construction location (within lines 189-197), confirming argv was not moved
