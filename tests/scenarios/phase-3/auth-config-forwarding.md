# Scenario: Auth and config forwarding

Relates to: Issue #23

## Setup
- The sandbox package (`internal/sandbox`) is imported directly
- The `CommandRunner` variable is stubbed to simulate `gh auth token` responses
- Host environment variables are set/unset in tests to control auth token presence
- No real API calls, Docker containers, or external services required

## Cases

### API key is collected
Set `ANTHROPIC_API_KEY` in the test environment and call `CollectAuthEnv`.
- Returned map contains `ANTHROPIC_API_KEY` with the expected value

### OAuth token is collected
Set `CLAUDE_CODE_OAUTH_TOKEN` in the test environment and call `CollectAuthEnv`.
- Returned map contains `CLAUDE_CODE_OAUTH_TOKEN` with the expected value

### Both auth tokens collected
Set both `ANTHROPIC_API_KEY` and `CLAUDE_CODE_OAUTH_TOKEN` and call `CollectAuthEnv`.
- Returned map contains both tokens

### No auth tokens returns error
Unset both `ANTHROPIC_API_KEY` and `CLAUDE_CODE_OAUTH_TOKEN` and call `CollectAuthEnv`.
- An error is returned
- Error message mentions missing authentication tokens

### GH_TOKEN from environment
Set `GH_TOKEN` in the test environment and call `CollectAuthEnv`.
- Returned map contains `GH_TOKEN` with the expected value

### GH_TOKEN falls back to gh auth token
Unset `GH_TOKEN` and stub `gh auth token` to return a token. Call `CollectAuthEnv`.
- Returned map contains `GH_TOKEN` with the value from the stubbed command

### GH_TOKEN missing entirely returns error
Unset `GH_TOKEN` and stub `gh auth token` to fail. Call `CollectAuthEnv`.
- An error is returned
- Error message mentions GitHub token

### Claude config has onboarding fields
Call `GenerateClaudeConfig("/workspace")`.
- Output is valid JSON
- JSON contains `hasCompletedOnboarding` set to `true`
- JSON contains project trust entry for `/workspace`

### Auth tokens are masked in logs
Collect auth env with tokens set and verify structured log output.
- Log entries do not contain raw token values
