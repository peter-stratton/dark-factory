package sandbox

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
)

// CollectAuthEnv reads authentication tokens from the host environment
// and returns a map suitable for passing as container environment variables.
func CollectAuthEnv(logger *slog.Logger) (map[string]string, error) {
	env := make(map[string]string)

	// Anthropic auth: at least one of these must be set.
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		env["ANTHROPIC_API_KEY"] = v
	}
	if v := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"); v != "" {
		env["CLAUDE_CODE_OAUTH_TOKEN"] = v
	}
	if _, hasAPI := env["ANTHROPIC_API_KEY"]; !hasAPI {
		if _, hasOAuth := env["CLAUDE_CODE_OAUTH_TOKEN"]; !hasOAuth {
			return nil, fmt.Errorf("missing authentication: set ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN")
		}
	}

	// GitHub token: try env first, then gh CLI fallback.
	if v := os.Getenv("GH_TOKEN"); v != "" {
		env["GH_TOKEN"] = v
	} else {
		out, err := CommandRunner("gh", "auth", "token")
		if err != nil {
			return nil, fmt.Errorf("missing GitHub token: set GH_TOKEN or run gh auth login")
		}
		token := strings.TrimSpace(string(out))
		if token == "" {
			return nil, fmt.Errorf("missing GitHub token: set GH_TOKEN or run gh auth login")
		}
		env["GH_TOKEN"] = token
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	logger.Info("collected auth env", "keys", keys)

	return env, nil
}

// maskToken returns the first 4 characters of s followed by "***".
// Useful for debug logging without exposing full secrets.
func maskToken(s string) string {
	if len(s) <= 4 {
		return s + "***"
	}
	return s[:4] + "***"
}

// claudeConfig is the structure written to ~/.claude.json inside the container.
type claudeConfig struct {
	HasCompletedOnboarding bool                      `json:"hasCompletedOnboarding"`
	Theme                  string                    `json:"theme"`
	NumStartups            int                       `json:"numStartups"`
	Projects               map[string]projectConfig  `json:"projects"`
}

type projectConfig struct {
	HasTrustDialogAccepted         bool `json:"hasTrustDialogAccepted"`
	HasCompletedProjectOnboarding  bool `json:"hasCompletedProjectOnboarding"`
}

// GenerateClaudeConfig returns the JSON content for ~/.claude.json inside the
// container. The workDir parameter becomes the key in the projects map.
func GenerateClaudeConfig(workDir string) string {
	cfg := claudeConfig{
		HasCompletedOnboarding: true,
		Theme:                  "dark",
		NumStartups:            1,
		Projects: map[string]projectConfig{
			workDir: {
				HasTrustDialogAccepted:        true,
				HasCompletedProjectOnboarding: true,
			},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return string(data)
}
