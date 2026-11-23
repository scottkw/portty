package pty

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/testutil"
)

func TestSpawnPTY(t *testing.T) {
	b := NewNativePTYBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := b.Create(ctx, CreateRequest{CLI: "cat", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer b.Kill(sess.ID) //nolint:errcheck

	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if sess.State != StateRunning {
		t.Errorf("expected StateRunning, got %v", sess.State)
	}
}

func TestSpawnPTY_RealPTY(t *testing.T) {
	b := NewNativePTYBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := b.Create(ctx, CreateRequest{CLI: "cat", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer b.Kill(sess.ID) //nolint:errcheck

	// cat echoes input — write "hello\n" and read it back
	if _, err := sess.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read back with timeout via goroutine
	type result struct {
		data string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 256)
		var sb strings.Builder
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			n, err := sess.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if strings.Contains(sb.String(), "hello") {
				ch <- result{data: sb.String()}
				return
			}
			if err != nil {
				ch <- result{data: sb.String(), err: err}
				return
			}
		}
		ch <- result{data: sb.String()}
	}()

	select {
	case r := <-ch:
		if !strings.Contains(r.data, "hello") {
			t.Errorf("expected 'hello' in PTY output, got: %q (err: %v)", r.data, r.err)
		}
	case <-time.After(4 * time.Second):
		t.Error("timed out waiting for PTY output")
	}
}

func TestSpawnPTY_EnvSet(t *testing.T) {
	b := NewNativePTYBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := b.Create(ctx, CreateRequest{CLI: "env", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer b.Kill(sess.ID) //nolint:errcheck

	// env prints all environment variables and exits — collect all output until
	// EOF or we find what we need.
	type result struct {
		data string
	}
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 4096)
		var sb strings.Builder
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			n, err := sess.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
				// Found it — no need to read more.
				if strings.Contains(sb.String(), "TERM=xterm-256color") {
					ch <- result{data: sb.String()}
					return
				}
			}
			if err != nil {
				// EOF or error — stop reading.
				ch <- result{data: sb.String()}
				return
			}
		}
		ch <- result{data: sb.String()}
	}()

	select {
	case r := <-ch:
		if !strings.Contains(r.data, "TERM=xterm-256color") {
			t.Errorf("expected TERM=xterm-256color in env output, got: %q", r.data)
		}
	case <-time.After(4 * time.Second):
		t.Error("timed out waiting for env output")
	}
}

// Phase 11 (PREV-01): every session carries its own id in PORTTY_SESSION_ID,
// never one inherited by the daemon (a daemon started from a Portty shell).
func TestSpawnPTY_EnvSessionID(t *testing.T) {
	t.Setenv("PORTTY_SESSION_ID", "stale")
	b := NewNativePTYBackend()
	envLine := func() (string, string) {
		sess, err := b.Create(context.Background(), CreateRequest{CLI: "/usr/bin/env", Cols: 80, Rows: 24, WorkDir: t.TempDir()})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		t.Cleanup(func() { _ = b.Kill(sess.ID) })
		var out bytes.Buffer
		var mu sync.Mutex
		go readUntilErr(sess, &out, &mu, make(chan error, 1))
		for deadline := time.Now().Add(4 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			mu.Lock()
			text := out.String()
			mu.Unlock()
			for _, line := range strings.Split(text, "\n") {
				if v, ok := strings.CutPrefix(strings.TrimSpace(line), "PORTTY_SESSION_ID="); ok && strings.HasSuffix(line, "\r") {
					return sess.ID, v
				}
			}
		}
		t.Fatalf("no complete PORTTY_SESSION_ID line in env output: %q", out.String())
		return "", ""
	}
	id1, got1 := envLine()
	id2, got2 := envLine()
	if got1 != id1 || got2 != id2 {
		t.Errorf("PORTTY_SESSION_ID = %q, %q; want the session ids %q, %q (never the inherited \"stale\")", got1, got2, id1, id2)
	}
	if id1 == id2 {
		t.Errorf("two sessions share id %q", id1)
	}
}

func TestResize(t *testing.T) {
	b := NewNativePTYBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := b.Create(ctx, CreateRequest{CLI: "cat", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer b.Kill(sess.ID) //nolint:errcheck

	if err := b.Resize(sess.ID, 120, 40); err != nil {
		t.Errorf("Resize failed: %v", err)
	}
}

func TestResize_NotFound(t *testing.T) {
	b := NewNativePTYBackend()
	err := b.Resize("nonexistent", 80, 24)
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestList(t *testing.T) {
	b := NewNativePTYBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess1, err := b.Create(ctx, CreateRequest{CLI: "cat", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Create 1 failed: %v", err)
	}
	defer b.Kill(sess1.ID) //nolint:errcheck

	sess2, err := b.Create(ctx, CreateRequest{CLI: "cat", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Create 2 failed: %v", err)
	}
	defer b.Kill(sess2.ID) //nolint:errcheck

	list := b.List()
	if len(list) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(list))
	}
}

func TestKillClean(t *testing.T) {
	b := NewNativePTYBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := b.Create(ctx, CreateRequest{CLI: "cat", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := b.Kill(sess.ID); err != nil {
		t.Fatalf("Kill failed: %v", err)
	}

	// After Kill the session state must be StateStopped.
	if sess.State != StateStopped {
		t.Errorf("expected StateStopped after Kill, got %v", sess.State)
	}

	// After Kill the session must not appear in List.
	list := b.List()
	for _, s := range list {
		if s.ID == sess.ID {
			t.Errorf("killed session %s still appears in List", sess.ID)
		}
	}
}

func TestKillClean_NoOrphans(t *testing.T) {
	b := NewNativePTYBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Spawn a shell that starts a background sleep and says so — killing the
	// session must also terminate the sleep (no orphan process).
	// We use a short sleep so even if cleanup fails the test doesn't hang long.
	sess, err := b.Create(ctx, CreateRequest{
		CLI:  "sh",
		Args: []string{"-c", "sleep 300 & echo portty-bg-started; cat"},
		Cols: 80,
		Rows: 24,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var out bytes.Buffer
	var mu sync.Mutex
	go readUntilErr(sess, &out, &mu, make(chan error, 1))

	// Wait for the shell to have started the sleep.
	testutil.WaitFor(t, 5*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return strings.Contains(out.String(), "portty-bg-started")
	}, "the shell never started the background job")

	if err := b.Kill(sess.ID); err != nil {
		t.Fatalf("Kill failed: %v", err)
	}

	// The spawned process group should be gone. We verify by checking that
	// sending signal 0 to the (negated) pgid returns an error — meaning the
	// process group no longer exists.
	// pgid == pid because go-pty uses Setsid.
	if sess.cmd != nil && sess.cmd.Process != nil {
		pgid := sess.cmd.Process.Pid
		testutil.WaitFor(t, 5*time.Second, func() bool { return killSignal0(-pgid) != nil },
			"process group %d still exists after Kill — orphan processes", pgid)
	}
}

func TestKillClean_NotFound(t *testing.T) {
	b := NewNativePTYBackend()
	err := b.Kill("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// Quick 261116-ku9 (R2): the child must see the requested size at startup.
// zsh reads COLUMNS once when it starts; if the PTY is still 0×0 (or 80×24)
// at that moment, its PROMPT_SP padding is generated for the wrong width.
func TestNativeCreate_InitialSizeVisibleToChild(t *testing.T) {
	b := NewNativePTYBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := b.Create(ctx, CreateRequest{CLI: "/bin/stty", Args: []string{"size"}, Cols: 123, Rows: 45, WorkDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer b.Kill(sess.ID) //nolint:errcheck

	ch := make(chan string, 1)
	go func() {
		buf := make([]byte, 256)
		var sb strings.Builder
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			n, err := sess.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if strings.Contains(sb.String(), "\n") || err != nil {
				break
			}
		}
		ch <- sb.String()
	}()

	select {
	case out := <-ch:
		if got := strings.TrimSpace(out); got != "45 123" {
			t.Errorf("child saw size %q, want \"45 123\"", got)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("timed out waiting for stty output")
	}
}

// startSh starts `sh -c script` in a session that the test kills on cleanup.
func startSh(t *testing.T, b *NativePTYBackend, script string) *Session {
	t.Helper()
	sess, err := b.Create(context.Background(), CreateRequest{CLI: "/bin/sh", Args: []string{"-c", script}, Cols: 80, Rows: 24, WorkDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = b.Kill(sess.ID) })
	return sess
}

// readUntilErr reads sess into out until Read fails, then sends the error.
func readUntilErr(sess *Session, out *bytes.Buffer, mu *sync.Mutex, done chan<- error) {
	buf := make([]byte, 4096)
	for {
		n, err := sess.Read(buf)
		mu.Lock()
		out.Write(buf[:n])
		mu.Unlock()
		if err != nil {
			done <- err
			return
		}
	}
}

// Phase 9 (UPGR-01, probe P3): a read deadline stops a blocked Read at once
// and no output is lost or repeated across the freeze.
func TestSession_FreezeIdleRead(t *testing.T) {
	b := NewNativePTYBackend()
	sess := startSh(t, b, `i=0; while [ $i -lt 30 ]; do echo tick $i; i=$((i+1)); sleep 0.05; done`)

	var out bytes.Buffer
	var mu sync.Mutex
	done := make(chan error, 1)
	go readUntilErr(sess, &out, &mu, done)

	time.Sleep(400 * time.Millisecond)
	start := time.Now()
	if err := sess.SetReadDeadline(time.Now()); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("frozen Read err = %v, want os.ErrDeadlineExceeded", err)
		}
		if d := time.Since(start); d > 200*time.Millisecond {
			t.Fatalf("freeze took %v, want <= 200ms", d)
		}
	case <-time.After(time.Second):
		t.Fatal("Read was not interrupted by the deadline")
	}

	time.Sleep(300 * time.Millisecond) // output piles up unread in the PTY
	if err := sess.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear deadline: %v", err)
	}
	go readUntilErr(sess, &out, &mu, done)
	select {
	case <-done: // EOF once the shell exits
	case <-time.After(5 * time.Second):
		t.Fatal("no EOF after resume")
	}

	mu.Lock()
	text := out.String()
	mu.Unlock()
	var ticks []int
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		var k int
		if _, err := fmt.Sscanf(line, "tick %d", &k); err == nil {
			ticks = append(ticks, k)
		}
	}
	if len(ticks) != 30 {
		t.Fatalf("got %d ticks, want 30: %v", len(ticks), ticks)
	}
	for i, k := range ticks {
		if k != i {
			t.Fatalf("tick %d at position %d (missing or repeated): %v", k, i, ticks)
		}
	}
}

// Phase 9 (Pitfall 6): a shell that prints nothing can still be frozen.
func TestSession_FreezeTrulyIdleShell(t *testing.T) {
	b := NewNativePTYBackend()
	sess := startSh(t, b, `sleep 5`)

	var out bytes.Buffer
	var mu sync.Mutex
	done := make(chan error, 1)
	go readUntilErr(sess, &out, &mu, done)
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	if err := sess.SetReadDeadline(time.Now()); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("Read err = %v, want os.ErrDeadlineExceeded", err)
		}
		if d := time.Since(start); d > 200*time.Millisecond {
			t.Fatalf("freeze took %v, want <= 200ms", d)
		}
	case <-time.After(time.Second):
		t.Fatal("idle Read was not interrupted by the deadline")
	}
}

// Phase 9 (Pitfall 2): with the master non-blocking, a paste bigger than the
// PTY input buffer must block until the child reads, never fail with EAGAIN.
func TestSession_LargeWriteBlocksNotEAGAIN(t *testing.T) {
	b := NewNativePTYBackend()
	sess := startSh(t, b, `stty -echo; sleep 1; cat > /dev/null`)

	go func() { _, _ = io.Copy(io.Discard, sess) }() // drain output until the PTY closes

	line := []byte(strings.Repeat("x", 62) + "\n")
	payload := bytes.Repeat(line, (1<<20)/len(line)+1)[:1<<20]

	start := time.Now()
	n, err := sess.Write(payload)
	took := time.Since(start)
	if errors.Is(err, syscall.EAGAIN) {
		t.Fatalf("Write surfaced EAGAIN after %d bytes: %v", n, err)
	}
	if err != nil || n != len(payload) {
		t.Fatalf("Write = (%d, %v), want (%d, nil)", n, err, len(payload))
	}
	if took < 800*time.Millisecond {
		t.Fatalf("Write took %v; it should block while the child sleeps", took)
	}
}

// Phase 9 (Pitfall 6): a session without the pollable dup fails fast.
func TestSession_NoDeadlineWithoutDup(t *testing.T) {
	s := &Session{ID: "no-dup"}
	if err := s.SetReadDeadline(time.Now()); !errors.Is(err, os.ErrNoDeadline) {
		t.Fatalf("SetReadDeadline = %v, want os.ErrNoDeadline", err)
	}
}
