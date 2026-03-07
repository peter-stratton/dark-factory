package dashboard

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed templates static
var content embed.FS

// Config holds the dashboard server configuration.
type Config struct {
	Port   int
	Logger *slog.Logger
}

// Server is the dashboard HTTP server.
type Server struct {
	cfg    Config
	reader *rundata.Reader
	mux    *http.ServeMux
	tmpl   *template.Template
}

// New creates a new Server. Returns an error if templates fail to parse.
func New(cfg Config, reader *rundata.Reader) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	md := goldmark.New(goldmark.WithExtensions(extension.Table))
	funcMap := template.FuncMap{
		"renderMarkdown": func(s string) template.HTML {
			var buf bytes.Buffer
			if err := md.Convert([]byte(s), &buf); err != nil {
				return template.HTML(template.HTMLEscapeString(s)) //nolint:gosec
			}
			return template.HTML(buf.Bytes()) //nolint:gosec
		},
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	s := &Server{
		cfg:    cfg,
		reader: reader,
		mux:    http.NewServeMux(),
		tmpl:   tmpl,
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

	srv := &http.Server{Handler: s.mux}
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
	s.mux.HandleFunc("GET /runs/{owner}/{repo}/{timestamp}/logs", s.handleRunLogs)
	s.mux.HandleFunc("GET /runs/{owner}/{repo}/{timestamp}/logs/entries", s.handleRunLogsEntries)
	s.mux.Handle("GET /static/", http.FileServer(http.FS(content)))
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
