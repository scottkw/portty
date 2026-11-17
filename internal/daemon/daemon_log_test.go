package daemon

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/relay"
)

// Every test here uses t.TempDir(); the real config directory is never touched.

func TestOpenDaemonLog_RotatesAndIs0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.log")
	if err := os.WriteFile(path, []byte("old run"), 0600); err != nil {
		t.Fatal(err)
	}

	f, err := openDaemonLog(dir)
	if err != nil {
		t.Fatalf("openDaemonLog: %v", err)
	}
	if got, _ := os.ReadFile(path + ".1"); string(got) != "old run" {
		t.Fatalf("daemon.log.1 = %q, want %q", got, "old run")
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("daemon.log mode = %v, want 0600", fi.Mode().Perm())
	}
	if fi.Size() != 0 {
		t.Fatalf("fresh daemon.log size = %d, want 0", fi.Size())
	}
	if _, err := f.WriteString("second run"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if got, _ := os.ReadFile(path); string(got) != "second run" {
		t.Fatalf("daemon.log = %q, want %q", got, "second run")
	}

	f2, err := openDaemonLog(dir)
	if err != nil {
		t.Fatalf("second openDaemonLog: %v", err)
	}
	f2.Close()
	if got, _ := os.ReadFile(path + ".1"); string(got) != "second run" {
		t.Fatalf("after second rotation daemon.log.1 = %q, want %q", got, "second run")
	}
}

func TestDaemonLog_GridChangeLandsInFile(t *testing.T) {
	dir := t.TempDir()
	f, err := openDaemonLog(dir)
	if err != nil {
		t.Fatalf("openDaemonLog: %v", err)
	}
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(f)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		f.Close()
	})

	const sessionID = "abcdef0123456789"
	hub := relay.NewHub(sessionID, strings.NewReader(""), io.Discard, relay.DefaultScrollbackBytes, nil)
	sub := &relay.Subscriber{Msgs: make(chan []byte, 8), CloseSlow: func() {}, Origin: "local", Name: "panel"}
	hub.Subscribe(sub)
	if err := hub.ResizeClient(sub, 120, 40); err != nil {
		t.Fatalf("ResizeClient: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "daemon.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"viewer-diag: grid session=" + sessionID[:8], "0x0 -> 120x40", "by=local"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("daemon.log lacks %q:\n%s", want, got)
		}
	}
}
