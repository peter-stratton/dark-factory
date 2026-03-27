# Scenario: GIVEN/WHEN/THEN validation in vet scenarios

Relates to: Issue #680

## Setup
- `validateScenarioFile` in `internal/vet/scenarios.go` currently checks that
  each case has at least one `- ` outcome bullet
- The new check requires at least one `- GIVEN`, one `- WHEN`, and one `- THEN`
  bullet per case (case-insensitive)
- Existing tests in `scenarios_test.go` use plain `- outcome` bullets

## Cases

### Valid GIVEN/WHEN/THEN passes
Scenario spec with a case containing `- GIVEN x`, `- WHEN y`, `- THEN z`.
- No findings are produced for that case

### Missing GIVEN produces error
Case has `- WHEN y` and `- THEN z` but no GIVEN clause.
- An Error-severity finding is produced
- Finding message contains "GIVEN"
- Finding message contains the case name

### Missing WHEN produces error
Case has `- GIVEN x` and `- THEN z` but no WHEN clause.
- An Error-severity finding is produced
- Finding message contains "WHEN"

### Missing THEN produces error
Case has `- GIVEN x` and `- WHEN y` but no THEN clause.
- An Error-severity finding is produced
- Finding message contains "THEN"

### Case-insensitive matching
Case uses `- given x`, `- When y`, `- THEN z` (mixed case).
- No findings are produced for that case

### Multiple clauses allowed
Case has 2 GIVEN, 1 WHEN, and 3 THEN bullets.
- No findings are produced for that case

### Error message includes file path
Spec file at `phase-30/678-foo.md` has a case missing WHEN.
- Finding location includes the file path "phase-30/678-foo.md"

### Existing tests updated and passing
Run `go test ./internal/vet/...`.
- All tests pass (existing tests updated to use GIVEN/WHEN/THEN format)

### Build and vet pass
Run `go build ./...` and `go vet ./...`.
- Both complete with exit code 0
