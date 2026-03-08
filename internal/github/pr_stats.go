package github

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FetchPRStats fetches the total additions, deletions, and changed file count
// for the given pull request. Uses `gh pr view --json additions,deletions,changedFiles`.
func FetchPRStats(repo string, prNum int) (additions, deletions, fileCount int, err error) {
	out, err := CommandRunner("gh", "pr", "view",
		fmt.Sprintf("%d", prNum),
		"--repo", repo,
		"--json", "additions,deletions,changedFiles",
	)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("gh pr view: %w", err)
	}

	var pr struct {
		Additions    int `json:"additions"`
		Deletions    int `json:"deletions"`
		ChangedFiles int `json:"changedFiles"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return 0, 0, 0, fmt.Errorf("parsing gh output: %w", err)
	}

	return pr.Additions, pr.Deletions, pr.ChangedFiles, nil
}

// FetchPRChangedFiles returns the list of file paths modified by the given pull request.
// Uses `gh pr diff --name-only`.
func FetchPRChangedFiles(repo string, prNum int) ([]string, error) {
	out, err := CommandRunner("gh", "pr", "diff",
		fmt.Sprintf("%d", prNum),
		"--repo", repo,
		"--name-only",
	)
	if err != nil {
		return nil, fmt.Errorf("gh pr diff: %w", err)
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
