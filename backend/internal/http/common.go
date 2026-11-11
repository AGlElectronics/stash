package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/AGlElectronics/stash/backend/internal/authbase"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

// decodeJSON strict-decodes one JSON value from the request body into dst
// with the shared decoder (1 MiB limit, unknown fields refused) and returns
// a typed error handlers can hand to httperror.Render: 400 bad_request or
// 413 request_too_large. Uploads and imports are multipart and have their
// own limits.
func decodeJSON(r *http.Request, dst any) error {
	return decodeJSONLimit(r, dst, authbase.DefaultMaxBodyBytes)
}

// decodeJSONLimit is decodeJSON with a smaller limit.
func decodeJSONLimit(r *http.Request, dst any, maxBytes int64) error {
	err := authbase.DecodeJSON(nil, r, dst, authbase.DecodeOptions{MaxBytes: maxBytes, DisallowUnknownFields: true})
	var de *authbase.DecodeError
	if errors.As(err, &de) {
		return httperror.New(httperror.Code(de.Code), de.Message).WithCause(err)
	}
	return err
}

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	raw := chi.URLParam(r, name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, httperror.BadRequest("invalid " + name + " id")
	}
	return id, nil
}

func queryUUID(r *http.Request, key string) (*uuid.UUID, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, httperror.BadRequest("invalid " + key)
	}
	return &id, nil
}

func queryInt(r *http.Request, key string, def int32) int32 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return def
	}
	return int32(v)
}

func queryBool(r *http.Request, key string) bool {
	v := r.URL.Query().Get(key)
	return v == "1" || v == "true" || v == "yes"
}

func queryString(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

func queryTime(r *http.Request, key string) (*time.Time, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, httperror.BadRequest("invalid " + key + " (expected RFC3339)")
	}
	return &t, nil
}

// numericString returns the string representation of a pgtype.Numeric, or "" if null.
func numericString(n pgtype.Numeric) string {
	if !n.Valid {
		return ""
	}
	v, err := n.Value()
	if err != nil || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// dateString returns YYYY-MM-DD or "" if null.
func dateString(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

// timestampString returns RFC3339 or "" if null.
func timestampString(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}
