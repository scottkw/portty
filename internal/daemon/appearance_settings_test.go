package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Portty 02-13 (THEM-01 amended): Settings ▸ Appearance override persisted
// in settings.json as "appearance": system|light|dark.

func TestAppearanceSettings_DefaultSystem(t *testing.T) {
	dir := t.TempDir()
	e := &SessionEngine{configDir: dir}
	// Fresh install: no settings.json, so loadSettingsFromDisk returns early.
	e.loadSettingsFromDisk(dir)
	if got := e.GetAppearance(); got != "system" {
		t.Errorf("fresh engine (no settings.json): want system, got %q", got)
	}
}

func TestAppearanceSettings_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	e := &SessionEngine{configDir: dir}
	if err := e.SetAppearance("dark"); err != nil {
		t.Fatalf("SetAppearance(dark): %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not written: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw["appearance"] != "dark" {
		t.Errorf(`settings.json: want "appearance": "dark", got %v`, raw["appearance"])
	}

	e2 := &SessionEngine{configDir: dir}
	e2.loadSettingsFromDisk(dir)
	if got := e2.GetAppearance(); got != "dark" {
		t.Errorf("reloaded engine: want dark, got %q", got)
	}
}

func TestAppearanceSettings_InvalidOnDiskFallsBack(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"),
		[]byte(`{"appearance":"purple","schemaVersion":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	e := &SessionEngine{configDir: dir}
	e.loadSettingsFromDisk(dir)
	if got := e.GetAppearance(); got != "system" {
		t.Errorf(`on-disk "purple": want system, got %q`, got)
	}
}

func TestAppearanceSettings_SetRejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	e := &SessionEngine{configDir: dir}
	if err := e.SetAppearance("light"); err != nil {
		t.Fatalf("SetAppearance(light): %v", err)
	}
	for _, bad := range []string{"purple", "", "Dark"} {
		if err := e.SetAppearance(bad); err == nil {
			t.Errorf("SetAppearance(%q): want error, got nil", bad)
		}
	}
	if got := e.GetAppearance(); got != "light" {
		t.Errorf("after rejected sets: want light, got %q", got)
	}
}

func getAppearanceViaAPI(t *testing.T, socketPath string) string {
	t.Helper()
	status, body := rawGet(t, socketPath, "/settings/appearance")
	if status != 200 {
		t.Fatalf("GET /settings/appearance: want 200, got %d; body: %s", status, body)
	}
	var resp map[string]string
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode appearance response: %v", err)
	}
	return resp["appearance"]
}

func TestAPIAppearance_GetPatch(t *testing.T) {
	_, client, socketPath := testDaemon(t)

	if got := getAppearanceViaAPI(t, socketPath); got != "system" {
		t.Errorf("default GET: want system, got %q", got)
	}
	status, body := rawPatch(t, socketPath, "/settings/appearance", `{"appearance":"light"}`)
	if status != 204 {
		t.Fatalf("PATCH light: want 204, got %d; body: %s", status, body)
	}
	if got := getAppearanceViaAPI(t, socketPath); got != "light" {
		t.Errorf("GET after PATCH light: want light, got %q", got)
	}

	// Client round trip.
	if err := client.SetAppearance("dark"); err != nil {
		t.Fatalf("client.SetAppearance(dark): %v", err)
	}
	if got, err := client.GetAppearance(); err != nil || got != "dark" {
		t.Errorf("client.GetAppearance: want dark, got %q (err %v)", got, err)
	}
}

func TestAPIAppearance_PatchInvalid400(t *testing.T) {
	_, client, socketPath := testDaemon(t)
	if err := client.SetAppearance("dark"); err != nil {
		t.Fatalf("client.SetAppearance(dark): %v", err)
	}
	for _, body := range []string{`{"appearance":"purple"}`, `not-json`, `{}`} {
		status, _ := rawPatch(t, socketPath, "/settings/appearance", body)
		if status != 400 {
			t.Errorf("PATCH %s: want 400, got %d", body, status)
		}
	}
	if err := client.SetAppearance("purple"); err == nil {
		t.Error("client.SetAppearance(purple): want error, got nil")
	}
	if got := getAppearanceViaAPI(t, socketPath); got != "dark" {
		t.Errorf("after rejected PATCHes: want dark, got %q", got)
	}
}
