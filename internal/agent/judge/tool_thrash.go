package judge

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// queryRe extracts the "query" JSON field value from a ToolSearch audit line.
var queryRe = regexp.MustCompile(`"query"\s*:\s*"([^"]*)"`)

// toolThrashRule fires Kill when the same ToolSearch query is seen 3+ times
// within a sliding window, indicating the agent is stuck searching for a tool
// that does not exist.
type toolThrashRule struct {
	threshold  int
	windowSecs int
	// queryTimes maps a normalised query string to the timestamps of recent calls.
	queryTimes map[string][]time.Time
}

func newToolThrashRule(cfg Config) *toolThrashRule {
	return &toolThrashRule{
		threshold:  cfg.ToolThrashThreshold,
		windowSecs: cfg.ToolThrashWindowSecs,
		queryTimes: make(map[string][]time.Time),
	}
}

func (r *toolThrashRule) Name() string { return "tool_thrash" }

func (r *toolThrashRule) ProcessLine(line string, now time.Time) *Intervention {
	if !strings.Contains(line, "ToolSearch") {
		return nil
	}

	query := r.extractQuery(line)
	if query == "" {
		return nil // can't determine query — don't count toward thrash detection
	}
	window := time.Duration(r.windowSecs) * time.Second
	cutoff := now.Add(-window)

	times := append(r.queryTimes[query], now)
	pruned := times[:0]
	for _, ts := range times {
		if !ts.Before(cutoff) {
			pruned = append(pruned, ts)
		}
	}
	if len(pruned) == 0 {
		delete(r.queryTimes, query)
	} else {
		r.queryTimes[query] = pruned
	}

	if len(pruned) >= r.threshold {
		return &Intervention{
			Rule:     r.Name(),
			Judgment: Kill,
			Detail: fmt.Sprintf("ToolSearch query %q repeated %d times within %ds window",
				query, len(pruned), r.windowSecs),
			Counts:     map[string]int{"repeated_searches": len(pruned)},
			DetectedAt: now,
		}
	}
	return nil
}

// extractQuery pulls the query string from a ToolSearch log line.
// Returns empty string if no JSON query field is found.
func (r *toolThrashRule) extractQuery(line string) string {
	if m := queryRe.FindStringSubmatch(line); len(m) == 2 {
		return m[1]
	}
	return ""
}
