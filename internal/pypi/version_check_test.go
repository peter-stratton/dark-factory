package pypi

import (
	"errors"
	"strings"
	"testing"
)

// --- LatestVersion ---

func TestLatestVersion_OK(t *testing.T) {
	orig := HTTPGet
	defer func() { HTTPGet = orig }()

	HTTPGet = func(url string) ([]byte, error) {
		if !strings.Contains(url, "claude-agent-sdk") {
			t.Errorf("unexpected URL: %s", url)
		}
		return []byte(`{"info":{"version":"0.1.5"}}`), nil
	}

	got, err := LatestVersion("claude-agent-sdk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0.1.5" {
		t.Errorf("got %q, want %q", got, "0.1.5")
	}
}

func TestLatestVersion_HTTPError(t *testing.T) {
	orig := HTTPGet
	defer func() { HTTPGet = orig }()

	HTTPGet = func(url string) ([]byte, error) {
		return nil, errors.New("connection refused")
	}

	_, err := LatestVersion("claude-agent-sdk")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLatestVersion_Non200(t *testing.T) {
	orig := HTTPGet
	defer func() { HTTPGet = orig }()

	HTTPGet = func(url string) ([]byte, error) {
		return nil, errors.New("HTTP 404 from https://pypi.org/pypi/claude-agent-sdk/json")
	}

	_, err := LatestVersion("claude-agent-sdk")
	if err == nil {
		t.Fatal("expected error for non-200, got nil")
	}
}

func TestLatestVersion_BadJSON(t *testing.T) {
	orig := HTTPGet
	defer func() { HTTPGet = orig }()

	HTTPGet = func(url string) ([]byte, error) {
		return []byte(`{not valid json`), nil
	}

	_, err := LatestVersion("claude-agent-sdk")
	if err == nil {
		t.Fatal("expected error for bad JSON, got nil")
	}
}

func TestLatestVersion_EmptyVersion(t *testing.T) {
	orig := HTTPGet
	defer func() { HTTPGet = orig }()

	HTTPGet = func(url string) ([]byte, error) {
		return []byte(`{"info":{"version":""}}`), nil
	}

	_, err := LatestVersion("claude-agent-sdk")
	if err == nil {
		t.Fatal("expected error for empty version, got nil")
	}
}
