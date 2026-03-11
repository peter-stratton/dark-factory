package github

import (
	"fmt"
	"strconv"
	"strings"
)

// CreateRollupPR opens a pull request from baseBranch into defaultBranch and
// returns the PR number and URL. The PR is created with the given title and
// body. An error is returned when gh pr create fails or the URL cannot be
// parsed.
func CreateRollupPR(repo, baseBranch, defaultBranch, title, body string) (int, string, error) {
	out, err := CommandRunner("gh", "pr", "create",
		"--repo", repo,
		"--head", baseBranch,
		"--base", defaultBranch,
		"--title", title,
		"--body", body,
	)
	if err != nil {
		return 0, "", fmt.Errorf("gh pr create: %w", err)
	}

	url := strings.TrimSpace(string(out))
	prNum, err := parsePRNumber(url)
	if err != nil {
		return 0, url, fmt.Errorf("parsing PR number from %q: %w", url, err)
	}
	return prNum, url, nil
}

// MergeRollupPR merges the given PR with squash. An error is returned when
// gh pr merge fails.
func MergeRollupPR(repo string, prNum int) error {
	_, err := CommandRunner("gh", "pr", "merge",
		strconv.Itoa(prNum),
		"--repo", repo,
		"--squash",
		"--auto",
	)
	if err != nil {
		return fmt.Errorf("gh pr merge: %w", err)
	}
	return nil
}

// parsePRNumber extracts the pull request number from a GitHub PR URL of the
// form https://github.com/{owner}/{repo}/pull/{number}.
func parsePRNumber(url string) (int, error) {
	// Strip trailing slash if present.
	url = strings.TrimRight(url, "/")
	idx := strings.LastIndex(url, "/")
	if idx == -1 {
		return 0, fmt.Errorf("no slash found in URL %q", url)
	}
	numStr := url[idx+1:]
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("non-numeric PR number %q: %w", numStr, err)
	}
	return n, nil
}
