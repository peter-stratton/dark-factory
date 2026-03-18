# Scenario: Forward required_env to compose containers

Relates to: Issue #563

## Setup
- Config with `docker_compose` and `required_env` set
- Stubbed `CommandRunner` for `docker compose` commands
- Host environment with test variables set

## Cases

### Required env vars forwarded
Config has `required_env: ["DATABASE_URL", "PUBSUB_HOST"]`. Both are set in the host environment.
- `docker compose up` receives both variables
- Variables are available inside compose containers

### Auth-managed vars excluded
Host has `GH_TOKEN`, `ANTHROPIC_API_KEY`, and `CLAUDE_CODE_OAUTH_TOKEN` set.
- None of these appear in the compose env vars

### Missing vars silently skipped
Config has `required_env: ["DATABASE_URL", "MISSING_VAR"]`. Only `DATABASE_URL` is set.
- `DATABASE_URL` is forwarded
- No error for `MISSING_VAR`

### Env file cleaned up after compose down
A temporary `.env` file is created for compose.
- File exists during `docker compose up`
- File is removed after `docker compose down`
