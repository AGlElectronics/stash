package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/denlink"
	"github.com/AGlElectronics/stash/backend/internal/mail"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

type fakeSender struct{ sent []mail.Message }

func (f *fakeSender) Send(_ context.Context, cfg mail.Config, m mail.Message) error {
	if cfg.Password != "smtp-secret" || cfg.Host != "smtp.example.org" {
		panic("unexpected SMTP config")
	}
	f.sent = append(f.sent, m)
	return nil
}

func TestOverdueRemindersOncePerDay(t *testing.T) {
	f := newSignoutFixture(t)
	pool := f.pool
	receiver := testdb.Actor(t, pool, "Robin")
	ru, _ := auth.FromContext(receiver)
	if _, err := pool.Exec(context.Background(), `UPDATE users SET email = 'robin@example.org' WHERE id = $1`, ru.ID); err != nil {
		t.Fatal(err)
	}
	item := mustItem(t, pool, "Radio", 5)
	due := time.Now().Add(-2 * time.Hour)
	so, err := f.svc.Create(f.ctx, CreateSignoutInput{
		ToDepartmentID: f.dept, ExpectedReturnAt: &due, ReceivedByUserID: &ru.ID, ReceivedByName: "Robin",
		Items: []SignoutItemInput{line(item.ID, 2)}, Confirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(48 * time.Hour)
	f.svc.Create(f.ctx, CreateSignoutInput{ToDepartmentID: f.dept, ExpectedReturnAt: &future, Items: []SignoutItemInput{line(item.ID, 1)}, Confirm: true}) //nolint:errcheck

	settings := NewSettingsService(pool)
	sender := &fakeSender{}
	svc := NewOverdueService(pool, settings, sender, "smtp-secret", "https://stash.example.org", zerolog.Nop())

	list, err := svc.List(context.Background(), time.Now())
	if err != nil || len(list) != 1 || list[0].SignoutNumber != so.Signout.SignoutNumber {
		t.Fatalf("overdue list: %+v %v", list, err)
	}

	// Off by default: nothing is sent.
	if n, err := svc.SendReminders(context.Background(), time.Now()); err != nil || n != 0 || len(sender.sent) != 0 {
		t.Fatalf("emails off: %d %v %d", n, err, len(sender.sent))
	}
	if _, err := settings.PatchJSON(f.ctx, []byte(`{"overdue_email_enabled":true,"smtp_host":"smtp.example.org","smtp_from":"Stash <stash@example.org>","overdue_notify_emails":"logistics@example.org"}`)); err != nil {
		t.Fatal(err)
	}
	n, err := svc.SendReminders(context.Background(), time.Now())
	if err != nil || n != 1 {
		t.Fatalf("reminders: %d %v", n, err)
	}
	if len(sender.sent) != 2 || sender.sent[0].To[0] != "robin@example.org" || sender.sent[1].To[0] != "logistics@example.org" {
		t.Fatalf("sent: %+v", sender.sent)
	}
	if !strings.Contains(sender.sent[0].Body, "https://stash.example.org/signouts/"+so.Signout.ID.String()) {
		t.Fatalf("receiver email lacks the link: %s", sender.sent[0].Body)
	}
	if n, _ := svc.SendReminders(context.Background(), time.Now().Add(time.Hour)); n != 0 {
		t.Fatal("a sign-out must not be reminded twice within a day")
	}
	if n, _ := svc.SendReminders(context.Background(), time.Now().Add(25*time.Hour)); n != 1 {
		t.Fatal("the next day it is reminded again")
	}
	if c := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE action = 'signout.overdue_reminder'`); c != 2 {
		t.Fatalf("reminder audit rows = %d, want 2", c)
	}
}

type recordingSender struct {
	cfgs []mail.Config
	sent []mail.Message
}

func (r *recordingSender) Send(_ context.Context, cfg mail.Config, m mail.Message) error {
	r.cfgs = append(r.cfgs, cfg)
	r.sent = append(r.sent, m)
	return nil
}

func TestOverdueMailUsesDenPassword(t *testing.T) {
	f := newSignoutFixture(t)
	pool := f.pool
	receiver := testdb.Actor(t, pool, "Robin")
	ru, _ := auth.FromContext(receiver)
	if _, err := pool.Exec(context.Background(), `UPDATE users SET email = 'robin@example.org' WHERE id = $1`, ru.ID); err != nil {
		t.Fatal(err)
	}
	item := mustItem(t, pool, "Radio", 5)
	due := time.Now().Add(-2 * time.Hour)
	if _, err := f.svc.Create(f.ctx, CreateSignoutInput{
		ToDepartmentID: f.dept, ExpectedReturnAt: &due, ReceivedByUserID: &ru.ID, ReceivedByName: "Robin",
		Items: []SignoutItemInput{line(item.ID, 1)}, Confirm: true,
	}); err != nil {
		t.Fatal(err)
	}

	settings := NewSettingsService(pool).WithDen(fakeDen{paired: true, module: `{
		"data": {"smtp": {"host": "den-smtp.example.org", "port": 2525, "username": "den",
			"password_set": true, "security": "starttls",
			"from": "stash@con.example.org", "from_name": "Stash"}}
	}`})
	if _, err := settings.PatchJSON(f.ctx, []byte(`{"overdue_email_enabled":true,"smtp_host":"local.example.org","smtp_from":"local@example.org","overdue_notify_emails":"logistics@example.org"}`)); err != nil {
		t.Fatal(err)
	}
	sender := &recordingSender{}
	var calls int
	svc := NewOverdueService(pool, settings, sender, "env-secret", "https://stash.example.org", zerolog.Nop()).
		WithDenPassword(func(context.Context) (string, error) {
			calls++
			if calls == 1 {
				return "", denlink.ErrRevoked
			}
			return "from-den", nil
		})

	n, err := svc.SendReminders(context.Background(), time.Now())
	if !errors.Is(err, denlink.ErrRevoked) || n != 0 || len(sender.sent) != 0 {
		t.Fatalf("revoked fetch: n=%d err=%v sent=%d", n, err, len(sender.sent))
	}
	if denlink.UserMessage(err) == err.Error() {
		t.Fatalf("admin sentence collapsed into the Go error: %q", err.Error())
	}
	if c := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE action = 'signout.overdue_reminder'`); c != 0 {
		t.Fatalf("a failed password fetch must not count as a reminder, rows=%d", c)
	}

	n, err = svc.SendReminders(context.Background(), time.Now())
	if err != nil || n != 1 || len(sender.cfgs) != 2 {
		t.Fatalf("send: n=%d err=%v cfgs=%d", n, err, len(sender.cfgs))
	}
	for _, cfg := range sender.cfgs {
		if cfg.Host != "den-smtp.example.org" || cfg.Port != 2525 || cfg.Username != "den" || cfg.Password != "from-den" {
			t.Fatalf("mail used %+v, want Den's server and password", cfg)
		}
		if cfg.Password == "env-secret" || !strings.Contains(cfg.From, "stash@con.example.org") {
			t.Fatalf("from/password: %+v", cfg)
		}
	}
}
