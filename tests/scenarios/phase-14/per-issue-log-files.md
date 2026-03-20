# Scenario: per-issue log files

Relates to: Issue #594

## Setup
- A run with at least two issues configured
- `RunDataWriter` with per-issue directories under `issues/{num}/`
- `logFactory` function for creating loggers

## Cases

### Per-issue debug.log created
Process a single issue through the orchestrator.
- File `issues/{num}/debug.log` exists in the run directory
- File contains JSON-formatted log entries from agent execution

### Separate log content per issue
Process two issues (e.g., #1 and #2) in the same run.
- `issues/1/debug.log` contains only issue 1's agent log entries
- `issues/2/debug.log` contains only issue 2's agent log entries
- No log lines from issue 1 appear in issue 2's log file

### Orchestrator log separate from issue logs
Process issues and verify orchestrator events.
- Run-level `debug.log` contains wave dispatch and merge decision entries
- Per-issue `debug.log` files do not contain orchestrator-level events

### Log directory created before write
Process an issue where `IssueDir` does not yet exist.
- Directory is created automatically
- Logger writes successfully without error
