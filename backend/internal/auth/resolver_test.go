package auth

import (
	"strings"
	"testing"
)

func TestParseDevUser(t *testing.T) {
	cases := []struct {
		in, name, roles string
	}{
		{"", "dev-admin", "admin"},
		{"alice", "alice", "admin"},
		{"bob:volunteer", "bob", "volunteer"},
		{"carol: stash_frontdesk , readonly", "carol", "frontdesk,readonly"},
		{"dave:", "dave", "admin"},
	}
	for _, c := range cases {
		name, roles := ParseDevUser(c.in)
		if name != c.name || strings.Join(roles, ",") != c.roles {
			t.Errorf("ParseDevUser(%q) = %q %v, want %q %s", c.in, name, roles, c.name, c.roles)
		}
	}
}

func TestDisplayNameFromClaims(t *testing.T) {
	sub := "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	tests := []struct {
		name string
		in   *OIDCClaims
		want string
	}{
		{
			name: "nickname wins",
			in:   &OIDCClaims{Subject: sub, Nickname: "Lightning", Name: "Real Name", PreferredUsername: "user@example.com"},
			want: "Lightning",
		},
		{
			name: "name when no nickname",
			in:   &OIDCClaims{Subject: sub, Name: "Real Name", PreferredUsername: "user@example.com"},
			want: "Real Name",
		},
		{
			name: "preferred_username when no nickname or name",
			in:   &OIDCClaims{Subject: sub, PreferredUsername: "nahalo"},
			want: "nahalo",
		},
		{
			name: "subject fallback",
			in:   &OIDCClaims{Subject: sub},
			want: sub,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := displayNameFromClaims(tc.in); got != tc.want {
				t.Fatalf("displayNameFromClaims() = %q, want %q", got, tc.want)
			}
		})
	}
}
