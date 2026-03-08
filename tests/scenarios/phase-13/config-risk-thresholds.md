# Scenario: Config field for risk_thresholds block

Relates to: Issue #240

## Setup
- The `internal/config/` package is tested via Go unit tests
- YAML config files with various `risk_thresholds:` configurations

## Cases

### Config defaults
Parse a minimal `godark.yaml` with only `repo:` set.
- `Config.RiskThresholds` is nil

### Valid config
Parse a `godark.yaml` with `risk_thresholds: {max_lines: 100, max_files: 5}`.
- `Config.RiskThresholds` is non-nil
- `Config.RiskThresholds.MaxLines` is `100`
- `Config.RiskThresholds.MaxFiles` is `5`

### Zero max_lines rejected
Parse a `godark.yaml` with `risk_thresholds: {max_lines: 0, max_files: 10}`.
- `Load()` returns a validation error
- Error message mentions the invalid value

### Negative max_files rejected
Parse a `godark.yaml` with `risk_thresholds: {max_lines: 200, max_files: -1}`.
- `Load()` returns a validation error
- Error message mentions the invalid value

### Not configured
Parse a `godark.yaml` with no `risk_thresholds:` key.
- `Config.RiskThresholds` is nil
- No validation error
