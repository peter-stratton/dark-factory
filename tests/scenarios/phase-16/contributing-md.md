# Scenario: CONTRIBUTING.md

Relates to: Issue #293

## Setup
- The repo root is the working directory

## Cases

### File exists
Check that `CONTRIBUTING.md` is present at the repo root.
- `CONTRIBUTING.md` file exists

### Issues welcome
Read `CONTRIBUTING.md`.
- Contains language encouraging bug reports and feature requests

### PRs welcome with caveat
Read `CONTRIBUTING.md`.
- Contains language welcoming PRs for bug fixes and small improvements
- Notes that the project uses autonomous agents for most implementation work
- Recommends opening an issue first before large PRs

### Pipeline description
Read `CONTRIBUTING.md`.
- Mentions `godark run` or the three-agent pipeline

### Build instructions
Read `CONTRIBUTING.md`.
- Contains `go build` command
- Contains `go test` command

### Roadmap link
Read `CONTRIBUTING.md`.
- Contains a link to `docs/roadmap/`
