## Phase 16: Public Release ✅

**Goal**: The repo is public and source-available under ELv2, installable via
`brew` or `go install`, with automated releases on tag push. v0.1.0 is
published.

**Milestone**: `Phase 16` | **Label**: `phase-16`

- ELv2 license — add `LICENSE` file with Elastic License 2.0 text
- `godark version` command — version embedding via ldflags at build time
- GoReleaser setup — `.goreleaser.yaml` for macOS (arm64 + amd64) and Linux
  (amd64) binaries, GitHub Release with changelog generation
- GitHub Actions release workflow — build and publish on version tag push
  (`v*`), triggers GoReleaser
- Homebrew tap — `peter-stratton/homebrew-dark-factory` repo with formula,
  GoReleaser auto-updates formula on release
- README polish — what it does, install instructions (brew, go install,
  clone), prerequisites (Docker, GitHub token, Anthropic API key), link to
  roadmap
- Hardcoded value scrub — audit Makefile and codebase for machine-specific
  paths or assumptions, make install target portable
- Platform smoke test — verify Docker sandbox works on Linux, document Mac +
  Linux as supported platforms
- CONTRIBUTING.md — explains project is built by its own automation, issues
  welcome, PRs by invitation, how the godark pipeline works
- Repo visibility flip — make the repo public, verify nothing sensitive in
  git history

**Issues**: #285–#293

**Planning doc**: `docs/planning/phase-16-public-release.md`

