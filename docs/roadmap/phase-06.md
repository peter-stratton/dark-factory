## Phase 6: Multi-Language Support ✅

**Goal**: `godark` can orchestrate agents against non-Go projects. The
Dockerfile, config, and prompts are language-agnostic; project type is
auto-detected or configured explicitly.

**Milestone**: `Phase 6` | **Label**: `phase-6`

### Auto-detect project type
- Scan target repo for language markers: `go.mod` (Go), `pubspec.yaml`
  (Flutter/Dart), `package.json` (Node), `Cargo.toml` (Rust),
  `requirements.txt`/`pyproject.toml` (Python)
- Set sensible default `build_command` and `test_command` per detected type
- Allow explicit override in `godark.yaml` (config always wins over detection)
- Subsumes issue #33 (auto-detect Go version) into broader auto-detect

### Generic runtime config
- Replace `GoVersion` and `CrossCompile{GOOS, GOARCH}` with a `runtime:` block:
  ```yaml
  runtime:
    name: flutter    # go | flutter | node | rust | python
    version: "3.41"  # optional — auto-detected if omitted
  ```
- Preserve backwards compatibility: if `go_version` is set in existing configs,
  treat it as `runtime: {name: go, version: "<value>"}`
- Move `CrossCompile` env vars into a generic `sandbox_env:` map

### Pluggable Dockerfile generation
- Refactor `dockerfile.go` to select a toolchain install stanza based on
  `runtime.name` instead of hardcoding the Go tarball download
- Supported runtimes at launch: Go, Flutter/Dart, Node
- Each runtime provides: base packages, SDK install commands, PATH setup
- Runtime stanzas are Go templates, not external files (keep it simple)

### Language-aware reviewer prompt
- Replace hardcoded `go test ./{{.ReviewDir}} -v` in `reviewer.txt` with
  `{{.TestCommand}}` (already used by the implementer prompt)
- Make review test generation language-aware: reviewer prompt should reference
  the detected language so it generates tests in the right framework
  (Go tests, Dart tests, Jest, etc.)

### Cleanup
- Remove `GoVersion` field from `DockerConfig` and `Config` structs
- Remove `CrossCompile` struct; replace with `SandboxEnv map[string]string`
- Update `godark doctor` checklist references (Phase 8) to be runtime-aware
- Update docs (`CONTEXT.md`, `README.md`) to reflect multi-language support

**Issues**: #50–#54

