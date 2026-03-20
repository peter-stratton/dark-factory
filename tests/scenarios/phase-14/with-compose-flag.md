# Scenario: --with-compose flag and concurrent mode logic

Relates to: Issue #597

## Setup
- Config with `concurrency.max_workers` and `docker_compose` blocks
- CLI flags parsed via `parseCLIFlags()` and applied via `applyFlags()`

## Cases

### Concurrent mode skips compose
Config has `max_workers: 3` and `docker_compose.file: "docker-compose.yml"`. No `--with-compose` flag.
- `Config.DockerCompose` is nil after flag resolution
- `Config.Concurrency.MaxWorkers` remains `3`
- A log message indicates compose was skipped due to concurrent mode

### With-compose forces serial
Config has `max_workers: 3` and `docker_compose.file: "docker-compose.yml"`. `--with-compose` flag is set.
- `Config.Concurrency.MaxWorkers` equals `1`
- `Config.DockerCompose` is non-nil and preserved

### No compose config warning
Pass `--with-compose` flag with a config that has no `docker_compose` block.
- A warning is logged mentioning `--with-compose` and missing compose config
- No error returned
- `Config.Concurrency.MaxWorkers` equals `1`

### Default serial preserves compose
Config has `max_workers: 1` and `docker_compose.file: "docker-compose.yml"`. No `--with-compose` flag.
- `Config.DockerCompose` is non-nil and preserved
- `Config.Concurrency.MaxWorkers` remains `1`

### No flag no change
Config has `max_workers: 1` and no `docker_compose` block. No `--with-compose` flag.
- `Config.DockerCompose` remains nil
- `Config.Concurrency.MaxWorkers` remains `1`
