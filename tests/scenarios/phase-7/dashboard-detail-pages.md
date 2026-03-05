# Scenario: Run detail and issue detail pages

Relates to: Issue #102

## Setup
- The dashboard server is started on a random port
- A temporary runs directory contains a complete run with multiple issues,
  including outcomes, reviews, retries, and tool traces
- Tests make HTTP requests and inspect the HTML response
- No real browser required — HTML content is asserted via string matching

## Cases

### Run detail page shows issues
GET `/runs/owner/repo/20260301-120000`.
- Response is HTTP 200
- HTML contains each issue number and title
- HTML contains status for each issue (implemented, failed, etc.)

### Status color coding
Create a run with one implemented issue and one failed issue.
- The implemented issue row has a success-style indicator
- The failed issue row has an error-style indicator

### PR links present
Create an issue outcome with `pr_number: 57`.
- HTML contains a link to the GitHub PR (e.g., `github.com/owner/repo/pull/57`)

### Issue detail page shows timeline
GET `/runs/owner/repo/20260301-120000/issues/42`.
- Response is HTTP 200
- HTML shows the implement step
- HTML shows review steps in order
- HTML shows retry steps if present

### Each step shows telemetry
The issue detail page displays step data.
- Duration is shown for each step
- Cost is shown for each step
- Verdict is shown for review steps

### Tool trace is expandable
The issue detail page includes tool traces.
- Tool trace is present in the HTML
- The trace section has an Alpine.js toggle attribute (e.g., `x-show`, `@click`)

### Breadcrumb navigation
Navigate to the issue detail page.
- Breadcrumbs show: Runs > owner/repo > timestamp > Issue #42
- Each breadcrumb segment is a clickable link
