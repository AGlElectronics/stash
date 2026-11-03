package http

import (
	"testing"

	"github.com/google/uuid"
)

func TestItoa32(t *testing.T) {
	if got := itoa32(0); got != "0" {
		t.Errorf("itoa32(0)=%q", got)
	}
	if got := itoa32(-42); got != "-42" {
		t.Errorf("itoa32(-42)=%q", got)
	}
	if got := itoa32(2147483647); got != "2147483647" {
		t.Errorf("itoa32(max int32)=%q", got)
	}
}

func TestStrOrEmpty(t *testing.T) {
	if strOrEmpty(nil) != "" {
		t.Errorf("strOrEmpty(nil) should be empty")
	}
	s := "x"
	if strOrEmpty(&s) != "x" {
		t.Errorf("strOrEmpty(ptr)=%q", strOrEmpty(&s))
	}
}

func TestUUIDStr(t *testing.T) {
	if uuidStr(nil) != "" {
		t.Errorf("uuidStr(nil) should be empty")
	}
	id := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	if got := uuidStr(&id); got != id.String() {
		t.Errorf("uuidStr=%q want %q", got, id.String())
	}
}
