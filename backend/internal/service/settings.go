package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

// SystemSettings is the wire/JSON representation of system-wide toggles.
//
// Adding a new setting? Add a field here, give it a sane zero value and a
// JSON tag, and patch it in the merge step of Patch(). No migration needed
// because the underlying column is JSONB.
type SystemSettings struct {
	// ForceBarcodeOnSignout requires the logistics team to scan a barcode
	// for every line on a sign-out. When false, scanning is allowed but
	// optional.
	ForceBarcodeOnSignout bool `json:"force_barcode_on_signout"`

	// OrganizationName is the optional org label used for defaults/exports.
	// UI branding prefers /brand.json or Den; this is the server-side default.
	OrganizationName string `json:"organization_name,omitempty"`

	// DefaultItemOwner applied when creating items with an empty owner.
	DefaultItemOwner string `json:"default_item_owner,omitempty"`

	// DefaultCurrency is an ISO 4217 code used as the item form default.
	DefaultCurrency string `json:"default_currency,omitempty"`

	// BarcodePrefix and BarcodeWidth control generated asset tags (e.g. ST + 6).
	BarcodePrefix string `json:"barcode_prefix,omitempty"`
	BarcodeWidth  int    `json:"barcode_width,omitempty"`

	// PreserveSacredLostFound gates the immortal LF-0001 test-item rule.
	// Default false for reusable deployments; event overlays may enable it.
	PreserveSacredLostFound bool `json:"preserve_sacred_lostfound"`

	// ManagedBy is "den" while Stash is paired with Den; ManagedKeys are
	// then read-only here. Never stored.
	ManagedBy   string   `json:"managed_by,omitempty"`
	ManagedKeys []string `json:"managed_keys,omitempty"`
}

// OrgSource supplies organisation settings from Den when Stash is paired.
type OrgSource interface {
	Paired() bool
	OrganizationName() string
}

// denManagedKeys are owned by Den while paired.
var denManagedKeys = []string{"organization_name"}

type SettingsService struct {
	pool *pgxpool.Pool
	den  OrgSource
}

func NewSettingsService(pool *pgxpool.Pool) *SettingsService {
	return &SettingsService{pool: pool}
}

// WithDen makes Get/PatchJSON follow Den for organisation-wide keys.
func (s *SettingsService) WithDen(den OrgSource) *SettingsService {
	s.den = den
	return s
}

func (s *SettingsService) Get(ctx context.Context) (SystemSettings, error) {
	out, err := s.stored(ctx)
	if err != nil {
		return out, err
	}
	if s.den != nil && s.den.Paired() {
		out.OrganizationName = s.den.OrganizationName()
		out.ManagedBy, out.ManagedKeys = "den", denManagedKeys
	}
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


// PatchJSON merges a partial JSON object into the stored settings document.
// Unknown keys are preserved; only provided keys are overwritten.
func (s *SettingsService) PatchJSON(ctx context.Context, patch json.RawMessage) (SystemSettings, error) {
	cur, err := s.stored(ctx)
	if err != nil {
		return SystemSettings{}, err
	}
	base, err := json.Marshal(cur)
	if err != nil {
		return SystemSettings{}, httperror.Internal(err)
	}
	var baseMap map[string]any
	if err := json.Unmarshal(base, &baseMap); err != nil {
		return SystemSettings{}, httperror.Internal(err)
	}
	var patchMap map[string]any
	if len(patch) > 0 {
		if err := json.Unmarshal(patch, &patchMap); err != nil {
			return SystemSettings{}, httperror.BadRequest("invalid settings patch JSON")
		}
	}
	paired := s.den != nil && s.den.Paired()
	for k, v := range patchMap {
		if k == "managed_by" || k == "managed_keys" {
			continue
		}
		if paired && slices.Contains(denManagedKeys, k) {
			return SystemSettings{}, httperror.Conflict(k + " is managed in Den (Admin → Organisation)")
		}
		baseMap[k] = v
	}
	delete(baseMap, "managed_by")
	delete(baseMap, "managed_keys")
	buf, err := json.Marshal(baseMap)
	if err != nil {
		return SystemSettings{}, httperror.Internal(err)
	}
	_, err = s.pool.Exec(ctx, `
        INSERT INTO system_settings (id, data, updated_at)
        VALUES (1, $1::jsonb, now())
        ON CONFLICT (id) DO UPDATE
            SET data = EXCLUDED.data,
                updated_at = now()
    `, string(buf))
	if err != nil {
		return SystemSettings{}, httperror.Internal(err)
	}
	return s.Get(ctx)
}

// Patch merges a full SystemSettings struct (legacy callers). Prefer PatchJSON
// for partial SPA updates so omitted fields are not zeroed.
func (s *SettingsService) Patch(ctx context.Context, in SystemSettings) (SystemSettings, error) {
	buf, err := json.Marshal(in)
	if err != nil {
		return SystemSettings{}, httperror.Internal(err)
	}
	return s.PatchJSON(ctx, buf)
}
