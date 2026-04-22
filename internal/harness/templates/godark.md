# godark

This project is managed by [godark](https://github.com/peter-stratton/dark-factory),
a CLI that orchestrates autonomous AI agents to implement GitHub issues, review
their own work, and merge — without human intervention.

## How the pipeline works

When `godark run` or `godark implement` is invoked, each issue goes through:

```
implement → verify → quality review → functional review → merge
```

1. **Implementer** — an agent implements the issue on a feature branch and opens a PR
2. **Verify** — deterministic checks run in order: `generate → build → lint → test`
   - If any step fails, a fix agent reads the errors and pushes a correction
   - This repeats up to `verify.max_fix_attempts` times before failing
3. **Quality reviewer** — a separate agent audits the PR for security, performance, architecture compliance, and code quality
4. **Functional reviewer** — another agent reviews against human-authored scenario specs and writes ephemeral integration tests
5. **Merge or escalate** — approved PRs are squash-merged; failed PRs are labeled `needs-human-review`

If either reviewer requests changes, the implementer reads the review comments
and pushes fixes (up to `max_retries` cycles per gate).

## Configuration reference (godark.yaml)

### Build and verify commands

These commands run inside the sandbox container during the verify step.
Auto-detected from project marker files if not set.

| Field | Purpose | Example |
|-------|---------|---------|
| `build_command` | Compile the project | `go build ./...`, `flutter build` |
| `test_command` | Run the test suite | `go test ./...`, `flutter test` |
| `format_command` | Auto-format source files | `go fmt ./...`, `dart format .` |
| `lint_command` | Check formatting and lint rules | `dart format --set-exit-if-changed .` |
| `generate_command` | Run code generation before build | `dart run build_runner build` |

The verify step runs them in order: **generate → build → lint → test**. If any
step fails, the verify-fix agent attempts to correct the issue automatically.

### Agent behavior

| Field | Purpose | Default |
|-------|---------|---------|
| `max_retries` | Review/fix cycles before escalating to human | `3` |
| `max_resume_retries` | After this many retries, switch from session resumption to fresh session with structured handoff | `2` |
| `max_rebase_attempts` | Auto rebase/conflict-fix cycles before labeling needs-human-review (0 = disable) | `1` |
| `agent_timeout` | Max wall-clock time per agent run (Go duration) | `30m` |
| `model` | Default Claude model for all agent steps. Accepts an alias (`sonnet`, `opus`, `haiku`, `opusplan`) or a full model id (`claude-opus-4-7`, `claude-sonnet-4-6-20250929`), optionally with a variant suffix like `opus[1m]`. | `""` (CLI default) |
| `model_overrides` | Per-role model overrides (map of role → model, same value format as `model`) | `{}` |
| `auto_merge.feature` | Merge strategy for feature PRs after approval: `none`, `low_risk`, `all` | `none` |
| `auto_merge.rollup` | Rollup PR handling after a run completes: `none`, `manual`, `auto` | `manual` |
| `base_branch` | Base branch for feature PRs. Auto-generated when omitted: `godark/phase-N` for milestone runs, `godark/issue-N` for implement runs. Set to `main` to merge directly to the default branch without a rollup PR. | auto-generated |
| `rollup_title` | Custom title for rollup PR (supports `{{.BaseBranch}}` and `{{.DefaultBranch}}` templates) | `""` |
| `default_branch` | Default branch of the repo (auto-detected from GitHub if omitted) | auto-detect / `main` |
| `branch_prefix` | Prefix for auto-generated branch names | `godark` |
| `label_prefix` | Prefix for GitHub labels | `godark` |
| `quality_strictness_decay` | Use diminishing strictness on quality review retries | `true` |

#### Model overrides

Use `model_overrides` to run cheaper models on steps that don't need full Opus
reasoning. Keys are role names passed to the agent launcher:

```yaml
model: opus
model_overrides:
  planner: claude-opus-4-7
  recon: sonnet
  quality_reviewer: sonnet
  spec_generator: sonnet
```

Values can be a Claude Code alias (`sonnet`, `opus`, `haiku`, `opusplan`) or a
full model id (`claude-opus-4-7`, `claude-sonnet-4-6-20250929`), optionally
with a `[variant]` suffix like `opus[1m]`. Use full ids when you need to pin
exact versions as new models roll out.

Valid roles: `implementer`, `implementer_retry`, `reviewer`, `reviewer_semiformal`,
`quality_reviewer`, `recon`, `planner`, `spec_generator`, `verify_fix`,
`merge_coordinator`.

### Paths and constraints

| Field | Purpose | Default |
|-------|---------|---------|
| `protected_paths` | Files agents must never modify | `[]` |
| `denied_commands` | Shell commands agents must not run | `[]` |
| `generated_paths` | Glob patterns for generated code (excluded from review) | `[]` |
| `roadmap_path` | Directory containing roadmap files | `docs/roadmap/` |
| `planning_dir` | Directory for planning documents | `docs/planning/` |
| `scenario_dir` | Directory containing scenario spec files | `tests/scenarios/` |
| `review_dir` | Directory where the reviewer writes ephemeral integration tests | `tests/review/` |

### Architecture enforcement

| Field | Purpose | Default |
|-------|---------|---------|
| `architecture_doc` | Path to architecture doc (markdown) | `docs/architecture.md` |
| `architecture_json` | Path to machine-readable layer definitions | `docs/architecture.json` |
| `conventions_doc` | Path to coding conventions doc | `docs/conventions.md` |
| `enforce_architecture` | Inject architecture rules into agent prompts | `false` |

### Verify step tuning

| Field | Purpose | Default |
|-------|---------|---------|
| `verify.max_fix_attempts` | Auto-fix attempts per verify failure | `3` |
| `verify.blocking` | Fail the issue if verify doesn't pass | `true` |

### Review step tuning

| Field | Purpose | Default |
|-------|---------|---------|
| `review.semiformal` | Use semiformal reviewer | `false` |
| `review.semiformal_on_retry` | Use semiformal reviewer on retries | `false` |

### Quality review tuning

| Field | Purpose | Default |
|-------|---------|---------|
| `quality.min_review_cost_usd` | Flag reviews cheaper than this (likely rubber-stamped) | `0` |
| `quality.min_review_duration_seconds` | Flag reviews shorter than this | `0` |

### CI integration

| Field | Purpose | Default |
|-------|---------|---------|
| `wait_for_checks.timeout` | Max time to wait for CI checks to complete | |
| `wait_for_checks.required` | List of CI check names that must pass before merge | `[]` |
| `wait_for_checks.startup_grace` | Grace period for checks to appear before treating as N/A | `60s` |

### Output truncation

| Field | Purpose | Default |
|-------|---------|---------|
| `truncation.verify_output` | Max bytes from verify command stdout+stderr | `4096` |
| `truncation.pr_diff` | Max bytes from PR diff embedded in prompts | `30000` |

### Container health judge

| Field | Purpose | Default |
|-------|---------|---------|
| `judge.enabled` | Enable the container health judge | `true` |
| `judge.default_idle_timeout` | Seconds before killing an idle agent | `300` |
| `judge.default_no_progress_timeout` | Seconds with no meaningful progress (0 = disabled) | `0` |
| `judge.tool_thrash_threshold` | Repeated identical tool calls to trigger kill | `3` |
| `judge.tool_thrash_window_secs` | Window in seconds for tool thrash detection | `60` |
| `judge.transport_failure_threshold` | Transport errors before killing agent | `10` |
| `judge.container_retry_limit` | Container restart attempts | `2` |
| `judge.idle_timeout_by_role` | Per-role idle timeout overrides (map of role → seconds) | `{}` |
| `judge.no_progress_timeout_by_role` | Per-role no-progress timeout overrides | `{}` |

### Concurrency

| Field | Purpose | Default |
|-------|---------|---------|
| `concurrency.max_workers` | Maximum parallel workers for issue processing | `1` |

Mutually exclusive with `--integration` on the CLI: integration runs require
`max_workers=1` because compose services are shared host state.

### Watch

| Field | Purpose | Default |
|-------|---------|---------|
| `watch.poll_interval` | Polling interval for the watch command (Go duration) | `60s` |

### Environment

| Field | Purpose | Default |
|-------|---------|---------|
| `sandbox_env` | Environment variables passed into the sandbox (map of name → value) | `{}` |
| `required_env` | Environment variable names that must be set before a run starts; values are forwarded to the sandbox | `[]` |
| `auth_preference` | Preferred auth token when both are available: `oauth` or `api_key` | `oauth` |

### Rollup modes (`auto_merge.rollup`)

When godark runs against a non-default base branch (which happens automatically
unless `base_branch` is set to the default branch), a rollup PR merges the base
branch into the default branch after all feature PRs are done. The `rollup` field
controls what godark does with that rollup PR:

| Mode | Feature PRs → base branch | Base branch → main |
|---|---|---|
| `none` | godark merges | human does everything (inspects branch, opens PR manually) |
| `manual` (default) | godark merges | godark opens PR, human reviews and merges |
| `auto` | godark merges | godark opens PR and merges |

To disable rollup PRs and merge feature branches directly to the default branch,
set `base_branch: main` (or your repo's default branch name) in `godark.yaml`.

### Risk thresholds (for `auto_merge.feature: low_risk`)

| Field | Purpose | Default |
|-------|---------|---------|
| `risk_thresholds.max_lines` | PRs changing more lines are not low-risk | `500` |
| `risk_thresholds.max_files` | PRs changing more files are not low-risk | `10` |

### Multi-module projects

```yaml
modules:
  backend:
    build_command: "cd backend && go build ./..."
    test_command: "cd backend && go test ./..."
    depends_on: []
  frontend:
    build_command: "cd frontend && npm run build"
    test_command: "cd frontend && npm test"
    depends_on: [backend]
```

Modules are verified in dependency order. Each module can override the root-level
build, test, lint, and generate commands. Commands run from the repo root — include
any `cd` needed to target the correct subdirectory.

### Notifications

```yaml
notify:
  - provider: telegram
    events: [run_complete, abort]
    settings:
      bot_token: ${TELEGRAM_BOT_TOKEN}
      chat_id: ${TELEGRAM_CHAT_ID}
```

### Runtime

| Field | Purpose | Example |
|-------|---------|---------|
| `runtime.name` | Project toolchain | `go`, `node`, `flutter`, `rust`, `python` |
| `runtime.version` | Toolchain version (auto-detected if empty) | `1.23`, `20` |

The runtime determines which language tools are installed in the sandbox container.

### Docker Compose

If the project uses Docker Compose for local services (databases, caches, etc.),
declare them so agents know what's available during development and testing.

```yaml
docker_compose:
  file: docker-compose.yml
  project_name: myproject
  services:
    - name: postgres
      description: PostgreSQL 15 on port 5432, database "app_dev"
    - name: redis
      description: Redis 7 on port 6379, no auth
```

| Field | Purpose |
|-------|---------|
| `docker_compose.file` | Path to the compose file (required when block is present) |
| `docker_compose.project_name` | Project name prefix for containers (optional) |
| `docker_compose.services[].name` | Service name as defined in the compose file |
| `docker_compose.services[].description` | What the service provides (ports, credentials, database names) |

### Host services

If the project depends on services that run on the host (outside Docker Compose),
declare them so godark can verify they are reachable before processing issues and
inject their descriptions into agent prompts.

```yaml
host_services:
  - name: supabase
    description: "Supabase local stack (Postgres, Auth, Realtime, Studio)"
    health_check:
      command: "curl -sf http://localhost:54321/rest/v1/"
      timeout: "10s"
      retries: 5
  - name: wrangler
    description: "Cloudflare Workers local dev (R2, KV, DO)"
    health_check:
      command: "curl -sf http://localhost:8787/"
```

| Field | Purpose |
|-------|---------|
| `host_services[].name` | Service name (required) |
| `host_services[].description` | What the service provides — injected into agent prompts |
| `host_services[].health_check.command` | Shell command to check reachability (required when health_check is set) |
| `host_services[].health_check.timeout` | Max time per health check attempt (default: `5s`) |
| `host_services[].health_check.retries` | Number of attempts before failing (default: `3`) |

godark does **not** start or stop host services — it only verifies they are
reachable. Start them before running `godark run` or `godark implement`.

### Docker sandbox

| Field | Purpose | Default |
|-------|---------|---------|
| `docker.image` | Base image | `ubuntu:22.04` |
| `docker.dockerfile` | Custom Dockerfile path (overrides auto-generated one) | |
| `docker.mount` | Docker mount configuration | |
| `docker.user` | User to run commands as in the container | |
| `docker.node_version` | Node.js major version to install | `20` |
| `docker.extra_packages` | Additional apt packages to install | `[]` |
| `docker.install_commands` | Shell commands to run during image build (after runtime setup) | `[]` |

## Run history and artifacts

godark stores run data on the host machine at `~/.godark/`:

- `stats.db` — SQLite database with cost, duration, and outcome trends across all runs
- `runs/{owner}/{repo}/{timestamp}/run.json` — per-run record with issue numbers, timing, and pass/fail summary

When debugging a failed implementation, check the most recent run.json for the
repo to see which issues were attempted and their outcomes.

## Common troubleshooting

**CI fails on formatting** — Add `lint_command` to godark.yaml (e.g.,
`dart format --set-exit-if-changed .`). The verify step runs lint before review,
and the fix agent auto-corrects formatting failures.

**Agent modifies protected files** — Add paths to `protected_paths` in
godark.yaml. The guard rail step closes the PR if protected files are touched.

**Agent runs commands it shouldn't** — Add commands to `denied_commands` (e.g.,
`rm -rf`). The sandbox blocks these at execution time.

**Generated code causes review noise** — Add glob patterns to `generated_paths`
(e.g., `**/*.freezed.dart`). The reviewer skips these files.

**Tests pass locally but fail in CI** — Add the CI check names to
`wait_for_checks.required` so godark waits for CI before merging.
