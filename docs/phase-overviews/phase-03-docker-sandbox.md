# Phase 3: Docker Sandbox

Phase 3 introduced container isolation so that autonomous agents never touch
the host filesystem. Every agent invocation -- implementation, review, spec
generation -- runs inside a purpose-built Docker container. The repo is cloned
fresh inside the container, auth tokens are forwarded as environment variables,
and a non-root user prevents Claude Code from refusing to run with
`--dangerously-skip-permissions`. If you need to skip the sandbox (CI
environments, quick local tests), a single flag turns it off.

---

## Dockerfile Generation

Generates a Dockerfile on the fly from your project config, installing the
right language toolchain, GitHub CLI, Node.js, Claude Code, and any extra
packages you specify.

The generated Dockerfile is a Go template that adapts to your `runtime:` block
in `godark.yaml`. For a Go project:

```yaml
# godark.yaml
runtime:
  name: go
  version: "1.26.0"
docker:
  image: ubuntu:22.04
  node_version: "20"
  extra_packages:
    - jq
    - ripgrep
```

This produces a Dockerfile that installs Go 1.26.0 from the official tarball,
adds Node 20 for Claude Code, installs `jq` and `ripgrep`, and creates a
non-root `devloop` user. The image is tagged with a content-addressable hash
(`godark-runner:a1b2c3d4e5f6`) so rebuilds are skipped when nothing changes.

For a Flutter project, swap `runtime.name` to `flutter` and the Go tarball
stanza is replaced with a Flutter SDK clone. The rest of the image stays
identical.

---

## Container Lifecycle

Manages the full create-start-wait-logs-cleanup cycle for each agent run.
The container is always removed after execution, even on failure or timeout.

When `godark run` dispatches an agent, the sandbox package:

1. Calls `docker create` with the image tag and environment variables
2. Calls `docker start` to begin execution
3. Calls `docker wait` to block until the container exits (or the timeout fires)
4. Calls `docker logs` to capture stdout and stderr separately
5. Calls `docker rm -f` to clean up -- always, via a deferred cleanup

A typical log sequence looks like:

```
INFO running container  name=godark-k7x2m9ab  image=godark-runner:a1b2c3d4e5f6
INFO container created  id=sha256:8f3a...
INFO container finished name=godark-k7x2m9ab  exit_code=0  timed_out=false
INFO removing container name=godark-k7x2m9ab
```

If the container exceeds the configured `agent_timeout`, the sandbox stops it,
marks the result as timed out, and the orchestrator treats it as a failure.
There is also a wall-clock safety net that catches cases where macOS sleep
suspends the monotonic clock but wall time keeps advancing.

---

## Auth Forwarding

Reads authentication tokens from the host environment and passes them into the
container so agents can call the Anthropic API and interact with GitHub.

`CollectAuthEnv` resolves three tokens:

- **Anthropic auth**: prefers `CLAUDE_CODE_OAUTH_TOKEN` by default, falls back
  to `ANTHROPIC_API_KEY`. The `auth_preference` config field lets you flip
  this (`"api_key"` prefers the API key).
- **GitHub token**: reads `GH_TOKEN` from the environment, or shells out to
  `gh auth token` as a fallback. Fails fast if neither is available.
- **Required env**: any additional variables listed in `required_env` (e.g.
  `CLOUDSMITH_TOKEN`) are forwarded from the host. Auth-managed variables
  cannot be overridden through this mechanism.

None of these values are logged. The log output shows only the key names:

```
INFO collected auth env  keys=[ANTHROPIC_API_KEY, GH_TOKEN]
```

---

## Repo Cloning Inside the Container

The target repository is cloned inside the container rather than bind-mounted
from the host. This guarantees the host working directory is never modified.

The clone script runs as the first step of the container entrypoint:

```sh
set -e
gh auth setup-git
git config --global user.name "dark-factory"
git config --global user.email "dark-factory@noreply"
git clone https://github.com/owner/repo.git /workspace
cd /workspace && git checkout feature-branch
```

The agent command runs immediately after the clone. If a branch is specified
(as it is for retry runs against an existing PR), the script checks it out
after cloning.

---

## Non-Root User

The container runs as a non-root user named `devloop`. Claude Code refuses to
run with `--dangerously-skip-permissions` as root, so this is a hard
requirement.

The Dockerfile handles it:

```dockerfile
RUN useradd -m -s /bin/bash devloop
USER devloop
WORKDIR /workspace
```

The username is configurable via `docker.user` in `godark.yaml`, but the
default works for all standard cases. The workspace directory is owned by
this user, so the agent can clone, write files, and run tests without
permission issues.

---

## The `--no-sandbox` Flag

Bypasses Docker entirely and runs the agent directly on the host. Useful for
CI environments that already provide isolation, or for quick local debugging.

From the CLI:

```
godark run --milestone "Phase 4" --repo owner/repo --no-sandbox
```

Or in `godark.yaml`:

```yaml
no_sandbox: true
```

The CLI flag always wins over the config file. When `--no-sandbox` is active,
the launcher writes the embedded `agent_runner.py` to a temp file on the host,
sets the prompt and role via environment variables, and runs `python3` directly.
The same structured output parsing applies -- session ID, cost, verdict, and
tool trace are extracted identically regardless of execution mode.

---

## Image Caching

The image tag is a content-addressable SHA-256 hash of the generated
Dockerfile. If your config does not change between runs, the tag stays the
same and Docker's layer cache makes the build a no-op.

```go
// ImageTag returns "godark-runner:<first-12-chars-of-sha256>"
func ImageTag(dockerfileContent string) string {
    h := sha256.Sum256([]byte(dockerfileContent))
    return fmt.Sprintf("godark-runner:%x", h[:6])
}
```

Change the Go version, add an extra package, or modify the base image, and
the tag changes, triggering a fresh build. Leave the config alone and the
image is reused instantly.
