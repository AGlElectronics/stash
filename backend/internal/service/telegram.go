package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
	"github.com/AGlElectronics/stash/backend/internal/telegram"
)

// TelegramAPI is the part of the Bot API the Lost & Found bot uses.
type TelegramAPI interface {
	Configured() bool
	GetUpdates(ctx context.Context, offset int64) ([]telegram.Update, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
	BotUsername(ctx context.Context) (string, error)
}

// LostFoundTelegram is Stash's own Telegram bot for Lost & Found. A guest
// links a report by sending "/start LF0042" (or the report number) to the
// bot; the desk can then notify them. "/stop" forgets the guest's chat on
// every report. Chat ids are deleted with the report by the retention job.
type LostFoundTelegram struct {
	pool     *pgxpool.Pool
	settings *SettingsService
	bot      TelegramAPI
	log      zerolog.Logger

	mu       sync.Mutex
	username string
}

func NewLostFoundTelegram(pool *pgxpool.Pool, settings *SettingsService, bot TelegramAPI, log zerolog.Logger) *LostFoundTelegram {
	return &LostFoundTelegram{pool: pool, settings: settings, bot: bot, log: log}
}

// MaxNotifyLength caps a message from the desk.
const MaxNotifyLength = 1000

var reportNumberRE = regexp.MustCompile(`(?i)^LF-?0*(\d{1,9})$`)

// NormalizeReportNumber turns "LF0042", "lf-42" or "LF-0042" into "LF-0042".
func NormalizeReportNumber(s string) (string, bool) {
	m := reportNumberRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return "", false
	}
	return fmt.Sprintf("LF-%04d", n), true
}

// TelegramStatus is GET /api/lostfound/telegram.
type TelegramStatus struct {
	Enabled     bool   `json:"enabled"`
	Configured  bool   `json:"configured"`
	BotUsername string `json:"bot_username,omitempty"`
}

// Status says whether the bot is usable and its @username.
func (t *LostFoundTelegram) Status(ctx context.Context) (TelegramStatus, error) {
	set, err := t.settings.Get(ctx)
	if err != nil {
		return TelegramStatus{}, err
	}
	st := TelegramStatus{Enabled: set.TelegramEnabled, Configured: t.bot != nil && t.bot.Configured()}
	if st.Enabled && st.Configured {
		st.BotUsername = t.botUsername(ctx)
	}
	return st, nil
}

func (t *LostFoundTelegram) botUsername(ctx context.Context) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.username == "" {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if name, err := t.bot.BotUsername(cctx); err == nil {
			t.username = name
		}
	}
	return t.username
}

func (t *LostFoundTelegram) active(ctx context.Context) (bool, error) {
	if t.bot == nil || !t.bot.Configured() {
		return false, nil
	}
	set, err := t.settings.Get(ctx)
	if err != nil {
		return false, err
	}
	return set.TelegramEnabled, nil
}

// Run long-polls for messages until ctx ends. Call it on one replica only
// (the job leader): Telegram allows one poller per bot.
func (t *LostFoundTelegram) Run(ctx context.Context) {
	for ctx.Err() == nil {
		on, err := t.active(ctx)
		if err != nil || !on {
			sleep(ctx, 30*time.Second)
			continue
		}
		if err := t.PollOnce(ctx); err != nil && ctx.Err() == nil {
			t.log.Warn().Err(err).Msg("telegram: poll failed")
			sleep(ctx, 10*time.Second)
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// PollOnce fetches one batch of updates, handles them and stores the next
// offset so no message is handled twice, even after a leader change.
func (t *LostFoundTelegram) PollOnce(ctx context.Context) error {
	var offset int64
	err := t.pool.QueryRow(ctx, `SELECT update_offset FROM telegram_state WHERE id = 1`).Scan(&offset)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	updates, err := t.bot.GetUpdates(ctx, offset)
	if err != nil {
		return err
	}
	for _, u := range updates {
		if err := t.HandleUpdate(ctx, u); err != nil {
			t.log.Warn().Err(err).Int64("update_id", u.UpdateID).Msg("telegram: handle update")
		}
		if u.UpdateID >= offset {
			offset = u.UpdateID + 1
		}
	}
	if len(updates) == 0 {
		return nil
	}
	_, err = t.pool.Exec(ctx, `
		INSERT INTO telegram_state (id, update_offset, updated_at) VALUES (1, $1, now())
		ON CONFLICT (id) DO UPDATE SET update_offset = EXCLUDED.update_offset, updated_at = now()`, offset)
	return err
}

const (
	msgHelp     = "Hello! Send the number on your Lost & Found receipt, for example /start LF0042, and we will message you here when there is news. Send /stop to unlink."
	msgUnknown  = "We could not find report %s. Please check the number on your receipt, or ask at the Lost & Found desk."
	msgLinked   = "Linked to Lost & Found report %s. We will message you here when there is news. Send /stop to unlink."
	msgUnlinked = "Done: we will no longer message you about Lost & Found reports."
)

// HandleUpdate answers one message. Only private chats are served.
func (t *LostFoundTelegram) HandleUpdate(ctx context.Context, u telegram.Update) error {
	if u.Message == nil || u.Message.Chat.Type != "private" {
		return nil
	}
	chatID := u.Message.Chat.ID
	text := strings.TrimSpace(u.Message.Text)
	cmd, arg, _ := strings.Cut(text, " ")
	cmd = strings.ToLower(strings.SplitN(cmd, "@", 2)[0])

	switch {
	case cmd == "/stop":
		if err := t.unlink(ctx, chatID); err != nil {
			return err
		}
		return t.bot.SendMessage(ctx, chatID, msgUnlinked)
	case cmd == "/start" && strings.TrimSpace(arg) == "", cmd == "/help":
		return t.bot.SendMessage(ctx, chatID, msgHelp)
	}
	candidate := text
	if cmd == "/start" {
		candidate = arg
	}
	number, ok := NormalizeReportNumber(candidate)
	if !ok {
		return t.bot.SendMessage(ctx, chatID, msgHelp)
	}
	linked, err := t.link(ctx, number, chatID)
	if err != nil {
		return err
	}
	if !linked {
		return t.bot.SendMessage(ctx, chatID, fmt.Sprintf(msgUnknown, number))
	}
	return t.bot.SendMessage(ctx, chatID, fmt.Sprintf(msgLinked, number))
}

func (t *LostFoundTelegram) link(ctx context.Context, number string, chatID int64) (bool, error) {
	linked := false
	err := db.WithTx(ctx, t.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		row, err := q.LinkLostFoundTelegram(ctx, sqlcgen.LinkLostFoundTelegramParams{ReportNumber: number, TelegramChatID: &chatID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		linked = true
		return audit.Record(ctx, q, "lostfound.telegram_link", "lostfound", row.ID, nil, nil, "guest linked the report via Telegram")
	})
	return linked, err
}

func (t *LostFoundTelegram) unlink(ctx context.Context, chatID int64) error {
	return db.WithTx(ctx, t.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		ids, err := q.UnlinkTelegramChat(ctx, &chatID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := audit.Record(ctx, q, "lostfound.telegram_unlink", "lostfound", id, nil, nil, "guest sent /stop"); err != nil {
				return err
			}
		}
		return nil
	})
}

// DefaultNotifyMessage is sent when the desk does not write its own text.
func DefaultNotifyMessage(r sqlcgen.LostFoundReport) string {
	return fmt.Sprintf("News about your Lost & Found report %s (%s): please come to the Lost & Found desk.", r.ReportNumber, r.ItemName)
}

// Notify sends a message to the guest who linked the report. Every message
// is audited (lostfound.notify); the text itself is not stored.
func (t *LostFoundTelegram) Notify(ctx context.Context, reportID uuid.UUID, message string) error {
	message = strings.TrimSpace(message)
	if utf8.RuneCountInString(message) > MaxNotifyLength {
		return httperror.Validation(fmt.Sprintf("message must be at most %d characters", MaxNotifyLength), nil)
	}
	on, err := t.active(ctx)
	if err != nil {
		return err
	}
	if !on {
		return httperror.Conflict("the Telegram bot is off (Settings → Privacy and notifications) or TELEGRAM_BOT_TOKEN is not set")
	}
	q := sqlcgen.New(t.pool)
	r, err := q.GetLostFoundReport(ctx, reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httperror.NotFound("lost & found report")
	}
	if err != nil {
		return httperror.Internal(err)
	}
	if r.TelegramChatID == nil {
		return httperror.Conflict("the guest has not linked this report on Telegram yet")
	}
	if message == "" {
		message = DefaultNotifyMessage(r)
	}
	if err := t.bot.SendMessage(ctx, *r.TelegramChatID, message); err != nil {
		t.log.Warn().Err(err).Str("report", r.ReportNumber).Msg("telegram: notify failed")
		return httperror.New(httperror.CodeConflict, "Telegram did not accept the message; the guest may have blocked the bot")
	}
	note := fmt.Sprintf("Telegram message sent (%d characters)", utf8.RuneCountInString(message))
	return audit.Record(ctx, q, "lostfound.notify", "lostfound", reportID, nil, nil, note)
}
