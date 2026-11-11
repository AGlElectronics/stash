package mail

import (
	"context"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBuildStripsHeaderInjection(t *testing.T) {
	from := &mail.Address{Name: "Stash", Address: "stash@example.org"}
	raw, err := Build(from, Message{
		To:      []string{"robin@example.org"},
		Subject: "Hello\r\nBcc: victim@example.org",
		Body:    "line one\nline two",
	}, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "\r\nBcc:") {
		t.Fatalf("header injection: %q", s)
	}
	if !strings.Contains(s, "\r\n\r\nline one\r\nline two") {
		t.Fatalf("body not CRLF-normalised: %q", s)
	}
	if _, err := Build(from, Message{To: []string{"not an address"}}, time.Now()); err == nil {
		t.Fatal("bad recipients must be refused")
	}
}

// TestSendToLocalServer sends through a local test SMTP server (mailpit in
// the dev stack) when STASH_TEST_SMTP_ADDR is set, e.g. localhost:1025.
func TestSendToLocalServer(t *testing.T) {
	addr := os.Getenv("STASH_TEST_SMTP_ADDR")
	if addr == "" {
		t.Skip("STASH_TEST_SMTP_ADDR not set")
	}
	host, portStr, _ := strings.Cut(addr, ":")
	port, _ := strconv.Atoi(portStr)
	err := SMTP{}.Send(context.Background(), Config{Host: host, Port: port, From: "Stash <stash@example.org>", TLS: "none"},
		Message{To: []string{"robin@example.org"}, Subject: "Stash test", Body: "It works."})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
}
