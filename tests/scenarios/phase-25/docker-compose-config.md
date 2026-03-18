# Scenario: docker_compose config block and validation

Relates to: Issue #556

## Setup
- `internal/config/config.go` with `DockerCompose` struct
- Test configs with various `docker_compose` block states

## Cases

### Valid compose config
Parse a `godark.yaml` with `docker_compose.file: "docker-compose.test.yml"`.
- `Config.DockerCompose` is non-nil
- `Config.DockerCompose.File` equals `"docker-compose.test.yml"`

### Missing file field rejected
Parse a `godark.yaml` with `docker_compose:` block but no `file` field.
- Validation returns an error mentioning `file`

### Unsafe project name rejected
Parse a `godark.yaml` with `docker_compose.project_name: "../bad"`.
- Validation returns an error mentioning unsafe characters

### Valid project name accepted
Parse a `godark.yaml` with `docker_compose.project_name: "my-tests"`.
- Validation passes
- `Config.DockerCompose.ProjectName` equals `"my-tests"`

### Absent block results in nil
Parse a `godark.yaml` without a `docker_compose` block.
- `Config.DockerCompose` is nil
- No validation error
