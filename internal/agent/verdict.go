package agent

import "strings"

// ParseVerdict scans agent stdout for a result line matching the given keyword
// (e.g. "REVIEW" or "QUALITY") and returns "APPROVED", "CHANGES_REQUESTED",
// "BLOCKED_BY_PROTECTED_PATH", or "" if no sentinel is found.
// Matching is case-insensitive; first match wins.
func ParseVerdict(stdout, keyword string) string {
	upper := strings.ToUpper(stdout)
	kw := strings.ToUpper(keyword)
	for _, line := range strings.Split(upper, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "BLOCKED_BY_PROTECTED_PATH") && strings.Contains(line, kw) && strings.Contains(line, "RESULT") {
			return "BLOCKED_BY_PROTECTED_PATH"
		}
		if strings.Contains(line, "APPROVED") && strings.Contains(line, kw) && strings.Contains(line, "RESULT") {
			if strings.Contains(line, "CHANGES") {
				return "CHANGES_REQUESTED"
			}
			return "APPROVED"
		}
		if strings.Contains(line, "CHANGES_REQUESTED") && strings.Contains(line, kw) {
			return "CHANGES_REQUESTED"
		}
	}
	return ""
}
