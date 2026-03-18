# Phase 25: Docker Socket Mount & Compose Lifecycle

> **Goal:** Projects with `docker-compose` test infrastructure can run integration
> tests inside the sandbox. godark manages the compose lifecycle (up before agent,
> down after) via host Docker socket mount. The agent runs tests against
> already-running infrastructure without managing containers itself.

## Milestone

`Phase 25`

---

## Issue 556: Add docker_compose config block and validation

### Description

Add a new `DockerCompose` struct and `docker_compose` YAML field to the config.
This is the user-facing configuration that tells godark which compose file to
use and optionally what project name prefix to apply.

### Key constraints

- Add `DockerCompose` struct to `internal/config/config.go` with fields:
  - `File string` (`yaml:"file"`) — path to the compose file (e.g. `docker-compose.test.yml`)
  - `ProjectName string` (`yaml:"project_name"`) — optional prefix; auto-generated if empty
- Add `DockerCompose *DockerCompose` field to the `Config` struct (nil = disabled)
- Add `validateDockerCompose()` function: file path must not be empty when block is present, project_name must match safe pattern if set
- No default value — `docker_compose` is opt-in

### Acceptance criteria

- [ ] `docker_compose` block is parsed from `godark.yaml`
- [ ] Validation rejects empty `file` when block is present
- [ ] Validation rejects unsafe `project_name` values
- [ ] Absent `docker_compose` block results in nil (feature disabled)
- [ ] Config loads successfully with and without the block

### Test cases

- **Valid config**: Parse a config with `docker_compose.file` set; verify struct is populated
- **Missing file field**: Parse a config with `docker_compose` block but no `file`; verify validation error
- **Unsafe project name**: Parse a config with `project_name: "../bad"`; verify validation error
- **Absent block**: Parse a config without `docker_compose`; verify field is nil
- **Valid project name**: Parse a config with `project_name: "my-tests"`; verify no error

---

## Issue 557: Map docker_compose config through DockerConfig

**Blocked by**: #556

### Description

Wire the new `DockerCompose` config through the `DockerConfig` struct in
`internal/sandbox/config.go` so the sandbox layer has access to compose settings.

### Key constraints

- Add `ComposeFile string` and `ComposeProjectName string` fields to `DockerConfig` in `internal/sandbox/config.go`
- Update `DockerConfigFromConfig()` to populate compose fields from `config.DockerCompose` (nil-safe)
- No behavior change — just data plumbing

### Acceptance criteria

- [ ] `DockerConfig` carries compose file path and project name
- [ ] `DockerConfigFromConfig()` populates fields when `DockerCompose` is configured
- [ ] Fields are empty strings when `DockerCompose` is nil

### Test cases

- **Compose configured**: Call `DockerConfigFromConfig` with compose config set; verify fields populated
- **Compose absent**: Call `DockerConfigFromConfig` with nil compose; verify empty strings
- **Round-trip**: Verify compose fields survive the config → DockerConfig mapping

---

## Issue 558: Mount Docker socket into sandbox container

**Blocked by**: #557

### Description

When docker-compose is configured, mount the host Docker socket into the sandbox
container so the agent (and godark's compose lifecycle commands) can communicate
with the host Docker daemon.

### Key constraints

- Modify `RunContainer` in `internal/sandbox/container.go`
- Add `MountDockerSocket bool` field to `RunOpts`
- When `MountDockerSocket` is true, add `-v /var/run/docker.sock:/var/run/docker.sock` to `docker create` args
- The caller (launcher.go or orchestrator) sets `MountDockerSocket` based on config
- Socket mount is in addition to the existing `Mount` field (bind mount for workspace)

### Acceptance criteria

- [ ] `RunOpts` has `MountDockerSocket` field
- [ ] Docker socket is mounted when `MountDockerSocket` is true
- [ ] Docker socket is not mounted when `MountDockerSocket` is false
- [ ] Existing `Mount` field behavior is unchanged

### Test cases

- **Socket mount enabled**: Stub `CommandRunner`; verify `-v /var/run/docker.sock:/var/run/docker.sock` appears in create args
- **Socket mount disabled**: Stub `CommandRunner`; verify socket mount does not appear
- **Both mounts**: Enable socket mount and workspace mount; verify both `-v` flags present

---

## Issue 559: Install Docker CLI in sandbox image

**Blocked by**: #557

### Description

When docker-compose is configured, install the Docker CLI and docker-compose
plugin in the sandbox container image so compose commands can be executed.

### Key constraints

- Modify `GenerateDockerfile()` in `internal/sandbox/dockerfile.go`
- Add a conditional block that installs `docker.io` and `docker-compose-plugin` packages when compose is configured
- Pass compose config presence as a boolean to the Dockerfile template
- The Docker CLI inside the container talks to the host daemon via the mounted socket

### Acceptance criteria

- [ ] Generated Dockerfile includes Docker CLI installation when compose is configured
- [ ] Generated Dockerfile omits Docker CLI when compose is not configured
- [ ] `docker compose` command is available inside the container when installed

### Test cases

- **Compose enabled**: Generate Dockerfile with compose config; verify `docker.io` in apt-get install
- **Compose disabled**: Generate Dockerfile without compose config; verify no Docker CLI installation
- **Package list**: Verify `docker-compose-plugin` is included alongside `docker.io`

---

## Issue 560: Run docker-compose up before agent execution

**Blocked by**: #558, #559

### Description

Before the agent starts, run `docker-compose up -d` on the host to bring up test
infrastructure. The compose containers run as siblings on the host, sharing the
host network. The agent's tests connect to them via localhost ports.

### Key constraints

- Add compose startup to `internal/orchestrator/orchestrator.go` (or a new helper in `internal/sandbox/`)
- Run `docker compose -f <file> -p <project_name> up -d` using `CommandRunner`
- Project name must be set (from config or auto-generated) to isolate from other runs
- Startup happens after image build but before `processIssues` loop
- If `docker compose up` fails, abort the run with a clear error
- Log the compose project name and file path

### Acceptance criteria

- [ ] `docker compose up -d` is called before agent execution
- [ ] Project name is passed via `-p` flag
- [ ] Compose file path is passed via `-f` flag
- [ ] Failure aborts the run with an error message
- [ ] Compose is not started when `docker_compose` is not configured

### Test cases

- **Compose starts successfully**: Stub `CommandRunner`; verify `docker compose up -d` is called with correct args
- **Compose startup fails**: Stub `CommandRunner` to return error; verify run is aborted
- **Compose not configured**: Run without compose config; verify no compose commands executed
- **Project name passed**: Verify `-p <project_name>` appears in compose command args

---

## Issue 561: Run docker-compose down in deferred cleanup

**Blocked by**: #560

### Description

After the run completes (or fails/times out), tear down the compose
infrastructure with `docker-compose down`. This must run even on panics,
context cancellation, or agent crashes.

### Key constraints

- Add deferred `docker compose -f <file> -p <project_name> down` call in the orchestrator
- Use the same project name and file as the startup step
- Deferred cleanup must run even if the orchestrator returns an error
- Add `--volumes` flag to remove anonymous volumes (avoid disk leak)
- Log success/failure of teardown but do not fail the run on teardown error
- Teardown errors are best-effort (log warning, don't propagate)

### Acceptance criteria

- [ ] `docker compose down --volumes` is called after run completion
- [ ] Cleanup runs even when the run fails or times out
- [ ] Cleanup uses the same project name and file as startup
- [ ] Teardown failure logs a warning but does not change the run exit status

### Test cases

- **Normal cleanup**: Stub compose up + down; verify down is called after run
- **Cleanup after failure**: Stub orchestrator to fail; verify compose down still called
- **Cleanup after timeout**: Cancel context; verify compose down still called
- **Teardown error**: Stub compose down to fail; verify warning logged but no error returned

---

## Issue 562: Auto-generate unique project names per issue

**Blocked by**: #556

### Description

When `project_name` is not set in the config, auto-generate a unique name per
issue to avoid port and container name collisions between concurrent runs or
between issues in the same run.

### Key constraints

- Generate project name in the format `godark-<issue-number>` (e.g. `godark-543`)
- If `project_name` is set in config, use it as a prefix: `<project_name>-<issue-number>`
- Resolution happens at issue processing time, not at config load time
- The generated name must be valid for `docker compose -p` (lowercase alphanumeric + hyphens)

### Acceptance criteria

- [ ] Empty `project_name` generates `godark-<issue-number>`
- [ ] Non-empty `project_name` generates `<project_name>-<issue-number>`
- [ ] Generated names are valid docker-compose project names
- [ ] Each issue in a run gets a distinct project name

### Test cases

- **Auto-generated name**: No project_name set, issue 42; verify `godark-42`
- **Prefixed name**: project_name "myapp", issue 42; verify `myapp-42`
- **Name validity**: Verify generated names contain only lowercase, digits, hyphens

---

## Issue 563: Forward required_env to compose containers

**Blocked by**: #560

### Description

Environment variables listed in `required_env` (and auth env vars like database
URLs) need to be available inside compose containers. Pass them through to
`docker compose up` so services can read connection strings, emulator hosts, etc.

### Key constraints

- Pass env vars to `docker compose up` via `-e KEY=VALUE` flags or by writing a temporary `.env` file
- Include all vars from `required_env` that are set in the host environment
- Do not include auth-managed vars (`ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN`, `GH_TOKEN`)
- Clean up any temporary `.env` file in the deferred cleanup
- Env vars must not be logged (may contain secrets)

### Acceptance criteria

- [ ] `required_env` values are available inside compose containers
- [ ] Auth-managed vars are excluded
- [ ] Temporary `.env` file (if used) is cleaned up
- [ ] Missing `required_env` values are silently skipped (same as sandbox behavior)

### Test cases

- **Env vars forwarded**: Set required_env vars; verify they appear in compose env
- **Auth vars excluded**: Verify `GH_TOKEN` and `ANTHROPIC_API_KEY` are not passed to compose
- **Missing vars skipped**: Set some but not all required_env vars; verify no error
- **Env file cleanup**: Verify temporary .env file is removed after compose down

---

## Issue 564: Update godark doctor for Docker socket and compose

**Blocked by**: #557

### Description

Add pre-flight checks to `godark doctor` that verify the Docker socket is
accessible and the `docker compose` CLI is available when compose is configured.

### Key constraints

- Modify `internal/doctor/doctor.go`
- Add check: `/var/run/docker.sock` exists and is readable (only when compose configured)
- Add check: `docker compose version` exits successfully (only when compose configured)
- Checks should be conditional — only run when a `godark.yaml` with `docker_compose` is present in the current directory
- If no `godark.yaml` or no compose config, skip these checks silently

### Acceptance criteria

- [ ] Doctor checks Docker socket accessibility when compose is configured
- [ ] Doctor checks `docker compose` CLI availability when compose is configured
- [ ] Checks are skipped when compose is not configured
- [ ] Failure messages explain what to install or configure

### Test cases

- **Socket exists**: Stub socket stat; verify check passes
- **Socket missing**: Stub socket stat to fail; verify check fails with helpful message
- **Compose CLI available**: Stub `docker compose version` success; verify check passes
- **Compose CLI missing**: Stub command failure; verify check fails with install instructions
- **No compose config**: No docker_compose in config; verify checks are skipped
