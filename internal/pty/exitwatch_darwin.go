package pty

import (
	"errors"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// WatchExit reports pid's exit code once, on a buffered channel, for a
// process this one may not have forked (a session adopted in a handoff, or
// the takeover daemon). -1 means the process is gone but its status is
// unknown: it was killed by a signal (as ProcessState.ExitCode reports), it
// was already gone at registration, or macOS refused the status. The engine
// shows -1 as today's default (D-10), as CONTEXT decided.
//
// man 2 kqueue says NOTE_EXITSTATUS is "Valid only on child processes", yet
// on macOS 27.2 it returned the real code for a non-child (probe P1:
// data=1792, exited code 7). Hence the EACCES fallback to NOTE_EXIT alone.
func WatchExit(pid int) <-chan int {
	ch := make(chan int, 1)
	kq, err := unix.Kqueue()
	if err != nil {
		go pollExit(pid, ch)
		return ch
	}
	ev := []unix.Kevent_t{{Ident: uint64(pid), Filter: unix.EVFILT_PROC,
		Flags: unix.EV_ADD | unix.EV_ONESHOT, Fflags: unix.NOTE_EXIT | unix.NOTE_EXITSTATUS}}
	statusKnown := true
	_, err = unix.Kevent(kq, ev, nil, nil)
	if errors.Is(err, unix.EACCES) {
		statusKnown = false
		ev[0].Fflags = unix.NOTE_EXIT
		_, err = unix.Kevent(kq, ev, nil, nil)
	}
	switch {
	case errors.Is(err, unix.ESRCH): // already gone (Pitfall 8)
		_ = unix.Close(kq)
		ch <- -1
		return ch
	case err != nil:
		_ = unix.Close(kq)
		go pollExit(pid, ch)
		return ch
	}
	// One blocked thread per watched pid; there are only a handful of sessions.
	go func() {
		defer unix.Close(kq) //nolint:errcheck
		out := make([]unix.Kevent_t, 1)
		for {
			n, err := unix.Kevent(kq, nil, out, nil)
			if errors.Is(err, unix.EINTR) || (err == nil && n == 0) {
				continue
			}
			if err != nil {
				pollExit(pid, ch)
				return
			}
			ws := syscall.WaitStatus(out[0].Data)
			if statusKnown && ws.Exited() {
				ch <- ws.ExitStatus()
			} else {
				ch <- -1
			}
			return
		}
	}()
	return ch
}

// pollExit is the fallback when kqueue itself fails: report -1 once pid is
// gone. A reused pid only makes it wait longer, never fire early.
func pollExit(pid int, ch chan<- int) {
	for unix.Kill(pid, 0) != unix.ESRCH {
		time.Sleep(100 * time.Millisecond)
	}
	ch <- -1
}
