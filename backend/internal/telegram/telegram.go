// Package telegram is a minimal Telegram Bot API client: long polling for
// updates, sending a text message and reading the bot's own name.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the Bot API server.
const DefaultBaseURL = "https://api.telegram.org"

// Client talks to the Bot API with one bot token.
type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// New returns a client; baseURL "" means DefaultBaseURL.
func New(token, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		token:   token,
		baseURL: strings.TrimRight(baseURL, "/"),
		// Long polls wait up to PollTimeout; leave room for the answer.
		http: &http.Client{Timeout: PollTimeout + 15*time.Second},
	}
}

// Configured reports whether a bot token is set.
func (c *Client) Configured() bool { return c != nil && c.token != "" }

// PollTimeout is how long one getUpdates call waits for new messages.
const PollTimeout = 25 * time.Second

// Update is one incoming update; only messages are used.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// Message is a chat message.
type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
}

// Chat identifies where to answer.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// ErrNotConfigured is returned when no token is set.
var ErrNotConfigured = errors.New("telegram bot token not configured")

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
}

func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	endpoint := c.baseURL + "/bot" + url.PathEscape(c.token) + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		// The URL contains the token; never put it in an error.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			return fmt.Errorf("telegram %s: %w", method, uerr.Err)
		}
		return fmt.Errorf("telegram %s: request failed", method)
	}
	defer func() { _ = res.Body.Close() }()
	var r apiResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&r); err != nil {
		return fmt.Errorf("telegram %s: HTTP %d, unreadable answer", method, res.StatusCode)
	}
	if !r.OK {
		return fmt.Errorf("telegram %s: %s", method, r.Description)
	}
	if out != nil {
		return json.Unmarshal(r.Result, out)
	}
	return nil
}

// GetUpdates long-polls for updates after offset.
func (c *Client) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	var out []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         int(PollTimeout / time.Second),
		"allowed_updates": []string{"message"},
	}, &out)
	return out, err
}

// SendMessage sends plain text to a chat.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return c.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, nil)
}

// BotUsername returns the bot's @username (without the @).
func (c *Client) BotUsername(ctx context.Context) (string, error) {
	var me struct {
		Username string `json:"username"`
	}
	if err := c.call(ctx, "getMe", map[string]any{}, &me); err != nil {
		return "", err
	}
	return me.Username, nil
}
