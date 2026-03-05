package github

import (
	"fmt"
	"testing"
)

func TestEnsureLabel_CreatesWhenMissing(t *testing.T) {
	var createCalled bool
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "create" {
				createCalled = true
				return []byte(`{}`), nil
			}
		}
		// list call: return empty label list
		return []byte(`[]`), nil
	}
	defer func() { CommandRunner = orig }()

	if err := EnsureLabel("owner/repo", "godark-in-progress", "FF6B35", "test label"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !createCalled {
		t.Error("expected label create to be called")
	}
}

func TestEnsureLabel_SkipsWhenExists(t *testing.T) {
	var createCalled bool
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "create" {
				createCalled = true
				return []byte(`{}`), nil
			}
		}
		// list call: label already exists
		return []byte(`[{"name":"godark-in-progress"}]`), nil
	}
	defer func() { CommandRunner = orig }()

	if err := EnsureLabel("owner/repo", "godark-in-progress", "FF6B35", "test label"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createCalled {
		t.Error("expected label creation to be skipped when label already exists")
	}
}

func TestEnsureLabel_CaseInsensitiveMatch(t *testing.T) {
	var createCalled bool
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "create" {
				createCalled = true
				return []byte(`{}`), nil
			}
		}
		// label exists with different casing
		return []byte(`[{"name":"GODARK-IN-PROGRESS"}]`), nil
	}
	defer func() { CommandRunner = orig }()

	if err := EnsureLabel("owner/repo", "godark-in-progress", "FF6B35", "test label"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createCalled {
		t.Error("expected label creation to be skipped (case-insensitive match)")
	}
}

func TestEnsureLabel_ErrorOnListFailure(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh: not authenticated")
	}
	defer func() { CommandRunner = orig }()

	if err := EnsureLabel("owner/repo", "godark-in-progress", "FF6B35", "test"); err == nil {
		t.Fatal("expected error when listing labels fails")
	}
}

func TestAddIssueLabel_CallsGH(t *testing.T) {
	var gotArgs []string
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte{}, nil
	}
	defer func() { CommandRunner = orig }()

	if err := AddIssueLabel("owner/repo", 42, "godark-in-progress"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotArgs) == 0 {
		t.Fatal("expected CommandRunner to be called with args")
	}
	// Verify the issue number and label appear in args.
	found42, foundLabel := false, false
	for _, a := range gotArgs {
		if a == "42" {
			found42 = true
		}
		if a == "godark-in-progress" {
			foundLabel = true
		}
	}
	if !found42 {
		t.Errorf("issue number 42 not in args: %v", gotArgs)
	}
	if !foundLabel {
		t.Errorf("label not in args: %v", gotArgs)
	}
}

func TestAddIssueLabel_Error(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh error")
	}
	defer func() { CommandRunner = orig }()

	if err := AddIssueLabel("owner/repo", 1, "godark-in-progress"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRemoveIssueLabel_CallsGH(t *testing.T) {
	var called bool
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		called = true
		return []byte{}, nil
	}
	defer func() { CommandRunner = orig }()

	if err := RemoveIssueLabel("owner/repo", 7, "godark-in-progress"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected CommandRunner to be called")
	}
}

func TestFindIssuesWithLabel_ReturnsList(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`[{"number":7},{"number":14}]`), nil
	}
	defer func() { CommandRunner = orig }()

	nums, err := FindIssuesWithLabel("owner/repo", "godark-in-progress")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nums) != 2 || nums[0] != 7 || nums[1] != 14 {
		t.Errorf("got %v, want [7 14]", nums)
	}
}

func TestFindIssuesWithLabel_Empty(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	}
	defer func() { CommandRunner = orig }()

	nums, err := FindIssuesWithLabel("owner/repo", "godark-in-progress")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nums) != 0 {
		t.Errorf("expected empty, got %v", nums)
	}
}

func TestFindIssuesWithLabel_Error(t *testing.T) {
	orig := CommandRunner
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh error")
	}
	defer func() { CommandRunner = orig }()

	_, err := FindIssuesWithLabel("owner/repo", "godark-in-progress")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
