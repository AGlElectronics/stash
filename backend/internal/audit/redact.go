package audit

import "encoding/json"

// Redacted replaces guest personal data in audit before/after snapshots.
const Redacted = "[redacted]"

// guestFields are the Lost & Found report fields that can hold a guest's
// personal data.
var guestFields = []string{
	"contact_name", "contact_info", "returned_to", "resolution_note",
	"description", "telegram_chat_id",
}

// HasGuestData reports whether an audit row may carry guest personal data:
// Lost & Found reports and files attached to them.
func HasGuestData(entityType string, snapshots ...[]byte) bool {
	if entityType == "lostfound" {
		return true
	}
	if entityType != "file" {
		return false
	}
	for _, s := range snapshots {
		var f struct {
			EntityType string `json:"entity_type"`
		}
		if json.Unmarshal(s, &f) == nil && f.EntityType == "lostfound" {
			return true
		}
	}
	return false
}

// RedactGuestData returns the snapshot with every guest field that holds a
// value replaced by Redacted (and, for files, the original filename). It
// returns the input unchanged when it is not a JSON object.
func RedactGuestData(snapshot []byte) []byte {
	if len(snapshot) == 0 {
		return snapshot
	}
	var m map[string]any
	if err := json.Unmarshal(snapshot, &m); err != nil || m == nil {
		return snapshot
	}
	fields := guestFields
	if m["entity_type"] == "lostfound" {
		fields = append(append([]string{}, guestFields...), "original_filename")
	}
	for _, k := range fields {
		if v, ok := m[k]; ok && v != nil && v != "" {
			m[k] = Redacted
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return snapshot
	}
	return out
}
