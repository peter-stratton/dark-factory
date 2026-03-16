package github

import (
	"encoding/json"
	"fmt"
	"time"
)

// PRInfo holds the fields returned by gh pr list for a single pull request.
type PRInfo struct {
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
}

// WatchedPR holds the richer fields needed to render the watch dashboard page.
type WatchedPR struct {
	Number    int
	Title     string
	Labels    []string
	UpdatedAt time.Time
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

// ListWatchedPRs returns open pull requests in repo that carry the given label,
// fetching the richer field set needed for the watch dashboard page.
func ListWatchedPRs(repo, label string) ([]WatchedPR, error) {
	out, err := CommandRunner("gh", "pr", "list",
		"--repo", repo,
		"--label", label,
		"--json", "number,title,labels,updatedAt",
		"--limit", "200",
	)
	if err != nil {
		return nil, fmt.Errorf("listing watched PRs with label %q: %w", label, err)
	}

	var raw []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		UpdatedAt time.Time `json:"updatedAt"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing watched pr list: %w", err)
	}

	prs := make([]WatchedPR, len(raw))
	for i, r := range raw {
		labels := make([]string, len(r.Labels))
		for j, l := range r.Labels {
			labels[j] = l.Name
		}
		prs[i] = WatchedPR{
			Number:    r.Number,
			Title:     r.Title,
			Labels:    labels,
			UpdatedAt: r.UpdatedAt,
		}
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

// FetchReviewComments returns the body text of all inline comments on a
// specific pull request review identified by reviewID.
func FetchReviewComments(repo string, prNum int, reviewID int) ([]string, error) {
	out, err := CommandRunner("gh", "api",
		fmt.Sprintf("repos/%s/pulls/%d/reviews/%d/comments", repo, prNum, reviewID),
	)
	if err != nil {
		return nil, fmt.Errorf("fetching review comments for PR #%d review #%d: %w", prNum, reviewID, err)
	}

	var raw []struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing review comments for PR #%d: %w", prNum, err)
	}

	bodies := make([]string, 0, len(raw))
	for _, c := range raw {
		if c.Body != "" {
			bodies = append(bodies, c.Body)
		}
	}
	return bodies, nil
}
