# Scenario: Dashboard displays base branch on run detail page

Relates to: Issue #316

## Setup
- The `internal/dashboard/` server with test run data
- Run data with and without `BaseBranch` set in `RunMeta`

## Cases

### Detail page shows base branch when configured
Request the run detail page for a run with `BaseBranch: "feature/foo"`.
- Response body contains "feature/foo"
- Base branch is displayed near the repo and milestone metadata

### Detail page hides base branch when empty
Request the run detail page for a run with empty `BaseBranch`.
- Response body does not contain a base branch label or empty value
- No visual element for base branch is rendered
