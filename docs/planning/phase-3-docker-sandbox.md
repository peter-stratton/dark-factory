# Phase 3: Docker Sandbox

> **Goal:** Run agents in isolated containers for safety. The user's working
> directory is never touched — all agent work happens in a container. Required
> before agent execution so that `--dangerously-skip-permissions` runs in a
> confined environment.

## Milestone

`Phase 3`

---

## Issue 20: Dockerfile generation and image management

### Description

Generate a Dockerfile for the agent execution environment and provide commands
to build, tag, and cache the image. The image must include Go, Node.js (for
Claude Code npm package), git, gh CLI, and a non-root user. Claude Code refuses
`--dangerously-skip-permissions` when running as root, so the non-root user is
a hard requirement.

The Dockerfile is generated at runtime (not shipped as a static file) so it can
adapt to configuration — e.g., the Go version, additional apt packages, or a
custom base image specified in `godark.yaml`.

### Key constraints

- Package: `internal/sandbox/dockerfile.go`
- Function: `GenerateDockerfile(cfg DockerConfig) (string, error)` returns the
  Dockerfile content as a string
- `DockerConfig` struct in `internal/sandbox/config.go` with fields:
  - `Image` (base image, default `ubuntu:22.04`)
  - `GoVersion` (default from `.tool-versions` or `1.23`)
  - `NodeVersion` (default `20`)
  - `User` (non-root username, default `devloop`)
  - `ExtraPackages` (additional apt packages to install)
- The generated Dockerfile must:
  - Install Go, Node.js, git, gh CLI, curl
  - Install Claude Code via `npm install -g @anthropic-ai/claude-code`
  - Create a non-root user with a home directory
  - Set `USER <non-root-user>` as the final directive
  - Set `WORKDIR /workspace`
- Function: `BuildImage(ctx context.Context, cfg DockerConfig, logger *slog.Logger) error`
  - Writes the generated Dockerfile to a temp dir
  - Runs `docker build -t <image-name> .`
  - Uses the `CommandRunner` pattern from `internal/github/` for testability
- Image naming: `godark-runner:<hash>` where hash is derived from the
  Dockerfile content (for cache invalidation)
- Config integration: `godark.yaml` gains a `docker:` section with fields
  mapping to `DockerConfig`

### Acceptance criteria

- [ ] `GenerateDockerfile` produces a valid Dockerfile with Go, Node.js, git, gh, Claude Code
- [ ] Generated Dockerfile creates a non-root user and sets `USER` directive
- [ ] `BuildImage` invokes `docker build` with the correct tag
- [ ] Image tag includes a content hash for cache invalidation
- [ ] `DockerConfig` is populated from `godark.yaml` `docker:` section
- [ ] Default config values work when `docker:` section is omitted from config
- [ ] `go test ./internal/sandbox/` passes

### Test cases

- **Default Dockerfile generation**: `GenerateDockerfile` with zero-value config produces Dockerfile containing `FROM ubuntu:22.04`, `npm install -g @anthropic-ai/claude-code`, `USER devloop`, `WORKDIR /workspace`
- **Custom base image**: Config with `Image: "debian:bookworm"` produces `FROM debian:bookworm`
- **Custom Go version**: Config with `GoVersion: "1.22"` produces correct Go download URL
- **Extra packages**: Config with `ExtraPackages: ["jq", "ripgrep"]` includes them in `apt-get install`
- **Non-root user creation**: Dockerfile includes `useradd -m -s /bin/bash <user>` and `USER <user>`
- **Image tag hashing**: Two identical configs produce the same tag; changing any field changes the tag
- **BuildImage invokes docker**: `BuildImage` calls `docker build` with the generated Dockerfile and tag (verified via stubbed `CommandRunner`)

---

## Issue 22: Container lifecycle management

**Blocked by**: #20

### Description

Implement the container lifecycle: create a container from the built image, run
an agent command inside it, capture stdout/stderr, and clean up the container
when done. This is the core sandbox execution engine that Phase 4 (Agent
Execution) will use to launch implementer and reviewer agents.

### Key constraints

- Package: `internal/sandbox/container.go`
- `Container` struct with fields: `ID`, `ImageTag`, `Status`
- Function: `RunContainer(ctx context.Context, opts RunOpts, logger *slog.Logger) (*RunResult, error)`
  - `RunOpts`: `ImageTag`, `Cmd []string`, `Env map[string]string`,
    `WorkDir string`, `Timeout time.Duration`
  - `RunResult`: `ExitCode int`, `Stdout string`, `Stderr string`,
    `ContainerID string`
- Execution flow:
  1. `docker create` with env vars, working dir, and command
  2. `docker start -a` to attach and capture output
  3. `docker wait` to get exit code
  4. `docker rm` for cleanup (always, even on error)
- Cleanup must run even if the context is cancelled or the command times out
- Use `CommandRunner` pattern for all `docker` CLI calls
- Timeout: if `Timeout` is set, cancel the container after that duration
  (use `docker stop` followed by `docker rm`)
- Structured logging: log container create, start, exit code, cleanup events

### Acceptance criteria

- [ ] `RunContainer` creates, starts, and cleans up a Docker container
- [ ] stdout and stderr are captured in `RunResult`
- [ ] Exit code is captured from the container
- [ ] Container is always removed after execution (even on error or timeout)
- [ ] Timeout cancels the container via `docker stop`
- [ ] Environment variables from `RunOpts.Env` are passed to the container
- [ ] All docker commands use the `CommandRunner` pattern
- [ ] `go test ./internal/sandbox/` passes

### Test cases

- **Successful run**: `RunContainer` with a simple command (e.g., `echo hello`) returns exit code 0 and captures stdout
- **Failed command**: Command that exits with code 1 returns exit code 1 in `RunResult`
- **Stderr capture**: Command that writes to stderr has it captured in `RunResult.Stderr`
- **Environment variables**: Env vars passed in `RunOpts.Env` are available inside the container (verified via stubbed runner)
- **Timeout**: Container running longer than `Timeout` is stopped and removed; `RunResult` reflects the timeout
- **Cleanup on error**: If `docker start` fails, `docker rm` is still called
- **Cleanup on context cancel**: If context is cancelled mid-execution, container is stopped and removed

---

## Issue 23: Auth and config forwarding

**Blocked by**: #22

### Description

Forward authentication tokens and Claude Code configuration into the container
so agents can access the Anthropic API and GitHub. Auth tokens must be passed as
environment variables at container runtime (never baked into the image). A
pre-configured `.claude.json` must be generated inside the container to skip
Claude Code's interactive onboarding prompts.

### Key constraints

- Package: `internal/sandbox/auth.go`
- Function: `CollectAuthEnv(logger *slog.Logger) (map[string]string, error)`
  - Collects from the host environment:
    - `ANTHROPIC_API_KEY` (API key auth)
    - `CLAUDE_CODE_OAUTH_TOKEN` (subscription-based auth via `claude setup-token`)
    - `GH_TOKEN` (GitHub auth; falls back to running `gh auth token` if not set)
  - At least one of `ANTHROPIC_API_KEY` or `CLAUDE_CODE_OAUTH_TOKEN` must be present
  - `GH_TOKEN` is required (error if missing and `gh auth token` fails)
  - Returns a `map[string]string` suitable for `RunOpts.Env`
- Function: `GenerateClaudeConfig(workDir string) string`
  - Returns the JSON content for `~/.claude.json` inside the container
  - Must include: `hasCompletedOnboarding: true`, `theme: "dark"`,
    `numStartups: 1`, project trust for the workspace path
  - See `docs/CONTEXT.md` "Claude Code interactive prompts" section for the
    exact format
- The `.claude.json` must be written to the container user's home directory
  before the agent starts — this can be done via a startup script or by
  generating it as part of the `docker create` command
- Auth tokens must never appear in logs — mask them in structured log output
- Auth tokens must never be written to the Dockerfile or image layers

### Acceptance criteria

- [ ] `CollectAuthEnv` returns `ANTHROPIC_API_KEY` when set in host environment
- [ ] `CollectAuthEnv` returns `CLAUDE_CODE_OAUTH_TOKEN` when set in host environment
- [ ] `CollectAuthEnv` returns `GH_TOKEN` from environment or `gh auth token` fallback
- [ ] Error returned when neither `ANTHROPIC_API_KEY` nor `CLAUDE_CODE_OAUTH_TOKEN` is set
- [ ] Error returned when `GH_TOKEN` is missing and `gh auth token` fails
- [ ] `GenerateClaudeConfig` produces valid JSON with onboarding and trust fields
- [ ] Auth tokens are never logged in plaintext (masked in structured logs)
- [ ] `go test ./internal/sandbox/` passes

### Test cases

- **API key present**: Host has `ANTHROPIC_API_KEY` set → included in returned map
- **OAuth token present**: Host has `CLAUDE_CODE_OAUTH_TOKEN` set → included in returned map
- **Both auth tokens present**: Both are included (Claude Code picks the one it prefers)
- **No auth tokens**: Neither `ANTHROPIC_API_KEY` nor `CLAUDE_CODE_OAUTH_TOKEN` set → error with descriptive message
- **GH_TOKEN from environment**: `GH_TOKEN` set → included in returned map
- **GH_TOKEN fallback**: `GH_TOKEN` not set, `gh auth token` succeeds → token from `gh auth token` is used
- **GH_TOKEN missing**: `GH_TOKEN` not set and `gh auth token` fails → error with descriptive message
- **Claude config content**: `GenerateClaudeConfig("/workspace")` returns JSON with `hasCompletedOnboarding: true` and project trust for `/workspace`
- **Auth masking**: Logging with auth env does not include raw token values

---

## Issue 24: Repo cloning inside container

**Blocked by**: #23

### Description

Clone the target repository inside the container at startup so agent work
happens on an isolated copy. The user's working directory is never mounted into
the container. This eliminates dirty-tree conflicts and ensures all agent
changes go through git push to the remote.

### Key constraints

- Package: `internal/sandbox/repo.go`
- Function: `CloneScript(repo string, branch string, workDir string) string`
  - Returns a shell script that clones the repo and checks out the branch
  - Uses `https://` clone URL with `GH_TOKEN` for auth (the token is in the
    environment, not hardcoded in the URL)
  - Clone destination: `workDir` (default `/workspace`)
  - If `branch` is empty, clone default branch
  - Script must configure git user name/email for commits (use
    `godark[bot]` / `noreply@godark.dev` or similar)
- The clone script runs as the first command in the container, before the agent
  command — this can be a multi-command entrypoint or a wrapper script
- Function: `EntrypointScript(cloneScript string, agentCmd string) string`
  - Combines the clone script and agent command into a single entrypoint
  - Clone runs first; if it fails, exit with error (don't start the agent)
  - Agent command runs second
- The container must have network access for cloning and pushing
- Private repos: `GH_TOKEN` in the environment enables `gh` and `git` auth
  via `gh auth setup-git` in the clone script

### Acceptance criteria

- [ ] `CloneScript` produces a shell script that clones the repo via HTTPS
- [ ] Clone script uses `GH_TOKEN` environment variable for auth (not hardcoded)
- [ ] Clone script checks out the specified branch when provided
- [ ] Clone script configures git user name and email
- [ ] Clone script runs `gh auth setup-git` for GitHub auth
- [ ] `EntrypointScript` runs clone first, then agent command
- [ ] `EntrypointScript` exits with error if clone fails (does not start agent)
- [ ] `go test ./internal/sandbox/` passes

### Test cases

- **Clone script content**: `CloneScript("owner/repo", "", "/workspace")` produces script with `git clone https://github.com/owner/repo.git /workspace`
- **Branch checkout**: `CloneScript("owner/repo", "feature-branch", "/workspace")` includes `git checkout feature-branch`
- **No branch**: `CloneScript("owner/repo", "", "/workspace")` does not include `git checkout`
- **Git config**: Clone script includes `git config user.name` and `git config user.email`
- **Auth setup**: Clone script includes `gh auth setup-git`
- **Entrypoint combines commands**: `EntrypointScript(clone, "claude -p ...")` produces script running clone then agent command
- **Entrypoint fails on clone error**: Script uses `set -e` or explicit error check so clone failure stops execution
- **Token not in URL**: Clone URL does not contain `$GH_TOKEN` or the token value — auth is handled by `gh auth setup-git`

---

## Issue 21: `--no-sandbox` flag

### Description

Add a `--no-sandbox` flag to `godark run` that bypasses Docker entirely and
runs agents directly on the host machine. This is useful for development,
debugging, and environments where Docker is unavailable. When used, a warning
is printed to inform the user that agent execution is not sandboxed.

### Key constraints

- File: `internal/cmd/run.go` (add the flag to the existing `run` command)
- Flag: `--no-sandbox` (bool, default `false`)
- Also configurable via `godark.yaml`: `no_sandbox: true`
- CLI flag overrides config file value
- When `--no-sandbox` is set:
  - Skip Docker image build and container creation
  - Run the agent command directly on the host via `exec.CommandContext`
  - Still use the `CommandRunner` pattern for testability
  - Auth collection (`CollectAuthEnv`) still runs (agent needs tokens)
  - Repo cloning is skipped (agent works in current directory or a local
    worktree — this is Phase 4's concern)
- When `--no-sandbox` is set, print a warning to stderr:
  `WARNING: Running without sandbox. Agent has full access to the host system.`
- Structured log entry: `slog.Warn("sandbox disabled", "flag", "no-sandbox")`

### Acceptance criteria

- [ ] `godark run --no-sandbox` is accepted as a valid flag
- [ ] `--no-sandbox` skips Docker image build and container creation
- [ ] Agent command runs directly on the host when `--no-sandbox` is set
- [ ] Warning message is printed to stderr when `--no-sandbox` is used
- [ ] `no_sandbox: true` in `godark.yaml` has the same effect as the flag
- [ ] CLI flag overrides config file value
- [ ] Auth collection still runs in no-sandbox mode
- [ ] `go test ./internal/cmd/` passes
- [ ] `go test ./internal/sandbox/` passes

### Test cases

- **Flag parsing**: `godark run --no-sandbox` sets the no-sandbox option to true
- **Default is sandboxed**: Without `--no-sandbox`, the sandbox path is used (Docker)
- **Warning output**: `--no-sandbox` prints warning to stderr containing "without sandbox"
- **Config file**: `no_sandbox: true` in config is respected when flag is not provided
- **Flag overrides config**: Config has `no_sandbox: false`, flag `--no-sandbox` is passed → no-sandbox mode is active
- **Auth still collected**: In no-sandbox mode, `CollectAuthEnv` is still called
- **No Docker calls**: In no-sandbox mode, no `docker` commands are invoked (verified via stubbed `CommandRunner`)
