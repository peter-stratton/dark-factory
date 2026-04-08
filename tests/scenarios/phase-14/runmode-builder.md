# Scenario: RunMode type and BuildRunMode constructor

Relates to: Issue #768

## Setup
- A `*config.Config` value loaded from a minimal in-memory YAML, with `Concurrency.MaxWorkers` set to a known ceiling and `DockerCompose` either populated or nil per case.
- A `config.CLIFlags` value constructed inline per case (no flags by default).

## Cases

### Default no flags resolves workers from config ceiling
- GIVEN `cfg.Concurrency.MaxWorkers == 4` and an empty `CLIFlags{}`
- WHEN `BuildRunMode(cfg, flags)` is called
- THEN it returns `RunMode{Workers: 4, Integration: false}` and no error

### Explicit --workers respected within ceiling
- GIVEN `cfg.Concurrency.MaxWorkers == 4` and `CLIFlags{Workers: ptr(2)}`
- WHEN `BuildRunMode(cfg, flags)` is called
- THEN it returns `RunMode{Workers: 2, Integration: false}` and no error

### --integration forces serial regardless of ceiling
- GIVEN `cfg.Concurrency.MaxWorkers == 4`, `cfg.DockerCompose != nil`, and `CLIFlags{Integration: ptr(true)}`
- WHEN `BuildRunMode(cfg, flags)` is called
- THEN it returns `RunMode{Workers: 1, Integration: true}` and no error

### --integration without docker_compose block errors
- GIVEN `cfg.DockerCompose == nil` and `CLIFlags{Integration: ptr(true)}`
- WHEN `BuildRunMode(cfg, flags)` is called
- THEN it returns a non-nil error whose message contains "requires a docker_compose block"

### --integration combined with --workers > 1 errors
- GIVEN `cfg.DockerCompose != nil`, `cfg.Concurrency.MaxWorkers == 4`, and `CLIFlags{Integration: ptr(true), Workers: ptr(2)}`
- WHEN `BuildRunMode(cfg, flags)` is called
- THEN it returns a non-nil error whose message contains "cannot be combined"

### --workers exceeding ceiling errors
- GIVEN `cfg.Concurrency.MaxWorkers == 4` and `CLIFlags{Workers: ptr(10)}`
- WHEN `BuildRunMode(cfg, flags)` is called
- THEN it returns a non-nil error whose message contains "exceeds concurrency.max_workers"

### --workers below 1 errors
- GIVEN `cfg.Concurrency.MaxWorkers == 4` and `CLIFlags{Workers: ptr(0)}`
- WHEN `BuildRunMode(cfg, flags)` is called
- THEN it returns a non-nil error whose message contains "must be >= 1"

### Constructor does not mutate config
- GIVEN any `cfg` populated with `DockerCompose`, `Concurrency.MaxWorkers`, and a deep-copied snapshot taken before the call
- WHEN `BuildRunMode(cfg, flags)` is called for each of the success and error cases above
- THEN `cfg` deep-equals the snapshot afterwards in every case
