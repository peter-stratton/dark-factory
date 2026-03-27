package vet

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/peter-stratton/dark-factory/internal/github"
	"github.com/peter-stratton/dark-factory/internal/mdutil"
	"github.com/peter-stratton/dark-factory/internal/patterns"
)

// relatesToPattern matches "Relates to: Issue #N" lines, capturing all #N refs.
var relatesToPattern = regexp.MustCompile(`(?i)^relates\s+to\s*:\s*(.+)$`)

// ValidateScenarios checks scenario spec files in scenarioDir for correct
// format and cross-references them against milestone issues.
func ValidateScenarios(scenarioDir string, milestoneIssues []github.Issue, allIssueNumbers map[int]bool) *Report {
	r := &Report{}

	var files []string
	err := mdutil.WalkMarkdownFiles(scenarioDir, func(path string) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		r.Add(Finding{Severity: Error, Location: scenarioDir, Message: fmt.Sprintf("reading scenario dir: %v", err)})
		return r
	}

	// Track which milestone issues are covered by at least one spec
	covered := make(map[int]bool)

	for _, f := range files {
		coveredByFile := validateScenarioFile(r, f, allIssueNumbers)
		for n := range coveredByFile {
			covered[n] = true
		}
	}

	// Coverage cross-ref: milestone issues without a matching spec
	for _, iss := range milestoneIssues {
		if !covered[iss.Number] {
			r.Add(Finding{
				Severity: Warning,
				Location: fmt.Sprintf("#%d", iss.Number),
				Message:  "milestone issue has no matching scenario spec",
			})
		}
	}

	return r
}

// validateScenarioFile checks a single scenario file and returns the set of
// issue numbers it relates to.
func validateScenarioFile(r *Report, path string, allIssueNumbers map[int]bool) map[int]bool {
	// Show path relative to the scenario dir parent for clearer diagnostics.
	loc := filepath.Base(path)
	if parts := strings.Split(filepath.ToSlash(path), "/"); len(parts) >= 2 {
		parent := parts[len(parts)-2]
		if strings.HasPrefix(parent, "phase-") {
			loc = parent + "/" + filepath.Base(path)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		r.Add(Finding{Severity: Error, Location: loc, Message: fmt.Sprintf("reading file: %v", err)})
		return nil
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Check for # Scenario: title
	hasTitle := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# Scenario:") {
			hasTitle = true
			break
		}
	}
	if !hasTitle {
		r.Add(Finding{Severity: Error, Location: loc, Message: "missing # Scenario: title"})
	}

	// Check for Relates to: Issue #N
	relatedIssues := make(map[int]bool)
	hasRelatesTo := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		m := relatesToPattern.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		hasRelatesTo = true
		refs := patterns.IssueRef.FindAllStringSubmatch(m[1], -1)
		if len(refs) == 0 {
			r.Add(Finding{Severity: Error, Location: loc, Message: fmt.Sprintf("malformed Relates to line (no issue refs): %q", trimmed)})
			continue
		}
		for _, ref := range refs {
			num, _ := strconv.Atoi(ref[1])
			relatedIssues[num] = true
			if allIssueNumbers != nil && !allIssueNumbers[num] {
				r.Add(Finding{Severity: Warning, Location: loc, Message: fmt.Sprintf("Relates to references non-existent issue #%d", num)})
			}
		}
	}
	if !hasRelatesTo {
		r.Add(Finding{Severity: Error, Location: loc, Message: "missing Relates to: Issue #N line"})
	}

	// Check for ## Setup section
	if _, hasSetup := sectionBody(content, "Setup"); !hasSetup {
		r.Add(Finding{Severity: Error, Location: loc, Message: "missing ## Setup section"})
	}

	// Check for ## Cases section
	casesBody, hasCases := sectionBody(content, "Cases")
	if !hasCases {
		r.Add(Finding{Severity: Error, Location: loc, Message: "missing ## Cases section"})
	} else {
		// Check each ### Case has at least one - bullet outcome
		validateCaseOutcomes(r, loc, casesBody)
	}

	return relatedIssues
}

// validateCaseOutcomes checks that each ### case heading is followed by at
// least one GIVEN, one WHEN, and one THEN bullet.
func validateCaseOutcomes(r *Report, loc string, casesBody string) {
	lines := strings.Split(casesBody, "\n")
	var currentCase string
	var hasGiven, hasWhen, hasThen bool

	flush := func() {
		if currentCase == "" {
			return
		}
		for _, clause := range []struct {
			have bool
			name string
		}{
			{hasGiven, "GIVEN"}, {hasWhen, "WHEN"}, {hasThen, "THEN"},
		} {
			if !clause.have {
				r.Add(Finding{Severity: Error, Location: loc,
					Message: fmt.Sprintf("case %q missing %s clause", currentCase, clause.name)})
			}
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			flush()
			currentCase = strings.TrimSpace(trimmed[4:])
			hasGiven, hasWhen, hasThen = false, false, false
			continue
		}
		if currentCase != "" {
			up := strings.ToUpper(trimmed)
			if strings.HasPrefix(up, "- GIVEN ") {
				hasGiven = true
			}
			if strings.HasPrefix(up, "- WHEN ") {
				hasWhen = true
			}
			if strings.HasPrefix(up, "- THEN ") {
				hasThen = true
			}
		}
	}
	flush()
}
