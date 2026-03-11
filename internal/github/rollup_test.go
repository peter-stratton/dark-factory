package github

import (
	"testing"
)

func TestParsePRNumber(t *testing.T) {
	tests := []struct {
		url     string
		want    int
		wantErr bool
	}{
		{"https://github.com/owner/repo/pull/42", 42, false},
		{"https://github.com/owner/repo/pull/42/", 42, false},
		{"https://github.com/owner/repo/pull/1000", 1000, false},
		{"https://github.com/owner/repo/pull/abc", 0, true},
		{"no-slash", 0, true},
	}

	for _, tt := range tests {
		got, err := parsePRNumber(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("parsePRNumber(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parsePRNumber(%q) = %d, want %d", tt.url, got, tt.want)
		}
	}
}

func TestCreateRollupPR_Success(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("https://github.com/owner/repo/pull/77\n"), nil
	}

	prNum, prURL, err := CreateRollupPR("owner/repo", "feature-branch", "main", "chore: rollup", "body text")
	if err != nil {
		t.Fatalf("CreateRollupPR() error = %v", err)
	}
	if prNum != 77 {
		t.Errorf("CreateRollupPR() prNum = %d, want 77", prNum)
	}
	if prURL != "https://github.com/owner/repo/pull/77" {
		t.Errorf("CreateRollupPR() prURL = %q, want URL", prURL)
	}
}

func TestCreateRollupPR_CommandError(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, errFake
	}

	_, _, err := CreateRollupPR("owner/repo", "feature-branch", "main", "title", "body")
	if err == nil {
		t.Fatal("expected error when command fails")
	}
}

func TestMergeRollupPR_Success(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	var capturedArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		capturedArgs = append([]string{name}, args...)
		return []byte(""), nil
	}

	if err := MergeRollupPR("owner/repo", 42); err != nil {
		t.Fatalf("MergeRollupPR() error = %v", err)
	}

	// Verify gh pr merge was called with expected args.
	found := false
	for _, a := range capturedArgs {
		if a == "--squash" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --squash flag in args, got %v", capturedArgs)
	}
}

func TestMergeRollupPR_CommandError(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, errFake
	}

	if err := MergeRollupPR("owner/repo", 42); err == nil {
		t.Fatal("expected error when command fails")
	}
}

// errFake is a sentinel error for tests.
var errFake = &fakeError{"fake command error"}

type fakeError struct{ msg string }

func (e *fakeError) Error() string { return e.msg }
