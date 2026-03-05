# Scenario: Log viewer

Relates to: Issue #103

## Setup
- The dashboard server is started on a random port
- A temporary run directory contains a `debug.log` file with JSON lines
  (each line has `time`, `level`, `msg`, and optional structured fields)
- Tests make HTTP requests and inspect the HTML response
- Log file contains a mix of info, warn, and error entries

## Cases

### Log viewer renders entries
Create a `debug.log` with 10 JSON lines, GET `/runs/owner/repo/20260301-120000/logs`.
- Response is HTTP 200
- HTML contains a table or list with log entries
- Each entry shows timestamp, level, and message

### Level filtering
GET the log viewer with a level filter parameter (e.g., `?level=error`).
- Only error-level entries are shown
- Info and warn entries are excluded

### Search within logs
GET the log viewer with a search parameter (e.g., `?q=issue_number`).
- Only entries containing "issue_number" in the message or fields are shown
- Non-matching entries are excluded

### Pagination loads more entries
Create a `debug.log` with 100 JSON lines.
- Initial page load shows a limited number of entries (first page)
- An htmx "load more" element is present in the HTML
- Requesting the next page returns additional entries

### Breadcrumb navigation
Navigate to the log viewer page.
- Breadcrumbs show: Runs > owner/repo > timestamp > Logs
- Each breadcrumb segment is a clickable link

### Structured fields displayed
Create a log entry with extra fields like `issue_number: 42` and `pr_number: 57`.
- The structured fields are visible in the log entry display
- Fields are rendered as key-value pairs
