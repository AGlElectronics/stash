package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/AGlElectronics/stash/backend/internal/auth"
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
