# Scenario: Config fields for wait_for_checks

Relates to: Issue #219

## Setup
- The `internal/config/` package is tested via Go unit tests
- YAML config files with various `wait_for_checks:` configurations

## Cases

### Config defaults
Parse a minimal `godark.yaml` with only `repo:` set.
- `Config.WaitForChecks` is nil

### Valid config parsed
Parse a `godark.yaml` with `wait_for_checks: {timeout: "10m", required: [golangci-lint, test]}`.
- `Config.WaitForChecks` is non-nil
- `Timeout` is `"10m"`
- `Required` has two entries: `"golangci-lint"` and `"test"`

### Invalid timeout rejected
Parse a `godark.yaml` with `wait_for_checks: {timeout: "not-a-duration", required: [lint]}`.
- `Load()` returns a validation error
- Error message mentions the invalid timeout

### Empty required list rejected
Parse a `godark.yaml` with `wait_for_checks: {timeout: "5m", required: []}`.
- `Load()` returns a validation error
- Error message indicates required checks list is empty

### Not configured preserves behavior
Parse a `godark.yaml` with no `wait_for_checks:` key.
- `Config.WaitForChecks` is nil
- No validation errors related to wait_for_checks
