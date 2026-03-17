# Scenario: Include resource summary in report sprint output

Relates to: Issue #547

## Setup
- Stats database with runs containing resource data in the report window
- `internal/report/` package with report generation
- `godark report` command with `--format` flag

## Cases

### Terminal format includes resource section
Run `godark report --since 2w` against runs with resource data.
- Output includes a resource usage section
- Shows peak memory high-water mark (single highest step value)
- Shows total CPU time
- Shows average CPU per issue

### Markdown format includes resource section
Run `godark report --since 2w --format markdown`.
- Markdown output includes a resource usage section
- Values formatted consistently with terminal output

### HTML format includes resource section
Run `godark report --since 2w --format html`.
- HTML output includes a resource usage section

### Section omitted for old runs
Run `godark report` where the report window contains only pre-feature runs.
- No resource usage section in output
- Report otherwise renders normally

### Peak memory shows highest single step
Run `godark report` where implement steps peak at 400MB and review steps peak at 200MB.
- Peak memory shows 400MB (not an average, not a sum)
