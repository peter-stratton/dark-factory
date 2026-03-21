package ghapp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

// CommandRunner executes a command and returns its combined output.
// Replaceable for testing.
var CommandRunner = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// Config holds the GitHub App credentials needed to generate installation tokens.
type Config struct {
	AppID          string
	InstallationID string
	PrivateKeyPath string
}

// activeConfig holds the GitHub App config set during auth collection.
// Used by RefreshToken to mint fresh installation tokens on demand.
// Access is atomic because multiple agent goroutines may call RefreshToken
// concurrently.
var activeConfig atomic.Pointer[Config]

// SetActiveConfig stores the GitHub App config for later token refresh.
func SetActiveConfig(cfg *Config) {
	activeConfig.Store(cfg)
}

// RefreshToken generates a fresh GitHub App installation token using the
// stored config. Returns ("", nil) when no GitHub App config is active
// (i.e. the run is using a PAT or gh CLI auth instead).
func RefreshToken() (string, error) {
	cfg := activeConfig.Load()
	if cfg == nil {
		return "", nil
	}
	return cfg.InstallationToken()
}

// LoadFromEnv reads GitHub App credentials from environment variables.
// Returns nil if none of the variables are set (app auth not configured).
// Returns an error if some but not all variables are set.
func LoadFromEnv() (*Config, error) {
	appID := os.Getenv("GODARK_APP_ID")
	installID := os.Getenv("GODARK_APP_INSTALLATION_ID")
	keyPath := os.Getenv("GODARK_APP_PRIVATE_KEY_PATH")

	set := 0
	if appID != "" {
		set++
	}
	if installID != "" {
		set++
	}
	if keyPath != "" {
		set++
	}

	if set == 0 {
		return nil, nil
	}
	if set < 3 {
		var missing []string
		if appID == "" {
			missing = append(missing, "GODARK_APP_ID")
		}
		if installID == "" {
			missing = append(missing, "GODARK_APP_INSTALLATION_ID")
		}
		if keyPath == "" {
			missing = append(missing, "GODARK_APP_PRIVATE_KEY_PATH")
		}
		return nil, fmt.Errorf("incomplete GitHub App config: missing %s", strings.Join(missing, ", "))
	}

	return &Config{
		AppID:          appID,
		InstallationID: installID,
		PrivateKeyPath: keyPath,
	}, nil
}

// InstallationToken generates a short-lived GitHub installation token by:
// 1. Reading the private key from disk
// 2. Creating a JWT signed with RS256
// 3. Exchanging it for an installation token via the GitHub API
//
// The returned token is valid for 1 hour.
func (c *Config) InstallationToken() (string, error) {
	keyData, err := os.ReadFile(c.PrivateKeyPath)
	if err != nil {
		return "", fmt.Errorf("reading private key: %w", err)
	}

	key, err := parsePrivateKey(keyData)
	if err != nil {
		return "", fmt.Errorf("parsing private key: %w", err)
	}

	jwt, err := signJWT(c.AppID, key)
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}

	token, err := exchangeToken(c.InstallationID, jwt)
	if err != nil {
		return "", fmt.Errorf("exchanging installation token: %w", err)
	}

	return token, nil
}

func parsePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format as fallback.
		parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("parsing private key (PKCS1: %v, PKCS8: %v)", err, err2)
		}
		rsaKey, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
		return rsaKey, nil
	}
	return key, nil
}

func signJWT(appID string, key *rsa.PrivateKey) (string, error) {
	now := time.Now()
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]interface{}{
		"iat": now.Add(-60 * time.Second).Unix(), // 60 seconds in the past for clock drift
		"exp": now.Add(10 * time.Minute).Unix(),  // max 10 minutes
		"iss": appID,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64
	hash := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("signing: %w", err)
	}

	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + sigB64, nil
}

// installationTokenResponse is the JSON response from GitHub's
// POST /app/installations/{id}/access_tokens endpoint.
type installationTokenResponse struct {
	Token string `json:"token"`
}

// exchangeToken exchanges a JWT for a short-lived installation access token
// using the GitHub API via curl (avoids gh CLI which may not be authed yet).
func exchangeToken(installationID, jwt string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/app/installations/%s/access_tokens", installationID)
	out, err := CommandRunner("curl", "-s", "-X", "POST",
		"-H", "Accept: application/vnd.github+json",
		"-H", fmt.Sprintf("Authorization: Bearer %s", jwt),
		url,
	)
	if err != nil {
		return "", fmt.Errorf("curl: %w\noutput: %s", err, out)
	}

	var resp installationTokenResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("parsing token response: %w\noutput: %s", err, out)
	}
	if resp.Token == "" {
		return "", fmt.Errorf("empty token in response: %s", out)
	}

	return resp.Token, nil
}
