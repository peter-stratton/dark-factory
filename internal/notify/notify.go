package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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
// Each returned Notifier filters sends to only the event types listed in its
// config's Events slice.
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
		eventSet := make(map[string]bool, len(cfg.Events))
		for _, e := range cfg.Events {
			eventSet[e] = true
		}
		notifiers = append(notifiers, &filteredNotifier{notifier: n, events: eventSet})
	}
	return notifiers, nil
}

// filteredNotifier wraps a Notifier and only forwards events whose Type
// appears in the subscribed events set. This implements per-notifier event
// filtering as specified in the notify config's events list.
type filteredNotifier struct {
	notifier Notifier
	events   map[string]bool
}

// Send forwards the event to the underlying notifier only if event.Type is in
// the subscribed events set. Unsubscribed events are silently dropped.
func (f *filteredNotifier) Send(ctx context.Context, event Event) error {
	if !f.events[event.Type] {
		return nil
	}
	return f.notifier.Send(ctx, event)
}

// Fire sends event to all notifiers. Each Send call gets a 10s timeout.
// Errors are logged as warnings and never block execution.
func Fire(ctx context.Context, notifiers []Notifier, event Event, logger *slog.Logger) {
	for _, n := range notifiers {
		sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := n.Send(sendCtx, event)
		cancel()
		if err != nil {
			logger.Warn("notification send failed", "event_type", event.Type, "error", err)
		}
	}
}

// newProvider dispatches to the appropriate provider constructor.
func newProvider(cfg config.NotifyProviderConfig) (Notifier, error) {
	switch cfg.Provider {
	case "telegram":
		return NewTelegram(cfg.Settings)
	default:
		return nil, fmt.Errorf("notify: unknown provider %q", cfg.Provider)
	}
}
