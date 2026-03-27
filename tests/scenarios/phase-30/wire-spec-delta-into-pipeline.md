# Scenario: Wire spec delta into pipeline

Relates to: Issue #681

## Setup
- `specdelta.Diff()` and `specdelta.Format()` exist from issue #679
- `punchlist.ReadScenarioSpec()` reads scenario spec content by issue number
- PR merge happens in `runFunctionalReviewCycle` in `internal/agent/loop.go`
- `RunDataHook` interface defines write methods for per-issue run data
- PR comments are posted via `GuardRunner("gh", "pr", "comment", ...)`

## Cases

### Spec delta posted when spec changes
Mock a merge where the scenario spec has different content before and after.
- `gh pr comment` is called with a body containing "## Spec Delta"
- The comment body includes information about added, removed, or changed cases

### No comment when no spec exists
Issue has no scenario spec before or after merge.
- `gh pr comment` is not called for spec delta
- No `spec-delta.json` is written

### No comment when spec unchanged
Scenario spec exists but is identical before and after merge.
- `gh pr comment` is not called for spec delta

### Run data written on spec change
Mock a merge with spec changes.
- `WriteSpecDelta` is called on the `RunDataHook`
- Called with the correct issue number
- `SpecDeltaData` contains non-empty `AddedCases`, `RemovedCases`, or
  `ChangedCases` fields as appropriate

### RunDataHook interface updated
Inspect `RunDataHook` interface in `internal/agent/loop.go`.
- Interface includes `WriteSpecDelta` method
- All implementations compile (production writer and test stubs)

### Spec delta JSON file written
After a merge with spec changes, check run data directory.
- `issues/<issueNum>/spec-delta.json` exists
- File contains valid JSON with `before`, `after`, `added_cases`,
  `removed_cases`, `changed_cases`, and `setup_changed` fields

### Build and vet pass
Run `go build ./...` and `go vet ./...`.
- Both complete with exit code 0
