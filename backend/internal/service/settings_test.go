package service

import (
	"errors"
	"slices"
	"testing"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

type fakeDen struct {
	paired bool
	data   map[string]any
}

func (f fakeDen) Paired() bool                { return f.paired }
func (f fakeDen) OrganizationName() string    { return "Den Org" }
func (f fakeDen) DenSettings() map[string]any { return f.data }

func TestSettingsDefaults(t *testing.T) {
	pool := testdb.New(t)
	s, err := NewSettingsService(pool).Get(testdb.Actor(t, pool, "Admin"))
	if err != nil {
		t.Fatal(err)
	}
	if s.LostFoundRetentionMode != RetentionAfterClose || s.LostFoundRetentionHours != 720 || s.LostFoundRetentionDays != 30 {
		t.Errorf("retention defaults: %+v", s)
	}
	if s.PreserveSacredLostFound || s.TelegramEnabled || s.OverdueEmailEnabled {
		t.Errorf("opt-in features must default off: %+v", s)
	}
	if s.SMTPPort != 587 || s.SMTPTLS != SMTPTLSStartTLS {
		t.Errorf("smtp defaults: %+v", s)
	}
}

func TestSettingsPatchValidatesAndAudits(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Admin")
	svc := NewSettingsService(pool)

	out, err := svc.PatchJSON(ctx, []byte(`{"lostfound_retention_mode":"after_con","con_end_date":"2026-08-16","lostfound_retention_days":14,"default_currency":"CHF"}`))
	if err != nil {
		t.Fatalf("valid patch: %v", err)
	}
	if out.LostFoundRetentionMode != RetentionAfterCon || out.ConEndDate != "2026-08-16" || out.LostFoundRetentionDays != 14 || out.DefaultCurrency != "CHF" {
		t.Fatalf("patch not applied: %+v", out)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE action = 'settings.update' AND entity_type = 'settings'`); n != 1 {
		t.Fatalf("settings.update audit rows = %d, want 1", n)
	}

	// The same values again change nothing and write no audit row.
	if _, err := svc.PatchJSON(ctx, []byte(`{"lostfound_retention_days":14}`)); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM audit_log WHERE action = 'settings.update'`); n != 1 {
		t.Fatalf("no-op patch wrote an audit row (%d)", n)
	}

	bad := []string{
		`{"is_admin": true}`,
		`{"lostfound_retention_mode": "forever"}`,
		`{"lostfound_retention_days": 0}`,
		`{"barcode_width": 2.5}`,
		`{"default_currency": "euro"}`,
		`{"con_end_date": "16.08.2026"}`,
		`{"smtp_from": "not an address"}`,
		`{"overdue_notify_emails": "a@example.org, nope"}`,
		`{"force_barcode_on_signout": "yes"}`,
		`[1,2]`,
	}
	for _, body := range bad {
		_, err := svc.PatchJSON(ctx, []byte(body))
		if err == nil {
			t.Errorf("%s: accepted", body)
			continue
		}
		var he *httperror.Error
		if !errors.As(err, &he) || (he.Code != httperror.CodeValidation && he.Code != httperror.CodeBadRequest) {
			t.Errorf("%s: want 422/400, got %v", body, err)
		}
	}
}

func TestSettingsFollowDenWhilePaired(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Admin")
	svc := NewSettingsService(pool).WithDen(fakeDen{paired: true, data: map[string]any{
		"stash_lostfound_retention_mode":  "after_con",
		"stash_lostfound_retention_days":  float64(7),
		"stash_con_end_date":              "2026-08-16",
		"stash_smtp_host":                 "smtp.example.org",
		"stash_lostfound_retention_hours": "not a number",
	}})

	s, err := svc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.ManagedBy != "den" || s.OrganizationName != "Den Org" {
		t.Fatalf("not managed by Den: %+v", s)
	}
	if s.LostFoundRetentionMode != RetentionAfterCon || s.LostFoundRetentionDays != 7 || s.SMTPHost != "smtp.example.org" {
		t.Fatalf("Den values not applied: %+v", s)
	}
	if s.LostFoundRetentionHours != 720 || slices.Contains(s.ManagedKeys, "lostfound_retention_hours") {
		t.Fatalf("an invalid Den value must be ignored: %+v", s)
	}

	_, err = svc.PatchJSON(ctx, []byte(`{"lostfound_retention_days": 10}`))
	wantCode(t, err, httperror.CodeValidation)
	if _, err := svc.PatchJSON(ctx, []byte(`{"lostfound_retention_hours": 48}`)); err != nil {
		t.Fatalf("keys Den does not set stay editable: %v", err)
	}
}
