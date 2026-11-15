package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	e := &SessionEngine{
		configDir: dir,
	}
	// Should not panic or error — missing file is first-run
	e.loadSettingsFromDisk(dir)
	if e.GetStartMinimized() {
		t.Error("expected default startMinimized=false on first run")
	}
}

func TestStartMinimizedPersistence(t *testing.T) {
	dir := t.TempDir()
	e := &SessionEngine{
		configDir: dir,
	}

	// Default is false
	if e.GetStartMinimized() {
		t.Error("expected default startMinimized=false")
	}

	// Set to true and verify settings.json
	e.SetStartMinimized(true)

	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not found: %v", err)
	}
	var s daemonSettings
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !s.StartMinimized {
		t.Error("expected startMinimized=true in settings.json")
	}

	// Create new engine, load from disk, verify
	e2 := &SessionEngine{
		configDir: dir,
	}
	e2.loadSettingsFromDisk(dir)
	if !e2.GetStartMinimized() {
		t.Error("expected loaded startMinimized=true")
	}

	// Set back to false, reload, verify
	e2.SetStartMinimized(false)
	e3 := &SessionEngine{
		configDir: dir,
	}
	e3.loadSettingsFromDisk(dir)
	if e3.GetStartMinimized() {
		t.Error("expected loaded startMinimized=false after set(false)")
	}
}

func TestStartMinimizedWithoutCLIPaths(t *testing.T) {
	dir := t.TempDir()
	// Write settings.json with only startMinimized (no legacy cliPaths key)
	data := []byte(`{"startMinimized":true}`)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	e := &SessionEngine{
		configDir: dir,
	}
	e.loadSettingsFromDisk(dir)
	if !e.GetStartMinimized() {
		t.Error("expected startMinimized=true when loaded from a startMinimized-only file")
	}
}

func TestSettingsFilePermissions(t *testing.T) {
	dir := t.TempDir()

	e := &SessionEngine{
		configDir: dir,
	}
	e.SetStartMinimized(true)

	info, err := os.Stat(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("settings.json permissions = %o, want 0600", perm)
	}
}
