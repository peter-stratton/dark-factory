# Scenario: Judge config block in godark.yaml

Relates to: Issue #642

## Setup
- `internal/config/config.go` contains the `Judge` struct and `Config.Judge` field
- `JudgeConfig()` method on `Config` returns the judge config with defaults applied
- YAML parsing uses the `yaml` struct tags on the `Judge` struct

## Cases

### Defaults applied when judge block absent
Parse a YAML config with no `judge:` block.
Call `JudgeConfig()`.
- Enabled is true
- DefaultIdleTimeout is 300
- ToolThrashThreshold is 3
- ToolThrashWindowSecs is 60
- TransportFailureThreshold is 10
- ContainerRetryLimit is 2

### Explicit disable
Parse YAML with `judge: { enabled: false }`.
Call `JudgeConfig()`.
- Enabled is false

### Custom thresholds override defaults
Parse YAML with `judge: { default_idle_timeout: 600, tool_thrash_threshold: 5 }`.
Call `JudgeConfig()`.
- DefaultIdleTimeout is 600
- ToolThrashThreshold is 5
- Other fields retain their defaults

### Per-role idle timeout parsed
Parse YAML with `judge: { idle_timeout_by_role: { recon: 180, implementer: 300 } }`.
Call `JudgeConfig()`.
- IdleTimeoutByRole map contains "recon" → 180 and "implementer" → 300
