package pypi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// SDKPackage is the PyPI package name for the Claude Agent SDK.
	SDKPackage = "claude-agent-sdk"
)

const maxResponseBytes = 1 << 20 // 1 MiB

// HTTPGet fetches a URL and returns the response body.
// Replaceable for testing.
var HTTPGet = func(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
}

type pypiResponse struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
}

// LatestVersion fetches the latest published version of pkg from PyPI.
func LatestVersion(pkg string) (string, error) {
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", pkg)
	body, err := HTTPGet(url)
	if err != nil {
		return "", fmt.Errorf("fetching PyPI metadata for %s: %w", pkg, err)
	}

	var r pypiResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("parsing PyPI response for %s: %w", pkg, err)
	}
	if r.Info.Version == "" {
		return "", fmt.Errorf("empty version in PyPI response for %s", pkg)
	}
	return r.Info.Version, nil
}

