package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/config"
)

// --- NewTelegram constructor tests ---

func TestNewTelegramMissingBotToken(t *testing.T) {
	_, err := NewTelegram(map[string]string{"chat_id": "123"})
	if err == nil {
		t.Fatal("expected error for missing bot_token, got nil")
	}
	if !strings.Contains(err.Error(), "bot_token") {
		t.Errorf("error = %q, want mention of \"bot_token\"", err.Error())
	}
}

func TestNewTelegramEmptyBotToken(t *testing.T) {
	_, err := NewTelegram(map[string]string{"bot_token": "", "chat_id": "123"})
	if err == nil {
		t.Fatal("expected error for empty bot_token, got nil")
	}
	if !strings.Contains(err.Error(), "bot_token") {
		t.Errorf("error = %q, want mention of \"bot_token\"", err.Error())
	}
}

func TestNewTelegramMissingChatID(t *testing.T) {
	_, err := NewTelegram(map[string]string{"bot_token": "tok"})
	if err == nil {
		t.Fatal("expected error for missing chat_id, got nil")
	}
	if !strings.Contains(err.Error(), "chat_id") {
		t.Errorf("error = %q, want mention of \"chat_id\"", err.Error())
	}
}

func TestNewTelegramEmptyChatID(t *testing.T) {
	_, err := NewTelegram(map[string]string{"bot_token": "tok", "chat_id": ""})
	if err == nil {
		t.Fatal("expected error for empty chat_id, got nil")
	}
	if !strings.Contains(err.Error(), "chat_id") {
		t.Errorf("error = %q, want mention of \"chat_id\"", err.Error())
	}
}

func TestNewTelegramSuccess(t *testing.T) {
	n, err := NewTelegram(map[string]string{"bot_token": "tok", "chat_id": "123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n == nil {
		t.Fatal("expected non-nil TelegramNotifier")
	}
}

// --- Send tests ---

func TestSendSuccess(t *testing.T) {
	type telegramPayload struct {
		ChatID    string `json:"chat_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode"`
	}

	var received telegramPayload
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	n := &TelegramNotifier{
		botToken: "tok",
		chatID:   "chat123",
		client:   &http.Client{Transport: rewriteTransport{base: ts.Client().Transport, target: ts.URL}},
	}

	event := Event{Type: "run_complete", Repo: "owner/repo", Message: "all done"}
	err := n.Send(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.ChatID != "chat123" {
		t.Errorf("chat_id = %q, want %q", received.ChatID, "chat123")
	}
	if received.ParseMode != "Markdown" {
		t.Errorf("parse_mode = %q, want \"Markdown\"", received.ParseMode)
	}
	wantText := "✅ *Run complete* — owner/repo\nall done"
	if received.Text != wantText {
		t.Errorf("text = %q, want %q", received.Text, wantText)
	}
}

func TestSendFailureNon2xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	n := &TelegramNotifier{
		botToken: "tok",
		chatID:   "chat123",
		client:   &http.Client{Transport: rewriteTransport{base: ts.Client().Transport, target: ts.URL}},
	}
	err := n.Send(context.Background(), Event{Type: "abort", Repo: "r", Message: "m"})
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error = %q, want mention of status code 403", err.Error())
	}
}

func TestSendContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until request context is done.
		<-r.Context().Done()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	n := &TelegramNotifier{
		botToken: "tok",
		chatID:   "chat123",
		client:   &http.Client{Transport: rewriteTransport{base: ts.Client().Transport, target: ts.URL}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := n.Send(ctx, Event{Type: "abort", Repo: "r", Message: "m"})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// --- Message format tests ---

func TestFormatTelegramMessageRunComplete(t *testing.T) {
	got := formatTelegramMessage(Event{Type: "run_complete", Repo: "owner/repo", Message: "done"})
	want := "✅ *Run complete* — owner/repo\ndone"
	if got != want {
		t.Errorf("formatTelegramMessage() = %q, want %q", got, want)
	}
}

func TestFormatTelegramMessageImplementationComplete(t *testing.T) {
	got := formatTelegramMessage(Event{Type: "implementation_complete", Repo: "owner/repo", Message: "implemented"})
	want := "🔧 *Implementation complete* — owner/repo\nimplemented"
	if got != want {
		t.Errorf("formatTelegramMessage() = %q, want %q", got, want)
	}
}

func TestFormatTelegramMessageAbort(t *testing.T) {
	got := formatTelegramMessage(Event{Type: "abort", Repo: "owner/repo", Message: "error occurred"})
	want := "⛔ *Run aborted* — owner/repo\nerror occurred"
	if got != want {
		t.Errorf("formatTelegramMessage() = %q, want %q", got, want)
	}
}

// --- NewFromConfig registration test ---

func TestNewFromConfigTelegramRegistered(t *testing.T) {
	cfgs := []config.NotifyProviderConfig{
		{
			Provider: "telegram",
			Events:   []string{"run_complete"},
			Settings: map[string]string{"bot_token": "tok", "chat_id": "123"},
		},
	}
	notifiers, err := NewFromConfig(cfgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notifiers) != 1 {
		t.Fatalf("len(notifiers) = %d, want 1", len(notifiers))
	}
	// NewFromConfig wraps each provider in a filteredNotifier for event
	// subscription filtering. Verify the outer wrapper and inner provider types.
	fn, ok := notifiers[0].(*filteredNotifier)
	if !ok {
		t.Fatalf("notifiers[0] type = %T, want *filteredNotifier", notifiers[0])
	}
	if _, ok := fn.notifier.(*TelegramNotifier); !ok {
		t.Errorf("filteredNotifier.notifier type = %T, want *TelegramNotifier", fn.notifier)
	}
}

// rewriteTransport is an http.RoundTripper that rewrites all request URLs to
// point at the given target server, preserving the path and query. This lets
// us redirect Telegram API calls to a local httptest.Server without changing
// the production code's URL construction.
type rewriteTransport struct {
	base   http.RoundTripper
	target string // e.g. "http://127.0.0.1:PORT"
}

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = "http"
	// Strip scheme from target to get host.
	host := strings.TrimPrefix(rt.target, "http://")
	host = strings.TrimPrefix(host, "https://")
	cloned.URL.Host = host
	return rt.base.RoundTrip(cloned)
}
