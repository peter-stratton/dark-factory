---
name: godark-configure-project
description: Analyze an existing project and populate godark.yaml with modules, codegen, CI, and env configuration
argument-hint: "[project-path]"
disable-model-invocation: true
---

# Configure Project

Analyze an existing project and populate `godark.yaml` with the complex
configuration fields that describe project structure. Scans for language
markers, codegen configs, CI workflows, and project layout, then presents
findings for user confirmation before writing.

## Steps

1. **Read existing configuration** — Read `godark.yaml` if it exists. Note
   which fields are already set — those fields must not be overwritten. If
   `godark.yaml` does not exist, start with an empty config.

2. **Detect project modules** — Search recursively for module definition files
   to identify multi-module projects:
   - `go.mod` files (Go modules)
   - `pubspec.yaml` files (Dart/Flutter packages)
   - `package.json` files (Node.js/JavaScript packages — skip
     `node_modules/` directories)

   For each module found, note its directory path relative to the project
   root. If more than one module is found, prepare a `modules:` map keyed
   by module name (typically the directory path). For each module, propose:
   - `build_command:` — language-appropriate default (e.g. `go build ./...`,
     `flutter build`, `npm run build`)
   - `test_command:` — language-appropriate default (e.g. `go test ./...`,
     `flutter test`, `npm test`)
   - `lint_command:` — language-appropriate linter (optional, omit if unknown)
   - `generate_command:` — per-module codegen command (optional, omit if none)
   - `depends_on:` — list of other module names this module imports; omit
     if none

3. **Detect code generation configuration** — Search for codegen config files
   and patterns that indicate generated code:
   - `sqlc.yml` or `sqlc.yaml` (SQL code generation via sqlc)
   - `gqlgen.yml` or `gqlgen.yaml` (GraphQL code generation via gqlgen)
   - `.mockery.yaml` (mock generation via mockery)
   - `buf.yaml` or `buf.gen.yaml` (protobuf generation via buf)
   - `*.proto` files (protocol buffer definitions — implies `buf generate` or
     `protoc`)
   - `Makefile` — scan for targets or commands containing `generate` (e.g.
     `go generate ./...`, `sqlc generate`, `buf generate`)
   - `build.yaml` or other custom build definitions

   From the findings, propose:
   - `generate_command:` — the command(s) that run all code generation (e.g.
     `go generate ./...`, `make generate`)
   - `generated_paths:` — directories where generated files are written (e.g.
     `internal/db/generated/`, `internal/mocks/`, `gen/`)

4. **Detect environment variable requirements** — Search for environment
   variable declarations and references:
   - `.env.example` or `.env.sample` — extract variable names (lines of the
     form `VAR_NAME=` or `export VAR_NAME=`)
   - `required_env:` keys already present in CI config files
   - `environment:` or `secrets:` keys in `docker-compose*.yml` and
     `compose.yml` files
   - `env:` blocks in `.github/workflows/*.yml` files at the workflow or
     job level

   Compile a deduplicated, sorted list of variable names for the `required_env`
   field. Exclude variables that look like build metadata (e.g. `CI`, `RUNNER_*`,
   `GITHUB_*`) unless they appear in `.env.example`.

5. **Detect CI workflow configuration** — Scan `.github/workflows/*.yml` files:
   - For each workflow file, read the `jobs:` block and extract job keys.
   - Identify jobs that represent required checks (any job that runs tests,
     linting, building, or static analysis — e.g. jobs named `test`, `build`,
     `lint`, `ci`, `check`, `verify`, `validate`).
   - Prepare a `wait_for_checks:` block with `timeout` (e.g. `"10m"`) and
     `required` (list of check names). The check name for a job is typically
     the job key or its `name:` field if set.

6. **Detect Docker Compose usage** — Check for `docker-compose.yml`,
   `docker-compose.yaml`, `docker-compose*.yml`, or `compose.yml` files at
   any level of the project tree. If found:
   - Read the compose file and extract the `services:` block.
   - For each service, note its name and try to infer a short description
     from the `image:` field, port mappings, and any comments. For example,
     `image: postgres:16` with `ports: ["5432:5432"]` →
     `"PostgreSQL 16 test database on port 5432"`.
   - Prepare a `docker_compose:` block with:
     - `file:` — the path to the compose file (relative to project root)
     - `project_name:` — leave empty unless the compose file sets one
     - `services:` — list of `{name, description}` entries for each service
   - If multiple compose files exist, prefer the one that looks test-oriented
     (e.g. `docker-compose.test.yml`) or ask the user which one to use.

7. **Present findings** — Display a structured summary of all detected
   configuration. For each section, show the proposed `godark.yaml` snippet:

   - **Modules** — list detected modules and proposed build/test commands
   - **Code generation** — list detected tools and proposed `generate_command`
     and `generated_paths`
   - **Required environment** — list proposed `required_env` variables
   - **CI checks** — list proposed `wait_for_checks` names
   - **Docker Compose** — show the proposed `docker_compose:` block with
     file path and service descriptions

   For each section, ask the user to confirm, edit, or skip before writing.
   Do not write anything until the user has reviewed and approved the findings.

8. **Merge into `godark.yaml`** — Write only the confirmed fields. Merge rules:
   - Do not overwrite any fields that were already set when the skill started.
   - Append new top-level fields after existing content.
   - If `godark.yaml` does not exist, create it with just the confirmed fields.
   - Preserve existing comments and formatting.
   - If the user skipped a section, do not write those fields.

## Schema

The full JSON Schema for `godark.yaml` is in
`godark-config-schema.json` (sibling of this file). **Always consult the
schema before generating YAML** to verify field names, types, and nesting.

Key points the schema enforces:
- `modules` is a **map** (object), NOT a list. Keys are module names
  (typically directory paths); values are Module objects.
- `generate_command` is a **single string**, not a structured object.
  Chain multiple commands with `&&`.
- `Module` fields: `build_command`, `test_command`, `lint_command`,
  `generate_command` (all strings), `depends_on` (array of strings).
  All optional.

## Format

Example of correct `godark.yaml` output for the fields this skill populates:

```yaml
# Multi-module: keys are directory paths, values are Module objects
modules:
  services/api:
    build_command: go build ./...
    test_command: go test ./...
  services/worker:
    build_command: go build ./...
    test_command: go test ./...
    generate_command: "sqlc generate"
    depends_on:
      - services/api

# Top-level codegen (alternative to per-module generate_command)
generate_command: "cd services/api && go generate ./... && cd ../worker && sqlc generate"
generated_paths:
  - internal/db/generated/
  - internal/mocks/

required_env:
  - DATABASE_URL
  - API_KEY
  - SECRET_TOKEN

wait_for_checks:
  timeout: "10m"
  required:
    - test
    - lint
    - build

docker_compose:
  file: docker-compose.test.yml
  services:
    - name: postgres
      description: "PostgreSQL 16 test database on port 5432"
    - name: redis
      description: "Redis 7 cache on port 6379"
```

## Rules

- This skill is **re-runnable**. Always read existing `godark.yaml` first and
  merge findings rather than overwriting.
- Do not overwrite any fields already present in `godark.yaml` before this
  skill ran.
- Do not skip the confirmation step — the user must approve each section
  before it is written.
- If no multi-module structure is found (only one module at the root), do not
  add a `modules:` field.
- If no codegen configuration is found, do not add `generate_command` or
  `generated_paths` fields.
- If no environment variable requirements are detected, do not add
  `required_env`.
- If no CI workflows are found, do not add `wait_for_checks`.
- When Docker Compose is detected, populate the `docker_compose:` block
  with service descriptions inferred from the compose file. Do not add it
  automatically without user confirmation.
- Skip `node_modules/`, `.git/`, and other vendor directories when scanning.
