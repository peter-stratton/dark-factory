package sandbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

// testNopLogger returns a logger that discards all output.
func testNopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// composeRecordingHandler is a slog.Handler that captures all log records.
type composeRecordingHandler struct {
	records []slog.Record
}

func (h *composeRecordingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *composeRecordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}
func (h *composeRecordingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *composeRecordingHandler) WithGroup(_ string) slog.Handler      { return h }

func TestComposeDown_NoOp_WhenComposeFileEmpty(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	called := false
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}

	dc := DockerConfig{ComposeFile: ""}
	ComposeDown(dc, testNopLogger())

	if called {
		t.Error("expected CommandRunner not to be called when ComposeFile is empty")
	}
}

func TestComposeDown_CallsDockerComposeDown(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var gotName string
	var gotArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = args
		return []byte("done"), nil
	}

	dc := DockerConfig{
		ComposeFile:        "docker-compose.yml",
		ComposeProjectName: "myproject",
	}
	ComposeDown(dc, testNopLogger())

	if gotName != "docker" {
		t.Errorf("expected command %q, got %q", "docker", gotName)
	}

	wantArgs := []string{"compose", "-f", "docker-compose.yml", "-p", "myproject", "down", "--volumes"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("expected args %v, got %v", wantArgs, gotArgs)
	}
	for i, w := range wantArgs {
		if gotArgs[i] != w {
			t.Errorf("arg[%d]: want %q, got %q", i, w, gotArgs[i])
		}
	}
}

func TestComposeDown_DefaultProjectName_WhenEmpty(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var gotArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}

	dc := DockerConfig{
		ComposeFile:        "docker-compose.yml",
		ComposeProjectName: "",
	}
	ComposeDown(dc, testNopLogger())

	// Find the -p flag and its value.
	for i, a := range gotArgs {
		if a == "-p" && i+1 < len(gotArgs) {
			if gotArgs[i+1] != "godark" {
				t.Errorf("expected default project name %q, got %q", "godark", gotArgs[i+1])
			}
			return
		}
	}
	t.Errorf("expected -p flag in args %v", gotArgs)
}

func TestComposeDown_ErrorLogged_NotReturned(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("some error output"), errors.New("exit status 1")
	}

	handler := &composeRecordingHandler{}
	logger := slog.New(handler)

	dc := DockerConfig{
		ComposeFile:        "docker-compose.yml",
		ComposeProjectName: "myproject",
	}

	// ComposeDown must not panic and returns void (error is not propagated).
	ComposeDown(dc, logger)

	// Verify a warning was logged.
	found := false
	for _, r := range handler.records {
		if r.Level == slog.LevelWarn {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a warning log record when compose down fails")
	}
}

func TestComposeDown_IncludesVolumesFlag(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var gotArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}

	dc := DockerConfig{
		ComposeFile:        "docker-compose.yml",
		ComposeProjectName: "proj",
	}
	ComposeDown(dc, testNopLogger())

	found := false
	for _, a := range gotArgs {
		if a == "--volumes" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --volumes flag in args %v", gotArgs)
	}
}
