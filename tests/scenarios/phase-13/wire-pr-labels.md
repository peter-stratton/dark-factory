# Scenario: Wire PR labels into orchestrator

Relates to: Issue #244

## Setup
- The `internal/agent/` package tested with stubbed `GuardRunner` and `Runner`
- The `internal/orchestrator/` package tested with stubbed GitHub API calls
- Config with `auto_merge` set to various values

## Cases

### None mode labels PR awaiting review
Run `ProcessIssue` with `auto_merge: none`. Mock reviewer returns `APPROVED`.
- `github.AddIssueLabel` is called with `"godark:awaiting-human-review"` on the PR
- Outcome status is `"ready-to-merge"`

### All mode skips lifecycle labels
Run `ProcessIssue` with `auto_merge: all`. Mock reviewer returns `APPROVED`.
- No lifecycle label is applied to the PR
- PR is merged normally

### Escalation applies awaiting label
Run `ProcessIssue` with max retries exhausted (reviewer always returns `CHANGES_REQUESTED`).
- `github.AddIssueLabel` is called with `"godark:awaiting-human-review"` on the PR
- `needs-human-review` label is also applied (existing behavior)

### Merge removes lifecycle labels
Run `ProcessIssue` with `auto_merge: all`. Mock reviewer returns `APPROVED`.
- After merge, `github.RemoveIssueLabel` is called for each label in `label.All()`

### Labels ensured at startup
Run orchestrator `Run()` with stubbed GitHub API.
- `github.EnsureLabel` is called for each label in `label.All()`
- Labels are ensured before any issue processing begins
