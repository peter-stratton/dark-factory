# Phase 6: Multi-Language Support

> **Goal:** `godark` can orchestrate agents against non-Go projects. The
> Dockerfile, config, and prompts are language-agnostic; project type is
> auto-detected or configured explicitly.

## Milestone

`Phase 6`

---

## Issue 50: Generic runtime config

### Description

Replace the Go-specific `GoVersion` and `CrossCompile` fields with a
language-agnostic `Runtime` struct. This is the foundational data model that
all other Phase 6 issues build on.

The new `Runtime` struct captures the toolchain name (go, flutter, node, rust,
python), an optional version, and a generic `SandboxEnv` map that replaces
the `CrossCompile{GOOS, GOARCH}` struct. `BuildCommand` and `TestCommand`
already exist on `Config` and are language-agnostic — they stay as-is.

### Key constraints

- New type in `internal/config/config.go`:
  ```go
  type Runtime struct {
      Name    string `yaml:"name"`    // go, flutter, node, rust, python
      Version string `yaml:"version"` // optional — auto-detected if empty
  }
  ```
- Replace `CrossCompile CrossCompile` on `Config` with
  `SandboxEnv map[string]string` (`yaml:"sandbox_env"`) — a generic env var
  map forwarded to the container (users can put `GOOS`, `GOARCH`, `FLUTTER_ROOT`,
  or anything else here)
- Add `Runtime Runtime` field to `Config` (`yaml:"runtime"`)
- Remove `CrossCompile` struct entirely
- Update `DockerConfig` in `internal/sandbox/config.go`:
  - Replace `GoVersion string` with `Runtime config.Runtime`
  - Keep `NodeVersion` — Node.js is always needed for Claude Code regardless
    of project language
- Update `DefaultDockerConfig()`: no default runtime (empty means
  "auto-detect or fail")
- Update `DockerConfigFromConfig()` to map the new `Runtime` field
- Delete `GoVersion` from `config.Docker` YAML struct
- Delete `go_version` from the YAML schema (breaking change, no alias)
- Update `config_test.go` and `dockerfile_test.go` for the new types

### Acceptance criteria

- [ ] `Runtime` struct exists with `Name` and `Version` fields
- [ ] `Config.Runtime` is populated from `godark.yaml` `runtime:` section
- [ ] `Config.SandboxEnv` replaces `Config.CrossCompile`
- [ ] `CrossCompile` struct is deleted
- [ ] `DockerConfig.Runtime` replaces `DockerConfig.GoVersion`
- [ ] `DockerConfigFromConfig` maps the new runtime field correctly
- [ ] `go_version` YAML key is no longer recognized (breaking change)
- [ ] `go test ./internal/config/...` passes
- [ ] `go test ./internal/sandbox/...` passes

### Test cases

- **Runtime from YAML**: Config with `runtime: {name: flutter, version: "3.41"}` populates `Config.Runtime` correctly
- **Empty runtime**: Config with no `runtime:` key leaves `Config.Runtime` zero-valued
- **SandboxEnv from YAML**: Config with `sandbox_env: {GOOS: linux, GOARCH: arm64}` populates the map
- **Empty SandboxEnv**: Config with no `sandbox_env:` key leaves the map nil
- **CrossCompile removed**: `Config` has no `CrossCompile` field (compile-time verification)
- **DockerConfig maps runtime**: `DockerConfigFromConfig` with `Runtime{Name: "go", Version: "1.26.0"}` populates `DockerConfig.Runtime`
- **Default DockerConfig**: `DefaultDockerConfig()` has zero-valued `Runtime`

---

## Issue 52: Auto-detect project type

**Blocked by**: #50

### Description

Add a detection function that scans a local repo path for language marker files
and returns a `Runtime` plus sensible default `BuildCommand` and `TestCommand`.
This runs during `godark run` / `godark implement` when `Config.Runtime` is
not explicitly set. The detected values are logged but never written to config —
explicit config always wins.

### Key constraints

- New file: `internal/detect/detect.go`
- Function: `DetectRuntime(repoPath string) (*DetectedProject, error)`
- `DetectedProject` struct:
  ```go
  type DetectedProject struct {
      Runtime      config.Runtime
      BuildCommand string
      TestCommand  string
  }
  ```
- Detection order (first match wins):
  1. `go.mod` → `Runtime{Name: "go"}`, parse `go <version>` line for version,
     `BuildCommand: "go build ./..."`, `TestCommand: "go test ./..."`
  2. `pubspec.yaml` → `Runtime{Name: "flutter"}`, parse `environment.sdk`
     for version hint, `BuildCommand: ""` (no default build for mobile),
     `TestCommand: "flutter test"`
  3. `package.json` → `Runtime{Name: "node"}`, parse `engines.node` for
     version, `BuildCommand: "npm run build"`, `TestCommand: "npm test"`
  4. `Cargo.toml` → `Runtime{Name: "rust"}`, `BuildCommand: "cargo build"`,
     `TestCommand: "cargo test"`
  5. `pyproject.toml` or `requirements.txt` → `Runtime{Name: "python"}`,
     `BuildCommand: ""`, `TestCommand: "pytest"`
- If no marker file is found, return an error: `"could not detect project
  type: no go.mod, pubspec.yaml, package.json, Cargo.toml, or pyproject.toml
  found in <path>"`
- Version parsing is best-effort — if the version can't be extracted, leave
  `Runtime.Version` empty (Dockerfile generation will need to handle this)
- The function only reads files, never modifies anything
- Wire into orchestration: in `internal/orchestrator/loop.go` (or wherever
  the config is finalized before agent launch), if `cfg.Runtime.Name == ""`
  and `cfg.BuildCommand == ""` and `cfg.TestCommand == ""`, call
  `DetectRuntime` on the target repo. Apply detected values to the config,
  log them at `slog.Info` level

### Acceptance criteria

- [ ] `DetectRuntime` correctly identifies Go projects via `go.mod`
- [ ] `DetectRuntime` correctly identifies Flutter projects via `pubspec.yaml`
- [ ] `DetectRuntime` correctly identifies Node projects via `package.json`
- [ ] `DetectRuntime` correctly identifies Rust projects via `Cargo.toml`
- [ ] `DetectRuntime` correctly identifies Python projects via `pyproject.toml`
- [ ] Version is extracted from `go.mod` when present
- [ ] Version is extracted from `pubspec.yaml` `environment.sdk` when present
- [ ] Returns error when no marker file is found
- [ ] Explicit `Config.Runtime` is never overwritten by detection
- [ ] Detected runtime is logged at Info level
- [ ] `go test ./internal/detect/...` passes

### Test cases

- **Go project**: Directory with `go.mod` containing `go 1.26` → `Runtime{Name: "go", Version: "1.26"}`, `TestCommand: "go test ./..."`
- **Flutter project**: Directory with `pubspec.yaml` → `Runtime{Name: "flutter"}`, `TestCommand: "flutter test"`
- **Node project**: Directory with `package.json` → `Runtime{Name: "node"}`, `TestCommand: "npm test"`
- **Rust project**: Directory with `Cargo.toml` → `Runtime{Name: "rust"}`, `TestCommand: "cargo test"`
- **Python project**: Directory with `pyproject.toml` → `Runtime{Name: "python"}`, `TestCommand: "pytest"`
- **Python fallback**: Directory with `requirements.txt` (no `pyproject.toml`) → `Runtime{Name: "python"}`
- **No marker files**: Empty directory → error containing "could not detect project type"
- **Multiple markers**: Directory with both `go.mod` and `package.json` → Go wins (first-match order)
- **Go version parsing**: `go.mod` with `go 1.26.0` extracts version `"1.26.0"`
- **Go version missing**: `go.mod` without `go` directive → `Runtime.Version == ""`
- **Config wins over detection**: `Config.Runtime{Name: "node"}` is not overwritten even if `go.mod` exists

---

## Issue 53: Pluggable Dockerfile generation

**Blocked by**: #50

### Description

Refactor `dockerfile.go` to select the correct toolchain installation stanza
based on `DockerConfig.Runtime.Name` instead of always installing Go. Each
supported runtime gets its own install block in the Dockerfile template.
Node.js is always installed (Claude Code requires it) regardless of project
runtime.

### Key constraints

- File: `internal/sandbox/dockerfile.go`
- The Dockerfile template becomes conditional on `Runtime.Name`:
  - `go`: install Go from `go.dev` tarball (current behavior)
  - `flutter`: install Flutter SDK via git clone to `/usr/local/flutter`,
    add to PATH, run `flutter precache`
  - `node`: no additional install needed (Node.js is already installed for
    Claude Code). Just ensure the version matches if specified.
  - `rust`: install via `rustup` (curl script)
  - `python`: install `python3`, `python3-pip`, `python3-venv` (may already
    be present for the agent runner — ensure no conflict)
  - empty/unknown: skip toolchain install entirely, log a warning. The user
    is responsible for providing the right base image or extra packages.
- When `Runtime.Version` is empty, use a sensible default for each runtime:
  - Go: error — Go version is required (can't install without it)
  - Flutter: install latest stable (git clone with `--branch stable`)
  - Node: use whatever version is installed for Claude Code
  - Rust: install latest stable via `rustup`
  - Python: use system python3
- Template data struct gains a `RuntimeName` and `RuntimeVersion` field
  (replacing `GoVersion`)
- The template uses `{{if eq .RuntimeName "go"}}...{{end}}` blocks
- `SandboxEnv` entries from config are added as `ENV` directives in the
  Dockerfile
- Update `GenerateDockerfile` signature: still takes `DockerConfig`, but
  now uses `DockerConfig.Runtime` instead of `DockerConfig.GoVersion`

### Acceptance criteria

- [ ] Go runtime: Dockerfile installs Go at the specified version
- [ ] Flutter runtime: Dockerfile installs Flutter SDK
- [ ] Node runtime: Dockerfile does not install a second Node.js
- [ ] Rust runtime: Dockerfile installs Rust via rustup
- [ ] Python runtime: Dockerfile installs python3/pip/venv
- [ ] Empty runtime: Dockerfile skips toolchain install (no error)
- [ ] Go without version: `GenerateDockerfile` returns error
- [ ] Node.js is always installed (Claude Code dependency) regardless of runtime
- [ ] `SandboxEnv` entries appear as `ENV` lines in the Dockerfile
- [ ] `DockerConfig.GoVersion` is no longer referenced (compile-time check)
- [ ] `go test ./internal/sandbox/...` passes

### Test cases

- **Go Dockerfile**: `Runtime{Name: "go", Version: "1.26.0"}` → Dockerfile contains `go1.26.0.linux-amd64.tar.gz`
- **Flutter Dockerfile**: `Runtime{Name: "flutter"}` → Dockerfile contains `git clone.*flutter` and `flutter precache`
- **Node Dockerfile**: `Runtime{Name: "node"}` → Dockerfile does NOT contain a second Node.js install
- **Rust Dockerfile**: `Runtime{Name: "rust"}` → Dockerfile contains `rustup`
- **Python Dockerfile**: `Runtime{Name: "python"}` → Dockerfile contains `python3-venv`
- **Unknown runtime**: `Runtime{Name: ""}` → Dockerfile has no toolchain section, no error
- **Go requires version**: `Runtime{Name: "go", Version: ""}` → `GenerateDockerfile` returns error
- **Flutter no version**: `Runtime{Name: "flutter", Version: ""}` → installs stable branch (no error)
- **SandboxEnv in Dockerfile**: `SandboxEnv: {"GOOS": "linux"}` → Dockerfile contains `ENV GOOS=linux`
- **Claude Code always present**: All runtimes produce Dockerfile with `npm install -g @anthropic-ai/claude-code`
- **Image tag changes**: Changing runtime changes the image tag hash

---

## Issue 51: Language-agnostic prompts

### Description

Remove Go-specific language from the prompt templates so agents infer test
conventions from the target project's CLAUDE.md and codebase rather than from
hardcoded instructions. The agent reads CLAUDE.md as its first step and adapts
to whatever language and test framework it finds.

### Key constraints

- File: `prompts/implementer.txt`
  - Change `Write unit tests (foo_test.go next to foo.go)` →
    `Write unit tests alongside the implementation`
- File: `prompts/reviewer.txt`
  - Change `Generate Go integration tests in {{.ReviewDir}}` →
    `Generate integration tests in {{.ReviewDir}}`
  - Change `Run your integration tests: go test ./{{.ReviewDir}} -v` →
    `Run your integration tests` (the agent will use the appropriate test
    runner for the project language, or fall back to `{{.TestCommand}}`)
- File: `prompts/implementer_retry.txt` — review for any Go-specific
  language; currently looks clean (uses `{{.TestCommand}}` and
  `{{.BuildCommand}}` already)
- No new template variables needed — the agent figures out the right test
  framework from the project context
- Prompt template tests in `internal/agent/prompt_test.go` must be updated
  to match the new text

### Acceptance criteria

- [ ] `implementer.txt` contains no Go-specific file naming conventions
- [ ] `reviewer.txt` contains no `go test` references
- [ ] `reviewer.txt` uses generic "generate integration tests" language
- [ ] `implementer_retry.txt` contains no Go-specific language
- [ ] All prompts still use `{{.BuildCommand}}` and `{{.TestCommand}}` for
      build/test execution
- [ ] Prompt template rendering tests pass
- [ ] `go test ./internal/agent/...` passes

### Test cases

- **Implementer prompt rendered**: Rendered implementer prompt does not contain `foo_test.go` or `foo.go`
- **Reviewer prompt rendered**: Rendered reviewer prompt does not contain `go test`
- **Reviewer still references ReviewDir**: Rendered reviewer prompt still contains the configured `ReviewDir` path
- **Retry prompt clean**: Rendered retry prompt contains `{{.TestCommand}}` expansion, no Go references
- **Template vars preserved**: `{{.BuildCommand}}`, `{{.TestCommand}}`, `{{.ReviewDir}}` still render correctly

---

## Issue 54: Integration and cleanup

**Blocked by**: #50, #51, #52, #53

### Description

Wire everything together, remove dead code, and update documentation. This
is the final issue that ensures all pieces work end-to-end: auto-detection
feeds into config, config feeds into Dockerfile generation, and prompts work
for any supported language.

### Key constraints

- Wire auto-detection into the orchestration flow:
  - In the run/implement entry point, after config loading but before agent
    launch: if `cfg.Runtime.Name` is empty, call `detect.DetectRuntime` on
    the target repo path
  - Apply detected `Runtime`, `BuildCommand` (if not configured), and
    `TestCommand` (if not configured) to the config
  - Log: `slog.Info("detected project type", "runtime", detected.Runtime.Name, "version", detected.Runtime.Version)`
  - If detection fails and no runtime is configured, log a warning and
    proceed without a toolchain install (user may have a custom base image)
- Remove dead code:
  - Delete `GoVersion` field from `config.Docker` struct (if not already
    done by the config issue)
  - Verify no references to `CrossCompile` remain anywhere
  - Remove any `GoVersion` references in `internal/sandbox/` besides
    what was already handled
- Update `README.md`:
  - Replace `go_version` config example with `runtime:` example
  - Replace `cross_compile:` example with `sandbox_env:` example
  - Add a section listing supported runtimes
- Update `docs/CONTEXT.md`:
  - Replace Go-specific agent prompt examples with the new generic versions
  - Note multi-language support in the project description
- Close issue #33 (auto-detect Go version) — subsumed by auto-detection
- All tests pass: `go test ./...`
- Build succeeds: `go build ./cmd/godark`

### Acceptance criteria

- [ ] Auto-detection runs when `Config.Runtime` is not set
- [ ] Detected runtime populates config before Dockerfile generation
- [ ] Detected build/test commands are used when not explicitly configured
- [ ] Detection failure with no configured runtime logs a warning (not an error)
- [ ] No references to `GoVersion` or `CrossCompile` remain in source code
- [ ] `README.md` documents the `runtime:` config and supported runtimes
- [ ] `docs/CONTEXT.md` agent prompt examples are language-agnostic
- [ ] `go test ./...` passes
- [ ] `go build ./cmd/godark` succeeds

### Test cases

- **End-to-end Go detection**: Config with no runtime + repo with `go.mod` → DockerConfig gets `Runtime{Name: "go", Version: ...}`
- **End-to-end Flutter detection**: Config with no runtime + repo with `pubspec.yaml` → DockerConfig gets `Runtime{Name: "flutter"}`
- **Explicit config wins**: Config with `runtime: {name: node}` + repo with `go.mod` → runtime is `node`, not `go`
- **Explicit commands win**: Config with `test_command: "make test"` → detection does not overwrite it
- **Detection failure graceful**: Config with no runtime + repo with no markers → warning logged, no error, Dockerfile skips toolchain
- **No dead code**: `grep -r GoVersion internal/` returns no results; `grep -r CrossCompile internal/` returns no results
- **README updated**: `README.md` contains `runtime:` YAML example
