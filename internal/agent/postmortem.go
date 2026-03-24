package agent

import (
	"fmt"
	"strings"

	"github.com/peter-stratton/dark-factory/internal/rundata"
)

const (
	// maxPostMortemLines is the maximum number of log lines captured for post-mortem.
	maxPostMortemLines = 500
	// maxPostMortemBytes is the maximum byte size of captured log (~100KB).
	maxPostMortemBytes = 100 * 1024
)

// AnalyzeFailure scans container log output and returns structured failure findings.
// It is best-effort: the caller should not rely on this for correctness, only for
// diagnostic signal.
func AnalyzeFailure(log string, exitCode int) rundata.FailureAnalysis {
	lines := splitLines(log)

	var patterns []rundata.FailurePattern

	// Pattern: stream_closed — API connection drops.
	streamClosedCount := countContaining(lines, "stream closed")
	if streamClosedCount > 0 {
		severity := "medium"
		if streamClosedCount >= 50 {
			severity = "high"
		}
		patterns = append(patterns, rundata.FailurePattern{
			Code:     "stream_closed",
			Count:    streamClosedCount,
			Severity: severity,
			Message:  fmt.Sprintf("%d stream disconnects detected — likely context exhaustion or API instability", streamClosedCount),
		})
	}

	// Pattern: zero_tool_calls — stream died before any tool execution.
	// Detected by high stream_closed count with no tool audit log lines.
	toolCallCount := countContaining(lines, `"tool":`)
	if streamClosedCount >= 10 && toolCallCount == 0 {
		patterns = append(patterns, rundata.FailurePattern{
			Code:     "zero_tool_calls",
			Count:    1,
			Severity: "high",
			Message:  "agent never completed a single tool call — CLI transport failure on startup",
		})
	}

	// Pattern: api_overloaded — Anthropic API capacity issue (HTTP 529).
	if n := countContaining(lines, "529", "overloaded"); n > 0 {
		patterns = append(patterns, rundata.FailurePattern{
			Code:     "api_overloaded",
			Count:    n,
			Severity: "high",
			Message:  fmt.Sprintf("%d API overload responses detected — Anthropic API capacity issue", n),
		})
	}

	// Pattern: api_server_error — server-side API errors (5xx).
	if n := countContaining(lines, "503", "500", "server error", "internal server error"); n > 0 {
		patterns = append(patterns, rundata.FailurePattern{
			Code:     "api_server_error",
			Count:    n,
			Severity: "medium",
			Message:  fmt.Sprintf("%d API server errors detected", n),
		})
	}

	// Pattern: rate_limited — client is being rate limited.
	if n := countContaining(lines, "rate_limit", "rate-limit", "too many requests"); n > 0 {
		patterns = append(patterns, rundata.FailurePattern{
			Code:     "rate_limited",
			Count:    n,
			Severity: "medium",
			Message:  fmt.Sprintf("%d rate limit responses detected", n),
		})
	}

	// Pattern: high_subagent_count — heavy parallelization, possible context pressure.
	if n := countContaining(lines, "parent_tool_use_id"); n > 0 {
		severity := "low"
		if n >= 400 {
			severity = "high"
		} else if n >= 200 {
			severity = "medium"
		}
		patterns = append(patterns, rundata.FailurePattern{
			Code:     "high_subagent_count",
			Count:    n,
			Severity: severity,
			Message:  fmt.Sprintf("%d subagent spawns — heavy parallelization may have contributed to context pressure", n),
		})
	}

	// Pattern: timed_out — agent hit the configured timeout.
	if n := countContaining(lines, "timed_out", "timed out", "timeout"); n > 0 {
		patterns = append(patterns, rundata.FailurePattern{
			Code:     "timed_out",
			Count:    n,
			Severity: "medium",
			Message:  fmt.Sprintf("%d timeout signals detected", n),
		})
	}

	// Pattern: context_exhausted — explicit context window exhaustion.
	if n := countContaining(lines, "context length", "context_length", "token limit", "max_tokens"); n > 0 {
		patterns = append(patterns, rundata.FailurePattern{
			Code:     "context_exhausted",
			Count:    n,
			Severity: "high",
			Message:  fmt.Sprintf("%d context/token limit signals detected — context window exhausted", n),
		})
	}

	// Pattern: nonzero_exit — process crash or error exit.
	if exitCode != 0 {
		patterns = append(patterns, rundata.FailurePattern{
			Code:     "nonzero_exit",
			Count:    1,
			Severity: "medium",
			Message:  fmt.Sprintf("container exited with code %d", exitCode),
		})
	}

	return rundata.FailureAnalysis{
		Patterns:          patterns,
		LogLinesCaptured:  len(lines),
		ContainerExitCode: exitCode,
	}
}

// boundLog returns the last n lines of log, also capped to maxBytes.
// Lines are separated by newline characters. Returned string preserves
// line structure without a trailing newline.
func boundLog(log string, maxLines, maxBytes int) string {
	// Byte-limit first to avoid splitting huge inputs line by line.
	if len(log) > maxBytes {
		log = log[len(log)-maxBytes:]
		// Re-align to a line boundary (skip partial first line).
		if idx := strings.Index(log, "\n"); idx >= 0 {
			log = log[idx+1:]
		}
	}

	lines := strings.Split(log, "\n")

	// Remove trailing empty line that results from a trailing newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	return strings.Join(lines, "\n")
}

// splitLines splits s into individual lines, handling both \n and empty input.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	// Remove trailing empty entry from a trailing newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// countContaining counts the number of lines that contain any of the given
// substrings (case-insensitive search using ToLower on each line).
func countContaining(lines []string, substrs ...string) int {
	n := 0
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, sub := range substrs {
			if strings.Contains(lower, strings.ToLower(sub)) {
				n++
				break // count each line at most once per pattern group
			}
		}
	}
	return n
}
