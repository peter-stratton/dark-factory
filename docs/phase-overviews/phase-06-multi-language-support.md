# Phase 6: Multi-Language Support

Dark Factory started as a Go-only tool. Phase 6 made the entire pipeline --
auto-detection, Dockerfile generation, config schema, and reviewer prompts --
language-agnostic, so that `godark run` works against Go, Flutter/Dart, Node,
Rust, Elixir, and Python projects without manual setup.


## Auto-Detection of Project Type

The `internal/detect/` package scans the target repo for language marker files
and infers the runtime, build command, and test command. Detection follows a
priority order: Go (`go.mod`), Flutter (`pubspec.yaml`), Node (`package.json`),
Rust (`Cargo.toml`), Elixir (`mix.exs`), Python (`pyproject.toml` or
`requirements.txt`). First match wins.

Detection runs automatically at the start of every `godark run` or
`godark implement` invocation. If the config already specifies a runtime,
detection is skipped entirely -- explicit config always wins.

**Example: zero-config Flutter project.**
You clone a Flutter repo that has a `pubspec.yaml` and run godark against it
with a minimal config:

```yaml
# godark.yaml
repo: myorg/flutter-app
```

godark detects the runtime from `pubspec.yaml`, reads the SDK constraint from
the `environment.sdk` field, and fills in the defaults:

```
INFO detected project type  runtime=flutter version=">=3.0.0 <4.0.0"
```

The agent's sandbox gets Flutter installed, and the test command defaults to
`flutter test`. No `runtime:` block needed.

**Example: overriding detected defaults.**
A Node project uses Vitest instead of the default `npm test`. You set only the
test command and let detection handle the rest:

```yaml
# godark.yaml
repo: myorg/node-api
test_command: "npx vitest run"
```

Detection fills in `runtime: {name: node}` and `build_command: "npm run build"`,
but leaves `test_command` alone because it was explicitly set.


## Generic Runtime Config

The old config had `go_version` and `cross_compile` fields baked into the
schema. Phase 6 replaced these with a generic `runtime:` block that works for
any language.

**Example: configuring an Elixir project.**

```yaml
# godark.yaml
repo: myorg/phoenix-app
runtime:
  name: elixir
  version: "~> 1.14"
build_command: "mix compile"
test_command: "mix test"
```

The `name` field selects the toolchain installed in the sandbox container.
The `version` field is optional -- when omitted, detection tries to extract it
from the project's own config files (e.g., the `elixir:` key in `mix.exs`,
the `go` directive in `go.mod`, or the `engines.node` field in `package.json`).


## Pluggable Dockerfile Generation

`internal/sandbox/dockerfile.go` uses a single Go template with conditional
blocks selected by `runtime.name`. Each runtime gets its own install stanza:
Go downloads a tarball, Flutter clones the SDK repo, Rust runs `rustup`, Node
uses the NodeSource APT repo, Python adds `python3-venv`, and Elixir installs
Erlang/OTP plus Elixir from the Erlang Solutions repository.

The generated Dockerfile is deterministic -- same config produces the same
output, and the image tag is derived from a SHA-256 hash of the Dockerfile
content. This means Docker's layer cache is reused across runs unless the
config actually changes.

**Example: what the generated Dockerfile looks like for a Rust project.**

With `runtime: {name: rust}` in the config, the generated Dockerfile includes:

```dockerfile
# Install Rust
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"
```

For a Go project with `runtime: {name: go, version: "1.26.0"}`:

```dockerfile
# Install Go
RUN curl -fsSL https://go.dev/dl/go1.26.0.linux-amd64.tar.gz \
      | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"
```

**Example: passing environment variables to the sandbox.**

The old `cross_compile` struct is gone, replaced by a generic `sandbox_env` map.
Any key-value pairs you set are rendered as `ENV` directives in the Dockerfile:

```yaml
# godark.yaml
sandbox_env:
  GOOS: linux
  GOARCH: arm64
```

This produces:

```dockerfile
ENV GOARCH=arm64
ENV GOOS=linux
```

Keys are sorted alphabetically for deterministic output.


## Language-Aware Reviewer Prompts

Before Phase 6, the reviewer prompt hardcoded `go test ./tests/review/ -v` as
the command for running integration tests. Now it uses the `{{.TestCommand}}`
template variable, which resolves to whatever the detected or configured test
command is.

The reviewer prompt also references the detected language so the agent
generates tests in the right framework -- Go table tests for Go projects,
Dart test files for Flutter, Jest suites for Node, and so on.

**Example: what the reviewer sees for a Flutter project.**

The rendered reviewer prompt includes:

```
7. Run unit tests: flutter test
...
10. MANDATORY: Run your integration tests (use the appropriate test runner
    for the project language, or fall back to flutter test).
```

The reviewer agent generates Dart test files in `tests/review/` and runs them
with `flutter test`, rather than trying to invoke `go test` against a
Flutter codebase.


## Supported Runtimes at a Glance

| Runtime  | Marker file        | Default build command | Default test command | Version source                     |
|----------|--------------------|-----------------------|----------------------|------------------------------------|
| Go       | `go.mod`           | `go build ./...`      | `go test ./...`      | `go` directive in `go.mod`         |
| Flutter  | `pubspec.yaml`     | (none)                | `flutter test`       | `environment.sdk` in `pubspec.yaml`|
| Node     | `package.json`     | `npm run build`       | `npm test`           | `engines.node` in `package.json`   |
| Rust     | `Cargo.toml`       | `cargo build`         | `cargo test`         | (none)                             |
| Elixir   | `mix.exs`          | `mix compile`         | `mix test`           | `elixir:` key in `mix.exs`         |
| Python   | `pyproject.toml` / `requirements.txt` | (none) | `pytest`   | (none)                             |

All runtimes follow the same rule: if you set `runtime:`, `build_command`, or
`test_command` explicitly in `godark.yaml`, your values take precedence over
anything auto-detected.
