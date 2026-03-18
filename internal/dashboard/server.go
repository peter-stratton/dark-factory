package dashboard

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/stats"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed templates static
var content embed.FS

// WatchedPRInfo holds the fields needed to render a single PR on the watch page.
// It is defined here so the presentation layer can remain decoupled from the
// infrastructure layer (internal/github).
type WatchedPRInfo struct {
	Number    int
	Title     string
	Labels    []string
	UpdatedAt time.Time
}

// WatchQuerier retrieves open watched PRs from a repository. It is an
// interface so the presentation layer does not depend on infrastructure.
// The concrete implementation lives in the cmd layer (internal/cmd).
type WatchQuerier interface {
	ListWatchedPRs(ctx context.Context, repo, label string) ([]WatchedPRInfo, error)
}

// Config holds the dashboard server configuration.
type Config struct {
	Port         int
	Logger       *slog.Logger
	WatchQuerier WatchQuerier // optional; if nil, /watch shows a disabled state
}

// Server is the dashboard HTTP server.
type Server struct {
	cfg     Config
	reader  *rundata.Reader
	statsDB *stats.DB
	mux     *http.ServeMux
	tmpl    *template.Template
}

// New creates a new Server. Returns an error if templates fail to parse.
// statsDB may be nil; when nil the analysis page reads from the filesystem
// reader instead of the database.
func New(cfg Config, reader *rundata.Reader, statsDB *stats.DB) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	md := goldmark.New(goldmark.WithExtensions(extension.Table))
	funcMap := template.FuncMap{
		"mulf": func(a, b float64) float64 { return a * b },
		"renderMarkdown": func(s string) template.HTML {
			var buf bytes.Buffer
			if err := md.Convert([]byte(s), &buf); err != nil {
				return template.HTML(template.HTMLEscapeString(s)) //nolint:gosec
			}
			return template.HTML(buf.Bytes()) //nolint:gosec
		},
		"toJSON": func(v any) (template.JS, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return template.JS(b), nil //nolint:gosec
		},
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	s := &Server{
		cfg:     cfg,
		reader:  reader,
		statsDB: statsDB,
		mux:     http.NewServeMux(),
		tmpl:    tmpl,
	}
	s.routes()
	return s, nil
}

// ServeHTTP implements http.Handler, enabling use with httptest in tests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Serve starts the HTTP server and blocks until ctx is done or a fatal error
// occurs. It opens the browser on startup (best-effort) and shuts down
// gracefully when ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	url := fmt.Sprintf("http://%s", addr)
	s.cfg.Logger.Info("dashboard server started", "url", url)
	go BrowserOpener(url)

	srv := &http.Server{Handler: s.mux, ReadHeaderTimeout: 10 * time.Second}
	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-done:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /analysis", s.handleAnalysis)
	s.mux.HandleFunc("GET /partials/runs-table", s.handleRunsTable)
	s.mux.HandleFunc("GET /partials/analysis-stats", s.handleAnalysisStats)
	s.mux.HandleFunc("GET /runs/{owner}/{repo}/{timestamp}", s.handleRunDetail)
	s.mux.HandleFunc("GET /runs/{owner}/{repo}/{timestamp}/issues-table", s.handleIssuesTable)
	s.mux.HandleFunc("GET /runs/{owner}/{repo}/{timestamp}/issues/{number}", s.handleIssueDetail)
	s.mux.HandleFunc("GET /runs/{owner}/{repo}/{timestamp}/issues/{number}/review-chain", s.handleReviewChain)
	s.mux.HandleFunc("GET /runs/{owner}/{repo}/{timestamp}/logs", s.handleRunLogs)
	s.mux.HandleFunc("GET /runs/{owner}/{repo}/{timestamp}/logs/entries", s.handleRunLogsEntries)
	s.mux.HandleFunc("DELETE /runs/{owner}/{repo}/{timestamp}", s.handleDeleteRun)
	s.mux.Handle("GET /static/", http.FileServer(http.FS(content)))
	s.mux.HandleFunc("GET /watch", s.handleWatchStatus)
}

// BrowserOpener opens a URL in the system default browser. Errors are silently
// ignored. Replaceable for testing.
var BrowserOpener = func(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	_ = exec.Command(cmd, args...).Start() //nolint:gosec
}
