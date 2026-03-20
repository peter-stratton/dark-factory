# Scenario: concurrency config block with max_workers

Relates to: Issue #593

## Setup
- `internal/config/config.go` with `Concurrency` struct
- Test configs with various `concurrency` block states

## Cases

### Valid max_workers parsed
Parse a `godark.yaml` with `concurrency.max_workers: 3`.
- `Config.Concurrency` is non-nil
- `Config.Concurrency.MaxWorkers` equals `3`

### Default value when block absent
Parse a `godark.yaml` without a `concurrency` block.
- `Config.Concurrency.MaxWorkers` equals `1`
- No validation error

### Zero rejected
Parse a `godark.yaml` with `concurrency.max_workers: 0`.
- Validation returns an error mentioning `max_workers`

### Negative value rejected
Parse a `godark.yaml` with `concurrency.max_workers: -1`.
- Validation returns an error mentioning `max_workers`

### Partial block uses default
Parse a `godark.yaml` with `concurrency:` block but no `max_workers` field.
- `Config.Concurrency.MaxWorkers` equals `1`
- No validation error
