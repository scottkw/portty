package relay

import "testing"

func TestSanitizeChatContent(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain text unchanged", "hello @alice", "hello @alice"},
		{"C0 controls and ESC stripped", "a\x00b\x1b[2Jc\r\n", "ab[2Jc"},
		{"DEL stripped", "x\x7fy", "xy"},
		{"C1 stripped", "x\u0085y\u009bz", "xyz"},
		{"bidi overrides stripped", "safe‮evil⁦", "safeevil"},
		{"non-control unicode kept", "café ✓", "café ✓"},
	}
	for _, tc := range cases {
		if got := SanitizeChatContent(tc.in); got != tc.want {
			t.Errorf("%s: SanitizeChatContent(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}
