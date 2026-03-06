# Scenario: Structured PR comment parser

Relates to: Issue #148

## Setup
- The `internal/dialogue/` package is tested directly via Go unit tests
- Input is raw PR comment body text (strings), no GitHub API calls
- No external services required

## Cases

### Full implementation notes parsed
Call `ParseImplementationNotes` with a comment body containing all four
subsections (Approach, Key Decisions, Known Limitations, Architecture).
- Returns a non-nil `*ImplementationNotes`
- Each field contains the corresponding section text
- `Raw` field contains the full original comment text

### Partial implementation notes parsed
Call `ParseImplementationNotes` with a comment missing the "Known Limitations"
subsection.
- Returns a non-nil `*ImplementationNotes`
- `KnownLimitations` is an empty string
- Other fields are populated correctly

### Full review notes parsed
Call `ParseReviewNotes` with a comment body containing all three subsections
(Approved, Changes Requested, Architecture Compliance).
- Returns a non-nil `*ReviewNotes`
- Each field contains the corresponding section text

### Non-matching comment returns nil
Call `ParseImplementationNotes` with a regular comment body that does not
contain "## Implementation Notes".
- Returns nil

### ParseComments with multiple comments
Call `ParseComments` with three comment bodies: one implementation notes, one
review notes, and another implementation notes.
- Returns a slice of two `ImplementationNotes` and one `ReviewNotes`
- Notes are returned in the order they appeared in the input

### Whitespace is trimmed
Call `ParseImplementationNotes` with a comment where section content has
leading and trailing whitespace.
- Section fields have leading and trailing whitespace removed

### Raw field preserved
Call `ParseImplementationNotes` with any valid comment.
- `Raw` field contains the exact original comment text (untrimmed)
