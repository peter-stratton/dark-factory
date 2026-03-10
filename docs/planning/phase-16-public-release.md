# Phase 16: Public Release

> **Goal:** The repo is public and source-available under ELv2, installable via
> `brew` or `go install`, with automated releases on tag push. v0.1.0 is
> published.

## Milestone

`Phase 16`

---

## Issue 288: Add ELv2 license file

### Description

Add a `LICENSE` file at the repo root containing the full Elastic License 2.0
text with the project-specific fields filled in:

- **Licensed Work**: Dark Factory
- **Change Date**: 2029-03-09
- **Licensor**: Peter Stratton

The ELv2 text is standardized — use the canonical version from
https://www.elastic.co/licensing/elastic-license with only the three header
fields customized.

### Key constraints

- New file: `LICENSE` at repo root
- Use the exact canonical ELv2 text — do not paraphrase or modify the license
  body
- The three header fields (Licensed Work, Change Date, Licensor) must be filled
  in, not left as placeholders

### Acceptance criteria

- [ ] `LICENSE` file exists at repo root
- [ ] License body is the canonical Elastic License 2.0 text
- [ ] Licensed Work is `Dark Factory`
- [ ] Change Date is `2029-03-09`
- [ ] Licensor is `Peter Stratton`

### Test cases

- **File exists**: `LICENSE` is present at the repo root
- **Header fields**: File contains `Dark Factory`, `2029-03-09`, and
  `Peter Stratton` in the header section
- **License text**: File contains the string `Elastic License 2.0` and the
  standard ELv2 grant and limitation clauses

---

## Issue 289: Wire Version into ldflags

### Description

Update the Makefile `build` target to pass `Version` via ldflags using
`git describe --tags --always --dirty`. This gives local builds a meaningful
version string:

- No tags yet: `abcdef1` (short commit hash)
- On a tag: `v0.1.0`
- After a tag: `v0.1.0-3-gabcdef1` (3 commits past tag)
- Uncommitted changes: appends `-dirty`

The `version.go` file already has `Version = "dev"` as the default and
`BuildTime` is already wired. This issue adds `Version` to the existing
ldflags.

### Key constraints

- Modify `Makefile`:
  - Add `-X github.com/phs/dark-factory/internal/cmd.Version=$$(git describe --tags --always --dirty)`
    to the existing `-ldflags` string in the `build` target
- No changes to `internal/cmd/version.go` — the `Version` variable and
  `BuildTime` variable are already defined with correct defaults

### Acceptance criteria

- [ ] `make build` produces a binary where `godark version` prints a git-derived
  version string instead of `dev`
- [ ] `BuildTime` ldflags injection is preserved (not broken by the change)
- [ ] `Version` defaults to `dev` when built without ldflags (e.g.,
  `go run ./cmd/godark`)

### Test cases

- **Local build version**: `make build && bin/godark version` prints a commit
  hash or tag-based version, not `dev`
- **Build time preserved**: `godark version` output still includes a build
  timestamp
- **Default fallback**: `go run ./cmd/godark version` prints `godark dev`

---

## Issue 290: GoReleaser configuration

### Description

Add a `.goreleaser.yaml` that builds `godark` for macOS (arm64 + amd64) and
Linux (amd64), creates a GitHub Release with auto-generated changelog, and
updates the Homebrew tap formula.

This issue assumes the `peter-stratton/homebrew-dark-factory` repo already
exists (created manually before this issue runs).

### Key constraints

- New file: `.goreleaser.yaml` at repo root
- Builds:
  - `CGO_ENABLED=0`
  - GOOS: `darwin`, `linux`
  - GOARCH: `amd64`, `arm64` (arm64 for darwin only — linux arm64 can be added
    later if needed)
  - Binary name: `godark`
  - Main: `./cmd/godark`
  - Ldflags: `-s -w -X github.com/phs/dark-factory/internal/cmd.Version={{.Version}} -X github.com/phs/dark-factory/internal/cmd.BuildTime={{.Date}}`
- Archives: `tar.gz` format
- Changelog: sort ascending, exclude `docs:` and `test:` prefixed commits
- Brews section:
  ```yaml
  brews:
    - repository:
        owner: peter-stratton
        name: homebrew-dark-factory
        token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
      homepage: https://github.com/peter-stratton/dark-factory
      description: "Orchestrate autonomous AI agents to implement GitHub issues"
      license: "Elastic-2.0"
  ```
- Do NOT run `gh repo create` or any other side effects — the tap repo is
  created manually

### Acceptance criteria

- [ ] `.goreleaser.yaml` exists at repo root
- [ ] Builds for darwin/amd64, darwin/arm64, and linux/amd64
- [ ] Ldflags set both `Version` and `BuildTime`
- [ ] `CGO_ENABLED=0` for static binaries
- [ ] Brews section targets `peter-stratton/homebrew-dark-factory`
- [ ] Changelog excludes `docs:` and `test:` commits

### Test cases

- **Config valid**: `goreleaser check` passes (if goreleaser is available
  locally, otherwise verify YAML structure manually)
- **Three build targets**: Config specifies darwin/amd64, darwin/arm64,
  linux/amd64
- **Ldflags present**: Both `Version` and `BuildTime` appear in ldflags string
- **Tap repo**: Brews repository owner is `peter-stratton` and name is
  `homebrew-dark-factory`

---

## Issue 291: GitHub Actions release workflow

**Blocked by**: #290

### Description

Add a GitHub Actions workflow that triggers on version tag pushes (`v*`) and
runs GoReleaser to build binaries, create a GitHub Release, and update the
Homebrew tap formula.

### Key constraints

- New file: `.github/workflows/release.yml`
- Trigger: `push.tags: ['v*']`
- Permissions: `contents: write` (needed for creating releases)
- Steps:
  1. `actions/checkout@v4` with `fetch-depth: 0` (GoReleaser needs full git
     history for changelog)
  2. `actions/setup-go@v5` with `go-version-file: go.mod`
  3. `goreleaser/goreleaser-action@v6` with `args: release --clean`
- Environment variables:
  - `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` (for creating the release)
  - `HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}` (for pushing to
    the tap repo — must be a PAT with repo scope, added as a repo secret
    manually)
- Single job, runs on `ubuntu-latest`

### Acceptance criteria

- [ ] Workflow file exists at `.github/workflows/release.yml`
- [ ] Triggers only on `v*` tag pushes
- [ ] Checks out with full git history
- [ ] Uses `go-version-file: go.mod` (not a hardcoded Go version)
- [ ] Passes both `GITHUB_TOKEN` and `HOMEBREW_TAP_TOKEN` to GoReleaser

### Test cases

- **Trigger filter**: Workflow `on` block specifies `push.tags: ['v*']`
- **Full history**: Checkout step has `fetch-depth: 0`
- **Go version**: Setup-go uses `go-version-file`, not `go-version`
- **Both tokens**: GoReleaser step env block includes both token variables
- **Permissions**: Top-level `permissions.contents` is `write`

---

## Issue 292: README install instructions

**Blocked by**: #290

### Description

Add an "Install" section near the top of `README.md` (after the Philosophy
section, before Features) with instructions for installing godark via Homebrew,
`go install`, or downloading a binary from GitHub Releases.

### Key constraints

- Modify `README.md`:
  - Add an `## Install` section after the Philosophy section
  - Three methods:
    1. **Homebrew** (macOS):
       ```
       brew install peter-stratton/dark-factory/godark
       ```
    2. **Go install** (requires Go 1.21+):
       ```
       go install github.com/peter-stratton/dark-factory/cmd/godark@latest
       ```
    3. **Binary download**: link to GitHub Releases page
       (`https://github.com/peter-stratton/dark-factory/releases`)
  - Add a note that all methods require Docker and `gh` CLI as runtime
    dependencies (link to the existing Prerequisites section)
- Add a one-line license notice at the bottom of the README:
  `Licensed under the [Elastic License 2.0](LICENSE).`

### Acceptance criteria

- [ ] Install section exists with Homebrew, go install, and binary download
  methods
- [ ] Homebrew command uses the correct tap path
- [ ] `go install` command uses the correct module path
- [ ] Releases link points to the correct GitHub URL
- [ ] License notice at bottom of README links to `LICENSE` file

### Test cases

- **Section exists**: README contains `## Install` heading
- **Homebrew command**: README contains
  `brew install peter-stratton/dark-factory/godark`
- **Go install command**: README contains
  `go install github.com/peter-stratton/dark-factory/cmd/godark@latest`
- **Releases link**: README contains a link to the GitHub releases page
- **License line**: README ends with a license notice linking to `LICENSE`

---

## Issue 293: CONTRIBUTING.md

### Description

Add a `CONTRIBUTING.md` file explaining the project's development model and
how to contribute. The project uses godark itself to implement most changes
via autonomous agents, which is a unique model that contributors should
understand.

### Key constraints

- New file: `CONTRIBUTING.md` at repo root
- Sections:
  - **Opening issues** — bug reports and feature requests welcome; describe the
    problem or use case clearly; the team triages and schedules into milestones
  - **Pull requests** — PRs are welcome for bug fixes and small improvements;
    note that this project uses autonomous agents for most implementation work,
    so large feature PRs may not align with the current roadmap; recommend
    opening an issue first to discuss before investing time in a PR
  - **How development works** — brief description of the pipeline: human writes
    roadmap and specs → `godark run` implements, reviews, and merges → human
    spot-checks; link to `docs/ROADMAP.md` for the full roadmap
  - **Local development** — `go build ./cmd/godark`, `go test ./...`,
    prerequisites (Go, Docker, gh CLI)
- Tone: welcoming and clear; the automation model is a feature, not gatekeeping

### Acceptance criteria

- [ ] `CONTRIBUTING.md` exists at repo root
- [ ] Explains that issues are welcome
- [ ] Explains that PRs are welcome for bug fixes and small improvements
- [ ] Recommends opening an issue before large PRs
- [ ] Describes the godark pipeline at a high level
- [ ] Includes local development instructions
- [ ] Links to `docs/ROADMAP.md`

### Test cases

- **File exists**: `CONTRIBUTING.md` is present at the repo root
- **Issues welcome**: File contains language encouraging issue submissions
- **PRs welcome with caveat**: File contains language welcoming PRs for bug
  fixes while noting the agent-driven development model
- **Issue first**: File recommends opening an issue before large PRs
- **Pipeline description**: File mentions `godark run` or the three-agent loop
- **Build instructions**: File contains `go build` and `go test` commands
- **Roadmap link**: File contains a link to `docs/ROADMAP.md`
