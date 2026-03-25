package judge

import (
	"fmt"
	"strings"
	"time"
)

// transportFailureRule fires RetryContainer when enough stream-closed /
// stream-error lines are seen and no tool call lines have been observed,
// indicating that the SDK transport is dead and a fresh container may fix it.
type transportFailureRule struct {
	threshold    int
	streamErrors int
	toolCallSeen bool
}

func newTransportFailureRule(cfg Config) *transportFailureRule {
	return &transportFailureRule{
		threshold: cfg.TransportFailureThreshold,
	}
}

func (r *transportFailureRule) Name() string { return "transport_failure" }

func (r *transportFailureRule) ProcessLine(line string, now time.Time) *Intervention {
	lower := strings.ToLower(line)

	if strings.Contains(line, `"tool":`) {
		r.toolCallSeen = true
	}

	if strings.Contains(lower, "stream closed") || strings.Contains(lower, "stream error") {
		r.streamErrors++
	}

	if r.streamErrors >= r.threshold && !r.toolCallSeen {
		return &Intervention{
			Rule:     r.Name(),
			Judgment: RetryContainer,
			Detail: fmt.Sprintf("%d stream errors observed with no successful tool calls (threshold %d)",
				r.streamErrors, r.threshold),
			Counts:     map[string]int{"stream_errors": r.streamErrors},
			DetectedAt: now,
		}
	}
	return nil
}
