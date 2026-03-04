# Scenario: Generic runtime config

Relates to: Issue #50

## Setup
- The config package (`internal/config`) is imported directly
- The sandbox package (`internal/sandbox`) is imported directly
- A temporary directory containing a `godark.yaml` config file
- No external services or network access required

## Cases

### Runtime struct populated from YAML
Parse a `godark.yaml` with `runtime: {name: flutter, version: "3.41"}`.
- `Config.Runtime.Name` equals `"flutter"`
- `Config.Runtime.Version` equals `"3.41"`

### Empty runtime leaves zero value
Parse a `godark.yaml` with no `runtime:` key.
- `Config.Runtime.Name` equals `""`
- `Config.Runtime.Version` equals `""`

### SandboxEnv populated from YAML
Parse a `godark.yaml` with `sandbox_env: {GOOS: linux, GOARCH: arm64}`.
- `Config.SandboxEnv["GOOS"]` equals `"linux"`
- `Config.SandboxEnv["GOARCH"]` equals `"arm64"`

### Empty SandboxEnv leaves nil map
Parse a `godark.yaml` with no `sandbox_env:` key.
- `Config.SandboxEnv` is nil

### CrossCompile struct no longer exists
Attempt to reference `Config.CrossCompile` in code.
- The code does not compile (compile-time verification that the field is removed)

### DockerConfig maps runtime from config
Call `DockerConfigFromConfig` with `Docker{...}` and `Runtime{Name: "go", Version: "1.26.0"}`.
- `DockerConfig.Runtime.Name` equals `"go"`
- `DockerConfig.Runtime.Version` equals `"1.26.0"`

### DefaultDockerConfig has zero-valued runtime
Call `DefaultDockerConfig()`.
- `DockerConfig.Runtime.Name` equals `""`
- `DockerConfig.Runtime.Version` equals `""`
- `DockerConfig.NodeVersion` is still populated (Node.js is always needed)
