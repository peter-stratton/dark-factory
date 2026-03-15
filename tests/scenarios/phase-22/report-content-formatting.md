# Scenario: Report content and formatting

Relates to: Issue #497

## Setup
- `godark report` command with `--format` flag (terminal, markdown, html)
- Stats database populated with sprint-period run data
- `SprintReport` struct computed from queried data

## Cases

### Terminal format renders readable summary
Run `godark report --since 2w --format terminal` with 3 runs and 15 issues.
- Output contains a header with the date range
- Output contains issues processed, implemented, failed counts
- Output contains success rate and first-pass rate percentages
- Output contains total cost and avg cost per success
- Output is formatted with aligned columns (tabwriter style)

### Markdown format produces valid markdown
Run `godark report --since 2w --format markdown`.
- Output contains `##` section headers
- Output contains `**bold**` metric values
- Output contains a markdown table for notable failures (if any)
- Output is pasteable into Slack or a wiki

### HTML format produces self-contained document
Run `godark report --since 2w --format html`.
- Output contains `<html>` opening tag
- Output contains inline styles (no external CSS dependencies)
- Output contains metric values in styled elements
- Output is viewable in a browser without additional files

### Notable failures listed
Data contains 3 failed issues with varying costs.
- Report includes a notable failures section
- Issues listed by cost descending
- Each entry includes issue number, title, and error message

### No notable failures when all succeed
All issues in the period implemented successfully.
- Notable failures section is omitted (not an empty table)

### Empty period shows clear message
Run `godark report --since 1d` with no runs in the last day.
- Output shows "No runs found in this period" (or equivalent)
- No error or stack trace

### Report respects repo filter
Run `godark report --since 2w --repo org/repo-a` with data from multiple repos.
- Report only includes runs and issues from org/repo-a
- Metrics computed from repo-a data only

### Failure reasons included in report
Data contains verify failures, timeouts, and exhaustions.
- Report includes a failure reason breakdown section
- Shows counts for each category

### Wasted cost highlighted
Data contains $15 of wasted cost on failed issues.
- Report includes wasted cost metric
- Value is formatted as dollar amount
