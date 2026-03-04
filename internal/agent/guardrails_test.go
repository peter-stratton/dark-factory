package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReviewResult_Approved(t *testing.T) {
	stdout := "some output\nREVIEW_RESULT=APPROVED\nmore output"
	got := ParseReviewResult(stdout)
	if got != "APPROVED" {
		t.Errorf("ParseReviewResult() = %q, want %q", got, "APPROVED")
	}
}

func TestParseReviewResult_ChangesRequested(t *testing.T) {
	stdout := "output\nREVIEW_RESULT=CHANGES_REQUESTED\n"
	got := ParseReviewResult(stdout)
	if got != "CHANGES_REQUESTED" {
		t.Errorf("ParseReviewResult() = %q, want %q", got, "CHANGES_REQUESTED")
	}
}

func TestParseReviewResult_NotFound(t *testing.T) {
	stdout := "just some random output\nno result here"
	got := ParseReviewResult(stdout)
	if got != "" {
		t.Errorf("ParseReviewResult() = %q, want empty", got)
	}
}

func TestFindPR_ReturnsPRNumber(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte(`{"number": 42}`), nil
	})

	num, err := FindPR("owner/repo", "test-branch")
	if err != nil {
		t.Fatalf("FindPR() error = %v", err)
	}
	if num != 42 {
		t.Errorf("FindPR() = %d, want 42", num)
	}
}

func TestFindPR_ReturnsZeroOnError(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("no PR found")
	})

	num, err := FindPR("owner/repo", "test-branch")
	if err != nil {
		t.Fatalf("FindPR() error = %v", err)
	}
	if num != 0 {
		t.Errorf("FindPR() = %d, want 0", num)
	}
}

func TestEnsureClosesRef_NoOpWhenPresent(t *testing.T) {
	var editCalled bool
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "pr" && args[1] == "view" {
			return []byte(`{"body": "Some PR body\n\nCloses #5"}`), nil
		}
		if len(args) > 0 && args[0] == "pr" && args[1] == "edit" {
			editCalled = true
		}
		return nil, nil
	})

	err := EnsureClosesRef("owner/repo", 10, 5)
	if err != nil {
		t.Fatalf("EnsureClosesRef() error = %v", err)
	}
	if editCalled {
		t.Error("expected no edit when Closes ref already present")
	}
}

func TestEnsureClosesRef_EditsWhenMissing(t *testing.T) {
	var editBody string
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		if len(args) > 2 && args[1] == "view" {
			return []byte(`{"body": "PR body without issue ref"}`), nil
		}
		if len(args) > 2 && args[1] == "edit" {
			for i, a := range args {
				if a == "--body" && i+1 < len(args) {
					editBody = args[i+1]
				}
			}
			return nil, nil
		}
		return nil, nil
	})

	err := EnsureClosesRef("owner/repo", 10, 5)
	if err != nil {
		t.Fatalf("EnsureClosesRef() error = %v", err)
	}
	if !strings.Contains(editBody, "Closes #5") {
		t.Errorf("expected edit body to contain 'Closes #5', got: %s", editBody)
	}
}

func TestCheckProtectedDrift_ReturnsTouchedFiles(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte("CLAUDE.md\nsrc/main.go\ntests/scenarios/spec.md\n"), nil
	})

	touched, err := CheckProtectedDrift("abc123", []string{"CLAUDE.md", "tests/scenarios/spec.md", "safe/file.go"})
	if err != nil {
		t.Fatalf("CheckProtectedDrift() error = %v", err)
	}
	if len(touched) != 2 {
		t.Fatalf("expected 2 touched files, got %d: %v", len(touched), touched)
	}
}

func TestCheckProtectedDrift_EmptyWhenClean(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte("src/main.go\nREADME.md\n"), nil
	})

	touched, err := CheckProtectedDrift("abc123", []string{"CLAUDE.md", "tests/scenarios/"})
	if err != nil {
		t.Fatalf("CheckProtectedDrift() error = %v", err)
	}
	if len(touched) != 0 {
		t.Errorf("expected no touched files, got %v", touched)
	}
}

func TestHasScenarioSpec_Found(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("Relates to: Issue #5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !HasScenarioSpec(dir, 5) {
		t.Error("HasScenarioSpec() = false, want true")
	}
}

func TestHasScenarioSpec_NotFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("Relates to: Issue #99\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if HasScenarioSpec(dir, 5) {
		t.Error("HasScenarioSpec() = true, want false")
	}
}

func TestHasScenarioSpec_MissingDir(t *testing.T) {
	if HasScenarioSpec("/nonexistent-dir-12345", 5) {
		t.Error("HasScenarioSpec() = true, want false for missing dir")
	}
}

func TestWarnMissingScenario_NoWarningWhenFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("Relates to: Issue #5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var commentPosted bool
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[1] == "comment" {
			commentPosted = true
		}
		return nil, nil
	})

	err := WarnMissingScenario("owner/repo", 10, 5, dir, testLogger(t))
	if err != nil {
		t.Fatalf("WarnMissingScenario() error = %v", err)
	}
	if commentPosted {
		t.Error("expected no comment when scenario found")
	}
}

func TestWarnMissingScenario_PostsWarningWhenMissing(t *testing.T) {
	dir := t.TempDir()
	// No scenario files referencing issue #5

	var commentPosted bool
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[1] == "comment" {
			commentPosted = true
		}
		return nil, nil
	})

	err := WarnMissingScenario("owner/repo", 10, 5, dir, testLogger(t))
	if err != nil {
		t.Fatalf("WarnMissingScenario() error = %v", err)
	}
	if !commentPosted {
		t.Error("expected warning comment when no scenario found")
	}
}

func TestFindPR_MalformedJSON(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte(`not json`), nil
	})

	_, err := FindPR("owner/repo", "test-branch")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parsing PR JSON") {
		t.Errorf("error = %v, want 'parsing PR JSON'", err)
	}
}

func TestEnsureClosesRef_NoOpWhenFixesPresent(t *testing.T) {
	var editCalled bool
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "pr" && args[1] == "view" {
			return []byte(`{"body": "Some PR body\n\nFixes #5"}`), nil
		}
		if len(args) > 0 && args[0] == "pr" && args[1] == "edit" {
			editCalled = true
		}
		return nil, nil
	})

	err := EnsureClosesRef("owner/repo", 10, 5)
	if err != nil {
		t.Fatalf("EnsureClosesRef() error = %v", err)
	}
	if editCalled {
		t.Error("expected no edit when Fixes ref already present")
	}
}

func TestEnsureClosesRef_MalformedJSON(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte(`not json`), nil
	})

	err := EnsureClosesRef("owner/repo", 10, 5)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parsing PR body") {
		t.Errorf("error = %v, want 'parsing PR body'", err)
	}
}

func TestClosePR_ReturnsError(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("network error")
	})

	err := ClosePR("owner/repo", 10, "reason")
	if err == nil {
		t.Fatal("expected error from ClosePR")
	}
	if !strings.Contains(err.Error(), "closing PR #10") {
		t.Errorf("error = %v, want 'closing PR #10'", err)
	}
}

func TestParseReviewResult_FirstMatchWins(t *testing.T) {
	stdout := "REVIEW_RESULT=APPROVED\nREVIEW_RESULT=CHANGES_REQUESTED\n"
	got := ParseReviewResult(stdout)
	if got != "APPROVED" {
		t.Errorf("ParseReviewResult() = %q, want %q (first match should win)", got, "APPROVED")
	}
}

func TestParseReviewResult_WhitespaceHandling(t *testing.T) {
	stdout := "  REVIEW_RESULT=APPROVED  \n"
	got := ParseReviewResult(stdout)
	if got != "APPROVED" {
		t.Errorf("ParseReviewResult() = %q, want %q", got, "APPROVED")
	}
}

func TestHasScenarioSpec_SkipsNonMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("Relates to: Issue #5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if HasScenarioSpec(dir, 5) {
		t.Error("HasScenarioSpec() should skip non-.md files")
	}
}

func TestLabelPR(t *testing.T) {
	var labelArg string
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		for i, a := range args {
			if a == "--add-label" && i+1 < len(args) {
				labelArg = args[i+1]
			}
		}
		return nil, nil
	})

	err := LabelPR("owner/repo", 10, "needs-human-review")
	if err != nil {
		t.Fatalf("LabelPR() error = %v", err)
	}
	if labelArg != "needs-human-review" {
		t.Errorf("expected label 'needs-human-review', got %q", labelArg)
	}
}
