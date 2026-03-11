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

	"github.com/phs/dark-factory/internal/analysis"
	"github.com/phs/dark-factory/internal/rundata"
)

// RunView is the view model for a single run in the list.
type RunView struct {
	Repo          string
	Milestone     string
	IssueCount    int
	Passed        int
	Failed        int
	PassPct       int    // 0–100, for the progress bar
	FailPct       int    // 0–100, for the progress bar
	StatusClass   string // "success", "danger", or "info"
	StatusLabel   string // "Passed", "Failed", or "Running"
	When          string // human-readable relative time
	StartedAt     time.Time
	URL           string // link to run detail page, e.g. /runs/owner/repo/20260305-123456
	AwaitingCount int    // count of PRs awaiting human review (ready-to-merge)
}

// RunDetailData is the data passed to the run-detail template.
type RunDetailData struct {
	Owner          string
	Repo           string
	Timestamp      string
	Meta           rundata.RunMeta
	Issues         []IssueRowView
	AwaitingReview []IssueRowView // issues with ready-to-merge status
	RunURL         string         // canonical URL for this run detail page
}

// IssueRowView is the view model for one issue row in the run detail table.
type IssueRowView struct {
	IssueNumber   int
	Title         string // issue title, e.g. "Architecture layer parser"
	Status        string // "Implemented", "Failed", "Running"
	StatusClass   string // "success", "danger", "info"
	PRNumber      int
	PRLink        string // GitHub PR URL, empty if no PR
	RetryCount    int
	FlagCount     int    // total quality flags across all steps
	Cost          string // formatted total cost, e.g. "$0.0042" or "—"
	URL           string // link to issue detail page
	AwaitingHuman bool   // true when outcome status is "ready-to-merge"
	ErrorReason   string // failure reason from outcome, shown inline for failed issues
}

// IssueDetailData is the data passed to the issue-detail template.
type IssueDetailData struct {
	Owner           string
	Repo            string
	Timestamp       string
	IssueNumber     int
	Title           string
	Description     string
	Status          string // outcome status, e.g. "failed", "implemented"
	PRNumber        int
	PRLink          string
	IssueLink       string
	RunURL          string // link back to run detail page
	ReviewChainURL  string // URL for the review chain polling partial
	ErrorReason     string // failure reason from outcome, shown prominently for failed issues
	Timeline        []TimelineStepView
	Punchlist       *rundata.PunchlistData
	Dialogue        []rundata.DialogueEntry
	FailureAnalysis *rundata.FailureAnalysis
}

// CheckSummaryView is the view model for one check within a verify step summary.
type CheckSummaryView struct {
	Name   string
	Passed bool
}

// TimelineStepView is the view model for one step in the issue timeline.
type TimelineStepView struct {
	Name           string
	MarkerClass    string // "success", "danger", "warning", "info", "neutral"
	Duration       string // formatted duration, e.g. "42s" or "—"
	Cost           string // formatted cost, e.g. "$0.0042" or "—"
	Verdict        string // "Passed", "Failed", "Flagged", "Error", "—"
	VerdictClass   string // badge class suffix
	Flags          []rundata.Flag
	HasOutput      bool
	Output         string
	ToolTrace      []string           // tool calls recorded during this step
	CheckSummaries []CheckSummaryView // per-check results for verify steps; nil for other steps
}

// IndexData is the data passed to the index template.
type IndexData struct {
	Runs           []RunView
	Repos          []string // unique repo names, sorted
	RepoFilter     string   // currently active repo filter (empty = all)
	Summary        *analysis.Report
	Trends         []analysis.TrendPoint
	HasSummary     bool
	HasTrends      bool
	SuccessRatePct float64 // pre-computed for the summary card
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
	awaitingCounts := s.loadAwaitingCounts(metas)
	runs := filteredRuns(metas, repo, awaitingCounts)
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

	awaitingCounts := s.loadAwaitingCounts(allMetas)
	runs := filteredRuns(allMetas, repoFilter, awaitingCounts)

	// Load up to 50 most recent full run details for the analytics summary.
	const maxSummaryRuns = 50
	summaryMetas := allMetas
	if len(summaryMetas) > maxSummaryRuns {
		summaryMetas = summaryMetas[:maxSummaryRuns]
	}
	var fullRuns []rundata.RunDetail
	for _, m := range summaryMetas {
		parts := strings.SplitN(m.Repo, "/", 2)
		if len(parts) != 2 {
			continue
		}
		timestamp := m.StartedAt.UTC().Format("20060102-150405")
		detail, err := s.reader.LoadRun(parts[0], parts[1], timestamp)
		if err != nil {
			s.cfg.Logger.Warn("loading run detail for summary", "repo", m.Repo, "error", err)
			continue
		}
		fullRuns = append(fullRuns, *detail)
	}

	report := analysis.Aggregate(fullRuns)
	trends := analysis.ComputeTrends(fullRuns)

	var successRatePct float64
	if report.IssueCount > 0 {
		successRatePct = float64(report.Outcomes["implemented"]) / float64(report.IssueCount) * 100
	}

	return &IndexData{
		Runs:           runs,
		Repos:          repos,
		RepoFilter:     repoFilter,
		Summary:        &report,
		Trends:         trends,
		HasSummary:     len(fullRuns) > 0,
		HasTrends:      len(trends) >= 2,
		SuccessRatePct: successRatePct,
	}, nil
}

func filteredRuns(metas []rundata.RunMeta, repoFilter string, awaitingCounts map[string]int) []RunView {
	views := make([]RunView, 0, len(metas))
	for _, m := range metas {
		if repoFilter != "" && m.Repo != repoFilter {
			continue
		}
		v := metaToView(m)
		v.AwaitingCount = awaitingCounts[v.URL]
		views = append(views, v)
	}
	return views
}

// loadAwaitingCounts loads full run details for up to maxSummaryRuns metas and
// returns a map from run detail URL to the count of issues with ready-to-merge status.
func (s *Server) loadAwaitingCounts(metas []rundata.RunMeta) map[string]int {
	const maxSummaryRuns = 50
	if len(metas) > maxSummaryRuns {
		metas = metas[:maxSummaryRuns]
	}
	counts := make(map[string]int)
	for _, m := range metas {
		parts := strings.SplitN(m.Repo, "/", 2)
		if len(parts) != 2 {
			continue
		}
		timestamp := m.StartedAt.UTC().Format("20060102-150405")
		detail, err := s.reader.LoadRun(parts[0], parts[1], timestamp)
		if err != nil {
			s.cfg.Logger.Warn("loading run detail for awaiting counts", "repo", m.Repo, "error", err)
			continue
		}
		count := 0
		for _, issue := range detail.Issues {
			if issue.Outcome.Status == "ready-to-merge" {
				count++
			}
		}
		url := runDetailURL(m.Repo, m.StartedAt)
		counts[url] = count
	}
	return counts
}

func metaToView(m rundata.RunMeta) RunView {
	milestone := m.Milestone
	if milestone == "" && len(m.IssueNumbers) > 0 {
		labels := make([]string, len(m.IssueNumbers))
		for i, n := range m.IssueNumbers {
			labels[i] = fmt.Sprintf("#%d", n)
		}
		milestone = "Issue " + strings.Join(labels, ", ")
		if len(m.IssueNumbers) > 1 {
			milestone = "Issues " + strings.Join(labels, ", ")
		}
	}
	v := RunView{
		Repo:        m.Repo,
		Milestone:   milestone,
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
		row := issueToRowView(issue, owner, repo, timestamp)
		data.Issues = append(data.Issues, row)
		if row.AwaitingHuman {
			data.AwaitingReview = append(data.AwaitingReview, row)
		}
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
	filter := r.URL.Query().Get("filter")

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
		row := issueToRowView(issue, owner, repo, timestamp)
		if filter == "awaiting_human" && !row.AwaitingHuman {
			continue
		}
		rows = append(rows, row)
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

	reviewChainURL := fmt.Sprintf("/runs/%s/%s/%s/issues/%d/review-chain", owner, repo, timestamp, issueNum)
	data := IssueDetailData{
		Owner:           owner,
		Repo:            repo,
		Timestamp:       timestamp,
		IssueNumber:     issueNum,
		Title:           found.Outcome.Title,
		Description:     found.Outcome.Description,
		Status:          found.Outcome.Status,
		PRNumber:        found.Outcome.PRNumber,
		PRLink:          prLink,
		IssueLink:       issueLink,
		RunURL:          runURL,
		ReviewChainURL:  reviewChainURL,
		ErrorReason:     found.Outcome.Error,
		Timeline:        buildTimeline(*found),
		Punchlist:       found.Punchlist,
		Dialogue:        found.Dialogue,
		FailureAnalysis: found.FailureAnalysis,
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

func (s *Server) handleReviewChain(w http.ResponseWriter, r *http.Request) {
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
		s.cfg.Logger.Error("loading run for review chain", "error", err)
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

	timeline := buildTimeline(*found)
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "review-chain-steps", timeline); err != nil {
		s.cfg.Logger.Error("rendering review-chain-steps template", "error", err)
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
		flagCount += len(retry.Retry.Flags) + len(retry.QualityReview.Flags) + len(retry.FunctionalReview.Flags)
		totalCost += retry.Retry.CostUSD + retry.QualityReview.CostUSD + retry.FunctionalReview.CostUSD
	}

	statusLabel, statusClass := issueStatusLabelAndClass(issue)

	prLink := ""
	if issue.Outcome.PRNumber > 0 {
		prLink = fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, issue.Outcome.PRNumber)
	}

	issueURL := fmt.Sprintf("/runs/%s/%s/%s/issues/%d", owner, repo, timestamp, issue.IssueNumber)

	return IssueRowView{
		IssueNumber:   issue.IssueNumber,
		Title:         issue.Outcome.Title,
		Status:        statusLabel,
		StatusClass:   statusClass,
		PRNumber:      issue.Outcome.PRNumber,
		PRLink:        prLink,
		RetryCount:    len(issue.Retries),
		FlagCount:     flagCount,
		Cost:          formatCost(totalCost),
		URL:           issueURL,
		AwaitingHuman: issue.Outcome.Status == "ready-to-merge",
		ErrorReason:   issue.Outcome.Error,
	}
}

// issueStatusLabelAndClass returns the display label and CSS badge class for
// an issue row. Final outcomes take precedence; live status is checked for
// in-progress issues; pending/blocked is derived from dependency info.
func issueStatusLabelAndClass(issue rundata.IssueDetail) (label, class string) {
	// Final outcomes take precedence over live status.
	switch issue.Outcome.Status {
	case "implemented":
		return "Implemented", "success"
	case "ready-to-merge":
		return "Ready to Merge", "success"
	case "needs-human-review":
		return "Needs Human Review", "warning"
	case "failed":
		return "Failed", "danger"
	}

	// No final outcome — check live status file for in-progress issues.
	switch issue.LiveStatus.Status {
	case "implementing":
		return "Implementing", "info"
	case "in_review":
		return "In Review", "info"
	}

	// No live status — derive pending vs blocked from dependency info.
	if len(issue.BlockedBy) > 0 {
		return "Blocked", "warning"
	}
	return "Pending", "secondary"
}

// buildTimeline constructs the ordered list of timeline steps for one issue.
// Steps with no recorded data (no output, no error, no timing) are omitted.
func buildTimeline(issue rundata.IssueDetail) []TimelineStepView {
	var steps []TimelineStepView

	if hasStepData(issue.SpecGenerator) {
		steps = append(steps, stepToView("Spec Generator", issue.SpecGenerator))
	}
	if hasStepData(issue.Implement) {
		steps = append(steps, stepToView("Implement", issue.Implement))
	}
	for _, vr := range issue.VerifyResults {
		steps = append(steps, verifyToTimelineView(vr))
	}
	if hasStepData(issue.QualityReview) {
		steps = append(steps, stepToView("Quality Review", issue.QualityReview))
	}
	for _, retry := range issue.Retries {
		if hasStepData(retry.FunctionalReview) {
			steps = append(steps, stepToView("Functional Review", retry.FunctionalReview))
		}
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
		ToolTrace:    step.ToolTrace,
	}
}

// verifyToTimelineView converts a VerifyStepResult to a TimelineStepView.
func verifyToTimelineView(vr rundata.VerifyStepResult) TimelineStepView {
	name := "Verify"
	if vr.FixAttempted {
		name = fmt.Sprintf("Verify (fix attempt %d)", vr.Attempt)
	}

	verdictClass := "success"
	markerClass := "success"
	verdict := "Passed"
	if !vr.AllPassed {
		verdictClass = "danger"
		markerClass = "danger"
		verdict = "Failed"
	}

	// Build per-check summaries for the timeline header.
	var checkSummaries []CheckSummaryView
	for _, c := range vr.Checks {
		checkSummaries = append(checkSummaries, CheckSummaryView{
			Name:   c.Name,
			Passed: c.Passed,
		})
	}

	// Build output from check results for the expandable detail.
	var sb strings.Builder
	for _, c := range vr.Checks {
		status := "PASS"
		if !c.Passed {
			status = "FAIL"
		}
		fmt.Fprintf(&sb, "**%s**: %s (exit code %d)\n", c.Name, status, c.ExitCode)
		if c.Output != "" && !c.Passed {
			sb.WriteString("```\n")
			sb.WriteString(c.Output)
			if !strings.HasSuffix(c.Output, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("```\n")
		}
	}
	output := sb.String()

	return TimelineStepView{
		Name:           name,
		MarkerClass:    markerClass,
		Duration:       "—",
		Cost:           "—",
		Verdict:        verdict,
		VerdictClass:   verdictClass,
		HasOutput:      output != "",
		Output:         output,
		CheckSummaries: checkSummaries,
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

// OutcomeRow is one row in the outcome distribution table on the analysis page.
type OutcomeRow struct {
	Status  string
	Count   int
	Percent float64
}

// GapView is the view model for one prompt gap on the analysis page.
type GapView struct {
	Finding        string
	FailPctWith    float64 // fail rate as a percentage (0–100)
	FailPctWithout float64
	SamplesWith    int
	SamplesWithout int
}

// AnalysisData is the data passed to the analysis template.
type AnalysisData struct {
	Report     analysis.Report
	Gaps       []GapView
	Outcomes   []OutcomeRow // sorted by count desc
	Trends     []analysis.TrendPoint
	Repos      []string
	RepoFilter string
	HasData    bool
	HasTrends  bool // true when Trends has at least 2 points
}

func (s *Server) handleAnalysis(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	data, err := s.buildAnalysisData(repo)
	if err != nil {
		s.cfg.Logger.Error("building analysis data", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "analysis.html", data); err != nil {
		s.cfg.Logger.Error("rendering analysis template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func (s *Server) handleAnalysisStats(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	data, err := s.buildAnalysisData(repo)
	if err != nil {
		s.cfg.Logger.Error("building analysis stats", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "analysis-stats", data); err != nil {
		s.cfg.Logger.Error("rendering analysis-stats template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func (s *Server) buildAnalysisData(repoFilter string) (*AnalysisData, error) {
	allMetas, err := s.reader.ListRuns()
	if err != nil {
		return nil, err
	}

	// Build repo list for the filter dropdown.
	repoSet := make(map[string]struct{})
	for _, m := range allMetas {
		repoSet[m.Repo] = struct{}{}
	}
	repos := make([]string, 0, len(repoSet))
	for repo := range repoSet {
		repos = append(repos, repo)
	}
	sort.Strings(repos)

	// Filter metas by repo if requested.
	var filteredMetas []rundata.RunMeta
	for _, m := range allMetas {
		if repoFilter != "" && m.Repo != repoFilter {
			continue
		}
		filteredMetas = append(filteredMetas, m)
	}

	// Load full run details for each filtered meta.
	var runs []rundata.RunDetail
	for _, m := range filteredMetas {
		parts := strings.SplitN(m.Repo, "/", 2)
		if len(parts) != 2 {
			continue
		}
		timestamp := m.StartedAt.UTC().Format("20060102-150405")
		detail, err := s.reader.LoadRun(parts[0], parts[1], timestamp)
		if err != nil {
			s.cfg.Logger.Warn("loading run detail for analysis", "repo", m.Repo, "error", err)
			continue
		}
		runs = append(runs, *detail)
	}

	report := analysis.Aggregate(runs)
	rawGaps := analysis.DetectGaps(runs)
	trends := analysis.ComputeTrends(runs)
	outcomes := buildOutcomeRows(report)
	gaps := buildGapViews(rawGaps)

	return &AnalysisData{
		Report:     report,
		Gaps:       gaps,
		Outcomes:   outcomes,
		Trends:     trends,
		Repos:      repos,
		RepoFilter: repoFilter,
		HasData:    len(runs) > 0,
		HasTrends:  len(trends) >= 2,
	}, nil
}

// buildGapViews converts raw PromptGap slices to GapView with pre-computed percentages.
func buildGapViews(gaps []analysis.PromptGap) []GapView {
	views := make([]GapView, 0, len(gaps))
	for _, g := range gaps {
		views = append(views, GapView{
			Finding:        g.Finding,
			FailPctWith:    g.FailRateWith * 100,
			FailPctWithout: g.FailRateWithout * 100,
			SamplesWith:    g.SamplesWith,
			SamplesWithout: g.SamplesWithout,
		})
	}
	return views
}

// buildOutcomeRows converts the outcome map in a Report to a sorted slice with
// pre-computed percentages, ready for template rendering.
func buildOutcomeRows(report analysis.Report) []OutcomeRow {
	rows := make([]OutcomeRow, 0, len(report.Outcomes))
	for status, count := range report.Outcomes {
		var pct float64
		if report.IssueCount > 0 {
			pct = float64(count) / float64(report.IssueCount) * 100
		}
		rows = append(rows, OutcomeRow{
			Status:  status,
			Count:   count,
			Percent: pct,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Status < rows[j].Status
	})
	return rows
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
