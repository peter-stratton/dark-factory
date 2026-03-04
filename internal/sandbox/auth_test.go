package sandbox

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// stubCommandRunner saves the original CommandRunner and returns a restore func.
func stubCommandRunner(fn func(string, ...string) ([]byte, error)) func() {
	orig := CommandRunner
	CommandRunner = fn
	return func() { CommandRunner = orig }
}

func TestCollectAuthEnv_APIKey(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return []byte("gho_fake\n"), nil
	})()

	t.Setenv("ANTHROPIC_API_KEY", "sk-test-1234")
	t.Setenv("GH_TOKEN", "gho_test")

	env, err := CollectAuthEnv(slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["ANTHROPIC_API_KEY"] != "sk-test-1234" {
		t.Errorf("ANTHROPIC_API_KEY = %q, want sk-test-1234", env["ANTHROPIC_API_KEY"])
	}
}

func TestCollectAuthEnv_NoAuthTokens(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("not logged in")
	})()

	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GH_TOKEN", "")

	_, err := CollectAuthEnv(slog.Default())
	if err == nil {
		t.Fatal("expected error when no auth tokens set")
	}
	if !strings.Contains(err.Error(), "missing authentication") {
		t.Errorf("error = %q, want 'missing authentication' message", err)
	}
}

// TestCollectAuthEnv_OAuthTokenNotFallback confirms that setting only
// CLAUDE_CODE_OAUTH_TOKEN (without ANTHROPIC_API_KEY) is not accepted.
func TestCollectAuthEnv_OAuthTokenNotFallback(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return []byte("gho_fake\n"), nil
	})()

	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GH_TOKEN", "gho_test")

	_, err := CollectAuthEnv(slog.Default())
	if err == nil {
		t.Fatal("expected error: ANTHROPIC_API_KEY is required; OAuth token is not a fallback")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("error = %q, want mention of ANTHROPIC_API_KEY", err)
	}
}

func TestCollectAuthEnv_GHTokenFromEnv(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		t.Error("CommandRunner should not be called when GH_TOKEN is set")
		return nil, fmt.Errorf("should not be called")
	})()

	t.Setenv("ANTHROPIC_API_KEY", "sk-key")
	t.Setenv("GH_TOKEN", "gho_from_env")

	env, err := CollectAuthEnv(slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["GH_TOKEN"] != "gho_from_env" {
		t.Errorf("GH_TOKEN = %q, want gho_from_env", env["GH_TOKEN"])
	}
}

func TestCollectAuthEnv_GHTokenFallback(t *testing.T) {
	defer stubCommandRunner(func(name string, args ...string) ([]byte, error) {
		if name == "gh" && len(args) >= 2 && args[0] == "auth" && args[1] == "token" {
			return []byte("gho_from_cli\n"), nil
		}
		return nil, fmt.Errorf("unexpected command: %s %v", name, args)
	})()

	t.Setenv("ANTHROPIC_API_KEY", "sk-key")
	t.Setenv("GH_TOKEN", "")

	env, err := CollectAuthEnv(slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["GH_TOKEN"] != "gho_from_cli" {
		t.Errorf("GH_TOKEN = %q, want gho_from_cli", env["GH_TOKEN"])
	}
}

func TestCollectAuthEnv_GHTokenMissing(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("not logged in")
	})()

	t.Setenv("ANTHROPIC_API_KEY", "sk-key")
	t.Setenv("GH_TOKEN", "")

	_, err := CollectAuthEnv(slog.Default())
	if err == nil {
		t.Fatal("expected error when GH_TOKEN missing")
	}
	if !strings.Contains(err.Error(), "missing GitHub token") {
		t.Errorf("error = %q, want 'missing GitHub token' message", err)
	}
}

func TestCollectAuthEnv_NoSecretsInLog(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return []byte("gho_fake\n"), nil
	})()

	t.Setenv("ANTHROPIC_API_KEY", "sk-secret-key-12345")
	t.Setenv("GH_TOKEN", "gho_secret_token_789")

	var buf strings.Builder
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)

	env, err := CollectAuthEnv(logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := buf.String()

	// Log should contain key names but not raw token values.
	if !strings.Contains(logOutput, "ANTHROPIC_API_KEY") {
		t.Error("log should mention ANTHROPIC_API_KEY key name")
	}
	if strings.Contains(logOutput, env["ANTHROPIC_API_KEY"]) {
		t.Error("log should not contain raw ANTHROPIC_API_KEY value")
	}
	if strings.Contains(logOutput, env["GH_TOKEN"]) {
		t.Error("log should not contain raw GH_TOKEN value")
	}
}
