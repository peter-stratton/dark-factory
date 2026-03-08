# Phase 8: Harness Engineering

Phase 8 makes any project — new or existing — ready for agent-driven development by scaffolding a set of structured "harness" files that agents read before they write code. These files define architecture layers, coding conventions, prompt templates, and the communication format agents use on pull requests. The phase also adds validation tooling so you can catch bad architecture definitions before agents ever execute, and two interactive skills that help you fill in the harness files through conversation rather than from scratch.

---

## `godark new` — Create a harness-ready project from scratch

Creates a new directory with every harness file an agent needs to operate: CLAUDE.md, architecture and conventions templates, prompt templates, a roadmap skeleton, and a default `godark.yaml`. Runs `git init` and installs planning skills. The result is a project that is immediately ready for `/godark-create-roadmap` and then `godark run`.

### Example

```
$ godark new billing-service --repo acme/billing-service

wrote billing-service/CLAUDE.md
wrote billing-service/.gitignore
initialized git repository in billing-service/
wrote .claude/skills/godark-create-roadmap/SKILL.md
wrote .claude/skills/godark-create-planning-doc/SKILL.md
wrote .claude/skills/godark-create-issues/SKILL.md
wrote .claude/skills/godark-create-scenarios/SKILL.md
wrote .claude/skills/godark-define-architecture/SKILL.md
wrote .claude/skills/godark-define-conventions/SKILL.md
wrote godark.yaml
wrote docs/architecture.md
wrote docs/architecture.json
wrote docs/conventions.md
wrote docs/ROADMAP.md
wrote prompts/implementer.txt
wrote prompts/implementer_retry.txt
wrote prompts/reviewer.txt

Project "billing-service" created.

Next steps:
  1. Fill in the language-specific sections of CLAUDE.md
  2. Run /godark-create-roadmap to define phases
  3. Use `godark vet` to validate before execution
```

The scaffolded CLAUDE.md is deliberately short — under 20 lines. It contains pointers to subordinate docs, not the docs themselves:

```markdown
# CLAUDE.md

## Where to look

- Architecture and layer definitions — docs/architecture.md
- Coding conventions — docs/conventions.md
- Roadmap and phasing — docs/ROADMAP.md
- Build, test, and runtime config — godark.yaml
```

This keeps CLAUDE.md stable. File paths, build commands, and style rules live in prompt templates and `godark.yaml`, where they can change without touching the file that loads into every agent session.

---

## `godark init` — Retrofit harness files into an existing project

For projects that already have code, `godark init` scaffolds the same harness docs alongside the existing skills and config installation. Every file uses skip-if-exists semantics, so running `init` twice is safe. By default it does not write CLAUDE.md (existing projects have their own), but the `--reset-claude-md` flag opts in.

### Example

```
$ cd ~/code/existing-api
$ godark init

wrote .claude/skills/godark-create-roadmap/SKILL.md
wrote .claude/skills/godark-define-architecture/SKILL.md
wrote .claude/skills/godark-define-conventions/SKILL.md
...
skipped godark.yaml (already exists)
wrote docs/architecture.md
wrote docs/architecture.json
wrote docs/conventions.md
skipped docs/ROADMAP.md (already exists)
wrote prompts/implementer.txt
wrote prompts/implementer_retry.txt
wrote prompts/reviewer.txt
hint: review your CLAUDE.md against the harness principles in docs/architecture.md (use --reset-claude-md to replace it)
```

Files that already exist are left alone. If you want to replace your CLAUDE.md with the harness template:

```
$ godark init --reset-claude-md
replacing existing CLAUDE.md with harness template
wrote CLAUDE.md
```

---

## Harness document templates

All harness files come from embedded Go templates in `internal/harness/templates/`. This means they ship inside the `godark` binary — no network fetch, no external dependencies. The templates include:

| File | Purpose |
|------|---------|
| `claude.md` | Short control doc with section pointers |
| `architecture.md` | Layer definition prose with guidance comments |
| `architecture.json` | Machine-readable layer definitions (example structure) |
| `conventions.md` | Coding convention sections: error handling, logging, testing, naming |
| `roadmap.md` | Minimal roadmap skeleton |
| `gitignore` | Ignores `tests/review/` and `logs/` |
| `prompts/implementer.txt` | Implementer agent prompt with dialogue instructions |
| `prompts/implementer_retry.txt` | Retry prompt — reads prior review feedback from PR comments |
| `prompts/reviewer.txt` | Reviewer agent prompt with architecture compliance checks |

The `WriteIfNotExists` helper creates parent directories and skips files that already exist, making both `godark new` and `godark init` idempotent.

---

## Architecture layer parser

The `internal/harness/layers/` package parses `docs/architecture.json` into a directed dependency graph. The JSON format is compact and agent-resistant (agents are less likely to corrupt JSON than markdown tables):

```json
{
  "layers": [
    {
      "name": "domain",
      "description": "Core business logic, no external dependencies.",
      "paths": ["internal/domain/"],
      "may_depend_on": [],
      "must_not_depend_on": ["infrastructure", "cmd"]
    },
    {
      "name": "infrastructure",
      "description": "Database, HTTP clients, external service adapters.",
      "paths": ["internal/infra/"],
      "may_depend_on": ["domain"],
      "must_not_depend_on": ["cmd"]
    },
    {
      "name": "cmd",
      "description": "CLI entrypoints and HTTP handlers.",
      "paths": ["cmd/", "internal/api/"],
      "may_depend_on": ["domain", "infrastructure"],
      "must_not_depend_on": []
    }
  ]
}
```

The parser validates that layer names are unique, all dependency references point to real layers, every layer has at least one path, and no names are empty. It deliberately does not check for cycles — that is the vet command's job.

---

## `godark vet architecture` — Validate layer definitions

Detects cycles in the architecture dependency graph using DFS with gray/black coloring. Reports all layers involved in each cycle. Gracefully skips when no architecture file exists (harnesses are opt-in).

### Example: clean architecture

```
$ godark vet architecture
No findings.
```

### Example: cycle detected

```
$ godark vet architecture
ERROR [architecture] cycle detected: service → storage → service
```

### Example: no architecture file

```
$ godark vet architecture
info: architecture file "docs/architecture.json" not found — skipping (harnesses are opt-in)
```

The `--architecture-file` flag lets you point at a different path, and `--json` produces machine-readable output for CI integration.

---

## Agent dialogue via PR comments

Prompt templates now instruct agents to communicate reasoning through structured PR comments instead of opaque diffs. The implementer posts **Implementation Notes** after opening a PR. The reviewer reads those notes and posts **Review Notes** when requesting changes. On retry, the implementer reads the full comment thread — its own prior notes and the reviewer's challenges — before making fixes.

### What the PR comment thread looks like

After a typical implement-review-retry-approve cycle, a PR accumulates this thread:

```
[Implementer] ## Implementation Notes
### Approach
Added a new storage.Writer interface in the infrastructure layer...
### Key Decisions
- Chose buffered writes over streaming because...
### Architecture
Touched infrastructure and domain layers. Domain defines the Writer
interface; infrastructure implements it.

[Reviewer] ## Review Notes
### Changes Requested
- Writer.Flush() swallows errors — must propagate to caller
### Architecture Compliance
No layer violations found.

[Implementer] ## Implementation Notes
### Approach
Fixed Flush() to return error and updated all call sites...

[Reviewer] ## Review Notes
### Approved
All acceptance criteria met. Error propagation verified.
### Architecture Compliance
No violations.
```

This gives humans a readable audit trail when spot-checking in the dashboard or on GitHub, and it gives the retry agent the specific reasoning it needs to fix issues without re-discovering context.

---

## Architecture and conventions references in prompts

The implementer and reviewer prompt templates now conditionally inject architecture and conventions doc content at execution time via template variables (`{{.ArchitectureDocContent}}`, `{{.ConventionsDocContent}}`, `{{.ArchitectureJSON}}`). When the files exist, agents see the full layer rules and coding standards inline in their prompt. When the files are absent or empty, the conditional blocks are skipped — no errors, no wasted tokens.

The reviewer prompt gets both the prose architecture doc and the machine-readable JSON layer definitions, so it can mechanically check that imports in changed files respect `may_depend_on` and `must_not_depend_on` rules for each layer.

---

## `/godark-define-architecture` skill

An interactive skill that helps you create or update `docs/architecture.json` and `docs/architecture.md`. For existing codebases, it scans package directories and import relationships to propose layers. For new projects, it asks about your language and framework and recommends idiomatic layers. It runs `godark vet architecture` to validate the result, and if it finds discrepancies between your defined layers and actual code, suggests running `/godark-create-roadmap` to plan a codebase alignment phase.

### Example session

```
> /godark-define-architecture

I'll help you define architecture layers for this project.

I found the following package structure:
  internal/cmd/      — CLI commands
  internal/service/  — business logic
  internal/storage/  — database access
  internal/config/   — configuration loading
  internal/types/    — shared types

I'd propose these layers (bottom to top):
  types → config → storage → service → cmd

Does this match your intent? Should any packages move layers
or should I add additional constraints?
```

The skill is re-runnable. It reads existing definitions first and proposes updates rather than overwriting.

---

## `/godark-define-conventions` skill

An interactive skill that helps you create or update `docs/conventions.md`. For existing codebases, it reads a sample of source files to identify patterns for error handling, logging, testing, naming, and dependency injection. For new projects, it recommends idiomatic conventions. Every recommendation passes through an agent-friendliness filter:

- **Explicit over implicit** — named dependencies, not DI containers
- **Local over global** — function signatures, not shared mutable state
- **Clear boundaries** — interfaces between layers, not deep inheritance
- **Discoverable** — patterns visible in a few files, not requiring whole-system knowledge

The skill flags patterns that impede agentic development (heavy code generation, convention-over-configuration magic, implicit runtime behavior) and suggests agent-friendly alternatives. If the codebase uses inconsistent conventions, it suggests `/godark-create-roadmap` to plan a standardization phase.

---

## Updated planning skills

The existing `/godark-create-roadmap` and `/godark-create-planning-doc` skills were updated to read `docs/architecture.json` and `docs/conventions.md` for context. When a roadmap conversation reveals architectural decisions, the skill prompts you to run `/godark-define-architecture`. When a planning doc introduces packages that don't fit the current layer structure, the skill prompts you to update `docs/architecture.json`. This closes the loop: planning conversations feed harness files, and harness files feed agent execution.

---

## Key files

| Path | What it is |
|------|-----------|
| `internal/harness/templates/` | Embedded harness document templates |
| `internal/harness/layers/layers.go` | Architecture JSON parser |
| `internal/vet/architecture.go` | Cycle detection for layer definitions |
| `internal/cmd/new.go` | `godark new` command |
| `internal/cmd/init.go` | `godark init` command (expanded) |
| `internal/cmd/vet_architecture.go` | `godark vet architecture` subcommand |
| `internal/skills/godark-define-architecture/` | Architecture definition skill |
| `internal/skills/godark-define-conventions/` | Conventions definition skill |
| `docs/design/harnesses.md` | Design doc: harness types and principles |
| `docs/design/godark-new.md` | Design doc: `godark new` scaffolding |
