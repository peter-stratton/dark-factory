# Scenario: godark report command scaffold

Relates to: Issue #492

## Setup
- `internal/cmd/report.go` with `godark report` subcommand registered
- `~/.godark/stats.db` may or may not exist
- Flags: `--since`, `--until`, `--repo`, `--format`

## Cases

### Command is registered
Run `godark report --help`.
- Output contains usage information for the report command
- No error

### Duration parsing weeks
Run with `--since 2w`.
- The since date resolves to 14 days before the until date

### Duration parsing days
Run with `--since 30d`.
- The since date resolves to 30 days before the until date

### Duration parsing single week
Run with `--since 1w`.
- The since date resolves to 7 days before the until date

### Invalid duration rejected
Run with `--since abc`.
- An error is returned mentioning invalid duration format

### Repo filter passed through
Run with `--repo org/my-repo`.
- The stats query filters by repo "org/my-repo"

### Format flag accepts terminal
Run with `--format terminal`.
- No error; terminal format selected

### Format flag accepts markdown
Run with `--format markdown`.
- No error; markdown format selected

### Format flag accepts html
Run with `--format html`.
- No error; html format selected

### Default format is terminal
Run with no `--format` flag.
- Terminal format is used

### Missing stats database
Delete `~/.godark/stats.db` and run `godark report`.
- Error message contains "No stats database found"
- Error message suggests running `godark run` or `godark implement`
