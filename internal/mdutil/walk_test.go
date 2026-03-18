package mdutil_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/mdutil"
)

func TestWalkMarkdownFiles_OnlyMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "")
	writeFile(t, filepath.Join(dir, "b.go"), "")
	writeFile(t, filepath.Join(dir, "c.txt"), "")

	var got []string
	err := mdutil.WalkMarkdownFiles(dir, func(path string) error {
		got = append(got, filepath.Base(path))
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "a.md" {
		t.Errorf("got %v, want [a.md]", got)
	}
}

func TestWalkMarkdownFiles_SkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(subdir, "nested.md"), "")

	var got []string
	err := mdutil.WalkMarkdownFiles(dir, func(path string) error {
		got = append(got, path)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// fn should only be called with file paths, not directory paths
	for _, p := range got {
		info, statErr := os.Stat(p)
		if statErr != nil {
			t.Fatalf("stat %s: %v", p, statErr)
		}
		if info.IsDir() {
			t.Errorf("fn was called with a directory path: %s", p)
		}
	}
}

func TestWalkMarkdownFiles_PropagatesFnError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "")

	sentinel := errors.New("stop")
	err := mdutil.WalkMarkdownFiles(dir, func(path string) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("got %v, want sentinel error", err)
	}
}

func TestWalkMarkdownFiles_MissingDirectory(t *testing.T) {
	err := mdutil.WalkMarkdownFiles("/nonexistent/path/does/not/exist", func(path string) error {
		return nil
	})
	if err == nil {
		t.Error("expected error for missing directory, got nil")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
