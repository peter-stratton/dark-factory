package vet

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/patterns"
)

// relatesToPattern matches "Relates to: Issue #N" lines, capturing all #N refs.
var relatesToPattern = regexp.MustCompile(`(?i)^relates\s+to\s*:\s*(.+)$`)

// ValidateScenarios checks scenario spec files in scenarioDir for correct
// format and cross-references them against milestone issues.
func ValidateScenarios(scenarioDir string, milestoneIssues []github.Issue, allIssueNumbers map[int]bool) *Report {
	r := &Report{}

	var files []string
	err := filepath.WalkDir(scenarioDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			files = append(files, path)
		}
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
// least one "- " bullet point outcome.
func validateCaseOutcomes(r *Report, loc string, casesBody string) {
	lines := strings.Split(casesBody, "\n")
	var currentCase string
	hasBullet := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			// If we had a previous case, check it
			if currentCase != "" && !hasBullet {
				r.Add(Finding{Severity: Error, Location: loc, Message: fmt.Sprintf("case %q has no outcome bullets", currentCase)})
			}
			currentCase = strings.TrimSpace(trimmed[4:])
			hasBullet = false
			continue
		}
		if currentCase != "" && strings.HasPrefix(trimmed, "- ") {
			hasBullet = true
		}
	}

	// Check last case
	if currentCase != "" && !hasBullet {
		r.Add(Finding{Severity: Error, Location: loc, Message: fmt.Sprintf("case %q has no outcome bullets", currentCase)})
	}
}
