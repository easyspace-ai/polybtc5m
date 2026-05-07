package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/silver/pmvibes/pkg/polymarket"
)

const apiBase = "https://api.telegram.org/bot"

// Notifier sends messages via a Telegram bot.
type Notifier struct {
	botToken string
	chatID   string
	client   *http.Client
	enabled  bool
}

// NewNotifier creates a new Telegram notifier. If botToken or chatID is empty,
// the notifier is disabled and all Send calls become no-ops.
func NewNotifier(botToken, chatID string) *Notifier {
	enabled := botToken != "" && chatID != ""
	return &Notifier{
		botToken: botToken,
		chatID:   chatID,
		client:   polymarket.NewHTTPClientWithProxy(15 * time.Second),
		enabled:  enabled,
	}
}

// Enabled returns whether the notifier is active.
func (n *Notifier) Enabled() bool { return n.enabled }

// Send dispatches a plain-text message. Errors are logged but not returned
// so that notification failures never block trading logic.
func (n *Notifier) Send(ctx context.Context, text string) {
	if !n.enabled {
		return
	}
	if err := n.send(ctx, text, ""); err != nil {
		slog.Warn("telegram send failed", "err", err)
	}
}

// SendHTML dispatches an HTML-formatted message.
func (n *Notifier) SendHTML(ctx context.Context, text string) {
	if !n.enabled {
		return
	}
	if err := n.send(ctx, text, "HTML"); err != nil {
		slog.Warn("telegram html send failed", "err", err)
	}
}

func (n *Notifier) send(ctx context.Context, text, parseMode string) error {
	payload := map[string]string{
		"chat_id": n.chatID,
		"text":    text,
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	url := apiBase + n.botToken + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram API error: %s", string(respBody))
	}
	return nil
}
