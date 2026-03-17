# Scenario: Surface resource stats in dashboard issue detail view

Relates to: Issue #546

## Setup
- `internal/dashboard/` templates with issue detail view
- Run data with step results containing resource fields
- `godark status` serving the dashboard

## Cases

### Step table shows resource columns
View the issue detail page for a run with resource data.
- Table has "Peak Memory" and "CPU Time" columns
- Implement step shows memory formatted as MB (e.g., "200.0 MB")
- Implement step shows CPU formatted as seconds (e.g., "4.50s")

### Steps without resource data show dash
View the issue detail page for a run from before this feature.
- "Peak Memory" column shows "—" for each step
- "CPU Time" column shows "—" for each step
- Page renders without errors

### Mixed steps in same run
View a run where some steps have resource data and others don't.
- Steps with data show formatted values
- Steps without data show "—"
- No layout or rendering issues
