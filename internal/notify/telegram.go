package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TelegramNotifier sends notifications via the Telegram Bot API.
type TelegramNotifier struct {
	botToken string
	chatID   string
	client   *http.Client
}

// NewTelegram constructs a TelegramNotifier from the given settings map.
// Required keys: "bot_token" and "chat_id". Returns an error if either is
// missing or empty.
func NewTelegram(settings map[string]string) (*TelegramNotifier, error) {
	botToken := settings["bot_token"]
	if botToken == "" {
		return nil, fmt.Errorf("notify: telegram: missing required setting \"bot_token\"")
	}
	chatID := settings["chat_id"]
	if chatID == "" {
		return nil, fmt.Errorf("notify: telegram: missing required setting \"chat_id\"")
	}
	return &TelegramNotifier{botToken: botToken, chatID: chatID, client: &http.Client{}}, nil
}

// Send delivers event to the configured Telegram chat via the Bot API
// sendMessage endpoint. It respects ctx cancellation and returns an error for
// any non-2xx HTTP response.
func (t *TelegramNotifier) Send(ctx context.Context, event Event) error {
	text := formatTelegramMessage(event)

	payload := struct {
		ChatID    string `json:"chat_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode"`
	}{
		ChatID:    t.chatID,
		Text:      text,
		ParseMode: "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notify: telegram: marshal payload: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify: telegram: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		// Sanitize error to prevent the bot token from appearing in log output.
		// Go's net/http embeds the request URL verbatim in transport errors.
		sanitized := strings.ReplaceAll(err.Error(), t.botToken, "[REDACTED]")
		return fmt.Errorf("notify: telegram: send request: %s", sanitized)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body) // drain to allow TCP connection reuse
		resp.Body.Close()                      //nolint:errcheck // best-effort close; ignore error
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notify: telegram: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// formatTelegramMessage returns the Markdown-formatted message text for event.
func formatTelegramMessage(event Event) string {
	switch event.Type {
	case "run_complete":
		return fmt.Sprintf("✅ *Run complete* — %s\n%s", event.Repo, event.Message)
	case "implementation_complete":
		return fmt.Sprintf("🔧 *Implementation complete* — %s\n%s", event.Repo, event.Message)
	case "abort":
		return fmt.Sprintf("⛔ *Run aborted* — %s\n%s", event.Repo, event.Message)
	default:
		return fmt.Sprintf("*%s* — %s\n%s", event.Type, event.Repo, event.Message)
	}
}
