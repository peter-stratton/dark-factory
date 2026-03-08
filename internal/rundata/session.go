package rundata

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// FindSessionID scans run directories under runsDir for the given repo and
// returns the most recent session ID stored for the given PR number.
// It inspects outcome.json files to match the PR, then reads the session ID
// from the latest retry step or the initial implement step.
// Returns ("", nil) when no matching session ID is found (caller should cold-start).
func FindSessionID(runsDir, repo string, prNum int) (string, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("repo must be in owner/name format, got %q", repo)
	}
	owner, repoName := parts[0], parts[1]

	repoDir := filepath.Join(runsDir, owner, repoName)
	tsEntries, err := os.ReadDir(repoDir)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading repo dir: %w", err)
	}

	// Sort descending by directory name (timestamp format YYYYMMDD-HHMMSS)
	// so most-recent run is first.
	sort.Slice(tsEntries, func(i, j int) bool {
		return tsEntries[i].Name() > tsEntries[j].Name()
	})

	r := &Reader{logger: slog.Default(), baseDir: runsDir}

	for _, tsEntry := range tsEntries {
		if !tsEntry.IsDir() {
			continue
		}
		runDir := filepath.Join(repoDir, tsEntry.Name())
		issuesDir := filepath.Join(runDir, "issues")

		issueEntries, err := os.ReadDir(issuesDir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			continue
		}

		for _, issueEntry := range issueEntries {
			if !issueEntry.IsDir() {
				continue
			}
			issueDir := filepath.Join(issuesDir, issueEntry.Name())

			outcome := r.readOutcome(filepath.Join(issueDir, "outcome.json"))
			if outcome.PRNumber != prNum {
				continue
			}

			// Found matching PR — extract session ID from most recent step.
			if sid := latestRetrySessionID(r, issueDir); sid != "" {
				return sid, nil
			}
			impl := r.readStep(filepath.Join(issueDir, "implement.json"))
			if impl.SessionID != "" {
				return impl.SessionID, nil
			}
		}
	}

	return "", nil
}

// latestRetrySessionID returns the session ID from the highest-numbered retry
// directory, or "" if none exist or none carry a session ID.
func latestRetrySessionID(r *Reader, issueDir string) string {
	retriesDir := filepath.Join(issueDir, "retries")
	entries, err := os.ReadDir(retriesDir)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		return ""
	}

	maxAttempt := -1
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		attempt, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		if attempt > maxAttempt {
			maxAttempt = attempt
		}
	}

	if maxAttempt < 0 {
		return ""
	}

	step := r.readStep(filepath.Join(retriesDir, strconv.Itoa(maxAttempt), "retry.json"))
	return step.SessionID
}
