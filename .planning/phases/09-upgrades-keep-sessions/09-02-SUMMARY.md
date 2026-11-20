---
phase: 09-upgrades-keep-sessions
plan: 02
subsystem: pty, fdpass, webserver
tags: [handoff, scm_rights, kqueue, pty, nonblocking, tls-listener]
status: complete
requirements-completed: [UPGR-01, UPGR-04]
requires: []
provides:
  - "Session.SetReadDeadline / Pid / HandoffFiles"
  - "NativePTYBackend.Adopt + AdoptMeta"
  - "pty.WatchExit"
  - "internal/fdpass: Send, Recv, PeerUID, PeerPID"
  - "WebServer.Listener / StartOn, Config.APIVersion (api on /info)"
affects: [09-03 Hub.Freeze, 09-04 handoff protocol, 09-06 TESTING.md]
tech-stack:
  added: []
  patterns:
    - "pollable non-blocking dup of the PTY master, made inside SyscallConn().Control, never the Fd method"
    - "SCM_RIGHTS with a 4 KiB control buffer, so every fd the kernel delivers can be seen and closed"
    - "kqueue EVFILT_PROC exit watch as the only proof an adopted pid is alive"
key-files:
  created:
    - internal/fdpass/fdpass.go
    - internal/fdpass/fdpass_test.go
    - internal/pty/adopt_darwin.go
    - internal/pty/exitwatch_darwin.go
    - internal/pty/adopt_test.go
    - internal/webserver/listener_test.go
  modified:
    - go.mod
    - internal/pty/session.go
    - internal/pty/native.go
    - internal/pty/native_test.go
    - internal/pty/cleanup.go
    - internal/pty/reap_other.go
    - internal/webserver/server.go
decisions:
  - "fdpass.Recv uses a control buffer of at least 4 KiB, not CmsgSpace(nfds*4): macOS installs every passed fd even when the buffer is too small to list them, so a tight buffer leaks the extras. Measured: 254 fds per message accepted, 400 refused with EINVAL"
  - "All new dups (session I/O file, fdpass.Send) use F_DUPFD_CLOEXEC instead of unix.Dup, so a dup never leaks into a later child shell"
  - "Adopt wraps a pollable dup of the received master and closes the received copy, so the session's file is netpoller-managed whatever state the caller's *os.File was in"
  - "WatchExit falls back to polling kill(pid,0) if kqueue itself fails: a reused pid can only delay the report, never fire it early"
metrics:
  duration: "~30 min"
  completed: 2026-11-19
estimate:
  tokens: 110000
  tasks: 3
actuals:
  tokens: 13000
  tasks: 3
  commits: 6
plan_head_before: 3be899dd
plan_head_after: 97c43377724fc792f408a62b1556859ce59ab412
---

# Phase 9 Plan 02: Kernel Substrate for the Handoff Summary

Every session now reads and writes through a non-blocking, pollable dup of its PTY master. `internal/fdpass` moves fds over SCM_RIGHTS and reads the peer's uid and pid from the kernel. A session can be adopted from passed fds plus a pid, with a kqueue exit watcher and a kill that never signals a dead pid. The share server's TCP listener can be handed over, and `/info` reports `api`.

## Exported API added

| Symbol | Signature |
|---|---|
| Session.SetReadDeadline | `func (s *Session) SetReadDeadline(t time.Time) error` (returns `os.ErrNoDeadline` with no dup) |
| Session.Pid | `func (s *Session) Pid() int` |
| Session.HandoffFiles | `func (s *Session) HandoffFiles() (master, slave *os.File, pid int, err error)` |
| AdoptMeta | `type AdoptMeta struct { ID, CLI string; CreatedAt time.Time; Pid int }` |
| NativePTYBackend.Adopt | `func (b *NativePTYBackend) Adopt(meta AdoptMeta, master, slave *os.File) (*Session, error)` (on success it owns both files; on error the caller still does) |
| WatchExit | `func WatchExit(pid int) <-chan int` (fires once: the exit code, or -1 if unknown) |
| fdpass.Send | `func Send(c *net.UnixConn, header []byte, conns ...syscall.Conn) error` |
| fdpass.Recv | `func Recv(c *net.UnixConn, header []byte, nfds int) (n int, files []*os.File, err error)` |
| fdpass.PeerUID | `func PeerUID(c *net.UnixConn) (uint32, error)` |
| fdpass.PeerPID | `func PeerPID(c *net.UnixConn) (int, error)` |
| WebServer.Listener | `func (ws *WebServer) Listener() net.Listener` (the raw TCP listener; nil after Stop) |
| WebServer.StartOn | `func (ws *WebServer) StartOn(raw net.Listener) error` |
| Config.APIVersion | `APIVersion int` → `"api"` on `GET /api/sessions/{id}/info` (omitted when 0) |

No existing signature changed. `Start` is now `listenTCP` + `StartOn`. `assertTailnetBindIP` runs inside StartOn, so both start paths keep it.

## Test files added (for 09-06 / TESTING.md)

- `internal/fdpass/fdpass_test.go`: TestSendRecv_PipeFds, TestRecv_TruncatedClosesFds, TestRecv_CountMismatch, TestPeerUID, TestPeerPID, TestFlockFollowsPassedFd (A4), TestUnixListenerMovesWithoutUnlink (A5)
- `internal/pty/adopt_test.go` (darwin): TestAdopt_FromPassedFds, TestExitWatch_Orphan, TestExitWatch_AlreadyGone, TestExitWatch_NonChildKilled, TestAdopted_ReapNaturalExit, TestKillAdopted
- `internal/webserver/listener_test.go`: TestWebServer_StartOnPassedListener, TestSessionInfo_ReportsAPIVersion
- `internal/pty/native_test.go` (extended): TestSession_FreezeIdleRead, TestSession_FreezeTrulyIdleShell, TestSession_LargeWriteBlocksNotEAGAIN, TestSession_NoDeadlineWithoutDup

## Findings

- **TestExitWatch_Orphan saw 7** on this macOS (27.2): NOTE_EXITSTATUS returns the real exit code of a non-child, despite the man page's "Valid only on child processes". The -1 fallback is still in the code.
- **go.mod was updated with `GOFLAGS=-mod=mod GOPROXY=off go mod tidy`.** Its only change was moving `golang.org/x/sys v0.46.0` into the direct block. go.sum is unchanged against b4d63644.
- **The large-paste pin bites.** When Write was pointed at go-pty's master file as a mutation, the test failed with `write /dev/ptmx: resource temporarily unavailable` after 1022 bytes. Through the dup, 1 MiB blocks about 1.1 s and completes.
- **The Pitfall 9 pin bites.** With the `alive()` guard removed, TestKillAdopted fails with `Kill of an exited adopted session sent [hangup]`.
- **Per-message fd limit (research A3):** macOS accepts 254 fds in one SCM_RIGHTS message and refuses 400 with EINVAL. 09-04's per-session batches are far below that.
- **A tty session leader's exit blocks until its echoed output is read** (`ps` showed state `?Es`). The hub always reads, so production is unaffected. Tests that let a shell exit must drain its output.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] fdpass.Recv leaked fds on truncation**
- **Found during:** Task 2 (TestRecv_TruncatedClosesFds: 17 fds open after, 14 before)
- **Issue:** With a control buffer of `CmsgSpace(nfds*4)`, macOS still installs every passed fd in the receiver but lists only the ones that fit, so the extras cannot be found and closed.
- **Fix:** The control buffer is at least 4 KiB (`maxOOB`), which holds every fd the kernel accepts in one message (254 measured). Extras become a visible count mismatch, and all of them are closed. The MSG_CTRUNC check stays for anything larger.
- **Files modified:** internal/fdpass/fdpass.go
- **Commit:** a05b9839

**2. [Rule 2 - Missing critical] close-on-exec dups**
- **Issue:** The plan said `unix.Dup`, which leaves the dup inheritable. The session's I/O dup would then leak into every shell started later, which holds that session's master open.
- **Fix:** `unix.FcntlInt(fd, F_DUPFD_CLOEXEC, 0)` in `pollableDup` (pty) and in `fdpass.Send`. Received fds get `CloseOnExec` as planned. The remaining microsecond window has a `shortcut:` comment: darwin has no MSG_CMSG_CLOEXEC.
- **Commits:** 19064456, a05b9839

**3. [Rule 1 - Test race] TestKillAdopted setup**
- **Issue:** Kill landed before the shell had installed `trap "" HUP`, so SIGHUP alone killed it. Separately, the exited-session half hung because nothing read the echoed newline, and a tty session leader's exit waits for its output to drain.
- **Fix:** The test waits for an `echo ready` marker before Kill and runs a reader on the second session, as the hub does. The assertions are unchanged.
- **Commit:** 97c43377

**4. [Minor] Dup taken before `cmd.Start()`**
- The plan said after Start. Doing it before Start means a failure has no running child to clean up. Child behaviour is the same, since O_NONBLOCK on the master does not affect the slave.

## Known Stubs

None.

## Threat Flags

None. The surfaces are the ones in the plan's threat register: T-09-06 is covered by PeerUID/PeerPID, T-09-07 by Recv closing every fd, T-09-08 by the freeze and paste pins, T-09-09 by killAdopted, and T-09-10 by the bind check on both start paths, with csp_mw.go, capability_mw.go, origin_mw.go and tailnet/live.go unchanged against b4d63644.

## Verification

- `go build ./... && go vet ./...`: clean.
- Plan suite (run once): `go test -race -short -count=1 ./internal/pty/ ./internal/fdpass/ ./internal/webserver/` → `ok pty 8.305s`, `ok fdpass 1.020s`, `ok webserver 12.664s`.
- Grep gates: x/sys is direct and not indirect, go.sum is unchanged, `SetNonblock` and `SetReadDeadline` are present, there are 0 Fd method calls in native.go and session.go, `MSG_CTRUNC`, `LOCAL_PEERCRED`, `LOCAL_PEERPID` and `tls.NewListener` are present, `tls.Listen(` appears 0 times, `StartOn`, `Adopt`, `WatchExit` and `NOTE_EXITSTATUS` are present, `fdpass.Send` is in adopt_test.go, and the MaxInputPayload 32767 guard holds.

## Self-Check: PASSED

- All six created files exist, and all seven modified files are in the commits.
- Commits 37128a28, 19064456, 96f1bdf4, a05b9839, e08626c1 and 97c43377 are ancestors of HEAD.
