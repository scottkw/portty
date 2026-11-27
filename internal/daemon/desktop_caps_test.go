package daemon

import (
	"os"
	"regexp"
	"testing"
)

func TestDesktopCapStore_PutValidation(t *testing.T) {
	s := NewDesktopCapStore()
	for _, tc := range []struct{ name, host, base, tok string }{
		{"empty host", "", "https://h.example:7444", "t"},
		{"empty token", "h.example", "https://h.example:7444", ""},
		{"http base", "h.example", "http://h.example:7444", "t"},
		{"no host in base", "h.example", "https://", "t"},
		{"empty base", "h.example", "", "t"},
	} {
		if err := s.Put(tc.host, tc.base, "", tc.tok); err == nil {
			t.Errorf("%s: Put succeeded; want an error", tc.name)
		}
	}
	if _, _, _, ok := s.Get("h.example"); ok {
		t.Fatal("a rejected Put left an entry")
	}
}

func TestDesktopCapStore_KeysAndDelete(t *testing.T) {
	s := NewDesktopCapStore()
	if err := s.Put("Office-Air.Example.", "https://office-air.example:7444", "100.64.0.9", "tok"); err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"office-air.example", "OFFICE-AIR.EXAMPLE", "office-air.example."} {
		base, ip, tok, ok := s.Get(host)
		if !ok || base != "https://office-air.example:7444" || ip != "100.64.0.9" || tok != "tok" {
			t.Fatalf("Get(%q) = %q %q %q %v", host, base, ip, tok, ok)
		}
	}
	s.Delete("OFFICE-AIR.example")
	if _, _, _, ok := s.Get("office-air.example"); ok {
		t.Fatal("Get after Delete hit")
	}
	s.Delete("never-held") // no-op
}

// TestDesktopCapStore_NoDiskWriteAPIs is the source-grep guard of the
// no-disk-persistence invariant (the same list as remote_caps_test.go).
func TestDesktopCapStore_NoDiskWriteAPIs(t *testing.T) {
	src, err := os.ReadFile("desktop_caps.go")
	if err != nil {
		t.Fatal(err)
	}
	stripped := regexp.MustCompile(`(?m)^\s*//.*$`).ReplaceAll(src, nil)
	if loc := regexp.MustCompile(`os\.WriteFile|ioutil\.WriteFile|os\.Create`).FindIndex(stripped); loc != nil {
		t.Fatalf("desktop_caps.go must not reference disk-write APIs; found %q", stripped[loc[0]:loc[1]])
	}
}
