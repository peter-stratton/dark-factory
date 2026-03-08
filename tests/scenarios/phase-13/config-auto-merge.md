# Scenario: Config field for auto_merge

Relates to: Issue #238

## Setup
- The `internal/config/` package is tested via Go unit tests
- YAML config files with various `auto_merge:` values
- The `internal/agent/` package merge flow tested with stubbed `GuardRunner`

## Cases

### Config default
Parse a minimal `godark.yaml` with only `repo:` set.
- `Config.AutoMerge` is `"none"`

### NoMerge field removed
Confirm the `Config` struct has no `NoMerge` field.
- No `NoMerge` field exists on the struct
- No `no_merge` yaml tag exists in config

### Valid value none
Parse a `godark.yaml` with `auto_merge: none`.
- `Config.AutoMerge` is `"none"`

### Valid value low_risk
Parse a `godark.yaml` with `auto_merge: low_risk`.
- `Config.AutoMerge` is `"low_risk"`

### Valid value all
Parse a `godark.yaml` with `auto_merge: all`.
- `Config.AutoMerge` is `"all"`

### Invalid value rejected
Parse a `godark.yaml` with `auto_merge: always`.
- `Load()` returns a validation error
- Error message mentions the invalid value

### Flag override
Parse a `godark.yaml` with `auto_merge: none` and CLI flag `--auto-merge all`.
- `Config.AutoMerge` is `"all"` (flag wins)

### None skips merge
Run `ProcessIssue` with `auto_merge: none` and a mock that returns `APPROVED`.
- `gh pr merge` is never called
- Outcome status is `"ready-to-merge"`

### All merges PR
Run `ProcessIssue` with `auto_merge: all` and a mock that returns `APPROVED`.
- `gh pr merge` is called
- Outcome status is `"implemented"`
