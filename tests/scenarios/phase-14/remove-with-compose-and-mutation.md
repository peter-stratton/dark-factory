# Scenario: Remove --with-compose flag and applyFlags config mutation

Relates to: Issue #771

## Setup
- A `godark.yaml` loaded into a `*config.Config` with `concurrency.max_workers: 4` and a populated `docker_compose` block.
- The `godark run` and `godark implement` commands invoked via the Cobra command tree.

## Cases

### applyFlags does not mutate DockerCompose with empty flags
- GIVEN a freshly loaded `cfg` with `DockerCompose` populated and `Concurrency.MaxWorkers == 4`, plus a deep snapshot taken immediately after load
- WHEN `applyFlags(cfg, CLIFlags{})` is called
- THEN `cfg.DockerCompose` deep-equals the snapshot and `cfg.Concurrency.MaxWorkers == 4`

### applyFlags does not mutate DockerCompose for any flag combination
- GIVEN the same fresh `cfg` and the deep snapshot
- WHEN `applyFlags` is called with each combination of `Repo`, `MaxRetries`, `BaseBranch`, `DefaultBranch`, `NoJudge`, and `Model` flags set
- THEN `cfg.DockerCompose` and `cfg.Concurrency` deep-equal the snapshot in every case

### --with-compose flag is rejected
- GIVEN the post-issue binary
- WHEN `godark run --with-compose` is invoked
- THEN exit code is non-zero and stderr contains "unknown flag"

### --with-compose flag is rejected on implement
- GIVEN the post-issue binary
- WHEN `godark implement 1 --with-compose` is invoked
- THEN exit code is non-zero and stderr contains "unknown flag"

### CLIFlags struct no longer has WithCompose field
- GIVEN the post-issue source tree
- WHEN the test greps `WithCompose` across `internal/config/`
- THEN zero matches are returned

### cmdutil no longer references with-compose
- GIVEN the post-issue source tree
- WHEN the test greps `with-compose` across `internal/cmd/cmdutil.go`
- THEN zero matches are returned

### applyFlags mutation block is gone
- GIVEN the post-issue source tree
- WHEN the test greps `cfg.DockerCompose = nil` across `internal/config/config.go`
- THEN zero matches are returned

### Full build and test suite passes
- GIVEN the post-issue source tree
- WHEN `go build ./...` and `go test ./...` are run
- THEN both commands exit 0
