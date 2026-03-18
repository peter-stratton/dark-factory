# Phase 16: Public Release

Dark Factory has been building itself since Phase 1, but until Phase 16 it lived in a private repo with manual builds. This phase makes the project installable by anyone: an Elastic License 2.0 file, a `godark version` command with git-derived version strings, GoReleaser producing cross-platform binaries, a GitHub Actions workflow that publishes releases on tag push, a Homebrew tap for one-line installs, and the documentation a public project needs -- install instructions, contributor guide, issue templates, and branch protection.

---

## ELv2 License

**What it does:** Adds a `LICENSE` file at the repo root containing the full Elastic License 2.0 text. ELv2 is source-available: anyone can use, modify, and distribute the software, but cannot offer it as a hosted service.

**Example:** The license header identifies the project and sets a three-year change date after which the code becomes Apache 2.0:

```
Elastic License 2.0

Licensed Work:  Dark Factory
Change Date:    2029-03-09
Licensor:       Peter Stratton
```

The README links to it at the bottom:

```
Dark Factory is licensed under the [Elastic License 2.0](LICENSE).
```

---

## Version Command

**What it does:** `godark version` prints the binary's version and build timestamp. The version is injected at build time via ldflags -- local builds get a git-derived version string, release builds get the tag.

**Example:** The Makefile's `build` target passes both values:

```makefile
go build -ldflags "-X github.com/peter-stratton/dark-factory/internal/cmd.Version=$$(git describe --tags --always --dirty) \
  -X github.com/peter-stratton/dark-factory/internal/cmd.BuildTime=$$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
  -o bin/godark ./cmd/godark
```

After building locally, three commits past the v0.1.0 tag with uncommitted changes:

```
$ godark version
godark v0.1.0-3-gabcdef1-dirty (built 2026-03-09T14:30:00Z)
```

When built without ldflags (e.g., `go run ./cmd/godark`), the defaults in `internal/cmd/version.go` kick in:

```go
var (
    Version   = "dev"
    BuildTime = "unknown"
)
```

```
$ go run ./cmd/godark version
godark dev (built unknown)
```

---

## GoReleaser Configuration

**What it does:** `.goreleaser.yaml` defines how release binaries are built, packaged, and distributed. It produces static binaries for three platform targets, generates a changelog, and updates the Homebrew tap formula.

**Example:** The build matrix in `.goreleaser.yaml`:

```yaml
builds:
  - main: ./cmd/godark
    binary: godark
    env:
      - CGO_ENABLED=0
    ldflags:
      - -s -w
      - -X github.com/peter-stratton/dark-factory/internal/cmd.Version={{.Version}}
      - -X github.com/peter-stratton/dark-factory/internal/cmd.BuildTime={{.Date}}
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: linux
        goarch: arm64
```

This produces three binaries: `darwin/amd64`, `darwin/arm64`, and `linux/amd64`. Linux arm64 is excluded for now. `CGO_ENABLED=0` ensures fully static binaries with no system library dependencies.

The changelog filters out noise:

```yaml
changelog:
  filters:
    exclude:
      - "^docs:"
      - "^test:"
```

The Homebrew cask section points at the tap repo, using a separate PAT for cross-repo push access:

```yaml
homebrew_casks:
  - name: godark
    repository:
      owner: peter-stratton
      name: homebrew-dark-factory
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
```

---

## Release Workflow

**What it does:** A GitHub Actions workflow at `.github/workflows/release.yml` triggers on version tag pushes (`v*`), runs GoReleaser, and publishes a GitHub Release with binaries and an auto-generated changelog. It also pushes an updated formula to the Homebrew tap.

**Example:** Tagging and pushing a release:

```
$ git tag v0.1.0
$ git push origin v0.1.0
```

The workflow checks out with full history (GoReleaser needs it for changelog generation), sets up Go from `go.mod` rather than a hardcoded version, and runs `goreleaser release --clean`:

```yaml
on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

Two secrets are required: `GITHUB_TOKEN` (automatic, for creating the release) and `HOMEBREW_TAP_TOKEN` (a PAT with repo scope, added manually, for pushing the updated formula to `peter-stratton/homebrew-dark-factory`).

---

## Install Instructions

**What it does:** The README's Install section gives three installation methods -- Homebrew, `go install`, and direct binary download -- so users can pick whichever fits their workflow.

**Example:** The section sits right after the Philosophy blurb, before Features:

```markdown
## Install

**Homebrew** (macOS):

    brew install peter-stratton/dark-factory/godark

**Go install**:

    go install github.com/peter-stratton/dark-factory/cmd/godark@latest

**Binary download**: grab a pre-built binary from
[GitHub Releases](https://github.com/peter-stratton/dark-factory/releases).
```

All three methods produce the same binary. Runtime dependencies (Docker, `gh` CLI, Anthropic API key) are documented in the existing Prerequisites section.

---

## CONTRIBUTING.md

**What it does:** Explains the project's unusual development model -- most implementation is done by autonomous agents -- and how external contributors can participate. Issues are welcome, PRs for bug fixes and small improvements are welcome, and large features should start with an issue to avoid colliding with the agent pipeline.

**Example:** The "How development works" section describes the pipeline in three steps:

```markdown
1. A human writes the roadmap and specs (architecture, conventions, issue
   descriptions)
2. `godark run` picks up issues and assigns three agents -- implementer,
   reviewer, and merger -- that work autonomously
3. A human spot-checks the results
```

The local development section lists prerequisites and build commands:

```
$ go build ./cmd/godark
$ go test ./...
```

The tone is welcoming -- the automation model is presented as a feature of the project, not a gate on contributions.

---

## Issue Templates

**What it does:** Adds structured GitHub issue templates for bug reports and feature requests, and disables blank issues to guide contributors toward the templates.

**Example:** The `.github/ISSUE_TEMPLATE/` directory contains three files:

- `bug_report.md` -- template with sections for description, reproduction steps, expected behavior, and environment
- `feature_request.md` -- template for describing the use case and proposed solution
- `config.yml` -- disables blank issues:

```yaml
blank_issues_enabled: false
```

When someone opens an issue on the repo, GitHub presents the two templates as options rather than a blank text box. This ensures bug reports include reproduction steps and feature requests describe the use case before proposing a solution.

---

## Branch Protection

**What it does:** Configures branch protection rules on `main` to prevent direct pushes and require CI to pass before merging.

**Example:** After Phase 16, pushing directly to `main` is blocked -- all changes flow through pull requests. The CI workflow (`.github/workflows/ci.yml`) runs on every PR, and its status check is required to pass before merging. This applies to both human PRs and agent-created PRs, ensuring the same quality bar for everyone.

---

## Repo Visibility

**What it does:** Flips the GitHub repository from private to public after verifying no sensitive data exists in the git history.

**Example:** Before the flip, the git history was audited for secrets, API keys, and machine-specific paths. The repo was then made public via GitHub settings. Combined with the ELv2 license, install instructions, CONTRIBUTING.md, and issue templates, the project is ready for external users and contributors.
