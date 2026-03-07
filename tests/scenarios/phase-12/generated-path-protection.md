# Scenario: Generated path protection in PreToolUse hook

Relates to: Issue #220

## Setup
- The `internal/agent/runner/agent_runner.py` PreToolUse hook logic
- The `internal/agent/implementer.go` env variable wiring
- Mock or unit-level testing of the generated path hook function
- Prompt templates with `{{.GeneratedPaths}}` variable

## Cases

### Directory prefix blocked
The hook receives a Write tool call targeting `service/api/grpc/gen/foo.go`.
Config has `generated_paths: ["service/api/grpc/gen/"]`.
- Hook returns `decision: block`
- Hook returns a `systemMessage` containing "generated"

### Glob pattern blocked
The hook receives an Edit tool call targeting `lib/models/load.freezed.dart`.
Config has `generated_paths: ["**/*.freezed.dart"]`.
- Hook returns `decision: block`
- Hook returns a `systemMessage` containing "generated"

### Non-matching file allowed
The hook receives a Write tool call targeting `service/api/graph/resolver.go`.
Config has `generated_paths: ["service/api/grpc/gen/", "**/*.freezed.dart"]`.
- Hook does not block the write

### Empty generated paths disables check
No `GODARK_GENERATED_PATHS` env var is set.
The hook receives a Write tool call targeting any file.
- Hook does not block the write

### Environment variable wiring
Call `newRunOpts` with a config containing `GeneratedPaths: ["service/api/grpc/gen/", "**/*.g.dart"]`.
- The resulting `RunOpts.Env` contains `GODARK_GENERATED_PATHS` with value `service/api/grpc/gen/,**/*.g.dart`

### Prompt template includes generated paths
Render the implementer prompt with `GeneratedPaths` set to a non-empty string.
- The rendered prompt contains the generated paths listing

### Protected paths and generated paths coexist
Config has both `protected_paths: ["CLAUDE.md"]` and `generated_paths: ["gen/"]`.
- Write to `CLAUDE.md` is blocked with "protected" message
- Write to `gen/foo.go` is blocked with "generated" message
- Write to `src/main.go` is allowed
