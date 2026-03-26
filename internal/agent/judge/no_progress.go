package judge

import (
	"fmt"
	"strings"
	"time"
)

// noProgressRule fires Kill when no tool call has been seen for longer than the
// configured threshold. Unlike idleTimeoutRule (which watches for stream
// silence), this rule detects agents that are streaming text endlessly without
// making progress. Only lines containing a tool audit record reset the timer.
type noProgressRule struct {
	role         string
	threshold    time.Duration
	lastToolCall time.Time
	started      bool
}

func newNoProgressRule(role string, cfg Config) *noProgressRule {
	secs := cfg.DefaultNoProgressTimeout
	if v, ok := cfg.NoProgressTimeoutByRole[role]; ok {
		secs = v
	}
	return &noProgressRule{
		role:      role,
		threshold: time.Duration(secs) * time.Second,
	}
}

func (r *noProgressRule) Name() string { return "no_progress" }

func (r *noProgressRule) ProcessLine(line string, now time.Time) *Intervention {
	if !r.started {
		r.lastToolCall = now
		r.started = true
	}
	// Match tool activity from either format:
	//   SDK audit:   {"tool": "Read", ...}
	//   CLI stream:  {"type":"tool_use", "name":"Read", ...}  (inside assistant message)
	if strings.Contains(line, `"tool":`) || strings.Contains(line, `"tool_use"`) {
		r.lastToolCall = now
	}
	idle := now.Sub(r.lastToolCall)
	if idle > r.threshold {
		return &Intervention{
			Rule:     r.Name(),
			Judgment: Kill,
			Detail: fmt.Sprintf("no tool call for %s (threshold %s, role %q)",
				idle.Round(time.Second), r.threshold, r.role),
			Counts:     map[string]int{"idle_seconds": int(idle.Seconds())},
			DetectedAt: now,
		}
	}
	return nil
}
