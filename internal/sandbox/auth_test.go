package sandbox

import (
	"encoding/json"
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

	env, err := CollectAuthEnv(slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["ANTHROPIC_API_KEY"] != "sk-test-1234" {
		t.Errorf("ANTHROPIC_API_KEY = %q, want sk-test-1234", env["ANTHROPIC_API_KEY"])
	}
}

func TestCollectAuthEnv_OAuthToken(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return []byte("gho_fake\n"), nil
	})()

	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-token-xyz")
	t.Setenv("GH_TOKEN", "gho_test")

	env, err := CollectAuthEnv(slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["CLAUDE_CODE_OAUTH_TOKEN"] != "oauth-token-xyz" {
		t.Errorf("CLAUDE_CODE_OAUTH_TOKEN = %q, want oauth-token-xyz", env["CLAUDE_CODE_OAUTH_TOKEN"])
	}
}

func TestCollectAuthEnv_BothAuthTokens_PrefersOAuth(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return []byte("gho_fake\n"), nil
	})()

	t.Setenv("ANTHROPIC_API_KEY", "sk-key")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-tok")
	t.Setenv("GH_TOKEN", "gho_test")

	env, err := CollectAuthEnv(slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := env["ANTHROPIC_API_KEY"]; ok {
		t.Error("ANTHROPIC_API_KEY should not be set when OAuth token is available")
	}
	if env["CLAUDE_CODE_OAUTH_TOKEN"] != "oauth-tok" {
		t.Errorf("CLAUDE_CODE_OAUTH_TOKEN = %q, want oauth-tok", env["CLAUDE_CODE_OAUTH_TOKEN"])
	}
}

func TestCollectAuthEnv_NoAuthTokens(t *testing.T) {
	defer stubCommandRunner(func(string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("not logged in")
	})()

	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, err := CollectAuthEnv(slog.Default())
	if err == nil {
		t.Fatal("expected error when no auth tokens set")
	}
	if !strings.Contains(err.Error(), "missing authentication") {
		t.Errorf("error = %q, want 'missing authentication' message", err)
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
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
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
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, err := CollectAuthEnv(slog.Default())
	if err == nil {
		t.Fatal("expected error when GH_TOKEN missing")
	}
	if !strings.Contains(err.Error(), "missing GitHub token") {
		t.Errorf("error = %q, want 'missing GitHub token' message", err)
	}
}

func TestGenerateClaudeConfig(t *testing.T) {
	result := GenerateClaudeConfig("/workspace")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if v, ok := parsed["hasCompletedOnboarding"].(bool); !ok || !v {
		t.Error("hasCompletedOnboarding should be true")
	}
	if v, ok := parsed["theme"].(string); !ok || v != "dark" {
		t.Errorf("theme = %q, want dark", v)
	}
	if v, ok := parsed["numStartups"].(float64); !ok || v != 1 {
		t.Errorf("numStartups = %v, want 1", v)
	}

	projects, ok := parsed["projects"].(map[string]interface{})
	if !ok {
		t.Fatal("projects is not an object")
	}
	ws, ok := projects["/workspace"].(map[string]interface{})
	if !ok {
		t.Fatal("projects[/workspace] is not an object")
	}
	if v, ok := ws["hasTrustDialogAccepted"].(bool); !ok || !v {
		t.Error("hasTrustDialogAccepted should be true")
	}
	if v, ok := ws["hasCompletedProjectOnboarding"].(bool); !ok || !v {
		t.Error("hasCompletedProjectOnboarding should be true")
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

func TestMaskToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sk-1234567890", "sk-1***"},
		{"abc", "abc***"},
		{"", "***"},
	}
	for _, tt := range tests {
		got := maskToken(tt.input)
		if got != tt.want {
			t.Errorf("maskToken(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
