# Scenario: /godark-configure-project skill

Relates to: Issue #225

## Setup
- The `internal/skills/godark-configure-project/SKILL.md` skill file
- The `internal/skills/embed.go` embed directive
- The `internal/cmd/init.go` skill installation logic

## Cases

### Skill file installed by godark init
Run `godark init` in an empty directory.
- `.claude/skills/godark-configure-project/SKILL.md` exists
- File contains `name: godark-configure-project` in frontmatter

### Skill embedded in binary
The `internal/skills/embed.go` file includes `godark-configure-project` in the embed directive.
- `SkillFiles` embed.FS can read the skill file

### Init idempotent
Run `godark init` twice in the same directory.
- Skill file exists after both runs
- No errors on second run
- File content is unchanged

### Skill detects multi-module project
The skill describes detection of multiple `go.mod`/`pubspec.yaml`/`package.json` files.
- SKILL.md mentions detecting multiple module manifests
- SKILL.md describes suggesting `modules:` config block

### Skill detects codegen configs
The skill describes detection of codegen configuration files.
- SKILL.md mentions `sqlc.yml`, `gqlgen.yml`, `.mockery.yaml`, `build.yaml`
- SKILL.md describes suggesting `generate_command` and `generated_paths`

### Skill detects CI workflows
The skill describes detection of GitHub Actions workflow files.
- SKILL.md mentions `.github/workflows/*.yml`
- SKILL.md describes suggesting `wait_for_checks` with extracted check names

### Skill merges into existing config
The skill describes merging behavior for existing `godark.yaml` files.
- SKILL.md states it does not overwrite fields already set
- SKILL.md describes interactive confirmation before writing
