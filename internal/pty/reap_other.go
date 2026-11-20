package pty

import "time"

// ReapNaturalExit reaps a naturally-exited child (PTY master EOF already
// observed) and caches its real exit code from ProcessState.
//
// On Unix go-pty installs no waitOnContext goroutine and there is no exit
// detector (reap is driven by PTY master EOF). So nothing calls cmd.Wait() on
// the natural-exit path and ProcessState is never populated. The
// engine would then read a cached -1 and normalize it to 0 (D-10), making the
// Hub's CARD-08 stopped-err state unreachable for any non-zero natural exit.
//
// Safety: the child is already dead (EOF), so cmd.Wait() returns immediately
// without blocking. go-pty's wait() guards a double Wait ("Wait was already
// called"), and the kill path (killSession) is gated out by the engine's
// IsKilled() check before this runs — so this call races nothing. ProcessState
// is read in the same goroutine after Wait returns (happens-after; no race).
func (s *Session) ReapNaturalExit() {
	s.mu.Lock()
	cmd, exited := s.cmd, s.exited
	s.mu.Unlock()
	if cmd == nil {
		if exited != nil {
			// Adopted session (Phase 9): not our child, so no Wait. EOF can
			// arrive before the kqueue watcher fires; give it 500 ms, else
			// the code stays -1 (unknown).
			select {
			case <-exited:
				s.cacheWatchCode()
			case <-time.After(500 * time.Millisecond):
			}
		}
		return
	}
	_ = cmd.Wait()
	if cmd.ProcessState != nil {
		s.SetExitCode(cmd.ProcessState.ExitCode())
	}
}

// cacheWatchCode caches an adopted session's exit watcher result, if it fired.
func (s *Session) cacheWatchCode() {
	if s.exited == nil {
		return
	}
	select {
	case <-s.exited:
		s.mu.Lock()
		s.exitCode = s.watchCode
		s.mu.Unlock()
	default:
	}
}
