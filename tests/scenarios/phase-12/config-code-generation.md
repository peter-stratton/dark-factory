# Scenario: Config fields for code generation

Relates to: Issue #216

## Setup
- The `internal/config/` package is tested via Go unit tests
- YAML config files with various `generate_command` and `generated_paths` values

## Cases

### Config defaults
Parse a minimal `godark.yaml` with only `repo:` set.
- `Config.GenerateCommand` is an empty string
- `Config.GeneratedPaths` is nil

### Generate command override
Parse a `godark.yaml` with `generate_command: "make generate"`.
- `Config.GenerateCommand` is `"make generate"`

### Generated paths with directories
Parse a `godark.yaml` with `generated_paths: ["service/api/grpc/gen/", "service/test/mocks/"]`.
- `Config.GeneratedPaths` has two entries
- First entry is `"service/api/grpc/gen/"`
- Second entry is `"service/test/mocks/"`

### Generated paths with globs
Parse a `godark.yaml` with `generated_paths: ["**/*.freezed.dart", "**/*.g.dart"]`.
- `Config.GeneratedPaths` has two entries
- First entry is `"**/*.freezed.dart"`
- Second entry is `"**/*.g.dart"`

### Both fields set together
Parse a `godark.yaml` with both `generate_command` and `generated_paths` set.
- Both fields are populated correctly
- Existing config fields (repo, max_retries, etc.) are unaffected
