# Scenario: Repo cloning inside container

Relates to: Issue #24

## Setup
- The sandbox package (`internal/sandbox`) is imported directly
- Functions are pure string generators (`CloneScript`, `EntrypointScript`) — no real git or Docker operations
- Script output is inspected as strings for expected commands
- No external services, network access, or Docker daemon required

## Cases

### Clone script clones via HTTPS
Call `CloneScript("owner/repo", "", "/workspace")`.
- Output contains `git clone https://github.com/owner/repo.git /workspace`

### Clone script checks out branch when specified
Call `CloneScript("owner/repo", "feature-branch", "/workspace")`.
- Output contains `git checkout feature-branch`

### Clone script skips checkout when no branch
Call `CloneScript("owner/repo", "", "/workspace")`.
- Output does not contain `git checkout`

### Clone script configures git identity
Call `CloneScript("owner/repo", "", "/workspace")`.
- Output contains `git config` with a user name
- Output contains `git config` with a user email

### Clone script sets up GitHub auth
Call `CloneScript("owner/repo", "", "/workspace")`.
- Output contains `gh auth setup-git`

### Token is not embedded in clone URL
Call `CloneScript("owner/repo", "", "/workspace")`.
- The `git clone` URL does not contain `$GH_TOKEN` or any token value
- Auth is handled via `gh auth setup-git`, not URL embedding

### Entrypoint runs clone then agent command
Call `EntrypointScript(cloneScript, "claude -p --dangerously-skip-permissions '...'")`.
- Output contains the clone script content before the agent command
- Output contains the agent command after the clone script

### Entrypoint exits on clone failure
Inspect the output of `EntrypointScript`.
- Script uses `set -e` or an explicit error check after the clone step
- A clone failure prevents the agent command from running
