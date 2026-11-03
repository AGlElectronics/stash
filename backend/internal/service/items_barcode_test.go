package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

func TestFormatDefaultBarcode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int64
		want string
	}{
		{1, "ST000001"},
		{42, "ST000042"},
		{999999, "ST999999"},
		// Sequence values larger than the padding still render correctly;
		// we just lose alignment, which is acceptable as a soft cap.
		{1000000, "ST1000000"},
	}
	for _, c := range cases {
		if got := FormatDefaultBarcode(c.in); got != c.want {
			t.Errorf("FormatDefaultBarcode(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHasItemScanCode(t *testing.T) {
	t.Parallel()
	tag := "ST000001"
	qr := "QR-1"
	empty := ""
	space := "  "

	cases := []struct {
		name string
		tag  *string
		qr   *string
		want bool
	}{
		{"none", nil, nil, false},
		{"empty strings", &empty, &empty, false},
		{"whitespace", &space, &space, false},
		{"asset tag only", &tag, nil, true},
		{"qr only", nil, &qr, true},
		{"both", &tag, &qr, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasItemScanCode(c.tag, c.qr); got != c.want {
				t.Fatalf("hasItemScanCode() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestSetItemBarcode_Validates(t *testing.T) {
	t.Parallel()
	s := &ItemService{pool: nil}

	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"contains space", "AW 12"},
		{"contains tab", "AW\t12"},
		{"too long", string(make([]byte, 65))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := s.SetItemBarcode(context.Background(), uuid.New(), c.in)
			if err == nil {
				t.Fatalf("expected error for %q", c.in)
			}
			var aErr *httperror.Error
			if !errors.As(err, &aErr) {
				t.Fatalf("expected *httperror.Error, got %T: %v", err, err)
			}
			if aErr.Code != httperror.CodeValidation {
				t.Errorf("expected validation code, got %s", aErr.Code)
			}
		})
	}
}
