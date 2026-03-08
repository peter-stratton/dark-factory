package notify

import (
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/config"
)

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

func TestNewFromConfigTelegramNotImplemented(t *testing.T) {
	_, err := NewFromConfig([]config.NotifyProviderConfig{
		{
			Provider: "telegram",
			Events:   []string{"run_complete"},
			Settings: map[string]string{"bot_token": "tok", "chat_id": "123"},
		},
	})
	if err == nil {
		t.Fatal("expected error for unimplemented telegram provider, got nil")
	}
	if !strings.Contains(err.Error(), "telegram") {
		t.Errorf("error = %q, want mention of 'telegram'", err.Error())
	}
}
