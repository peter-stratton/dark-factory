# Scenario: --no-sandbox flag

Relates to: Issue #21

## Setup
- The cmd package (`internal/cmd`) and sandbox package (`internal/sandbox`) are imported directly
- The `CommandRunner` variable is stubbed to verify which commands are invoked (no real Docker or agent execution)
- Config loading is tested with YAML content containing `no_sandbox` field
- No external services, Docker daemon, or network access required

## Cases

### Flag is accepted
Run `godark run --no-sandbox` (or construct the command with the flag programmatically).
- The flag is parsed without error
- The no-sandbox option is set to true

### Default is sandboxed
Construct the run command without `--no-sandbox`.
- The no-sandbox option is false
- The sandbox (Docker) execution path would be used

### Warning printed to stderr
Run with `--no-sandbox` enabled.
- A warning message is output containing "without sandbox"

### Config file sets no-sandbox
Load config YAML containing `no_sandbox: true`.
- The no-sandbox option is set to true without the CLI flag

### CLI flag overrides config
Load config YAML with `no_sandbox: false` and pass `--no-sandbox` on the CLI.
- The no-sandbox option is true (flag wins)

### Auth is still collected in no-sandbox mode
Run with `--no-sandbox` and verify `CollectAuthEnv` is called.
- Auth environment variables are still collected

### No Docker commands in no-sandbox mode
Run with `--no-sandbox` and a stubbed `CommandRunner`.
- No `docker` commands are invoked via the runner
