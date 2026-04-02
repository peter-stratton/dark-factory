package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestWriteFileWithDirsCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c.txt")

	if err := writeFileWithDirs(path, []byte("hello")); err != nil {
		t.Fatalf("writeFileWithDirs: %v", err)
	}

	parentDir := filepath.Join(dir, "a", "b")
	info, err := os.Stat(parentDir)
	if err != nil {
		t.Fatalf("parent directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory", parentDir)
	}
}

func TestWriteFileWithDirsWritesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.txt")
	content := []byte("expected content")

	if err := writeFileWithDirs(path, content); err != nil {
		t.Fatalf("writeFileWithDirs: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestSemiformalReviewerScaffoldInstalled(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	cmd := &cobra.Command{}
	cmd.SetOut(new(bytes.Buffer))
	if err := writeHarnessPrompts(cmd); err != nil {
		t.Fatalf("writeHarnessPrompts: %v", err)
	}

	dest := filepath.Join("prompts", "reviewer_semiformal.txt")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reviewer_semiformal.txt not written: %v", err)
	}
	if len(data) == 0 {
		t.Error("reviewer_semiformal.txt is empty")
	}
}

func TestWriteFileWithDirsOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	if err := writeFileWithDirs(path, []byte("original")); err != nil {
		t.Fatalf("first write: %v", err)
	}

	newContent := []byte("updated")
	if err := writeFileWithDirs(path, newContent); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file after overwrite: %v", err)
	}
	if string(got) != string(newContent) {
		t.Errorf("expected overwritten content %q, got %q", newContent, got)
	}
}
