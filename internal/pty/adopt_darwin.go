package pty

import (
	"errors"
	"fmt"
	"os"
	"time"

	gopty "github.com/aymanbagabas/go-pty"
)

// AdoptMeta describes a session handed over from another daemon (Phase 9).
type AdoptMeta struct {
	ID, CLI   string
	CreatedAt time.Time
	Pid       int
}

// Adopt registers a session rebuilt from a received PTY master and slave and
// the child's pid. This process is not the child's parent, so there is no
// exec.Cmd: exit status comes from a kqueue watcher, resize is an ioctl on
// the master, and kill signals only while the watcher has not fired.
//
// On success the session owns master's open file description through its own
// pollable dup (master itself is closed) and keeps slave. On error the caller
// still owns both.
func (b *NativePTYBackend) Adopt(meta AdoptMeta, master, slave *os.File) (*Session, error) {
	if meta.ID == "" || meta.Pid <= 1 || master == nil || slave == nil { // pid 1: killAdopted's kill(-pid) would be kill(-1)
		return nil, fmt.Errorf("adopt: incomplete session %q (pid %d)", meta.ID, meta.Pid)
	}
	// The open file description is already non-blocking (the sender set it);
	// the dup makes sure ours is netpoller-managed whatever master's state.
	ioFile, err := pollableDup(master, "ptmx")
	if err != nil {
		return nil, fmt.Errorf("adopt %s: dup master: %w", meta.ID, err)
	}
	_ = master.Close()
	// After the dup, so a failed dup leaves no watcher thread behind (IN-03).
	// An exit before this is still seen (Pitfall 8): a shell that exited
	// stays a zombie of the old daemon, frozen and not reaping, and the
	// registration then reports it gone at once (ESRCH).
	codes := WatchExit(meta.Pid)

	sess := &Session{
		ID:        meta.ID,
		CLI:       meta.CLI,
		State:     StateRunning,
		CreatedAt: meta.CreatedAt,
		ioFile:    ioFile,
		slave:     slave,
		pid:       meta.Pid,
		exitCode:  -1,
		watchCode: -1,
		exited:    make(chan struct{}),
	}
	go func() {
		code := <-codes
		sess.mu.Lock()
		sess.watchCode = code
		sess.mu.Unlock()
		close(sess.exited)
	}()
	b.registry.Add(sess)
	return sess, nil
}

// Forget drops an adopted session from the registry without signalling it:
// an import that failed after Adopt (IN-03). The caller closes its files.
func (b *NativePTYBackend) Forget(id string) {
	b.registry.Remove(id)
}

// HandoffFiles returns what a handoff sends for this session: the pollable
// master, the slave and the child's pid. The files stay owned by the session.
func (s *Session) HandoffFiles() (master, slave *os.File, pid int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slave = s.slave
	if slave == nil && s.pty != nil {
		if up, ok := s.pty.(gopty.UnixPty); ok {
			slave = up.Slave()
		}
	}
	if s.ioFile == nil || slave == nil || s.pid <= 0 {
		return nil, nil, 0, errors.New("session " + s.ID + ": no PTY files to hand off")
	}
	return s.ioFile, slave, s.pid, nil
}
