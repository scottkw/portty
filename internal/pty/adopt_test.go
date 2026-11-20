//go:build darwin

package pty

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/fdpass"
	"golang.org/x/sys/unix"
)

// outBuf collects a session's output from a reader goroutine.
type outBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
	eof chan struct{}
}

func readAll(sess *Session) *outBuf {
	o := &outBuf{eof: make(chan struct{})}
	go func() {
		defer close(o.eof)
		b := make([]byte, 4096)
		for {
			n, err := sess.Read(b)
			o.mu.Lock()
			o.buf.Write(b[:n])
			o.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return o
}

func (o *outBuf) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.String()
}

func (o *outBuf) waitFor(t *testing.T, s string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(o.String(), s) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %q; output so far: %q", s, o.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func unixPair(t *testing.T) (*net.UnixConn, *net.UnixConn) {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	conn := func(fd int) *net.UnixConn {
		f := os.NewFile(uintptr(fd), "sp")
		defer f.Close()
		c, err := net.FileConn(f)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		return c.(*net.UnixConn)
	}
	return conn(fds[0]), conn(fds[1])
}

// adoptVia creates `sh -c script` on one backend, moves its master and slave
// to a second backend over a socketpair with fdpass, as the daemon handoff
// does, and closes the original copies without a kill (the old daemon exits).
func adoptVia(t *testing.T, script string) (*NativePTYBackend, *Session) {
	t.Helper()
	b1 := NewNativePTYBackend()
	orig := startSh(t, b1, script)
	master, slave, pid, err := orig.HandoffFiles()
	if err != nil {
		t.Fatalf("HandoffFiles: %v", err)
	}
	if pid != orig.Pid() || pid <= 0 {
		t.Fatalf("HandoffFiles pid %d, Pid() %d", pid, orig.Pid())
	}

	a, b := unixPair(t)
	if err := fdpass.Send(a, []byte(orig.ID), master, slave); err != nil {
		t.Fatalf("fdpass.Send: %v", err)
	}
	hdr := make([]byte, 64)
	n, files, err := fdpass.Recv(b, hdr, 2)
	if err != nil {
		t.Fatalf("fdpass.Recv: %v", err)
	}

	b2 := NewNativePTYBackend()
	adopted, err := b2.Adopt(AdoptMeta{ID: string(hdr[:n]), CLI: orig.CLI, CreatedAt: orig.CreatedAt, Pid: pid}, files[0], files[1])
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	t.Cleanup(func() { _ = b2.Kill(adopted.ID) })
	orig.closePTY()
	return b2, adopted
}

// T-P2: an adopted session reads, writes and resizes, and its shell is not
// hung up when the sender's copies close.
func TestAdopt_FromPassedFds(t *testing.T) {
	b2, sess := adoptVia(t, `trap 'echo GOT-HUP; exit 9' HUP; while read l; do case $l in size) stty size;; *) echo got:$l;; esac; done`)
	if sess.ID == "" || sess.GetState() != StateRunning || len(b2.List()) != 1 {
		t.Fatalf("adopted session not registered: %+v", sess)
	}
	out := readAll(sess)

	if _, err := sess.Write([]byte("hi\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out.waitFor(t, "got:hi")

	if err := b2.Resize(sess.ID, 100, 30); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if _, err := sess.Write([]byte("size\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out.waitFor(t, "30 100")
	if strings.Contains(out.String(), "GOT-HUP") {
		t.Fatalf("shell got a hangup: %q", out.String())
	}
}

// orphan starts `sh -c script` and returns the pid it prints: a background
// process that launchd adopts once sh exits.
func orphan(t *testing.T, script string) int {
	t.Helper()
	b, err := exec.Command("/bin/sh", "-c", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("pid %q: %v", b, err)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	return pid
}

// T-P3: the exit status of a process this one did not fork.
func TestExitWatch_Orphan(t *testing.T) {
	pid := orphan(t, `(sleep 0.3; exit 7) >/dev/null 2>&1 & echo $!`)
	select {
	case code := <-WatchExit(pid):
		switch code {
		case 7:
			t.Log("NOTE_EXITSTATUS reported the orphan's real exit code 7")
		case -1:
			t.Log("macOS refused NOTE_EXITSTATUS for a non-child: exit code unknown (-1)")
		default:
			t.Fatalf("WatchExit = %d, want 7 or -1", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("WatchExit never fired")
	}
}

func TestExitWatch_AlreadyGone(t *testing.T) {
	cmd := exec.Command("/usr/bin/true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-WatchExit(cmd.Process.Pid):
		if code != -1 {
			t.Fatalf("WatchExit on a reaped pid = %d, want -1", code)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("WatchExit on a reaped pid did not report within 100ms")
	}
}

// 09-04's takeover watch: a same-uid process this test did not fork.
func TestExitWatch_NonChildKilled(t *testing.T) {
	pid := orphan(t, `sleep 30 >/dev/null 2>&1 & echo $!`)
	ch := WatchExit(pid)
	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-ch:
		if code != -1 {
			t.Fatalf("WatchExit after SIGKILL = %d, want -1", code)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("WatchExit did not fire within 200ms of kill -9")
	}
}

func TestAdopted_ReapNaturalExit(t *testing.T) {
	_, sess := adoptVia(t, `read x; exit 3`)
	out := readAll(sess)
	if _, err := sess.Write([]byte("\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	select {
	case <-out.eof:
	case <-time.After(5 * time.Second):
		t.Fatal("no EOF after the adopted shell exited")
	}
	sess.ReapNaturalExit()
	switch code := sess.ExitCode(); code {
	case 3:
	case -1:
		t.Log("exit status unknown (-1 fallback)")
	default:
		t.Fatalf("ExitCode = %d, want 3 or -1", code)
	}
}

// recordSignals replaces the kill seam and returns the signals sent.
func recordSignals(t *testing.T) func() []syscall.Signal {
	t.Helper()
	var mu sync.Mutex
	var sent []syscall.Signal
	prev := signalGroup
	signalGroup = func(pid int, sig syscall.Signal) error {
		mu.Lock()
		sent = append(sent, sig)
		mu.Unlock()
		return prev(pid, sig)
	}
	t.Cleanup(func() { signalGroup = prev })
	return func() []syscall.Signal {
		mu.Lock()
		defer mu.Unlock()
		return append([]syscall.Signal(nil), sent...)
	}
}

// T-P4 / Pitfall 9: kill escalates while the process lives and never signals
// a pid whose exit the watcher has already seen.
func TestKillAdopted(t *testing.T) {
	sent := recordSignals(t)

	b2, sess := adoptVia(t, `trap "" HUP; echo ready; sleep 30`)
	readAll(sess).waitFor(t, "ready") // the HUP trap is in place
	start := time.Now()
	if err := b2.Kill(sess.ID); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	select {
	case <-sess.exited:
	case <-time.After(3500*time.Millisecond - time.Since(start)):
		t.Fatal("exit watcher did not fire within 3.5s of Kill")
	}
	if got := sent(); len(got) != 2 || got[0] != syscall.SIGHUP || got[1] != syscall.SIGKILL {
		t.Fatalf("signals sent = %v, want [SIGHUP SIGKILL]", got)
	}

	b3, gone := adoptVia(t, `read x; exit 0`)
	readAll(gone) // exit waits for the echoed output to drain, like the hub does
	if _, err := gone.Write([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-gone.exited:
	case <-time.After(5 * time.Second):
		t.Fatal("exit watcher did not fire for the exited shell")
	}
	before := len(sent())
	if err := b3.Kill(gone.ID); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if got := sent(); len(got) != before {
		t.Fatalf("Kill of an exited adopted session sent %v", got[before:])
	}
}
