# Scenario: Config fields for modules block

Relates to: Issue #217

## Setup
- The `internal/config/` package is tested via Go unit tests
- YAML config files with various `modules:` configurations

## Cases

### Config defaults
Parse a minimal `godark.yaml` with only `repo:` set.
- `Config.Modules` is nil

### Single module mode preserved
Parse a `godark.yaml` with no `modules:` key, but with root-level `build_command` and `test_command`.
- `Config.Modules` is nil
- Root-level `BuildCommand` and `TestCommand` are set as before

### Two modules parsed
Parse a `godark.yaml` with:
```yaml
modules:
  service:
    build_command: "go build ./..."
    test_command: "go test ./..."
  admin-cli:
    build_command: "go build ./..."
    depends_on: [service]
```
- `Config.Modules` has two entries: `"service"` and `"admin-cli"`
- `service` module has `BuildCommand` and `TestCommand` set
- `admin-cli` module has `DependsOn` containing `"service"`

### Per-module commands independent
Parse a config where two modules have different `lint_command` values.
- Each module's `LintCommand` is its own value, not shared

### Unknown dependency rejected
Parse a `godark.yaml` where a module has `depends_on: [nonexistent]`.
- `Load()` returns a validation error
- Error message mentions the unknown module name

### Cycle detection
Parse a `godark.yaml` where module `a` depends on `b` and module `b` depends on `a`.
- `Load()` returns a validation error
- Error message indicates a dependency cycle

### Self-dependency rejected
Parse a `godark.yaml` where module `foo` has `depends_on: [foo]`.
- `Load()` returns a validation error
