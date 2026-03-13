package github

import (
	"encoding/json"
	"fmt"
	"strings"
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

	// Include the PR body only as a fallback — if any comment already
	// contains "## Implementation Notes", the PR body is redundant and
	// including it would cause the dialogue parser to double-count the
	// implementer's contribution.
	commentHasImplNotes := false
	for _, c := range pr.Comments {
		if strings.Contains(c.Body, "## Implementation Notes") {
			commentHasImplNotes = true
			break
		}
	}

	bodies := make([]string, 0, 1+len(pr.Comments))
	if pr.Body != "" && !commentHasImplNotes {
		bodies = append(bodies, pr.Body)
	}
	for _, c := range pr.Comments {
		bodies = append(bodies, c.Body)
	}
	return bodies, nil
}

// DeleteLastPRCommentWithHeader finds the most recent comment on the given PR
// whose body contains the specified header string and deletes it. Returns nil
// if no matching comment is found (best-effort).
func DeleteLastPRCommentWithHeader(repo string, prNum int, header string) error {
	out, err := CommandRunner("gh", "api",
		fmt.Sprintf("repos/%s/issues/%d/comments", repo, prNum),
		"--jq", ".",
	)
	if err != nil {
		return fmt.Errorf("listing PR comments: %w", err)
	}

	var comments []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(out, &comments); err != nil {
		return fmt.Errorf("parsing PR comments: %w", err)
	}

	// Walk backwards to find the most recent matching comment.
	for i := len(comments) - 1; i >= 0; i-- {
		if strings.Contains(comments[i].Body, header) {
			_, err := CommandRunner("gh", "api",
				"--method", "DELETE",
				fmt.Sprintf("repos/%s/issues/comments/%d", repo, comments[i].ID),
			)
			if err != nil {
				return fmt.Errorf("deleting comment %d: %w", comments[i].ID, err)
			}
			return nil
		}
	}

	return nil // no matching comment found — nothing to delete
}
