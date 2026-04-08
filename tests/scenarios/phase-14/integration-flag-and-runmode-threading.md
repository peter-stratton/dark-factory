# Scenario: --integration and --workers flags thread RunMode into orchestrator

Relates to: Issue #770

## Setup
- A `godark.yaml` containing `concurrency.max_workers: 4` and a `docker_compose` block with at least one service.
- The `godark run` and `godark implement` commands invoked via the Cobra command tree (using a fake orchestrator entry to capture the `RunMode` argument).
- A test orchestrator harness that records the `runMode` it receives and the semaphore cap it constructs.

## Cases

### Default no flags uses config ceiling and skips compose
- GIVEN the setup above and no flags passed to `godark run`
- WHEN the run command builds `RunMode` and dispatches the orchestrator
- THEN the captured `runMode` is `{Workers: 4, Integration: false}`, the orchestrator semaphore cap is 4, and the compose-start branch at `orchestrator.go:749` is NOT taken

### Explicit --workers 2 caps parallelism without starting compose
- GIVEN the setup above and `godark run --workers 2`
- WHEN the run command builds `RunMode` and dispatches the orchestrator
- THEN the captured `runMode` is `{Workers: 2, Integration: false}`, the semaphore cap is 2, and compose is not started

### --integration forces serial and starts compose
- GIVEN the setup above and `godark run --integration`
- WHEN the run command builds `RunMode` and dispatches the orchestrator
- THEN the captured `runMode` is `{Workers: 1, Integration: true}`, the semaphore cap is 1, the compose-start branch is taken, and agent functions receive `integration == true`

### --integration combined with --workers > 1 fails before any work
- GIVEN the setup above and `godark run --integration --workers 3`
- WHEN the run command builds `RunMode`
- THEN command exit is non-zero, stderr contains "cannot be combined", and the orchestrator entry function is NEVER called

### --workers exceeding ceiling fails before any work
- GIVEN the setup above and `godark run --workers 10`
- WHEN the run command builds `RunMode`
- THEN command exit is non-zero, stderr contains "exceeds concurrency.max_workers", and the orchestrator entry function is NEVER called

### --integration with no docker_compose block fails before any work
- GIVEN a `godark.yaml` WITHOUT a `docker_compose` block, and `godark run --integration`
- WHEN the run command builds `RunMode`
- THEN command exit is non-zero, stderr contains "requires a docker_compose block", and the orchestrator entry function is NEVER called

### Implement single-issue path receives RunMode
- GIVEN the setup above and `godark implement <issue> --integration`
- WHEN the implement command constructs the docker config at `internal/cmd/implement.go:155`
- THEN `DockerConfigFromConfig` is called with the populated `cfg.DockerCompose` argument

### Implement single-issue path passes nil compose when integration unset
- GIVEN the setup above and `godark implement <issue>` with no flags
- WHEN the implement command constructs the docker config at `internal/cmd/implement.go:155`
- THEN `DockerConfigFromConfig` is called with `nil` for the compose argument

### Config is not mutated by a --workers run
- GIVEN the setup above, a deep snapshot of `cfg`, and `godark run --workers 4`
- WHEN the run completes (or errors)
- THEN `cfg.DockerCompose` deep-equals the snapshot and `cfg.Concurrency.MaxWorkers` is unchanged

### Orchestrator no longer reads cfg.DockerCompose for activation decisions
- GIVEN the post-issue source tree
- WHEN the test greps `cfg.DockerCompose != nil` across `internal/orchestrator/orchestrator.go`
- THEN zero matches are returned for activation gates (orchestrator.go:373, :733, :749)
