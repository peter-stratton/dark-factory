# Scenario: /godark-define-architecture skill

Relates to: Issue #122

## Setup
- The skill file is at
  `internal/skills/godark-define-architecture/SKILL.md`
- Validation checks the file's YAML frontmatter and step content
- No external services required — this is a static content check

## Cases

### Skill file has valid frontmatter
Read the SKILL.md file and parse the YAML frontmatter.
- The frontmatter contains a `name` field
- The frontmatter contains a `description` field
- The frontmatter contains an `argument-hint` field
- The frontmatter contains `disable-model-invocation: true`

### Steps describe codebase scanning for existing projects
Read the SKILL.md body content.
- The steps include scanning for package or module directories
- The steps include identifying import relationships

### Steps describe recommendations for new projects
Read the SKILL.md body content.
- The steps include asking about language and framework
- The steps include proposing idiomatic layers

### Steps include validation
Read the SKILL.md body content.
- The steps include running `godark vet architecture`

### Steps suggest milestone for discrepancies
Read the SKILL.md body content.
- The steps include suggesting `/godark-create-milestone` when discrepancies
  exist between defined layers and the actual codebase

### Skill writes both output files
Read the SKILL.md body content.
- The steps include writing `docs/architecture.json`
- The steps include writing or updating `docs/architecture.md`
