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
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	t.Setenv("GH_TOKEN", "gho_test")

	env, err := CollectAuthEnv(slog.Default(), "oauth")
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
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, err := CollectAuthEnv(slog.Default(), "oauth")
	if err == nil {
		t.Fatal("expected error when no auth tokens set")
	}
	if !strings.Contains(err.Error(), "missing authentication") {
		t.Errorf("error = %q, want 'missing authentication' message", err)
	}
}

func TestCollectAuthEnv_OAuthTokenOnly(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return []byte("gho_fake\n"), nil
	})()

	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-token-abc")
	t.Setenv("GH_TOKEN", "gho_test")

	env, err := CollectAuthEnv(slog.Default(), "oauth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["CLAUDE_CODE_OAUTH_TOKEN"] != "oauth-token-abc" {
		t.Errorf("CLAUDE_CODE_OAUTH_TOKEN = %q, want oauth-token-abc", env["CLAUDE_CODE_OAUTH_TOKEN"])
	}
	if _, ok := env["ANTHROPIC_API_KEY"]; ok {
		t.Error("ANTHROPIC_API_KEY should not be set when OAuth token is used")
	}
}

func TestCollectAuthEnv_OAuthPreferredOverAPIKey(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return []byte("gho_fake\n"), nil
	})()

	t.Setenv("ANTHROPIC_API_KEY", "sk-key")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-token-abc")
	t.Setenv("GH_TOKEN", "gho_test")

	env, err := CollectAuthEnv(slog.Default(), "oauth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["CLAUDE_CODE_OAUTH_TOKEN"] != "oauth-token-abc" {
		t.Errorf("CLAUDE_CODE_OAUTH_TOKEN = %q, want oauth-token-abc", env["CLAUDE_CODE_OAUTH_TOKEN"])
	}
	if _, ok := env["ANTHROPIC_API_KEY"]; ok {
		t.Error("ANTHROPIC_API_KEY should not be set when OAuth token takes priority")
	}
}

func TestCollectAuthEnv_APIKeyPreferredOverOAuth(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return []byte("gho_fake\n"), nil
	})()

	t.Setenv("ANTHROPIC_API_KEY", "sk-key")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-token-abc")
	t.Setenv("GH_TOKEN", "gho_test")

	env, err := CollectAuthEnv(slog.Default(), "api_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["ANTHROPIC_API_KEY"] != "sk-key" {
		t.Errorf("ANTHROPIC_API_KEY = %q, want sk-key", env["ANTHROPIC_API_KEY"])
	}
	if _, ok := env["CLAUDE_CODE_OAUTH_TOKEN"]; ok {
		t.Error("CLAUDE_CODE_OAUTH_TOKEN should not be set when API key takes priority")
	}
}

func TestCollectAuthEnv_APIKeyPreference_FallsBackToOAuth(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return []byte("gho_fake\n"), nil
	})()

	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-token-abc")
	t.Setenv("GH_TOKEN", "gho_test")

	env, err := CollectAuthEnv(slog.Default(), "api_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["CLAUDE_CODE_OAUTH_TOKEN"] != "oauth-token-abc" {
		t.Errorf("CLAUDE_CODE_OAUTH_TOKEN = %q, want oauth-token-abc", env["CLAUDE_CODE_OAUTH_TOKEN"])
	}
	if _, ok := env["ANTHROPIC_API_KEY"]; ok {
		t.Error("ANTHROPIC_API_KEY should not be set when falling back to OAuth")
	}
}

func TestCollectAuthEnv_GHTokenFromEnv(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		t.Error("CommandRunner should not be called when GH_TOKEN is set")
		return nil, fmt.Errorf("should not be called")
	})()

	t.Setenv("ANTHROPIC_API_KEY", "sk-key")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	t.Setenv("GH_TOKEN", "gho_from_env")

	env, err := CollectAuthEnv(slog.Default(), "oauth")
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
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	env, err := CollectAuthEnv(slog.Default(), "oauth")
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
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, err := CollectAuthEnv(slog.Default(), "oauth")
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
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	t.Setenv("GH_TOKEN", "gho_secret_token_789")

	var buf strings.Builder
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)

	env, err := CollectAuthEnv(logger, "oauth")
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
