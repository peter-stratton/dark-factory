# Scenario: Human feedback agent resumption

Relates to: Issue #249

## Setup
- The `internal/cmd/watch.go` with agent invocation wired in
- Stubbed `agent.Runner` and `github.CommandRunner`
- Run data directory with prior session data for a PR
- Mock `FetchReviewComments` returning human feedback text

## Cases

### Feedback fed to implementer
Watch detects `CHANGES_REQUESTED` review on PR #42 with body "Please fix the error handling".
- `agent.Retry()` is called
- The review body text is included in the retry prompt

### Session resumed from run data
Prior run data contains session ID `sess-abc123` for PR #42.
- `agent.Retry()` is called with session ID `"sess-abc123"`
- Agent resumes prior context

### Labels swapped after fix
Agent completes fix and pushes successfully.
- `github.RemoveIssueLabel` is called with `"godark:fixing-review-feedback"` on PR #42
- `github.AddIssueLabel` is called with `"godark:awaiting-human-review"` on PR #42

### Missing session ID falls back to cold start
No prior run data exists for PR #42.
- `agent.Retry()` is called with empty session ID
- Agent runs without session resumption (cold start)
- Fix still proceeds normally

### Run data written for watch fix
Watch-initiated fix completes.
- A new run data directory is created
- Fix step result is written to the run data

### Multiple review comments concatenated
Review has multiple inline comments plus the review body.
- All comment bodies are concatenated into a single feedback string
- The concatenated feedback is passed to `agent.Retry()`
