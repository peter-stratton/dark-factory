# Scenario: Update create-scenarios skill and spec_generator prompt for GIVEN/WHEN/THEN

Relates to: Issue #682

## Setup
- `prompts/spec_generator.txt` currently shows plain `- Expected outcome`
  bullets in the format example
- `internal/skills/godark-create-scenarios/SKILL.md` defines the scenario
  spec format for the skill
- `.claude/skills/` contains installed copies of skills that must stay in sync

## Cases

### Prompt format uses GIVEN/WHEN/THEN
Read `prompts/spec_generator.txt`.
- The format example block contains `- GIVEN`
- The format example block contains `- WHEN`
- The format example block contains `- THEN`
- Plain `- Expected outcome` bullets no longer appear in the format example

### Skill format uses GIVEN/WHEN/THEN
Read `internal/skills/godark-create-scenarios/SKILL.md`.
- Format examples or instructions contain `- GIVEN`
- Format examples or instructions contain `- WHEN`
- Format examples or instructions contain `- THEN`

### Skill copies in sync
Run `diff` between `internal/skills/godark-create-scenarios/SKILL.md` and
`.claude/skills/godark-create-scenarios/SKILL.md`.
- No differences

### No Go files modified
Run `git diff --name-only` after the change.
- No `.go` files appear in the diff
