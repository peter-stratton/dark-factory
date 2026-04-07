package dashboard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/peter-stratton/dark-factory/internal/analysis"
	"github.com/peter-stratton/dark-factory/internal/label"
	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/stats"
)

// writePartial renders a partial HTML fragment with ETag support. If the
// client sends an If-None-Match header matching the content hash, a 304 Not
// Modified is returned and no body is written.
func (s *Server) writePartial(w http.ResponseWriter, r *http.Request, buf *bytes.Buffer) {
	hash := sha256.Sum256(buf.Bytes())
	etag := `"` + hex.EncodeToString(hash[:8]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = buf.WriteTo(w)
}

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
	DeleteURL     string // URL for the DELETE endpoint (same value as URL)
	AwaitingCount int    // count of PRs awaiting human review (ready-to-merge)
}

// RunDetailData is the data passed to the run-detail template.
type RunDetailData struct {
	Owner             string
	Repo              string
	Timestamp         string
	Meta              rundata.RunMeta
	Issues            []IssueRowView
	AwaitingReview    []IssueRowView // issues with ready-to-merge status
	RunURL            string         // canonical URL for this run detail page
	HoldResetsAt      *time.Time     // non-nil when the run is paused by a Claude usage limit
	Waves             []WaveView     // non-nil when concurrent waves exist
	ConcurrencySaved  string         // e.g. "4m0s"; empty for serial runs
}

// WaveView is the view model for one wave in the run detail.
type WaveView struct {
	Wave     int
	Issues   []IssueRowView
	Duration string // formatted wave wall-clock duration
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
	AwaitingHuman         bool   // true when outcome status is "ready-to-merge"
	ErrorReason           string // failure reason from outcome, shown inline for failed issues
	HasJudgeInterventions bool   // true when judge intervened at least once
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
	TraceID         string // trace ID from outcome; empty for old runs
	CloneSHA        string // HEAD SHA captured inside the container after clone; empty for old runs
	Timeline        []TimelineStepView
	Punchlist           *rundata.PunchlistData
	Dialogue            []rundata.DialogueEntry
	FailureAnalysis     *rundata.FailureAnalysis
	JudgeInterventions  []rundata.JudgeIntervention
	SpecDelta           *rundata.SpecDeltaData
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
	PeakMemory     string // formatted peak memory, e.g. "123.4 MB" or "—"
	CPUTime        string // formatted CPU time, e.g. "1.23s" or "—"
	Verdict        string // "Passed", "Failed", "Flagged", "Error", "—"
	VerdictClass   string // badge class suffix
	Flags          []rundata.Flag
	HasOutput      bool
	Output         string
	ToolTrace      []string           // tool calls recorded during this step
	CheckSummaries []CheckSummaryView // per-check results for verify steps; nil for other steps
	TraceID        string             // trace ID for this step; empty for old runs
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
	s.writePartial(w, r, &buf)
}

func (s *Server) buildIndexData(repoFilter string) (*IndexData, error) {
	allMetas, err := s.reader.ListRuns()
	if err != nil {
		return nil, err
	}

	// Exclude test runs before building repo set, run list, and analytics.
	filtered := allMetas[:0]
	for _, m := range allMetas {
		if !isTestRun(m) {
			filtered = append(filtered, m)
		}
	}
	allMetas = filtered

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

// isTestRun returns true for runs created during testing that should be
// excluded from the dashboard and analytics (test-data sentinels).
func isTestRun(m rundata.RunMeta) bool {
	return m.Repo == "owner/repo" || m.Milestone == "test-milestone"
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
	v.DeleteURL = v.URL
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
		Owner:        owner,
		Repo:         repo,
		Timestamp:    timestamp,
		Meta:         detail.RunMeta,
		RunURL:       runURL,
		HoldResetsAt: detail.RateLimitResetsAt,
	}
	for _, issue := range detail.Issues {
		row := issueToRowView(issue, owner, repo, timestamp)
		data.Issues = append(data.Issues, row)
		if row.AwaitingHuman {
			data.AwaitingReview = append(data.AwaitingReview, row)
		}
	}

	if len(detail.Waves) > 1 {
		rowByIssue := make(map[int]IssueRowView, len(data.Issues))
		for _, row := range data.Issues {
			rowByIssue[row.IssueNumber] = row
		}
		for _, w := range detail.Waves {
			wv := WaveView{
				Wave:     w.Wave,
				Duration: formatDuration(w.FinishedAt.Sub(w.StartedAt).Seconds()),
			}
			for _, n := range w.IssueNumbers {
				if row, ok := rowByIssue[n]; ok {
					wv.Issues = append(wv.Issues, row)
				}
			}
			data.Waves = append(data.Waves, wv)
		}
		data.ConcurrencySaved = computeConcurrencySaved(detail)
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
	s.writePartial(w, r, &buf)
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
		TraceID:         found.Outcome.TraceID,
		CloneSHA:        found.Outcome.CloneSHA,
		Timeline:           buildTimeline(*found),
		Punchlist:          found.Punchlist,
		Dialogue:           found.Dialogue,
		FailureAnalysis:    found.FailureAnalysis,
		JudgeInterventions: found.JudgeInterventions,
		SpecDelta:          found.SpecDelta,
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
	s.writePartial(w, r, &buf)
}

// issueToRowView converts an IssueDetail to the view model for the run detail table.
func issueToRowView(issue rundata.IssueDetail, owner, repo, timestamp string) IssueRowView {
	flagCount := len(issue.Implement.Flags) + len(issue.QualityReview.Flags) + len(issue.FunctionalReview.Flags)
	totalCost := issue.Recon.CostUSD + issue.Implement.CostUSD + issue.QualityReview.CostUSD + issue.FunctionalReview.CostUSD
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
		AwaitingHuman:         issue.Outcome.Status == "ready-to-merge",
		ErrorReason:           issue.Outcome.Error,
		HasJudgeInterventions: len(issue.JudgeInterventions) > 0,
	}
}

// issueStatusLabelAndClass returns the display label and CSS badge class for
// an issue row. Final outcomes take precedence; live status is checked for
// in-progress issues; pending/blocked is derived from dependency info.
func issueStatusLabelAndClass(issue rundata.IssueDetail) (label, class string) {
	// Final outcomes take precedence over live status.
	switch issue.Outcome.Status {
	case rundata.OutcomeStatusImplemented:
		return "Implemented", "success"
	case rundata.OutcomeStatusReadyToMerge:
		return "Ready to Merge", "success"
	case rundata.OutcomeStatusNeedsHumanReview:
		return "Needs Human Review", "warning"
	case rundata.OutcomeStatusFailed:
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
	if hasStepData(issue.Recon) {
		steps = append(steps, stepToView("Recon", issue.Recon))
	}
	if hasStepData(issue.Planner) {
		steps = append(steps, stepToView("Planner", issue.Planner))
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
	if hasStepData(issue.MergeCoordinator) {
		steps = append(steps, stepToView("Merge Coordinate", issue.MergeCoordinator))
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

	var approved, changesRequested bool
	for _, line := range strings.Split(step.Output, "\n") {
		switch strings.TrimSpace(line) {
		case "AGENT_RESULT=APPROVED":
			approved = true
		case "AGENT_RESULT=CHANGES_REQUESTED":
			changesRequested = true
		}
	}

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
		PeakMemory:   formatMemoryMB(step.PeakMemoryBytes),
		CPUTime:      formatCPUSecs(step.CPUNanoseconds),
		Verdict:      verdict,
		VerdictClass: verdictClass,
		Flags:        step.Flags,
		HasOutput:    step.Output != "",
		Output:       step.Output,
		ToolTrace:    step.ToolTrace,
		TraceID:      step.TraceID,
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
		PeakMemory:     "—",
		CPUTime:        "—",
		Verdict:        verdict,
		VerdictClass:   verdictClass,
		HasOutput:      output != "",
		Output:         output,
		CheckSummaries: checkSummaries,
		TraceID:        vr.TraceID,
	}
}

// computeConcurrencySaved calculates wall-clock time saved by running issues
// concurrently. Returns a formatted duration string, or empty if no savings.
func computeConcurrencySaved(detail *rundata.RunDetail) string {
	if detail.FinishedAt == nil {
		return ""
	}
	wallSecs := detail.FinishedAt.Sub(detail.StartedAt).Seconds()
	var serialSecs float64
	for _, issue := range detail.Issues {
		serialSecs += issue.Implement.DurationSeconds + issue.QualityReview.DurationSeconds + issue.FunctionalReview.DurationSeconds
		for _, retry := range issue.Retries {
			serialSecs += retry.Retry.DurationSeconds + retry.QualityReview.DurationSeconds + retry.FunctionalReview.DurationSeconds
		}
	}
	saved := serialSecs - wallSecs
	if saved <= 0 {
		return ""
	}
	return formatDuration(saved)
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

// formatMemoryMB formats bytes as MB with 1 decimal place, or "—" for zero.
func formatMemoryMB(bytes int64) string {
	if bytes == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/1048576)
}

// formatCPUSecs formats nanoseconds as seconds with 2 decimal places, or "—" for zero.
func formatCPUSecs(nanos int64) string {
	if nanos == 0 {
		return "—"
	}
	return fmt.Sprintf("%.2fs", float64(nanos)/1e9)
}

// OutcomeRow is one row in the outcome distribution table on the analysis page.
type OutcomeRow struct {
	Status  string
	Count   int
	Percent float64
}

// CostByStepRow is one row in the cost-by-step breakdown table on the analysis page.
type CostByStepRow struct {
	Step    string
	CostUSD float64
	Percent float64
}

// DurationByStepRow is one row in the duration-by-step breakdown table on the analysis page.
type DurationByStepRow struct {
	Step     string
	Seconds  float64
	Duration string // pre-formatted, e.g. "7m00s"
	Percent  float64
}

// VerifyRow is one row in the verify check failures table on the analysis page.
type VerifyRow struct {
	Name       string
	Count      int
	FailurePct float64 // failure count as a percentage of total issues
}

// RepoRow is one row in the per-repo success rate table on the analysis page.
type RepoRow struct {
	Repo               string
	Total              int
	Implemented        int
	Failed             int
	SuccessPct         float64 // success rate as a percentage (0–100)
	FirstPassRate      float64 // first-pass rate as a fraction (0–1)
	AvgCostPerIssueUSD float64
}

// OverviewMetrics holds top-level aggregate metrics for the analysis page.
type OverviewMetrics struct {
	TotalRuns            int
	TotalIssues          int
	SuccessRate          float64 // fraction (0–1)
	FirstPassRate        float64 // fraction (0–1)
	TotalCostUSD         float64
	AvgCostPerSuccessUSD float64
	WastedCostUSD        float64
	TimeoutRate          float64 // fraction (0–1)
}

// FailureReasonRow is one row in the failure reason breakdown table on the analysis page.
type FailureReasonRow struct {
	Reason  string
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
	WithLabel      string
	WithoutLabel   string
}

// AnalysisData is the data passed to the analysis template.
type AnalysisData struct {
	Report         analysis.Report
	Gaps           []GapView
	Outcomes       []OutcomeRow         // sorted by count desc
	CostByStep     []CostByStepRow      // sorted by cost desc
	DurationByStep []DurationByStepRow  // sorted by duration desc
	VerifyRows     []VerifyRow          // sorted by count desc, then name asc
	RepoRows       []RepoRow            // sorted by repo name asc
	Trends         []analysis.TrendPoint
	Repos          []string
	RepoFilter     string
	HasData        bool
	HasTrends      bool             // true when Trends has at least 2 points
	Overview       OverviewMetrics
	FailureReasons []FailureReasonRow // sorted by count desc
	AvgTimeToMerge string            // human-readable duration, e.g. "7m30s"
}

func (s *Server) handleAnalysis(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	data, err := s.buildAnalysisData(r.Context(), repo)
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
	data, err := s.buildAnalysisData(r.Context(), repo)
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
	s.writePartial(w, r, &buf)
}

func (s *Server) buildAnalysisData(ctx context.Context, repoFilter string) (*AnalysisData, error) {
	if s.statsDB != nil {
		return s.buildAnalysisDataFromDB(ctx, repoFilter)
	}
	return s.buildAnalysisDataFromFS(repoFilter)
}

// buildAnalysisDataFromDB queries the SQLite stats database for run details
// and passes them to the analysis functions.
func (s *Server) buildAnalysisDataFromDB(ctx context.Context, repoFilter string) (*AnalysisData, error) {
	// Build repo list for the filter dropdown from all runs in the DB.
	allRuns, err := stats.QueryRuns(ctx, s.statsDB, stats.RunFilter{
		ExcludeRepo:      []string{"owner/repo"},
		ExcludeMilestone: []string{"test-milestone"},
	})
	if err != nil {
		return nil, fmt.Errorf("querying repos from stats db: %w", err)
	}
	repoSet := make(map[string]struct{})
	for _, r := range allRuns {
		repoSet[r.Repo] = struct{}{}
	}
	repos := make([]string, 0, len(repoSet))
	for repo := range repoSet {
		repos = append(repos, repo)
	}
	sort.Strings(repos)

	// Query runs, outcomes, and steps filtered by repo.
	filter := stats.RunFilter{
		Repo:             repoFilter,
		ExcludeRepo:      []string{"owner/repo"},
		ExcludeMilestone: []string{"test-milestone"},
	}
	runRecords, err := stats.QueryRuns(ctx, s.statsDB, filter)
	if err != nil {
		return nil, fmt.Errorf("querying runs from stats db: %w", err)
	}
	outcomeRecords, err := stats.QueryIssueOutcomes(ctx, s.statsDB, filter)
	if err != nil {
		return nil, fmt.Errorf("querying issue outcomes from stats db: %w", err)
	}
	stepRecords, err := stats.QueryStepResults(ctx, s.statsDB, filter)
	if err != nil {
		return nil, fmt.Errorf("querying step results from stats db: %w", err)
	}

	runs := stats.ToRunDetails(runRecords, outcomeRecords, stepRecords)

	report := analysis.Aggregate(runs)
	rawGaps := analysis.DetectGaps(runs)
	trends := analysis.ComputeTrends(runs)
	outcomes := buildOutcomeRows(report)
	costByStep := buildCostByStepRows(report)
	durationByStep := buildDurationByStepRows(report)
	verifyRows := buildVerifyRows(report)
	repoRows := buildRepoRows(report)
	gaps := buildGapViews(rawGaps)
	overview := buildOverviewMetrics(report)
	failureReasons := buildFailureReasonRows(report)

	return &AnalysisData{
		Report:         report,
		Gaps:           gaps,
		Outcomes:       outcomes,
		CostByStep:     costByStep,
		DurationByStep: durationByStep,
		VerifyRows:     verifyRows,
		RepoRows:       repoRows,
		Trends:         trends,
		Repos:          repos,
		RepoFilter:     repoFilter,
		HasData:        len(runs) > 0,
		HasTrends:      len(trends) >= 2,
		Overview:       overview,
		FailureReasons: failureReasons,
		AvgTimeToMerge: formatDuration(report.DurationStats.AvgTimeToMergeSeconds),
	}, nil
}

// buildAnalysisDataFromFS reads run details from the filesystem reader and
// passes them to the analysis functions. Used when no stats DB is available.
func (s *Server) buildAnalysisDataFromFS(repoFilter string) (*AnalysisData, error) {
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
	costByStep := buildCostByStepRows(report)
	durationByStep := buildDurationByStepRows(report)
	verifyRows := buildVerifyRows(report)
	repoRows := buildRepoRows(report)
	gaps := buildGapViews(rawGaps)
	overview := buildOverviewMetrics(report)
	failureReasons := buildFailureReasonRows(report)

	return &AnalysisData{
		Report:         report,
		Gaps:           gaps,
		Outcomes:       outcomes,
		CostByStep:     costByStep,
		DurationByStep: durationByStep,
		VerifyRows:     verifyRows,
		RepoRows:       repoRows,
		Trends:         trends,
		Repos:          repos,
		RepoFilter:     repoFilter,
		HasData:        len(runs) > 0,
		HasTrends:      len(trends) >= 2,
		Overview:       overview,
		FailureReasons: failureReasons,
		AvgTimeToMerge: formatDuration(report.DurationStats.AvgTimeToMergeSeconds),
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
			WithLabel:      g.WithLabel,
			WithoutLabel:   g.WithoutLabel,
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

// buildCostByStepRows converts the CostByStep map in a Report to a sorted slice with
// pre-computed percentages, ready for template rendering.
func buildCostByStepRows(report analysis.Report) []CostByStepRow {
	rows := make([]CostByStepRow, 0, len(report.CostStats.CostByStep))
	for step, cost := range report.CostStats.CostByStep {
		if cost <= 0 {
			continue
		}
		var pct float64
		if report.CostStats.TotalUSD > 0 {
			pct = cost / report.CostStats.TotalUSD * 100
		}
		rows = append(rows, CostByStepRow{
			Step:    step,
			CostUSD: cost,
			Percent: pct,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CostUSD != rows[j].CostUSD {
			return rows[i].CostUSD > rows[j].CostUSD
		}
		return rows[i].Step < rows[j].Step
	})
	return rows
}

// buildDurationByStepRows converts the DurationByStep map in a Report to a sorted slice with
// pre-computed percentages and pre-formatted duration strings, ready for template rendering.
func buildDurationByStepRows(report analysis.Report) []DurationByStepRow {
	rows := make([]DurationByStepRow, 0, len(report.DurationStats.DurationByStep))
	var total float64
	for _, secs := range report.DurationStats.DurationByStep {
		total += secs
	}
	for step, secs := range report.DurationStats.DurationByStep {
		if secs <= 0 {
			continue
		}
		var pct float64
		if total > 0 {
			pct = secs / total * 100
		}
		dur := time.Duration(secs * float64(time.Second))
		h := int(dur.Hours())
		m := int(dur.Minutes()) % 60
		s := int(dur.Seconds()) % 60
		var formatted string
		if h > 0 {
			formatted = fmt.Sprintf("%dh%02dm%02ds", h, m, s)
		} else {
			formatted = fmt.Sprintf("%dm%02ds", m, s)
		}
		rows = append(rows, DurationByStepRow{
			Step:     step,
			Seconds:  secs,
			Duration: formatted,
			Percent:  pct,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Seconds != rows[j].Seconds {
			return rows[i].Seconds > rows[j].Seconds
		}
		return rows[i].Step < rows[j].Step
	})
	return rows
}

// buildVerifyRows converts the VerifyStats map in a Report to a sorted slice with
// pre-computed failure percentages, ready for template rendering.
func buildVerifyRows(report analysis.Report) []VerifyRow {
	rows := make([]VerifyRow, 0, len(report.VerifyStats))
	for name, count := range report.VerifyStats {
		var pct float64
		if report.IssueCount > 0 {
			pct = float64(count) / float64(report.IssueCount) * 100
		}
		rows = append(rows, VerifyRow{
			Name:       name,
			Count:      count,
			FailurePct: pct,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

// buildRepoRows converts the RepoStats map in a Report to a sorted slice with
// pre-computed success percentages, ready for template rendering.
func buildRepoRows(report analysis.Report) []RepoRow {
	rows := make([]RepoRow, 0, len(report.RepoStats))
	for repo, rs := range report.RepoStats {
		rows = append(rows, RepoRow{
			Repo:               repo,
			Total:              rs.Total,
			Implemented:        rs.Implemented,
			Failed:             rs.Failed,
			SuccessPct:         rs.SuccessRate * 100,
			FirstPassRate:      rs.FirstPassRate,
			AvgCostPerIssueUSD: rs.AvgCostPerIssueUSD,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Repo < rows[j].Repo
	})
	return rows
}

// buildOverviewMetrics builds the top-level overview metrics from a Report.
func buildOverviewMetrics(report analysis.Report) OverviewMetrics {
	var successRate float64
	if report.IssueCount > 0 {
		successRate = float64(report.Outcomes["implemented"]) / float64(report.IssueCount)
	}
	return OverviewMetrics{
		TotalRuns:            report.RunCount,
		TotalIssues:          report.IssueCount,
		SuccessRate:          successRate,
		FirstPassRate:        report.FirstPassSuccessRate,
		TotalCostUSD:         report.CostStats.TotalUSD,
		AvgCostPerSuccessUSD: report.AvgCostPerSuccessUSD,
		WastedCostUSD:        report.WastedCostUSD,
		TimeoutRate:          report.TimeoutRate,
	}
}

// buildFailureReasonRows converts the FailureReasons map in a Report to a
// sorted slice with pre-computed percentages, ready for template rendering.
func buildFailureReasonRows(report analysis.Report) []FailureReasonRow {
	rows := make([]FailureReasonRow, 0, len(report.FailureReasons))
	for reason, count := range report.FailureReasons {
		var pct float64
		if report.IssueCount > 0 {
			pct = float64(count) / float64(report.IssueCount) * 100
		}
		rows = append(rows, FailureReasonRow{
			Reason:  reason,
			Count:   count,
			Percent: pct,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Reason > rows[j].Reason
	})
	return rows
}

func (s *Server) handleDeleteRun(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	timestamp := r.PathValue("timestamp")

	// Validate path components (same guards as LoadRun in reader.go).
	for _, part := range []string{owner, repo, timestamp} {
		if part == "" {
			http.Error(w, "invalid path component: must not be empty", http.StatusBadRequest)
			return
		}
		if strings.Contains(part, "..") {
			http.Error(w, fmt.Sprintf("invalid path component: must not contain ..: %q", part), http.StatusBadRequest)
			return
		}
		if strings.ContainsAny(part, `\/`) {
			http.Error(w, fmt.Sprintf("invalid path component: must not contain path separators: %q", part), http.StatusBadRequest)
			return
		}
	}

	runDir := s.reader.RunDir(owner, repo, timestamp)

	if _, err := os.Stat(runDir); err != nil { //nolint:gosec // path components validated above
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		s.cfg.Logger.Error("stat run dir", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := os.RemoveAll(runDir); err != nil { //nolint:gosec // path components validated above
		s.cfg.Logger.Error("removing run dir", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Clean up empty parent directories (repo dir, then owner dir).
	repoDir := s.reader.RunDir(owner, repo, "")
	if entries, err := os.ReadDir(repoDir); err == nil && len(entries) == 0 {
		if err := os.Remove(repoDir); err == nil { //nolint:gosec // path components validated above
			ownerDir := s.reader.RunDir(owner, "", "")
			if entries, err := os.ReadDir(ownerDir); err == nil && len(entries) == 0 {
				_ = os.Remove(ownerDir) //nolint:gosec // path components validated above
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// WatchPRView is the view model for a single PR on the watch status page.
type WatchPRView struct {
	Number       int
	Title        string
	CurrentLabel string // full label name, e.g. "godark:awaiting-human-review"
	BadgeClass   string // CSS badge modifier, e.g. "badge--info"
	BadgeLabel   string // short human-readable label, e.g. "Awaiting Review"
	TimeInState  string // human-readable duration, e.g. "2 hours"
	GitHubURL    string // https://github.com/owner/repo/pull/123
}

// WatchStatusData is the data passed to the watch.html template.
type WatchStatusData struct {
	Repo        string   // currently selected repo, empty if none
	Repos       []string // known repos from run data, for the selector
	PRs         []WatchPRView
	HasQuerier  bool // false when WatchQuerier is not configured
	Error       string
}

func (s *Server) handleWatchStatus(w http.ResponseWriter, r *http.Request) {
	data := s.buildWatchStatusData(r.Context(), r)
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "watch.html", data); err != nil {
		s.cfg.Logger.Error("rendering watch template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func (s *Server) buildWatchStatusData(ctx context.Context, r *http.Request) WatchStatusData {
	repo := r.URL.Query().Get("repo")

	// Collect known repos from run data for the selector.
	repos := s.knownRepos()

	data := WatchStatusData{
		Repo:       repo,
		Repos:      repos,
		HasQuerier: s.cfg.WatchQuerier != nil,
	}

	if s.cfg.WatchQuerier == nil || repo == "" {
		return data
	}

	// Query both watched labels and merge results by PR number.
	byNumber := make(map[int]WatchedPRInfo)
	for _, lbl := range []string{label.AwaitingHumanReview, label.FixingReviewFeedback} {
		prs, err := s.cfg.WatchQuerier.ListWatchedPRs(ctx, repo, lbl)
		if err != nil {
			s.cfg.Logger.Error("listing watched PRs", "repo", repo, "label", lbl, "error", err)
			data.Error = fmt.Sprintf("Failed to fetch PRs: %v", err)
			return data
		}
		for _, pr := range prs {
			byNumber[pr.Number] = pr
		}
	}

	views := make([]WatchPRView, 0, len(byNumber))
	for _, pr := range byNumber {
		views = append(views, watchPRToView(pr, repo))
	}
	sort.Slice(views, func(i, j int) bool {
		return views[i].Number < views[j].Number
	})
	data.PRs = views
	return data
}

// knownRepos returns the unique repos seen in run data, sorted.
func (s *Server) knownRepos() []string {
	metas, err := s.reader.ListRuns()
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	for _, m := range metas {
		seen[m.Repo] = struct{}{}
	}
	repos := make([]string, 0, len(seen))
	for repo := range seen {
		repos = append(repos, repo)
	}
	sort.Strings(repos)
	return repos
}

// watchPRToView converts a WatchedPRInfo into a WatchPRView for rendering.
func watchPRToView(pr WatchedPRInfo, repo string) WatchPRView {
	currentLabel, badgeClass, badgeLabel := watchedPRBadge(pr.Labels)
	return WatchPRView{
		Number:       pr.Number,
		Title:        pr.Title,
		CurrentLabel: currentLabel,
		BadgeClass:   badgeClass,
		BadgeLabel:   badgeLabel,
		TimeInState:  humanizeAge(pr.UpdatedAt),
		GitHubURL:    fmt.Sprintf("https://github.com/%s/pull/%d", repo, pr.Number),
	}
}

// watchedPRBadge selects the most relevant lifecycle label from a PR's label
// list and returns the label name, badge CSS class, and display text.
func watchedPRBadge(labels []string) (name, class, display string) {
	for _, l := range labels {
		switch l {
		case label.FixingReviewFeedback:
			return l, "badge--warning", "Fixing Feedback"
		case label.AwaitingHumanReview:
			return l, "badge--info", "Awaiting Review"
		}
	}
	return "", "badge--secondary", "Unknown"
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
