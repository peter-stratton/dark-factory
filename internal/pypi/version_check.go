package pypi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// SDKPackage is the PyPI package name for the Claude Agent SDK.
	SDKPackage = "claude-agent-sdk"

	// SDKPinnedUpperBound is the exclusive upper bound of the version range
	// pinned in the generated Dockerfile (">=0.1.0,<0.2.0").
	SDKPinnedUpperBound = "0.2.0"

	// SDKPinnedRange is the full pinned specifier shown in warnings.
	SDKPinnedRange = ">=0.1.0,<0.2.0"
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

// parseSemver parses a "X.Y.Z" version string into its three components.
// A two-part "X.Y" string is treated as "X.Y.0".
func parseSemver(v string) (major, minor, patch int, err error) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, 0, fmt.Errorf("invalid version %q: expected at least X.Y", v)
	}
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	if major, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major in %q: %w", v, err)
	}
	if minor, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor in %q: %w", v, err)
	}
	if patch, err = strconv.Atoi(parts[2]); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch in %q: %w", v, err)
	}
	return major, minor, patch, nil
}

// ExceedsPinnedRange reports whether latest is >= upperBound in semver order.
// upperBound corresponds to the exclusive upper bound of the pinned range
// (e.g. "0.2.0" for the pin ">=0.1.0,<0.2.0").
func ExceedsPinnedRange(latest, upperBound string) (bool, error) {
	lMaj, lMin, lPatch, err := parseSemver(latest)
	if err != nil {
		return false, fmt.Errorf("parsing latest version: %w", err)
	}
	uMaj, uMin, uPatch, err := parseSemver(upperBound)
	if err != nil {
		return false, fmt.Errorf("parsing upper bound: %w", err)
	}

	if lMaj != uMaj {
		return lMaj > uMaj, nil
	}
	if lMin != uMin {
		return lMin > uMin, nil
	}
	return lPatch >= uPatch, nil
}

// WarnIfSDKOutdated checks PyPI for the latest claude-agent-sdk version and,
// if it meets or exceeds the pinned upper bound, writes a warning to w and
// records a structured log entry. Errors are silently ignored — the check is
// best-effort and must not block startup.
func WarnIfSDKOutdated(w io.Writer, logger *slog.Logger) {
	latest, err := LatestVersion(SDKPackage)
	if err != nil {
		if logger != nil {
			logger.Debug("SDK version check skipped", "pkg", SDKPackage, "error", err)
		}
		return
	}

	exceeded, err := ExceedsPinnedRange(latest, SDKPinnedUpperBound)
	if err != nil {
		if logger != nil {
			logger.Debug("SDK version comparison failed", "pkg", SDKPackage, "error", err)
		}
		return
	}

	if exceeded {
		fmt.Fprintf(w, "WARNING: %s %s is available on PyPI — pinned range %s may need updating\n",
			SDKPackage, latest, SDKPinnedRange)
		if logger != nil {
			logger.Warn("SDK version exceeds pinned range",
				"package", SDKPackage,
				"latest", latest,
				"pin", SDKPinnedRange,
			)
		}
	}
}
