# Scenario: Consolidate file scaffold functions with writeFileWithDirs

Relates to: Issue #397

## Setup
- A `writeFileWithDirs(path, data)` helper exists in `internal/cmd/`
- Shared `docFiles` and `promptFiles` definitions used by both `init.go` and
  `new.go`

## Cases

### writeFileWithDirs creates parent directories
Call `writeFileWithDirs("/tmp/test/a/b/c.txt", []byte("hello"))`.
- Directory `/tmp/test/a/b/` exists
- File `/tmp/test/a/b/c.txt` contains `"hello"`

### Init skips existing files
Create a file at the doc destination. Run `godark init`.
- The existing file is not overwritten

### New always writes files
Create a file at the doc destination. Run `godark new` (in the new project flow).
- The file is overwritten with the template content

### docFiles defined once
Search `internal/cmd/` for `docFiles` struct literal definitions.
- Found in exactly one location (not duplicated between init.go and new.go)

### promptFiles defined once
Search `internal/cmd/` for `promptFiles` struct literal definitions.
- Found in exactly one location
