## Phase 12: Complex Project Support ✅

**Goal**: `godark` handles production repos with multiple modules, code
generation pipelines, compose-based test infrastructure, and build secrets.
Without this, godark is limited to simple single-module projects — blocking
adoption at orgs with real-world service complexity.

**Milestone**: `Phase 12` | **Label**: `phase-12`

### Multi-module monorepo support
- `modules:` section in `godark.yaml` maps subdirectories to their own
  build/test/lint commands:
  ```yaml
  modules:
    service:
      build_command: "go build ./..."
      test_command: "go test ./..."
      generate_command: "go generate ./..."
    admin-cli:
      build_command: "go build ./..."
      test_command: "go test ./..."
      depends_on: [service]
  ```
- If `modules:` is absent, current behavior is preserved (single root module)
- Change detection: when an issue touches files in `service/`, only
  `service` and its dependents (`admin-cli`) need build/test — skip unrelated
  modules
- The verify pipeline (Phase 10) runs per-module in dependency order
- Prompt templates receive module context so agents know which subdirectory
  they're working in

### Code generation pipeline
- `generate_command` field (string or list) runs before `build_command` in
  the verify pipeline — supports protoc, sqlc, gqlgen, mockery, etc.
- Per-module or root-level (if no `modules:` block)
- Generated file protection: `generated_paths` list marks directories or
  glob patterns that agents must not hand-edit (e.g., `service/api/grpc/gen/`,
  `service/repository/crdb/generated/`, `**/*.freezed.dart`)
  ```yaml
  generated_paths:
    - service/api/grpc/gen/
    - service/api/graph/generated/
    - service/repository/crdb/generated/
    - service/test/mocks/
    - "**/*.freezed.dart"    # glob patterns for co-located generated files
    - "**/*.g.dart"
  ```
- Enforced via the existing `PreToolUse` hook — same mechanism as
  `protected_paths` but with a distinct error message ("this file is
  generated — edit the source file instead")
- Prompt templates include a generated-files summary so agents know which
  source files drive which generated outputs

### Build secrets and environment
- `required_env` list in `godark.yaml` — fail fast at startup if any are
  missing:
  ```yaml
  required_env:
    - CLOUDSMITH_TOKEN
    - PUBSUB_EMULATOR_HOST
  ```
- Forwarded to sandbox automatically (extends current `CollectAuthEnv`)
- Secrets are never logged or written to run data

### CI status check awareness
- `wait_for_checks` config option — after PR creation, poll GitHub status
  checks before proceeding to review:
  ```yaml
  wait_for_checks:
    timeout: 10m
    required: [golangci-lint, apollo-check]
  ```
- If required checks fail, feed the failure output to the implementer for
  a fix cycle (same pattern as verify failures in Phase 10)
- If not configured, current behavior is preserved (proceed immediately)

### `/godark-configure-project` skill
- Embedded skill (installed by `godark init`) that analyzes an existing project
  and populates `godark.yaml` with the complex config fields added in this phase
- Detects: multiple `go.mod`/`pubspec.yaml`/`package.json` files → suggests
  `modules:` with per-module build/test commands and `depends_on` relationships
- Detects: `docker-compose*.yml` files → suggests `test_infra:` block with
  setup/teardown commands
- Detects: codegen config files (`sqlc.yml`, `gqlgen.yml`, `.mockery.yaml`,
  `build.yaml`, `protoc` invocations in Makefile/scripts) → suggests
  `generate_command` and `generated_paths`
- Detects: `.env.example`, `required_env` in CI configs, secret references →
  suggests `required_env` list
- Detects: CI workflow files → suggests `wait_for_checks` with required check
  names
- Interactive: presents findings and lets the user confirm/edit before writing
- Merges into existing `godark.yaml` (does not overwrite fields already set)
- Same pattern as `/godark-define-architecture` — analyze, recommend, confirm

### Config summary
```yaml
modules:
  service:
    build_command: "go build ./..."
    test_command: "go test ./..."
    generate_command: "make generate"
    depends_on: []
  admin-cli:
    build_command: "go build ./..."
    test_command: "go test ./..."
    depends_on: [service]
generated_paths:
  - service/api/grpc/gen/
  - service/api/graph/generated/
  - "**/*.freezed.dart"
  - "**/*.g.dart"
required_env:
  - CLOUDSMITH_TOKEN
wait_for_checks:
  timeout: 10m
  required: [golangci-lint]
```

**Issues**: #216–#226

**Planning doc**: `docs/planning/phase-12-complex-project-support.md`

