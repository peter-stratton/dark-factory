package github

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EnsureLabel creates the label in the repo if it does not already exist.
// If the label exists, this is a no-op.
func EnsureLabel(repo, name, color, description string) error {
	out, err := CommandRunner("gh", "label", "list",
		"--repo", repo,
		"--search", name,
		"--json", "name",
	)
	if err != nil {
		return fmt.Errorf("searching for label: %w", err)
	}

	var labels []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &labels); err != nil {
		return fmt.Errorf("parsing label list: %w", err)
	}

	for _, l := range labels {
		if strings.EqualFold(l.Name, name) {
			return nil // already exists
		}
	}

	_, err = CommandRunner("gh", "label", "create",
		"--repo", repo,
		name,
		"--color", color,
		"--description", description,
	)
	if err != nil {
		return fmt.Errorf("creating label %q: %w", name, err)
	}
	return nil
}

// AddIssueLabel applies a label to a GitHub issue.
func AddIssueLabel(repo string, issueNum int, label string) error {
	_, err := CommandRunner("gh", "issue", "edit",
		fmt.Sprintf("%d", issueNum),
		"--repo", repo,
		"--add-label", label,
	)
	if err != nil {
		return fmt.Errorf("adding label %q to issue #%d: %w", label, issueNum, err)
	}
	return nil
}

// RemoveIssueLabel removes a label from a GitHub issue.
func RemoveIssueLabel(repo string, issueNum int, label string) error {
	_, err := CommandRunner("gh", "issue", "edit",
		fmt.Sprintf("%d", issueNum),
		"--repo", repo,
		"--remove-label", label,
	)
	if err != nil {
		return fmt.Errorf("removing label %q from issue #%d: %w", label, issueNum, err)
	}
	return nil
}

// FindIssuesWithLabel returns the numbers of open issues that have the given label.
func FindIssuesWithLabel(repo, label string) ([]int, error) {
	out, err := CommandRunner("gh", "issue", "list",
		"--repo", repo,
		"--state", "open",
		"--label", label,
		"--json", "number",
		"--limit", "200",
	)
	if err != nil {
		return nil, fmt.Errorf("finding issues with label %q: %w", label, err)
	}

	var raw []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing issue list: %w", err)
	}

	numbers := make([]int, len(raw))
	for i, r := range raw {
		numbers[i] = r.Number
	}
	return numbers, nil
}
