# Phase 12: Complex Project Support

> **Goal:** `godark` handles production repos with multiple modules, code
> generation pipelines, build secrets, and CI-gated merges. Without this,
> godark is limited to simple single-module projects — blocking adoption at
> orgs with real-world service complexity.

## Milestone

`Phase 12`

---

## Issue: Config fields for code generation

### Description

Add `generate_command` and `generated_paths` fields to the config struct.
`generate_command` is a shell command (string) that runs code generation
(protoc, sqlc, gqlgen, mockery, `flutter pub run build_runner build`, etc.).
`generated_paths` is a list of directory prefixes or glob patterns marking
files that agents must not hand-edit.

This is config-only — no behavioral changes. The verify pipeline, hook
enforcement, and prompt integration are handled in follow-on issues.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `GenerateCommand string` field with yaml tag `generate_command`
  - Add `GeneratedPaths []string` field with yaml tag `generated_paths`
  - Defaults: `GenerateCommand: ""` (skip), `GeneratedPaths: nil`
- No validation required — empty values mean "not configured"

### Acceptance criteria

- [ ] `Config` has `GenerateCommand` field, empty by default
- [ ] `Config` has `GeneratedPaths` field, nil by default
- [ ] Setting both fields in YAML is reflected in parsed config
- [ ] Existing tests still pass (no regressions)

### Test cases

- **Config defaults**: New config has empty `GenerateCommand` and nil
  `GeneratedPaths`
- **Generate command override**: Setting `generate_command: "make generate"`
  in YAML is reflected in parsed config
- **Generated paths with directories**: Setting `generated_paths:
  ["service/api/grpc/gen/", "service/test/mocks/"]` is reflected in parsed
  config
- **Generated paths with globs**: Setting `generated_paths:
  ["**/*.freezed.dart", "**/*.g.dart"]` is reflected in parsed config

---

## Issue: Generated path protection in PreToolUse hook

**Blocked by**: Config fields for code generation

### Description

Wire `generated_paths` into the agent runner's `PreToolUse` hook so that
agents are blocked from editing generated files. The hook uses the same
mechanism as `protected_paths` but with a distinct error message: "this file
is generated — edit the source file or re-run code generation instead."

Directory prefixes use simple `strings.HasPrefix` matching (same as
`protected_paths`). Glob patterns (entries containing `*`) use
`filepath.Match` against the file path.

### Key constraints

- Modify `internal/config/config.go`:
  - No changes (fields added in prior issue)
- Modify `internal/agent/implementer.go` (or wherever `GODARK_PROTECTED_PATHS`
  is set):
  - Add `GODARK_GENERATED_PATHS` env var (comma-separated from
    `cfg.GeneratedPaths`)
- Modify `internal/agent/runner/agent_runner.py`:
  - Read `GODARK_GENERATED_PATHS` from environment
  - In the existing `PreToolUse` hook for `Write|Edit`, check the file path
    against generated paths in addition to protected paths
  - For directory entries (no `*`): `strings.HasPrefix` / startswith match
  - For glob entries (contains `*`): `fnmatch` / `filepath.Match` match
  - On match: return `{"decision": "block", "systemMessage": "this file is
    generated — edit the source file or re-run code generation instead"}`
- Add generated paths summary to implementer and reviewer prompt templates
  via a new `{{.GeneratedPaths}}` template variable

### Acceptance criteria

- [ ] `GODARK_GENERATED_PATHS` is passed to agent runner via env
- [ ] Agent runner blocks Write/Edit to files matching directory prefixes
- [ ] Agent runner blocks Write/Edit to files matching glob patterns
- [ ] Blocked writes receive the "generated file" system message
- [ ] Non-matching file writes are allowed through
- [ ] Prompt templates include generated paths summary

### Test cases

- **Directory prefix blocked**: Write to `service/api/grpc/gen/foo.go`
  matches `service/api/grpc/gen/` and is blocked
- **Glob pattern blocked**: Write to `lib/models/load.freezed.dart` matches
  `**/*.freezed.dart` and is blocked
- **Non-matching allowed**: Write to `service/api/graph/resolver.go` does not
  match any generated path and is allowed
- **Empty generated paths**: No `GODARK_GENERATED_PATHS` env var disables
  the check
- **System message**: Blocked write response contains "generated" in the
  message

---

## Issue: Generate step in verify pipeline

**Blocked by**: Config fields for code generation

### Description

Insert `generate_command` into the verify pipeline so it runs before
`build_command`. When configured, the generate step runs as the first check
in the verify sequence: generate, then build, then lint, then test. If
generation fails, subsequent checks are skipped (same stop-on-failure
behavior as existing checks).

### Key constraints

- Modify `internal/agent/loop.go` (or wherever the `[]Check` list is built):
  - When `cfg.GenerateCommand` is non-empty, prepend a `Check{Name:
    "generate", Command: cfg.GenerateCommand}` to the check list
  - Order: generate → build → lint → test
- No changes to `verify.go` — `RunVerify` already handles arbitrary checks
  in sequence

### Acceptance criteria

- [ ] Generate command runs before build in verify pipeline
- [ ] Empty `generate_command` skips the generate step
- [ ] Generate failure stops the pipeline (build/lint/test do not run)
- [ ] Generate success proceeds to build

### Test cases

- **Generate runs first**: With all four commands configured, generate runs
  before build
- **Generate skipped**: Empty `generate_command` produces check list without
  generate entry
- **Generate fails**: Generate exits non-zero, build/lint/test are skipped
- **Generate succeeds**: Generate exits 0, build runs next

---

## Issue: Config fields for modules block

### Description

Add a `Modules` field to the config struct that maps module names to their
per-module build/test/lint/generate commands and dependency relationships.
When `modules` is absent or empty, all existing single-module behavior is
preserved.

This is config-only — per-module verify execution is handled in a follow-on
issue.

### Key constraints

- Modify `internal/config/config.go`:
  - New type:
    ```go
    type Module struct {
        BuildCommand    string   `yaml:"build_command"`
        TestCommand     string   `yaml:"test_command"`
        LintCommand     string   `yaml:"lint_command"`
        GenerateCommand string   `yaml:"generate_command"`
        DependsOn       []string `yaml:"depends_on"`
    }
    ```
  - Add `Modules map[string]Module` field to `Config` with yaml tag `modules`
  - Default: `nil` (single-module mode)
- Add validation in `validate()`:
  - If `modules` is set, check that all `depends_on` references point to
    module names that exist in the map
  - Reject cycles in module dependencies (topological sort check)

### Acceptance criteria

- [ ] `Config` has `Modules` field, nil by default
- [ ] Each module can specify build, test, lint, and generate commands
- [ ] Each module can specify `depends_on` for ordering
- [ ] Validation rejects unknown module references in `depends_on`
- [ ] Validation rejects cycles in module dependencies
- [ ] Nil `modules` preserves existing single-module behavior

### Test cases

- **Config defaults**: New config has nil `Modules`
- **Single module config**: No `modules` key in YAML, config loads normally
- **Two modules**: `modules: {service: {build_command: "go build ./..."},
  admin-cli: {depends_on: [service]}}` parses correctly
- **Unknown dependency**: `depends_on: [nonexistent]` fails validation
- **Cycle detection**: `a depends_on b, b depends_on a` fails validation
- **Per-module commands**: Each module's build/test/lint/generate commands
  are independent

---

## Issue: Per-module verify pipeline

**Blocked by**: Generate step in verify pipeline, Config fields for modules block

### Description

When `modules` is configured, the verify pipeline runs per-module in
dependency order instead of using the root-level commands. Each module's
checks (generate, build, lint, test) run in the module's subdirectory.
Modules without explicit commands inherit the root-level defaults.

In v1, all modules are verified unconditionally (no change detection).

### Key constraints

- Modify `internal/agent/loop.go`:
  - When `cfg.Modules` is non-nil:
    - Topologically sort modules by `depends_on`
    - For each module in order, build a `[]Check` list from the module's
      commands (falling back to root-level commands if a module field is empty)
    - Each command runs with `cd <module-dir> &&` prepended (or the
      `CommandRunner` is scoped to the module directory)
    - If any module fails, stop (don't run dependent modules)
  - When `cfg.Modules` is nil: existing single-module behavior unchanged
- Fix cycle handling for the verify fix loop: when modules are configured,
  the fix prompt should indicate which module failed and include only that
  module's error output
- Add `{{.ModuleContext}}` template variable to prompt data so agents know
  which module directories exist and their relationships

### Acceptance criteria

- [ ] Modules verified in dependency order
- [ ] Each module's commands run in the module's subdirectory
- [ ] Module without explicit commands inherits root-level defaults
- [ ] Module failure stops dependent modules
- [ ] Nil `modules` preserves single-module behavior
- [ ] Fix prompt identifies the failing module

### Test cases

- **Dependency order**: Module `admin-cli` depends on `service` — `service`
  verifies first
- **Module commands used**: Module with `build_command: "go build ./cmd/..."`
  uses that instead of root command
- **Inherit root commands**: Module without `lint_command` inherits root
  `lint_command`
- **Module failure stops dependents**: `service` fails build — `admin-cli`
  is skipped
- **Single module unchanged**: No `modules` in config runs root-level verify
- **Fix prompt context**: Verify failure in `service` module produces fix
  prompt mentioning "service"

---

## Issue: Config fields for required_env

### Description

Add a `required_env` field to the config struct — a list of environment
variable names that must be set before a run starts. This supports projects
that need tokens, credentials, or emulator endpoints available in the
sandbox.

This is config-only — the startup validation is handled in a follow-on issue.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `RequiredEnv []string` field with yaml tag `required_env`
  - Default: `nil` (no required env vars)
- Required env vars are forwarded to the sandbox alongside existing auth env
  vars — modify `CollectAuthEnv` (or the env-building code) to include
  values for each `required_env` name from `os.Getenv`
- Secrets must never be logged or written to run data

### Acceptance criteria

- [ ] `Config` has `RequiredEnv` field, nil by default
- [ ] Setting `required_env: [CLOUDSMITH_TOKEN, PUBSUB_EMULATOR_HOST]` in
  YAML is reflected in parsed config
- [ ] Required env values are included in sandbox env alongside auth env
- [ ] Required env values are not logged or written to run data

### Test cases

- **Config defaults**: New config has nil `RequiredEnv`
- **Required env parsed**: Setting `required_env: [FOO, BAR]` produces
  `[]string{"FOO", "BAR"}`
- **Env forwarded**: When `FOO=secret` is in the environment and `required_env`
  includes `FOO`, the sandbox env map contains `FOO=secret`

---

## Issue: Required env validation at startup

**Blocked by**: Config fields for required_env

### Description

Add fail-fast validation at the start of `godark run` and `godark implement`
that checks all `required_env` variables are set in the current environment.
If any are missing, the command exits with a clear error listing the missing
variables. This prevents wasted agent runs that would fail inside the
container due to missing credentials.

### Key constraints

- New function in `internal/config/config.go` (or a new file
  `internal/config/env.go`):
  ```go
  // ValidateRequiredEnv checks that all required environment variables are
  // set. Returns an error listing any missing variables.
  func ValidateRequiredEnv(required []string) error
  ```
- Call `ValidateRequiredEnv(cfg.RequiredEnv)` early in the `run` and
  `implement` command execution (before building the Docker image)
- Error message format: `"missing required environment variables: FOO, BAR"`
- Empty or nil `required_env` skips validation (no error)

### Acceptance criteria

- [ ] Missing required env var fails with clear error message
- [ ] Multiple missing vars are listed in a single error
- [ ] All vars present passes validation
- [ ] Empty `required_env` skips validation

### Test cases

- **All present**: `required_env: [HOME]` passes (HOME is always set)
- **One missing**: `required_env: [DEFINITELY_NOT_SET_12345]` fails with
  error containing the variable name
- **Multiple missing**: Two missing vars both appear in the error message
- **Empty list**: `required_env: []` passes without checking anything
- **Nil list**: No `required_env` in config passes without checking anything

---

## Issue: Config fields for wait_for_checks

### Description

Add a `WaitForChecks` struct to the config for CI check gating before merge.
When configured, godark polls GitHub status checks on the PR after the
review cycle passes, and only merges once required checks succeed.

This is config-only — the polling and gating logic is handled in a follow-on
issue.

### Key constraints

- Modify `internal/config/config.go`:
  - New type:
    ```go
    type WaitForChecks struct {
        Timeout  string   `yaml:"timeout"`  // duration string, e.g. "10m"
        Required []string `yaml:"required"` // check names to wait for
    }
    ```
  - Add `WaitForChecks *WaitForChecks` field to `Config` with yaml tag
    `wait_for_checks` (pointer so nil = not configured)
  - Default: `nil` (merge immediately after review, current behavior)
- Add validation in `validate()`:
  - If `wait_for_checks` is set, `timeout` must parse as a `time.Duration`
  - If `wait_for_checks` is set, `required` must be non-empty

### Acceptance criteria

- [ ] `Config` has `WaitForChecks` field, nil by default
- [ ] Setting `wait_for_checks: {timeout: "10m", required: [lint, test]}`
  in YAML is reflected in parsed config
- [ ] Validation rejects invalid timeout duration
- [ ] Validation rejects empty `required` list when `wait_for_checks` is set
- [ ] Nil `wait_for_checks` preserves current merge-immediately behavior

### Test cases

- **Config defaults**: New config has nil `WaitForChecks`
- **Valid config**: `wait_for_checks: {timeout: "10m", required: [golangci-lint]}`
  parses correctly
- **Invalid timeout**: `timeout: "not-a-duration"` fails validation
- **Empty required**: `wait_for_checks: {timeout: "5m", required: []}` fails
  validation
- **Not configured**: No `wait_for_checks` in YAML, field is nil

---

## Issue: CI check gate in merge flow

**Blocked by**: Config fields for wait_for_checks

### Description

After the review cycle approves a PR, poll GitHub status checks using
`gh pr checks` and wait for all required checks to pass before merging.
If a required check fails, feed the failure output back to the implementer
for a fix cycle (same pattern as verify failures). If checks don't pass
within the timeout, fail the issue with a descriptive error.

### Key constraints

- New function (in `internal/agent/guardrails.go` or new file
  `internal/agent/checks.go`):
  ```go
  // WaitForChecks polls GitHub PR checks until all required checks pass,
  // any required check fails, or the timeout expires.
  // Returns the names of failed checks and their output, or nil if all passed.
  func WaitForChecks(ctx context.Context, repo string, prNum int,
      required []string, timeout time.Duration, logger *slog.Logger,
  ) ([]CheckFailure, error)

  type CheckFailure struct {
      Name   string
      Output string // summary from gh pr checks
  }
  ```
- Implementation uses `gh pr checks <prNum> --repo <repo> --json name,state,
  conclusion` to poll check status
- Poll interval: 30 seconds (not configurable in v1)
- Check states: `pending`/`queued` → keep waiting; `completed` with
  `conclusion: success` → pass; `completed` with other conclusion → fail
- Modify `internal/agent/loop.go`:
  - After functional review approves, if `cfg.WaitForChecks` is non-nil:
    - Parse timeout duration from config
    - Call `WaitForChecks`
    - If all pass: proceed to merge
    - If any fail: format failure output, feed back to implementer via
      verify fix prompt (reuse `VerifyFix` flow), then re-push and wait again
    - If timeout: fail the issue with "CI checks timed out" error
    - Respect `cfg.Verify.MaxFixAttempts` for CI fix retries
  - If `cfg.WaitForChecks` is nil: merge immediately (current behavior)

### Acceptance criteria

- [ ] PR checks are polled after review approval
- [ ] All required checks passing triggers merge
- [ ] Failed check feeds output back to implementer for fix cycle
- [ ] Timeout expires with pending checks fails the issue
- [ ] Nil `WaitForChecks` config preserves immediate merge behavior
- [ ] Fix cycle respects `MaxFixAttempts` limit

### Test cases

- **All checks pass**: Required checks return `success` — merge proceeds
- **Check fails**: Required check returns `failure` — fix cycle triggered
  with check output
- **Fix succeeds**: Check fails, implementer fixes, checks pass on retry —
  merge proceeds
- **Timeout**: Checks stay `pending` past timeout — issue fails with timeout
  error
- **Not configured**: No `wait_for_checks` — merge immediately after review
- **Unrequired check fails**: Non-required check fails — ignored, merge
  proceeds
- **Fix exhausted**: Check fails, fix attempts exhausted — issue fails

---

## Issue: `/godark-configure-project` skill

**Blocked by**: Config fields for code generation, Config fields for modules
block, Config fields for required_env, Config fields for wait_for_checks

### Description

New embedded skill installed by `godark init` that analyzes an existing
project and populates `godark.yaml` with the complex config fields added in
this phase. The skill scans for language markers, codegen configs, CI
workflows, and project structure, then presents findings for user
confirmation before writing.

### Key constraints

- New skill directory: `internal/skills/godark-configure-project/SKILL.md`
- Add to `internal/skills/embed.go` embed directive
- Update `internal/cmd/init.go` to install the new skill
- Skill behavior (described in SKILL.md):
  - Detects multiple `go.mod`/`pubspec.yaml`/`package.json` files → suggests
    `modules:` with per-module build/test commands and `depends_on`
  - Detects codegen config files (`sqlc.yml`, `gqlgen.yml`, `.mockery.yaml`,
    `build.yaml`, `Makefile` with generate targets, proto files) → suggests
    `generate_command` and `generated_paths`
  - Detects `.env.example`, `required_env` in CI configs, secret references
    in compose files → suggests `required_env` list
  - Detects CI workflow files (`.github/workflows/*.yml`) → suggests
    `wait_for_checks` with required check names extracted from workflow jobs
  - Detects `docker-compose*.yml` → notes that `no_sandbox: true` may be
    needed for integration tests run via CI
  - Interactive: presents findings and lets user confirm/edit before writing
  - Merges into existing `godark.yaml` (does not overwrite fields already set)
- The skill file is a Claude skill (markdown with frontmatter), not Go code
  — same pattern as `/godark-define-architecture`

### Acceptance criteria

- [ ] Skill file exists at correct path and is embedded
- [ ] `godark init` installs the skill
- [ ] Skill detects multi-module projects and suggests `modules` config
- [ ] Skill detects codegen configs and suggests `generate_command` and
  `generated_paths`
- [ ] Skill detects CI workflows and suggests `wait_for_checks`
- [ ] Skill detects env requirements and suggests `required_env`
- [ ] Skill merges into existing `godark.yaml` without overwriting

### Test cases

- **Skill installed**: `godark init` creates
  `.claude/skills/godark-configure-project/SKILL.md`
- **Embed updated**: `internal/skills/embed.go` includes the new skill
  directory
- **Skill frontmatter**: SKILL.md contains `name: godark-configure-project`
- **Init idempotent**: Running `godark init` twice does not duplicate the
  skill file

---

## Issue: Update architecture.json for new packages

### Description

If any new packages are introduced in this phase (e.g., `internal/agent/checks.go`
adding to the existing `internal/agent/` path, or a new `internal/config/env.go`
within `internal/config/`), verify that `docs/architecture.json` layer paths
already cover them. Since all new code lives within existing package
directories (`internal/config/`, `internal/agent/`, `internal/skills/`), no
architecture.json changes should be needed — but this issue exists as an
explicit verification step.

### Key constraints

- Read `docs/architecture.json` and confirm all paths used by Phase 12 code
  are covered by existing layer definitions
- Run `godark vet architecture` and confirm it passes
- If any new package directories were created, add them to the appropriate
  layer

### Acceptance criteria

- [ ] `godark vet architecture` passes after all Phase 12 issues are merged
- [ ] No new package directories exist outside of defined layer paths

### Test cases

- **Vet passes**: `godark vet architecture` exits 0 with no findings
- **No unknown paths**: All `.go` files are within paths defined in
  architecture.json layers
