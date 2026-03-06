# Scenario: Update architecture.json for dialogue package

Relates to: Issue #146

## Setup
- `docs/architecture.json` and `docs/architecture.md` exist in the project root
- `godark vet architecture` is available and functional
- No external services required

## Cases

### JSON domain layer includes dialogue path
Read `docs/architecture.json` and parse the domain layer entry.
- The domain layer's `paths` array contains `"internal/dialogue/"`
- The JSON file parses without errors

### Markdown domain layer lists dialogue path
Read `docs/architecture.md` and find the domain layer section.
- The **Paths** list includes `internal/dialogue/`

### Vet produces no findings
Run `godark vet architecture`.
- Exit code is 0
- No cycle errors or warnings are reported
