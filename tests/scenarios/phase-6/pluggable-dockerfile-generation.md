# Scenario: Pluggable Dockerfile generation

Relates to: Issue #53

## Setup
- The sandbox package (`internal/sandbox`) is imported directly
- `DockerConfig` structs are constructed in-test with various `Runtime` values
- No external services, network access, or Docker daemon required

## Cases

### Go runtime installs Go
Call `GenerateDockerfile` with `Runtime{Name: "go", Version: "1.26.0"}`.
- Output contains `go1.26.0.linux-amd64.tar.gz`
- Output contains `ENV PATH="/usr/local/go/bin:${PATH}"`
- Output contains `npm install -g @anthropic-ai/claude-code` (Claude Code always present)

### Flutter runtime installs Flutter SDK
Call `GenerateDockerfile` with `Runtime{Name: "flutter"}`.
- Output contains a `git clone` of the Flutter repository
- Output contains `flutter precache`
- Output contains `npm install -g @anthropic-ai/claude-code`
- Output does NOT contain a Go tarball download

### Node runtime skips extra install
Call `GenerateDockerfile` with `Runtime{Name: "node"}`.
- Output contains `npm install -g @anthropic-ai/claude-code`
- Output does NOT contain a second Node.js installation
- Output does NOT contain a Go tarball download

### Rust runtime installs via rustup
Call `GenerateDockerfile` with `Runtime{Name: "rust"}`.
- Output contains `rustup`
- Output contains `npm install -g @anthropic-ai/claude-code`

### Python runtime installs python3-venv
Call `GenerateDockerfile` with `Runtime{Name: "python"}`.
- Output contains `python3-venv`
- Output contains `npm install -g @anthropic-ai/claude-code`

### Empty runtime skips toolchain install
Call `GenerateDockerfile` with `Runtime{Name: ""}`.
- Output does NOT contain a Go tarball download
- Output does NOT contain `flutter`
- Output does NOT contain `rustup`
- Output contains `npm install -g @anthropic-ai/claude-code` (still present)
- No error is returned

### Go without version returns error
Call `GenerateDockerfile` with `Runtime{Name: "go", Version: ""}`.
- An error is returned
- The error message indicates Go version is required

### Flutter without version uses stable
Call `GenerateDockerfile` with `Runtime{Name: "flutter", Version: ""}`.
- No error is returned
- Output contains `--branch stable`

### SandboxEnv entries become ENV directives
Call `GenerateDockerfile` with `SandboxEnv: map[string]string{"GOOS": "linux", "GOARCH": "arm64"}`.
- Output contains `ENV GOOS=linux`
- Output contains `ENV GOARCH=arm64`

### Image tag changes with runtime
Generate Dockerfiles with two different runtime configs.
- The two Dockerfiles produce different image tags via `ImageTag()`

### Node.js always installed
Call `GenerateDockerfile` for each supported runtime (go, flutter, rust, python, empty).
- Every generated Dockerfile contains a Node.js installation
- Every generated Dockerfile contains `npm install -g @anthropic-ai/claude-code`
