# Scenario: GitHub Actions release workflow

Relates to: Issue #291

## Setup
- The repo root is the working directory
- `.github/workflows/release.yml` exists

## Cases

### Workflow file exists
Check that the workflow file is present.
- `.github/workflows/release.yml` exists

### Trigger on version tags
Read the `on` block of the workflow.
- Triggers on `push.tags` matching `v*`

### Full git history checkout
Read the checkout step.
- `fetch-depth` is `0`

### Go version from go.mod
Read the setup-go step.
- Uses `go-version-file: go.mod`, not a hardcoded `go-version`

### Both tokens passed
Read the GoReleaser step environment variables.
- `GITHUB_TOKEN` is set from `secrets.GITHUB_TOKEN`
- `HOMEBREW_TAP_TOKEN` is set from `secrets.HOMEBREW_TAP_TOKEN`

### Write permissions
Read the top-level permissions block.
- `contents` is set to `write`
