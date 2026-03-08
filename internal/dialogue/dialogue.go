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

// QualityReviewNotes holds the structured content from a "## Quality Review Notes"
// PR comment posted by the quality reviewer agent.
type QualityReviewNotes struct {
	ChangesRequested string
	IssuesFound      string
	Raw              string // full comment text
}

// ParseQualityReviewNotes extracts Quality Review Notes sections from a PR
// comment body. Returns nil if the comment does not contain a
// "## Quality Review Notes" header.
func ParseQualityReviewNotes(body string) *QualityReviewNotes {
	sections := parseSections(body, "## Quality Review Notes")
	if sections == nil {
		return nil
	}
	return &QualityReviewNotes{
		ChangesRequested: sections["### Changes Requested"],
		IssuesFound:      sections["### Issues Found"],
		Raw:              body,
	}
}

// ParseComments scans a slice of comment bodies and returns all implementation
// notes, review notes, and quality review notes found, in order.
func ParseComments(bodies []string) ([]ImplementationNotes, []ReviewNotes, []QualityReviewNotes) {
	var implNotes []ImplementationNotes
	var reviewNotes []ReviewNotes
	var qualityNotes []QualityReviewNotes
	for _, body := range bodies {
		if n := ParseImplementationNotes(body); n != nil {
			implNotes = append(implNotes, *n)
		}
		if n := ParseQualityReviewNotes(body); n != nil {
			qualityNotes = append(qualityNotes, *n)
		} else if n := ParseReviewNotes(body); n != nil {
			reviewNotes = append(reviewNotes, *n)
		}
	}
	return implNotes, reviewNotes, qualityNotes
}

// Entry is a single turn in the agent dialogue, preserving execution order.
type Entry struct {
	Role  string // "implementer", "quality_reviewer", or "reviewer"
	Round int    // 1-indexed round within this role
	Body  string // raw comment text
}

// ParseCommentsInOrder scans comment bodies in their original order (which
// matches GitHub's chronological ordering) and returns one Entry per
// recognised structured comment. The Round field reflects how many times
// that particular role has appeared so far, so two implementer comments
// get rounds 1 and 2 regardless of what comes between them.
func ParseCommentsInOrder(bodies []string) []Entry {
	roundCounters := map[string]int{}
	var entries []Entry
	for _, body := range bodies {
		var role string
		if ParseImplementationNotes(body) != nil {
			role = "implementer"
		} else if ParseQualityReviewNotes(body) != nil {
			role = "quality_reviewer"
		} else if ParseReviewNotes(body) != nil {
			role = "reviewer"
		} else {
			continue
		}
		roundCounters[role]++
		entries = append(entries, Entry{
			Role:  role,
			Round: roundCounters[role],
			Body:  body,
		})
	}
	return entries
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
