package pypi

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// --- ExceedsPinnedRange ---

func TestExceedsPinnedRange(t *testing.T) {
	cases := []struct {
		latest     string
		upperBound string
		want       bool
		wantErr    bool
	}{
		// Within range (< 0.2.0)
		{"0.1.0", "0.2.0", false, false},
		{"0.1.9", "0.2.0", false, false},
		{"0.1.99", "0.2.0", false, false},

		// At the upper bound (== 0.2.0) → exceeds exclusive upper bound
		{"0.2.0", "0.2.0", true, false},

		// Above the upper bound
		{"0.2.1", "0.2.0", true, false},
		{"0.3.0", "0.2.0", true, false},
		{"1.0.0", "0.2.0", true, false},

		// Major version bump
		{"2.0.0", "0.2.0", true, false},

		// Two-part versions treated as X.Y.0
		{"0.1", "0.2.0", false, false},
		{"0.2", "0.2.0", true, false},

		// Same major, same minor, patch comparison
		{"0.2.5", "0.2.0", true, false},

		// Malformed inputs
		{"not-a-version", "0.2.0", false, true},
		{"0.1.0", "bad", false, true},
		{"0", "0.2.0", false, true},
	}

	for _, tc := range cases {
		got, err := ExceedsPinnedRange(tc.latest, tc.upperBound)
		if (err != nil) != tc.wantErr {
			t.Errorf("ExceedsPinnedRange(%q, %q): err=%v, wantErr=%v", tc.latest, tc.upperBound, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("ExceedsPinnedRange(%q, %q) = %v, want %v", tc.latest, tc.upperBound, got, tc.want)
		}
	}
}

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

// --- WarnIfSDKOutdated ---

func TestWarnIfSDKOutdated_WarnsWhenExceeded(t *testing.T) {
	orig := HTTPGet
	defer func() { HTTPGet = orig }()

	HTTPGet = func(url string) ([]byte, error) {
		return []byte(`{"info":{"version":"0.2.0"}}`), nil
	}

	var buf bytes.Buffer
	WarnIfSDKOutdated(&buf, slog.Default())

	out := buf.String()
	if !strings.Contains(out, "WARNING") {
		t.Errorf("expected WARNING in output, got: %q", out)
	}
	if !strings.Contains(out, "0.2.0") {
		t.Errorf("expected version 0.2.0 in output, got: %q", out)
	}
	if !strings.Contains(out, SDKPackage) {
		t.Errorf("expected package name in output, got: %q", out)
	}
}

func TestWarnIfSDKOutdated_NoWarnWhenWithinRange(t *testing.T) {
	orig := HTTPGet
	defer func() { HTTPGet = orig }()

	HTTPGet = func(url string) ([]byte, error) {
		return []byte(`{"info":{"version":"0.1.9"}}`), nil
	}

	var buf bytes.Buffer
	WarnIfSDKOutdated(&buf, slog.Default())

	if buf.Len() != 0 {
		t.Errorf("expected no output for within-range version, got: %q", buf.String())
	}
}

func TestWarnIfSDKOutdated_NoWarnOnHTTPError(t *testing.T) {
	orig := HTTPGet
	defer func() { HTTPGet = orig }()

	HTTPGet = func(url string) ([]byte, error) {
		return nil, errors.New("network unreachable")
	}

	var buf bytes.Buffer
	// Must not panic or print a warning on network failure.
	WarnIfSDKOutdated(&buf, nil)

	if buf.Len() != 0 {
		t.Errorf("expected no output on HTTP error, got: %q", buf.String())
	}
}

func TestWarnIfSDKOutdated_NilLogger(t *testing.T) {
	orig := HTTPGet
	defer func() { HTTPGet = orig }()

	HTTPGet = func(url string) ([]byte, error) {
		return []byte(`{"info":{"version":"0.3.0"}}`), nil
	}

	var buf bytes.Buffer
	// Must not panic when logger is nil.
	WarnIfSDKOutdated(&buf, nil)

	if !strings.Contains(buf.String(), "WARNING") {
		t.Errorf("expected WARNING even with nil logger, got: %q", buf.String())
	}
}
