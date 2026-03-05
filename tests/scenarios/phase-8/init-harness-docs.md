# Scenario: Expand godark init to scaffold harness docs

Relates to: Issue #126

## Setup
- Tests run `godark init` (or call the init command's `RunE` directly) in a
  temporary directory
- No external services, Docker, or GitHub API required
- The temporary directory may be pre-populated with files to test
  skip-if-exists behavior

## Cases

### Fresh init creates all harness files
Run `godark init` in an empty temporary directory.
- `docs/architecture.md` exists with non-empty content
- `docs/architecture.json` exists with non-empty content
- `docs/conventions.md` exists with non-empty content
- `docs/ROADMAP.md` exists with non-empty content
- `prompts/implementer.txt` exists with non-empty content
- `prompts/implementer_retry.txt` exists with non-empty content
- `prompts/reviewer.txt` exists with non-empty content
- `docs/planning/` directory exists
- `tests/scenarios/` directory exists
- `CLAUDE.md` does NOT exist (not written by default)

### Re-run does not overwrite harness files
Run `godark init` in a directory, modify `docs/architecture.md`, then run
`godark init` again.
- `docs/architecture.md` retains the modified content
- Output contains "skipped" messages for existing files

### Partial state fills in missing files
Create only `docs/architecture.md` and `docs/conventions.md` in the temp
directory. Run `godark init`.
- `docs/architecture.json` is created (was missing)
- `docs/ROADMAP.md` is created (was missing)
- `docs/architecture.md` is not overwritten
- `docs/conventions.md` is not overwritten

### Reset flag writes CLAUDE.md
Run `godark init --reset-claude-md` in an empty directory.
- `CLAUDE.md` is created with template content
- Output contains "wrote" message for CLAUDE.md

### Reset flag replaces existing CLAUDE.md
Create a `CLAUDE.md` with custom content. Run `godark init --reset-claude-md`.
- `CLAUDE.md` is overwritten with template content
- Output contains a warning about replacing existing CLAUDE.md

### Guidance printed without reset flag
Run `godark init` in a directory that already has a `CLAUDE.md`.
- Output contains the hint about reviewing CLAUDE.md
- Output mentions `--reset-claude-md`

### Skills still overwritten
Create a skill file in `.claude/skills/`. Run `godark init`.
- The skill file is overwritten with the latest version (existing behavior)

### Config still skipped if exists
Create a `godark.yaml` with custom content. Run `godark init`.
- `godark.yaml` retains the custom content (existing behavior)
