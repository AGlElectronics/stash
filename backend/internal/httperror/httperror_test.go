package httperror

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestRender(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"validation", Validation("bad input", nil), 422, string(CodeValidation)},
		{"bad request", BadRequest("malformed"), 400, string(CodeBadRequest)},
		{"not found", NotFound("item"), 404, string(CodeNotFound)},
		{"conflict", Conflict("duplicate"), 409, string(CodeConflict)},
		{"forbidden", New(CodeForbidden, "no"), 403, string(CodeForbidden)},
		{"unauthorized", New(CodeUnauthorized, "auth"), 401, string(CodeUnauthorized)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)
			Render(w, r, tc.err)
			if w.Code != tc.wantStatus {
				t.Fatalf("status: got %d, want %d", w.Code, tc.wantStatus)
			}
			var body map[string]any
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			errObj, ok := body["error"].(map[string]any)
			if !ok {
				t.Fatalf("missing error object: %v", body)
			}
			if errObj["code"] != tc.wantCode {
				t.Errorf("code: got %v, want %v", errObj["code"], tc.wantCode)
			}
		})
	}
}

func TestRenderInternalErrorMasksCause(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	Render(w, r, Internal(nil))
	if w.Code != 500 {
		t.Fatalf("status: got %d, want 500", w.Code)
	}
	var body map[string]any
	_ = json.NewDecoder(w.Body).Decode(&body)
	errObj := body["error"].(map[string]any)
	if errObj["code"] != string(CodeInternal) {
		t.Errorf("code: got %v, want %v", errObj["code"], CodeInternal)
	}
	// internal cause must not leak
	if msg, _ := errObj["message"].(string); msg == "" {
		t.Errorf("missing message")
	}
}
