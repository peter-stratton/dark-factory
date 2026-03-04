# Scenario: Migration cleanup

Relates to: Issue #41

## Setup
- The sandbox package (`internal/sandbox`) and config package (`internal/config`) are imported directly
- The `CommandRunner` variable is stubbed for `gh auth token` fallback tests
- Environment variables are controlled per test to simulate auth scenarios
- No real Docker daemon, Claude API, or GitHub access required

## Cases

### GenerateClaudeConfig is removed
Attempt to reference `sandbox.GenerateClaudeConfig` in a test file.
- The function does not exist (compilation fails if referenced)
- The `claudeConfig` and `projectConfig` types are also removed

### ClaudeFlags removed from Config struct
Parse a `Config` struct from YAML.
- The `Config` struct has no `ClaudeFlags` field
- YAML with a `claude_flags` key does not cause a parse error (unknown fields are ignored by `yaml.v3`)

### ClaudeFlags removed from RunOpts
Inspect the `RunOpts` struct in `internal/agent/launcher.go`.
- The struct has no `ClaudeFlags` field
- No code references `ClaudeFlags` anywhere in the codebase

### CollectAuthEnv requires ANTHROPIC_API_KEY
Call `CollectAuthEnv` with neither `ANTHROPIC_API_KEY` nor `CLAUDE_CODE_OAUTH_TOKEN` set.
- An error is returned
- The error message mentions `ANTHROPIC_API_KEY`

### CollectAuthEnv does not accept CLAUDE_CODE_OAUTH_TOKEN
Call `CollectAuthEnv` with only `CLAUDE_CODE_OAUTH_TOKEN` set (no `ANTHROPIC_API_KEY`).
- An error is returned (OAuth token is no longer a valid auth method)

### CollectAuthEnv succeeds with ANTHROPIC_API_KEY
Call `CollectAuthEnv` with `ANTHROPIC_API_KEY` set and `GH_TOKEN` set.
- Returns a map containing `ANTHROPIC_API_KEY` and `GH_TOKEN`
- No error is returned
- The map does not contain `CLAUDE_CODE_OAUTH_TOKEN`

### GH_TOKEN forwarding still works from environment
Call `CollectAuthEnv` with `ANTHROPIC_API_KEY` and `GH_TOKEN` set in the environment.
- The returned map contains `GH_TOKEN` with the correct value

### GH_TOKEN fallback to gh auth token still works
Call `CollectAuthEnv` with `ANTHROPIC_API_KEY` set but `GH_TOKEN` not set, and a stubbed `CommandRunner` that returns a token for `gh auth token`.
- The returned map contains `GH_TOKEN` with the value from the stubbed command

### GH_TOKEN missing with no fallback
Call `CollectAuthEnv` with `ANTHROPIC_API_KEY` set but `GH_TOKEN` not set, and a stubbed `CommandRunner` that fails for `gh auth token`.
- An error is returned mentioning `GH_TOKEN`

### No dead code remains
Run `go vet ./...` and `go build ./cmd/godark` after the cleanup.
- No compilation errors
- No unused imports or variables
- `go test ./...` passes

### maskToken removed if unused
Search the codebase for references to `maskToken`.
- If `maskToken` is no longer called by any code, it has been removed
- No unused function warnings from `go vet`
