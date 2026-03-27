package specdelta

import "strings"

// CaseChange describes a case present in both specs but with different content.
type CaseChange struct {
	Name   string
	Before string
	After  string
}

// Delta describes the differences between two scenario spec strings.
type Delta struct {
	AddedCases   []string
	RemovedCases []string
	ChangedCases []CaseChange
	SetupChanged bool
}

// IsEmpty reports whether the delta contains no changes.
func IsEmpty(d Delta) bool {
	return len(d.AddedCases) == 0 && len(d.RemovedCases) == 0 &&
		len(d.ChangedCases) == 0 && !d.SetupChanged
}

// Diff computes the delta between two raw scenario spec strings.
func Diff(before, after string) Delta {
	var d Delta

	beforeSetup, _ := sectionBody(before, "Setup")
	afterSetup, _ := sectionBody(after, "Setup")
	d.SetupChanged = beforeSetup != afterSetup

	beforeCasesBody, _ := sectionBody(before, "Cases")
	afterCasesBody, _ := sectionBody(after, "Cases")

	beforeNames, beforeBodies := parseCases(beforeCasesBody)
	afterNames, afterBodies := parseCases(afterCasesBody)

	afterSet := make(map[string]bool, len(afterNames))
	for _, name := range afterNames {
		afterSet[name] = true
	}

	beforeSet := make(map[string]bool, len(beforeNames))
	for _, name := range beforeNames {
		beforeSet[name] = true
	}

	for _, name := range beforeNames {
		if !afterSet[name] {
			d.RemovedCases = append(d.RemovedCases, name)
		} else if beforeBodies[name] != afterBodies[name] {
			d.ChangedCases = append(d.ChangedCases, CaseChange{
				Name:   name,
				Before: beforeBodies[name],
				After:  afterBodies[name],
			})
		}
	}

	for _, name := range afterNames {
		if !beforeSet[name] {
			d.AddedCases = append(d.AddedCases, name)
		}
	}

	return d
}

// Format renders a Delta as a markdown summary for a PR comment.
// Returns "" for an empty delta.
func Format(d Delta) string {
	if IsEmpty(d) {
		return ""
	}

	var b strings.Builder

	if len(d.AddedCases) > 0 {
		b.WriteString("### Added\n\n")
		for _, name := range d.AddedCases {
			b.WriteString("- ")
			b.WriteString(name)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(d.RemovedCases) > 0 {
		b.WriteString("### Removed\n\n")
		for _, name := range d.RemovedCases {
			b.WriteString("- ")
			b.WriteString(name)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(d.ChangedCases) > 0 {
		b.WriteString("### Changed\n\n")
		for _, cc := range d.ChangedCases {
			b.WriteString("#### ")
			b.WriteString(cc.Name)
			b.WriteString("\n\n")
			b.WriteString("**Before:**\n\n")
			b.WriteString(strings.TrimSpace(cc.Before))
			b.WriteString("\n\n")
			b.WriteString("**After:**\n\n")
			b.WriteString(strings.TrimSpace(cc.After))
			b.WriteString("\n\n")
		}
	}

	if d.SetupChanged {
		b.WriteString("### Setup changed\n\n")
		b.WriteString("The Setup section was modified.\n")
	}

	return b.String()
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

// parseCases returns an ordered list of case names and a map of name→body.
func parseCases(casesBody string) ([]string, map[string]string) {
	var names []string
	bodies := make(map[string]string)
	var current string
	var buf strings.Builder

	for _, line := range strings.Split(casesBody, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			if current != "" {
				bodies[current] = strings.TrimRight(buf.String(), "\n\r ")
			}
			current = strings.TrimSpace(trimmed[4:])
			names = append(names, current)
			buf.Reset()
			continue
		}
		if current != "" {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	if current != "" {
		bodies[current] = strings.TrimRight(buf.String(), "\n\r ")
	}
	return names, bodies
}
