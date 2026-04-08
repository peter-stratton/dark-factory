# Scenario: integration flag and config immutability

Relates to: Issue #771

## Setup
- A `godark.yaml` with `concurrency.max_workers` and optionally `docker_compose` configured
- CLI flags parsed via `parseCLIFlags()`

## Cases

### No flags with parallel config preserves compose verbatim
- GIVEN a config with `max_workers: 4` and a `docker_compose` block
- WHEN `applyFlags` is called with empty `CLIFlags`
- THEN `cfg.DockerCompose` is preserved verbatim (not nilled or mutated)
- AND `cfg.Concurrency.MaxWorkers` remains 4

### --integration starts compose with serial
- GIVEN a config with `max_workers: 4` and a `docker_compose` block
- WHEN `--integration` flag is set
- THEN compose is started and `Workers=1`

### --integration --workers 2 is a validation error
- GIVEN a config with `max_workers: 4` and a `docker_compose` block
- WHEN `--integration --workers 2` flags are set
- THEN a validation error is returned before any side effect

### --with-compose is rejected as unknown flag
- WHEN `godark run --with-compose` is invoked
- THEN exit code is non-zero
- AND stderr contains `unknown flag`
