# Scenario: Config fields for required_env

Relates to: Issue #218

## Setup
- The `internal/config/` package is tested via Go unit tests
- The auth/env collection code for sandbox env wiring

## Cases

### Config defaults
Parse a minimal `godark.yaml` with only `repo:` set.
- `Config.RequiredEnv` is nil

### Required env parsed
Parse a `godark.yaml` with `required_env: [CLOUDSMITH_TOKEN, PUBSUB_EMULATOR_HOST]`.
- `Config.RequiredEnv` has two entries
- Entries are `"CLOUDSMITH_TOKEN"` and `"PUBSUB_EMULATOR_HOST"`

### Env values forwarded to sandbox
Set `FOO=secret` in the environment. Config has `required_env: [FOO]`.
- The sandbox env map contains key `FOO` with value `secret`
- Existing auth env vars (GH_TOKEN, etc.) are still present

### Missing env still collected
Config has `required_env: [DEFINITELY_NOT_SET]`. The variable is not in the environment.
- The sandbox env map does not contain `DEFINITELY_NOT_SET` (or contains empty string)
- No error at collection time (validation is a separate issue)

### Env values not logged
Set `SECRET_TOKEN=hunter2` in the environment. Config has `required_env: [SECRET_TOKEN]`.
- The value `hunter2` does not appear in any log output or run data files
