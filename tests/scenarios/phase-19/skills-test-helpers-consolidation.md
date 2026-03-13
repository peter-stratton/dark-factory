# Scenario: Consolidate skills test helpers

Relates to: Issue #399

## Setup
- `internal/skills/helpers_test.go` exists with `readSkill()` and
  `parseFrontmatter()`

## Cases

### readSkill loads file
Call `readSkill(t, "godark-create-roadmap")`.
- Returns non-empty string containing YAML frontmatter

### readSkill fatals on missing
Call `readSkill(t, "nonexistent-skill")` in a sub-test.
- The test is marked as failed via `t.Fatalf`

### parseFrontmatter extracts YAML
Call `parseFrontmatter("---\nname: test\n---\nbody")`.
- Returns `"name: test"`

### No per-file readXxxSkill functions remain
Search `internal/skills/*_test.go` for `func read.*Skill(`.
- No matches found (all replaced by `readSkill`)

### parseFrontmatter not in architecture test
Read `internal/skills/godark_define_architecture_test.go`.
- Does not define `parseFrontmatter` (it's in `helpers_test.go`)

### All skills tests pass
Run `go test ./internal/skills/...`.
- All tests pass
