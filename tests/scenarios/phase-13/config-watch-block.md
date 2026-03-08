# Scenario: Config field for watch block

Relates to: Issue #239

## Setup
- The `internal/config/` package is tested via Go unit tests
- YAML config files with various `watch:` configurations

## Cases

### Config defaults
Parse a minimal `godark.yaml` with only `repo:` set.
- `Config.Watch` is nil

### Valid config
Parse a `godark.yaml` with `watch: {poll_interval: "30s"}`.
- `Config.Watch` is non-nil
- `Config.Watch.PollInterval` is `"30s"`

### Invalid interval rejected
Parse a `godark.yaml` with `watch: {poll_interval: "not-a-duration"}`.
- `Load()` returns a validation error
- Error message mentions the invalid duration

### Not configured
Parse a `godark.yaml` with no `watch:` key.
- `Config.Watch` is nil
- No validation error

### Empty poll interval valid
Parse a `godark.yaml` with `watch: {poll_interval: ""}`.
- `Config.Watch` is non-nil
- `Config.Watch.PollInterval` is `""` (consumer uses default)
