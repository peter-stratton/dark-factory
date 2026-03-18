# Scenario: Map docker_compose config through DockerConfig

Relates to: Issue #557

## Setup
- `internal/sandbox/config.go` with `DockerConfig` struct
- `DockerConfigFromConfig()` function

## Cases

### Compose fields populated when configured
Call `DockerConfigFromConfig` with a config that has `DockerCompose.File` set to `"docker-compose.test.yml"` and `ProjectName` set to `"myapp"`.
- `DockerConfig.ComposeFile` equals `"docker-compose.test.yml"`
- `DockerConfig.ComposeProjectName` equals `"myapp"`

### Compose fields empty when not configured
Call `DockerConfigFromConfig` with `DockerCompose` set to nil.
- `DockerConfig.ComposeFile` equals `""`
- `DockerConfig.ComposeProjectName` equals `""`

### Other DockerConfig fields unaffected
Call `DockerConfigFromConfig` with compose config set alongside existing Docker config fields.
- Image, Mount, User, and other fields are populated as before
- Compose fields are also populated
