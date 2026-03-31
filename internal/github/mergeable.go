package github

import (
	"encoding/json"
	"fmt"
	"time"
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

// WaitForMergeable polls GitHub until the mergeable status resolves from
// UNKNOWN to either MERGEABLE or CONFLICTING. Returns the resolved status.
// Times out after the given duration, returning "UNKNOWN" if still unresolved.
func WaitForMergeable(repo string, prNum int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	delay := 2 * time.Second

	for time.Now().Before(deadline) {
		status, err := CheckMergeable(repo, prNum)
		if err != nil {
			return "", err
		}
		if status != "UNKNOWN" {
			return status, nil
		}
		remaining := time.Until(deadline)
		if delay > remaining {
			delay = remaining
		}
		if delay <= 0 {
			break
		}
		time.Sleep(delay)
		delay = min(delay*2, 10*time.Second)
	}

	return "UNKNOWN", nil
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
