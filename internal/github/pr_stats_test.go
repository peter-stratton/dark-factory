package github

import (
	"fmt"
	"testing"
)

func TestFetchPRStats_Success(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"additions":50,"deletions":20,"changedFiles":3}`), nil
	}

	additions, deletions, fileCount, err := FetchPRStats("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if additions != 50 {
		t.Errorf("additions = %d, want 50", additions)
	}
	if deletions != 20 {
		t.Errorf("deletions = %d, want 20", deletions)
	}
	if fileCount != 3 {
		t.Errorf("fileCount = %d, want 3", fileCount)
	}
}

func TestFetchPRStats_CLIError(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exit status 1")
	}

	_, _, _, err := FetchPRStats("owner/repo", 42)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchPRStats_MalformedJSON(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`not valid json`), nil
	}

	_, _, _, err := FetchPRStats("owner/repo", 42)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestFetchPRChangedFiles_Success(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("internal/foo/bar.go\ninternal/baz/qux.go\n"), nil
	}

	files, err := FetchPRChangedFiles("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}
	if files[0] != "internal/foo/bar.go" {
		t.Errorf("files[0] = %q, want %q", files[0], "internal/foo/bar.go")
	}
	if files[1] != "internal/baz/qux.go" {
		t.Errorf("files[1] = %q, want %q", files[1], "internal/baz/qux.go")
	}
}

func TestFetchPRChangedFiles_EmptyOutput(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	files, err := FetchPRChangedFiles("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d: %v", len(files), files)
	}
}

func TestFetchPRChangedFiles_CLIError(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exit status 1")
	}

	_, err := FetchPRChangedFiles("owner/repo", 42)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchPRChangedFiles_StripsBlankLines(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("file1.go\n\nfile2.go\n"), nil
	}

	files, err := FetchPRChangedFiles("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files (blank lines stripped), got %d: %v", len(files), files)
	}
}
