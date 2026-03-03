# Scenario: Issue structure validation

Relates to: Issue #15

## Setup
- The vet issues package (`internal/vet`) is imported directly
- The `github.CommandRunner` variable is stubbed to return controlled JSON responses (no real GitHub API calls)
- Stub responses include issue bodies with various combinations of required sections
- No external services or network access required

## Cases

### Complete issue produces no error findings
Stub a single issue with `## Description`, `## Acceptance criteria` (with checkboxes), `## Test cases` (with entries), valid `**Blocked by**: #1` notation, and the correct phase label.
- No error findings are produced
- No warning findings are produced

### Missing acceptance criteria section
Stub an issue body that has `## Description` and `## Test cases` but no `## Acceptance criteria` section.
- An error finding is produced
- The finding message mentions "acceptance criteria"
- The finding location identifies the issue number

### Missing test cases section
Stub an issue body that has `## Description` and `## Acceptance criteria` but no `## Test cases` section.
- An error finding is produced
- The finding message mentions "test cases"
- The finding location identifies the issue number

### Empty acceptance criteria section
Stub an issue body with `## Acceptance criteria` heading but no `- [ ]` checkbox items beneath it.
- An error finding is produced
- The finding message mentions "acceptance criteria" and "empty" or "no checkboxes"

### Empty test cases section
Stub an issue body with `## Test cases` heading but no `- **Name**:` entries beneath it.
- An error finding is produced
- The finding message mentions "test cases" and "empty" or "no entries"

### Malformed blocker notation
Stub an issue body containing `Blocked by #1` (missing colon after "Blocked by").
- A warning finding is produced
- The finding message mentions malformed dependency notation

### Non-existent blocker reference
Stub an issue body with `**Blocked by**: #999` and stub GitHub to indicate #999 does not exist.
- A warning finding is produced
- The finding message mentions the non-existent issue number

### Missing phase label
Stub an issue in the Phase 2 milestone that has no `phase-2` label.
- A warning finding is produced
- The finding message mentions the missing phase label

### Multiple issues are all validated
Stub three issues: one valid, one missing acceptance criteria, one missing test cases.
- Two error findings are produced (one per invalid issue)
- The valid issue produces no error findings
