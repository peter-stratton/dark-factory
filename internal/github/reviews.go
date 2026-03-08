package github

import (
	"encoding/json"
	"fmt"
)

// PRInfo holds the fields returned by gh pr list for a single pull request.
type PRInfo struct {
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
}

// PRReview holds the fields from a GitHub pull request review.
type PRReview struct {
	ID     int    `json:"id"`
	State  string `json:"state"`
	Body   string `json:"body"`
	Author string `json:"author"`
}

// ListPRsWithLabel returns open pull requests in repo that carry the given label.
func ListPRsWithLabel(repo, label string) ([]PRInfo, error) {
	out, err := CommandRunner("gh", "pr", "list",
		"--repo", repo,
		"--label", label,
		"--json", "number,headRefName",
		"--limit", "200",
	)
	if err != nil {
		return nil, fmt.Errorf("listing PRs with label %q: %w", label, err)
	}

	var prs []PRInfo
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("parsing pr list: %w", err)
	}
	return prs, nil
}

// FetchPRReviews returns all reviews for the given pull request number in repo.
func FetchPRReviews(repo string, prNum int) ([]PRReview, error) {
	out, err := CommandRunner("gh", "api",
		fmt.Sprintf("repos/%s/pulls/%d/reviews", repo, prNum),
	)
	if err != nil {
		return nil, fmt.Errorf("fetching reviews for PR #%d: %w", prNum, err)
	}

	// The GitHub API nests the reviewer under user.login rather than a flat author field.
	var raw []struct {
		ID    int    `json:"id"`
		State string `json:"state"`
		Body  string `json:"body"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing reviews for PR #%d: %w", prNum, err)
	}

	reviews := make([]PRReview, len(raw))
	for i, r := range raw {
		reviews[i] = PRReview{
			ID:     r.ID,
			State:  r.State,
			Body:   r.Body,
			Author: r.User.Login,
		}
	}
	return reviews, nil
}
