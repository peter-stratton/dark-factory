package dashboard

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const logPageSize = 50

// LogEntry is the view model for one parsed debug.log line.
type LogEntry struct {
	Timestamp  string // formatted wall-clock time, e.g. "15:04:05.000"
	Level      string // "DEBUG", "INFO", "WARN", "ERROR"
	LevelClass string // badge class suffix: "neutral", "info", "warning", "danger"
	Msg        string // log message
	Fields     string // remaining structured fields as compact JSON, may be empty
}

// LogViewerData is the data passed to the log-viewer template and log-entries partial.
type LogViewerData struct {
	Owner     string
	Repo      string
	Timestamp string
	RunURL    string // link back to run detail page
	LogsURL   string // canonical URL for this logs page (used to build HTMX URLs)
	Entries   []LogEntry
	Page      int  // current page (1-indexed)
	NextPage  int  // Page + 1
	HasMore   bool // whether there are more entries beyond this page
	Level     string // current level filter (empty = all)
	Search    string // current search query (empty = all)
}

func (s *Server) handleRunLogs(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	timestamp := r.PathValue("timestamp")
	level := r.URL.Query().Get("level")
	search := r.URL.Query().Get("q")
	if search == "" {
		search = r.URL.Query().Get("search")
	}
	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}

	data, err := s.buildLogViewerData(owner, repo, timestamp, level, search, page)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		s.cfg.Logger.Error("building log viewer data", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "log-viewer.html", data); err != nil {
		s.cfg.Logger.Error("rendering log-viewer template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func (s *Server) handleRunLogsEntries(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	timestamp := r.PathValue("timestamp")
	level := r.URL.Query().Get("level")
	search := r.URL.Query().Get("q")
	if search == "" {
		search = r.URL.Query().Get("search")
	}
	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}

	data, err := s.buildLogViewerData(owner, repo, timestamp, level, search, page)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		s.cfg.Logger.Error("building log entries data", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, "log-entries", data); err != nil {
		s.cfg.Logger.Error("rendering log-entries template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// buildLogViewerData loads, filters, and paginates debug.log entries for the
// given run. Returns os.ErrNotExist (wrapped) if the run directory or run.json
// is absent.
func (s *Server) buildLogViewerData(owner, repo, timestamp, level, search string, page int) (*LogViewerData, error) {
	// Validate path components (same rules as reader.go LoadRun).
	for _, part := range []string{owner, repo, timestamp} {
		if part == "" || strings.Contains(part, "..") || strings.ContainsAny(part, `\/`) {
			return nil, fmt.Errorf("%w: invalid path component %q", os.ErrNotExist, part)
		}
	}

	runDir := s.reader.RunDir(owner, repo, timestamp)

	// Verify the run exists by checking for run.json.
	if _, err := os.Stat(filepath.Join(runDir, "run.json")); err != nil {
		return nil, err
	}

	logPath := filepath.Join(runDir, "debug.log")
	allEntries, err := parseLogFile(logPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading debug.log: %w", err)
	}

	filtered := filterLogEntries(allEntries, level, search)

	// Reverse so newest entries appear first.
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	total := len(filtered)
	hasMore := page*logPageSize < total
	start := (page - 1) * logPageSize
	var pageEntries []LogEntry
	if start < total {
		end := start + logPageSize
		if end > total {
			end = total
		}
		pageEntries = filtered[start:end]
	}

	runURL := fmt.Sprintf("/runs/%s/%s/%s", owner, repo, timestamp)
	logsURL := runURL + "/logs"

	return &LogViewerData{
		Owner:     owner,
		Repo:      repo,
		Timestamp: timestamp,
		RunURL:    runURL,
		LogsURL:   logsURL,
		Entries:   pageEntries,
		Page:      page,
		NextPage:  page + 1,
		HasMore:   hasMore,
		Level:     level,
		Search:    search,
	}, nil
}

// parseLogFile reads a JSON-lines log file and returns all parseable entries.
// Returns nil entries (no error) if the file does not exist.
func parseLogFile(path string) ([]LogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		entry, ok := parseLogLine(line)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// parseLogLine parses one JSON log line produced by slog's JSON handler.
func parseLogLine(line []byte) (LogEntry, bool) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return LogEntry{}, false
	}

	var timeStr, level, msg string
	if v, ok := raw["time"]; ok {
		_ = json.Unmarshal(v, &timeStr)
	}
	if v, ok := raw["level"]; ok {
		_ = json.Unmarshal(v, &level)
	}
	if v, ok := raw["msg"]; ok {
		_ = json.Unmarshal(v, &msg)
	}

	// Collect remaining structured fields and render as key: value pairs.
	var fieldParts []string
	for k, v := range raw {
		if k == "time" || k == "level" || k == "msg" {
			continue
		}
		// Unmarshal the raw value into a plain interface for human-readable display.
		var val any
		if err := json.Unmarshal(v, &val); err == nil {
			fieldParts = append(fieldParts, fmt.Sprintf("%s: %v", k, val))
		}
	}
	sort.Strings(fieldParts)
	fieldsStr := strings.Join(fieldParts, ", ")

	// Format timestamp as HH:MM:SS.mmm for compact display.
	formattedTime := timeStr
	if ts, err := time.Parse(time.RFC3339Nano, timeStr); err == nil {
		formattedTime = ts.UTC().Format("15:04:05.000")
	}

	return LogEntry{
		Timestamp:  formattedTime,
		Level:      level,
		LevelClass: levelToClass(level),
		Msg:        msg,
		Fields:     fieldsStr,
	}, true
}

// levelToClass maps a slog level string to a badge CSS class suffix.
func levelToClass(level string) string {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return "neutral"
	case "INFO":
		return "info"
	case "WARN", "WARNING":
		return "warning"
	case "ERROR":
		return "danger"
	default:
		return "neutral"
	}
}

// filterLogEntries applies level and search filters to a slice of log entries.
func filterLogEntries(entries []LogEntry, level, search string) []LogEntry {
	if level == "" && search == "" {
		return entries
	}
	levelUpper := strings.ToUpper(level)
	searchLower := strings.ToLower(search)

	out := make([]LogEntry, 0, len(entries))
	for _, e := range entries {
		if level != "" && strings.ToUpper(e.Level) != levelUpper {
			continue
		}
		if search != "" {
			if !strings.Contains(strings.ToLower(e.Msg), searchLower) &&
				!strings.Contains(strings.ToLower(e.Fields), searchLower) {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}
