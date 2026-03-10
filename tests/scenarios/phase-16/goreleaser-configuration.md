# Scenario: GoReleaser configuration

Relates to: Issue #290

## Setup
- The repo root is the working directory
- `.goreleaser.yaml` exists at the repo root

## Cases

### Config file exists
Check that `.goreleaser.yaml` is present at the repo root.
- `.goreleaser.yaml` file exists

### Build targets
Read the builds section of `.goreleaser.yaml`.
- Targets include darwin/amd64, darwin/arm64, and linux/amd64
- `CGO_ENABLED=0` is set in the env

### Ldflags
Read the builds section ldflags.
- Ldflags contain `cmd.Version`
- Ldflags contain `cmd.BuildTime`

### Homebrew tap
Read the brews section of `.goreleaser.yaml`.
- Repository owner is `peter-stratton`
- Repository name is `homebrew-dark-factory`

### Changelog filtering
Read the changelog section.
- Commits prefixed with `docs:` are excluded
- Commits prefixed with `test:` are excluded
