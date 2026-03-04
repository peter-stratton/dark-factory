# Scenario: Auto-detect project type

Relates to: Issue #52

## Setup
- The detect package (`internal/detect`) is imported directly
- Temporary directories are created for each case, containing the relevant
  marker files (e.g., `go.mod`, `pubspec.yaml`, `package.json`)
- No external services, network access, or real project dependencies required
- Marker files contain minimal valid content sufficient for version parsing

## Cases

### Detect Go project via go.mod
Create a temp directory with a `go.mod` containing `go 1.26`.
- `DetectedProject.Runtime.Name` equals `"go"`
- `DetectedProject.Runtime.Version` equals `"1.26"`
- `DetectedProject.BuildCommand` equals `"go build ./..."`
- `DetectedProject.TestCommand` equals `"go test ./..."`

### Detect Flutter project via pubspec.yaml
Create a temp directory with a `pubspec.yaml`.
- `DetectedProject.Runtime.Name` equals `"flutter"`
- `DetectedProject.TestCommand` equals `"flutter test"`

### Detect Node project via package.json
Create a temp directory with a `package.json`.
- `DetectedProject.Runtime.Name` equals `"node"`
- `DetectedProject.BuildCommand` equals `"npm run build"`
- `DetectedProject.TestCommand` equals `"npm test"`

### Detect Rust project via Cargo.toml
Create a temp directory with a `Cargo.toml`.
- `DetectedProject.Runtime.Name` equals `"rust"`
- `DetectedProject.BuildCommand` equals `"cargo build"`
- `DetectedProject.TestCommand` equals `"cargo test"`

### Detect Python project via pyproject.toml
Create a temp directory with a `pyproject.toml`.
- `DetectedProject.Runtime.Name` equals `"python"`
- `DetectedProject.TestCommand` equals `"pytest"`

### Python fallback to requirements.txt
Create a temp directory with only a `requirements.txt` (no `pyproject.toml`).
- `DetectedProject.Runtime.Name` equals `"python"`

### No marker files returns error
Create an empty temp directory.
- `DetectRuntime` returns an error
- The error message contains `"could not detect project type"`

### Multiple markers use priority order
Create a temp directory with both `go.mod` and `package.json`.
- `DetectedProject.Runtime.Name` equals `"go"` (Go wins by priority)

### Go version extracted from go.mod
Create a temp directory with a `go.mod` containing `go 1.26.0`.
- `DetectedProject.Runtime.Version` equals `"1.26.0"`

### Missing Go version leaves version empty
Create a temp directory with a `go.mod` that has no `go` directive.
- `DetectedProject.Runtime.Version` equals `""`
