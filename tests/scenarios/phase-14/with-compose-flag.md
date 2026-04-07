# Scenario: --with-compose flag and concurrent mode logic

Relates to: Issue #748

## Setup
- A `godark.yaml` with `concurrency.max_workers` and optionally `docker_compose` configured
- CLI flags parsed via `parseCLIFlags()`

## Cases

### Concurrent mode skips compose
- GIVEN a config with `max_workers: 3` and a `docker_compose` block
- WHEN flags are applied without `--with-compose`
- THEN `cfg.DockerCompose` is nil

### With-compose forces serial
- GIVEN a config with `max_workers: 3` and a `docker_compose` block
- WHEN `--with-compose` flag is set
- THEN `cfg.Concurrency.MaxWorkers` equals 1 and `cfg.DockerCompose` is preserved

### No compose config warns
- GIVEN a config with no `docker_compose` block
- WHEN `--with-compose` flag is set
- THEN a warning is logged (not an error)

### Default serial preserves compose
- GIVEN a config with `max_workers: 1` and a `docker_compose` block
- WHEN flags are applied without `--with-compose`
- THEN `cfg.DockerCompose` is preserved (not nilled)
