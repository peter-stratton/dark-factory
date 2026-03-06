package dashboard

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
)

// RunView is the view model for a single run in the list.
type RunView struct {
	Repo        string
	Milestone   string
	IssueCount  int
	Passed      int
	Failed      int
	PassPct     int    // 0–100, for the progress bar
	FailPct     int    // 0–100, for the progress bar
	StatusClass string // "success", "danger", or "info"
	StatusLabel string // "Passed", "Failed", or "Running"
	When        string // human-readable relative time
	StartedAt   time.Time
	URL         string // link to run detail page, e.g. /runs/owner/repo/20260305-123456
}

// RunDetailData is the data passed to the run-detail template.
type RunDetailData struct {
	Owner     string
	Repo      string
	Timestamp string
	Meta      rundata.RunMeta
	Issues    []IssueRowView
	RunURL    string // canonical URL for this run detail page
}

// IssueRowView is the view model for one issue row in the run detail table.
type IssueRowView struct {
	IssueNumber int
	Title       string // issue title, e.g. "Architecture layer parser"
	Status      string // "Implemented", "Failed", "Running"
	StatusClass string // "success", "danger", "info"
	PRNumber    int
	PRLink      string // GitHub PR URL, empty if no PR
	RetryCount  int
	FlagCount   int    // total quality flags across all steps
	Cost        string // formatted total cost, e.g. "$0.0042" or "—"
	URL         string // link to issue detail page
}

// IssueDetailData is the data passed to the issue-detail template.
type IssueDetailData struct {
	Owner       string
	Repo        string
	Timestamp   string
	IssueNumber int
	Title       string
	PRNumber    int
	PRLink      string
	IssueLink   string
	RunURL      string // link back to run detail page
	Timeline    []TimelineStepView
	Punchlist   *rundata.PunchlistData
	Dialogue    []rundata.DialogueEntry
}

// TimelineStepView is the view model for one step in the issue timeline.
type TimelineStepView struct {
	Name         string
	MarkerClass  string // "success", "danger", "warning", "info", "neutral"
	Duration     string // formatted duration, e.g. "42s" or "—"
	Cost         string // formatted cost, e.g. "$0.0042" or "—"
	Verdict      string // "Passed", "Failed", "Flagged", "Error", "—"
	VerdictClass string // badge class suffix
	Flags        []rundata.Flag
	HasOutput    bool
	Output       string
}

// IndexData is the data passed to the index template.
type IndexData struct {
	Runs       []RunView
	Repos      []string // unique repo names, sorted
	RepoFilter string   // currently active repo filter (empty = all)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	data, err := s.buildIndexData(repo)
	if err != nil {
		s.cfg.Logger.Error("building index data", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "index.html", data); err != nil {
		s.cfg.Logger.Error("rendering index template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func (s *Server) handleRunsTable(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	metas, err := s.reader.ListRuns()
	if err != nil {
		s.cfg.Logger.Error("building run rows", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	runs := filteredRuns(metas, repo)
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "runs-rows", runs); err != nil {
		s.cfg.Logger.Error("rendering runs-rows template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func (s *Server) buildIndexData(repoFilter string) (*IndexData, error) {
	allMetas, err := s.reader.ListRuns()
	if err != nil {
		return nil, err
	}

	repoSet := make(map[string]struct{})
	for _, m := range allMetas {
		repoSet[m.Repo] = struct{}{}
	}
	repos := make([]string, 0, len(repoSet))
	for repo := range repoSet {
		repos = append(repos, repo)
	}
	sort.Strings(repos)

	runs := filteredRuns(allMetas, repoFilter)

	return &IndexData{
		Runs:       runs,
		Repos:      repos,
		RepoFilter: repoFilter,
	}, nil
}

func filteredRuns(metas []rundata.RunMeta, repoFilter string) []RunView {
	views := make([]RunView, 0, len(metas))
	for _, m := range metas {
		if repoFilter != "" && m.Repo != repoFilter {
			continue
		}
		views = append(views, metaToView(m))
	}
	return views
}

func metaToView(m rundata.RunMeta) RunView {
	v := RunView{
		Repo:        m.Repo,
		Milestone:   m.Milestone,
		IssueCount:  len(m.IssueNumbers),
		StartedAt:   m.StartedAt,
		When:        humanizeAge(m.StartedAt),
		StatusClass: "info",
		StatusLabel: "Running",
		URL:         runDetailURL(m.Repo, m.StartedAt),
	}
	if m.FinishedAt != nil && m.Summary != nil {
		v.Passed = m.Summary.Implemented
		v.Failed = m.Summary.Failed
		total := m.Summary.Total
		if total > 0 {
			v.PassPct = v.Passed * 100 / total
			v.FailPct = v.Failed * 100 / total
		}
		if m.Summary.Failed > 0 {
			v.StatusClass = "danger"
			v.StatusLabel = "Failed"
		} else {
			v.StatusClass = "success"
			v.StatusLabel = "Passed"
		}
	}
	return v
}

// runDetailURL constructs the URL for a run detail page from repo ("owner/name") and start time.
func runDetailURL(repo string, startedAt time.Time) string {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return ""
	}
	timestamp := startedAt.UTC().Format("20060102-150405")
	return fmt.Sprintf("/runs/%s/%s/%s", parts[0], parts[1], timestamp)
}

func (s *Server) handleRunDetail(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	timestamp := r.PathValue("timestamp")

	detail, err := s.reader.LoadRun(owner, repo, timestamp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		s.cfg.Logger.Error("loading run detail", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	runURL := fmt.Sprintf("/runs/%s/%s/%s", owner, repo, timestamp)
	data := RunDetailData{
		Owner:     owner,
		Repo:      repo,
		Timestamp: timestamp,
		Meta:      detail.RunMeta,
		RunURL:    runURL,
	}
	for _, issue := range detail.Issues {
		data.Issues = append(data.Issues, issueToRowView(issue, owner, repo, timestamp))
	}

	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "run-detail.html", data); err != nil {
		s.cfg.Logger.Error("rendering run-detail template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func (s *Server) handleIssuesTable(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	timestamp := r.PathValue("timestamp")

	detail, err := s.reader.LoadRun(owner, repo, timestamp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		s.cfg.Logger.Error("loading run for issues table", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var rows []IssueRowView
	for _, issue := range detail.Issues {
		rows = append(rows, issueToRowView(issue, owner, repo, timestamp))
	}

	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "issues-rows", rows); err != nil {
		s.cfg.Logger.Error("rendering issues-rows template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func (s *Server) handleIssueDetail(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	timestamp := r.PathValue("timestamp")
	numberStr := r.PathValue("number")

	issueNum, err := strconv.Atoi(numberStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	detail, err := s.reader.LoadRun(owner, repo, timestamp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		s.cfg.Logger.Error("loading run for issue detail", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var found *rundata.IssueDetail
	for i := range detail.Issues {
		if detail.Issues[i].IssueNumber == issueNum {
			found = &detail.Issues[i]
			break
		}
	}
	if found == nil {
		http.NotFound(w, r)
		return
	}

	runURL := fmt.Sprintf("/runs/%s/%s/%s", owner, repo, timestamp)
	prLink := ""
	if found.Outcome.PRNumber > 0 {
		prLink = fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, found.Outcome.PRNumber)
	}
	issueLink := fmt.Sprintf("https://github.com/%s/%s/issues/%d", owner, repo, issueNum)

	data := IssueDetailData{
		Owner:       owner,
		Repo:        repo,
		Timestamp:   timestamp,
		IssueNumber: issueNum,
		Title:       found.Outcome.Title,
		PRNumber:    found.Outcome.PRNumber,
		PRLink:      prLink,
		IssueLink:   issueLink,
		RunURL:      runURL,
		Timeline:    buildTimeline(*found),
		Punchlist:   found.Punchlist,
		Dialogue:    found.Dialogue,
	}

	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "issue-detail.html", data); err != nil {
		s.cfg.Logger.Error("rendering issue-detail template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// issueToRowView converts an IssueDetail to the view model for the run detail table.
func issueToRowView(issue rundata.IssueDetail, owner, repo, timestamp string) IssueRowView {
	flagCount := len(issue.Implement.Flags) + len(issue.QualityReview.Flags) + len(issue.FunctionalReview.Flags)
	totalCost := issue.Implement.CostUSD + issue.QualityReview.CostUSD + issue.FunctionalReview.CostUSD
	for _, retry := range issue.Retries {
		flagCount += len(retry.Retry.Flags) + len(retry.QualityReview.Flags)
		totalCost += retry.Retry.CostUSD + retry.QualityReview.CostUSD
	}

	statusLabel := "Running"
	statusClass := "info"
	switch issue.Outcome.Status {
	case "implemented":
		statusLabel = "Implemented"
		statusClass = "success"
	case "failed":
		statusLabel = "Failed"
		statusClass = "danger"
	}

	prLink := ""
	if issue.Outcome.PRNumber > 0 {
		prLink = fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, issue.Outcome.PRNumber)
	}

	issueURL := fmt.Sprintf("/runs/%s/%s/%s/issues/%d", owner, repo, timestamp, issue.IssueNumber)

	return IssueRowView{
		IssueNumber: issue.IssueNumber,
		Title:       issue.Outcome.Title,
		Status:      statusLabel,
		StatusClass: statusClass,
		PRNumber:    issue.Outcome.PRNumber,
		PRLink:      prLink,
		RetryCount:  len(issue.Retries),
		FlagCount:   flagCount,
		Cost:        formatCost(totalCost),
		URL:         issueURL,
	}
}

// buildTimeline constructs the ordered list of timeline steps for one issue.
// Steps with no recorded data (no output, no error, no timing) are omitted.
func buildTimeline(issue rundata.IssueDetail) []TimelineStepView {
	var steps []TimelineStepView

	if hasStepData(issue.Implement) {
		steps = append(steps, stepToView("Implement", issue.Implement))
	}
	if hasStepData(issue.QualityReview) {
		steps = append(steps, stepToView("Quality Review", issue.QualityReview))
	}
	for _, retry := range issue.Retries {
		if hasStepData(retry.Retry) {
			steps = append(steps, stepToView(fmt.Sprintf("Retry %d", retry.Attempt+1), retry.Retry))
		}
		if hasStepData(retry.QualityReview) {
			steps = append(steps, stepToView(fmt.Sprintf("Quality Review (Retry %d)", retry.Attempt+1), retry.QualityReview))
		}
	}
	if hasStepData(issue.FunctionalReview) {
		steps = append(steps, stepToView("Functional Review", issue.FunctionalReview))
	}

	return steps
}

// hasStepData reports whether a StepResult contains any recorded data.
func hasStepData(s rundata.StepResult) bool {
	return s.Output != "" || s.Error != "" || s.StartedAt != nil || s.DurationSeconds > 0
}

// stepToView converts a StepResult to a TimelineStepView.
func stepToView(name string, step rundata.StepResult) TimelineStepView {
	verdict := "—"
	verdictClass := "neutral"
	markerClass := "neutral"

	approved := strings.Contains(step.Output, "QUALITY_RESULT=APPROVED") ||
		strings.Contains(step.Output, "REVIEW_RESULT=APPROVED")
	changesRequested := strings.Contains(step.Output, "QUALITY_RESULT=CHANGES_REQUESTED") ||
		strings.Contains(step.Output, "REVIEW_RESULT=CHANGES_REQUESTED")

	switch {
	case step.Error != "":
		verdict = "Error"
		verdictClass = "danger"
		markerClass = "danger"
	case changesRequested:
		verdict = "Changes Requested"
		verdictClass = "danger"
		markerClass = "danger"
	case approved:
		verdict = "Passed"
		verdictClass = "success"
		markerClass = "success"
	case len(step.Flags) > 0:
		verdict = "Flagged"
		verdictClass = "warning"
		markerClass = "warning"
	case step.Output != "" || step.DurationSeconds > 0:
		verdict = "Passed"
		verdictClass = "success"
		markerClass = "success"
	}

	return TimelineStepView{
		Name:         name,
		MarkerClass:  markerClass,
		Duration:     formatDuration(step.DurationSeconds),
		Cost:         formatCost(step.CostUSD),
		Verdict:      verdict,
		VerdictClass: verdictClass,
		Flags:        step.Flags,
		HasOutput:    step.Output != "",
		Output:       step.Output,
	}
}

// formatCost formats a USD cost as a human-readable string.
func formatCost(usd float64) string {
	if usd == 0 {
		return "—"
	}
	return fmt.Sprintf("$%.4f", usd)
}

// formatDuration formats a duration in seconds as a human-readable string.
func formatDuration(seconds float64) string {
	if seconds == 0 {
		return "—"
	}
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", seconds)
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", mins, secs)
}

func humanizeAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", mins)
	case d < 24*time.Hour:
		hrs := int(d.Hours())
		if hrs == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hrs)
	case d < 48*time.Hour:
		return "yesterday"
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	}
}
