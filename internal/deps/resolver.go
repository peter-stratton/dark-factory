package deps

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/phs/dark-factory/internal/github"
)

// depPattern matches dependency declarations like:
//   - **Blocked by**: #1 (description)
//   - Depends on: #3, #5
//   - BLOCKED BY: #4
var depPattern = regexp.MustCompile(`(?im)^\**(?:blocked by|depends on)\**:\s*(.+)$`)

// issueRefPattern matches issue number references like #1, #42.
var issueRefPattern = regexp.MustCompile(`#(\d+)`)

// ParseDeps extracts dependency issue numbers from an issue body.
func ParseDeps(body string) []int {
	var deps []int
	for _, match := range depPattern.FindAllStringSubmatch(body, -1) {
		refs := issueRefPattern.FindAllStringSubmatch(match[1], -1)
		for _, ref := range refs {
			n, err := strconv.Atoi(ref[1])
			if err != nil {
				continue
			}
			deps = append(deps, n)
		}
	}
	return deps
}

// FilterUnblocked returns issues whose dependencies are all closed.
// closedIssues contains the set of issue numbers that are closed.
// Issues with no dependency declarations are always included.
// The input order is preserved.
func FilterUnblocked(issues []github.Issue, closedIssues map[int]bool) []github.Issue {
	var result []github.Issue
	for _, issue := range issues {
		deps := ParseDeps(issue.Body)
		if allClosed(deps, closedIssues) {
			result = append(result, issue)
		}
	}
	return result
}

// allClosed returns true if every issue number in deps is in the closed set,
// or if deps is empty.
func allClosed(deps []int, closed map[int]bool) bool {
	for _, d := range deps {
		if !closed[d] {
			return false
		}
	}
	return true
}

// ClosedSet builds a lookup set from a slice of issue numbers.
func ClosedSet(numbers []int) map[int]bool {
	m := make(map[int]bool, len(numbers))
	for _, n := range numbers {
		m[n] = true
	}
	return m
}

// IssueWithDeps pairs an issue with its parsed dependency numbers for inspection.
type IssueWithDeps struct {
	Issue github.Issue
	Deps  []int
}

// Resolve takes a list of issues and a set of closed issue numbers, and
// returns only issues that are unblocked, preserving the original order.
// This is the main entry point that combines parsing and filtering.
func Resolve(issues []github.Issue, closedIssues map[int]bool) []github.Issue {
	return FilterUnblocked(issues, closedIssues)
}

// ResolveWithDeps is like Resolve but also returns dependency info for each issue.
func ResolveWithDeps(issues []github.Issue, closedIssues map[int]bool) []IssueWithDeps {
	var result []IssueWithDeps
	for _, issue := range issues {
		deps := ParseDeps(issue.Body)
		if allClosed(deps, closedIssues) {
			result = append(result, IssueWithDeps{Issue: issue, Deps: deps})
		}
	}
	return result
}

// CollectAllDeps scans a slice of issues and returns every unique dependency
// issue number referenced across all of them.
func CollectAllDeps(issues []github.Issue) []int {
	seen := make(map[int]bool)
	var all []int
	for _, issue := range issues {
		for _, d := range ParseDeps(issue.Body) {
			if !seen[d] {
				seen[d] = true
				all = append(all, d)
			}
		}
	}
	return all
}

// depLinePattern matches individual lines that look like dependency declarations,
// used internally to avoid false positives from unrelated #N references.
var depLinePattern = regexp.MustCompile(`(?i)^\**(?:blocked by|depends on)\**:`)

// HasDeps returns true if the issue body contains any dependency declarations.
func HasDeps(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if depLinePattern.MatchString(strings.TrimSpace(line)) {
			return true
		}
	}
	return false
}
