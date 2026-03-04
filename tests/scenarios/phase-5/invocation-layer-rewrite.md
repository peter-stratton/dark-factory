# Scenario: Invocation layer rewrite

Relates to: Issue #36

## Setup
- The agent package (`internal/agent`) and sandbox package (`internal/sandbox`) are imported directly
- The `Runner` variable in `launcher.go` is stubbed to capture command invocations (no real `python3` or `claude` calls)
- The `CommandRunner` variable in `sandbox/` is stubbed to capture `docker build` invocations
- `agent_runner.py` is embedded via `//go:embed` in `internal/agent/runner/`
- No real Docker daemon, Python interpreter, or Claude API calls required

## Cases

### Embedded runner file is accessible
Read the embedded `agent_runner.py` via the `runner.FS` embed filesystem.
- The file exists and contains valid Python source
- The file imports `claude_agent_sdk`
- The file reads `GODARK_PROMPT` from the environment

### Host mode invokes python3
Call `Run()` with `noSandbox=true` and a stubbed `Runner`.
- The stubbed runner receives `python3` as the command (not `claude`)
- The second argument is a path to a temp file containing `agent_runner.py`
- The temp file is cleaned up after the run completes

### Host mode passes environment variables
Call `Run()` with `noSandbox=true` and `RunOpts.Env` containing auth tokens.
- `GODARK_PROMPT` is set in the environment with the rendered prompt
- `GODARK_ROLE` is set in the environment
- Auth tokens from `RunOpts.Env` are forwarded to the subprocess

### Sandbox entrypoint calls python runner
Call `Run()` with `noSandbox=false` (sandbox mode).
- The entrypoint script passed to `RunContainer` contains `python3 /usr/local/bin/agent_runner.py`
- The entrypoint script does not contain `claude -p`
- `GODARK_PROMPT` is set in the container environment

### Dockerfile includes Python and SDK
Call `GenerateDockerfile` with default config.
- Output contains `python3` in an `apt-get install` line
- Output contains `pip install claude-agent-sdk`
- Output still contains `npm install -g @anthropic-ai/claude-code` (SDK depends on CLI)
- Output contains `COPY agent_runner.py /usr/local/bin/agent_runner.py`

### BuildImage writes runner to build context
Call `BuildImage` with a stubbed `CommandRunner`.
- The embedded `agent_runner.py` is written to the temp build directory
- The file exists at the expected path before `docker build` is invoked

### Structured result parsing from runner output
Parse runner stdout that contains a final JSON line.
- Given stdout ending with `{"session_id": "sess-123", "result": "Implementation complete", "cost_usd": 0.42, "is_error": false}`:
  - `Result.SessionID` is `"sess-123"`
  - `Result.CostUSD` is `0.42`
  - `Result.Stdout` contains the full output

### Structured result with error
Parse runner stdout where `is_error` is true.
- `Result.ExitCode` reflects the error state
- `Result.SessionID` is still captured (for potential debugging)

### Prompt passed via environment variable
Call `Run()` with a prompt containing special characters (quotes, newlines, dollar signs).
- `GODARK_PROMPT` environment variable contains the exact prompt text
- The prompt is not passed as a CLI argument (no shell quoting issues)

### ClaudeFlags no longer used
Construct `RunOpts` without `ClaudeFlags` field.
- The `RunOpts` struct does not have a `ClaudeFlags` field
- No `--dangerously-skip-permissions` flag appears in any subprocess invocation
