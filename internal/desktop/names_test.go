package desktop

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNames_Sanitize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bidi override", "Pat\u202eColleague", "PatColleague"},
		{"bidi isolates", "\u2066Pat\u2069", "Pat"},
		{"zero width", "Pa\u200bt\ufeff", "Pat"},
		{"nul", "Pat\x00Colleague", "PatColleague"},
		{"newline and tab become one space", "Pat\nCol\tleague", "Pat Col league"},
		{"whitespace runs collapse and trim", "  Pat \t Colleague  ", "Pat Colleague"},
		{"invalid utf8", "Pat\xffX", "Pat�X"},
		{"all control", "\u202e\u200b\x00\n", ""},
		{"plain", "Pat Colleague", "Pat Colleague"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SanitizeName(c.in); got != c.want {
				t.Fatalf("SanitizeName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNames_CapsAtRuneLimit(t *testing.T) {
	got := SanitizeName(strings.Repeat("é", 200))
	if n := utf8.RuneCountInString(got); n != maxNameRunes {
		t.Fatalf("rune count = %d, want %d", n, maxNameRunes)
	}
	if !utf8.ValidString(got) {
		t.Fatal("capped name is not valid UTF-8")
	}
}

func TestNames_NewIdentitySanitizesNames(t *testing.T) {
	id := NewIdentity(7, "a\u202e@example.com", " Pat\n", "lap\u200btop", "nSTABLE", "192.0.2.9")
	if id.LoginName != "a@example.com" || id.DisplayName != "Pat" || id.NodeName != "laptop" {
		t.Fatalf("names not sanitized: %+v", id)
	}
	if id.UserID != 7 || id.StableID != "nSTABLE" || id.TailnetIP != "192.0.2.9" {
		t.Fatalf("non-name fields changed: %+v", id)
	}
}
