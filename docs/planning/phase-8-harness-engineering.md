# Phase 8: Harness Engineering

> **Goal:** Any project — new or existing — can adopt structured harness files
> that make agents dramatically more effective. `godark new` creates a
> harness-ready project from scratch. `godark init` scaffolds harness docs
> into existing projects. Agents communicate reasoning via structured PR
> comments. Architecture constraints are validated before execution.

## Milestone

`Phase 8`

---

## Issue: Harness document templates package

### Description

Create a new `internal/harness/templates/` package with embedded static files
for all harness documents that `godark init` and `godark new` scaffold into
target projects. Includes a helper function for writing files with
skip-if-exists semantics.

This is pure new code — no existing files are modified.

### Key constraints

- New package: `internal/harness/templates/`
- Embedded static files via `//go:embed`:
  - `architecture.md` — prose template explaining layers, referencing
    `architecture.json` for definitions, with guidance comments
  - `architecture.json` — example layer definitions (commented structure
    or minimal valid JSON with a single example layer)
  - `conventions.md` — section headers for error handling, logging,
    testing, naming, with guidance comments per section
  - `roadmap.md` — minimal roadmap template with format header
  - `claude.md` — short control document (~100 lines) with section headers:
    Project, Build and Test, Architecture (conceptual pointer to docs/),
    Principles, Protected Paths, Git Workflow, Definition of Done. No file
    paths, no code examples, no style rules. Guidance comments in each
    section.
  - `gitignore` — includes `tests/review/` and `logs/`
  - `prompts/implementer.txt` — default implementer prompt with
    implementation notes PR comment instructions and architecture doc
    reference via template variable
  - `prompts/implementer_retry.txt` — default retry prompt with
    instructions to read prior implementation notes and reviewer challenges
    from PR comment thread
  - `prompts/reviewer.txt` — default reviewer prompt with review notes
    PR comment instructions and architecture compliance check instruction
- Exported helper:
  ```go
  // WriteIfNotExists writes the embedded file at srcPath to destPath.
  // Creates parent directories as needed. Returns true if the file was
  // written, false if it already existed. Returns an error if the write
  // fails.
  func WriteIfNotExists(srcPath, destPath string) (written bool, err error)
  ```
- Exported `embed.FS` for cases where callers need direct access to
  embedded content (e.g., `godark new` writing CLAUDE.md which `init`
  skips)

### Acceptance criteria

- [ ] All template files are embedded and accessible via the package
- [ ] `WriteIfNotExists` creates parent directories
- [ ] `WriteIfNotExists` writes the file if it does not exist
- [ ] `WriteIfNotExists` skips and returns false if the file exists
- [ ] Prompt templates include implementation notes and review notes format

### Test cases

- **File written**: `WriteIfNotExists` to a non-existent path creates the file with correct content
- **File skipped**: `WriteIfNotExists` to an existing path returns false and does not overwrite
- **Parent dirs created**: `WriteIfNotExists` to `a/b/c/file.md` creates intermediate directories
- **All templates accessible**: Each embedded file can be read via the exported `embed.FS`
- **Prompt has dialogue instructions**: Implementer prompt contains "Implementation Notes" section format
- **Prompt has architecture reference**: Implementer prompt references architecture doc via template variable

---

## Issue: Expand godark init to scaffold harness docs

**Blocked by**: Harness document templates package

### Description

Update `init.go` to scaffold harness documentation files alongside the
existing skills and config. Uses the templates package `WriteIfNotExists`
helper so all files are safe to skip if they already exist. By default,
does NOT scaffold CLAUDE.md — that is too project-specific for existing
projects. The `--reset-claude-md` flag opts in to replacing an existing
CLAUDE.md with the harness template. Prints a guidance message suggesting
the user review their CLAUDE.md.

### Key constraints

- Modify `internal/cmd/init.go`
- Add a `writeHarnessDocs` function called from the init command's `RunE`
- New flag: `--reset-claude-md` — when set, writes the CLAUDE.md template
  even if one already exists. Without this flag, CLAUDE.md is never
  written by `init` (only by `godark new` for greenfield projects).
  When the flag is used and an existing CLAUDE.md is found, prints a
  warning: `"replacing existing CLAUDE.md with harness template"`
- Files scaffolded (all via `WriteIfNotExists`):
  - `docs/architecture.md`
  - `docs/architecture.json`
  - `docs/conventions.md`
  - `docs/ROADMAP.md`
  - `docs/planning/` (directory only)
  - `tests/scenarios/` (directory only)
  - `prompts/implementer.txt`
  - `prompts/implementer_retry.txt`
  - `prompts/reviewer.txt`
- Prints `"wrote <path>"` for each file written, `"skipped <path> (already
  exists)"` for each file skipped
- After all files are written, if CLAUDE.md was not written, prints
  guidance: `"hint: review your CLAUDE.md against the harness principles
  in docs/architecture.md (use --reset-claude-md to replace it)"`
- Existing behavior unchanged — skills are overwritten, config is skipped
  if exists

### Acceptance criteria

- [ ] `godark init` creates all harness doc files
- [ ] Existing files are not overwritten
- [ ] `docs/planning/` and `tests/scenarios/` directories are created
- [ ] `--reset-claude-md` replaces existing CLAUDE.md with template
- [ ] Without `--reset-claude-md`, CLAUDE.md is not written and guidance is printed

### Test cases

- **Fresh init**: Running `init` in an empty directory creates all harness files (no CLAUDE.md)
- **Re-run safe**: Running `init` twice does not overwrite any harness files
- **Partial state**: Running `init` with some files present writes missing files, skips existing ones
- **Reset flag writes CLAUDE.md**: `init --reset-claude-md` writes CLAUDE.md template
- **Reset flag replaces existing**: `init --reset-claude-md` with existing CLAUDE.md overwrites it and prints warning
- **Guidance without flag**: Output contains CLAUDE.md guidance hint when flag not used
- **Skills still overwritten**: Skill files are written even if they exist (existing behavior)
- **Config still skipped**: `godark.yaml` is skipped if it exists (existing behavior)

---

## Issue: godark new command

**Blocked by**: Expand godark init to scaffold harness docs

### Description

Add a `godark new <project-name>` command that creates a new directory with
all harness files for a greenfield project. Creates the directory, runs
`git init`, scaffolds CLAUDE.md and `.gitignore` (files that `init` does
not create), then runs the equivalent of `godark init` internally.

### Key constraints

- New file: `internal/cmd/new.go`
- `godark new <project-name>` — exactly one positional argument required
- `--repo` flag — optional, pre-populates `repo:` in `godark.yaml`
- Behavior:
  1. If `<project-name>` directory exists, return error
  2. Create the directory
  3. Write `CLAUDE.md` from templates package
  4. Write `.gitignore` from templates package
  5. Run `git init` in the new directory
  6. Run the init logic (skills, config, harness docs) in the new
     directory, with `--repo` passed through if provided
  7. Print summary of created files
  8. Print next-steps guidance: fill in CLAUDE.md sections, run
     `/godark-create-roadmap`
- The init logic must work when called from a different directory than
  the current working directory — either `chdir` before calling or pass
  the target directory as a parameter
- The `--repo` value is written into `godark.yaml` by modifying the
  default config template before writing

### Acceptance criteria

- [ ] `godark new myproject` creates the directory with all harness files
- [ ] CLAUDE.md and .gitignore are created (not created by `init`)
- [ ] `git init` is run in the new directory
- [ ] `--repo` flag populates `godark.yaml` repo field
- [ ] Existing directory causes an error

### Test cases

- **Creates project**: `godark new testproject` creates directory with all expected files
- **CLAUDE.md created**: The new directory contains CLAUDE.md with expected section headers
- **Git initialized**: The new directory contains a `.git/` directory
- **Repo flag**: `godark new testproject --repo owner/repo` produces `godark.yaml` with `repo: owner/repo`
- **Directory exists**: `godark new testproject` when `testproject/` exists returns an error
- **No argument**: `godark new` with no argument returns a usage error

---

## Issue: Architecture layer parser

**Blocked by**: Harness document templates package

### Description

Create a new `internal/harness/layers/` package that parses layer
definitions from `architecture.json` and returns a directed graph of
layer dependencies. JSON-only — no markdown table parsing.

### Key constraints

- New package: `internal/harness/layers/`
- JSON schema:
  ```json
  {
    "layers": [
      {"name": "types", "dir": "internal/types/", "imports": []},
      {"name": "config", "dir": "internal/config/", "imports": ["types"]}
    ]
  }
  ```
- Exported types:
  ```go
  type Layer struct {
      Name    string   `json:"name"`
      Dir     string   `json:"dir"`
      Imports []string `json:"imports"`
  }

  type Definition struct {
      Layers []Layer `json:"layers"`
  }
  ```
- `Parse(r io.Reader) (*Definition, error)` — decodes JSON, returns error
  on invalid JSON or empty layers
- `ParseFile(path string) (*Definition, error)` — opens file, calls `Parse`
- Validation on parse:
  - Layer names must be non-empty and unique
  - Imports must reference existing layer names
  - Dir must be non-empty
- Parse does NOT check for cycles — that is the vet package's
  responsibility

### Acceptance criteria

- [ ] `Parse` decodes valid JSON into `Definition`
- [ ] `ParseFile` reads from disk and parses
- [ ] Duplicate layer names return an error
- [ ] Import referencing non-existent layer returns an error
- [ ] Empty layer name or dir returns an error

### Test cases

- **Valid JSON**: Standard 5-layer definition parses correctly
- **Empty layers**: `{"layers": []}` returns an error
- **Duplicate names**: Two layers named "config" returns an error
- **Bad import reference**: Layer importing "nonexistent" returns an error
- **Empty name**: Layer with `"name": ""` returns an error
- **Empty dir**: Layer with `"dir": ""` returns an error
- **Invalid JSON**: Malformed input returns a parse error
- **No imports field**: Layer without `"imports"` key parses with empty imports slice

---

## Issue: godark vet architecture subcommand

**Blocked by**: Architecture layer parser

### Description

Add a `godark vet architecture` subcommand that validates the layer
definitions in `docs/architecture.json`. Uses the layer parser from
`internal/harness/layers/` and validates that the definition is a valid
DAG. Keeps the scope narrow — only checks properties that are universally
wrong (cycles). Project-specific structural concerns (directory existence,
import distance) are better handled by project-specific linting.

### Key constraints

- New file: `internal/vet/architecture.go`
  - `ValidateArchitecture(def *layers.Definition) vet.Report`
  - Checks:
    - **Cycle detection** (Error) — detect cycles in the dependency graph
      using topological sort or DFS. Report all layers involved in the
      cycle.
- New file: `internal/cmd/vet_architecture.go`
  - Subcommand of `vetCmd`
  - Reads `docs/architecture.json` (or path from `--architecture-file` flag)
  - Calls `layers.ParseFile`, then `vet.ValidateArchitecture`
  - Uses existing `printReport` helper
  - If the architecture file does not exist, prints info message
    and exits 0 (harnesses are opt-in)
- `--json` flag supported (already handled by `printReport`)

### Acceptance criteria

- [ ] `godark vet architecture` reports cycle errors
- [ ] Missing architecture file exits 0 with info message
- [ ] `--json` flag produces JSON output
- [ ] Valid DAG produces no findings

### Test cases

- **No cycles**: Valid DAG produces no errors
- **Simple cycle**: A→B→A produces error naming both layers
- **Transitive cycle**: A→B→C→A produces error naming all three layers
- **Self-import**: A imports A produces error
- **Clean architecture**: Well-structured 5-layer definition produces no findings
- **No architecture file**: Missing `docs/architecture.json` prints info and exits 0
- **JSON output**: `--json` flag produces valid JSON with findings array

---

## Issue: Agent dialogue and architecture reference prompt templates

**Blocked by**: Harness document templates package

### Description

Update the embedded prompt templates in `internal/harness/templates/prompts/`
to include structured PR comment instructions for agent dialogue and
references to architecture/conventions docs. Also update the existing
prompt templates in the dark-factory `prompts/` directory to match.

### Key constraints

- Modify embedded templates in `internal/harness/templates/prompts/`:
  - `implementer.txt`:
    - After opening the PR, post a PR comment with structured Implementation
      Notes (Approach, Key Decisions, Known Limitations, Architecture)
    - Add step: "Read `{{.ArchitectureDoc}}` for dependency layer rules"
      (with conditional — skip if variable is empty)
    - Add step: "Read `{{.ConventionsDoc}}` for coding conventions"
      (with conditional)
  - `implementer_retry.txt`:
    - Read previous implementation notes and reviewer challenges from
      the PR comment thread before fixing
    - Post updated implementation notes explaining what changed and why
  - `reviewer.txt`:
    - Read implementation notes from PR comments to understand reasoning
    - After deciding verdict, post structured Review Notes (Approved,
      Changes Requested, Architecture Compliance)
    - Add step: "Check that imports respect the dependency layers in
      `{{.ArchitectureDoc}}`" (with conditional)
- Update dark-factory's own `prompts/` directory to match the new
  embedded defaults (these are dark-factory's prompts for its own runs)
- New template variables needed: `{{.ArchitectureDoc}}`,
  `{{.ConventionsDoc}}` — these will be wired in a future issue when
  the launcher is updated. For now, the templates reference them but
  callers may leave them empty (conditionals handle this)

### Acceptance criteria

- [ ] Implementer prompt includes implementation notes comment instructions
- [ ] Implementer retry prompt reads prior notes from PR comment thread
- [ ] Reviewer prompt includes review notes comment instructions
- [ ] All prompts reference architecture and conventions docs conditionally
- [ ] Dark-factory's own `prompts/` updated to match

### Test cases

- **Implementer has dialogue**: Embedded implementer.txt contains "Implementation Notes" format
- **Retry reads thread**: Embedded implementer_retry.txt contains instruction to read PR comments
- **Reviewer has dialogue**: Embedded reviewer.txt contains "Review Notes" format
- **Architecture conditional**: Templates use `{{if .ArchitectureDoc}}` guard
- **Dark-factory prompts match**: `prompts/implementer.txt` matches embedded template content

---

## Issue: /godark-define-architecture skill

### Description

Create a new Claude Code skill that helps users define or update the
architecture layer definitions for their project. For existing projects,
it scans the codebase to identify packages and import relationships and
proposes layers. For new projects, it asks about the language and framework
and proposes idiomatic layers. Writes `docs/architecture.json` and
`docs/architecture.md`.

### Key constraints

- New file: `internal/skills/godark-define-architecture/SKILL.md`
- Skill steps:
  1. Read `godark.yaml` for repo and runtime info
  2. Read existing `docs/architecture.json` and `docs/architecture.md`
     if they exist
  3. For existing projects with code: scan for package/module directories,
     identify import relationships, propose layers based on what exists
  4. For new/empty projects: ask about language, framework, project type,
     propose idiomatic layers
  5. Discuss with user — ask what to keep vs change, identify
     inconsistencies (circular imports, unclear boundaries)
  6. Write `docs/architecture.json` with the agreed layer definitions
  7. Update `docs/architecture.md` prose to describe the layers
  8. Run `godark vet architecture` to validate the result
  9. If discrepancies exist between defined layers and actual codebase,
     suggest running `/godark-create-roadmap` to plan a codebase alignment
     phase
- Re-runnable — reads existing definitions and proposes updates rather
  than overwriting
- `disable-model-invocation: true` (interactive skill, like the other
  planning skills)

### Acceptance criteria

- [ ] Skill file exists with correct YAML frontmatter
- [ ] Steps include codebase scanning for existing projects
- [ ] Steps include idiomatic recommendations for new projects
- [ ] Writes both `architecture.json` and `architecture.md`
- [ ] Suggests `/godark-create-roadmap` for discrepancies

### Test cases

- **Skill format**: SKILL.md has valid frontmatter with name, description, argument-hint
- **Existing project flow**: Steps describe scanning packages and proposing layers
- **New project flow**: Steps describe asking about language/framework
- **Validation step**: Steps include running `godark vet architecture`
- **Discrepancy guidance**: Steps include suggesting roadmap for alignment

---

## Issue: /godark-define-conventions skill

### Description

Create a new Claude Code skill that helps users define or update coding
conventions for their project. For existing projects, it reads source files
to identify patterns in use and proposes conventions to standardize on. For
new projects, it recommends idiomatic conventions filtered for
agent-friendliness. Writes `docs/conventions.md`.

### Key constraints

- New file: `internal/skills/godark-define-conventions/SKILL.md`
- Skill steps:
  1. Read `godark.yaml` for runtime info
  2. Read existing `docs/conventions.md` if it exists
  3. Read `CLAUDE.md` for any existing convention references
  4. For existing projects: read a sample of source files, identify
     patterns (error handling, logging, test style, naming, dependency
     injection approach)
  5. For new/empty projects: ask about language and framework, propose
     idiomatic conventions
  6. Apply agent-friendliness filter — recommend patterns that work well
     with agents:
     - Explicit over implicit (named dependencies, not DI containers)
     - Local over global (function signatures, not shared mutable state)
     - Clear boundaries (interfaces between layers, not deep inheritance)
     - Discoverable (patterns visible in a few files, not requiring
       whole-system understanding)
  7. Flag conventions that impede agentic development (heavy code
     generation, convention-over-configuration magic, implicit behavior)
  8. Discuss with user — ask what to standardize on
  9. Write `docs/conventions.md` with agreed conventions
  10. If codebase uses inconsistent conventions, suggest running
      `/godark-create-roadmap` to plan a standardization phase
- Re-runnable — reads existing conventions and proposes updates
- `disable-model-invocation: true`

### Acceptance criteria

- [ ] Skill file exists with correct YAML frontmatter
- [ ] Steps include source file analysis for existing projects
- [ ] Steps include agent-friendliness filter recommendations
- [ ] Steps flag conventions that impede agentic development
- [ ] Suggests `/godark-create-roadmap` for inconsistencies

### Test cases

- **Skill format**: SKILL.md has valid frontmatter with name, description, argument-hint
- **Existing project flow**: Steps describe reading source files and identifying patterns
- **Agent-friendliness**: Steps list explicit > implicit, local > global principles
- **Anti-patterns flagged**: Steps mention code generation and convention-over-configuration
- **Standardization guidance**: Steps include suggesting roadmap for inconsistent conventions

---

## Issue: Update planning skills and embed new skills

**Blocked by**: /godark-define-architecture skill, /godark-define-conventions skill

### Description

Update the existing planning skills to read architecture and conventions
docs for context, and prompt users to update them when relevant. Add the
two new skills to the embed directive in `embed.go`.

### Key constraints

- Modify `internal/skills/godark-create-roadmap/SKILL.md`:
  - In step 1 (gather context), add: read `docs/architecture.json` and
    `docs/conventions.md` if they exist
  - In step 2 (discuss goals), add: ask about architecture layers when
    the conversation reveals structural decisions
  - Add guidance: if `docs/architecture.md` is empty or missing, suggest
    running `/godark-define-architecture`
- Modify `internal/skills/godark-create-planning-doc/SKILL.md`:
  - In step 3 (read project context), add: read `docs/architecture.json`
    and `docs/conventions.md`
  - In step 4 (discuss each issue), add: when new phases introduce
    packages that don't fit current layers, prompt user to update
    `docs/architecture.json`
- Modify `internal/skills/embed.go`:
  - Add `godark-define-architecture` and `godark-define-conventions` to
    the `//go:embed` directive

### Acceptance criteria

- [ ] `/godark-create-roadmap` reads architecture and conventions docs
- [ ] `/godark-create-roadmap` suggests `/godark-define-architecture` when needed
- [ ] `/godark-create-planning-doc` reads architecture and conventions docs
- [ ] `/godark-create-planning-doc` prompts for architecture updates
- [ ] New skills are included in the embed directive

### Test cases

- **Roadmap reads context**: SKILL.md step 1 references `architecture.json` and `conventions.md`
- **Roadmap suggests skill**: SKILL.md mentions `/godark-define-architecture` when architecture is missing
- **Planning reads context**: SKILL.md step 3 references `architecture.json` and `conventions.md`
- **Planning prompts update**: SKILL.md step 4 mentions updating architecture when layers don't fit
- **Embed includes new skills**: `embed.go` directive includes both new skill directories
