package pty

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	gopty "github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/unix"
)

// SessionState represents the current lifecycle state of a Session.
type SessionState int

const (
	// StateRunning means the CLI process is active.
	StateRunning SessionState = iota

	// StateStopped means the CLI process has exited or been killed.
	StateStopped
)

// Session represents a single running CLI session backed by a PTY.
// It implements io.ReadWriter so output fan-out can be added in Phase 2.
type Session struct {
	// ID is the unique identifier for this session.
	ID string

	// CLI is the path of the binary that was launched (e.g. "/bin/zsh").
	CLI string

	// State is the current lifecycle state of the session.
	State SessionState

	// CreatedAt is the time the session was created.
	CreatedAt time.Time

	// pty is the underlying pseudo-terminal handle. Only Resize and Close go
	// through it: its master file is non-blocking once ioFile exists, and a
	// write on that unpolled file would return EAGAIN (Phase 9 Pitfall 2).
	pty gopty.Pty

	// ioFile is a pollable, non-blocking dup of the master. Every Read and
	// Write goes through it, so a read deadline can freeze the session's
	// reader without losing a byte (Phase 9, probe P3).
	ioFile *os.File

	// pid is the child's pid (and process group id: go-pty uses Setsid).
	pid int

	// Adopted sessions only (Adopt; nil/zero for created ones). slave is the
	// kept PTY slave (the daemon holds it, as go-pty does for created ones).
	// exited closes once the kqueue exit watcher fires; until then the
	// process is known alive, and after it the pid is never signalled again
	// (Phase 9 Pitfall 9). watchCode is the watcher's result (-1 unknown).
	slave     *os.File
	exited    chan struct{}
	watchCode int

	// cmd is the PTY-attached command (holds the process reference).
	cmd *gopty.Cmd

	// cancel stops the context that was used to start the process.
	cancel context.CancelFunc

	// mu protects concurrent access to mutable fields (State, pty, killed).
	mu sync.Mutex

	// killed is set by MarkKilled before killSession runs. The exit watcher
	// goroutine checks this to avoid calling cmd.Wait() on the kill path,
	// which would race with go-pty's internal waitOnContext goroutine.
	killed bool

	// reaped is set once cmd.Wait has returned for a created session
	// (ReapNaturalExit or killSession's wait). Its pgid may then belong to an
	// unrelated process group, so killSession never signals it again.
	reaped atomic.Bool

	// exitCode caches the exit code once the process has exited (-1 = not set).
	exitCode int

	// ptyCloseOnce gates s.pty.Close() so the killSession path and the Linux
	// exit detector goroutine cannot race on go-pty's unsynchronised
	// unixPty.Close() field writes. Both paths must close through closePTY().
	ptyCloseOnce sync.Once
}

// closePTY closes the underlying PTY exactly once across all callers,
// even when killSession and the Linux exit detector race to clean up.
func (s *Session) closePTY() {
	s.mu.Lock()
	p, f, sl := s.pty, s.ioFile, s.slave
	s.mu.Unlock()
	s.ptyCloseOnce.Do(func() {
		if f != nil {
			_ = f.Close()
		}
		if sl != nil {
			_ = sl.Close()
		}
		if p != nil {
			_ = p.Close()
		}
	})
}

// resize sets the terminal size: through go-pty for created sessions, and by
// ioctl on the I/O file for adopted ones, which have no go-pty handle.
func (s *Session) resize(cols, rows int) error {
	s.mu.Lock()
	p, f := s.pty, s.ioFile
	s.mu.Unlock()
	if p != nil {
		return p.Resize(cols, rows)
	}
	if f == nil {
		return fmt.Errorf("session %s: PTY not initialised", s.ID)
	}
	sc, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var ierr error
	if err := sc.Control(func(fd uintptr) {
		ierr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, &unix.Winsize{Row: uint16(rows), Col: uint16(cols)})
	}); err != nil {
		return err
	}
	return ierr
}

// rw returns the file session I/O goes through: the pollable dup when set.
func (s *Session) rw() (io.ReadWriter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ioFile != nil {
		return s.ioFile, nil
	}
	if s.pty == nil {
		return nil, fmt.Errorf("session %s: PTY not initialised", s.ID)
	}
	return s.pty, nil
}

// SetReadDeadline sets the read deadline on the session's pollable master.
// A session without one returns os.ErrNoDeadline, so a freeze fails fast
// instead of hanging on a blocking read (Phase 9 Pitfall 6).
func (s *Session) SetReadDeadline(t time.Time) error {
	s.mu.Lock()
	f := s.ioFile
	s.mu.Unlock()
	if f == nil {
		return os.ErrNoDeadline
	}
	return f.SetReadDeadline(t)
}

// Pid returns the child's pid (0 if unknown).
func (s *Session) Pid() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pid
}

// Read reads output from the underlying PTY.
// Implements io.Reader — Phase 2 will fan this out to connected WebSocket clients.
func (s *Session) Read(p []byte) (int, error) {
	f, err := s.rw()
	if err != nil {
		return 0, err
	}
	return f.Read(p)
}

// Write sends input to the underlying PTY (i.e. to the CLI process stdin).
// Implements io.Writer.
func (s *Session) Write(p []byte) (int, error) {
	f, err := s.rw()
	if err != nil {
		return 0, err
	}
	return f.Write(p)
}

// String returns a human-readable description of the session.
func (s *Session) String() string {
	return fmt.Sprintf("Session{ID: %q, CLI: %q}", s.ID, s.CLI)
}

// Signal sends the given signal to the session's child process.
// Returns an error if the process has not been started or has already exited.
// The caller must hold no lock on s.mu — this method acquires it internally.
func (s *Session) Signal(sig os.Signal) error {
	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("session %s: process not running", s.ID)
	}
	return cmd.Process.Signal(sig)
}

// MarkKilled sets the killed flag so the exit watcher skips cmd.Wait().
// Must be called before killSession triggers PTY close or context cancel.
func (s *Session) MarkKilled() {
	s.mu.Lock()
	s.killed = true
	s.mu.Unlock()
}

// IsKilled returns whether the session was killed (vs natural exit).
func (s *Session) IsKilled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.killed
}

// GetState returns the current lifecycle state under the internal mutex.
// Safe to call from any goroutine.
func (s *Session) GetState() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State
}

// SetState updates the session lifecycle state under the internal mutex.
// Safe to call from any goroutine.
func (s *Session) SetState(state SessionState) {
	s.mu.Lock()
	s.State = state
	s.mu.Unlock()
}

// CancelContext cancels the session's context, triggering go-pty's internal
// waitOnContext goroutine to reap the process.
func (s *Session) CancelContext() {
	if s.cancel != nil {
		s.cancel()
	}
}

// SetExitCode caches the exit code. Safe to call from any goroutine.
func (s *Session) SetExitCode(code int) {
	s.mu.Lock()
	s.exitCode = code
	s.mu.Unlock()
}

// ExitCode returns the cached exit code, or -1 if not yet set.
// Does NOT read cmd.ProcessState directly — that field is written by go-pty
// without our mutex and would race.
func (s *Session) ExitCode() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitCode
}
