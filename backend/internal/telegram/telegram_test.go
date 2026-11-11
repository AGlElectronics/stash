package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientCallsBotAPI(t *testing.T) {
	var gotPaths []string
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		switch {
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":5,"message":{"text":"/start LF0042","chat":{"id":9,"type":"private"}}}]}`))
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			_ = json.NewDecoder(r.Body).Decode(&sent)
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"username":"stash_bot"}}`))
		}
	}))
	defer srv.Close()

	c := New("123:secret", srv.URL)
	ups, err := c.GetUpdates(context.Background(), 4)
	if err != nil || len(ups) != 1 || ups[0].Message.Chat.ID != 9 {
		t.Fatalf("updates: %+v %v", ups, err)
	}
	if err := c.SendMessage(context.Background(), 9, "hi"); err != nil {
		t.Fatal(err)
	}
	if sent["chat_id"].(float64) != 9 || sent["text"] != "hi" {
		t.Fatalf("sent %v", sent)
	}
	if name, err := c.BotUsername(context.Background()); err != nil || name != "stash_bot" {
		t.Fatalf("username %q %v", name, err)
	}
	if gotPaths[0] != "/bot123:secret/getUpdates" {
		t.Fatalf("path %q", gotPaths[0])
	}
}

func TestClientErrorsDoNotLeakToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Forbidden: bot was blocked by the user"}`))
	}))
	c := New("123:secret", srv.URL)
	err := c.SendMessage(context.Background(), 1, "x")
	if err == nil || !strings.Contains(err.Error(), "blocked") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err = %v", err)
	}
	srv.Close()
	err = c.SendMessage(context.Background(), 1, "x")
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("connection error leaks the token: %v", err)
	}
	if New("", "").Configured() {
		t.Fatal("no token means not configured")
	}
	if err := New("", "").SendMessage(context.Background(), 1, "x"); err != ErrNotConfigured {
		t.Fatalf("err = %v", err)
	}
}
