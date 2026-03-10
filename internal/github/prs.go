package github

import (
	"encoding/json"
	"fmt"
)

// FetchPRCommentBodies returns the PR body followed by the body text of all
// comments on the given PR. The PR body is included first because the
// implementer agent may embed its Implementation Notes in the PR description
// rather than posting a separate comment.
func FetchPRCommentBodies(repo string, prNum int) ([]string, error) {
	out, err := CommandRunner("gh", "pr", "view",
		fmt.Sprintf("%d", prNum),
		"--repo", repo,
		"--json", "body,comments",
	)
	if err != nil {
		return nil, fmt.Errorf("gh pr view: %w", err)
	}

	var pr struct {
		Body     string `json:"body"`
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, fmt.Errorf("parsing gh output: %w", err)
	}

	// PR body first (may contain Implementation Notes), then comments in order.
	bodies := make([]string, 0, 1+len(pr.Comments))
	if pr.Body != "" {
		bodies = append(bodies, pr.Body)
	}
	for _, c := range pr.Comments {
		bodies = append(bodies, c.Body)
	}
	return bodies, nil
}
