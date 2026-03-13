# Scenario: OutcomeStatus type and constants

Relates to: Issue #386

## Setup
- The `internal/agent` package defines `OutcomeStatus` type and constants
- `IssueOutcome.Status` is typed `OutcomeStatus`

## Cases

### Constants have correct string values
Check each constant's underlying string value.
- `StatusImplemented` equals `"implemented"`
- `StatusReadyToMerge` equals `"ready-to-merge"`
- `StatusNeedsHumanReview` equals `"needs-human-review"`
- `StatusFailed` equals `"failed"`

### JSON marshal unchanged
Marshal an `IssueOutcome` with `Status: StatusImplemented` to JSON.
- The JSON contains `"status":"implemented"`

### No bare status strings in loop.go
Search `internal/agent/loop.go` for bare string literals `"implemented"`, `"ready-to-merge"`, `"needs-human-review"`, `"failed"` in status assignments.
- No matches found (all use named constants)

### Existing tests pass
Run `go test ./internal/agent/...`.
- All loop tests pass without modification
