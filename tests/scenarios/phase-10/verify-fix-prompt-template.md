# Scenario: Verify fix prompt template

Relates to: Issue #178

## Setup
- The `prompts/` directory with embedded prompt templates
- The `internal/agent/` package prompt loading and rendering logic
- No external services required

## Cases

### Template renders with verify errors
Call `RenderPrompt` on the verify fix template with `VerifyErrors` set to a
multi-line string describing build and lint failures.
- Output contains the verify error text
- Output contains the repo name, issue number, and PR number

### Empty verify errors renders cleanly
Call `RenderPrompt` on the verify fix template with `VerifyErrors` set to
an empty string.
- Template renders without error
- Output does not contain placeholder or error artifacts

### Load embedded default
Call `LoadPrompts` with no `verify_fix` path configured.
- `Prompts.VerifyFix` contains the embedded `verify_fix.txt` content
- Content is non-empty

### Load from custom config path
Write a custom prompt file and set `prompts.verify_fix` in config to its path.
- `Prompts.VerifyFix` contains the custom file content

### Template includes protected paths
Call `RenderPrompt` on the verify fix template with `ProtectedPaths` set.
- Output contains the protected paths reminder

### Template does not instruct agent to re-run checks
Render the template with failing check data.
- Output does not contain instructions to run build, lint, or test commands
- The prompt tells the agent to fix and push, not to verify
