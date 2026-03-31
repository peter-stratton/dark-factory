package github

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheckMergeable_Mergeable(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"mergeable":"MERGEABLE"}`), nil
	}

	status, err := CheckMergeable("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "MERGEABLE" {
		t.Errorf("got %q, want %q", status, "MERGEABLE")
	}
}

func TestCheckMergeable_Conflicting(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"mergeable":"CONFLICTING"}`), nil
	}

	status, err := CheckMergeable("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "CONFLICTING" {
		t.Errorf("got %q, want %q", status, "CONFLICTING")
	}
}

func TestCheckMergeable_Unknown(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"mergeable":"UNKNOWN"}`), nil
	}

	status, err := CheckMergeable("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "UNKNOWN" {
		t.Errorf("got %q, want %q", status, "UNKNOWN")
	}
}

func TestCheckMergeable_CLIError(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exit status 1")
	}

	_, err := CheckMergeable("owner/repo", 42)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCheckMergeable_MalformedJSON(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`not valid json`), nil
	}

	_, err := CheckMergeable("owner/repo", 42)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestCheckMergeable_PassesCorrectArgs(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var capturedArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		capturedArgs = append([]string{name}, args...)
		return []byte(`{"mergeable":"MERGEABLE"}`), nil
	}

	_, err := CheckMergeable("owner/repo", 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"gh", "pr", "view", "99", "--repo", "owner/repo", "--json", "mergeable"}
	if len(capturedArgs) != len(want) {
		t.Fatalf("args length: got %d, want %d\ngot: %v", len(capturedArgs), len(want), capturedArgs)
	}
	for i, w := range want {
		if capturedArgs[i] != w {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], w)
		}
	}
}

func TestWaitForMergeable_ResolvesFromUnknown(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var calls atomic.Int32
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		n := calls.Add(1)
		if n <= 2 {
			return []byte(`{"mergeable":"UNKNOWN"}`), nil
		}
		return []byte(`{"mergeable":"MERGEABLE"}`), nil
	}

	status, err := WaitForMergeable("owner/repo", 1, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "MERGEABLE" {
		t.Errorf("got %q, want MERGEABLE", status)
	}
	if c := calls.Load(); c < 3 {
		t.Errorf("expected at least 3 calls, got %d", c)
	}
}

func TestWaitForMergeable_ImmediatelyResolved(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"mergeable":"CONFLICTING"}`), nil
	}

	status, err := WaitForMergeable("owner/repo", 1, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "CONFLICTING" {
		t.Errorf("got %q, want CONFLICTING", status)
	}
}

func TestWaitForMergeable_TimesOut(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"mergeable":"UNKNOWN"}`), nil
	}

	start := time.Now()
	status, err := WaitForMergeable("owner/repo", 1, 3*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "UNKNOWN" {
		t.Errorf("got %q, want UNKNOWN", status)
	}
	if elapsed < 2*time.Second {
		t.Errorf("expected to wait at least 2s, waited %v", elapsed)
	}
}

func TestWaitForMergeable_PropagatesError(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("network error")
	}

	_, err := WaitForMergeable("owner/repo", 1, 10*time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateBranch_Success(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	err := UpdateBranch("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateBranch_CLIError(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exit status 1")
	}

	err := UpdateBranch("owner/repo", 42)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateBranch_PassesCorrectArgs(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var capturedArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		capturedArgs = append([]string{name}, args...)
		return []byte(""), nil
	}

	err := UpdateBranch("owner/repo", 77)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"gh", "pr", "update-branch", "77", "--repo", "owner/repo"}
	if len(capturedArgs) != len(want) {
		t.Fatalf("args length: got %d, want %d\ngot: %v", len(capturedArgs), len(want), capturedArgs)
	}
	for i, w := range want {
		if capturedArgs[i] != w {
			t.Errorf("arg[%d]: got %q, want %q", i, capturedArgs[i], w)
		}
	}
}
