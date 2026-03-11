# Scenario: Config base_branch field and CLI flag

Relates to: Issue #311

## Setup
- The `internal/config/` package is tested via Go unit tests
- YAML config files with various `base_branch` values
- CLIFlags struct with BaseBranch pointer field

## Cases

### Config default
Parse a minimal `godark.yaml` with only `repo:` set.
- `Config.BaseBranch` is an empty string

### YAML override
Parse a `godark.yaml` with `base_branch: "my-feature"`.
- `Config.BaseBranch` is `"my-feature"`

### CLI flag override
Parse a `godark.yaml` with `base_branch: "yaml-branch"` and CLIFlags with `BaseBranch` set to `"cli-branch"`.
- `Config.BaseBranch` is `"cli-branch"`

### CLI flag not set
Parse a `godark.yaml` with `base_branch: "yaml-branch"` and CLIFlags with `BaseBranch` as nil.
- `Config.BaseBranch` is `"yaml-branch"`

### Empty string is valid
Parse a `godark.yaml` with `base_branch: ""`.
- `Config.BaseBranch` is an empty string
- No validation error is returned
