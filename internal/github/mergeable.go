package github

import (
	"encoding/json"
	"fmt"
)

// CheckMergeable returns the mergeable status of a pull request.
// Possible return values are "MERGEABLE", "CONFLICTING", and "UNKNOWN".
// Returns an error if the GitHub API call fails or the output cannot be parsed.
func CheckMergeable(repo string, prNum int) (string, error) {
	out, err := CommandRunner("gh", "pr", "view",
		fmt.Sprintf("%d", prNum),
		"--repo", repo,
		"--json", "mergeable",
	)
	if err != nil {
		return "", fmt.Errorf("gh pr view --json mergeable: %w", err)
	}

	var result struct {
		Mergeable string `json:"mergeable"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("parsing mergeable status: %w", err)
	}

	return result.Mergeable, nil
}

// UpdateBranch updates a PR branch by rebasing it onto the base branch via
// GitHub's built-in rebase mechanism. Returns an error if the update fails
// (e.g. the PR has unresolvable conflicts).
func UpdateBranch(repo string, prNum int) error {
	_, err := CommandRunner("gh", "pr", "update-branch",
		fmt.Sprintf("%d", prNum),
		"--repo", repo,
	)
	if err != nil {
		return fmt.Errorf("gh pr update-branch #%d: %w", prNum, err)
	}
	return nil
}
