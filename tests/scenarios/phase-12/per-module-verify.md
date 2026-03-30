# Scenario: Per-module verify pipeline

Relates to: Issue #223

## Setup
- The `internal/agent/` package verify pipeline with module support
- A mock `CommandRunner` function simulating shell command execution
- Config with `modules:` block containing multiple modules with dependencies

## Cases

### Modules verified in dependency order
Config has modules `service` (no deps) and `admin-cli` (depends on `service`).
- `service` verify runs before `admin-cli`

### Module commands run verbatim
Config has module `service` with `build_command: "cd service && go build ./..."`.
- The command runner receives the command exactly as written
- Module commands must include any `cd` needed to run in the correct directory

### Module inherits root commands
Config has root `lint_command: "golangci-lint run"`. Module `service` has no `lint_command`.
- `service` verify includes a lint check using the root `lint_command`

### Module-specific commands override root
Config has root `build_command: "go build ./..."`. Module `service` has `build_command: "go build ./cmd/..."`.
- `service` verify uses `"go build ./cmd/..."` not the root command

### Module failure stops dependents
Config has `service` and `admin-cli` (depends on `service`).
Mock runner returns exit 1 for `service` build.
- `service` verify fails
- `admin-cli` verify is skipped entirely

### Single module mode unchanged
Config has no `modules:` block, only root-level commands.
- Verify runs root-level commands as before
- No subdirectory scoping

### Fix prompt identifies failing module
`service` module fails its build check.
- The verify fix prompt includes the module name `"service"`
- The fix prompt includes only `service`'s error output, not other modules

### Three-level dependency chain
Config has modules `base` (no deps), `lib` (depends on `base`), `app` (depends on `lib`).
- Verify order is: `base`, `lib`, `app`
- `lib` failure skips `app` but `base` result is still recorded

### Module context in prompt templates
Config has modules defined.
- `{{.ModuleContext}}` template variable is populated
- Rendered prompt lists module names and their relationships
