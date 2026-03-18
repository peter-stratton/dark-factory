# Scenario: Update godark doctor for Docker socket and compose

Relates to: Issue #564

## Setup
- `internal/doctor/doctor.go` with pre-flight checks
- `godark.yaml` with optional `docker_compose` block
- Stubbed filesystem and command runners

## Cases

### Socket check passes when accessible
Config has `docker_compose` set. Docker socket exists at `/var/run/docker.sock`.
- Check passes with a success message

### Socket check fails when missing
Config has `docker_compose` set. Docker socket does not exist.
- Check fails with a message explaining the socket is not found

### Compose CLI check passes
Config has `docker_compose` set. `docker compose version` exits successfully.
- Check passes with a success message

### Compose CLI check fails
Config has `docker_compose` set. `docker compose version` returns an error.
- Check fails with a message suggesting installation

### Checks skipped without compose config
Config does not have a `docker_compose` block.
- Docker socket check is not run
- Compose CLI check is not run
- No failure or warning for compose-related checks
