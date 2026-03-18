# Scenario: Run docker-compose up before agent execution

Relates to: Issue #560

## Setup
- `internal/orchestrator/orchestrator.go` or compose helper
- Stubbed `CommandRunner` for `docker compose` commands
- Config with `docker_compose.file` set

## Cases

### Compose starts before agent
Run the orchestrator with compose configured. Stub all external commands.
- `docker compose up -d` is called before any agent execution
- Command includes `-f <compose-file>` flag
- Command includes `-p <project-name>` flag

### Compose startup failure aborts run
Stub `docker compose up` to return a non-zero exit code.
- The run returns an error
- No agent execution occurs

### Compose skipped when not configured
Run the orchestrator without `docker_compose` in config.
- No `docker compose` commands are executed
- Agent execution proceeds normally

### Project name included in compose command
Run with `docker_compose.project_name: "myapp"` and issue number 42.
- The `-p` flag value includes the project name
