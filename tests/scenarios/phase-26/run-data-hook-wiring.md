# Scenario: Run data and RunDataHook wiring

Relates to: Issue #609

## Setup
- `internal/agent/runhook.go` defines the `RunDataHook` interface
- `internal/rundata/writer.go` implements step result writers
- `internal/rundata/reader.go` defines `IssueDetail` and loads step results
- A temporary run directory is used for write/read round-trip tests

## Cases

### RunDataHook interface includes merge coordinator method
Read `internal/agent/runhook.go`.
- The `RunDataHook` interface includes
  `WriteMergeCoordinatorResult(issueNumber int, step rundata.StepResult) error`

### Writer writes merge_coordinator.json
Create a `Writer`, call `WriteMergeCoordinatorResult(42, step)` with a
populated `StepResult`.
- File `issues/42/merge_coordinator.json` exists in the run directory
- File contents deserialize to the same `StepResult` values

### Reader populates MergeCoordinator field
Write a `merge_coordinator.json` file to `issues/42/` in a run directory.
Load the run via the reader.
- `IssueDetail.MergeCoordinator` has non-zero `DurationSeconds`
- Fields match the written JSON

### Reader handles missing merge_coordinator.json
Load a run where `issues/42/` has no `merge_coordinator.json`.
- `IssueDetail.MergeCoordinator` is a zero-value `StepResult`
- No error is returned

### All RunDataHook implementations compile
Build the project with `go build ./...`.
- No compile errors related to missing `WriteMergeCoordinatorResult` on any
  type implementing `RunDataHook`
