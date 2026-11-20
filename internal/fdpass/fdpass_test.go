package fdpass

import (
	"bufio"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// socketpair returns both ends of a connected unix stream socketpair.
func socketpair(t *testing.T) (*net.UnixConn, *net.UnixConn) {
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

func pipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })
	return r, w
}

func openFds(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(ents)
}

// shortTempDir is under /tmp so unix socket paths fit sun_path.
func shortTempDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "fdp-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

func TestSendRecv_PipeFds(t *testing.T) {
	a, b := socketpair(t)
	r, w := pipe(t)

	if err := Send(a, []byte("hdr"), w); err != nil {
		t.Fatalf("Send: %v", err)
	}
	hdr := make([]byte, 16)
	n, files, err := Recv(b, hdr, 1)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	defer files[0].Close()
	if string(hdr[:n]) != "hdr" || len(files) != 1 {
		t.Fatalf("Recv = %q, %d files", hdr[:n], len(files))
	}
	if _, err := files[0].Write([]byte("ping\n")); err != nil {
		t.Fatalf("write on received fd: %v", err)
	}
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil || line != "ping\n" {
		t.Fatalf("pipe read = %q, %v", line, err)
	}
}

func TestRecv_TruncatedClosesFds(t *testing.T) {
	a, b := socketpair(t)
	_, w1 := pipe(t)
	_, w2 := pipe(t)
	_, w3 := pipe(t)
	before := openFds(t)

	if err := Send(a, []byte("x"), w1, w2, w3); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if _, files, err := Recv(b, make([]byte, 8), 1); err == nil {
		t.Fatalf("Recv of 3 fds into room for 1 succeeded with %d files", len(files))
	}
	if after := openFds(t); after != before {
		t.Fatalf("open fds %d after truncated Recv, want %d (leak)", after, before)
	}
}

func TestRecv_CountMismatch(t *testing.T) {
	a, b := socketpair(t)
	_, w := pipe(t)
	before := openFds(t)

	if err := Send(a, []byte("x"), w); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if _, files, err := Recv(b, make([]byte, 8), 2); err == nil {
		t.Fatalf("Recv expecting 2 fds got %d and no error", len(files))
	}
	if after := openFds(t); after != before {
		t.Fatalf("open fds %d after mismatched Recv, want %d (leak)", after, before)
	}
}

func TestPeerUID(t *testing.T) {
	a, b := socketpair(t)
	for _, c := range []*net.UnixConn{a, b} {
		uid, err := PeerUID(c)
		if err != nil {
			t.Fatalf("PeerUID: %v", err)
		}
		if uid != uint32(os.Getuid()) {
			t.Fatalf("PeerUID = %d, want %d", uid, os.Getuid())
		}
	}
}

func TestPeerPID(t *testing.T) {
	a, b := socketpair(t)
	for _, c := range []*net.UnixConn{a, b} {
		pid, err := PeerPID(c)
		if err != nil {
			t.Fatalf("PeerPID: %v", err)
		}
		if pid != os.Getpid() {
			t.Fatalf("socketpair PeerPID = %d, want %d", pid, os.Getpid())
		}
	}

	path := filepath.Join(shortTempDir(t), "p.sock")
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	nc := exec.Command("/usr/bin/nc", "-U", path)
	stdin, err := nc.StdinPipe() // held open so nc stays connected
	if err != nil {
		t.Fatal(err)
	}
	if err := nc.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = nc.Process.Kill(); _ = nc.Wait() })

	_ = ln.SetDeadline(time.Now().Add(5 * time.Second))
	c, err := ln.AcceptUnix()
	if err != nil {
		t.Fatalf("accept the child's dial: %v", err)
	}
	defer c.Close()
	pid, err := PeerPID(c)
	if err != nil {
		t.Fatalf("PeerPID: %v", err)
	}
	if pid != nc.Process.Pid {
		t.Fatalf("PeerPID = %d, want the child's pid %d", pid, nc.Process.Pid)
	}
}

// A4: flock belongs to the open file description, so a lock passed by fd
// stays held until the last copy closes.
func TestFlockFollowsPassedFd(t *testing.T) {
	a, b := socketpair(t)
	path := filepath.Join(t.TempDir(), "daemon.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	flock := func(f *os.File, how int) error {
		sc, err := f.SyscallConn()
		if err != nil {
			return err
		}
		var ferr error
		if err := sc.Control(func(fd uintptr) { ferr = unix.Flock(int(fd), how) }); err != nil {
			return err
		}
		return ferr
	}
	if err := flock(f, unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	if err := Send(a, []byte("lock"), f); err != nil {
		t.Fatalf("Send: %v", err)
	}
	_, files, err := Recv(b, make([]byte, 8), 1)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	_ = f.Close() // the sender's copy

	probe, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	if err := flock(probe, unix.LOCK_EX|unix.LOCK_NB); !errors.Is(err, unix.EWOULDBLOCK) {
		t.Fatalf("lock while the passed copy is open: %v, want EWOULDBLOCK", err)
	}
	_ = files[0].Close()
	if err := flock(probe, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatalf("lock after the last copy closed: %v", err)
	}
}

// A5 / Pattern 7: a unix listener passed by fd keeps serving the same path
// after the sender closes its copy without unlinking.
func TestUnixListenerMovesWithoutUnlink(t *testing.T) {
	a, b := socketpair(t)
	path := filepath.Join(shortTempDir(t), "d.sock")
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	orig, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := Send(a, []byte("ln"), ln); err != nil {
		t.Fatalf("Send: %v", err)
	}
	_, files, err := Recv(b, make([]byte, 8), 1)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	moved, err := net.FileListener(files[0])
	_ = files[0].Close()
	if err != nil {
		t.Fatalf("FileListener: %v", err)
	}
	defer moved.Close()

	acceptOne := func() {
		t.Helper()
		c, err := net.Dial("unix", path)
		if err != nil {
			t.Fatalf("dial %s: %v", path, err)
		}
		defer c.Close()
		_ = moved.(*net.UnixListener).SetDeadline(time.Now().Add(5 * time.Second))
		s, err := moved.Accept()
		if err != nil {
			t.Fatalf("moved listener accept: %v", err)
		}
		_ = s.Close()
	}
	acceptOne()

	ln.SetUnlinkOnClose(false)
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	now, err := os.Stat(path)
	if err != nil {
		t.Fatalf("socket path gone after the sender closed: %v", err)
	}
	if !os.SameFile(orig, now) {
		t.Fatal("socket path names a different inode after the sender closed")
	}
	acceptOne()
}

// Send must not need the Fd method: every passable type implements syscall.Conn.
var _ = []syscall.Conn{(*os.File)(nil), (*net.UnixListener)(nil), (*net.TCPListener)(nil)}
