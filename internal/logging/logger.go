package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// ANSI escape codes for terminal colors.
const (
	ansiReset  = "\033[0m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiBold   = "\033[1m"
)

// NewLogger creates a logger that writes structured JSON to debug.log in dir
// and human-readable text to stdout. The directory is created if it does not exist.
func NewLogger(dir string) (*slog.Logger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	path := filepath.Join(dir, "debug.log")

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}

	jsonHandler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})

	var stdout io.Writer = os.Stdout
	if term.IsTerminal(int(os.Stdout.Fd())) {
		stdout = &colorWriter{w: os.Stdout}
	}
	textHandler := slog.NewTextHandler(stdout, &slog.HandlerOptions{Level: slog.LevelDebug})

	return slog.New(&multiHandler{json: jsonHandler, text: textHandler}), nil
}

// colorWriter wraps a writer and applies ANSI color to log lines that contain
// verdict information (APPROVED / CHANGES_REQUESTED) or key status messages.
type colorWriter struct {
	w io.Writer
}

func (c *colorWriter) Write(p []byte) (int, error) {
	line := string(p)

	if color := verdictColor(line); color != "" {
		var buf bytes.Buffer
		buf.WriteString(ansiBold)
		buf.WriteString(color)
		buf.Write(p)
		buf.WriteString(ansiReset)
		_, err := c.w.Write(buf.Bytes())
		return len(p), err
	}

	return c.w.Write(p)
}

// verdictColor returns the ANSI color for a log line based on its content,
// or empty string if no highlighting is needed.
func verdictColor(line string) string {
	upper := strings.ToUpper(line)
	if strings.Contains(upper, "VERDICT=APPROVED") || strings.Contains(upper, `VERDICT="APPROVED"`) {
		return ansiGreen
	}
	if strings.Contains(upper, "CHANGES_REQUESTED") {
		return ansiYellow
	}
	if strings.Contains(upper, "VERIFY STEP PASSED") {
		return ansiGreen
	}
	if strings.Contains(upper, "TIMED_OUT=TRUE") || strings.Contains(upper, `TIMED_OUT="TRUE"`) {
		return ansiRed
	}
	return ""
}

// multiHandler fans out log records to multiple handlers.
type multiHandler struct {
	json slog.Handler
	text slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.json.Enabled(ctx, level) || h.text.Enabled(ctx, level)
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	if err := h.json.Handle(ctx, r); err != nil {
		return err
	}
	return h.text.Handle(ctx, r)
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &multiHandler{
		json: h.json.WithAttrs(attrs),
		text: h.text.WithAttrs(attrs),
	}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	return &multiHandler{
		json: h.json.WithGroup(name),
		text: h.text.WithGroup(name),
	}
}
