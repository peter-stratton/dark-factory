# Scenario: Scenario spec validation

Relates to: Issue #16

## Setup
- The vet scenarios package (`internal/vet`) is imported directly
- A temporary directory containing scenario spec markdown files with various formats
- The `github.CommandRunner` variable is stubbed to return controlled issue lists (for cross-referencing)
- No external services or network access required

## Cases

### Valid scenario spec produces no findings
A spec file with `# Scenario:` title, `Relates to: Issue #14`, `## Setup` section, `## Cases` section with cases that have bullet-point outcomes.
- No error findings are produced
- No warning findings are produced

### Missing Setup section
A spec file with `# Scenario:` title and `## Cases` but no `## Setup` section.
- An error finding is produced
- The finding message mentions "Setup"
- The finding location is the file path

### Missing Cases section
A spec file with `# Scenario:` title and `## Setup` but no `## Cases` section.
- An error finding is produced
- The finding message mentions "Cases"
- The finding location is the file path

### Case without expected outcomes
A spec file where a `### Case name` heading is followed by descriptive text but no bullet points (lines starting with `- `).
- An error finding is produced
- The finding message mentions the case name or "expected outcomes"

### Missing Relates to line
A spec file with valid format but no `Relates to:` line.
- An error finding is produced
- The finding message mentions "Relates to"

### Malformed Relates to line
A spec file with `Relates to: 5` instead of `Relates to: Issue #5`.
- An error finding is produced
- The finding message mentions the malformed format

### Non-existent issue reference
A spec file with `Relates to: Issue #999` where issue #999 does not exist on GitHub.
- A warning finding is produced
- The finding message mentions the non-existent issue

### Issue in milestone with no scenario spec coverage
Stub GitHub to return issue #14 in the milestone, but no spec file contains `Relates to: Issue #14`.
- A warning finding is produced
- The finding message mentions issue #14 and "no scenario spec"

### Multiple Relates to lines cover multiple issues
A spec file with both `Relates to: Issue #14` and `Relates to: Issue #15`.
- Both issue #8 and issue #9 are considered covered by this spec
- No "missing coverage" warnings are produced for either issue
