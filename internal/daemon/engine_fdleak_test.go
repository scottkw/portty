package daemon

import (
	"context"
	"net"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/pty"
	"golang.org/x/sys/unix"
)

// ptyFDCount counts this process's open character-device descriptors. A
// created session holds three (go-pty's /dev/ptmx master, the Phase 9
// pollable dup and the slave), an adopted one two (dup and slave); nothing
// else in these tests opens character devices once the binary is running.
// Once the shell exits macOS revokes the slave: it stays open, but fstat
// fails with EBADF, so an open descriptor fstat refuses counts too.
func ptyFDCount(t *testing.T) int {
	t.Helper()
	dir, err := os.Open("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, name := range names {
		fd, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err != nil {
			continue // closed since the listing (the /dev/fd handle itself)
		}
		var st syscall.Stat_t
		if syscall.Fstat(fd, &st) != nil || st.Mode&syscall.S_IFMT == syscall.S_IFCHR {
			n++
		}
	}
	return n
}

// ptyFDBaseline counts once pending *os.File finalizers have run, so a file
// an earlier test dropped cannot close mid-measurement and hide a leak.
func ptyFDBaseline(t *testing.T) int {
	t.Helper()
	for range 2 { // the second sentinel is queued behind every finalizer of the first GC
		done := make(chan struct{})
		runtime.SetFinalizer(new([64]byte), func(*[64]byte) { close(done) })
		runtime.GC()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}
	return ptyFDCount(t)
}

// checkPTYFDs fails if more character devices are open than at base. Only
// growth fails: a session an earlier test leaked may still close meanwhile.
func checkPTYFDs(t *testing.T, base int, what string) {
	t.Helper()
	if got := ptyFDCount(t); got > base {
		t.Errorf("%s: %d PTY descriptors still open (baseline %d)", what, got-base, base)
	}
}

func createSessions(t *testing.T, e *SessionEngine, n int) []string {
	t.Helper()
	t.Cleanup(func() { killAllSessions(e) }) // a failed check must not leave PTYs behind
	ids := make([]string, n)
	for i := range ids {
		id, err := e.CreateSession(context.Background(), "fd", "", 80, 24, nil)
		if err != nil {
			t.Fatalf("CreateSession %d: %v", i, err)
		}
		ids[i] = id
	}
	return ids
}

// passFDs sends master and slave across a socketpair with the handoff's own
// sendFDs/recvFDs, so the receiver holds new descriptors as a takeover does.
func passFDs(t *testing.T, master, slave *os.File) (*os.File, *os.File) {
	t.Helper()
	sp, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
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
	a, b := conn(sp[0]), conn(sp[1])
	if err := sendFDs(a, 1, master, slave); err != nil {
		t.Fatalf("sendFDs: %v", err)
	}
	files, err := recvFDs(b, 1, 2)
	if err != nil {
		t.Fatalf("recvFDs: %v", err)
	}
	return files[0], files[1]
}

// TestEngine_PTYFDsReleased: every way a session ends in a daemon that keeps
// running gives back its PTY master, the pollable dup and the slave. A leak
// here exhausts kern.tty.ptmx_max (511 on macOS) on a long-lived daemon.
// Daemon shutdown and the old side of a committed handoff end in process
// exit, which releases everything, so they have no in-process case.
func TestEngine_PTYFDsReleased(t *testing.T) {
	const n = 5

	t.Run("KillSession", func(t *testing.T) {
		e := newCatEngine()
		base := ptyFDBaseline(t)
		ids := createSessions(t, e, n)
		if got := ptyFDCount(t); got < base+3*n {
			t.Fatalf("%d sessions show %d new character devices, want %d: the counter cannot see them", n, got-base, 3*n)
		}
		for _, id := range ids {
			if err := e.KillSession(id); err != nil {
				t.Fatal(err)
			}
		}
		checkPTYFDs(t, base, "after KillSession")
	})

	t.Run("natural exit then dismiss", func(t *testing.T) {
		e := newCatEngine()
		e.SetSpawnCommandForTest("/bin/sh", []string{"-c", "exit 3"})
		base := ptyFDBaseline(t)
		ids := createSessions(t, e, n)
		waitFor(t, "every shell to exit", 10*time.Second, func() bool {
			for _, id := range ids {
				if s, ok := e.registry.Get(id); !ok || s.GetState() != pty.StateStopped {
					return false
				}
			}
			return true
		})
		// By design the stopped card keeps its PTY until it is dismissed
		// (the GUI dismisses stopped sessions with KillSession, TERM-05).
		t.Logf("%d stopped sessions hold %d PTY descriptors until dismissed", n, ptyFDCount(t)-base)
		for _, id := range ids {
			if err := e.KillSession(id); err != nil {
				t.Fatal(err)
			}
		}
		checkPTYFDs(t, base, "after natural exit and dismissal")
	})

	t.Run("failed spawn", func(t *testing.T) {
		e := newCatEngine()
		e.SetSpawnCommandForTest("/nonexistent/portty-fd-test", nil)
		base := ptyFDBaseline(t)
		for range n {
			if _, err := e.CreateSession(context.Background(), "fd", "", 80, 24, nil); err == nil {
				t.Fatal("CreateSession of a missing binary succeeded")
			}
		}
		checkPTYFDs(t, base, "after failed spawns")
	})

	// Export from one engine, pass the fds over SCM_RIGHTS, adopt in another,
	// kill there; then the old engine releases its copies (in production its
	// process exits instead). The received master must be closed by Adopt.
	t.Run("handoff export and adopt", func(t *testing.T) {
		old, nw := newCatEngine(), newCatEngine()
		base := ptyFDBaseline(t)
		id := createSessions(t, old, 1)[0]
		sess, _ := old.registry.Get(id)
		hub, _ := old.manager.Get(id)
		if err := hub.Freeze(); err != nil {
			t.Fatalf("Freeze: %v", err)
		}
		st, err := hub.Export()
		if err != nil {
			t.Fatalf("Export: %v", err)
		}
		master, slave, pid, err := sess.HandoffFiles()
		if err != nil {
			t.Fatal(err)
		}
		rm, rs := passFDs(t, master, slave)
		meta := pty.AdoptMeta{ID: id, CLI: sess.CLI, CreatedAt: sess.CreatedAt, Pid: pid}
		_, nhub, err := nw.importSession(meta, "fd", "", st, rm, rs)
		if err != nil {
			t.Fatalf("importSession: %v", err)
		}
		if got := ptyFDCount(t); got > base+5 {
			t.Errorf("after adopt: %d PTY descriptors, want 5 (old 3, new dup and slave)", got-base)
		}
		nhub.Start()
		if err := nw.KillSession(id); err != nil {
			t.Fatal(err)
		}
		if got := ptyFDCount(t); got > base+3 {
			t.Errorf("after killing the adopted session: %d PTY descriptors, want the old engine's 3", got-base)
		}
		if err := old.KillSession(id); err != nil { // reaps the child too: this process forked it
			t.Fatal(err)
		}
		checkPTYFDs(t, base, "after the handoff round trip")
	})
}
