# Phase 12: Complex Project Support

Before this phase, godark could only orchestrate single-module projects with straightforward build pipelines. That covers a weekend project or a microservice with one `go.mod`, but it falls apart the moment you point it at a real production repo -- one with multiple modules that depend on each other, generated code from protobuf or sqlc, environment variables for private registries, and CI checks that must pass before anything merges. Phase 12 adds first-class support for all of these, turning godark from a tool for simple repos into one that handles the messy reality of production codebases.

---

## Multi-Module Monorepo Support

Maps subdirectories in a monorepo to their own build, test, lint, and generate commands, with explicit dependency relationships between modules. When modules are configured, the verify pipeline runs each module in topological order -- dependencies first, dependents after -- and stops immediately if a dependency fails.

**Example: configuring a two-module Go project**

Your repo has `service/` (the core API) and `admin-cli/` (a CLI that imports packages from `service/`). You add this to `godark.yaml`:

```yaml
modules:
  service:
    build_command: "go build ./..."
    test_command: "go test ./..."
    lint_command: "golangci-lint run"
  admin-cli:
    build_command: "go build ./..."
    test_command: "go test ./..."
    depends_on: [service]
```

When an agent implements an issue, the verify pipeline runs `service` first. If the build fails in `service/`, `admin-cli` is skipped entirely and the failure output is fed back to the implementer agent with the module name included:

```
Module: service

=== build (exit code 1) ===
service/handler.go:42:15: cannot use resp (variable of type *Response) as *OldResponse...
```

The agent sees exactly which module broke and why. If `service` passes, `admin-cli` runs next. Each module's commands execute with `cd <module-name> &&` prepended, so they run in the correct subdirectory.

When no `modules:` block is present, everything works exactly as before -- single-module mode with root-level commands.

**Config validation catches problems early.** A `depends_on` reference to a nonexistent module name fails at config load time. Cycles (A depends on B, B depends on A) are detected via DFS with three-color marking and rejected before any agent runs:

```
$ godark run --milestone "Phase 3"
Error: module "admin-cli" depends_on unknown module "svc"
```

Module names are also validated for filesystem safety -- only alphanumerics, hyphens, underscores, and dots are allowed, preventing shell injection via crafted module names.

---

## Code Generation Pipeline

Adds a `generate_command` field that runs before `build_command` in the verify pipeline, supporting tools like protoc, sqlc, gqlgen, mockery, and Flutter's build_runner. Pairs with `generated_paths` to protect generated files from direct agent edits.

**Example: a project using sqlc and mockery**

```yaml
generate_command: "make generate"
generated_paths:
  - service/repository/crdb/generated/
  - service/test/mocks/
  - "**/*.freezed.dart"
```

The verify pipeline now runs four steps in order: generate, build, lint, test. If generation fails, the remaining steps are skipped.

The `generated_paths` list is enforced by the agent runner's `PreToolUse` hook. When an agent tries to edit a file matching a generated path, the write is blocked with a targeted system message:

```
this file is generated -- edit the source file or re-run code generation instead
```

Directory entries use prefix matching (`service/repository/crdb/generated/` matches any file under that directory). Glob entries containing `*` use `fnmatch`-style matching (`**/*.freezed.dart` matches `lib/models/user.freezed.dart`). This is the same hook mechanism used for `protected_paths`, but with a distinct error message that tells the agent what to do instead.

The generated paths are also injected into prompt templates via `{{.GeneratedPaths}}`, so agents know upfront which files are generated and should not be hand-edited.

Per-module `generate_command` is also supported -- each module in the `modules:` block can specify its own generation step, falling back to the root-level command if omitted.

---

## Build Secrets and Required Environment

Declares environment variables that must be present before a run starts. Their values are forwarded into the sandbox container alongside auth tokens, but are never logged or written to run data.

**Example: a project that needs a private package registry token**

```yaml
required_env:
  - CLOUDSMITH_TOKEN
  - PUBSUB_EMULATOR_HOST
```

At startup, `godark run` or `godark implement` calls `ValidateRequiredEnv` before building the Docker image. If any variable is missing, you get a clear error instead of a cryptic failure ten minutes into an agent run:

```
$ godark run --milestone "Phase 3"
Error: missing required environment variables: CLOUDSMITH_TOKEN
```

When all variables are present, their values are forwarded to the sandbox via `CollectAuthEnv`. The forwarding logic explicitly prevents `required_env` from overriding auth-managed variables (`ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN`, `GH_TOKEN`) -- even if you list them in `required_env`, the auth preference logic stays in control. This prevents accidental bypasses of the `auth_preference` setting.

---

## CI Status Check Awareness

After the review cycle approves a PR, godark can poll GitHub status checks and wait for required CI checks to pass before merging. If a check fails, the failure output is fed back to the implementer for a fix cycle, using the same session resumption pattern as verify failures.

**Example: gating merge on a lint check**

```yaml
wait_for_checks:
  timeout: 10m
  required: [golangci-lint, apollo-check]
```

After the functional reviewer approves, godark polls `gh pr checks` every 30 seconds. Each required check is tracked individually:

- `COMPLETED` with `SUCCESS` -- passed, move on.
- `COMPLETED` with any other conclusion -- failed. The check name and conclusion are formatted and sent to the implementer agent via the verify-fix prompt for a fix cycle.
- `PENDING`, `QUEUED`, or `IN_PROGRESS` -- keep waiting.
- Not yet present in the check list -- treat as pending (the check may not have started yet).

If any required check fails, godark triggers a fix cycle immediately without waiting for other pending checks. Fix cycles respect `verify.max_fix_attempts`. If fixes are exhausted, the issue fails:

```
Error: CI checks failed: golangci-lint
```

If all checks stay pending past the timeout, the issue fails with a timeout error. When `wait_for_checks` is not configured, godark merges immediately after review approval -- the existing behavior.

Config validation ensures the timeout parses as a valid `time.Duration` and that the required list is non-empty when the block is present.

---

## The `/godark-configure-project` Skill

An embedded Claude skill installed by `godark init` that analyzes an existing project and proposes `godark.yaml` configuration for all the features in this phase. It scans the project tree for structural markers and presents findings for user confirmation before writing anything.

**Example: running the skill on an existing Go monorepo**

You have a project with two Go modules, sqlc for database code, GitHub Actions CI, and a `.env.example` file. You run the skill:

```
/godark-configure-project
```

The skill scans the project and finds:

1. **Modules** -- two `go.mod` files at `service/` and `admin-cli/`. It proposes a `modules:` block with per-module build/test commands and suggests `admin-cli` depends on `service` based on import analysis.

2. **Code generation** -- a `sqlc.yml` and a Makefile target called `generate`. It proposes `generate_command: "make generate"` and `generated_paths: ["service/repository/crdb/generated/"]`.

3. **Required environment** -- `.env.example` contains `CLOUDSMITH_TOKEN` and `DATABASE_URL`. It proposes `required_env: [CLOUDSMITH_TOKEN, DATABASE_URL]`. Build metadata variables like `CI` and `GITHUB_*` are filtered out.

4. **CI checks** -- `.github/workflows/ci.yml` has jobs named `test` and `lint`. It proposes `wait_for_checks: {timeout: "10m", required: [test, lint]}`.

5. **Docker Compose** -- a `docker-compose.yml` exists. It notes that `no_sandbox: true` may be needed for integration tests.

Each section is presented individually. You confirm, edit, or skip each one. Only confirmed fields are written. The skill merges into an existing `godark.yaml` without overwriting fields that were already set -- so it is safe to re-run as your project evolves.

The skill file lives at `internal/skills/godark-configure-project/SKILL.md` and is embedded via `internal/skills/embed.go`. It follows the same pattern as `/godark-define-architecture` -- a Claude skill described in markdown with frontmatter, not compiled Go code.
