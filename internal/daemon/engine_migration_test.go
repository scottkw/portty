package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixtureV31Path returns the on-disk location of the realistic v3.1 settings
// fixture. The Go test working directory is the package directory
// (internal/daemon/), so the fixture lives two levels up.
func fixtureV31Path(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "tests", "fixtures", "settings_v3.1.json")
}

// copyFixtureToTempDir reads the v3.1 fixture and writes it as settings.json
// inside dir. Returns the destination path for convenience.
func copyFixtureToTempDir(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(fixtureV31Path(t))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dst := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(dst, data, 0600); err != nil {
		t.Fatalf("write fixture into temp: %v", err)
	}
	return dst
}

// TestSettingsMigrationV3_1ToV3_2 is the load-bearing CI gate per ROADMAP SC-1.
// A returning v3.1 user (no plugins block, no schemaVersion key) must:
//  1. Load with the 8 v3.2 plugin defaults (7 ON, Progress OFF) populated.
//  2. Trigger a re-write so the on-disk settings.json now contains both the
//     plugins block and schemaVersion: 2.
//  3. Drop the removed cliPaths key on that re-write (Portty 01-11: the
//     CLI path override plumbing is gone).
func TestSettingsMigrationV3_1ToV3_2(t *testing.T) {
	dir := t.TempDir()
	copyFixtureToTempDir(t, dir)

	e := &SessionEngine{
		configDir: dir,
	}
	e.loadSettingsFromDisk(dir)

	// In-memory: the 8 v3.2 defaults must be populated (Pitfall #14 mitigation).
	got := e.GetPluginSettings()
	want := defaultPluginSettings()
	if got != want {
		t.Errorf("GetPluginSettings after v3.1 load: got %+v, want %+v", got, want)
	}

	// Phase 99 SC-3 — per-field assertions for diagnostic clarity when a v3.3 default
	// change breaks the migration contract. The struct-equality check above is the
	// "fast fail" sentinel; these per-field assertions name the failing field.
	if !got.WebGL {
		t.Errorf("plugin defaults: WebGL = false, want true")
	}
	if !got.Unicode11 {
		t.Errorf("plugin defaults: Unicode11 = false, want true")
	}
	if !got.Search {
		t.Errorf("plugin defaults: Search = false, want true")
	}
	if !got.WebLinks {
		t.Errorf("plugin defaults: WebLinks = false, want true")
	}
	if !got.Image {
		t.Errorf("plugin defaults: Image = false, want true")
	}
	if !got.Serialize {
		t.Errorf("plugin defaults: Serialize = false, want true")
	}
	if !got.Clipboard {
		t.Errorf("plugin defaults: Clipboard = false, want true")
	}
	if got.Progress {
		t.Errorf("plugin defaults: Progress = true, want false (default OFF in v3.2 — flips ON in v3.3 after field validation)")
	}

	// SearchConfig defaults (replaces the previous zero-value check — forward-compatible
	// for v3.3 when a non-zero default may be introduced).
	wantSearch := SearchConfig{Regex: false, CaseSensitive: false, WholeWord: false}
	if got.SearchConfig != wantSearch {
		t.Errorf("SearchConfig defaults: got %+v, want %+v", got.SearchConfig, wantSearch)
	}

	// WebLinksConfig defaults.
	wantWebLinks := WebLinksConfig{
		Modifier:         "platform",
		ConfirmOSC8:      true,
		ConfirmIDN:       true,
		ConfirmTyposquat: true,
	}
	if got.WebLinksConfig != wantWebLinks {
		t.Errorf("WebLinksConfig defaults: got %+v, want %+v", got.WebLinksConfig, wantWebLinks)
	}

	// ImageConfig defaults — StorageLimit override (16 MB; upstream default is 100 MB).
	wantImage := ImageConfig{StorageLimit: 16}
	if got.ImageConfig != wantImage {
		t.Errorf("ImageConfig defaults: got %+v, want %+v (v3.2 overrides upstream 100 MB to 16 MB to prevent tab OOM)", got.ImageConfig, wantImage)
	}

	// On-disk: the upgrade re-write fired, so the file now contains both the
	// plugins block AND schemaVersion: 2.
	raw, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("re-read settings.json: %v", err)
	}
	var s daemonSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unmarshal rewritten settings: %v", err)
	}
	if s.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("on-disk schemaVersion = %d, want %d", s.SchemaVersion, CurrentSchemaVersion)
	}
	if !s.Plugins.WebGL {
		t.Errorf("on-disk plugins.webgl = false, want true (proves the upgrade re-write fired)")
	}
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("unmarshal rewritten settings as map: %v", err)
	}
	if _, ok := onDisk["cliPaths"]; ok {
		t.Error("rewritten settings.json still contains cliPaths; the removed key should be dropped")
	}
}

// TestSettingsMigrationIdempotent asserts that loading a settings.json that is
// already at CurrentSchemaVersion does NOT trigger an unnecessary re-write.
// We verify this by comparing the file's mtime across two consecutive loads:
// the first load performs the v3.1→v3.2 upgrade write; the second load is a
// no-op and must leave the mtime unchanged.
func TestSettingsMigrationIdempotent(t *testing.T) {
	dir := t.TempDir()
	copyFixtureToTempDir(t, dir)

	e := &SessionEngine{
		configDir: dir,
	}
	e.loadSettingsFromDisk(dir) // first load: upgrade re-write fires

	settingsFile := filepath.Join(dir, "settings.json")
	info1, err := os.Stat(settingsFile)
	if err != nil {
		t.Fatalf("stat after first load: %v", err)
	}
	mtime1 := info1.ModTime()

	// Sleep so a stray re-write would produce a detectably different mtime.
	time.Sleep(10 * time.Millisecond)

	e2 := &SessionEngine{
		configDir: dir,
	}
	e2.loadSettingsFromDisk(dir) // second load: should be a no-op (idempotent)

	info2, err := os.Stat(settingsFile)
	if err != nil {
		t.Fatalf("stat after second load: %v", err)
	}
	mtime2 := info2.ModTime()

	if !mtime1.Equal(mtime2) {
		t.Errorf("settings.json mtime changed across idempotent loads: %v -> %v (the second load triggered an unnecessary re-write)", mtime1, mtime2)
	}
}

// fixtureV32Path returns the on-disk location of the v3.2 settings fixture.
// Mirrors fixtureV31Path: the Go test working directory is the package
// directory (internal/daemon/), so the fixture lives two levels up.
// Phase 118 / FS-14: v3.2 fixture has plugins block + schemaVersion=2 but
// NO filesRead key — exercises the defaults-merge mitigation for Pitfall 16.
func fixtureV32Path(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "tests", "fixtures", "settings_v3.2.json")
}

// copyV32FixtureToTempDir reads the v3.2 fixture and writes it as settings.json
// inside dir.
func copyV32FixtureToTempDir(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(fixtureV32Path(t))
	if err != nil {
		t.Fatalf("read v3.2 fixture: %v", err)
	}
	dst := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(dst, data, 0600); err != nil {
		t.Fatalf("write v3.2 fixture into temp: %v", err)
	}
	return dst
}

// TestSettingsMigration_FilesReadSchemaVersionRewrite verifies that loading a
// v3.2 settings.json (schemaVersion=2) triggers the needsUpgradeWrite path so
// the on-disk file is rewritten with schemaVersion=3 (CurrentSchemaVersion).
// Mirrors the v3.1→v3.2 migration test pattern.
func TestSettingsMigration_FilesReadSchemaVersionRewrite(t *testing.T) {
	dir := t.TempDir()
	copyV32FixtureToTempDir(t, dir)

	e := &SessionEngine{
		configDir: dir,
	}
	e.loadSettingsFromDisk(dir)

	raw, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("re-read settings.json: %v", err)
	}
	var s daemonSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unmarshal rewritten settings: %v", err)
	}
	if s.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("on-disk schemaVersion = %d, want %d (v3.2→v3.3 upgrade re-write should fire)", s.SchemaVersion, CurrentSchemaVersion)
	}
}

// TestSettingsMigration_LegacyAgentHubV42 loads a real-shaped AgentHub v4.2
// settings.json that still carries the removed "filesWrite" and
// "autoCloseSession", "cliPaths", "shellPath", "notifyOnWaiting" and
// "stayOnHubAfterCreate" keys.
// Old settings must load without error, the unknown keys are ignored, the
// kept values survive, and the next save drops every removed key.
func TestSettingsMigration_LegacyAgentHubV42(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "settings_agenthub_v4.2.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var probe daemonSettings
	if err := json.Unmarshal(data, &probe); err != nil {
		t.Fatalf("legacy fixture must unmarshal into daemonSettings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), data, 0600); err != nil {
		t.Fatalf("write fixture into temp: %v", err)
	}

	e := &SessionEngine{
		configDir: dir,
	}
	e.loadSettingsFromDisk(dir)

	if !e.GetShellWebShareWarned() {
		t.Error("shellWebShareWarned = false, want true (kept value must survive)")
	}

	e.mu.Lock()
	e.saveSettingsToDisk()
	e.mu.Unlock()
	raw, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("re-read settings.json: %v", err)
	}
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("unmarshal saved settings: %v", err)
	}
	if _, ok := onDisk["filesWrite"]; ok {
		t.Error("saved settings.json still contains filesWrite; the removed key should be dropped on save")
	}
	// Portty 01-10 (CONTEXT Amendment 2): the auto-close toggle is gone; the
	// legacy key loads without error and is dropped on save.
	if _, ok := onDisk["autoCloseSession"]; ok {
		t.Error("saved settings.json still contains autoCloseSession; the removed key should be dropped on save")
	}
	// Portty 01-11 (CONTEXT Amendments 1 and 2): the shell and CLI path
	// overrides and the awaiting-input toggle are gone; the legacy keys load
	// without error and are dropped on save.
	// Portty 02-06 (NAV-03): the Hub is gone, so the stay-on-Hub preference
	// is too; the legacy key loads without error and is dropped on save.
	for _, k := range []string{"cliPaths", "shellPath", "notifyOnWaiting", "stayOnHubAfterCreate"} {
		if _, ok := onDisk[k]; ok {
			t.Errorf("saved settings.json still contains %s; the removed key should be dropped on save", k)
		}
	}
	if onDisk["shellWebShareWarned"] != true {
		t.Errorf("kept key lost on save: shellWebShareWarned=%v", onDisk["shellWebShareWarned"])
	}
	if _, ok := onDisk["plugins"].(map[string]any); !ok {
		t.Errorf("saved settings.json lost the plugins block: %v", onDisk["plugins"])
	}
}
