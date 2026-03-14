# Scenario: Progress reporter interface and plain-text implementation

Relates to: Issue #440

## Setup
- New package `internal/progress/` in the foundation layer
- `ProgressReporter` interface defined in `reporter.go`
- `TextReporter` struct in `text.go` implements `ProgressReporter`
- `TextReporter` writes to a configurable `io.Writer`

## Cases

### IssueCompleted writes implemented format
Create a `TextReporter` writing to a `bytes.Buffer`. Call `IssueCompleted(42, "add cost tracking", "implemented", 87, 0, "")`.
- Buffer contains `"  #42 add cost tracking — implemented (PR #87, 0 retries)\n"`

### IssueCompleted writes ready-to-merge format
Call `IssueCompleted(42, "add cost tracking", "ready-to-merge", 87, 1, "")`.
- Buffer contains `"  #42 add cost tracking — ready-to-merge (PR #87, 1 retries)\n"`

### IssueCompleted writes needs-human-review format
Call `IssueCompleted(42, "add cost tracking", "needs-human-review", 87, 0, "")`.
- Buffer contains `"  #42 add cost tracking — needs human review (PR #87)\n"`

### IssueCompleted writes failed format
Call `IssueCompleted(42, "add cost tracking", "failed", 0, 0, "timeout")`.
- Buffer contains `"  #42 add cost tracking — failed: timeout\n"`

### RunFinished writes summary line
Call `RunFinished(3, 1, 1, 1, 2)`.
- Buffer contains `"Results: 3 implemented, 1 ready-to-merge, 1 needs-human-review, 1 failed, 2 skipped (blocked)\n"`

### WaveStarted writes wave header
Call `WaveStarted(2, 3)`.
- Buffer contains `"\n--- Wave 2: 3 newly unblocked issues ---\n"`

### AllBlocked writes blocked message and summary
Call `AllBlocked(5, 5)`.
- Buffer contains `"All issues are blocked — nothing to process.\n"`
- Buffer contains `"Summary: 5 total, 5 blocked, 0 processable\n"`

### PunchlistText writes raw text
Call `PunchlistText("## Punchlist\n- item 1\n")`.
- Buffer contains the exact text passed in

### Package has no internal imports
Inspect the import statements in `internal/progress/reporter.go` and `internal/progress/text.go`.
- No imports from `github.com/phs/dark-factory/internal/` packages
- Only standard library imports are used

### Architecture vet passes with new package
Run `godark vet architecture` after adding `internal/progress/` to the foundation layer paths.
- No violations reported
