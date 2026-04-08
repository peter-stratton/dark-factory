# Scenario: Explicit integration parameter through agent layer

Relates to: Issue #769

## Setup
- A `*config.Config` value with `DockerCompose` populated (services list non-empty) and `HostServices` set.
- The agent layer entry functions in `internal/agent/loop.go`, `internal/agent/verify.go`, and `internal/agent/implementer.go` are invoked directly with the new `integration bool` parameter.

## Cases

### Implementer omits compose services when integration is false
- GIVEN a `cfg` whose `DockerCompose` lists services and `integration == false`
- WHEN `buildBasePromptData` (or its caller in `internal/agent/implementer.go`) is invoked
- THEN the returned `ComposeServices` field is the empty/zero value, regardless of `cfg.DockerCompose`

### Implementer includes compose services when integration is true
- GIVEN the same `cfg` and `integration == true`
- WHEN `buildBasePromptData` is invoked
- THEN the returned `ComposeServices` equals `buildComposeServices(cfg.DockerCompose, cfg.HostServices)`

### MountDockerSocket follows the integration parameter
- GIVEN `cfg.DockerCompose != nil` and the implementer entry function is invoked with `integration == true`
- WHEN the resulting sandbox config is constructed
- THEN `MountDockerSocket == true`

### MountDockerSocket is off when integration is false
- GIVEN `cfg.DockerCompose != nil` and the implementer entry function is invoked with `integration == false`
- WHEN the resulting sandbox config is constructed
- THEN `MountDockerSocket == false`

### Loop sandboxCommandRunner sources compose flag from parameter
- GIVEN the agent loop entry function is invoked with `integration == true`
- WHEN `sandboxCommandRunner` is constructed at the verify and re-verify call sites in `internal/agent/loop.go`
- THEN it is constructed with `useCompose == true`

### Verify sandboxCommandRunner sources compose flag from parameter
- GIVEN the agent verify entry function is invoked with `integration == false`
- WHEN `sandboxCommandRunner` is constructed at `internal/agent/verify.go`
- THEN it is constructed with `useCompose == false`

### Agent layer no longer references cfg.DockerCompose for activation
- GIVEN the post-issue source tree
- WHEN the test greps `cfg.DockerCompose != nil` across files under `internal/agent/`
- THEN zero matches are returned

### Existing agent and orchestrator tests still pass
- GIVEN the post-issue source tree with all call sites updated
- WHEN `go build ./...` and `go test ./internal/agent/... ./internal/orchestrator/...` are run
- THEN both commands exit 0 with no behavioural test failures
