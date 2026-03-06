package github

import (
	"encoding/json"
	"fmt"
)

// FetchPRCommentBodies returns the body text of all comments on the given PR.
func FetchPRCommentBodies(repo string, prNum int) ([]string, error) {
	out, err := CommandRunner("gh", "pr", "view",
		fmt.Sprintf("%d", prNum),
		"--repo", repo,
		"--json", "comments",
	)
	if err != nil {
		return nil, fmt.Errorf("gh pr view: %w", err)
	}

	var pr struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}

	bodies := make([]string, len(pr.Comments))
	for i, c := range pr.Comments {
		bodies[i] = c.Body
	}
	return bodies, nil
}
