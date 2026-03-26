package judge

import (
	"fmt"
	"time"
)

// idleTimeoutRule fires Kill when no log output has been seen for longer than
// the configured threshold. Any non-empty line resets the timer — the goal is
// to detect stalled streams, not to police which tools the agent calls.
type idleTimeoutRule struct {
	role         string
	threshold    time.Duration
	lastActivity time.Time // zero until first ProcessLine call
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
		r.lastActivity = now
		r.started = true
	}
	if line != "" {
		r.lastActivity = now
	}
	idle := now.Sub(r.lastActivity)
	if idle > r.threshold {
		return &Intervention{
			Rule:     r.Name(),
			Judgment: Kill,
			Detail: fmt.Sprintf("no output for %s (threshold %s, role %q)",
				idle.Round(time.Second), r.threshold, r.role),
			Counts:     map[string]int{"idle_seconds": int(idle.Seconds())},
			DetectedAt: now,
		}
	}
	return nil
}
