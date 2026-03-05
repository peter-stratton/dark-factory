# Scenario: Harness document templates package

Relates to: Issue #121

## Setup
- The `internal/harness/templates` package is tested directly via Go unit tests
- All file-write tests use a temporary directory so nothing is written to the
  real filesystem
- No external services, Docker, or GitHub API required
- The embedded `embed.FS` is accessed directly for template content verification

## Cases

### File written to non-existent path
Call `WriteIfNotExists("architecture.md", "<tmpdir>/docs/architecture.md")`.
- The file is created at the destination path
- The file content matches the embedded `architecture.md` template
- The function returns `(true, nil)`

### File skipped when it already exists
Create a file at `<tmpdir>/docs/conventions.md` with custom content. Call
`WriteIfNotExists("conventions.md", "<tmpdir>/docs/conventions.md")`.
- The function returns `(false, nil)`
- The file content is unchanged (still the custom content)

### Parent directories created
Call `WriteIfNotExists("roadmap.md", "<tmpdir>/a/b/c/roadmap.md")` where none
of the intermediate directories exist.
- The file is created at the full path
- All intermediate directories (`a/`, `a/b/`, `a/b/c/`) exist

### All template files accessible via embed.FS
Read each of the following files from the exported `embed.FS`:
`architecture.md`, `architecture.json`, `conventions.md`, `roadmap.md`,
`claude.md`, `gitignore`, `prompts/implementer.txt`,
`prompts/implementer_retry.txt`, `prompts/reviewer.txt`.
- Each file is non-empty
- No error is returned for any file

### Implementer prompt has dialogue instructions
Read `prompts/implementer.txt` from the exported `embed.FS`.
- The content contains "Implementation Notes" section format
- The content contains subsections for Approach, Key Decisions, Known
  Limitations, and Architecture

### Implementer prompt has architecture reference
Read `prompts/implementer.txt` from the exported `embed.FS`.
- The content contains a reference to `{{.ArchitectureDoc}}`
- The reference is guarded by a conditional (e.g., `{{if .ArchitectureDoc}}`)

### Reviewer prompt has review notes format
Read `prompts/reviewer.txt` from the exported `embed.FS`.
- The content contains "Review Notes" section format
- The content contains subsections for Approved/Changes Requested and
  Architecture Compliance

### CLAUDE.md template has expected sections
Read `claude.md` from the exported `embed.FS`.
- The content contains section headers: Project, Build and Test, Architecture,
  Principles, Protected Paths, Git Workflow, Definition of Done
- The content does not contain file paths or code examples
- The content is under 150 lines

### Gitignore template has expected entries
Read `gitignore` from the exported `embed.FS`.
- The content contains `tests/review/`
- The content contains `logs/`
