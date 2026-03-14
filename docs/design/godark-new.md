# Design: `godark new` — Harness Scaffolding

## Summary

`godark new <project-name>` creates a new directory with harness files that
make a project ready for agent-driven development via `godark run`. It does
not scaffold language-specific files (no `go.mod`, `package.json`, etc.) —
only the harness structure that agents need to operate effectively.

See `harnesses.md` for the full harness type definitions and design
principles.

## Usage

```bash
godark new my-project
godark new my-project --repo owner/my-project
```

## What it creates

```
my-project/
├── .git/                        # initialized repo
├── .gitignore                   # tests/review/, logs/
├── .claude/
│   └── skills/                  # godark planning skills (via init)
├── CLAUDE.md                    # short control document (principles only)
├── godark.yaml                  # orchestrator config
├── docs/
│   ├── ROADMAP.md               # empty roadmap template
│   ├── architecture.md          # layer definitions (template)
│   ├── conventions.md           # coding conventions (template)
│   └── planning/                # (empty dir, planning docs go here)
├── tests/
│   └── scenarios/               # (empty dir, scenario specs go here)
└── prompts/
    ├── implementer.txt          # default prompt templates
    ├── implementer_retry.txt
    └── reviewer.txt
```

## Harness files

### CLAUDE.md

A short control document (~50-100 lines scaffolded) with section headers
and brief guidance comments. Contains principles and rules only — no file
paths, no code examples, no style rules.

Scaffolded sections:

- **Project** — one-line description placeholder
- **Build and Test** — blank, to be filled per language
- **Architecture** — "architecture docs and coding conventions live in
  docs/" (conceptual pointer, no exact paths)
- **Principles** — pre-populated with universal rules:
  - Wrap errors with context
  - No global state
  - Dependencies flow downward (see architecture docs)
- **Protected Paths** — pre-populated: CLAUDE.md, tests/scenarios/
- **Git Workflow** — pre-populated with godark branch/commit conventions
- **Definition of Done** — pre-populated: tests pass, build succeeds,
  PR approved by reviewer

Follows the principle: "CLAUDE.md is the highest-leverage file in the
harness." Keep it short, keep it stable.

### docs/architecture.md

Template with sections for:

- **Layers** — commented-out example showing the table format (layer,
  directory, allowed imports)
- **Module Boundaries** — placeholder for additional constraints beyond
  layers

Includes an explanation of what layers mean and how agents should use them.
The planning skills prompt the user to fill this in during roadmap and
planning conversations.

### docs/conventions.md

Template with section headers for:

- **Error Handling** — how to handle and propagate errors
- **Logging** — what logging framework/approach to use
- **Testing** — test placement, naming, coverage expectations
- **Naming** — package/module/file naming patterns

Each section has a brief guidance comment. Language-agnostic — the human
fills in language-specific conventions during planning.

### docs/ROADMAP.md

Minimal template with the standard format header. Ready for
`/godark-create-milestone`.

### prompts/*.txt

Copied from godark's built-in defaults. These are the files that contain
specific paths — they reference `docs/architecture.md`,
`docs/conventions.md`, scenario spec paths, build/test commands, etc. via
template variables.

This keeps CLAUDE.md path-free. If a file gets renamed, the user updates
the prompt template or godark.yaml — not CLAUDE.md.

### godark.yaml

Same as current `godark init` output, with `repo` pre-populated if
`--repo` was provided.

## Behavior

1. If `<project-name>` directory already exists, abort with an error.
2. Create the directory and all harness files.
3. Run `git init` inside the new directory.
4. Run the equivalent of `godark init` (skills + config), with `repo`
   pre-populated if `--repo` was provided.
5. Print a summary of created files.
6. Print next steps:
   - Fill in the language-specific sections of CLAUDE.md
   - Run `/godark-create-milestone` to define phases
   - Use `godark vet` to validate before execution

## Flags

| Flag | Description |
|------|-------------|
| `--repo` | GitHub repository (owner/repo). Written into godark.yaml. |

## What it does NOT do

- No language-specific scaffolding (use `go mod init`, `npm init`, etc.)
- No GitHub repo creation (use `gh repo create`)
- No milestone creation (use `/godark-create-milestone`)
- No issue creation (use `/godark-create-issues`)
- No style rules in CLAUDE.md (use linters)
- No code examples in templates (they go stale)

## Relationship to `init`

- `godark new` = create a project from scratch with harness files
- `godark init` = retrofit harness support into an existing project

`new` calls `init` internally for skills and config, then adds the harness
files that `init` doesn't create (CLAUDE.md, architecture.md,
conventions.md, ROADMAP.md, prompts, directory structure).

`init` could be extended in the future to optionally scaffold CLAUDE.md
and docs/ for existing projects that want the full harness structure, but
that's a separate concern.
