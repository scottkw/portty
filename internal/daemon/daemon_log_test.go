package daemon

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/relay"
)

// Every test here uses t.TempDir(); the real config directory is never touched.

// WR-01: the spawning parent appends, so a duplicate or retried spawn never
// moves away the log of the run that just died.
func TestOpenDaemonLog_AppendsAndIs0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.log")
	if err := os.WriteFile(path, []byte("crashed run\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"spawn 1\n", "spawn 2\n"} {
		f, err := openDaemonLog(dir)
		if err != nil {
			t.Fatalf("openDaemonLog: %v", err)
		}
		if _, err := f.WriteString(line); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	if got, _ := os.ReadFile(path); string(got) != "crashed run\nspawn 1\nspawn 2\n" {
		t.Fatalf("daemon.log = %q; want all three runs appended", got)
	}
	if _, err := os.Stat(path + ".1"); !os.IsNotExist(err) {
		t.Fatalf("daemon.log.1 exists after spawns (err=%v); the parent must not rotate", err)
	}

	f, err := openDaemonLog(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if fi, _ := f.Stat(); fi.Mode().Perm() != 0600 {
		t.Fatalf("new daemon.log mode = %v, want 0600", fi.Mode().Perm())
	}
}

// WR-01: past the cap, daemon.log rolls over to daemon.log.1 and the
// descriptor (stderr in the daemon) follows to a fresh 0600 file.
func TestDaemonLog_RollsOverPastCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.log")
	out, err := openDaemonLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	d := &daemonLog{path: path, max: 10, out: out, fds: []int{int(out.Fd())}}

	for _, s := range []string{"aaaaaa", "bbbb", "cccccc"} {
		if _, err := d.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := os.ReadFile(path + ".1"); string(got) != "aaaaaabbbb" {
		t.Fatalf("daemon.log.1 = %q; want %q", got, "aaaaaabbbb")
	}
	if got, _ := os.ReadFile(path); string(got) != "cccccc" {
		t.Fatalf("daemon.log = %q; want %q", got, "cccccc")
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("rotated daemon.log mode = %v, want 0600", fi.Mode().Perm())
	}
	// A direct write on the descriptor (a panic, fmt.Fprintf(os.Stderr)) lands in the new file too.
	if _, err := out.WriteString("dd"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "ccccccdd" {
		t.Fatalf("daemon.log after direct write = %q; want %q", got, "ccccccdd")
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

// 19-07 finding: launchd starts the daemon with stdio on /dev/null, so
// startDaemonLog must point it at daemon.log. The check runs in a child copy
// of the test binary whose stdio is /dev/null (exec.Cmd's default), so this
// process's own stderr is never redirected.
func TestStartDaemonLog_AdoptsDevNullStderr(t *testing.T) {
	if dir := os.Getenv("PORTTY_TEST_NULL_STDIO_DIR"); dir != "" {
		startDaemonLog(dir)
		log.Print("launchd-managed line")
		os.Exit(0)
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestStartDaemonLog_AdoptsDevNullStderr$")
	cmd.Env = append(os.Environ(), "PORTTY_TEST_NULL_STDIO_DIR="+dir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("child: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "daemon.log"))
	if err != nil {
		t.Fatalf("daemon.log: %v", err)
	}
	if !strings.Contains(string(got), "launchd-managed line") {
		t.Fatalf("daemon.log lacks the child's log line:\n%s", got)
	}
}
