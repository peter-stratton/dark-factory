package notify

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/config"
)

// stubNotifier records every event it receives and optionally returns an error.
type stubNotifier struct {
	received []Event
	err      error
}

func (s *stubNotifier) Send(_ context.Context, event Event) error {
	if s.err != nil {
		return s.err
	}
	s.received = append(s.received, event)
	return nil
}

func TestNewFromConfigEmpty(t *testing.T) {
	notifiers, err := NewFromConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error for nil configs: %v", err)
	}
	if notifiers != nil {
		t.Errorf("notifiers = %v, want nil", notifiers)
	}
}

func TestNewFromConfigEmptySlice(t *testing.T) {
	notifiers, err := NewFromConfig([]config.NotifyProviderConfig{})
	if err != nil {
		t.Fatalf("unexpected error for empty configs: %v", err)
	}
	if notifiers != nil {
		t.Errorf("notifiers = %v, want nil", notifiers)
	}
}

func TestNewFromConfigUnknownProvider(t *testing.T) {
	_, err := NewFromConfig([]config.NotifyProviderConfig{
		{Provider: "carrier_pigeon", Events: []string{"run_complete"}},
	})
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "carrier_pigeon") {
		t.Errorf("error = %q, want mention of 'carrier_pigeon'", err.Error())
	}
}

func TestFilteredNotifier_SubscribedEventForwarded(t *testing.T) {
	stub := &stubNotifier{}
	fn := &filteredNotifier{
		notifier: stub,
		events:   map[string]bool{"run_complete": true},
	}

	event := Event{Type: "run_complete", Repo: "owner/repo", Message: "done"}
	if err := fn.Send(context.Background(), event); err != nil {
		t.Fatalf("Send() unexpected error: %v", err)
	}

	if len(stub.received) != 1 {
		t.Fatalf("got %d events, want 1", len(stub.received))
	}
	if stub.received[0].Type != "run_complete" {
		t.Errorf("event type = %q, want %q", stub.received[0].Type, "run_complete")
	}
}

func TestFilteredNotifier_UnsubscribedEventDropped(t *testing.T) {
	stub := &stubNotifier{}
	fn := &filteredNotifier{
		notifier: stub,
		events:   map[string]bool{"abort": true},
	}

	// send run_complete — not subscribed
	if err := fn.Send(context.Background(), Event{Type: "run_complete", Repo: "owner/repo"}); err != nil {
		t.Fatalf("Send() unexpected error: %v", err)
	}

	if len(stub.received) != 0 {
		t.Errorf("got %d events, want 0 (unsubscribed event should be dropped)", len(stub.received))
	}
}

func TestFilteredNotifier_PropagatesError(t *testing.T) {
	wantErr := errors.New("network timeout")
	stub := &stubNotifier{err: wantErr}
	fn := &filteredNotifier{
		notifier: stub,
		events:   map[string]bool{"abort": true},
	}

	err := fn.Send(context.Background(), Event{Type: "abort", Repo: "owner/repo"})
	if !errors.Is(err, wantErr) {
		t.Errorf("Send() error = %v, want %v", err, wantErr)
	}
}
