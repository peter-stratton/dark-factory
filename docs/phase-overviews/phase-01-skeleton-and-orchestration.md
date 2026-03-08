# Phase 1: Skeleton and Orchestration

Phase 1 laid the foundation for the entire system: a Go CLI that fetches GitHub issues from a milestone, resolves their dependency graph, sorts them by priority, and prints an execution plan. No agents run in this phase -- the goal was to get the orchestration loop right before anything autonomous touches code. It also introduced `godark init` to bootstrap new projects with skills, config, and prompt templates, establishing the pattern where godark manages its own project scaffolding.

---

## CLI Skeleton (Cobra)

The `godark` binary uses Cobra for command routing. Phase 1 established the root command and the `run` subcommand, with flags for repo, milestone, dry-run, and config path.

**In practice:** You install godark and immediately have a self-documenting CLI with help text, flag validation, and subcommand discovery.

```
$ godark run --help
Fetch issues from a GitHub milestone, resolve dependencies, and process
each unblocked issue through the implement -> review -> merge loop.

Usage:
  godark run [flags]

Flags:
      --config string      Path to configuration file (default "godark.yaml")
      --dry-run             Print execution plan without taking action
      --milestone string    GitHub milestone to process (exact title)
      --repo string         GitHub repository (owner/repo)
```

---

## YAML Config with CLI Flag Overrides

Configuration lives in `godark.yaml` at the project root. The config loader reads the YAML file, applies sensible defaults (e.g., `max_retries: 3`, `roadmap_path: docs/ROADMAP.md`), then merges any CLI flags on top. A missing config file is not an error -- flags can supply everything needed.

**In practice:** You commit a minimal config to your repo and override per-run as needed.

```yaml
# godark.yaml
repo: "acme/payments-service"

prompts:
  implementer: prompts/implementer.txt
  implementer_retry: prompts/implementer_retry.txt
  reviewer: prompts/reviewer.txt
```

```
$ godark run --milestone "Phase 1" --max-retries 5
```

The `--max-retries 5` overrides whatever is in the YAML. The config file sets the project baseline; flags let you experiment without editing it.

---

## GitHub Issue Fetching with Priority Sorting

The `github` package calls `gh issue list` to fetch all open issues in a milestone, then sorts them by priority label: p1 first, then p2, then p3, then unlabeled. Within each tier, issues sort by number ascending. This determines execution order -- critical issues get implemented before nice-to-haves.

**In practice:** You label your issues in GitHub and the execution order falls out automatically.

```
Processable issues:
  #3 Add payment webhook handler [priority: p1]
  #5 Implement retry logic for failed charges [priority: p1]
  #1 Update API documentation [priority: p2]
  #7 Add metrics endpoint [priority: none]
```

No need to manually sequence work. Label `p1` on the issues that matter and godark handles the rest.

---

## Dependency Resolution

Issues declare dependencies in their body using `Blocked by: #N` or `Depends on: #N` syntax. The `deps` package parses these declarations with a regex, checks which referenced issues are closed, and filters the issue list down to only those whose dependencies are all satisfied. Blocked issues are reported separately so you can see exactly what is holding them up.

**In practice:** You write natural-language dependency declarations in your issue bodies and godark respects them.

```markdown
<!-- In issue #4's body -->
**Blocked by**: #3 (webhook handler must exist before we add retry logic)
Depends on: #2
```

During a dry run, godark shows what is blocked and why:

```
Blocked issues:
  #4 Add webhook retry queue (blocked by: #3)
```

Once #3 closes, #4 becomes processable in the next run -- no manual intervention required.

---

## Structured Logging

Every run produces two log streams simultaneously: structured JSON written to `debug.log` for machine consumption, and human-readable text printed to stdout. This is implemented as a `multiHandler` that fans each `slog` record out to both a `JSONHandler` and a `TextHandler`.

**In practice:** You watch stdout during a run and grep the JSON log after.

stdout shows:
```
time=2025-01-15T10:30:00Z level=INFO msg="fetched issues" count=7
time=2025-01-15T10:30:00Z level=INFO msg="dependency resolution complete" total=7 blocked=2 processable=5
```

`debug.log` contains the same events as structured JSON, ready for `jq` or any log aggregation tool:
```json
{"time":"2025-01-15T10:30:00Z","level":"INFO","msg":"fetched issues","count":7}
```

---

## Dry-Run Mode

The `--dry-run` flag runs the full orchestration pipeline -- fetch issues, resolve dependencies, sort by priority -- but stops before executing any agents or making any changes. It prints the execution plan so you can verify that godark will do what you expect.

**In practice:** This is how you validate your issue setup before burning tokens on agent runs.

```
$ godark run --milestone "Phase 1" --repo acme/payments-service --dry-run

=== Execution Plan (dry-run) ===

Processable issues:
  #3 Add payment webhook handler [priority: p1]
  #5 Implement retry logic [priority: p1]
  #1 Update API docs [priority: p2]

Blocked issues:
  #4 Add webhook retry queue (blocked by: #3)

Summary: 4 issues total, 3 processable, 1 blocked
```

If an issue is blocked when it should not be, or the priority order looks wrong, you fix the issue labels and dependency declarations before committing to a real run.

---

## `godark init` -- Project Bootstrapping

The `init` command writes everything a project needs to work with godark: Claude Code skill files (always overwritten -- godark owns these), a default `godark.yaml` (skipped if one already exists), prompt templates, and harness documentation stubs for architecture and conventions.

**In practice:** You run one command in an existing repo and the project is ready for agent-driven development.

```
$ cd ~/code/my-service
$ godark init --repo acme/my-service

wrote .claude/skills/godark-create-roadmap/...
wrote .claude/skills/godark-create-issues/...
wrote .claude/skills/godark-create-planning-doc/...
wrote .claude/skills/godark-create-scenarios/...
wrote godark.yaml
wrote docs/architecture.md
wrote docs/conventions.md
wrote docs/ROADMAP.md
wrote prompts/implementer.txt
wrote prompts/implementer_retry.txt
wrote prompts/reviewer.txt
ensured label "godark-in-progress" in acme/my-service
```

Running `godark init` again is safe -- it overwrites the managed skill files (so you get updates) but skips config and docs that you have already customized.

---

## Planning Skills

Phase 1 shipped four Claude Code skills that turn high-level project goals into actionable GitHub issues. These are slash commands you invoke inside Claude Code to plan work before godark executes it.

**`/godark-create-roadmap`** -- Generates a phased roadmap document from your project goals, writing it to `docs/ROADMAP.md`.

**`/godark-create-planning-doc`** -- Produces a detailed planning document for a specific phase, breaking it into concrete implementation tasks.

**`/godark-create-issues`** -- Reads a planning document and creates GitHub issues with proper structure: acceptance criteria, dependency declarations, priority labels, and milestone assignment.

**`/godark-create-scenarios`** -- Generates scenario spec files that define expected behavior for each issue, used later by the reviewer agent to validate implementations.

**In practice:** The typical workflow is a pipeline from goals to executable issues.

```
> /godark-create-roadmap
  (writes docs/ROADMAP.md with phased milestones)

> /godark-create-planning-doc
  (writes docs/planning/phase-1-whatever.md with detailed tasks)

> /godark-create-issues
  (creates GitHub issues from the planning doc, with deps and labels)

> /godark-create-scenarios
  (writes tests/scenarios/ specs for reviewer validation)

$ godark run --milestone "Phase 1" --repo acme/my-service --dry-run
  (verifies everything looks right before executing)
```

The skills produce structured output that godark's later phases consume directly -- roadmap documents that `godark vet` validates, issue bodies with dependency syntax that the resolver parses, and scenario specs that the reviewer agent reads during code review.
