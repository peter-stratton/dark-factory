# Scenario: Orchestrator and cmd fmt output replaced with progress reporter

Relates to: Issue #441

## Setup
- `orchestrator.Run()` accepts a `progress.ProgressReporter` parameter
- `processIssues()` accepts a `progress.ProgressReporter` parameter
- `cmd/run.go` creates a `TextReporter` and passes it to `orchestrator.Run()`
- `cmd/implement.go` creates a `TextReporter` and uses it for output
- A mock `ProgressReporter` is used to verify calls in tests

## Cases

### Orchestrator calls IssueCompleted for implemented outcome
Stub `processIssueFn` to return `StatusImplemented` with PR #87 and 0 retries.
- Mock reporter receives `IssueCompleted` with status `"implemented"`, prNumber 87, retries 0
- `logger.Info("issue outcome", ...)` is still called with the same fields

### Orchestrator calls IssueCompleted for failed outcome
Stub `processIssueFn` to return a failed outcome with error "sandbox timeout".
- Mock reporter receives `IssueCompleted` with status `"failed"` and errMsg `"sandbox timeout"`

### Orchestrator calls WaveStarted on re-resolution
Process two waves: first wave merges an issue, second wave has 2 newly unblocked issues.
- Mock reporter receives `WaveStarted(2, 2)` before the second batch

### Orchestrator calls AllBlocked when no issues are processable
All issues in the milestone have unresolved dependencies.
- Mock reporter receives `AllBlocked` with total and blocked counts

### Orchestrator calls RunFinished with final counts
Process 3 issues: 2 implemented, 1 failed, 1 blocked.
- Mock reporter receives `RunFinished(2, 0, 0, 1, 1)`

### Orchestrator calls RollupCreated
Rollup mode is `manual`, base branch is non-default, 1 issue was implemented.
- Mock reporter receives `RollupCreated` with the PR number, URL, and `merged: false`

### Orchestrator calls PunchlistText
A punchlist entry is generated for a completed issue.
- Mock reporter receives `PunchlistText` with the generated text

### Dry-run does not call reporter
Run with `dryRun: true`.
- Mock reporter receives zero calls
- `printDryRun()` output appears on stdout as before

### Implement command calls IssueCompleted for each issue
Run `implement` with 2 issue numbers, both returning `StatusImplemented`.
- Mock reporter receives two `IssueCompleted` calls, one per issue

### Implement command calls RunFinished with totals
Run `implement` with 3 issues: 2 implemented, 1 failed.
- Mock reporter receives `RunFinished(2, 0, 0, 1, 0)`

### Logger calls are preserved alongside reporter
Process an issue to completion.
- `logger.Info("issue outcome", ...)` is called with issue_number, status, pr_number, retries, error fields
- Reporter also receives the corresponding `IssueCompleted` call
