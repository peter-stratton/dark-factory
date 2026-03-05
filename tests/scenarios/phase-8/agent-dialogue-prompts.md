# Scenario: Agent dialogue and architecture reference prompt templates

Relates to: Issue #125

## Setup
- The embedded prompt templates in `internal/harness/templates/prompts/` are
  read directly from the exported `embed.FS`
- Dark-factory's own `prompts/` directory files are read from disk
- No external services, Docker, or GitHub API required
- Template variables are checked as literal strings (not rendered)

## Cases

### Implementer prompt has implementation notes format
Read `prompts/implementer.txt` from the embedded templates.
- The content contains instructions to post a PR comment
- The content contains "Implementation Notes" as a section heading
- The content contains subsections: Approach, Key Decisions, Known Limitations,
  Architecture

### Implementer prompt references architecture doc conditionally
Read `prompts/implementer.txt` from the embedded templates.
- The content contains `{{.ArchitectureDoc}}`
- The content contains `{{if .ArchitectureDoc}}` (or equivalent conditional)
- The content contains `{{.ConventionsDoc}}`
- The content contains `{{if .ConventionsDoc}}` (or equivalent conditional)

### Retry prompt reads prior PR comments
Read `prompts/implementer_retry.txt` from the embedded templates.
- The content contains an instruction to read previous implementation notes
  from the PR comment thread
- The content contains an instruction to read reviewer challenges or feedback
  from the PR comment thread

### Retry prompt posts updated notes
Read `prompts/implementer_retry.txt` from the embedded templates.
- The content contains instructions to post updated implementation notes
- The content contains instructions to explain what changed and why

### Reviewer prompt has review notes format
Read `prompts/reviewer.txt` from the embedded templates.
- The content contains instructions to read implementation notes from PR
  comments
- The content contains "Review Notes" as a section heading
- The content contains subsections for verdict (Approved / Changes Requested)
  and Architecture Compliance

### Reviewer prompt checks architecture compliance
Read `prompts/reviewer.txt` from the embedded templates.
- The content contains an instruction to check imports against dependency layers
- The reference to `{{.ArchitectureDoc}}` is guarded by a conditional

### Dark-factory prompts match embedded templates
Read `prompts/implementer.txt` from the dark-factory project root and compare
to the embedded template.
- The content matches the embedded `implementer.txt`
- Repeat for `implementer_retry.txt` and `reviewer.txt`
