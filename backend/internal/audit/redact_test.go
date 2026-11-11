package audit

import (
	"encoding/json"
	"testing"
)

func TestRedactGuestData(t *testing.T) {
	in := []byte(`{"item_name":"Blue backpack","contact_name":"Alex","contact_info":"+43 660 123","returned_to":"","telegram_chat_id":12345,"status":"open"}`)
	var got map[string]any
	if err := json.Unmarshal(RedactGuestData(in), &got); err != nil {
		t.Fatal(err)
	}
	if got["contact_name"] != Redacted || got["contact_info"] != Redacted || got["telegram_chat_id"] != Redacted {
		t.Errorf("personal fields not redacted: %v", got)
	}
	if got["returned_to"] != "" {
		t.Errorf("empty fields should stay empty, got %v", got["returned_to"])
	}
	if got["item_name"] != "Blue backpack" || got["status"] != "open" {
		t.Errorf("non-personal fields changed: %v", got)
	}
}

func TestRedactFileOfReport(t *testing.T) {
	in := []byte(`{"entity_type":"lostfound","original_filename":"alex-passport.jpg","content_type":"image/jpeg"}`)
	var got map[string]any
	_ = json.Unmarshal(RedactGuestData(in), &got)
	if got["original_filename"] != Redacted {
		t.Errorf("filename of a report photo must be redacted: %v", got)
	}
	item := []byte(`{"entity_type":"item","original_filename":"receipt.pdf"}`)
	_ = json.Unmarshal(RedactGuestData(item), &got)
	if got["original_filename"] != "receipt.pdf" {
		t.Errorf("item filenames are not guest data: %v", got)
	}
}

func TestHasGuestData(t *testing.T) {
	if !HasGuestData("lostfound") {
		t.Error("lostfound rows carry guest data")
	}
	if !HasGuestData("file", nil, []byte(`{"entity_type":"lostfound"}`)) {
		t.Error("files of reports carry guest data")
	}
	if HasGuestData("file", []byte(`{"entity_type":"item"}`)) || HasGuestData("item") {
		t.Error("items carry no guest data")
	}
}

func TestRedactLeavesNonObjectsAlone(t *testing.T) {
	for _, in := range [][]byte{nil, []byte(`null`), []byte(`[1,2]`), []byte(`not json`)} {
		if string(RedactGuestData(in)) != string(in) {
			t.Errorf("RedactGuestData(%q) changed the input", in)
		}
	}
}
