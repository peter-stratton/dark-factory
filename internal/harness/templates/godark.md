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
| `lint_command` | Check formatting and lint rules | `dart format --set-exit-if-changed .` |
| `generate_command` | Run code generation before build | `dart run build_runner build` |

The verify step runs them in order: **generate → build → lint → test**. If any
step fails, the verify-fix agent attempts to correct the issue automatically.

### Agent behavior

| Field | Purpose | Default |
|-------|---------|---------|
| `max_retries` | Review/fix cycles before escalating to human | `3` |
| `agent_timeout` | Max wall-clock time per agent run | `30m` |
| `auto_merge.feature` | Merge strategy for feature PRs after approval: `none`, `low_risk`, `all` | `none` |
| `auto_merge.rollup` | Rollup PR handling after a run completes: `none`, `manual`, `auto` | `none` |
| `no_sandbox` | Run agents on host instead of Docker | `false` |

### Paths and constraints

| Field | Purpose | Default |
|-------|---------|---------|
| `protected_paths` | Files agents must never modify | `[]` |
| `denied_commands` | Shell commands agents must not run | `[]` |
| `generated_paths` | Glob patterns for generated code (excluded from review) | `[]` |
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

### Quality review tuning

| Field | Purpose | Default |
|-------|---------|---------|
| `quality.min_review_cost_usd` | Flag reviews cheaper than this (likely rubber-stamped) | `0` |
| `quality.min_review_duration_seconds` | Flag reviews shorter than this | `0` |
| `quality_strictness_decay` | Use diminishing strictness on quality review retries | `true` |

### CI integration

| Field | Purpose |
|-------|---------|
| `wait_for_checks.timeout` | Max time to wait for CI checks to complete |
| `wait_for_checks.required` | List of CI check names that must pass before merge |

### Rollup modes (`auto_merge.rollup`)

When godark runs against a non-default base branch, a rollup PR merges the base
branch into main after all feature PRs are done. The `rollup` field controls
what godark does with that rollup PR:

| Mode | Feature PRs → base branch | Base branch → main |
|---|---|---|
| `none` | godark merges | human does everything (inspects branch, opens PR manually) |
| `manual` | godark merges | godark opens PR, human reviews and merges |
| `auto` | godark merges | godark opens PR and merges |

### Risk thresholds (for `auto_merge.feature: low_risk`)

| Field | Purpose | Default |
|-------|---------|---------|
| `risk_thresholds.max_lines` | PRs changing more lines are not low-risk | `500` |
| `risk_thresholds.max_files` | PRs changing more files are not low-risk | `10` |

### Multi-module projects

```yaml
modules:
  backend:
    build_command: go build ./...
    test_command: go test ./...
    depends_on: []
  frontend:
    build_command: npm run build
    test_command: npm test
    depends_on: [backend]
```

Modules are verified in dependency order. Each module can override the root-level
build, test, lint, and generate commands.

### Notifications

```yaml
notify:
  - provider: telegram
    events: [run_complete, abort]
    settings:
      bot_token: ${TELEGRAM_BOT_TOKEN}
      chat_id: ${TELEGRAM_CHAT_ID}
```

### Docker sandbox

| Field | Purpose |
|-------|---------|
| `docker.image` | Base image (default: `ubuntu:22.04`) |
| `docker.dockerfile` | Custom Dockerfile path (overrides auto-generated one) |
| `docker.extra_packages` | Additional apt packages to install |
| `docker.install_commands` | Shell commands to run during image build (after runtime setup) |
| `docker.node_version` | Node.js major version to install (default: `20`) |

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
