package service

import (
	"context"
	"errors"
	"testing"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

// LookupByCode short-circuits before touching the database when the trimmed
// input is empty. We verify that with a nil pool — if the validation path
// did not return early it would panic on the nil-pool QueryRow.
func TestLookupByCode_ValidatesInput(t *testing.T) {
	t.Parallel()
	s := &ItemService{pool: nil}

	cases := []string{"", "   ", "\r\n", "\n   \r"}
	for _, in := range cases {
		_, err := s.LookupByCode(context.Background(), in)
		if err == nil {
			t.Fatalf("expected error for input %q", in)
		}
		var aErr *httperror.Error
		if !errors.As(err, &aErr) {
			t.Fatalf("expected *httperror.Error for %q, got %T: %v", in, err, err)
		}
		if aErr.Code != httperror.CodeValidation {
			t.Errorf("expected validation error for %q, got code=%s", in, aErr.Code)
		}
	}
}
