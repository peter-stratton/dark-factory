# Scenario: Run docker-compose down in deferred cleanup

Relates to: Issue #561

## Setup
- `internal/orchestrator/orchestrator.go` with compose lifecycle
- Stubbed `CommandRunner` for `docker compose` commands
- Config with `docker_compose.file` set

## Cases

### Cleanup runs after successful completion
Run the orchestrator with compose configured. All agents succeed.
- `docker compose down --volumes` is called after all agents finish
- Same `-f` and `-p` flags as the startup command

### Cleanup runs after run failure
Stub the orchestrator to fail mid-run.
- `docker compose down --volumes` is still called
- Cleanup uses the same project name and file

### Cleanup runs after context cancellation
Cancel the context during agent execution.
- `docker compose down --volumes` is still called

### Teardown failure does not change run result
Stub `docker compose down` to return an error.
- A warning is logged
- The run's exit status is unchanged (success stays success, failure stays failure)
