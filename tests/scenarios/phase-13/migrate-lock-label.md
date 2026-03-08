# Scenario: Migrate LockLabel to internal/label/

Relates to: Issue #243

## Setup
- The `internal/label/` and `internal/lock/` packages tested via Go unit tests
- Grep/search verification that old constant is fully removed

## Cases

### InProgress constant value
Import `internal/label` and reference `label.InProgress`.
- Value equals `"godark-in-progress"`

### LockLabel removed from lock package
Verify the `internal/lock/` package source code.
- No `LockLabel` constant definition exists in `internal/lock/lock.go`
- `internal/lock/` imports `internal/label/` for the label constant

### Init uses label package
Verify `internal/cmd/init.go` source code.
- References `label.InProgress` instead of `lock.LockLabel`

### Existing lock tests pass
Run the full `internal/lock/` test suite.
- All tests pass without behavioral changes
- Lock acquire and release still use the `"godark-in-progress"` label value
