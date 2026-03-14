# Scenario: /godark-define-conventions skill

Relates to: Issue #123

## Setup
- The skill file is at
  `internal/skills/godark-define-conventions/SKILL.md`
- Validation checks the file's YAML frontmatter and step content
- No external services required — this is a static content check

## Cases

### Skill file has valid frontmatter
Read the SKILL.md file and parse the YAML frontmatter.
- The frontmatter contains a `name` field
- The frontmatter contains a `description` field
- The frontmatter contains an `argument-hint` field
- The frontmatter contains `disable-model-invocation: true`

### Steps describe source file analysis for existing projects
Read the SKILL.md body content.
- The steps include reading a sample of source files
- The steps include identifying patterns (error handling, logging, test style,
  naming)

### Steps include agent-friendliness filter
Read the SKILL.md body content.
- The steps include "explicit over implicit" principle
- The steps include "local over global" principle
- The steps include "clear boundaries" principle
- The steps include "discoverable" principle

### Steps flag anti-patterns for agentic development
Read the SKILL.md body content.
- The steps mention heavy code generation as an impediment
- The steps mention convention-over-configuration magic as an impediment

### Steps suggest milestone for inconsistencies
Read the SKILL.md body content.
- The steps include suggesting `/godark-create-milestone` when the codebase uses
  inconsistent conventions

### Skill writes conventions doc
Read the SKILL.md body content.
- The steps include writing `docs/conventions.md`
