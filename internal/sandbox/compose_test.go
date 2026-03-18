package sandbox

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
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

// --- ComposeUp tests ---

func TestComposeUp_NoOp_WhenComposeFileEmpty(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	called := false
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}

	dc := DockerConfig{ComposeFile: ""}
	cleanup, err := ComposeUp(context.Background(), dc, nil, testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cleanup()

	if called {
		t.Error("expected CommandRunner not to be called when ComposeFile is empty")
	}
}

func TestComposeUp_CallsDockerComposeUp(t *testing.T) {
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
	cleanup, err := ComposeUp(context.Background(), dc, nil, testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cleanup()

	if gotName != "docker" {
		t.Errorf("expected command %q, got %q", "docker", gotName)
	}
	// Without required_env, no --env-file flag; args must include "up" and "-d".
	wantArgs := []string{"compose", "-f", "docker-compose.yml", "-p", "myproject", "up", "-d"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("expected args %v, got %v", wantArgs, gotArgs)
	}
	for i, w := range wantArgs {
		if gotArgs[i] != w {
			t.Errorf("arg[%d]: want %q, got %q", i, w, gotArgs[i])
		}
	}
}

func TestComposeUp_RequiredEnvForwardedViaEnvFile(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("PUBSUB_HOST", "localhost:8085")

	var gotArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}

	dc := DockerConfig{
		ComposeFile:        "docker-compose.yml",
		ComposeProjectName: "myapp",
	}
	cleanup, err := ComposeUp(context.Background(), dc, []string{"DATABASE_URL", "PUBSUB_HOST"}, testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	// Locate the --env-file flag and read the file.
	var envFilePath string
	for i, a := range gotArgs {
		if a == "--env-file" && i+1 < len(gotArgs) {
			envFilePath = gotArgs[i+1]
			break
		}
	}
	if envFilePath == "" {
		t.Fatalf("expected --env-file flag in args %v", gotArgs)
	}

	f, err := os.Open(envFilePath)
	if err != nil {
		t.Fatalf("could not open env file %q: %v", envFilePath, err)
	}
	defer f.Close()

	envContents := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			envContents[parts[0]] = parts[1]
		}
	}

	if envContents["DATABASE_URL"] != "postgres://localhost/test" {
		t.Errorf("DATABASE_URL = %q, want %q", envContents["DATABASE_URL"], "postgres://localhost/test")
	}
	if envContents["PUBSUB_HOST"] != "localhost:8085" {
		t.Errorf("PUBSUB_HOST = %q, want %q", envContents["PUBSUB_HOST"], "localhost:8085")
	}
}

func TestComposeUp_AuthVarsExcludedFromEnvFile(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	t.Setenv("GH_TOKEN", "gho_secret")
	t.Setenv("ANTHROPIC_API_KEY", "sk-secret")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-secret")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	var gotArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}

	dc := DockerConfig{
		ComposeFile:        "docker-compose.yml",
		ComposeProjectName: "myapp",
	}
	cleanup, err := ComposeUp(context.Background(), dc,
		[]string{"GH_TOKEN", "ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN", "DATABASE_URL"},
		testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	// Find and read the env file.
	var envFilePath string
	for i, a := range gotArgs {
		if a == "--env-file" && i+1 < len(gotArgs) {
			envFilePath = gotArgs[i+1]
			break
		}
	}
	if envFilePath == "" {
		t.Fatalf("expected --env-file flag in args %v", gotArgs)
	}

	data, err := os.ReadFile(envFilePath)
	if err != nil {
		t.Fatalf("could not read env file: %v", err)
	}
	contents := string(data)

	// Auth-managed vars must not appear in the env file.
	for _, forbidden := range []string{"GH_TOKEN", "ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"} {
		if strings.Contains(contents, forbidden) {
			t.Errorf("env file must not contain auth-managed var %q", forbidden)
		}
	}

	// Non-auth var must be present.
	if !strings.Contains(contents, "DATABASE_URL") {
		t.Errorf("env file must contain DATABASE_URL; contents: %s", contents)
	}
}

func TestComposeUp_MissingVarsSilentlySkipped(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Unsetenv("MISSING_VAR") //nolint:errcheck

	var gotArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}

	dc := DockerConfig{
		ComposeFile:        "docker-compose.yml",
		ComposeProjectName: "myapp",
	}
	cleanup, err := ComposeUp(context.Background(), dc,
		[]string{"DATABASE_URL", "MISSING_VAR"},
		testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error for missing var: %v", err)
	}
	defer cleanup()

	// Find and read the env file.
	var envFilePath string
	for i, a := range gotArgs {
		if a == "--env-file" && i+1 < len(gotArgs) {
			envFilePath = gotArgs[i+1]
			break
		}
	}
	if envFilePath == "" {
		t.Fatalf("expected --env-file flag in args %v", gotArgs)
	}

	data, err := os.ReadFile(envFilePath)
	if err != nil {
		t.Fatalf("could not read env file: %v", err)
	}
	contents := string(data)

	if !strings.Contains(contents, "DATABASE_URL") {
		t.Errorf("env file must contain DATABASE_URL; got: %s", contents)
	}
	if strings.Contains(contents, "MISSING_VAR") {
		t.Errorf("env file must not contain MISSING_VAR; got: %s", contents)
	}
}

func TestComposeUp_EnvFileCleanedUpAfterCleanupCalled(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	var gotArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}

	dc := DockerConfig{
		ComposeFile:        "docker-compose.yml",
		ComposeProjectName: "myapp",
	}
	cleanup, err := ComposeUp(context.Background(), dc, []string{"DATABASE_URL"}, testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Locate the temp env file path from args.
	var envFilePath string
	for i, a := range gotArgs {
		if a == "--env-file" && i+1 < len(gotArgs) {
			envFilePath = gotArgs[i+1]
			break
		}
	}
	if envFilePath == "" {
		t.Fatalf("expected --env-file flag in args %v", gotArgs)
	}

	// File must exist before cleanup.
	if _, err := os.Stat(envFilePath); err != nil {
		t.Fatalf("env file should exist before cleanup: %v", err)
	}

	// Simulate ComposeDown completing, then call cleanup.
	cleanup()

	// File must be removed after cleanup.
	if _, err := os.Stat(envFilePath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("env file should be removed after cleanup, stat error: %v", err)
	}
}

func TestComposeUp_NoEnvFile_WhenNoRequiredEnv(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var gotArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}

	dc := DockerConfig{
		ComposeFile:        "docker-compose.yml",
		ComposeProjectName: "myapp",
	}
	cleanup, err := ComposeUp(context.Background(), dc, nil, testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cleanup()

	for _, a := range gotArgs {
		if a == "--env-file" {
			t.Error("expected no --env-file flag when requiredEnv is empty")
		}
	}
}

func TestComposeUp_DefaultProjectName(t *testing.T) {
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
	cleanup, err := ComposeUp(context.Background(), dc, nil, testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cleanup()

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

func TestComposeUp_ReturnsError_OnCommandFailure(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("compose error"), errors.New("exit status 1")
	}

	dc := DockerConfig{
		ComposeFile:        "docker-compose.yml",
		ComposeProjectName: "myapp",
	}
	cleanup, err := ComposeUp(context.Background(), dc, nil, testNopLogger())
	cleanup() // always safe to call

	if err == nil {
		t.Fatal("expected error when docker compose up fails")
	}
	if !strings.Contains(err.Error(), "docker compose up") {
		t.Errorf("error = %q, want 'docker compose up' in message", err)
	}
}
