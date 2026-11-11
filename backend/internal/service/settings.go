package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/denlink"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// Lost & Found retention modes.
const (
	RetentionOff        = "off"
	RetentionAfterCon   = "after_con"
	RetentionAfterClose = "after_close"
)

// SMTP connection security.
const (
	SMTPTLSStartTLS = "starttls"
	SMTPTLSImplicit = "tls"
	SMTPTLSNone     = "none"
)

// SystemSettings is the wire/JSON representation of system-wide settings.
//
// Adding a setting? Add a field with a JSON tag, a default in
// withDefaults, a validator in settingValidators and, if Den may manage
// it, a mapping in denValues. No migration is needed: the column is JSONB.
type SystemSettings struct {
	// ForceBarcodeOnSignout requires a barcode scan for every sign-out line.
	ForceBarcodeOnSignout bool `json:"force_barcode_on_signout"`

	// OrganizationName is the optional org label used for defaults/exports.
	OrganizationName string `json:"organization_name,omitempty"`

	// DefaultItemOwner applied when creating items with an empty owner.
	DefaultItemOwner string `json:"default_item_owner,omitempty"`

	// DefaultCurrency is an ISO 4217 code used as the item form default.
	DefaultCurrency string `json:"default_currency,omitempty"`

	// BarcodePrefix and BarcodeWidth control generated asset tags (e.g. ST + 6).
	BarcodePrefix string `json:"barcode_prefix,omitempty"`
	BarcodeWidth  int    `json:"barcode_width,omitempty"`

	// PreserveSacredLostFound makes LF-0001 impossible to edit or remove
	// (an easter egg for the first test report). Off by default.
	PreserveSacredLostFound bool `json:"preserve_sacred_lostfound"`

	// ConEndDate (YYYY-MM-DD) is the last day of the convention; the
	// after_con retention counts from the end of that day (UTC).
	ConEndDate string `json:"con_end_date"`
	// LostFoundRetentionMode is off, after_con (delete every report
	// LostFoundRetentionDays after ConEndDate) or after_close (delete a
	// report LostFoundRetentionHours after it was returned or closed).
	LostFoundRetentionMode  string `json:"lostfound_retention_mode"`
	LostFoundRetentionDays  int    `json:"lostfound_retention_days"`
	LostFoundRetentionHours int    `json:"lostfound_retention_hours"`

	// UserInactiveDays anonymises users who have not signed in for this
	// many days (0 = never; people can still be anonymised by hand).
	UserInactiveDays int `json:"user_inactive_days"`

	// TelegramEnabled turns the Lost & Found Telegram bot on (it also
	// needs TELEGRAM_BOT_TOKEN).
	TelegramEnabled bool `json:"telegram_enabled"`

	// OverdueEmailEnabled emails overdue-return reminders.
	OverdueEmailEnabled bool `json:"overdue_email_enabled"`
	// OverdueNotifyEmails always get the overdue digest (comma-separated).
	OverdueNotifyEmails string `json:"overdue_notify_emails"`

	// SMTP server for emails. The password is SMTP_PASSWORD (a secret).
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPFrom     string `json:"smtp_from"`
	SMTPTLS      string `json:"smtp_tls"`

	// ManagedBy is "den" while Stash is paired with Den; ManagedKeys are
	// then read-only here. Never stored.
	ManagedBy   string   `json:"managed_by,omitempty"`
	ManagedKeys []string `json:"managed_keys,omitempty"`
}

// withDefaults fills unset values.
func (s SystemSettings) withDefaults() SystemSettings {
	if s.LostFoundRetentionMode == "" {
		s.LostFoundRetentionMode = RetentionAfterClose
	}
	if s.LostFoundRetentionDays == 0 {
		s.LostFoundRetentionDays = 30
	}
	if s.LostFoundRetentionHours == 0 {
		s.LostFoundRetentionHours = 720
	}
	if s.SMTPPort == 0 {
		s.SMTPPort = 587
	}
	if s.SMTPTLS == "" {
		s.SMTPTLS = SMTPTLSStartTLS
	}
	return s
}

// SettingsSource supplies values from Den when Stash is paired: the
// organisation name and Den's module settings for Stash
// (docs/concepts/module-settings.md).
type SettingsSource interface {
	Paired() bool
	OrganizationName() string
	// ModuleSettings is Den's module_settings from the last bundle (nil
	// when standalone or when Den sends none).
	ModuleSettings() *denlink.ModuleSettings
}

// Den's Lost & Found modes (stash.lost_found.mode) and the contract's
// defaults when Den sets a mode without its count.
const (
	denLostFoundKeptAfterCon        = "kept_after_con"
	denLostFoundDeletedAfterClosing = "deleted_after_closing"
	denLostFoundDefaultDays         = 30
	denLostFoundDefaultHours        = 48
)

// denValues maps Den's module settings to Stash's setting keys:
//
//	stash.lost_found                    lostfound_retention_mode/_days/_hours
//	retention.inactive_users_days       user_inactive_days
//	smtp.host/port/username/security    smtp_host/_port/_username/_tls
//	smtp.from, smtp.from_name           smtp_from ("Name <address>")
//
// Keys Den does not set are left out; Stash then uses its own value.
// The SMTP password is not a setting: the bundle only says password_set.
// denSMTPPassword fetches it when mail is sent.
func denValues(ms *denlink.ModuleSettings) map[string]any {
	out := map[string]any{}
	var lf struct {
		Mode  string `json:"mode"`
		Days  *int   `json:"days"`
		Hours *int   `json:"hours"`
	}
	if ok, err := ms.Decode("stash.lost_found", &lf); ok && err == nil {
		switch lf.Mode {
		case denLostFoundKeptAfterCon:
			out["lostfound_retention_mode"] = RetentionAfterCon
			out["lostfound_retention_days"] = intOr(lf.Days, denLostFoundDefaultDays)
		case denLostFoundDeletedAfterClosing:
			out["lostfound_retention_mode"] = RetentionAfterClose
			out["lostfound_retention_hours"] = intOr(lf.Hours, denLostFoundDefaultHours)
		}
	}
	if d := ms.Retention().InactiveUsersDays; d != nil {
		out["user_inactive_days"] = *d
	}
	if s, ok := ms.SMTP(); ok {
		out["smtp_host"] = s.Host
		out["smtp_port"] = s.Port
		out["smtp_username"] = s.Username
		out["smtp_tls"] = s.Security
		from := s.From
		if s.FromName != "" && s.From != "" {
			from = (&mail.Address{Name: s.FromName, Address: s.From}).String()
		}
		out["smtp_from"] = from
	}
	return out
}

func intOr(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

// denSMTPPassword is the password for one send.
//
// When Den sets smtp.host and password_set, fetch is called (the Den
// client's SMTPPassword) and its result is the password, including an
// empty string when Den's answer has none. When Den sets smtp.host and
// holds no password, the password is empty: Stash does not send
// SMTP_PASSWORD to Den's server. When Den does not provide SMTP
// (standalone, or smtp.host unset), envPassword (SMTP_PASSWORD) is used.
// A nil fetch while Den holds a password is an error.
func denSMTPPassword(ctx context.Context, ms *denlink.ModuleSettings, fetch func(context.Context) (string, error), envPassword string) (string, error) {
	smtp, ok := ms.SMTP()
	if !ok {
		return envPassword, nil
	}
	if !smtp.PasswordSet {
		return "", nil
	}
	if fetch == nil {
		return "", errors.New("den holds the smtp password, but stash cannot fetch it")
	}
	return fetch(ctx)
}

type SettingsService struct {
	pool *pgxpool.Pool
	den  SettingsSource
}

func NewSettingsService(pool *pgxpool.Pool) *SettingsService {
	return &SettingsService{pool: pool}
}

// WithDen makes Get follow Den for the keys Den manages. A nil source keeps
// the service standalone.
func (s *SettingsService) WithDen(den SettingsSource) *SettingsService {
	if den != nil {
		s.den = den
	}
	return s
}

// SMTPPassword is the password overdue mail logs in with. envPassword is
// SMTP_PASSWORD. fetch is the Den client's SMTPPassword; it is called only
// when Den sets smtp.host and password_set. See denSMTPPassword.
func (s *SettingsService) SMTPPassword(ctx context.Context, fetch func(context.Context) (string, error), envPassword string) (string, error) {
	var ms *denlink.ModuleSettings
	if s != nil && s.den != nil && s.den.Paired() {
		ms = s.den.ModuleSettings()
	}
	return denSMTPPassword(ctx, ms, fetch, envPassword)
}

// Get returns the effective settings: Stash's stored values with defaults,
// overridden by Den's values while paired.
func (s *SettingsService) Get(ctx context.Context) (SystemSettings, error) {
	out, err := s.stored(ctx)
	if err != nil {
		return out, err
	}
	out = out.withDefaults()
	if s.den == nil || !s.den.Paired() {
		return out, nil
	}
	out.OrganizationName = s.den.OrganizationName()
	managed := []string{"organization_name"}
	if values := denValues(s.den.ModuleSettings()); len(values) > 0 {
		m, err := toMap(out)
		if err != nil {
			return out, httperror.Internal(err)
		}
		for key, raw := range values {
			v, err := settingValidators[key](raw)
			if err != nil {
				// A bad value in Den must not break Stash; keep ours.
				continue
			}
			m[key] = v
			managed = append(managed, key)
		}
		if err := fromMap(m, &out); err != nil {
			return out, httperror.Internal(err)
		}
	}
	sort.Strings(managed)
	out.ManagedBy, out.ManagedKeys = "den", managed
	return out, nil
}

func (s *SettingsService) stored(ctx context.Context) (SystemSettings, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT data FROM system_settings WHERE id = 1`).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SystemSettings{}, nil
		}
		return SystemSettings{}, httperror.Internal(err)
	}
	var out SystemSettings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			return SystemSettings{}, httperror.Internal(err)
		}
	}
	return out, nil
}

// BarcodeFormat returns prefix + zero-pad width with safe defaults.
func (s SystemSettings) BarcodeFormat() (prefix string, width int) {
	prefix = strings.TrimSpace(s.BarcodePrefix)
	if prefix == "" {
		prefix = "ST"
	}
	width = s.BarcodeWidth
	if width <= 0 {
		width = 6
	}
	return prefix, width
}

// DefaultOwnerOrEmpty returns the configured default item owner (may be empty).
func (s SystemSettings) DefaultOwnerOrEmpty() string {
	return strings.TrimSpace(s.DefaultItemOwner)
}

// DefaultCurrencyOrEUR returns the configured currency or EUR.
func (s SystemSettings) DefaultCurrencyOrEUR() string {
	c := strings.TrimSpace(s.DefaultCurrency)
	if c == "" {
		return "EUR"
	}
	return c
}

// ConEnd returns the end of the convention (end of ConEndDate, UTC).
func (s SystemSettings) ConEnd() (time.Time, bool) {
	d, err := time.Parse("2006-01-02", s.ConEndDate)
	if err != nil {
		return time.Time{}, false
	}
	return d.Add(24 * time.Hour), true
}

// SMTPConfigured reports whether emails can be sent.
func (s SystemSettings) SMTPConfigured() bool {
	return s.SMTPHost != "" && s.SMTPFrom != ""
}

// NotifyEmails returns OverdueNotifyEmails as a list.
func (s SystemSettings) NotifyEmails() []string {
	var out []string
	for _, e := range strings.Split(s.OverdueNotifyEmails, ",") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// ----------------------------------------------------------------------------
// Validation
// ----------------------------------------------------------------------------

type validator func(v any) (any, error)

var (
	currencyRE      = regexp.MustCompile(`^[A-Z]{3}$`)
	barcodePrefixRE = regexp.MustCompile(`^[A-Za-z0-9-]{0,8}$`)
	hostRE          = regexp.MustCompile(`^[A-Za-z0-9.-]{0,253}$`)
)

func vBool(v any) (any, error) {
	b, ok := v.(bool)
	if !ok {
		return nil, errors.New("must be true or false")
	}
	return b, nil
}

func vString(maxLen int, re *regexp.Regexp, hint string) validator {
	return func(v any) (any, error) {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("must be a string")
		}
		s = strings.TrimSpace(s)
		if len(s) > maxLen {
			return nil, fmt.Errorf("must be at most %d characters", maxLen)
		}
		if re != nil && s != "" && !re.MatchString(s) {
			return nil, errors.New(hint)
		}
		return s, nil
	}
}

func vInt(lo, hi int) validator {
	return func(v any) (any, error) {
		f, ok := v.(float64)
		if !ok {
			if i, isInt := v.(int); isInt {
				f, ok = float64(i), true
			}
		}
		if !ok || f != float64(int(f)) {
			return nil, errors.New("must be a whole number")
		}
		if int(f) < lo || int(f) > hi {
			return nil, fmt.Errorf("must be between %d and %d", lo, hi)
		}
		return int(f), nil
	}
}

func vEnum(values ...string) validator {
	return func(v any) (any, error) {
		s, ok := v.(string)
		if !ok || !slices.Contains(values, s) {
			return nil, fmt.Errorf("must be one of %s", strings.Join(values, ", "))
		}
		return s, nil
	}
}

func vDate(v any) (any, error) {
	s, ok := v.(string)
	if !ok {
		return nil, errors.New("must be a date (YYYY-MM-DD) or empty")
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return nil, errors.New("must be a date (YYYY-MM-DD) or empty")
	}
	return s, nil
}

func vEmails(v any) (any, error) {
	s, ok := v.(string)
	if !ok {
		return nil, errors.New("must be a comma-separated list of email addresses")
	}
	var out []string
	for _, e := range strings.Split(s, ",") {
		if e = strings.TrimSpace(e); e == "" {
			continue
		}
		if _, err := mail.ParseAddress(e); err != nil {
			return nil, fmt.Errorf("%q is not an email address", e)
		}
		out = append(out, e)
	}
	if len(out) > 20 {
		return nil, errors.New("at most 20 addresses")
	}
	return strings.Join(out, ", "), nil
}

func vEmail(v any) (any, error) {
	s, ok := v.(string)
	if !ok {
		return nil, errors.New("must be an email address")
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if _, err := mail.ParseAddress(s); err != nil {
		return nil, errors.New("must be an email address, e.g. stash@example.org or Stash <stash@example.org>")
	}
	return s, nil
}

// settingValidators lists every key PATCH accepts.
var settingValidators = map[string]validator{
	"force_barcode_on_signout":  vBool,
	"organization_name":         vString(120, nil, ""),
	"default_item_owner":        vString(120, nil, ""),
	"default_currency":          vString(3, currencyRE, "must be a three-letter ISO 4217 code such as EUR"),
	"barcode_prefix":            vString(8, barcodePrefixRE, "may only contain letters, digits and '-'"),
	"barcode_width":             vInt(3, 12),
	"preserve_sacred_lostfound": vBool,
	"con_end_date":              vDate,
	"lostfound_retention_mode":  vEnum(RetentionOff, RetentionAfterCon, RetentionAfterClose),
	"lostfound_retention_days":  vInt(1, 3650),
	"lostfound_retention_hours": vInt(1, 87600),
	"user_inactive_days":        vInt(0, 3650),
	"telegram_enabled":          vBool,
	"overdue_email_enabled":     vBool,
	"overdue_notify_emails":     vEmails,
	"smtp_host":                 vString(253, hostRE, "must be a host name such as smtp.example.org"),
	"smtp_port":                 vInt(1, 65535),
	"smtp_username":             vString(200, nil, ""),
	"smtp_from":                 vEmail,
	"smtp_tls":                  vEnum(SMTPTLSStartTLS, SMTPTLSImplicit, SMTPTLSNone),
}

// PatchJSON validates a partial JSON object, merges it into the stored
// settings and writes an audit entry. Unknown keys and invalid values are
// rejected with 422 and a per-key message in details.
func (s *SettingsService) PatchJSON(ctx context.Context, patch json.RawMessage) (SystemSettings, error) {
	var patchMap map[string]any
	if err := json.Unmarshal(patch, &patchMap); err != nil || patchMap == nil {
		return SystemSettings{}, httperror.BadRequest("settings patch must be a JSON object")
	}
	delete(patchMap, "managed_by")
	delete(patchMap, "managed_keys")

	paired := s.den != nil && s.den.Paired()
	var managed []string
	if paired {
		cur, err := s.Get(ctx)
		if err != nil {
			return SystemSettings{}, err
		}
		managed = cur.ManagedKeys
	}
	problems := map[string]string{}
	clean := map[string]any{}
	for k, v := range patchMap {
		validate, ok := settingValidators[k]
		if !ok {
			problems[k] = "unknown setting"
			continue
		}
		if slices.Contains(managed, k) {
			problems[k] = "managed in Den (Admin → Module settings)"
			continue
		}
		nv, err := validate(v)
		if err != nil {
			problems[k] = err.Error()
			continue
		}
		clean[k] = nv
	}
	if len(problems) > 0 {
		keys := make([]string, 0, len(problems))
		for k := range problems {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return SystemSettings{}, httperror.Validation("invalid settings: "+strings.Join(keys, ", "), problems)
	}

	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT data FROM system_settings WHERE id = 1 FOR UPDATE`).Scan(&raw); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return httperror.Internal(err)
		}
		before := map[string]any{}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &before); err != nil {
				return httperror.Internal(err)
			}
		}
		after := make(map[string]any, len(before)+len(clean))
		for k, v := range before {
			if _, known := settingValidators[k]; known {
				after[k] = v
			}
		}
		changed := false
		for k, v := range clean {
			if old, ok := after[k]; !ok || fmt.Sprint(old) != fmt.Sprint(v) {
				changed = true
			}
			after[k] = v
		}
		if !changed {
			return nil
		}
		buf, err := json.Marshal(after)
		if err != nil {
			return httperror.Internal(err)
		}
		if _, err := tx.Exec(ctx, `
            INSERT INTO system_settings (id, data, updated_at)
            VALUES (1, $1::jsonb, now())
            ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, updated_at = now()`, string(buf)); err != nil {
			return httperror.Internal(err)
		}
		return audit.Record(ctx, sqlcgen.New(tx), "settings.update", "settings", uuid.Nil, before, after, "")
	})
	if err != nil {
		return SystemSettings{}, err
	}
	return s.Get(ctx)
}

func toMap(v any) (map[string]any, error) {
	buf, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(buf, &m)
}

func fromMap(m map[string]any, out any) error {
	buf, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, out)
}
