# Scenario: Per-step timeout configuration

Relates to: Issue #634

## Setup
- `internal/config/config.go` contains the `Timeouts` struct and `Config.Timeouts` field
- `newRunOpts()` in `internal/agent/prompt.go` resolves timeout by role
- Default timeout is 30 minutes when nothing is configured

## Cases

### Role-specific timeout applied
Config with `timeouts.recon: 5m`.
Call `newRunOpts` with role `"recon"`.
- The returned `RunOpts.Timeout` is 5 minutes

### Default fallback when role not specified
Config with `timeouts.default: 20m` and no role-specific timeout for recon.
Call `newRunOpts` with role `"recon"`.
- The returned `RunOpts.Timeout` is 20 minutes

### AgentTimeout backwards compatibility
Config with only `agent_timeout: 25m` (no `timeouts` block).
Call `newRunOpts` with any role.
- The returned `RunOpts.Timeout` is 25 minutes

### Cascade priority order
Config with `agent_timeout: 30m`, `timeouts.default: 20m`, `timeouts.recon: 5m`.
- `newRunOpts("recon")` returns 5 minutes
- `newRunOpts("implementer")` returns 20 minutes (default, no implementer-specific)

### Empty config uses 30 minute default
Config with no timeout fields at all.
Call `newRunOpts` with any role.
- The returned `RunOpts.Timeout` is 30 minutes

### All role fields parsed correctly
Config with all role-specific timeouts set.
- Each role resolves to its configured value
- No parsing errors
