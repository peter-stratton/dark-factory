# Scenario: Install Docker CLI in sandbox image

Relates to: Issue #559

## Setup
- `internal/sandbox/dockerfile.go` with `GenerateDockerfile()` function
- DockerConfig with compose fields

## Cases

### Docker CLI installed when compose configured
Generate a Dockerfile with `ComposeFile` set to a non-empty string.
- Generated Dockerfile contains `docker.io` in an apt-get install line
- Generated Dockerfile contains `docker-compose-plugin`

### Docker CLI omitted when compose not configured
Generate a Dockerfile with `ComposeFile` empty.
- Generated Dockerfile does not contain `docker.io`
- Generated Dockerfile does not contain `docker-compose-plugin`

### Docker CLI alongside other extra packages
Generate a Dockerfile with compose configured and `ExtraPackages: ["chromium"]`.
- Generated Dockerfile contains `docker.io`
- Generated Dockerfile also contains `chromium`
