# Phase 25: Docker Socket Mount & Compose Lifecycle

Some projects need real databases, message queues, or emulators running during integration tests -- mocks won't cut it. Phase 25 lets godark manage a `docker-compose` stack around the agent's sandbox: bring services up before execution, tear them down after, and handle all the plumbing in between. The agent runs tests against already-running infrastructure without ever touching `docker compose` itself. Configuration is a single block in `godark.yaml`, and cleanup is guaranteed even on crashes and timeouts.

---

## Docker Compose Config Block

**What it does:** A new `docker_compose` block in `godark.yaml` tells godark which compose file to use and optionally names the compose project. When the block is absent, the feature is entirely disabled. When present, the `file` field is required.

**Example:** A Go project with a Postgres test database and Redis cache:

```yaml
docker_compose:
  file: "docker-compose.test.yml"
  project_name: "myapp-tests"
  services:
    - name: postgres
      description: "PostgreSQL 16 on port 5433, test database auto-created"
    - name: redis
      description: "Redis 7 on port 6380, no auth"
```

The `DockerCompose` struct in `internal/config/config.go`:

```go
type ComposeService struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
}

type DockerCompose struct {
    File        string           `yaml:"file"`
    ProjectName string           `yaml:"project_name"`
    Services    []ComposeService `yaml:"services"`
}
```

Validation enforces that `file` is non-empty and a relative path (no `/` prefix, no `..` traversal), and that `project_name` contains only safe characters. The `services` list is optional -- when present, service descriptions are injected into agent prompts so the implementer knows what test infrastructure is available.

---

## Docker Socket Mount

**What it does:** When docker-compose is configured, the host's Docker socket is mounted into the sandbox container so compose commands can communicate with the host Docker daemon.

**Example:** The `RunOpts` struct in `internal/sandbox/container.go` includes a `MountDockerSocket` flag:

```go
type RunOpts struct {
    Image             string
    Cmd               []string
    Env               map[string]string
    Timeout           time.Duration
    Mount             string
    MountDockerSocket bool
}
```

When `MountDockerSocket` is true, `RunContainer` adds the socket volume to the `docker create` arguments:

```go
if opts.MountDockerSocket {
    createArgs = append(createArgs, "-v", "/var/run/docker.sock:/var/run/docker.sock")
}
```

This sits alongside the existing workspace bind mount. The socket mount is read-write, giving the container full access to the host Docker daemon -- necessary for compose lifecycle commands.

---

## Docker CLI in Sandbox Image

**What it does:** When compose is configured, the generated Dockerfile installs `docker.io` and `docker-compose-plugin` so compose commands are available inside the container.

**Example:** The Dockerfile template in `internal/sandbox/dockerfile.go` includes a conditional block:

```dockerfile
{{- if .HasCompose}}
# Install Docker CLI for compose support
RUN apt-get update && apt-get install -y --no-install-recommends \
    docker.io \
    docker-compose-plugin \
    && rm -rf /var/lib/apt/lists/*
{{- end}}
```

The `HasCompose` boolean is derived from whether `DockerConfig.ComposeFile` is non-empty. When compose is not configured, no Docker packages are installed and the image stays lean.

---

## Compose Up Before Agent Execution

**What it does:** Before any agent starts processing issues, godark runs `docker compose up -d` on the host to bring up test infrastructure. The compose services run as sibling containers sharing the host network. If compose startup fails, the run aborts with a clear error.

**Example:** In `internal/orchestrator/orchestrator.go`, compose starts before the issue-processing loop:

```go
if cfg.DockerCompose != nil {
    cleanupEnvFile, err := sandbox.ComposeUp(ctx, dc, cfg.RequiredEnv, logger)
    if err != nil {
        return fmt.Errorf("starting compose services: %w", err)
    }
    defer cleanupEnvFile()
    defer sandbox.ComposeDown(dc, logger)
}
```

The `ComposeUp` function in `internal/sandbox/compose.go` builds the compose command with the configured file and project name:

```go
args := []string{"compose", "-f", dc.ComposeFile, "-p", projectName, "up", "-d"}
```

The log output confirms the startup:

```
INFO starting compose services file=docker-compose.test.yml project=myapp-tests
INFO compose services started file=docker-compose.test.yml project=myapp-tests
```

If `docker compose up` fails (missing file, port conflict, image pull error), the run aborts immediately with the compose output included in the error message.

---

## Compose Down in Deferred Cleanup

**What it does:** After the run completes -- whether successfully, on error, or after a timeout -- godark tears down compose infrastructure with `docker compose down --volumes`. Teardown failures are logged as warnings but never fail the run.

**Example:** The `ComposeDown` function in `internal/sandbox/compose.go`:

```go
func ComposeDown(dc DockerConfig, logger *slog.Logger) {
    if dc.ComposeFile == "" {
        return
    }
    projectName := dc.ComposeProjectName
    if projectName == "" {
        projectName = "godark"
    }
    out, err := CommandRunner("docker", "compose", "-f", dc.ComposeFile,
        "-p", projectName, "down", "--volumes")
    if err != nil {
        logger.Warn("docker compose down failed", "error", err, "output", string(out))
        return
    }
    logger.Info("compose services stopped", "file", dc.ComposeFile, "project", projectName)
}
```

The `--volumes` flag removes anonymous volumes to prevent disk leaks from accumulating across runs. The `defer` in the orchestrator guarantees cleanup runs regardless of exit path -- normal completion, error return, context cancellation, or panic. The LIFO defer order ensures `ComposeDown` runs before the env file cleanup function, so compose still has access to its configuration during teardown.

---

## Unique Project Names Per Issue

**What it does:** Each issue in a run gets a distinct compose project name to avoid port and container name collisions. When `project_name` is not set, names follow the pattern `godark-<issue-number>`. When set, the configured name is used as a prefix.

**Example:** The `ResolveProjectName` function in `internal/config/config.go`:

```go
func ResolveProjectName(prefix string, issueNumber int) string {
    if prefix == "" {
        return fmt.Sprintf("godark-%d", issueNumber)
    }
    s := strings.NewReplacer(" ", "-", "_", "-").Replace(strings.ToLower(prefix))
    s = invalidBranchChars.ReplaceAllString(s, "-")
    s = repeatedHyphens.ReplaceAllString(s, "-")
    s = strings.Trim(s, "-")
    if s == "" {
        return fmt.Sprintf("godark-%d", issueNumber)
    }
    return fmt.Sprintf("%s-%d", s, issueNumber)
}
```

For a run processing issues #42, #43, and #44 with `project_name: "myapp-tests"`:

| Issue | Project Name |
|-------|-------------|
| #42 | `myapp-tests-42` |
| #43 | `myapp-tests-43` |
| #44 | `myapp-tests-44` |

Without a configured project name, the same issues produce `godark-42`, `godark-43`, `godark-44`. The sanitization pass lowercases, replaces spaces and underscores with hyphens, strips non-alphanumeric characters, and collapses repeated hyphens -- ensuring the result is always a valid `docker compose -p` argument.

---

## Environment Forwarding to Compose Containers

**What it does:** Variables listed in `required_env` are forwarded to compose containers via a temporary `.env` file passed to `docker compose up`. Auth-managed variables (`ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN`, `GH_TOKEN`) are excluded to prevent credentials from leaking into test infrastructure.

**Example:** With this config:

```yaml
required_env:
  - DATABASE_URL
  - REDIS_URL
  - GH_TOKEN
```

The `collectComposeEnv` function in `internal/sandbox/compose.go` filters the list:

```go
func collectComposeEnv(requiredEnv []string) map[string]string {
    env := make(map[string]string)
    for _, name := range requiredEnv {
        if _, isAuth := authManagedVars[name]; isAuth {
            continue
        }
        if v := os.Getenv(name); v != "" {
            env[name] = v
        }
    }
    return env
}
```

`GH_TOKEN` is skipped (auth-managed). `DATABASE_URL` and `REDIS_URL` are collected from the host environment. The values are written to a temp file in sorted key order and passed via `--env-file`:

```
docker compose -f docker-compose.test.yml -p myapp-tests --env-file /tmp/godark-compose-env-123456 up -d
```

Values containing newlines or carriage returns are silently skipped to prevent line injection in the env file format. The temp file is removed in deferred cleanup after `ComposeDown` completes. Environment values are never logged.

---

## Doctor Checks for Compose

**What it does:** When `docker_compose` is configured, `godark doctor` verifies that the Docker socket is accessible and the `docker compose` CLI is installed. The checks are skipped entirely when compose is not configured.

**Example:** Running `godark doctor` in a project with compose configured:

```
$ godark doctor
  Docker installed .......................... ok
  Docker daemon running ..................... ok
  Docker socket accessible .................. ok
  docker compose CLI available .............. ok
```

The checks are added conditionally in `internal/doctor/doctor.go`:

```go
if composeConfigured {
    checks = append(checks, &Check{
        Name: "Docker socket accessible",
        Fix:  "/var/run/docker.sock is not accessible. Ensure the Docker daemon is running...",
        run: func() bool {
            return SocketStat("/var/run/docker.sock") == nil
        },
    })
    checks = append(checks, &Check{
        Name: "docker compose CLI available",
        Fix:  "The `docker compose` plugin is not installed. Install Docker Desktop or...",
        run: func() bool {
            _, err := CommandRunner("docker", "compose", "version")
            return err == nil
        },
    })
}
```

The `SocketStat` variable wraps `os.Stat` for testability. When a check fails, the `Fix` message provides actionable guidance -- what to install or what command to run.

---

## Compose Service Descriptions in Agent Prompts

**What it does:** When the `services` list is populated in the `docker_compose` config block, service names and descriptions are injected into the agent's prompt context. This tells the implementer what test infrastructure is available and how to connect to it.

**Example:** With the config from the first section, the agent's prompt includes compose service context built by `buildComposeServices()`:

```
The following Docker Compose services are running and available for integration tests:
  - postgres: PostgreSQL 16 on port 5433, test database auto-created
  - redis: Redis 7 on port 6380, no auth
```

The `PromptData` struct carries a `ComposeServices string` field populated from the config. When `services` is empty or `docker_compose` is nil, the field is blank and no compose context appears in the prompt. This lets agents write tests that connect to real infrastructure without guessing at service names or ports.
