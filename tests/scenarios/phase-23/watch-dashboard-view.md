# Scenario: Watch dashboard view

Relates to: Issue #518

## Setup
- `internal/dashboard/` with handler for watch status page
- Dashboard server running via `godark status`
- GitHub API stubbed or real PRs with godark labels available

## Cases

### PRs in awaiting-human-review displayed
Two PRs labeled `godark:awaiting-human-review` exist in the repo.
- Dashboard page shows both PRs with their numbers and titles
- Status badge shows "awaiting human review"

### PRs in fixing-review-feedback displayed
One PR labeled `godark:fixing-review-feedback` exists.
- Dashboard page shows the PR with appropriate badge
- Badge color differs from awaiting-human-review

### Each PR links to GitHub
PRs displayed on the watch page.
- Each PR number is a clickable link to `https://github.com/{repo}/pull/{number}`

### Navigation sidebar includes watch link
View any dashboard page.
- Sidebar contains a link to the watch status page

### No PRs shows empty state
No PRs with godark labels exist.
- Page shows "No PRs awaiting review" or similar empty state message
- No error or blank page
