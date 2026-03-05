package dashboard

import (
	"fmt"
	"net/http"
	"sort"
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		s.cfg.Logger.Error("rendering index template", "error", err)
	}
}

func (s *Server) handleRunsTable(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	runs, err := s.filteredRuns(repo)
	if err != nil {
		s.cfg.Logger.Error("building run rows", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "runs-rows", runs); err != nil {
		s.cfg.Logger.Error("rendering runs-rows template", "error", err)
	}
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

	runs, err := s.filteredRuns(repoFilter)
	if err != nil {
		return nil, err
	}

	return &IndexData{
		Runs:       runs,
		Repos:      repos,
		RepoFilter: repoFilter,
	}, nil
}

func (s *Server) filteredRuns(repoFilter string) ([]RunView, error) {
	metas, err := s.reader.ListRuns()
	if err != nil {
		return nil, err
	}
	views := make([]RunView, 0, len(metas))
	for _, m := range metas {
		if repoFilter != "" && m.Repo != repoFilter {
			continue
		}
		views = append(views, metaToView(m))
	}
	return views, nil
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
