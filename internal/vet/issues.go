package vet

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/phs/dark-factory/internal/github"
)

// blockerPattern matches "Blocked by: #1, #2" or "Depends on: #3" lines.
var blockerPattern = regexp.MustCompile(`(?i)^(?:blocked by|depends on)\s*:\s*(.+)$`)

// issueRefPattern matches "#N" references.
var issueRefPattern = regexp.MustCompile(`#(\d+)`)

// ValidateIssues checks that each issue has the required structure for agent
// consumption. It returns a report of findings.
func ValidateIssues(issues []github.Issue, allIssueNumbers map[int]bool, phaseLabel string) *Report {
	r := &Report{}

	for _, iss := range issues {
		loc := fmt.Sprintf("#%d", iss.Number)

		// Check for ## Acceptance criteria section
		acBody, hasAC := sectionBody(iss.Body, "Acceptance criteria")
		if !hasAC {
			r.Add(Finding{Severity: Error, Location: loc, Message: "missing ## Acceptance criteria section"})
		} else if !strings.Contains(acBody, "- [ ]") {
			r.Add(Finding{Severity: Error, Location: loc, Message: "## Acceptance criteria has no checkbox items (- [ ])"})
		}

		// Check for ## Test cases section
		tcBody, hasTC := sectionBody(iss.Body, "Test cases")
		if !hasTC {
			r.Add(Finding{Severity: Error, Location: loc, Message: "missing ## Test cases section"})
		} else if !strings.Contains(tcBody, "- **") {
			r.Add(Finding{Severity: Error, Location: loc, Message: "## Test cases has no - **Name**: entries"})
		}

		// Check blocker lines
		for _, line := range strings.Split(iss.Body, "\n") {
			line = strings.TrimSpace(line)
			lowerLine := strings.ToLower(line)

			// Detect malformed blocker notation (has keyword but no colon)
			if (strings.HasPrefix(lowerLine, "blocked by") || strings.HasPrefix(lowerLine, "depends on")) &&
				!strings.Contains(line, ":") {
				r.Add(Finding{Severity: Warning, Location: loc, Message: fmt.Sprintf("malformed blocker notation (missing colon): %q", line)})
				continue
			}

			m := blockerPattern.FindStringSubmatch(line)
			if m == nil {
				continue
			}

			// Check each referenced issue exists
			refs := issueRefPattern.FindAllStringSubmatch(m[1], -1)
			for _, ref := range refs {
				num, _ := strconv.Atoi(ref[1])
				if !allIssueNumbers[num] {
					r.Add(Finding{Severity: Warning, Location: loc, Message: fmt.Sprintf("blocker references non-existent issue #%d", num)})
				}
			}
		}

		// Check phase label
		if phaseLabel != "" {
			hasLabel := false
			for _, l := range iss.Labels {
				if strings.EqualFold(l, phaseLabel) {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				r.Add(Finding{Severity: Warning, Location: loc, Message: fmt.Sprintf("missing phase label %q", phaseLabel)})
			}
		}
	}

	return r
}

// sectionBody extracts the text between a ## heading and the next ## or EOF.
// The heading match is case-insensitive.
func sectionBody(body, heading string) (string, bool) {
	lines := strings.Split(body, "\n")
	lowerHeading := strings.ToLower(heading)

	inSection := false
	var buf strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			sectionName := strings.TrimSpace(trimmed[3:])
			if strings.EqualFold(sectionName, lowerHeading) {
				inSection = true
				continue
			}
			if inSection {
				break
			}
			continue
		}

		if inSection {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}

	if !inSection {
		return "", false
	}
	return buf.String(), true
}
