# Scenario: Generate step in verify pipeline

Relates to: Issue #221

## Setup
- The `internal/agent/` package verify pipeline
- A mock `CommandRunner` function simulating shell command execution
- Config with various combinations of generate/build/lint/test commands

## Cases

### Generate runs first
Config has `generate_command: "make generate"`, `build_command: "go build ./..."`,
`lint_command: "golangci-lint run"`, `test_command: "go test ./..."`.
- The check list has four entries in order: generate, build, lint, test
- Generate runs before build

### Generate skipped when empty
Config has `generate_command: ""`, `build_command: "go build ./..."`.
- The check list does not contain a generate entry
- Build is the first check

### Generate fails stops pipeline
Config has `generate_command: "make generate"` and all other commands set.
Mock runner returns exit 1 for generate.
- `VerifyResult.AllPassed` is false
- Only generate result is in `Checks`
- Build, lint, and test are not executed

### Generate succeeds proceeds to build
Config has `generate_command: "make generate"` and `build_command: "go build ./..."`.
Mock runner returns exit 0 for generate and exit 0 for build.
- Both generate and build appear in `Checks` with `Passed: true`

### Only generate configured
Config has `generate_command: "make generate"` and all other commands empty.
Mock runner returns exit 0 for generate.
- `VerifyResult.AllPassed` is true
- `Checks` has one entry: generate with `Passed: true`
