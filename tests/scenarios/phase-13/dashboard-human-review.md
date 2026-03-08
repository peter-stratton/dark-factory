# Scenario: Dashboard human review views

Relates to: Issue #248

## Setup
- The `internal/dashboard/` package tested via Go handler tests
- Run data with various outcome statuses including `ready-to-merge`
- Dialogue JSON with entries from both AI and human reviewers

## Cases

### Awaiting review section shown
Load run detail view for a run with two issues: one `implemented`, one `ready-to-merge`.
- "PRs Awaiting Review" section is visible
- The `ready-to-merge` issue appears in the awaiting section
- The `implemented` issue does not appear in the awaiting section

### No awaiting section when all merged
Load run detail view for a run where all issues have status `implemented`.
- "PRs Awaiting Review" section is not rendered

### Filter by awaiting state
Load run detail view and apply awaiting-human filter.
- Only issues with `ready-to-merge` status are shown in the issue list
- Issues with `implemented` or `failed` status are hidden

### Human dialogue styled distinctly
Load issue detail view with dialogue entries from both `implementer` and `human` authors.
- Human-authored dialogue entries have a visually distinct style
- AI-authored dialogue entries use the existing style

### Awaiting count in run list
Load run list view with two runs: one has 2 `ready-to-merge` issues, one has none.
- First run row shows "2 awaiting" badge or count
- Second run row shows no awaiting indicator
