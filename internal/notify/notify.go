package notify

import (
	"context"
	"fmt"

	"github.com/phs/dark-factory/internal/config"
)

// Event describes a notification to be sent. Type matches one of the event
// names defined in the notify config ("run_complete", "implementation_complete",
// "abort").
type Event struct {
	Type    string
	Repo    string
	Message string
}

// Notifier is implemented by each notification provider.
type Notifier interface {
	Send(ctx context.Context, event Event) error
}

// NewFromConfig builds a Notifier for each entry in configs by dispatching to
// the appropriate provider constructor. An error is returned if any entry
// names an unknown provider. An empty (or nil) slice returns (nil, nil).
func NewFromConfig(configs []config.NotifyProviderConfig) ([]Notifier, error) {
	if len(configs) == 0 {
		return nil, nil
	}
	notifiers := make([]Notifier, 0, len(configs))
	for _, cfg := range configs {
		n, err := newProvider(cfg)
		if err != nil {
			return nil, err
		}
		notifiers = append(notifiers, n)
	}
	return notifiers, nil
}

// newProvider dispatches to the appropriate provider constructor.
func newProvider(cfg config.NotifyProviderConfig) (Notifier, error) {
	switch cfg.Provider {
	case "telegram":
		// Telegram provider constructor will be added in a follow-up issue.
		return nil, fmt.Errorf("notify: telegram provider not yet implemented")
	default:
		return nil, fmt.Errorf("notify: unknown provider %q", cfg.Provider)
	}
}
