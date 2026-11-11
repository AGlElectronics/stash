package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/mail"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// OverdueSignout is a sign-out whose expected return has passed.
type OverdueSignout struct {
	ID               uuid.UUID `json:"id"`
	SignoutNumber    string    `json:"signout_number"`
	ToDepartmentName string    `json:"to_department_name"`
	ReceivedByName   string    `json:"received_by_name"`
	ExpectedReturnAt string    `json:"expected_return_at"`
	receiverEmail    string
	notifiedAt       pgtype.Timestamptz
}

// OverdueService lists overdue sign-outs (shown in the app) and emails
// reminders once a day when overdue emails are on.
type OverdueService struct {
	pool         *pgxpool.Pool
	settings     *SettingsService
	sender       mail.Sender
	smtpPassword string
	denPassword  func(context.Context) (string, error)
	appBaseURL   string
	log          zerolog.Logger
}

func NewOverdueService(pool *pgxpool.Pool, settings *SettingsService, sender mail.Sender, smtpPassword, appBaseURL string, log zerolog.Logger) *OverdueService {
	return &OverdueService{pool: pool, settings: settings, sender: sender, smtpPassword: smtpPassword, appBaseURL: strings.TrimRight(appBaseURL, "/"), log: log}
}

// WithDenPassword fetches Den's SMTP password when Den sets smtp.host and
// password_set. fetch is denlink.Client.SMTPPassword. Without it, a Den
// mail server that holds a password cannot be used.
func (s *OverdueService) WithDenPassword(fetch func(context.Context) (string, error)) *OverdueService {
	s.denPassword = fetch
	return s
}

// List returns every overdue sign-out at time now, the oldest first.
func (s *OverdueService) List(ctx context.Context, now time.Time) ([]OverdueSignout, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.signout_number, d.name, coalesce(s.received_by_name, u.display_name, ''),
		       s.expected_return_at, coalesce(CASE WHEN u.anonymised_at IS NULL THEN u.email END, ''),
		       s.overdue_notified_at
		  FROM department_signouts s
		  JOIN departments d ON d.id = s.to_department_id
		  LEFT JOIN users u ON u.id = s.received_by_user_id
		 WHERE s.status IN ('signed_out','partially_returned')
		   AND s.expected_return_at IS NOT NULL AND s.expected_return_at < $1
		 ORDER BY s.expected_return_at, s.id`, now)
	if err != nil {
		return nil, httperror.Internal(err)
	}
	defer rows.Close()
	out := []OverdueSignout{}
	for rows.Next() {
		var o OverdueSignout
		var due pgtype.Timestamptz
		if err := rows.Scan(&o.ID, &o.SignoutNumber, &o.ToDepartmentName, &o.ReceivedByName, &due, &o.receiverEmail, &o.notifiedAt); err != nil {
			return nil, httperror.Internal(err)
		}
		o.ExpectedReturnAt = due.Time.UTC().Format(time.RFC3339)
		out = append(out, o)
	}
	return out, rows.Err()
}

// ReminderInterval is the minimum time between two reminders for one
// sign-out.
const ReminderInterval = 24 * time.Hour

// SendReminders emails each overdue sign-out's receiver (when Stash knows
// their email) and sends one digest to overdue_notify_emails. Each
// sign-out is reminded at most once per ReminderInterval. It returns the
// number of sign-outs reminded.
func (s *OverdueService) SendReminders(ctx context.Context, now time.Time) (int, error) {
	set, err := s.settings.Get(ctx)
	if err != nil {
		return 0, err
	}
	if !set.OverdueEmailEnabled || !set.SMTPConfigured() {
		return 0, nil
	}
	all, err := s.List(ctx, now)
	if err != nil {
		return 0, err
	}
	var due []OverdueSignout
	for _, o := range all {
		if !o.notifiedAt.Valid || now.Sub(o.notifiedAt.Time) >= ReminderInterval {
			due = append(due, o)
		}
	}
	if len(due) == 0 {
		return 0, nil
	}
	password, err := s.settings.SMTPPassword(ctx, s.denPassword, s.smtpPassword)
	if err != nil {
		return 0, err
	}
	cfg := mail.Config{
		Host: set.SMTPHost, Port: set.SMTPPort, Username: set.SMTPUsername,
		Password: password, From: set.SMTPFrom, TLS: set.SMTPTLS,
	}
	sent := 0
	for _, o := range due {
		recipients := 0
		if o.receiverEmail != "" {
			err := s.sender.Send(ctx, cfg, mail.Message{
				To:      []string{o.receiverEmail},
				Subject: fmt.Sprintf("Please return the gear from sign-out %s", o.SignoutNumber),
				Body:    s.receiverBody(o),
			})
			if err != nil {
				s.log.Warn().Err(err).Str("signout", o.SignoutNumber).Msg("overdue: email to receiver failed")
			} else {
				recipients++
			}
		}
		if err := s.markReminded(ctx, o, now, recipients); err != nil {
			return sent, err
		}
		sent++
	}
	if list := set.NotifyEmails(); len(list) > 0 {
		if err := s.sender.Send(ctx, cfg, mail.Message{
			To:      list,
			Subject: fmt.Sprintf("%d overdue sign-out(s) in Stash", len(due)),
			Body:    s.digestBody(due),
		}); err != nil {
			s.log.Warn().Err(err).Msg("overdue: digest email failed")
		}
	}
	return sent, nil
}

func (s *OverdueService) markReminded(ctx context.Context, o OverdueSignout, now time.Time, recipients int) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE department_signouts SET overdue_notified_at = $2 WHERE id = $1`, o.ID, now); err != nil {
		return err
	}
	note := fmt.Sprintf("overdue reminder emailed to the receiver (%d) and the digest list", recipients)
	if err := audit.Record(ctx, sqlcgen.New(tx), "signout.overdue_reminder", "signout", o.ID, nil, nil, note); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// link returns " <url>" for the sign-out page, or "" without APP_BASE_URL.
func (s *OverdueService) link(o OverdueSignout) string {
	if s.appBaseURL == "" {
		return ""
	}
	return " " + s.appBaseURL + "/signouts/" + o.ID.String()
}

func (s *OverdueService) receiverBody(o OverdueSignout) string {
	return fmt.Sprintf("Hello %s,\n\nthe gear from sign-out %s (%s) was due back on %s and has not been fully returned yet.\nPlease bring it back to logistics, or tell them if anything was used up, lost or damaged.\n%s\n\nThank you!",
		nonEmpty(o.ReceivedByName, "there"), o.SignoutNumber, o.ToDepartmentName, o.ExpectedReturnAt, strings.TrimSpace(s.link(o)))
}

func (s *OverdueService) digestBody(list []OverdueSignout) string {
	var b strings.Builder
	b.WriteString("These sign-outs are past their expected return:\n\n")
	for _, o := range list {
		fmt.Fprintf(&b, "- %s → %s, received by %s, due %s%s\n", o.SignoutNumber, o.ToDepartmentName,
			nonEmpty(o.ReceivedByName, "(unknown)"), o.ExpectedReturnAt, s.link(o))
	}
	return b.String()
}

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
