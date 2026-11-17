package main

import "testing"

// TestParseRemoteID is a table-driven test for the parseRemoteID helper.
func TestParseRemoteID(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		hostname string
		session  string
		remote   bool
	}{
		{"plain ID", "abc123", "", "abc123", false},
		{"hostname:id", "macbook:abc123", "macbook", "abc123", true},
		{"dotted hostname:id", "host.ts.net:abc123", "host.ts.net", "abc123", true},
		{"empty string", "", "", "", false},
		{"colon only", ":", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, s, r := parseRemoteID(tc.input)
			if h != tc.hostname {
				t.Errorf("hostname: got %q, want %q", h, tc.hostname)
			}
			if s != tc.session {
				t.Errorf("session: got %q, want %q", s, tc.session)
			}
			if r != tc.remote {
				t.Errorf("remote: got %v, want %v", r, tc.remote)
			}
		})
	}
}
