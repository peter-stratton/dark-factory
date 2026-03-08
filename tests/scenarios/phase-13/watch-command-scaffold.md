# Scenario: godark watch command scaffold

Relates to: Issue #246

## Setup
- The `internal/cmd/` package with watch command registered
- Stubbed `github.CommandRunner` for `gh pr list` and `gh api` calls
- Config with optional `watch:` block

## Cases

### Command registered
Run `godark watch --help`.
- Command exists and shows usage text
- `--repo` flag is documented

### Poll finds labeled PR
Mock `gh pr list --label "godark:awaiting-human-review"` returns one PR (#42).
Mock `gh api` for PR #42 reviews returns no `CHANGES_REQUESTED` reviews.
- Reviews are fetched for PR #42
- No label changes occur
- Poll loop continues

### Review detected and labels swapped
Mock `gh pr list` returns PR #42.
Mock reviews for PR #42 returns one `CHANGES_REQUESTED` review (ID 100).
- `github.AddIssueLabel` is called with `"godark:fixing-review-feedback"` on PR #42
- `github.RemoveIssueLabel` is called with `"godark:awaiting-human-review"` on PR #42
- Event is logged

### Duplicate review skipped
First poll: `CHANGES_REQUESTED` review ID 100 detected and processed.
Second poll: same review ID 100 returned again.
- No label changes on second poll
- Review ID 100 is tracked as already processed

### No labeled PRs
Mock `gh pr list` returns empty list.
- No review fetching occurs
- Poll loop sleeps and retries

### Default poll interval
Config has no `watch:` block.
- Watch loop uses 60 second interval

### Custom poll interval
Config has `watch: {poll_interval: "10s"}`.
- Watch loop uses 10 second interval

### Graceful shutdown
Send context cancellation (simulating SIGINT) during poll sleep.
- Watch loop exits cleanly
- No error reported
