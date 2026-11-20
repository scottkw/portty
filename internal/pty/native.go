package pty

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	gopty "github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/unix"
)

// NativePTYBackend implements SessionBackend using the host OS PTY facilities.
// It uses Unix PTYs via go-pty.
type NativePTYBackend struct {
	registry *SessionRegistry
}

// NewNativePTYBackend returns a NativePTYBackend backed by a fresh SessionRegistry.
func NewNativePTYBackend() *NativePTYBackend {
	return &NativePTYBackend{
		registry: NewSessionRegistry(),
	}
}

// Create starts a new CLI process in a PTY and returns a Session handle.
// The process receives TERM=xterm-256color and COLORTERM=truecolor in addition
// to the caller's Env slice and the current process environment.
// On POSIX the process is placed in its own process group (Setpgid: true).
func (b *NativePTYBackend) Create(ctx context.Context, req CreateRequest) (*Session, error) {
	p, err := gopty.New()
	if err != nil {
		return nil, fmt.Errorf("open pty: %w", err)
	}

	// All session I/O goes through a pollable dup of the master (Phase 9,
	// probe P3): creack/pty opens it blocking, a deadline on a blocking file
	// is unsupported, and an idle shell never wakes a blocked read, so a
	// freeze could hang. Setting O_NONBLOCK also flips go-pty's own master
	// file (the flag lives on the shared open file description); that file
	// is not polled, so nothing may read or write through it any more or a
	// full input buffer surfaces EAGAIN (Pitfall 2). Resize still uses it:
	// its ioctl works in either mode.
	ioFile, err := pollableDup(p.(gopty.UnixPty).Master(), "ptmx")
	if err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("dup pty master: %w", err)
	}

	childCtx, cancel := context.WithCancel(ctx)
	cmd := p.CommandContext(childCtx, req.CLI, req.Args...)
	cmd.Dir = req.WorkDir

	// Merge environment: inherit from current process, apply caller extras, enforce
	// required terminal vars.
	cmd.Env = mergeEnv(os.Environ(), req.Env, "TERM=xterm-256color", "COLORTERM=truecolor")

	// On POSIX, go-pty's start() already sets Setsid: true which creates a new
	// session — the child's process group ID equals its PID. We do NOT set
	// Setpgid here because combining Setpgid+Setsid causes EPERM on macOS.
	// killSession uses -pid (negative) to kill the entire process group.

	// Set initial terminal dimensions BEFORE the shell starts (quick 261116-ku9):
	// zsh reads its size at startup and pads its PROMPT_SP mark to it, so a
	// late resize leaves an inverted "%" line. Resize is an ioctl on the master.
	if err := p.Resize(req.Cols, req.Rows); err != nil {
		cancel()
		_ = ioFile.Close()
		_ = p.Close()
		return nil, fmt.Errorf("initial resize: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		_ = ioFile.Close()
		_ = p.Close()
		return nil, fmt.Errorf("start process: %w", err)
	}

	id, err := generateID()
	if err != nil {
		cancel()
		_ = ioFile.Close()
		_ = p.Close()
		return nil, fmt.Errorf("generate session ID: %w", err)
	}

	sess := &Session{
		ID:        id,
		CLI:       req.CLI,
		State:     StateRunning,
		CreatedAt: time.Now(),
		pty:       p,
		ioFile:    ioFile,
		pid:       cmd.Process.Pid,
		cmd:       cmd,
		cancel:    cancel,
		exitCode:  -1,
	}

	b.registry.Add(sess)

	return sess, nil
}

// Resize changes the terminal dimensions for the given session.
func (b *NativePTYBackend) Resize(id string, cols, rows int) error {
	sess, ok := b.registry.Get(id)
	if !ok {
		return ErrSessionNotFound
	}
	sess.mu.Lock()
	p := sess.pty
	sess.mu.Unlock()
	if p == nil {
		return fmt.Errorf("session %s: PTY not initialised", id)
	}
	return p.Resize(cols, rows)
}

// Kill terminates the process for the given session and removes it from the
// registry.
func (b *NativePTYBackend) Kill(id string) error {
	sess, ok := b.registry.Get(id)
	if !ok {
		return ErrSessionNotFound
	}

	sess.MarkKilled()
	sess.SetState(StateStopped)

	b.registry.Remove(id)
	return killSession(sess)
}

// List returns all sessions currently tracked by this backend.
func (b *NativePTYBackend) List() []*Session {
	return b.registry.List()
}

// pollableDup returns a non-blocking, close-on-exec dup of f that the Go
// netpoller manages, so deadlines work on it. It reads the descriptor inside
// SyscallConn().Control, never through the Fd method, which switches it to
// blocking.
func pollableDup(f *os.File, name string) (*os.File, error) {
	sc, err := f.SyscallConn()
	if err != nil {
		return nil, err
	}
	d, derr := -1, error(nil)
	if err := sc.Control(func(fd uintptr) { d, derr = unix.FcntlInt(fd, unix.F_DUPFD_CLOEXEC, 0) }); err != nil {
		return nil, err
	}
	if derr != nil {
		return nil, derr
	}
	if err := unix.SetNonblock(d, true); err != nil {
		_ = unix.Close(d)
		return nil, err
	}
	return os.NewFile(uintptr(d), name), nil // non-blocking at NewFile time, so pollable
}

// generateID returns a cryptographically random 16-byte hex string suitable
// for use as a session identifier.
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// mergeEnv merges base and extra environment slices, then overrides/adds the
// required key=value pairs. Later entries win: extra overrides base, required
// overrides both.
//
// Algorithm: build a map keyed by variable name, iterate in order so later
// assignments win, then serialise back to []string.
func mergeEnv(base []string, extra []string, required ...string) []string {
	// Preserve insertion order by tracking key order.
	order := make([]string, 0, len(base)+len(extra)+len(required))
	vals := make(map[string]string, len(base)+len(extra)+len(required))

	add := func(entry string) {
		k, v, _ := strings.Cut(entry, "=")
		if _, exists := vals[k]; !exists {
			order = append(order, k)
		}
		vals[k] = v
	}

	for _, e := range base {
		add(e)
	}
	for _, e := range extra {
		add(e)
	}
	for _, e := range required {
		add(e)
	}

	out := make([]string, 0, len(order))
	for _, k := range order {
		out = append(out, k+"="+vals[k])
	}
	return out
}
