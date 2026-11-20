//go:build !windows

package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// startDetachedDaemon spawns the daemon as a detached child process on Unix.
// Setsid creates a new session so the daemon is not killed when the parent exits.
// The child's stdout and stderr are appended to daemon.log in the config dir
// (the daemon rotates it, see startDaemonLog); a log problem is reported on
// the parent's stderr but never stops the daemon.
//
// The returned channel closes when the child exits (WR-04): waiting on it
// reaps the child, so a daemon that dies is no zombie, and lets EnsureDaemon
// stop waiting for one that died at spawn. kill sends SIGTERM to the child, so
// a cancelled EnsureDaemon (IN-07) can stop the fresh daemon it started; it is
// a no-op once exited has closed, so a reaped pid is never signalled.
func startDetachedDaemon(exe string) (exited <-chan struct{}, kill func(), err error) {
	cmd := exec.Command(exe, "daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	logFile, err := openDaemonLog(daemonConfigDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "portty: daemon log: %v\n", err)
	} else {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close() // the child keeps its own descriptor
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }() // Stdout/Stderr are files: no copy goroutines to wait on
	kill = func() {
		select {
		case <-done:
		default:
			_ = cmd.Process.Signal(syscall.SIGTERM) // os.Process refuses a process Wait has reaped
		}
	}
	return done, kill, nil
}

// startDetachedTakeover spawns `exe daemon --takeover` like startDetachedDaemon
// (Setsid, output to daemon.log), with the write end of a status pipe as its
// fd 3 and PORTTY_TAKEOVER_STATUS_FD=3 (RunTakeover). outcome yields the one
// byte the takeover writes (0 once committed, while it keeps running as the
// daemon), or, when it exits without writing, its exit code (1 when a signal
// killed it or it exited 0 unreported: only a written 0 means committed).
// The child is always reaped. There is no kill func on purpose: after COMMIT
// the takeover owns every session (Quit All must never end it).
func startDetachedTakeover(exe string) (pid int, outcome <-chan int, err error) {
	r, w, err := os.Pipe()
	if err != nil {
		return 0, nil, err
	}
	defer w.Close() // the child has its own copy after Start: EOF once it exits
	cmd := exec.Command(exe, "daemon", "--takeover")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.ExtraFiles = []*os.File{w} // fd 3
	cmd.Env = append(os.Environ(), takeoverStatusEnv+"=3")
	logFile, err := openDaemonLog(daemonConfigDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "portty: daemon log: %v\n", err)
	} else {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close()
	}
	if err := cmd.Start(); err != nil {
		r.Close()
		return 0, nil, err
	}
	out := make(chan int, 1)
	go func() {
		var b [1]byte
		n, _ := r.Read(b[:])
		r.Close()
		if n == 1 {
			out <- int(b[0])
		}
		_ = cmd.Wait()
		if n == 0 {
			code := cmd.ProcessState.ExitCode()
			if code <= 0 {
				code = 1
			}
			out <- code
		}
	}()
	return cmd.Process.Pid, out, nil
}

// lockHeld reports whether another process holds the daemon lock at path.
func lockHeld(path string) bool {
	f, err := lockDaemon(path, 0)
	if err != nil {
		return errors.Is(err, syscall.EWOULDBLOCK)
	}
	f.Close()
	return false
}

// lockDaemon takes the exclusive daemon lock at path, retrying for up to wait
// so a daemon that was just told to stop (EnsureDaemon's version-mismatch
// restart) can exit first. Only the holder may clean up and bind the socket,
// so two daemons spawned at once never unlink each other's socket (WR-05).
// The kernel drops the lock when the process exits; the caller keeps the
// file open for its lifetime.
func lockDaemon(path string, wait time.Duration) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || !time.Now().Before(deadline) {
			f.Close()
			return nil, err
		}
		time.Sleep(50 * time.Millisecond)
	}
}
