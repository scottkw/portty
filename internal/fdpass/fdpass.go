// Package fdpass moves file descriptors between processes over a unix socket
// with SCM_RIGHTS, and reads the kernel-attested identity of the peer. The
// daemon handoff uses it to give PTY masters, listeners and the daemon lock
// to the takeover daemon (Phase 9, UPGR-01).
//
// Two rules:
//   - Never call the Fd method or File() on a descriptor that will be passed:
//     Fd switches it to blocking mode, which breaks deadlines on a PTY master
//     for every holder of the open file description. Send takes syscall.Conn
//     and reads the descriptor inside SyscallConn().Control.
//   - Recv closes every descriptor that arrived whenever it returns an error,
//     so a truncated or malformed message never leaks a PTY or a listener.
package fdpass

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// maxOOB is Recv's minimum control buffer size (see Recv).
const maxOOB = 4096

// Send writes header with one SCM_RIGHTS control message carrying a
// duplicate of each conn's descriptor. *os.File, *net.UnixListener and
// *net.TCPListener all implement syscall.Conn.
func Send(c *net.UnixConn, header []byte, conns ...syscall.Conn) error {
	dups := make([]int, 0, len(conns))
	defer func() {
		for _, d := range dups {
			_ = unix.Close(d)
		}
	}()
	for _, sc := range conns {
		raw, err := sc.SyscallConn()
		if err != nil {
			return err
		}
		var d int
		var derr error
		// A close-on-exec dup stays valid after Control returns and never leaks into a child.
		if err := raw.Control(func(fd uintptr) { d, derr = unix.FcntlInt(fd, unix.F_DUPFD_CLOEXEC, 0) }); err != nil {
			return err
		}
		if derr != nil {
			return fmt.Errorf("fdpass: dup: %w", derr)
		}
		dups = append(dups, d)
	}
	_, _, err := c.WriteMsgUnix(header, unix.UnixRights(dups...), nil)
	return err
}

// Recv reads one message into header and expects exactly nfds descriptors
// with it. On a truncated control message, a parse error or any other count
// it closes every descriptor that arrived and returns an error. Received
// descriptors are marked close-on-exec and wrapped with os.NewFile.
func Recv(c *net.UnixConn, header []byte, nfds int) (n int, files []*os.File, err error) {
	// macOS installs every passed fd even when the control buffer is too small
	// to list them, so a buffer sized for nfds would leak the extras out of
	// sight. 4 KiB holds every fd the kernel accepts in one message (measured
	// on macOS 27.2: 254 fds sent, 400 refused with EINVAL), so any extras show
	// up as a count mismatch and are closed below.
	oob := make([]byte, max(unix.CmsgSpace(nfds*4), maxOOB))
	n, oobn, flags, _, err := c.ReadMsgUnix(header, oob)
	if err != nil {
		return n, nil, err
	}
	var fds []int
	var perr error
	if oobn > 0 {
		fds, perr = rightsFromOOB(oob[:oobn])
	}
	switch {
	case flags&unix.MSG_CTRUNC != 0:
		err = errors.New("fdpass: control message truncated (MSG_CTRUNC)")
	case perr != nil:
		err = fmt.Errorf("fdpass: parse control message: %w", perr)
	case len(fds) != nfds:
		err = fmt.Errorf("fdpass: got %d descriptors, want %d", len(fds), nfds)
	}
	if err != nil {
		for _, fd := range fds {
			_ = unix.Close(fd)
		}
		return n, nil, err
	}
	files = make([]*os.File, len(fds))
	for i, fd := range fds {
		// shortcut: a fork between recvmsg and here could inherit fd (darwin has no MSG_CMSG_CLOEXEC); the window is microseconds and only the handoff receives fds.
		unix.CloseOnExec(fd)
		files[i] = os.NewFile(uintptr(fd), "fdpass")
	}
	return n, files, nil
}

// rightsFromOOB returns the SCM_RIGHTS descriptors in oob. It walks the
// control messages one at a time, so a malformed message still returns the
// descriptors of every message before it (the whole-buffer parser returns
// none). The caller closes them; this never closes anything itself.
func rightsFromOOB(oob []byte) ([]int, error) {
	var fds []int
	for len(oob) >= unix.CmsgLen(0) {
		hdr, data, rest, err := unix.ParseOneSocketControlMessage(oob)
		if err != nil {
			return fds, err
		}
		if hdr.Level == unix.SOL_SOCKET && hdr.Type == unix.SCM_RIGHTS {
			got, err := unix.ParseUnixRights(&unix.SocketControlMessage{Header: hdr, Data: data})
			if err != nil {
				return fds, err
			}
			fds = append(fds, got...)
		}
		oob = rest
	}
	return fds, nil
}

// PeerUID returns the effective uid of the process at the other end of c,
// as recorded by the kernel when the connection was made (LOCAL_PEERCRED).
func PeerUID(c *net.UnixConn) (uint32, error) {
	var cred *unix.Xucred
	err := control(c, func(fd int) (e error) {
		cred, e = unix.GetsockoptXucred(fd, unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		return e
	})
	if err != nil {
		return 0, fmt.Errorf("fdpass: LOCAL_PEERCRED: %w", err)
	}
	return cred.Uid, nil
}

// PeerPID returns the pid of the process at the other end of c (LOCAL_PEERPID).
func PeerPID(c *net.UnixConn) (int, error) {
	var pid int
	err := control(c, func(fd int) (e error) {
		pid, e = unix.GetsockoptInt(fd, unix.SOL_LOCAL, unix.LOCAL_PEERPID)
		return e
	})
	if err != nil {
		return 0, fmt.Errorf("fdpass: LOCAL_PEERPID: %w", err)
	}
	return pid, nil
}

func control(c *net.UnixConn, f func(fd int) error) error {
	raw, err := c.SyscallConn()
	if err != nil {
		return err
	}
	var ferr error
	if err := raw.Control(func(fd uintptr) { ferr = f(int(fd)) }); err != nil {
		return err
	}
	return ferr
}
