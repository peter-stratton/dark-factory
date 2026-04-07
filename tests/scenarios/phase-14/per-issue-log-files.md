# Scenario: per-issue log files

Relates to: Issue #747

## Setup
- A `rundata.Writer` with a run directory containing `issues/` subdirectories
- An orchestrator processing issues via `processIssueFn`

## Cases

### Per-issue debug.log created
- GIVEN a Writer and a processable issue #10
- WHEN the issue is processed by the orchestrator
- THEN `issues/10/debug.log` exists and contains log entries from the agent execution

### Orchestrator events stay in run-level log
- GIVEN a run processing multiple issues
- WHEN wave-start and merge events are logged
- THEN those events appear in the run-level logger output, not in any per-issue debug.log

### Nil writer falls back to run-level logger
- GIVEN a nil Writer (e.g., dry-run mode)
- WHEN an issue is processed
- THEN no panic occurs and logging goes to the run-level logger
