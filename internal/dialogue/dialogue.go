package dialogue

import (
	"bufio"
	"strings"
)

// ImplementationNotes holds the structured content from an "## Implementation Notes"
// PR comment posted by the implementer agent.
type ImplementationNotes struct {
	Approach         string
	KeyDecisions     string
	KnownLimitations string
	Architecture     string
	Raw              string // full comment text
}

// ReviewNotes holds the structured content from a "## Review Notes"
// PR comment posted by the reviewer agent.
type ReviewNotes struct {
	Approved               string
	ChangesRequested       string
	ArchitectureCompliance string
	Raw                    string // full comment text
}

// ParseImplementationNotes extracts Implementation Notes sections from a PR
// comment body. Returns nil if the comment does not contain an
// "## Implementation Notes" header.
func ParseImplementationNotes(body string) *ImplementationNotes {
	sections := parseSections(body, "## Implementation Notes")
	if sections == nil {
		return nil
	}
	return &ImplementationNotes{
		Approach:         sections["### Approach"],
		KeyDecisions:     sections["### Key Decisions"],
		KnownLimitations: sections["### Known Limitations"],
		Architecture:     sections["### Architecture"],
		Raw:              body,
	}
}

// ParseReviewNotes extracts Review Notes sections from a PR comment body.
// Returns nil if the comment does not contain a "## Review Notes" header.
func ParseReviewNotes(body string) *ReviewNotes {
	sections := parseSections(body, "## Review Notes")
	if sections == nil {
		return nil
	}
	return &ReviewNotes{
		Approved:               sections["### Approved"],
		ChangesRequested:       sections["### Changes Requested"],
		ArchitectureCompliance: sections["### Architecture Compliance"],
		Raw:                    body,
	}
}

// ParseComments scans a slice of comment bodies and returns all implementation
// notes and review notes found, in order.
func ParseComments(bodies []string) ([]ImplementationNotes, []ReviewNotes) {
	var implNotes []ImplementationNotes
	var reviewNotes []ReviewNotes
	for _, body := range bodies {
		if n := ParseImplementationNotes(body); n != nil {
			implNotes = append(implNotes, *n)
		}
		if n := ParseReviewNotes(body); n != nil {
			reviewNotes = append(reviewNotes, *n)
		}
	}
	return implNotes, reviewNotes
}

// parseSections scans body for the given top-level header and returns a map of
// subsection header → trimmed content. Returns nil if the top-level header is
// not found.
func parseSections(body, topHeader string) map[string]string {
	scanner := bufio.NewScanner(strings.NewReader(body))

	// Advance until we find the top-level header.
	found := false
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == topHeader {
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	sections := make(map[string]string)
	var currentKey string
	var buf strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Stop at any new top-level (##) header that is not a subsection (###).
		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			break
		}

		if strings.HasPrefix(trimmed, "### ") {
			// Save previous subsection.
			if currentKey != "" {
				sections[currentKey] = strings.TrimSpace(buf.String())
			}
			currentKey = trimmed
			buf.Reset()
			continue
		}

		if currentKey != "" {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}

	// Save the last subsection.
	if currentKey != "" {
		sections[currentKey] = strings.TrimSpace(buf.String())
	}

	return sections
}
