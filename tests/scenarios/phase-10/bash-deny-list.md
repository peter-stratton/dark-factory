# Scenario: Bash deny-list in agent runner

Relates to: Issue #179

## Setup
- The `internal/config/` package for config parsing
- The `internal/agent/runner/agent_runner.py` for hook behavior
- The `internal/agent/implementer.go` for env variable wiring
- Mock or unit-level testing of the deny-list hook function

## Cases

### Config defaults include destructive commands
Parse a minimal `godark.yaml` with no `denied_commands` field.
- `Config.DeniedCommands` contains `rm -rf`
- `Config.DeniedCommands` contains `git push --force`
- `Config.DeniedCommands` contains `git push -f`
- `Config.DeniedCommands` contains `git reset --hard`
- `Config.DeniedCommands` contains `git clean -f`

### Config override replaces defaults
Parse a `godark.yaml` with `denied_commands: ["rm -rf"]`.
- `Config.DeniedCommands` has exactly one entry: `rm -rf`
- Default entries like `git push --force` are not present

### Empty deny-list disables hook
Parse a `godark.yaml` with `denied_commands: []`.
- `Config.DeniedCommands` is an empty slice
- No denied commands hook is registered in the agent runner

### Matching command is blocked
The deny-list hook receives a Bash tool call with command `rm -rf /workspace`.
- Hook returns `decision: block`
- Hook returns a `systemMessage` containing the matched pattern `rm -rf`

### Non-matching command is allowed
The deny-list hook receives a Bash tool call with command `go test ./...`.
- Hook returns an empty dict (no block)

### Partial match within command
The deny-list hook receives a Bash tool call with command
`git push --force origin main`.
- Hook returns `decision: block` (substring match on `git push --force`)

### Environment variable wiring
Call `newRunOpts` with a config containing `DeniedCommands: ["rm -rf", "git push --force"]`.
- The resulting `RunOpts.Env` contains `GODARK_DENIED_COMMANDS` with value `rm -rf,git push --force`
