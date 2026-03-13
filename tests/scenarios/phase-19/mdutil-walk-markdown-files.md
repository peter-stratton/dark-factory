# Scenario: WalkMarkdownFiles helper package

Relates to: Issue #384

## Setup
- A temporary directory containing files: `a.md`, `b.txt`, `c.go`, `sub/d.md`
- The `internal/mdutil` package exists with `WalkMarkdownFiles` exported

## Cases

### Walks only markdown files
Call `mdutil.WalkMarkdownFiles(tmpDir, collector)` where `collector` appends paths to a slice.
- The slice contains paths ending in `a.md` and `sub/d.md`
- The slice does not contain `b.txt` or `c.go`

### Skips directories
Call `mdutil.WalkMarkdownFiles(tmpDir, collector)`.
- No path in the collected slice is a directory

### Propagates callback error
Call `mdutil.WalkMarkdownFiles(tmpDir, fn)` where `fn` returns `errors.New("stop")` on the first call.
- `WalkMarkdownFiles` returns an error containing "stop"

### Handles missing directory
Call `mdutil.WalkMarkdownFiles("/nonexistent", fn)`.
- Returns a non-nil error

### Architecture updated
Read `docs/architecture.json`.
- The `foundation` layer's `paths` array includes `internal/mdutil/`
