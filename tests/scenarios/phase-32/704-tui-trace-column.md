# Scenario: TUI trace ID display for completed issues

Relates to: Issue #704

## Setup
- A TUI `Model` initialized with a set of issues via `RunStartedMsg`
- A mock `sender` that captures Bubble Tea messages for assertion
- Test stubs for `ProgressReporter` implementations (`fakeReporter`, `stubProgressReporter`, `mockReporter`) updated to accept the new `traceID` parameter

## Cases

### Trace ID shown on completed issue row
- GIVEN issue #42 is in the TUI issue table in "implement" stage
- WHEN an `IssueCompletedMsg` arrives with `TraceID` "abcd1234-5678-4abc-def0-111122223333"
- THEN the rendered row for issue #42 displays "abcd1234" (first 8 characters)

### No trace ID while issue is in progress
- GIVEN issue #42 is in the TUI issue table in "implement" stage
- WHEN no `IssueCompletedMsg` has been received yet
- THEN the rendered row for issue #42 does not show any trace ID fragment

### Empty trace ID produces no display
- GIVEN issue #42 is in the TUI issue table
- WHEN an `IssueCompletedMsg` arrives with `TraceID` set to ""
- THEN the rendered row for issue #42 does not show any trace ID fragment

### ProgressReporter interface accepts traceID
- GIVEN the `ProgressReporter` interface defines `IssueCompleted` with a `traceID string` parameter
- WHEN `TUIReporter.IssueCompleted` is called with traceID "xyz-123"
- THEN the sent `IssueCompletedMsg` has `TraceID` equal to "xyz-123"

### TextReporter logs trace ID
- GIVEN a `TextReporter` instance
- WHEN `IssueCompleted` is called with a non-empty traceID
- THEN the log output includes the trace ID string

### Production callers pass outcome TraceID
- GIVEN the orchestrator processes an issue that returns `IssueOutcome.TraceID` = "trace-abc"
- WHEN the orchestrator calls `reporter.IssueCompleted`
- THEN the `traceID` argument passed to the reporter equals "trace-abc"

### Test stubs compile with updated signature
- GIVEN `fakeReporter`, `stubProgressReporter`, and `mockReporter` implement the updated `IssueCompleted` signature
- WHEN `go build ./...` is run
- THEN compilation succeeds with no errors
