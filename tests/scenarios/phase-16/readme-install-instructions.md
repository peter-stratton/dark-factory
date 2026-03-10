# Scenario: README install instructions

Relates to: Issue #292

## Setup
- `README.md` exists at the repo root

## Cases

### Install section exists
Read `README.md`.
- File contains an `## Install` heading

### Homebrew command
Read the install section.
- Contains `brew install peter-stratton/dark-factory/godark`

### Go install command
Read the install section.
- Contains `go install github.com/peter-stratton/dark-factory/cmd/godark@latest`

### Binary download link
Read the install section.
- Contains a link to `https://github.com/peter-stratton/dark-factory/releases`

### License notice
Read the bottom of `README.md`.
- Contains a license notice linking to `LICENSE`
