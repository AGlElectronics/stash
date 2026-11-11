package http

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

func TestServeFileHeaders(t *testing.T) {
	cases := []struct {
		contentType, wantType, wantDisposition string
	}{
		{"image/png", "image/png", `inline; filename="a\b.png"`},
		{"image/svg+xml", "application/octet-stream", `attachment; filename="a\b.png"`},
		{"text/html", "application/octet-stream", `attachment; filename="a\b.png"`},
		{"application/pdf", "application/octet-stream", `attachment; filename="a\b.png"`},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		row := sqlcgen.File{ContentType: c.contentType, OriginalFilename: "a\"\r\n\\b.png", SizeBytes: 3}
		serveFile(rec, row, strings.NewReader("abc"), "private, max-age=300")
		h := rec.Header()
		if h.Get("Content-Type") != c.wantType {
			t.Errorf("%s: Content-Type = %q, want %q", c.contentType, h.Get("Content-Type"), c.wantType)
		}
		if got := h.Get("Content-Disposition"); got != strings.ReplaceAll(c.wantDisposition, `\`, "") {
			t.Errorf("%s: Content-Disposition = %q", c.contentType, got)
		}
		if h.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing nosniff", c.contentType)
		}
		if !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
			t.Errorf("%s: missing sandbox CSP", c.contentType)
		}
		if rec.Body.String() != "abc" {
			t.Errorf("%s: body = %q", c.contentType, rec.Body.String())
		}
	}
}
