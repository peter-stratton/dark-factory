package sandbox

import (
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

	oauthToken := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN")
	apiKey := os.Getenv("ANTHROPIC_API_KEY")

	switch {
	case oauthToken != "":
		env["CLAUDE_CODE_OAUTH_TOKEN"] = oauthToken
		logger.Info("using CLAUDE_CODE_OAUTH_TOKEN for Anthropic auth")
	case apiKey != "":
		env["ANTHROPIC_API_KEY"] = apiKey
	default:
		return nil, fmt.Errorf("missing authentication: set ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN")
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
