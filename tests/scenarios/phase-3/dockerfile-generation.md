# Scenario: Dockerfile generation and image management

Relates to: Issue #20

## Setup
- The sandbox package (`internal/sandbox`) is imported directly
- The `CommandRunner` variable is stubbed to capture `docker build` invocations (no real Docker calls)
- `DockerConfig` structs are constructed in-test with various field combinations
- No external services, network access, or Docker daemon required

## Cases

### Default config generates valid Dockerfile
Call `GenerateDockerfile` with a zero-value (default) `DockerConfig`.
- Output contains `FROM ubuntu:22.04`
- Output contains `npm install -g @anthropic-ai/claude-code`
- Output contains `USER devloop`
- Output contains `WORKDIR /workspace`
- Output installs Go, Node.js, git, gh CLI, and curl

### Custom base image
Call `GenerateDockerfile` with `Image` set to `"debian:bookworm"`.
- Output contains `FROM debian:bookworm`
- Output does not contain `ubuntu`

### Custom Go version
Call `GenerateDockerfile` with `GoVersion` set to `"1.22"`.
- Output contains a Go download URL referencing version `1.22`

### Extra packages included
Call `GenerateDockerfile` with `ExtraPackages` set to `["jq", "ripgrep"]`.
- Output contains `jq` and `ripgrep` in an `apt-get install` line

### Non-root user creation
Call `GenerateDockerfile` with default config.
- Output contains `useradd -m -s /bin/bash devloop`
- Output contains `USER devloop` after the useradd line

### Image tag uses content hash
Call the image tag function with two identical configs and then with a modified config.
- Two identical configs produce the same image tag
- Changing any config field produces a different image tag

### BuildImage invokes docker build
Call `BuildImage` with a stubbed `CommandRunner`.
- The runner receives a `docker build` command
- The `-t` flag includes the expected image tag
