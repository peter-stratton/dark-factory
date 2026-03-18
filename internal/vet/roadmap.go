package vet

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/peter-stratton/dark-factory/internal/github"
	"github.com/peter-stratton/dark-factory/internal/patterns"
)

// issueHeadingPattern matches "## Issue N:" or "## Issue #N:" headings.
var issueHeadingPattern = regexp.MustCompile(`(?i)^##\s+issue\s+#?(\d+)\s*:`)

// ValidateRoadmap checks planning docs in planningDir for correct cross-references
// against milestone issues.
func ValidateRoadmap(planningDir string, milestoneIssues []github.Issue, milestoneTitle string, allIssueNumbers map[int]bool) *Report {
	r := &Report{}

	files, err := filepath.Glob(filepath.Join(planningDir, "*.md"))
	if err != nil {
		r.Add(Finding{Severity: Error, Location: planningDir, Message: fmt.Sprintf("reading planning dir: %v", err)})
		return r
	}

	if len(files) == 0 {
		r.Add(Finding{Severity: Warning, Location: planningDir, Message: "no planning docs found"})
		// Still check label mismatches on issues
		checkLabelMismatch(r, milestoneIssues, milestoneTitle)
		return r
	}

	// Collect all issue numbers referenced by ## Issue N: headings across all docs
	documented := make(map[int]bool)

	for _, f := range files {
		loc := filepath.Base(f)

		data, err := os.ReadFile(f)
		if err != nil {
			r.Add(Finding{Severity: Error, Location: loc, Message: fmt.Sprintf("reading file: %v", err)})
			continue
		}

		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			m := issueHeadingPattern.FindStringSubmatch(trimmed)
			if m == nil {
				continue
			}
			num, _ := strconv.Atoi(m[1])
			documented[num] = true

			// Phantom ref: issue heading references non-existent issue
			if !allIssueNumbers[num] {
				r.Add(Finding{Severity: Warning, Location: loc, Message: fmt.Sprintf("references non-existent issue #%d (phantom)", num)})
			}
		}
	}

	// Orphan check: milestone issues not in any planning doc
	for _, iss := range milestoneIssues {
		if !documented[iss.Number] {
			r.Add(Finding{Severity: Warning, Location: fmt.Sprintf("#%d", iss.Number), Message: "milestone issue not in any planning doc (orphaned)"})
		}
	}

	// Label mismatch check
	checkLabelMismatch(r, milestoneIssues, milestoneTitle)

	return r
}

// checkLabelMismatch checks that issues with a phase label matching the
// pattern "phase-N" have one consistent with the milestone title.
func checkLabelMismatch(r *Report, milestoneIssues []github.Issue, milestoneTitle string) {
	var expectedLabel string
	if m := patterns.Phase.FindStringSubmatch(milestoneTitle); m != nil {
		expectedLabel = "phase-" + m[1]
	} else {
		expectedLabel = strings.ReplaceAll(strings.ToLower(milestoneTitle), " ", "-")
	}
	if expectedLabel == "" {
		return
	}

	for _, iss := range milestoneIssues {
		for _, l := range iss.Labels {
			lower := strings.ToLower(l)
			// Only check labels that look like phase labels
			if strings.HasPrefix(lower, "phase-") && lower != expectedLabel {
				r.Add(Finding{
					Severity: Error,
					Location: fmt.Sprintf("#%d", iss.Number),
					Message:  fmt.Sprintf("has label %q but milestone is %q", l, milestoneTitle),
				})
			}
		}
	}
}
