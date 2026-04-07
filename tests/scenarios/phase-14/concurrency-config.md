# Scenario: concurrency config block with max_workers

Relates to: Issue #745

## Setup
- A `godark.yaml` file loadable by `internal/config`
- No changes to runtime behavior - config parsing and validation only

## Cases

### Valid max_workers parsed
- GIVEN a `godark.yaml` with `concurrency.max_workers: 3`
- WHEN the config is loaded
- THEN `cfg.Concurrency.MaxWorkers` equals 3

### Absent concurrency block defaults to 1
- GIVEN a `godark.yaml` with no `concurrency` block
- WHEN the config is loaded
- THEN `cfg.Concurrency.MaxWorkers` equals 1

### Negative max_workers rejected
- GIVEN a `godark.yaml` with `concurrency.max_workers: -1`
- WHEN the config is validated
- THEN a validation error is returned mentioning max_workers
