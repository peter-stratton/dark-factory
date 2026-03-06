package dialogue

import "testing"

const fullImplComment = `## Implementation Notes

### Approach
Used a line-by-line scanner.

### Key Decisions
- Chose bufio.Scanner for simplicity.

### Known Limitations
- Does not handle deeply nested headers.

### Architecture
Placed in domain layer with no external dependencies.
`

const fullReviewComment = `## Review Notes

### Approved
- Clean implementation.

### Changes Requested
- Add more tests.

### Architecture Compliance
Layer boundaries respected.
`

func TestParseImplementationNotes_Full(t *testing.T) {
	t.Helper()
	got := ParseImplementationNotes(fullImplComment)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Approach != "Used a line-by-line scanner." {
		t.Errorf("Approach = %q, want %q", got.Approach, "Used a line-by-line scanner.")
	}
	if got.KeyDecisions != "- Chose bufio.Scanner for simplicity." {
		t.Errorf("KeyDecisions = %q, want %q", got.KeyDecisions, "- Chose bufio.Scanner for simplicity.")
	}
	if got.KnownLimitations != "- Does not handle deeply nested headers." {
		t.Errorf("KnownLimitations = %q, want %q", got.KnownLimitations, "- Does not handle deeply nested headers.")
	}
	if got.Architecture != "Placed in domain layer with no external dependencies." {
		t.Errorf("Architecture = %q, want %q", got.Architecture, "Placed in domain layer with no external dependencies.")
	}
	if got.Raw != fullImplComment {
		t.Errorf("Raw does not match original comment text")
	}
}

func TestParseImplementationNotes_Partial(t *testing.T) {
	body := `## Implementation Notes

### Approach
Simple approach.

### Key Decisions
Some decisions.
`
	got := ParseImplementationNotes(body)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Approach != "Simple approach." {
		t.Errorf("Approach = %q, want %q", got.Approach, "Simple approach.")
	}
	if got.KeyDecisions != "Some decisions." {
		t.Errorf("KeyDecisions = %q, want %q", got.KeyDecisions, "Some decisions.")
	}
	if got.KnownLimitations != "" {
		t.Errorf("KnownLimitations = %q, want empty string", got.KnownLimitations)
	}
	if got.Architecture != "" {
		t.Errorf("Architecture = %q, want empty string", got.Architecture)
	}
}

func TestParseReviewNotes_Full(t *testing.T) {
	got := ParseReviewNotes(fullReviewComment)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Approved != "- Clean implementation." {
		t.Errorf("Approved = %q, want %q", got.Approved, "- Clean implementation.")
	}
	if got.ChangesRequested != "- Add more tests." {
		t.Errorf("ChangesRequested = %q, want %q", got.ChangesRequested, "- Add more tests.")
	}
	if got.ArchitectureCompliance != "Layer boundaries respected." {
		t.Errorf("ArchitectureCompliance = %q, want %q", got.ArchitectureCompliance, "Layer boundaries respected.")
	}
	if got.Raw != fullReviewComment {
		t.Errorf("Raw does not match original comment text")
	}
}

func TestParseImplementationNotes_NotANotesComment(t *testing.T) {
	body := "This is just a regular PR comment with no special headers."
	got := ParseImplementationNotes(body)
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestParseReviewNotes_NotANotesComment(t *testing.T) {
	body := "LGTM! Great work."
	got := ParseReviewNotes(body)
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestParseComments_MultipleComments(t *testing.T) {
	impl1 := `## Implementation Notes

### Approach
First implementation.

### Key Decisions
Decision A.

### Known Limitations
None.

### Architecture
Domain layer.
`
	review1 := `## Review Notes

### Approved
Looks good.

### Changes Requested
Fix the typo.

### Architecture Compliance
Compliant.
`
	impl2 := `## Implementation Notes

### Approach
Second implementation after review.

### Key Decisions
Decision B.

### Known Limitations
None known.

### Architecture
Same domain layer.
`
	bodies := []string{impl1, review1, impl2}
	implNotes, reviewNotes := ParseComments(bodies)

	if len(implNotes) != 2 {
		t.Fatalf("len(implNotes) = %d, want 2", len(implNotes))
	}
	if len(reviewNotes) != 1 {
		t.Fatalf("len(reviewNotes) = %d, want 1", len(reviewNotes))
	}

	// Verify order is preserved.
	if implNotes[0].Approach != "First implementation." {
		t.Errorf("implNotes[0].Approach = %q, want %q", implNotes[0].Approach, "First implementation.")
	}
	if implNotes[1].Approach != "Second implementation after review." {
		t.Errorf("implNotes[1].Approach = %q, want %q", implNotes[1].Approach, "Second implementation after review.")
	}
	if reviewNotes[0].Approved != "Looks good." {
		t.Errorf("reviewNotes[0].Approved = %q, want %q", reviewNotes[0].Approved, "Looks good.")
	}
}

func TestParseImplementationNotes_WhitespaceHandling(t *testing.T) {
	body := `## Implementation Notes

### Approach

   Indented and padded approach.

### Key Decisions

  Some decisions with leading spaces.

`
	got := ParseImplementationNotes(body)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Approach != "Indented and padded approach." {
		t.Errorf("Approach = %q, want %q", got.Approach, "Indented and padded approach.")
	}
	if got.KeyDecisions != "Some decisions with leading spaces." {
		t.Errorf("KeyDecisions = %q, want %q", got.KeyDecisions, "Some decisions with leading spaces.")
	}
}

func TestParseImplementationNotes_RawPreserved(t *testing.T) {
	body := "## Implementation Notes\n\n### Approach\nDone.\n"
	got := ParseImplementationNotes(body)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Raw != body {
		t.Errorf("Raw = %q, want %q", got.Raw, body)
	}
}

func TestParseReviewNotes_RawPreserved(t *testing.T) {
	body := "## Review Notes\n\n### Approved\nAll good.\n"
	got := ParseReviewNotes(body)
	if got == nil {
		t.Fatal("expected non-nil, got nil")
	}
	if got.Raw != body {
		t.Errorf("Raw = %q, want %q", got.Raw, body)
	}
}

func TestParseComments_Empty(t *testing.T) {
	implNotes, reviewNotes := ParseComments([]string{})
	if len(implNotes) != 0 {
		t.Errorf("len(implNotes) = %d, want 0", len(implNotes))
	}
	if len(reviewNotes) != 0 {
		t.Errorf("len(reviewNotes) = %d, want 0", len(reviewNotes))
	}
}

func TestParseComments_NoMatchingComments(t *testing.T) {
	bodies := []string{"Just a regular comment.", "Another comment."}
	implNotes, reviewNotes := ParseComments(bodies)
	if len(implNotes) != 0 {
		t.Errorf("len(implNotes) = %d, want 0", len(implNotes))
	}
	if len(reviewNotes) != 0 {
		t.Errorf("len(reviewNotes) = %d, want 0", len(reviewNotes))
	}
}
