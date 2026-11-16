package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Quick 261116-ku9 (R1): Settings ▸ General ▸ Terminal Font persisted in
// settings.json as terminalFontFamily / terminalFontSize, validated at the
// daemon boundary because the family ends up in xterm font strings.

func TestTerminalFontSettings_Default(t *testing.T) {
	dir := t.TempDir()
	e := &SessionEngine{configDir: dir}
	e.loadSettingsFromDisk(dir)
	if got := e.GetTerminalFont(); got != (TerminalFont{Family: "", Size: 13}) {
		t.Errorf("fresh engine: want {\"\" 13}, got %+v", got)
	}
}

func TestTerminalFontSettings_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	e := &SessionEngine{configDir: dir}
	want := TerminalFont{Family: "Hack Nerd Font Mono", Size: 15}
	if err := e.SetTerminalFont(want); err != nil {
		t.Fatalf("SetTerminalFont: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not written: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw["terminalFontFamily"] != "Hack Nerd Font Mono" || raw["terminalFontSize"] != float64(15) {
		t.Errorf("settings.json keys: got family=%v size=%v", raw["terminalFontFamily"], raw["terminalFontSize"])
	}
	e2 := &SessionEngine{configDir: dir}
	e2.loadSettingsFromDisk(dir)
	if got := e2.GetTerminalFont(); got != want {
		t.Errorf("reloaded engine: want %+v, got %+v", want, got)
	}
}

func TestTerminalFontSettings_TrimsFamily(t *testing.T) {
	e := &SessionEngine{configDir: t.TempDir()}
	if err := e.SetTerminalFont(TerminalFont{Family: "  Menlo  ", Size: 13}); err != nil {
		t.Fatalf("SetTerminalFont: %v", err)
	}
	if got := e.GetTerminalFont().Family; got != "Menlo" {
		t.Errorf("want trimmed Menlo, got %q", got)
	}
}

func TestTerminalFontSettings_RejectsInvalid(t *testing.T) {
	e := &SessionEngine{configDir: t.TempDir()}
	good := TerminalFont{Family: "Menlo", Size: 14}
	if err := e.SetTerminalFont(good); err != nil {
		t.Fatalf("SetTerminalFont(good): %v", err)
	}
	bad := []TerminalFont{
		{Family: `Me"nlo`, Size: 14},
		{Family: "Me'nlo", Size: 14},
		{Family: "Menlo;", Size: 14},
		{Family: "Menlo{", Size: 14},
		{Family: "Menlo}", Size: 14},
		{Family: `Menlo\`, Size: 14},
		{Family: "Menlo\a", Size: 14},
		{Family: strings.Repeat("a", 65), Size: 14},
		{Family: "Menlo", Size: 5},
		{Family: "Menlo", Size: 33},
		{Family: "Menlo", Size: 0},
	}
	for _, f := range bad {
		if err := e.SetTerminalFont(f); err == nil {
			t.Errorf("SetTerminalFont(%+v): want error, got nil", f)
		}
	}
	if got := e.GetTerminalFont(); got != good {
		t.Errorf("after rejected sets: want %+v, got %+v", good, got)
	}
}

func TestTerminalFontSettings_InvalidOnDiskFallsBack(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"),
		[]byte(`{"terminalFontFamily":"x\";y","terminalFontSize":99,"schemaVersion":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	e := &SessionEngine{configDir: dir}
	e.loadSettingsFromDisk(dir)
	if got := e.GetTerminalFont(); got != (TerminalFont{Family: "", Size: 13}) {
		t.Errorf("invalid on-disk font: want defaults, got %+v", got)
	}
}

func getTerminalFontViaAPI(t *testing.T, socketPath string) TerminalFont {
	t.Helper()
	status, body := rawGet(t, socketPath, "/settings/terminal-font")
	if status != 200 {
		t.Fatalf("GET /settings/terminal-font: want 200, got %d; body: %s", status, body)
	}
	var f TerminalFont
	if err := json.Unmarshal(body, &f); err != nil {
		t.Fatalf("decode terminal font: %v", err)
	}
	return f
}

func TestAPITerminalFont_GetPatch(t *testing.T) {
	_, client, socketPath := testDaemon(t)
	if got := getTerminalFontViaAPI(t, socketPath); got != (TerminalFont{Size: 13}) {
		t.Errorf("default GET: want {\"\" 13}, got %+v", got)
	}
	status, body := rawPatch(t, socketPath, "/settings/terminal-font", `{"family":"Menlo","size":14}`)
	if status != 204 {
		t.Fatalf("PATCH: want 204, got %d; body: %s", status, body)
	}
	if got := getTerminalFontViaAPI(t, socketPath); got != (TerminalFont{Family: "Menlo", Size: 14}) {
		t.Errorf("GET after PATCH: got %+v", got)
	}
	if err := client.SetTerminalFont(TerminalFont{Family: "SF Mono", Size: 12}); err != nil {
		t.Fatalf("client.SetTerminalFont: %v", err)
	}
	if got, err := client.GetTerminalFont(); err != nil || got != (TerminalFont{Family: "SF Mono", Size: 12}) {
		t.Errorf("client.GetTerminalFont: got %+v (err %v)", got, err)
	}
}

func TestAPITerminalFont_PatchInvalid400(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	if status, _ := rawPatch(t, socketPath, "/settings/terminal-font", `{"family":"Menlo","size":14}`); status != 204 {
		t.Fatalf("seed PATCH: want 204, got %d", status)
	}
	for _, body := range []string{`{"family":"a;b","size":14}`, `{"family":"Menlo","size":40}`, `not-json`} {
		if status, _ := rawPatch(t, socketPath, "/settings/terminal-font", body); status != 400 {
			t.Errorf("PATCH %s: want 400, got %d", body, status)
		}
	}
	if got := getTerminalFontViaAPI(t, socketPath); got != (TerminalFont{Family: "Menlo", Size: 14}) {
		t.Errorf("after rejected PATCHes: want Menlo 14, got %+v", got)
	}
}
