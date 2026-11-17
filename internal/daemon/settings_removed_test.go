package daemon_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/scottkw/portty/internal/daemon"
)

// Portty 01-10 (CONTEXT Amendment 2, T-01-35): tabs always close on exit, so the
// auto-close setting and its routes are gone from the daemon socket mux.
func TestSettingsRemoved_AutoCloseSessionRoutes(t *testing.T) {
	h := newRemovedRoutesAPI(t).Handler()
	assertRouteGone(t, h, http.MethodGet, "/settings/auto-close-session")
	assertRouteGone(t, h, http.MethodPatch, "/settings/auto-close-session")

	// A kept sibling settings route still answers, so the 404/405 above is the
	// removal and not a broken handler.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/plugins", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /settings/plugins = %d; want 200", rec.Code)
	}
}

// Portty 01-11 (CONTEXT Amendment 1, T-01-37): every session runs /bin/zsh -l,
// so the shell-path setting and its routes are gone.
func TestSettingsRemoved_ShellPathRoutes(t *testing.T) {
	h := newRemovedRoutesAPI(t).Handler()
	assertRouteGone(t, h, http.MethodGet, "/settings/shell-path")
	assertRouteGone(t, h, http.MethodPatch, "/settings/shell-path")
}

// Portty 01-11 (CONTEXT Amendment 1, T-01-37): the Settings → Paths CLI path
// overrides (including the tailscale binary override) are gone.
func TestSettingsRemoved_CLIPathsRoutes(t *testing.T) {
	h := newRemovedRoutesAPI(t).Handler()
	assertRouteGone(t, h, http.MethodGet, "/settings/cli-paths")
	assertRouteGone(t, h, http.MethodPatch, "/settings/cli-paths/tailscale")
}

// Portty 01-11 (CONTEXT Amendment 2, T-01-37): the awaiting-input notification
// toggle and its routes are gone.
func TestSettingsRemoved_NotifyOnWaitingRoutes(t *testing.T) {
	h := newRemovedRoutesAPI(t).Handler()
	assertRouteGone(t, h, http.MethodGet, "/settings/notify-on-waiting")
	assertRouteGone(t, h, http.MethodPatch, "/settings/notify-on-waiting")
}

// Portty 02-06 (NAV-03, T-02-19): the Hub is gone, so the "stay on Hub after
// creating a session" preference and its routes are gone too.
func TestSettingsRemoved_StayOnHubAfterCreateRoutes(t *testing.T) {
	h := newRemovedRoutesAPI(t).Handler()
	assertRouteGone(t, h, http.MethodGet, "/settings/stay-on-hub-after-create")
	assertRouteGone(t, h, http.MethodPatch, "/settings/stay-on-hub-after-create")

	// The sibling start-minimized route still answers, so the 404/405 above is
	// the removal and not a broken handler.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/start-minimized", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /settings/start-minimized = %d; want 200", rec.Code)
	}
}

// Portty 04-01 (UI-SPEC: the one-time share reminder is removed): a settings.json
// that still carries the two legacy share-reminder keys loads without error and
// the next save writes neither key.
func TestSettingsRemoved_ShareReminderKeysDropped(t *testing.T) {
	home := t.TempDir() // os.UserConfigDir is $HOME/Library/Application Support on darwin
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	cfg, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	dir := filepath.Join(cfg, "portty")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := []string{"shellWebShareWarned", "shellWebShareWarningEnabled"}
	in, _ := json.Marshal(map[string]any{
		legacy[0]: true, legacy[1]: false, "startMinimized": true,
		"schemaVersion": daemon.CurrentSchemaVersion,
	})
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, in, 0o600); err != nil {
		t.Fatal(err)
	}

	e := daemon.NewSessionEngine()
	if !e.GetStartMinimized() {
		t.Fatal("kept key startMinimized did not load from a file carrying the legacy keys")
	}
	e.SetStartMinimized(false) // forces a save

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk map[string]any
	if err := json.Unmarshal(out, &onDisk); err != nil {
		t.Fatalf("saved settings.json: %v", err)
	}
	for _, k := range legacy {
		if _, ok := onDisk[k]; ok {
			t.Errorf("saved settings.json still contains %s; the removed key should be dropped on save", k)
		}
	}
}
