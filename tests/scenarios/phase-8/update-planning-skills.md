# Scenario: Update planning skills and embed new skills

Relates to: Issue #129

## Setup
- The skill files are at their existing paths under `internal/skills/`
- The embed directive is in `internal/skills/embed.go`
- Validation checks file content for expected references
- No external services required — this is a static content check

## Cases

### Roadmap skill reads architecture context
Read `internal/skills/godark-create-roadmap/SKILL.md`.
- Step 1 (gather context) references reading `docs/architecture.json`
- Step 1 references reading `docs/conventions.md`

### Roadmap skill suggests define-architecture
Read `internal/skills/godark-create-roadmap/SKILL.md`.
- The content mentions `/godark-define-architecture` as a suggestion when
  architecture docs are missing or empty

### Planning doc skill reads architecture context
Read `internal/skills/godark-create-planning-doc/SKILL.md`.
- Step 3 (read project context) references `docs/architecture.json`
- Step 3 references `docs/conventions.md`

### Planning doc skill prompts for architecture updates
Read `internal/skills/godark-create-planning-doc/SKILL.md`.
- Step 4 (discuss each issue) mentions prompting the user to update
  `docs/architecture.json` when new packages don't fit current layers

### Embed directive includes new skills
Read `internal/skills/embed.go`.
- The `//go:embed` directive includes `godark-define-architecture`
- The `//go:embed` directive includes `godark-define-conventions`
