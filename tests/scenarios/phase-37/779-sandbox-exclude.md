# Scenario: Sandbox directory exclusion

Relates to: Issue #779

## Setup
- A godark.yaml with a `docker` block
- A target repo containing an `eval/` directory and a `fixtures/` directory

## Cases

### Single directory excluded from sandbox
- GIVEN a godark.yaml with `docker.exclude: ["eval/"]`
- WHEN the sandbox clone script is generated
- THEN the script contains a `rm -rf` command for the `eval/` directory after the clone step

### Multiple directories excluded
- GIVEN a godark.yaml with `docker.exclude: ["eval/", "fixtures/"]`
- WHEN the sandbox clone script is generated
- THEN the script contains `rm -rf` commands for both `eval/` and `fixtures/`

### No exclude key preserves current behavior
- GIVEN a godark.yaml with no `exclude` key under `docker`
- WHEN the sandbox clone script is generated
- THEN the script is identical to the current behavior (full clone, no removals)

### Empty exclude list preserves current behavior
- GIVEN a godark.yaml with `docker.exclude: []`
- WHEN the sandbox clone script is generated
- THEN the script is identical to the current behavior (full clone, no removals)

### Path traversal rejected
- GIVEN a godark.yaml with `docker.exclude: ["../secret"]`
- WHEN the config is validated
- THEN a validation error is returned indicating path traversal is not allowed

### Absolute path rejected
- GIVEN a godark.yaml with `docker.exclude: ["/etc/passwd"]`
- WHEN the config is validated
- THEN a validation error is returned indicating absolute paths are not allowed

### Excluded directory not visible to agent
- GIVEN a godark.yaml with `docker.exclude: ["eval/"]` and a repo with an `eval/` directory
- WHEN an agent runs inside the sandbox
- THEN the `eval/` directory does not exist in the workspace
