# Scenario: Base branch tracked in run data

Relates to: Issue #315

## Setup
- The `internal/rundata/` package writer and reader
- Temporary directories for run data output

## Cases

### Write with base branch
Create a `Writer` with `BaseBranch: "feature/foo"` and write `run.json`.
- The `run.json` file contains `"base_branch":"feature/foo"`

### Write without base branch
Create a `Writer` with empty `BaseBranch` and write `run.json`.
- The `run.json` file does not contain the key `"base_branch"`

### Read data with base branch
Load a `run.json` that includes `"base_branch":"feature/foo"`.
- `RunMeta.BaseBranch` is `"feature/foo"`

### Read old data without base branch
Load a `run.json` that does not have a `base_branch` field.
- `RunMeta.BaseBranch` is an empty string
- No error is returned
