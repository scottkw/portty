//go:build !windows

package daemon

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// TestCreateSession_NaturalExit_CapturesNonZeroExitCode is a regression test for
// the macOS exit-code-capture gap (CARD-08 error-exit). On unix, go-pty has NO
// waitOnContext goroutine, so the natural-exit path must call cmd.Wait() itself
// to populate ProcessState — otherwise the real exit code is lost and the engine
// reports 0, making the Hub's stopped-err state unreachable.
//
// A process that exits non-zero (here /usr/bin/false → 1) must surface its real
// exit code via both the onExit callback and ListSessions.ExitCode.
func TestCreateSession_NaturalExit_CapturesNonZeroExitCode(t *testing.T) {
	falsePath, err := exec.LookPath("false")
	if err != nil {
		t.Skip("false binary not available")
	}
	e := NewSessionEngine()
	e.SetSpawnCommandForTest(falsePath, nil) // exits 1
	done := make(chan int, 1)
	id, err := e.CreateSession(
		context.Background(),
		"exit-nonzero",
		"",
		0, 0,
		func(_ string, code int) { done <- code },
	)
	if err != nil {
		t.Fatalf("CreateSession(false): %v", err)
	}

	select {
	case code := <-done:
		if code != 1 {
			t.Errorf("onExit code = %d, want 1 (/usr/bin/false natural exit)", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for natural exit")
	}

	var got *int
	for _, s := range e.ListSessions() {
		if s.ID == id {
			got = s.ExitCode
			break
		}
	}
	if got == nil {
		t.Fatalf("ListSessions: ExitCode is nil for stopped session %s", id)
	}
	if *got != 1 {
		t.Errorf("ListSessions ExitCode = %d, want 1", *got)
	}
}

// TestDerivedStatus_ExitedErrored (STRP-01): status comes from process state —
// exited after code 0, errored after non-zero (running: TestListSessions_ShellStatusRunning).
func TestDerivedStatus_ExitedErrored(t *testing.T) {
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Skipf("sh not found: %v", err)
	}
	cases := []struct {
		script, want string
	}{
		{"exit 0", "exited"},
		{"exit 3", "errored"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			e := NewSessionEngine()
			e.configDir = t.TempDir()
			e.SetSpawnCommandForTest(shPath, []string{"-c", tc.script})
			done := make(chan struct{}, 1)
			id, err := e.CreateSession(context.Background(), "tab", "", 80, 24, func(string, int) { done <- struct{}{} })
			if err != nil {
				t.Fatalf("CreateSession: %v", err)
			}
			t.Cleanup(func() { _ = e.KillSession(id) })
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for exit")
			}
			if got := e.GetSessionStatus(id); got != tc.want {
				t.Errorf("GetSessionStatus = %q, want %q", got, tc.want)
			}
			for _, s := range e.ListSessions() {
				if s.ID == id && s.Status != tc.want {
					t.Errorf("ListSessions Status = %q, want %q", s.Status, tc.want)
				}
			}
		})
	}
}
