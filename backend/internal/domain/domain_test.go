package domain

import "testing"

func TestItemTypeValid(t *testing.T) {
	tests := []struct {
		in   ItemType
		want bool
	}{
		{ItemTypeAsset, true},
		{ItemTypeConsumable, true},
		{ItemTypeContainer, true},
		{ItemTypeGrouped, true},
		{ItemType(""), false},
		{ItemType("not-a-type"), false},
	}
	for _, tc := range tests {
		if got := tc.in.Valid(); got != tc.want {
			t.Errorf("ItemType(%q).Valid()=%v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestLostFoundReportIsSacred(t *testing.T) {
	if !LostFoundReportIsSacred("LF-0001") {
		t.Fatal("LF-0001 should be sacred")
	}
	if LostFoundReportIsSacred("LF-0002") {
		t.Fatal("LF-0002 should not be sacred")
	}
	if !LostFoundReportIsSacred("lf-0001") {
		t.Fatal("sacred check should be case-insensitive")
	}
}

func TestFileTypeValid(t *testing.T) {
	tests := []struct {
		in   FileType
		want bool
	}{
		{FileTypeImage, true},
		{FileTypeReceipt, true},
		{FileTypeAttachment, true},
		{FileType("rogue"), false},
		{FileType(""), false},
	}
	for _, tc := range tests {
		if got := tc.in.Valid(); got != tc.want {
			t.Errorf("FileType(%q).Valid()=%v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestPermissionStrings(t *testing.T) {
	// Sanity check that constants don't drift from expected wire format
	// (these are also referenced by the seed migration).
	cases := map[string]string{
		"item.create":       PermItemCreate,
		"signout.create":    PermSignoutCreate,
		"access.manage":     PermAccessManage,
		"audit.view":        PermAuditView,
		"export.create":     PermExportCreate,
		"department.manage": PermDepartmentManage,
	}
	for want, got := range cases {
		if got != want {
			t.Errorf("permission constant mismatch: got %q, want %q", got, want)
		}
	}
}

func TestRoleConstants(t *testing.T) {
	if RoleAdmin != "admin" {
		t.Errorf("RoleAdmin=%q, want admin", RoleAdmin)
	}
	if RoleVolunteer != "volunteer" {
		t.Errorf("RoleVolunteer=%q, want volunteer", RoleVolunteer)
	}
}
