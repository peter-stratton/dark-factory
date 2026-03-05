# Scenario: godark new command

Relates to: Issue #127

## Setup
- Tests run `godark new` (or call the new command's `RunE` directly) targeting
  a subdirectory of a temporary directory
- No external services or GitHub API required
- Git must be available on the test system

## Cases

### Creates project with all harness files
Run `godark new testproject` from a temporary directory.
- `testproject/` directory is created
- `testproject/CLAUDE.md` exists with template section headers
- `testproject/.gitignore` exists with expected entries
- `testproject/docs/architecture.md` exists
- `testproject/docs/architecture.json` exists
- `testproject/docs/conventions.md` exists
- `testproject/docs/ROADMAP.md` exists
- `testproject/prompts/implementer.txt` exists
- `testproject/godark.yaml` exists

### CLAUDE.md has expected section headers
Run `godark new testproject`. Read `testproject/CLAUDE.md`.
- Contains "Project" section header
- Contains "Build and Test" section header
- Contains "Architecture" section header
- Contains "Principles" section header
- Contains "Protected Paths" section header
- Contains "Git Workflow" section header
- Contains "Definition of Done" section header

### Git initialized in new directory
Run `godark new testproject`.
- `testproject/.git/` directory exists
- Running `git -C testproject status` succeeds

### Repo flag populates config
Run `godark new testproject --repo owner/repo`.
- `testproject/godark.yaml` contains `repo: owner/repo`

### Existing directory causes error
Create `testproject/` directory. Run `godark new testproject`.
- The command returns an error
- The error message mentions the directory already exists
- No files are written inside the existing directory

### No argument returns usage error
Run `godark new` with no arguments.
- The command returns an error
- The error message indicates a project name is required
