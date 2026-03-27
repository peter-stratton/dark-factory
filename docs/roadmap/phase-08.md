## Phase 8: Harness Engineering ✅

**Goal**: Any project — new or existing — can adopt structured harness files
that make agents dramatically more effective. `godark new` creates a
harness-ready project from scratch. `godark init` scaffolds harness docs
into existing projects. Agents communicate reasoning via structured PR
comments. Architecture constraints are validated before execution.

**Milestone**: `Phase 8` | **Label**: `phase-8`

- Harness document templates package (`internal/harness/`) with embedded
  Go templates for all harness docs (architecture.md, conventions.md,
  ROADMAP.md, CLAUDE.md)
- Expand `godark init` to scaffold harness docs (architecture.md,
  conventions.md, ROADMAP.md, default prompt templates) using skip-if-exists
  pattern; does not scaffold CLAUDE.md; prints guidance message
- `godark new` command — creates project directory, runs `git init`,
  scaffolds CLAUDE.md template, then runs `godark init` internally;
  `--repo` flag; errors if directory exists
- Architecture layer parser (`internal/harness/layers/`) — parses layer
  definitions from markdown tables and optional JSON, returns directed
  dependency graph
- `godark vet architecture` subcommand — cycle detection, directory mapping
  warnings, import smell detection, layer skip warnings; skips gracefully
  if no architecture doc exists
- Agent dialogue and architecture reference prompt templates — update
  implementer, implementer_retry, and reviewer prompts with structured PR
  comment instructions (Implementation Notes / Review Notes) and
  architecture/conventions doc references via template variables
- `/godark-define-architecture` skill — analyzes existing codebase or
  recommends idiomatic layers for new projects; suggests
  `/godark-create-milestone` when discrepancies found between definition
  and codebase
- `/godark-define-conventions` skill — analyzes existing codebase or
  recommends idiomatic conventions with agent-friendliness filter; suggests
  `/godark-create-milestone` for standardization phases
- Update planning skills and embed new skills — update
  `/godark-create-milestone` and `/godark-create-planning-doc` to read
  architecture/conventions docs and prompt for updates; add new skills
  to `embed.go`

**Issues**: #121–#129

**Planning doc**: `docs/planning/phase-8-harness-engineering.md`

**Design docs**: `docs/design/harnesses.md`, `docs/design/godark-new.md`

