# Scenario: Switch dashboard analysis to read from SQLite

Relates to: Issue #463

## Setup
- `internal/dashboard/` server with `statsDB` field on the server struct
- `~/.godark/stats.db` populated with test run data
- Dashboard server started via `godark status`

## Cases

### Analysis page loads from SQLite
Request `GET /analysis` with a populated stats.db.
- Response is 200
- Page contains outcome distribution data
- Page contains trend charts

### Repo filter works
Request `GET /analysis?repo=org/repo-a`.
- Page only shows data for org/repo-a
- Repo dropdown has org/repo-a selected

### Empty database shows no-data state
Start the dashboard with an empty stats.db.
- `/analysis` returns 200
- Page shows a "no data" or empty state message
- No error or stack trace

### Missing stats.db starts without error
Delete `~/.godark/stats.db` before starting the dashboard.
- Server starts normally
- `/analysis` shows empty state
- Other dashboard pages (runs, issues) still work from filesystem

### Server opens DB at startup and closes on shutdown
Start and then stop the dashboard server.
- Stats DB is opened during startup
- Stats DB is closed during graceful shutdown
- No file descriptor leaks
