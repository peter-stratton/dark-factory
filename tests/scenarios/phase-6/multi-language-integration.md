# Scenario: Multi-language integration and cleanup

Relates to: Issue #54

## Setup
- The full config, detect, and sandbox packages are imported
- Temporary directories simulate target repos with various marker files
- `Config` structs are constructed with and without explicit `Runtime` values
- No external services or Docker daemon required

## Cases

### Auto-detection runs when runtime is not configured
Create a config with no `Runtime` set and a temp repo directory containing `go.mod`.
- After the detection step, the config's runtime name equals `"go"`
- The detected version is extracted from the `go.mod` file

### Auto-detection populates build and test commands
Create a config with no `Runtime`, `BuildCommand`, or `TestCommand` set.
Place a `pubspec.yaml` in the temp repo directory.
- After detection, `TestCommand` equals `"flutter test"`
- After detection, `Runtime.Name` equals `"flutter"`

### Explicit config overrides detection
Create a config with `Runtime{Name: "node"}` and a temp repo containing `go.mod`.
- After the detection step, the runtime name remains `"node"` (not `"go"`)

### Explicit commands are not overwritten
Create a config with `TestCommand: "make test"` and a temp repo containing `go.mod`.
- After detection, `TestCommand` remains `"make test"` (not `"go test ./..."`)

### Detection failure logs warning without error
Create a config with no `Runtime` and a temp repo with no marker files.
- No error is raised
- A warning is logged (verifiable via log capture or structured log output)
- The Dockerfile generation proceeds without a toolchain install section

### No references to GoVersion remain
Search all `.go` files in `internal/` for the string `GoVersion`.
- No matches are found

### No references to CrossCompile remain
Search all `.go` files in `internal/` for the string `CrossCompile`.
- No matches are found
