# Scenario: Mount Docker socket into sandbox container

Relates to: Issue #558

## Setup
- `internal/sandbox/container.go` with `RunContainer` function
- Stubbed `CommandRunner` capturing `docker create` args
- `RunOpts` with `MountDockerSocket` field

## Cases

### Socket mounted when enabled
Call `RunContainer` with `MountDockerSocket: true`.
- `docker create` args include `-v /var/run/docker.sock:/var/run/docker.sock`

### Socket not mounted when disabled
Call `RunContainer` with `MountDockerSocket: false`.
- `docker create` args do not include `/var/run/docker.sock`

### Both socket and workspace mounts
Call `RunContainer` with `MountDockerSocket: true` and `Mount: "/host:/workspace"`.
- `docker create` args include both `-v` flags
- Workspace mount is present
- Socket mount is present
