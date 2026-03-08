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
// authPreference controls which token is preferred when both are set:
// "oauth" (default) prefers CLAUDE_CODE_OAUTH_TOKEN; "api_key" prefers ANTHROPIC_API_KEY.
// requiredEnv is a list of additional environment variable names whose values
// are forwarded from the host environment into the sandbox. Missing values are
// silently skipped (startup validation is a separate concern). Values are never
// logged.
func CollectAuthEnv(logger *slog.Logger, authPreference string, requiredEnv []string) (map[string]string, error) {
	env := make(map[string]string)

	oauthToken := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN")
	apiKey := os.Getenv("ANTHROPIC_API_KEY")

	preferAPIKey := authPreference == "api_key"

	switch {
	case preferAPIKey && apiKey != "":
		env["ANTHROPIC_API_KEY"] = apiKey
		logger.Info("using ANTHROPIC_API_KEY for Anthropic auth")
	case preferAPIKey && oauthToken != "":
		env["CLAUDE_CODE_OAUTH_TOKEN"] = oauthToken
		logger.Info("using CLAUDE_CODE_OAUTH_TOKEN for Anthropic auth (api_key preferred but not set)")
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

	// authManagedVars are controlled exclusively by auth preference logic.
	// required_env must never add or override any of these — even when one
	// was intentionally excluded (e.g. ANTHROPIC_API_KEY excluded under
	// oauth preference) — to prevent auth_preference from being bypassed.
	authManagedVars := map[string]struct{}{
		"ANTHROPIC_API_KEY":       {},
		"CLAUDE_CODE_OAUTH_TOKEN": {},
		"GH_TOKEN":                {},
	}

	// Forward required env vars from the host. Values are not logged to avoid
	// leaking secrets.
	for _, name := range requiredEnv {
		if _, isAuth := authManagedVars[name]; isAuth {
			continue // auth_preference owns these vars exclusively
		}
		if v := os.Getenv(name); v != "" {
			env[name] = v
		}
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	logger.Info("collected auth env", "keys", keys)

	return env, nil
}
