package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})

	return slog.New(&multiHandler{json: jsonHandler, text: textHandler}), nil
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
