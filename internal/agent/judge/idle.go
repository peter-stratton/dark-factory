package judge

import (
	"fmt"
	"strings"
	"time"
)

// idleTimeoutRule fires Kill when no line containing `"tool":` has been seen
// for longer than the configured threshold.
type idleTimeoutRule struct {
	role         string
	threshold    time.Duration
	lastToolCall time.Time // zero until first ProcessLine call
	started      bool
}

func newIdleTimeoutRule(role string, cfg Config) *idleTimeoutRule {
	secs := cfg.DefaultIdleTimeout
	if v, ok := cfg.IdleTimeoutByRole[role]; ok {
		secs = v
	}
	return &idleTimeoutRule{
		role:      role,
		threshold: time.Duration(secs) * time.Second,
	}
}

func (r *idleTimeoutRule) Name() string { return "idle_timeout" }

func (r *idleTimeoutRule) ProcessLine(line string, now time.Time) *Intervention {
	if !r.started {
		// First line starts the clock.
		r.lastToolCall = now
		r.started = true
	}
	if strings.Contains(line, `"tool":`) {
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
