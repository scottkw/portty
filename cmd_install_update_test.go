package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// install-update is internal: the GUI's update spawns it, nobody types it, so --help never lists it.
func TestInstallUpdate_HiddenFromUsage(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	usage()
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if strings.Contains(buf.String(), installUpdateCmd) {
		t.Errorf("usage mentions %s:\n%s", installUpdateCmd, buf.String())
	}
}

func TestInstallUpdate_LaunchModeIsCLI(t *testing.T) {
	mode, args, err := launchMode([]string{installUpdateCmd, "/x/job.json"})
	if mode != modeCLI || err != nil || len(args) != 2 || args[0] != installUpdateCmd {
		t.Errorf("launchMode = %d, %v, %v; want the CLI with the arguments kept", mode, args, err)
	}
}

func TestInstallUpdate_BadJobExits1(t *testing.T) {
	dir := t.TempDir()
	malformed := filepath.Join(dir, "malformed.json")
	if err := os.WriteFile(malformed, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	loose := filepath.Join(dir, "loose.json")
	if err := os.WriteFile(loose, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(dir, "missing.json"), malformed, loose} {
		if rc := runInstallUpdate([]string{p}); rc != 1 {
			t.Errorf("%s: exit %d, want 1", filepath.Base(p), rc)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "started")); err == nil {
		t.Error("a refused job wrote the started marker")
	}
	for _, args := range [][]string{nil, {"a", "b"}} {
		if rc := runInstallUpdate(args); rc != 2 {
			t.Errorf("args %v: exit %d, want 2", args, rc)
		}
	}
}

func TestSpawnInstaller_RefusesUnderTest(t *testing.T) {
	proc, err := spawnInstaller("/usr/bin/true", filepath.Join(t.TempDir(), "job.json"))
	if err == nil || proc != nil {
		t.Errorf("spawnInstaller = %v, %v; want a refusal", proc, err)
	}
}

func TestOpenApp_RefusesUnderTest(t *testing.T) {
	if err := openApp(context.Background(), filepath.Join(t.TempDir(), "Portty.app")); err == nil {
		t.Error("openApp ran from a test binary")
	}
}

func TestBundleFromExecutable(t *testing.T) {
	home := os.Getenv("HOME") // TestMain's temporary home
	for exe, want := range map[string]string{
		"/Applications/Portty.app/Contents/MacOS/Portty":        "/Applications/Portty.app",
		home + "/Applications/Portty.app/Contents/MacOS/Portty": home + "/Applications/Portty.app",
	} {
		if got, err := bundleFromExecutable(exe); err != nil || got != want {
			t.Errorf("bundleFromExecutable(%q) = %q, %v; want %q", exe, got, err, want)
		}
	}
	for _, exe := range []string{
		"build/bin/Portty",
		"/Applications/Other.app/Contents/MacOS/Portty",
		"/Applications/Portty.app/Contents/MacOS/portty-installer",
		"/Applications/Portty.app/Contents/Resources/Portty",
		"/tmp/portty.test",
	} {
		if got, err := bundleFromExecutable(exe); err == nil {
			t.Errorf("bundleFromExecutable(%q) = %q, want an error", exe, got)
		}
	}
}
