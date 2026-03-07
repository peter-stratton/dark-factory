# Scenario: Lint command and verify config block

Relates to: Issue #176

## Setup
- A `godark.yaml` file with various config overrides
- The `internal/config/` package is tested directly via Go unit tests
- No external services required

## Cases

### Default config has correct verify defaults
Parse a minimal `godark.yaml` with only `repo: owner/repo` set.
- `Config.LintCommand` is empty string
- `Config.Verify.MaxFixAttempts` is 2
- `Config.Verify.Blocking` is true

### Lint command override
Parse a `godark.yaml` containing `lint_command: "./scripts/lint.sh"`.
- `Config.LintCommand` equals `"./scripts/lint.sh"`

### Verify block override
Parse a `godark.yaml` containing:
```yaml
verify:
  max_fix_attempts: 5
  blocking: false
```
- `Config.Verify.MaxFixAttempts` is 5
- `Config.Verify.Blocking` is false

### Verify fix prompt path
Parse a `godark.yaml` containing:
```yaml
prompts:
  verify_fix: "custom/fix.txt"
```
- `Config.Prompts.VerifyFix` equals `"custom/fix.txt"`

### Partial verify block preserves defaults
Parse a `godark.yaml` containing only `verify: {max_fix_attempts: 10}`.
- `Config.Verify.MaxFixAttempts` is 10
- `Config.Verify.Blocking` is true (default preserved)

### Empty lint command means skip
Parse a config with `lint_command: ""`.
- `Config.LintCommand` is empty string
- Verify step should treat empty lint command as "skip lint"
