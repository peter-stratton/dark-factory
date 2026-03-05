# Scenario: Dashboard server and run list page

Relates to: Issue #101

## Setup
- The `internal/dashboard` package provides an HTTP server
- Tests start the server on a random port and make HTTP requests
- A temporary `~/.godark/runs/` directory with pre-written run data is used
- Templates and static assets are embedded via `//go:embed`
- No real browser, Docker, or GitHub API required

## Cases

### Server starts and responds
Start the dashboard server on a random port.
- GET `/` returns HTTP 200
- Response content type is `text/html`

### Run list shows runs
Create 2 run directories with `run.json`, then GET `/`.
- Response HTML contains both run timestamps
- Response HTML contains repo names
- Response HTML contains milestone names

### Run list sorted most recent first
Create runs with timestamps `20260301-100000` and `20260302-100000`.
- The more recent run appears first in the HTML

### Empty state message
Start with no runs directory, GET `/`.
- Response HTML contains an empty state message (not a blank page or error)

### Static assets served
GET a static asset path (e.g., `/static/htmx.min.js`).
- Returns HTTP 200
- Content type is `application/javascript`

### Run list shows summary stats
Create a run with `summary: {total: 3, implemented: 2, failed: 1}`.
- Response HTML shows the issue count
- Response HTML shows pass/fail counts

### Graceful shutdown
Start the server, send SIGINT.
- The server stops accepting new connections
- The process exits cleanly
