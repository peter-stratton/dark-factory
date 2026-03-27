# Scenario: Planner agent function and prompt template

Relates to: Issue #693

## Setup
- `SandboxRunner` is stubbed to capture rendered prompts and return canned results
- A temporary `prompts/` directory contains a valid `planner.txt` template
- Test issue has number 42, title "Add widget support", and a multi-line body

## Cases

### Plan renders prompt with issue and recon brief
- GIVEN a test issue and a recon brief "Found 3 relevant files in lib/widgets/"
- WHEN `Plan()` is called with the issue and recon brief
- THEN the rendered prompt passed to the sandbox runner contains "Add widget support" and "Found 3 relevant files in lib/widgets/"

### Plan returns structured brief
- GIVEN the stubbed runner returns stdout containing "## Approach\n\nCreate a new widget class"
- WHEN `Plan()` completes
- THEN `Result.ResultText` contains "## Approach" and "Create a new widget class"

### Plan with empty recon brief
- GIVEN an empty string for `reconBrief`
- WHEN `Plan()` is called
- THEN the prompt renders without error and the runner is invoked

### Plan uses planner role
- GIVEN a stubbed runner that captures `RunOpts`
- WHEN `Plan()` is called
- THEN `RunOpts.Role` is `"planner"`

### LoadPrompts includes planner
- GIVEN a prompts directory containing `planner.txt` with content "Plan for {{.IssueTitle}}"
- WHEN `LoadPrompts()` is called with the config pointing to that directory
- THEN `Prompts.Planner` equals "Plan for {{.IssueTitle}}"

### Planner prompt in scaffold
- GIVEN a fresh project directory
- WHEN `godark init` is run
- THEN `prompts/planner.txt` exists in the project directory

### Build passes
- GIVEN all planner agent code is in place
- WHEN `go build ./...` is run
- THEN the command exits with code 0
