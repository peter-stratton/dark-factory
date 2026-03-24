# Scenario: Dashboard and TUI merge coordinator integration

Relates to: Issue #611

## Setup
- `internal/dashboard/handlers.go` contains `buildTimeline()`
- `internal/rundata/reader.go` `IssueDetail` has `MergeCoordinator StepResult`
- TUI uses string-based stage labels via `IssueStageChangedMsg`

## Cases

### Timeline includes merge coordinate step when present
Build a timeline from an `IssueDetail` with `MergeCoordinator` populated
(non-zero `DurationSeconds` and `StartedAt`).
- The timeline contains a step with name "Merge Coordinate"
- The step appears after review steps and before the final outcome

### Timeline omits merge coordinate when absent
Build a timeline from an `IssueDetail` with a zero-value `MergeCoordinator`.
- No "Merge Coordinate" step appears in the timeline

### Merge coordinate step shows telemetry fields
Build a timeline from an `IssueDetail` with `MergeCoordinator` populated
with `DurationSeconds: 120`, `PeakMemoryBytes: 500000000`,
`CPUNanoseconds: 30000000000`.
- The "Merge Coordinate" step view has formatted duration, peak memory,
  and CPU time values
- Hover tooltips are present (inherited from generic `[data-tooltip]` CSS)

### TUI displays merge-coordinate stage
Send an `IssueStageChangedMsg` with stage `"merge-coordinate"` for issue #42.
- The issue row in the TUI updates to show the merge-coordinate stage
- The stage indicator shows an in-progress marker
