package service

import (
	"errors"
	"testing"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

func ptrStr(s string) *string { return &s }

func TestValidateScannedCodes(t *testing.T) {
	t.Parallel()

	itemTagged := sqlcgen.Item{
		Name:         "Power strip",
		AssetTag:     ptrStr("ST000001"),
		QrCode:       ptrStr("QR-PS"),
		SerialNumber: ptrStr("SN-XYZ"),
	}
	itemNoCodes := sqlcgen.Item{Name: "Cable spool"}

	cases := []struct {
		name    string
		item    sqlcgen.Item
		codes   []string
		forced  bool
		wantErr bool
	}{
		{"forced: matching asset_tag passes", itemTagged, []string{"ST000001"}, true, false},
		{"forced: matching qr_code passes", itemTagged, []string{"QR-PS"}, true, false},
		{"forced: matching serial passes", itemTagged, []string{"SN-XYZ"}, true, false},
		{"forced: missing scan fails", itemTagged, nil, true, true},
		{"forced: wrong code fails", itemTagged, []string{"AW999999"}, true, true},
		{"forced: item with no codes can't be scanned", itemNoCodes, []string{"anything"}, true, true},
		{"unforced: no codes is fine", itemTagged, nil, false, false},
		{"unforced: wrong code still fails", itemTagged, []string{"WRONG"}, false, true},
		{"unforced: matching code is fine", itemTagged, []string{"ST000001"}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateScannedCodes(c.item, c.codes, c.forced)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				var aErr *httperror.Error
				if !errors.As(err, &aErr) {
					t.Fatalf("expected *httperror.Error, got %T: %v", err, err)
				}
				if aErr.Code != httperror.CodeValidation {
					t.Errorf("expected validation code, got %s", aErr.Code)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestMergeScannedNotes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		notes string
		codes []string
		want  string
	}{
		{"", nil, ""},
		{"hello", nil, "hello"},
		{"", []string{"ST000001"}, "scanned: ST000001"},
		{"my note", []string{"ST000001", "QR-PS"}, "my note\nscanned: ST000001, QR-PS"},
	}
	for _, c := range cases {
		got := mergeScannedNotes(c.notes, c.codes)
		if got != c.want {
			t.Errorf("merge(%q, %v) = %q, want %q", c.notes, c.codes, got, c.want)
		}
	}
}
