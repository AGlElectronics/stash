package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/AGlElectronics/stash/backend/internal/denlink"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

type fakeDen struct {
	paired bool
	// module is the module_settings JSON of Den's bundle.
	module string
}

func (f fakeDen) Paired() bool             { return f.paired }
func (f fakeDen) OrganizationName() string { return "Den Org" }
func (f fakeDen) ModuleSettings() *denlink.ModuleSettings {
	if f.module == "" {
		return nil
	}
	var ms denlink.ModuleSettings
	_ = json.Unmarshal([]byte(f.module), &ms)
	return &ms
}

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

func TestSMTPPasswordFollowsDen(t *testing.T) {
	ctx := context.Background()
	const (
		withPassword = `{"data":{"smtp":{"host":"smtp.example.org","password_set":true,"from":"stash@con.example.org"}}}`
		noPassword   = `{"data":{"smtp":{"host":"smtp.example.org","from":"stash@con.example.org"}}}`
		noSMTP       = `{"data":{"stash":{"lost_found":{"mode":"kept_after_con","days":7}}}}`
	)
	var calls int
	fetch := func(context.Context) (string, error) {
		calls++
		return "from-den", nil
	}
	svc := &SettingsService{den: fakeDen{paired: true, module: withPassword}}
	got, err := svc.SMTPPassword(ctx, fetch, "env-secret")
	if err != nil || got != "from-den" || calls != 1 {
		t.Fatalf("den password: %q %v calls=%d", got, err, calls)
	}

	calls = 0
	svc.den = fakeDen{paired: true, module: noPassword}
	got, err = svc.SMTPPassword(ctx, fetch, "env-secret")
	if err != nil || got != "" || calls != 0 {
		t.Fatalf("den server without a password must not use SMTP_PASSWORD: %q %v calls=%d", got, err, calls)
	}

	svc.den = fakeDen{paired: true, module: noSMTP}
	got, err = svc.SMTPPassword(ctx, fetch, "env-secret")
	if err != nil || got != "env-secret" || calls != 0 {
		t.Fatalf("no den smtp: %q %v calls=%d", got, err, calls)
	}

	svc.den = fakeDen{paired: false, module: withPassword}
	got, err = svc.SMTPPassword(ctx, fetch, "env-secret")
	if err != nil || got != "env-secret" || calls != 0 {
		t.Fatalf("standalone keeps SMTP_PASSWORD: %q %v calls=%d", got, err, calls)
	}

	svc.den = fakeDen{paired: true, module: withPassword}
	_, err = svc.SMTPPassword(ctx, nil, "env-secret")
	if err == nil {
		t.Fatal("missing fetch while Den holds a password")
	}
	_, err = svc.SMTPPassword(ctx, func(context.Context) (string, error) {
		return "", denlink.ErrRevoked
	}, "env-secret")
	if !errors.Is(err, denlink.ErrRevoked) {
		t.Fatalf("revoked: %v", err)
	}
	if denlink.UserMessage(err) == err.Error() || denlink.UserMessage(err) == "" {
		t.Fatalf("admin sentence %q, go error %q", denlink.UserMessage(err), err.Error())
	}
}

func TestSettingsFollowDenModuleSettingsWhilePaired(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Admin")
	svc := NewSettingsService(pool).WithDen(fakeDen{paired: true, module: `{
		"updated_at": "2026-11-11T09:30:00Z",
		"data": {
			"retention": {"inactive_users_days": 730, "module_data_days": 1095},
			"smtp": {"host": "smtp.example.org", "username": "conservices", "password_set": true,
			         "from": "stash@con.example.org", "from_name": "Fur Con Stash"},
			"stash": {"lost_found": {"mode": "kept_after_con", "days": 7}},
			"finance": {"seller": {"legal_name": "not for Stash"}}
		}
	}`})

	s, err := svc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.ManagedBy != "den" || s.OrganizationName != "Den Org" {
		t.Fatalf("not managed by Den: %+v", s)
	}
	if s.LostFoundRetentionMode != RetentionAfterCon || s.LostFoundRetentionDays != 7 || s.UserInactiveDays != 730 {
		t.Fatalf("Den retention not applied: %+v", s)
	}
	if s.SMTPHost != "smtp.example.org" || s.SMTPPort != 587 || s.SMTPTLS != SMTPTLSStartTLS ||
		s.SMTPUsername != "conservices" || s.SMTPFrom != `"Fur Con Stash" <stash@con.example.org>` {
		t.Fatalf("Den SMTP not applied: %+v", s)
	}
	if s.LostFoundRetentionHours != 720 || slices.Contains(s.ManagedKeys, "lostfound_retention_hours") || slices.Contains(s.ManagedKeys, "con_end_date") {
		t.Fatalf("keys Den does not set must stay Stash's: %+v", s)
	}

	_, err = svc.PatchJSON(ctx, []byte(`{"lostfound_retention_days": 10}`))
	wantCode(t, err, httperror.CodeValidation)
	if _, err := svc.PatchJSON(ctx, []byte(`{"lostfound_retention_hours": 48, "con_end_date": "2026-08-16"}`)); err != nil {
		t.Fatalf("keys Den does not set stay editable: %v", err)
	}
}

func TestDenLostFoundModesAndBadValues(t *testing.T) {
	pool := testdb.New(t)
	ctx := testdb.Actor(t, pool, "Admin")
	cases := []struct {
		name, module string
		mode         string
		days, hours  int
		managed      []string
	}{
		{"closing with hours", `{"data":{"stash":{"lost_found":{"mode":"deleted_after_closing","hours":24}}}}`,
			RetentionAfterClose, 30, 24, []string{"lostfound_retention_hours", "lostfound_retention_mode"}},
		{"closing without hours uses the contract default", `{"data":{"stash":{"lost_found":{"mode":"deleted_after_closing"}}}}`,
			RetentionAfterClose, 30, 48, []string{"lostfound_retention_hours", "lostfound_retention_mode"}},
		{"unknown mode is ignored", `{"data":{"stash":{"lost_found":{"mode":"forever","days":3}}}}`,
			RetentionAfterClose, 30, 720, nil},
		{"zero days falls back to Stash", `{"data":{"stash":{"lost_found":{"mode":"kept_after_con","days":0}}}}`,
			RetentionAfterCon, 30, 720, []string{"lostfound_retention_mode"}},
		{"older Den without data wrapper", `{"stash":{"lost_found":{"mode":"kept_after_con","days":5}}}`,
			RetentionAfterCon, 5, 720, []string{"lostfound_retention_days", "lostfound_retention_mode"}},
		{"no module settings", ``, RetentionAfterClose, 30, 720, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := NewSettingsService(pool).WithDen(fakeDen{paired: true, module: tc.module}).Get(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if s.LostFoundRetentionMode != tc.mode || s.LostFoundRetentionDays != tc.days || s.LostFoundRetentionHours != tc.hours {
				t.Fatalf("got mode %q days %d hours %d", s.LostFoundRetentionMode, s.LostFoundRetentionDays, s.LostFoundRetentionHours)
			}
			want := append([]string{"organization_name"}, tc.managed...)
			slices.Sort(want)
			if !slices.Equal(s.ManagedKeys, want) {
				t.Fatalf("managed keys %v, want %v", s.ManagedKeys, want)
			}
		})
	}
}
