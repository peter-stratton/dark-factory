package judge

import "time"

// Judgment is the action the judge recommends when an intervention fires.
type Judgment string

const (
	Kill           Judgment = "kill"
	RetryContainer Judgment = "retry_container"
	Warn           Judgment = "warn"
	Ignore         Judgment = "ignore"
)

// Intervention is returned by a Rule when it detects a problem.
type Intervention struct {
	Rule       string
	Judgment   Judgment
	Detail     string
	Counts     map[string]int
	DetectedAt time.Time
}

// Config holds judge configuration.
type Config struct {
	IdleTimeoutByRole  map[string]int // seconds, keyed by role name
	DefaultIdleTimeout int            // fallback; default 300

	ToolThrashThreshold  int // number of repeated ToolSearch queries before firing; default 3
	ToolThrashWindowSecs int // sliding window in seconds for tool thrash detection; default 60

	TransportFailureThreshold int // number of stream-closed/stream-error lines before firing; default 10
}

// Rule is implemented by each detection rule.
type Rule interface {
	Name() string
	ProcessLine(line string, now time.Time) *Intervention
}

// Judge holds a set of rules and delegates line processing to them.
type Judge struct {
	role  string
	cfg   Config
	rules []Rule
}

// NewJudge constructs a Judge for the given role and config.
func NewJudge(role string, cfg Config) *Judge {
	if cfg.DefaultIdleTimeout == 0 {
		cfg.DefaultIdleTimeout = 300
	}
	if cfg.ToolThrashThreshold == 0 {
		cfg.ToolThrashThreshold = 3
	}
	if cfg.ToolThrashWindowSecs == 0 {
		cfg.ToolThrashWindowSecs = 60
	}
	if cfg.TransportFailureThreshold == 0 {
		cfg.TransportFailureThreshold = 10
	}
	j := &Judge{role: role, cfg: cfg}
	j.rules = []Rule{
		newIdleTimeoutRule(role, cfg),
		newToolThrashRule(cfg),
		newTransportFailureRule(cfg),
	}
	return j
}

// ProcessLine passes the line to each rule and returns the first non-nil intervention.
func (j *Judge) ProcessLine(line string, now time.Time) *Intervention {
	for _, r := range j.rules {
		if iv := r.ProcessLine(line, now); iv != nil {
			return iv
		}
	}
	return nil
}
