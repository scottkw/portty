package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realHome and testHome are HOME before and after TestMain moved it.
var realHome, testHome string

// TestMain points HOME at a short /tmp dir before any test (HARD-06): the
// real settings.json leaked into the appearance and font tests, and
// runDaemonCore's test bound the real socket and lock. /tmp, not TMPDIR, so
// DefaultSocketPath fits the 103-char sun_path limit.
func TestMain(m *testing.M) {
	os.Exit(runInTempHome(m))
}

// runInTempHome runs the tests in a temp HOME and removes it on return. The
// defer lives here because os.Exit skips defers (IN-07); a panic in a test's
// own goroutine still kills the process first and leaves the /tmp dir.
func runInTempHome(m *testing.M) int {
	realHome = os.Getenv("HOME")
	home, err := os.MkdirTemp("/tmp", "pdh-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(home)
	testHome = home
	// XDG_CONFIG_HOME is ignored by os.UserConfigDir on darwin; set for Linux CI.
	for _, k := range []string{"HOME", "XDG_CONFIG_HOME"} {
		if err := os.Setenv(k, home); err != nil {
			panic(err)
		}
	}
	return m.Run()
}

func TestTestMain_IsolatesConfigDir(t *testing.T) {
	for _, p := range []string{daemonConfigDir(), DefaultSocketPath()} {
		if !strings.HasPrefix(p, testHome+string(filepath.Separator)) {
			t.Errorf("%s is not under the test HOME %s", p, testHome)
		}
		if realHome != "" && strings.HasPrefix(p, realHome) {
			t.Errorf("%s is under the real HOME %s", p, realHome)
		}
	}
}

// A settings file in the TestMain HOME (another test may write one) must not
// reach testDaemon's engine.
func TestTestDaemon_IgnoresSettingsFile(t *testing.T) {
	path := settingsPath(daemonConfigDir())
	if err := os.WriteFile(path, []byte(`{"appearance":"dark","terminalFontFamily":"Menlo","terminalFontSize":16}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	_, _, socketPath := testDaemon(t)

	if got := getAppearanceViaAPI(t, socketPath); got != "system" {
		t.Errorf("appearance = %q, want system", got)
	}
	if got := getTerminalFontViaAPI(t, socketPath); got != (TerminalFont{Size: 13}) {
		t.Errorf("terminal font = %+v, want {\"\" 13}", got)
	}
}
