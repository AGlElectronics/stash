package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/telegram"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

type sentMessage struct {
	chat int64
	text string
}

type fakeBot struct {
	updates []telegram.Update
	sent    []sentMessage
	sendErr error
	offsets []int64
}

func (f *fakeBot) Configured() bool { return true }
func (f *fakeBot) GetUpdates(_ context.Context, offset int64) ([]telegram.Update, error) {
	f.offsets = append(f.offsets, offset)
	u := f.updates
	f.updates = nil
	return u, nil
}
func (f *fakeBot) SendMessage(_ context.Context, chat int64, text string) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sent = append(f.sent, sentMessage{chat, text})
	return nil
}
func (f *fakeBot) BotUsername(context.Context) (string, error) { return "stash_lf_bot", nil }

func msg(id, chat int64, text string) telegram.Update {
	return telegram.Update{UpdateID: id, Message: &telegram.Message{Text: text, Chat: telegram.Chat{ID: chat, Type: "private"}}}
}

func TestNormalizeReportNumber(t *testing.T) {
	for in, want := range map[string]string{"LF0042": "LF-0042", "lf-42": "LF-0042", " LF-0042 ": "LF-0042", "LF12345": "LF-12345"} {
		if got, ok := NormalizeReportNumber(in); !ok || got != want {
			t.Errorf("%q → %q %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "LF", "LF-0", "42", "XX0042", "LF-42; DROP"} {
		if _, ok := NormalizeReportNumber(in); ok {
			t.Errorf("%q should be rejected", in)
		}
	}
}

func TestTelegramLinkNotifyUnlink(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Desk")
	settings := NewSettingsService(pool)
	if _, err := settings.PatchJSON(ctx, []byte(`{"telegram_enabled": true}`)); err != nil {
		t.Fatal(err)
	}
	id := mustReport(t, pool, ctx, "Blue backpack", "Alex")
	bot := &fakeBot{}
	lf := NewLostFoundTelegram(pool, settings, bot, zerolog.Nop())

	// Not linked yet.
	wantCode(t, lf.Notify(ctx, id, ""), httperror.CodeConflict)

	bot.updates = []telegram.Update{
		msg(10, 777, "/start LF0001"),
		msg(11, 888, "/start LF0099"),
		msg(12, 999, "hello"),
		{UpdateID: 13, Message: &telegram.Message{Text: "/start LF0001", Chat: telegram.Chat{ID: -5, Type: "group"}}},
	}
	if err := lf.PollOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(bot.sent) != 3 || !strings.Contains(bot.sent[0].text, "Linked to Lost & Found report LF-0001") ||
		!strings.Contains(bot.sent[1].text, "could not find report LF-0099") || !strings.Contains(bot.sent[2].text, "/start LF0042") {
		t.Fatalf("replies: %+v", bot.sent)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM lost_found_reports WHERE id = $1 AND telegram_chat_id = 777`, id); n != 1 {
		t.Fatal("chat not linked")
	}
	// The offset is stored, so the next poll continues after update 13.
	if err := lf.PollOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := bot.offsets[len(bot.offsets)-1]; got != 14 {
		t.Fatalf("next offset = %d, want 14", got)
	}

	bot.sent = nil
	if err := lf.Notify(ctx, id, ""); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(bot.sent) != 1 || bot.sent[0].chat != 777 || !strings.Contains(bot.sent[0].text, "LF-0001 (Blue backpack)") {
		t.Fatalf("notify sent %+v", bot.sent)
	}
	if err := lf.Notify(ctx, id, strings.Repeat("x", MaxNotifyLength+1)); err == nil {
		t.Fatal("over-long messages must be refused")
	}
	if n := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE action = 'lostfound.notify' AND entity_id = $1`, id); n != 1 {
		t.Fatalf("notify audit rows = %d, want 1", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE after_json::text LIKE '%777%' OR before_json::text LIKE '%777%'`); n != 0 {
		t.Fatal("chat ids must not reach the audit log")
	}

	bot.sendErr = errors.New("Forbidden: bot was blocked by the user")
	wantCode(t, lf.Notify(ctx, id, "Hi"), httperror.CodeConflict)
	bot.sendErr = nil

	bot.updates = []telegram.Update{msg(20, 777, "/stop")}
	if err := lf.PollOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM lost_found_reports WHERE telegram_chat_id IS NOT NULL`); n != 0 {
		t.Fatal("/stop must unlink the chat")
	}

	if _, err := settings.PatchJSON(ctx, []byte(`{"telegram_enabled": false}`)); err != nil {
		t.Fatal(err)
	}
	wantCode(t, lf.Notify(ctx, id, ""), httperror.CodeConflict)
	st, err := lf.Status(ctx)
	if err != nil || st.Enabled || !st.Configured {
		t.Fatalf("status: %+v %v", st, err)
	}
}
