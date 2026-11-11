// Package mail sends plain-text emails over SMTP (STARTTLS, implicit TLS
// or, for a local test server only, no TLS).
package mail

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Config is one SMTP server.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// TLS is "starttls", "tls" or "none".
	TLS string
}

// Message is one email.
type Message struct {
	To      []string
	Subject string
	Body    string
}

// Sender sends messages; tests replace it.
type Sender interface {
	Send(ctx context.Context, cfg Config, m Message) error
}

// SMTP is the real Sender.
type SMTP struct{}

const dialTimeout = 15 * time.Second

// Send delivers m through cfg's server.
func (SMTP) Send(ctx context.Context, cfg Config, m Message) error {
	if cfg.Host == "" || cfg.From == "" {
		return errors.New("smtp: host and from address are required")
	}
	if len(m.To) == 0 {
		return errors.New("smtp: no recipients")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return fmt.Errorf("smtp: from address: %w", err)
	}
	raw, err := Build(from, m, time.Now())
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	dialer := &net.Dialer{Timeout: dialTimeout}
	tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}

	var conn net.Conn
	if cfg.TLS == "tls" {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp: connect %s: %w", addr, err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(time.Minute))
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp: %w", err)
	}
	defer func() { _ = c.Close() }()
	if cfg.TLS == "starttls" {
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("smtp: STARTTLS: %w", err)
		}
	}
	if cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return fmt.Errorf("smtp: login: %w", err)
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return fmt.Errorf("smtp: MAIL FROM: %w", err)
	}
	for _, to := range m.To {
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("smtp: RCPT TO: %w", err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp: DATA: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("smtp: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: send: %w", err)
	}
	return c.Quit()
}

// Build renders the message with headers. Header values are stripped of
// line breaks so nothing can be injected.
func Build(from *mail.Address, m Message, now time.Time) ([]byte, error) {
	var to []string
	for _, a := range m.To {
		addr, err := mail.ParseAddress(a)
		if err != nil {
			return nil, fmt.Errorf("smtp: recipient %q: %w", a, err)
		}
		to = append(to, addr.String())
	}
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	domain := from.Address[strings.LastIndexByte(from.Address, '@')+1:]
	var b strings.Builder
	header := func(k, v string) {
		b.WriteString(k + ": " + strings.NewReplacer("\r", "", "\n", "").Replace(v) + "\r\n")
	}
	header("From", from.String())
	header("To", strings.Join(to, ", "))
	header("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(id)+"@"+domain+">")
	header("MIME-Version", "1.0")
	header("Content-Type", `text/plain; charset="utf-8"`)
	header("Content-Transfer-Encoding", "8bit")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(strings.ReplaceAll(m.Body, "\r\n", "\n"), "\n", "\r\n"))
	return []byte(b.String()), nil
}
