# Scenario: Auto-generate unique project names per issue

Relates to: Issue #562

## Setup
- Config with `docker_compose` block
- Project name resolution logic

## Cases

### Auto-generated name from issue number
Config has no `project_name` set. Processing issue 42.
- Generated project name equals `godark-42`

### Prefixed name from config
Config has `project_name: "myapp"`. Processing issue 42.
- Generated project name equals `myapp-42`

### Different issues get different names
Config has no `project_name`. Processing issues 42 and 43.
- Issue 42 gets `godark-42`
- Issue 43 gets `godark-43`

### Generated name is valid
Config has `project_name: "My App"`. Processing issue 42.
- Generated name contains only lowercase letters, digits, and hyphens
