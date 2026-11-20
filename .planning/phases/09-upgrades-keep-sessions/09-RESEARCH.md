# Phase 9: Upgrades Keep Sessions - Research

**Researched:** 2026-11-19
**Domain:** Live daemon-to-daemon handoff of PTY sessions on macOS (fd passing, process lifetime, relay state transfer), plus API versioning and GUI quit hygiene
**Confidence:** HIGH for the mechanism (probed on this Mac), MEDIUM for the emulator re-seed fidelity (needs the property test proposed below)

<user_constraints>
## User Constraints (from CONTEXT.md)

No `09-CONTEXT.md` exists (discuss-phase has not run). No locked decisions, discretion areas or deferred ideas are recorded for this phase. The binding constraints come from ROADMAP.md, REQUIREMENTS.md, PROJECT.md and CLAUDE.md:

- Phase 9 depends on Phase 8: **the DEBT-08 consent guard stays as the fallback** (ROADMAP Phase 9 "Depends on").
- PROJECT.md Key Decision (Pending, v1.2 Phase 9): "Upgrades hand live sessions to the new daemon; DEBT-08 consent stays as the fallback".
- PROJECT.md Constraints: keep the Go/Wails/React stack ("a strip-down, not a rewrite"); macOS only; tailnet only, no new relay servers or cloud dependencies; "Keep token-gated, tailnet-only share links and the CSP intact"; Chesterton's Fence for daemon, relay and capability code.
- User memory: rock-solid priority (nobody can get to the remote Mac to fix it); radical simplicity; GUI checks run in the Tart VMs, never on the host.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UPGR-01 | Upgrade keeps every session alive; old daemon hands PTYs and session state (ring, history, offsets, share records) to the new daemon; no consent dialog on success | SCM_RIGHTS transfer of the PTY master/slave, listener and lock fds (probe P1, verified); pollable master so the old hub's read loop can be frozen without losing a byte (probe P3, verified); state transfer table (what moves, what is rebuilt); emulator re-seed from a rendered snapshot |
| UPGR-02 | Viewers reconnect by themselves and resume with a delta, nothing duplicated or missing | Ring bytes and `Scrollback.end` move byte-exact, so `?resume=` offsets stay valid; old daemon closes viewers with a retryable code, never `1000 "session ended"`; two viewer paths that do not reconnect today must gain it (local GUI tabs, local `portty attach`) |
| UPGR-03 | A failed handoff loses nothing; old daemon keeps serving; user told; DEBT-08 fallback; v1.1 daemon gets DEBT-08 | Two-phase commit where the old daemon keeps every fd and only stops reading PTYs; COMMIT is sent only after the new daemon reports READY; fault-injection seam at each step; `handoff` field absent in `/health` means "cannot hand off" |
| UPGR-04 | `/health` reports an API version; incompatible daemon shows "update needed" | `HealthResponse` gains `api` and `handoff` ints (absent reads as v1.0/v1.1); GUI banner and CLI one-liner with non-zero exit |
| UPGR-05 | GUI quits on SIGTERM; Quit All cancels an in-flight restart; Cmd-Q never stacks under the restart dialog | Root cause of SIGTERM found in Wails v2.10.2 (signal handler routes to OnBeforeClose, which Portty vetoes); `signal.Reset` plus Portty's own handler; `ctx` on EnsureDaemon; modal ordering in App.tsx |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Go: `go fmt`, `golangci-lint`, context-aware functions (`ctx context.Context`). Use `GOTOOLCHAIN=go1.26.11` for every go command (TESTING.md toolchain note; root package refuses Go 1.27).
- Chesterton's Fence before removing anything; "Silent Fallbacks: `or {}` ... Let it crash" (applies to state import: a malformed handoff blob must abort the handoff, not import a half state).
- RULE 0: for catastrophic or unknown failures, stop. (Design implication: the handoff aborts toward the old daemon on any unexpected condition.)
- Testing: 80%+ coverage in critical components; integration tests for daemon routes.
- `TESTING.md` standing rule (repo CLAUDE.md): every new test file goes into Section 2 (manifest) and Section 4 (traceability, repo-relative path only, run `bash tests/check-traceability-paths.sh`); every non-automatable behaviour gets an M-NN item in Section 5.
- GSD workflow: edits only through GSD commands. Ponytail runs on executor agents; keep the smallest change that works.
- User instruction for this research: do not run full test suites (each test binary triggers a firewall prompt) and never touch the host's running Portty. The same applies to execution: VM for GUI and two-build checks.

## Summary

Today a version-mismatch restart sends `POST /shutdown`, and `handleShutdown` calls `os.Exit(0)` 50 ms later (internal/daemon/api.go:281-290). The PTY master and slave close with the process, so every shell gets a hangup. Nothing survives, which is what U-01 recorded in 08-VM-CHECKS.md.

The fix is a two-phase handoff between daemons over the existing 0600 unix socket. The new binary's EnsureDaemon starts `portty daemon --takeover` (proposed flag). That process connects to the old daemon and asks for its sessions. The old daemon freezes its hub read loops (each master is opened non-blocking and pollable, so a read deadline stops a blocked `Read` at once and loses no byte) and serializes the session state. It then sends the state plus the fds: PTY master and slave per session, the unix API listener, the relay TCP listener, the share TLS listener's TCP socket and the daemon.lock fd, all with `SCM_RIGHTS`. The new daemon imports into a staged state that serves nothing and reads nothing, then answers READY. Only then does the old daemon send COMMIT. The new daemon starts serving and reading and answers COMMITTED, and the old daemon closes its fd copies without unlinking the socket, kicks its viewers with a retryable close code and exits without touching the PTYs. Until COMMIT, the old daemon holds every fd and can thaw, so a failure at any step leaves it serving (UPGR-03).

I probed the risky kernel behaviour on this Mac (macOS 27.2.1, build 26B131):
- **P1 (SCM_RIGHTS):** a PTY master sent to an unrelated process keeps the shell alive after the sender exits without reaping. The shell gets no SIGHUP, is reparented to launchd (`ppid 1`), loses no buffered output, and accepts input from the receiver. The receiver sees EOF when the shell exits.
- **P2 (self-exec):** also works.
- **P3 (pollable master):** a non-blocking dup of the master gives a pollable `*os.File` whose `SetReadDeadline` interrupts an idle `Read` with no data loss.
- **Exit status of a non-child:** `kqueue` with `EVFILT_PROC` and `NOTE_EXIT|NOTE_EXITSTATUS` registered on a shell that was never the receiver's child and returned its real exit status (`code=7`). The local man page says NOTE_EXITSTATUS is "Valid only on child processes", so treat it as best-effort with a fallback to "unknown" (the code already normalises -1 to 0).

The vt emulator cannot be serialized and cannot be rebuilt from the ring alone. The ring wraps at 256 KiB, and the emulator is fed from byte 1 precisely because a full-screen app's one-time header writes leave the ring (M-51). It is rebuilt from a **rendered seed**, the same technique `rebuildAfterAltExitLocked` already uses in-process. The seed is extended to carry the main screen behind an alternate-screen app. Everything else in the hub is plain data and moves verbatim: ring bytes and offsets, History lines and wrap flags, modes, wrapSet, charState, geomEnd, mainRegion and the grid. Viewers do not depend on the seed at all, because their resume is a raw ring delta.

**Primary recommendation:** an SCM_RIGHTS two-phase handoff, pulled by a `daemon --takeover` process that the new build's EnsureDaemon spawns, with the old daemon as the only party that can roll back. Do not use self-exec: it cannot keep the old daemon serving if the new image fails after `execve` (UPGR-03).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Decide to hand off vs DEBT-08 | GUI/CLI process (EnsureDaemon, internal/daemon/process.go) | — | It already detects the mismatch and owns the consent policy; it spawns the takeover daemon like it spawns daemons today |
| Freeze, export, rollback, commit | Old daemon (API + relay hubs) | — | Only the old daemon owns the PTYs until COMMIT; it alone may resume reading |
| Import, staged start, serve after COMMIT | New daemon (`daemon --takeover`) | — | Must not read a PTY or accept a connection before COMMIT |
| PTY fd lifetime, hangup semantics, exit status | Kernel (pty, kqueue) | PTY package (internal/pty) | The master stays open while any process holds it; the in-flight SCM_RIGHTS message holds a reference too |
| Ring, History, emulator seed | Relay hub (internal/relay) | — | Hub-owned state; export and import belong next to the code that defines it |
| Share records, grants, capability key | Daemon API (share.go) + disk (capability.key) | Webserver (grants) | The key is already on disk; records and grants are memory-only and must move |
| Viewer reconnect and resume | Frontend RelayClient, CLI attach loop, browser share viewer | Peer daemon's ws proxy (passes close codes through) | All viewers already speak `?resume=`; two local paths lack reconnect |
| API version and "update needed" | Daemon `/health` | GUI banner, CLI message | Client-side check, server-side advertisement |
| SIGTERM quit, Quit All cancel, modal stacking | GUI Go (app.go, main.go) + frontend (App.tsx) | Wails | Wails installs the SIGTERM handler that causes the bug |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net` (`*net.UnixConn.WriteMsgUnix` / `ReadMsgUnix`, `net.FileListener`) | Go 1.26.11 toolchain | Send fds over the unix socket; rebuild listeners from received fds | `net.FileListener` is how go-systemd socket activation and tableflip rebuild listeners. [VERIFIED: probe P1 compiled and ran against go1.26.11] |
| `golang.org/x/sys/unix` | v0.46.0 (already in go.sum as `// indirect`; becomes a direct import) | `UnixRights`, `ParseSocketControlMessage`, `ParseUnixRights`, `Kqueue`/`Kevent` with `EVFILT_PROC`, `NOTE_EXIT`, `NOTE_EXITSTATUS`, `SetNonblock`, `Dup`, `GetsockoptXucred(SOL_LOCAL, LOCAL_PEERCRED)` | Already in the module graph via golang.org/x/term and go-pty. No new download. [VERIFIED: /Users/ken/go/pkg/mod/golang.org/x/sys@v0.46.0/unix/sockcmsg_unix.go:39,78,93; zerrors_darwin_arm64.go:448 `EVFILT_PROC = -0x5`, :1069 `NOTE_EXIT = 0x80000000`, :1070 `NOTE_EXITSTATUS = 0x4000000`; syscall_darwin.go:488 `func GetsockoptXucred(fd, level, opt int) (*Xucred, error)`] |
| `encoding/gob` (stdlib) | Go 1.26.11 | Encode the versioned state blob | Binary-safe for ring bytes and ANSI strings; matches fields by name, so adding a field (Phase 12 will add OSC 7 cwd) does not break an older reader. JSON would escape every ESC as `\u001b`. [ASSUMED: choice; encoding/json also works] |
| `github.com/coder/websocket` | v1.8.15 (existing) | `StatusServiceRestart` close code for viewers at commit | Already the WS library. [VERIFIED: close.go:51 `StatusServiceRestart          StatusCode = 1012`] |

### Supporting (existing, unchanged)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/aymanbagabas/go-pty` | v0.2.3 | Spawns new sessions (unchanged) | Only for `Create`; adopted sessions never touch it |
| `github.com/charmbracelet/x/vt` | v0.0.0-20260615092313-b57e5e6d29bb | Live emulator; the new daemon seeds a fresh one | Version is pinned by `TestVTLayout_AltScreenScrollback` (internal/relay/vtalt_test.go:20) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| SCM_RIGHTS pull by a takeover daemon | Self-exec (old daemon `syscall.Exec`s the new binary keeping fds) | Simpler fd story, keeps the PID so `wait4` and exit status work natively (probe P2 verified). **Rejected:** after `execve` succeeds the old image is gone, so an init or import failure in the new image ends every session, which violates UPGR-03. Also the old daemon would exec whatever is at its original path, not the caller's binary |
| SCM_RIGHTS | nginx-style fork+exec with inherited fds (`exec.Cmd.ExtraFiles`), old daemon spawns new | Keeps rollback (old stays alive). **Rejected:** the old daemon would choose the binary (its `os.Executable()` path) rather than the build that asked. It makes the new daemon a child of the old (no benefit: shells still reparent to launchd). It duplicates the spawn logic EnsureDaemon already owns |
| Handoff | Per-session holder processes (abduco/dtach model: a tiny process per PTY that never upgrades) | Removes the need for handoff entirely, but it is a re-architecture of daemon, pty and relay ownership. Out of bounds ("strip-down, not a rewrite") |
| Pass listener fds | New daemon binds fresh sockets | Binding fresh forces an unlink and rebind of daemon.sock (tripping both daemons' `socketLost` watchers), a new relay port, and an EADDRINUSE race on the fixed share port 7444. Passing three more fds avoids all three |

**Installation:** none. `golang.org/x/sys` moves from indirect to direct with `go mod tidy` (no version change).

## Package Legitimacy Audit

No new external packages are introduced. The only change is promoting `golang.org/x/sys` v0.46.0, already resolved in go.sum and pulled by golang.org/x/term and go-pty (`go mod why -m golang.org/x/sys` → `github.com/scottkw/portty` → `golang.org/x/term`), from indirect to direct. The gsd package-legitimacy seam covers npm/PyPI/crates, not Go modules, so it was not run.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| golang.org/x/sys | Go module proxy | Go project sub-repo, years | n/a | go.googlesource.com/sys | OK (existing dependency) | Approved: direct import of an existing dep |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
 user installs build B over A; launches B GUI (or runs B CLI)
                 |
                 v
   EnsureDaemon (B)  --GET /health-->  old daemon A  {version:A, api:2, handoff:1}
                 |                                         ^
   mismatch && live sessions && handoff supported          |
                 |                                         |
   spawn `B daemon --takeover` (Setsid, daemon.log)        |
                 |                                         |
                 v                                         |
   takeover daemon B --POST /handoff (hijacked conn)-----> A: check peer uid (LOCAL_PEERCRED),
                                                           |   versions, one handoff at a time
                 <------------- HELLO ok ------------------+
                                                           A: FREEZE
                                                           |  - mutating routes answer 503 Retry-After
                                                           |  - each hub: SetReadDeadline(now) -> Run exits
                                                           |    (no Shutdown, no Done, no byte read past it)
                                                           |  - export per-session state + seed
                 <-- STATE blob (gob) + SCM_RIGHTS fds ----+  fds: lock, api.sock ln, relay ln, share ln,
                 |                                             master+slave per session
   B: import into STAGED engine (no Serve, no Run)         |
      verify, build hubs, seed emulators, register exits   |
      [fail -> ABORT -> exit 1] ---------------------------> A: THAW (clear deadline, go hub.Run) -> serving
                 |                                         |       EnsureDaemon(B) sees takeover exit + A still
                 |                                         |       answering -> "could not be moved" -> DEBT-08
                 +---------------- READY ----------------->|
                 <--------------- COMMIT ------------------+ (point after which A never reads a PTY again)
   B: http.Serve on all listeners, go hub.Run,             |
      kqueue exit watchers, daemon.log, socketLost watch    |
                 +--------------- COMMITTED -------------->| A: close listener copies (SetUnlinkOnClose(false)),
                                                           |    kick viewers with 1012 "service restart",
                                                           |    os.Exit(0) without api.Stop / Hub.Shutdown
                 |
   EnsureDaemon (B): daemonReady -> /health version B -> return nil
                 |
   viewers (local tabs, peer tabs via home daemon proxy, browser, `portty attach`)
     non-terminal close -> backoff 1 s +/-20 % -> beforeReconnect -> redial ?resume=<offset>
     -> B's hub.Join: ReplayDelta from the byte-exact transferred ring
```

### Recommended file layout (new and touched)

```
internal/pty/
├── session.go        # Session gains pid (always set) and an io file used for ALL Read/Write
├── native.go         # Create: dup master non-blocking -> pollable *os.File (P3)
├── adopt_darwin.go   # NEW: AdoptSession(id, pid, master, slave *os.File, meta) for received fds
├── exitwatch_darwin.go # NEW: kqueue EVFILT_PROC NOTE_EXIT|NOTE_EXITSTATUS per adopted pid
└── cleanup.go        # killSession: pid path for adopted sessions (no exec.Cmd)
internal/relay/
├── hub.go            # Freeze()/Thaw(); Run distinguishes "frozen" from EOF; CreateStaged
├── handoff_state.go  # NEW: HubState export/import (ring, history, modes, wraps, cs, grid, seed)
└── vtalt.go          # unchanged unless the seed needs the main screen read-only (see Pitfall 7)
internal/daemon/
├── handoff.go        # NEW: /handoff route (old side) + takeover client (new side), state machine
├── handoff_fds.go    # NEW: SCM_RIGHTS send/recv helpers, MSG_CTRUNC check, peer-cred check
├── process.go        # EnsureDaemon(ctx, ...): handoff first, then RestartPolicy; RunTakeover
├── api.go            # freeze gate on mutating routes; HealthResponse api/handoff
├── share.go          # export/import share records + grants; start share server on a passed listener
└── types.go          # HealthResponse fields
internal/webserver/server.go   # listen = net.Listen + tls.NewListener so the raw TCP fd can move
app.go, main.go, cmd_cli.go, cmd_attach.go, cmd_daemon.go
frontend/src/App.tsx, frontend/src/components/TerminalPanel.tsx
```

### Pattern 1: What moves and what is rebuilt (the core decision)

Each entry below was read in the source this session.

| State | Where | Decision | Why |
|-------|-------|----------|-----|
| PTY master fd | go-pty `unixPty.master` (pty_unix.go:17-20) | **Transfer** (SCM_RIGHTS) | It is the session. P1 shows that the shell survives and buffered bytes are kept |
| PTY slave fd held by the daemon | go-pty `unixPty.slave` (pty_unix.go:17-20, opened by creack/pty pty_darwin.go) | **Transfer** (keeps today's semantics) | The daemon holds the slave today. On macOS the master still reaches EOF when the session leader exits (P1 and P3 got EOF while holding the slave), so holding it is harmless and keeps parity |
| Child pid | `cmd.Process.Pid` (cleanup.go:30) | **Transfer** | Needed for kill (`-pgid`, go-pty sets `Setsid`/`Setctty`, cmd_unix.go:45-46) and exit watch |
| exec.Cmd / go-pty Cmd | session.go:43 | **Cannot transfer; rebuild as pid-only** | The new daemon is not the parent: `wait4` returns ECHILD (P1). Exit status comes from kqueue instead |
| Exit watcher goroutine | engine.go:273-297 (`<-hub.Done()` then `ReapNaturalExit`) | **Rebuild** | Re-armed per adopted session; EOF still drives it. ReapNaturalExit for adopted sessions reads the kqueue status (or -1) |
| Scrollback ring | `Scrollback{buf, max, truncated, end}` (scrollback.go:24-30) | **Transfer verbatim** | `end` is the resume offset space. Byte-exact transfer is what keeps every viewer's `?resume=` valid (UPGR-02) |
| History | `History{lines, wrapped, head, bytes, maxLines, maxBytes}` (history.go:21-28) | **Transfer verbatim** (retained lines + wrap flags) | Rendered lines are not derivable from the ring once it wraps |
| Live vt emulator | `Hub.liveEmu` (hub.go:99-100) | **Rebuild from a rendered seed** | No serializer exists, and replaying the ring is wrong once it wraps or after a genuine resize (the reason for the M-51 eager build, hub.go:568-582) |
| modes, geomEnd, altExited, mainRegion, wraps, trackWraps, lastMove, cs | hub.go:102-143 | **Transfer** (plain data; trackWraps and altExited are transient false) | All "Hub-owned so it survives rebuilds" per their comments; the seed relies on them |
| Grid `ptyCols/ptyRows` | hub.go:80-85 | **Transfer** | Guests get the 0x02 grid on join; keeps `Cols()`/`Rows()` and the seed geometry right |
| Subscribers, nextConnID | hub.go:74-75 | **Rebuild** (empty) | Viewers reconnect; IDs are diagnostics only |
| Session names, workDir, createdAt, CLI | engine.go:36-37, session.go:28-37 | **Transfer** | Display, ordering, duration |
| Share records `{grantID, token}` | share.go:23-28 | **Transfer** | Memory-only; the token must stay the same (success criterion 1) |
| Webserver grants | webserver/server.go:84-87, 224-233 | **Rebuild from transferred records** (`AddGrant`) | As `startShareServer` already does on start (share.go:259-262) |
| Capability signing key | `capability.key` on disk (keystore.go:32-33 `return &FileKeyStore{path: filepath.Join(dir, "capability.key")}`) | **Reload from disk** (`BootstrapCapabilityState`) | Already persistent; the same key verifies the same tokens |
| `ct_disclosed`, settings.json | config dir (share.go:189; engine.go:89-92) | **Reload from disk at import** (not at process start) | A settings PATCH is gated during freeze, so the disk is current |
| RemoteCapStore (viewer-side caps for other Macs), tailnetCache | api.go:75-76 | **Rebuild** | The existing peer-tab `beforeReconnect` (`ConnectPeerSession`) and CLI `remoteReauth` refill them (App.tsx:890-899, cmd_attach.go:166-187) |
| Hijacked viewer WebSockets, peer proxy upstreams | per-connection fds | **Do not transfer; close** | Clients reconnect with resume; no protocol needs connection migration |
| tsnet node state | — | **Not applicable** | Portty does not import `tailscale.com/tsnet` (repo-wide grep empty). The share server is a TLS listener bound to the system tailscaled's tailnet IP on `const DefaultProbePort = 7444` (internal/tailnet/tailnet.go:29), with certs from LocalAPI (webserver/server.go:337-366). PROJECT.md's "tsnet" mention is stale |

### Pattern 2: Pollable PTY master (prerequisite for a lossless freeze)

creack/pty opens the master blocking (`pFD, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_CLOEXEC, 0)`, creack/pty@v1.1.24/pty_darwin.go:14). A blocked `Read` on such a file can be interrupted neither by a deadline (`os.ErrNoDeadline`) nor reliably by `Close`, and an idle shell may never produce the byte that would wake it. Probe P3 shows the fix:

```go
// Source: probe P3 (scratchpad nbprobe/main.go), verified on macOS 27.2.1
m := p.(gopty.UnixPty).Master()
var raw int
sc, _ := m.SyscallConn()
_ = sc.Control(func(fd uintptr) { raw = int(fd) }) // never m.Fd(): Fd() forces blocking mode
d, _ := unix.Dup(raw)
_ = unix.SetNonblock(d, true)            // O_NONBLOCK is on the open file description: shared by every fd to it
io := os.NewFile(uintptr(d), "ptmx")     // non-blocking at NewFile time -> pollable (netpoller/kqueue)
// freeze: io.SetReadDeadline(time.Now()) -> Read returns os.ErrDeadlineExceeded at once; unread bytes stay in the PTY
```

Probe P3 output (verbatim): `read loop ended after 1.002s: deadline=true err=read ptmx-nb: i/o timeout` / `second loop end err: EOF` / `all tokens=[tick 0 tick 1 tick 2 tick 3 tick 4 tick 5]`. The freeze landed while the shell was idle, every later tick was read after the deadline was cleared, and EOF still arrived when the shell exited.

**All Read AND Write must go through the pollable file** (Pitfall 2). Resize keeps using go-pty (`SetWinsize` goes through `SyscallConn().Control`, pty_unix.go:89-98, which is mode-agnostic).

### Pattern 3: SCM_RIGHTS transfer (verified)

```go
// Source: probe P1 (scratchpad fdprobe/main.go), verified on macOS 27.2.1, run 4 times
// sender (old daemon), on the hijacked *net.UnixConn
oob := unix.UnixRights(lockFD, apiLnFD, relayLnFD, shareLnFD, masterFD, slaveFD /* ... */)
_, _, err := conn.WriteMsgUnix(headerBytes, oob, nil)

// receiver (takeover daemon)
oob := make([]byte, unix.CmsgSpace(nFDs*4))
n, oobn, flags, _, err := conn.ReadMsgUnix(buf, oob)
if flags&unix.MSG_CTRUNC != 0 { /* abort: fds were dropped by the kernel */ }
msgs, _ := unix.ParseSocketControlMessage(oob[:oobn])
fds, _ := unix.ParseUnixRights(&msgs[0])
```

Probe P1 output (verbatim, receiver log):
```
receiver pid=9133 flags=0x0 MSG_CTRUNC=false oobn=20
receiver got fds=[5 6] shell pid=9121 parent last="tick 9"
kevent registered NOTE_EXIT|NOTE_EXITSTATUS on non-child pid 9121
shell ppid,stat after parent exit: "1 Ss+"
receiver line: "hello"
receiver line: "got:hello"
receiver first line="tick 10" last tick="tick 39" gap=false read end err=<nil>
kevent fired: fflags=0x84000000 NOTE_EXIT=true data=1792 (as WaitStatus: exited=true code=7)
wait4(non-child): err=no child processes
kill(pid,0) after exit: no such process
```
The shell's `trap 'echo GOT-HUP; exit 9' HUP` never fired, so no hangup reached it when the sender closed its copies and exited without `Wait`.

Send fds in batches with a fixed count per message (e.g. per session: `[master, slave]` plus a small header naming the session id) and size `oob` exactly. A per-message fd limit on macOS was not measured [ASSUMED risk, see A3]. Every received fd must be closed on abort.

### Pattern 4: Exit status of a shell the daemon did not fork

```go
// Source: probe P1; x/sys v0.46.0 constants quoted above
kq, _ := unix.Kqueue()
ev := []unix.Kevent_t{{Ident: uint64(pid), Filter: unix.EVFILT_PROC,
    Flags: unix.EV_ADD | unix.EV_ONESHOT, Fflags: unix.NOTE_EXIT | unix.NOTE_EXITSTATUS}}
if _, err := unix.Kevent(kq, ev, nil, nil); err != nil {
    // EACCES: retry with NOTE_EXIT only (status unknown -> -1 -> existing D-10 normalisation to 0)
    // ESRCH: already gone (status unknown)
}
// on event: syscall.WaitStatus(out[0].Data).ExitStatus() / .Signaled()
```
`man 2 kqueue` on this Mac (verbatim): "NOTE_EXITSTATUS The process has exited and its exit status is in filter specific data. Valid only on child processes and to be used along with NOTE_EXIT." Apple's archived man page adds: "If a process can normally see another process, it can attach an event to it." [CITED: developer.apple.com/library/archive/.../kqueue.2.html]. Observed behaviour contradicts the "child only" sentence. Keep the EACCES fallback and pin the behaviour with a unit test that makes a real orphan (Test Strategy, T-P3).

### Pattern 5: Freeze and thaw in Hub.Run without signalling exit

`Run` defers `h.Shutdown()` (hub.go:506-507), and `Shutdown` closes `done` (hub.go:1162-1169). Both viewer write pumps turn `hub.Done()` into the terminal close `1000 "session ended"` (relay/server.go:198-205; webserver/server.go:957-964), and the engine's exit goroutine treats it as a natural exit (engine.go:273-297). So a freeze must end `Run` **without** `Shutdown`:

```go
// proposed shape
func (h *Hub) Run() {
    for {
        n, err := h.reader.Read(buf)
        if n > 0 { /* unchanged: emuMu, recordFrameLocked, broadcast */ }
        if err != nil {
            if h.freezing.Load() && errors.Is(err, os.ErrDeadlineExceeded) {
                close(h.frozen) // Freeze() waits on this; Done stays open
                return
            }
            h.Shutdown()
            return
        }
    }
}
```
`Freeze` sets `freezing`, calls the reader's `SetReadDeadline(time.Now())` and waits on `frozen`. `Thaw` clears the deadline and the flag, makes a fresh `frozen` channel and starts `go h.Run()` again. Export runs after `frozen` closes, under `emuMu`.

### Pattern 6: Emulator re-seed

The seed is a byte string fed to a fresh `newLiveEmulator(cols, rows)` (hub.go:847, the only constructor). Build it like `rebuildAfterAltExitLocked` (hub.go:713-829), which already seeds a new emulator with mode restores, the main-screen body, the region, the saved cursor and pen, the pending-wrap state, the current pen, the charsets and the link (from probe cells). The handoff seed differs in two ways:
1. It is serialized and fed in another process. Refactor the existing rebuild into `seedLocked() []byte` + `applySeedLocked(seed)`. The in-process alt-exit rebuild then becomes export+import, and every existing alt-exit test (`TestAltScreenExit_RebuildMatchesReference`, `TestAltScreenExit_RebuildFuzz`, 600 fuzz streams) covers the handoff seed for free.
2. When the session is in the alternate screen (vim, Claude Code full-screen), the seed must carry both screens: the main-screen seed, then `\x1b[?1049h` (vt saves the cursor and enters a cleared alt screen), then the alt rows, alt cursor and pen. To read the main screen, use the trick `resizeLiveEmulator` already relies on: write `\x1b[?1049l` to the old (to-be-discarded) emulator (hub.go:1110-1113). This mutates the old emulator. On rollback the old hub therefore **re-imports its own export**, which makes the rollback path the same code as the import path (Pitfall 7).

Known, accepted seed losses are the ones already listed in the rebuild's `shortcut:` comment (hub.go:709-712): tab stops, G2/G3 and single shifts, the UK set, the saved cursor's visibility, zero-width cells. They affect only late joiners' full renders and future history lines, not viewers who resume with a delta.

### Pattern 7: Listener handover without unlink

- Old side: the unix listener was created by `listenDaemonSocket`, and Go unlinks the path on `Close` for listeners it created. Before closing its copy, the old daemon must call `SetUnlinkOnClose(false)`, exactly as the lost-socket branch does (process.go:98-104). It must never call `api.Stop()`, which runs `removeDaemonSocket` (api.go:236-261).
- New side: `net.FileListener(os.NewFile(fd, ...))` for all three. A unix listener rebuilt this way does not unlink on close by default [ASSUMED: Go net internals; harmless either way because api.Stop unlinks explicitly]. `socketLost` (process.go:297-311) keeps working because the path still names the same inode (`os.SameFile`).
- Share listener: `listenTLS` uses `tls.Listen` (webserver/server.go:375-385). Change it to `net.Listen` + `tls.NewListener(ln, tlsCfg)` so the raw TCP fd can be exported, and add a start-on-listener path for import.
- Lock: send the daemon.lock fd (process.go:42 `lock, err := lockDaemon(filepath.Join(filepath.Dir(socketPath), "daemon.lock"), 2*time.Second)`). flock belongs to the open file description, so the new daemon holds the same lock the moment it receives the fd, and `lockHeld` (process_unix.go:43-50) stays true across the whole handoff. If the new daemon dies, the old still holds it; if the old exits after COMMITTED, the new still holds it. [ASSUMED: flock(2) OFD semantics on macOS; standard BSD behaviour, cheap to pin in a unit test]

### Pattern 8: Commit protocol and rollback rules (UPGR-03)

| Step | Who | On failure |
|------|-----|-----------|
| 0. EnsureDaemon sees `handoff >= 1` on old, spawns `exe daemon --takeover` | caller | spawn error → DEBT-08 policy with reason "could not be moved" |
| 1. HELLO {handoffVersion, buildVersion, pid}; old checks `LOCAL_PEERCRED` uid == own uid, versions compatible, no handoff in progress (else 409) | old | refuse → takeover exits non-zero |
| 2. FREEZE: gate mutating routes (503 + `Retry-After: 1`), `Freeze()` each running hub | old | thaw everything, close conn |
| 3. STATE + fds | old → new | send error → thaw |
| 4. Import into a staged engine (no Serve, no Run) | new | ABORT + close received fds + exit 1; old sees ABORT/EOF → thaw |
| 5. READY {per-session render digest} | new → old | timeout (proposed 5 s) → old thaws, closes conn; new sees EOF before COMMIT → exits without reading |
| 6. COMMIT | old → new | — (after this point old never resumes reading on a timeout) |
| 7. Serve + `go hub.Run` + exit watchers, COMMITTED | new | new dies before COMMITTED: old sees EOF on the handoff conn → old **kills nothing (new is dead)**, re-imports its own export, thaws, keeps serving; any bytes the new daemon read in that window are lost (logged) |
| 8. Close copies (no unlink), kick viewers 1012, `os.Exit(0)` | old | — |

Extra rule: if the new daemon sees EOF before COMMIT **and** the old pid is gone (check `kill(pid, 0)` for up to ~500 ms), it proceeds as committed, because it holds every fd and a dead old daemon cannot read. This turns "old daemon crashed mid-handoff" into a save instead of a loss.

Only one process ever reads a PTY at a time: the old daemon until FREEZE, nobody until COMMIT, the new daemon after COMMIT. That is the invariant every fault-injection test asserts.

### Pattern 9: EnsureDaemon integration

```
EnsureDaemon(ctx, socketPath, policy):
  health -> {version, api, handoff}
  if StaleVersion(version):
      n := liveSessions()
      if n != 0 && handoff >= 1 && supported(handoff) && !keepDaemon:
          err := takeover(ctx)               // spawn `daemon --takeover`, wait daemonReady or exit
          if err == nil { return nil }       // success: no dialog (UPGR-01)
          reason = HandoffFailed             // then fall through to DEBT-08
      if n != 0 && !policy(Question{version, n, reason}) { return nil }   // DEBT-08 unchanged
      ... existing recount, /shutdown, spawn
```
- `RestartPolicy` needs to learn the reason, so the GUI dialog and CLI refusal can say "Portty couldn't move your N sessions to the updated background service. Restarting ends them." (copy is the planner's or UI's call). Change the signature to a struct, or add a parameter. Callers: `App.restartConsent` (app.go:315), `ensureCLIDaemon` (cmd_cli.go:125-148), `keepMismatchedDaemon` (cmd_attach.go:163-164), tests.
- Zero live sessions: keep today's plain restart (U-01/U-02 path, proven). A handoff of nothing has no value and a larger failure surface.
- Concurrency: two callers (GUI B and CLI B) can both spawn takeovers. The old daemon serializes; the loser gets 409 and exits; its caller's `waitForDaemon` sees version B become ready from the winner.
- `startDaemon`'s wait loop for a free lock (process.go:195-215) must be skipped for the takeover spawn, because the old daemon holds the lock by design.

### Anti-Patterns to Avoid
- **Calling `Hub.Shutdown`, `HubManager.Shutdown`, `api.Stop`, `KillSession` or any context cancel on the old daemon's handoff exit path.** Each one either sends viewers a terminal close, unlinks the socket the new daemon serves, or kills shells.
- **Letting the old daemon's SIGTERM path run after COMMIT.** `runDaemonCore` on SIGTERM calls `api.Stop()` (process.go:107-109), which unlinks daemon.sock. The new daemon's `socketLost` then sees the path gone within 5 s and exits "to free the lock", ending every session. After FREEZE, a `handedOff` flag must make the shutdown path skip the unlink and session teardown.
- **Writing the state blob to a temp file.** History holds terminal contents, and history.go:18-19 says: "It is never written to disk: terminal contents include secrets". Memory to memory over the 0600 socket only.
- **Using `(*os.File).Fd()` anywhere on the master or on listener files you will pass.** It switches the fd to blocking and breaks deadlines. Use `SyscallConn().Control`.
- **Killing the takeover child on caller cancellation (Quit All).** After COMMIT that child is the daemon that owns every session.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Ancillary-data encoding | Manual `cmsghdr` byte packing | `unix.UnixRights` / `ParseSocketControlMessage` / `ParseUnixRights` | Alignment and `CmsgSpace` differ by arch |
| Listener reconstruction | Custom accept loops on raw fds | `net.FileListener` | Registers with the netpoller and returns typed `*net.UnixListener` / `*net.TCPListener` |
| Interrupting a blocked PTY read | Writing a sentinel to the slave, signals, closing the fd from another goroutine | Non-blocking dup + `SetReadDeadline` (P3) | A sentinel pollutes the stream; tty reads restart after signals under SA_RESTART |
| Process exit of a non-child | Polling `kill(pid, 0)` | kqueue `EVFILT_PROC` | Event-driven, gives status on this OS, no zombie confusion (`kill(pid,0)` succeeds on a zombie) |
| Emulator state transfer | Reflect/unsafe walk of vt's cells | Rendered seed via the existing rebuild code | vt's internals are unexported; vtalt.go already shows how fragile one reflected field is |
| Peer authentication on the socket | Shared secret files | `GetsockoptXucred(SOL_LOCAL, LOCAL_PEERCRED)` uid check | Kernel-attested; the socket is already 0600 in the user's dir |
| Wire versioning | Ad hoc "if field present" checks | One `HandoffVersion int` in HELLO + gob field tolerance | Two independent builds must refuse each other cleanly |

**Key insight:** every piece of this handoff that touches the kernel is a few standard calls (SCM_RIGHTS, FileListener, kqueue, O_NONBLOCK). The real complexity is ordering, meaning who may read a PTY when. Spend the effort on the state machine and its fault-injection tests, not on transport.

## Runtime State Inventory (live daemon state at upgrade time)

Not a rename phase, but this is a live-state migration, so the five categories are answered explicitly:

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | settings.json, capability.key, ct_disclosed, daemon.log in `~/Library/Application Support/portty` (engine.go:61-69, keystore.go:33, share.go:189) | None: shared on disk; new daemon reads at import |
| Live service config | In-memory only: sessions, hubs, share records, webserver grants, remote caps (all in the old daemon's heap) | Transfer per Pattern 1 table |
| OS-registered state | Optional LaunchAgent `portty-daemon` (kardianos/service, `KeepAlive: false`, service.go:40-63); daemon.lock flock; daemon.sock path | Lock and socket move by fd. A daemon started by launchd hands off to a takeover daemon that launchd does not manage, so `portty daemon stop` (launchctl) would no longer stop it. Document or refuse handoff for service-mode daemons (Open Question 4) |
| Secrets/env vars | Share tokens in memory; capability key on disk; no env-var secrets | Tokens move inside the blob over the 0600 socket; never logged (match existing T-134-06-03 redaction discipline) |
| Build artifacts | The old daemon runs its unlinked old binary after an in-place `ditto` install (08-VM-CHECKS.md install method) | None: the takeover daemon is the caller's binary |

## Common Pitfalls

### Pitfall 1: Viewers told "session ended" during an upgrade
**What goes wrong:** viewers stop retrying and show the end banner.
**Why it happens:** any `Hub.Shutdown` produces `1000 "session ended"`, which `isTerminalClose` treats as final (closeReasons.ts:12-15: `return code === 1000 && (reason === CLOSE_REASON_SHARE_STOPPED || reason === CLOSE_REASON_SESSION_ENDED)`; relay/close_reasons.go: `CloseReasonShareStopped = "share stopped"`, `CloseReasonSessionEnded = "session ended"`).
**How to avoid:** at COMMITTED, close every subscriber with `1012` (`StatusServiceRestart`) and a fixed reason. A plain process exit (1006 to clients) also retries, so 1012 adds clarity, not correctness. The home daemon's proxy forwards close codes verbatim (remote_ws_proxy.go:165-171).
**Warning signs:** the "session ended" banner in a peer tab after an upgrade; `viewer-diag` lines with `code=1000 reason="session ended"` on the viewing Mac.

### Pitfall 2: EAGAIN on input after switching the master to non-blocking
**What goes wrong:** pastes fail with "resource temporarily unavailable".
**Why it happens:** O_NONBLOCK lives on the open file description. Once set, go-pty's own `master` `*os.File`, created when the fd was blocking and so not pollable, returns EAGAIN straight to the caller when the PTY input buffer is full.
**How to avoid:** `Session.Read` and `Session.Write` both go through the pollable dup. Test: write 1 MiB to a child that does not read (`sleep 5`) and assert the write blocks and later completes, never EAGAIN. This also matters for Phase 10's INPT-01 slow-drain scenario.
**Warning signs:** `write ptmx: resource temporarily unavailable` in daemon.log.

### Pitfall 3: Two processes reading one PTY
**What goes wrong:** output is split between daemons and lost from the ring.
**Why it happens:** rollback after the new daemon already started reading, or the new daemon serving before COMMIT.
**How to avoid:** the reader invariant in Pattern 8, asserted in every fault test (a tick stream on a fake PTY must be contiguous in exactly one daemon's ring).

### Pitfall 4: The old daemon unlinks the socket the new one serves
**What goes wrong:** everything dies about 5 s after a successful handoff.
**Why it happens:** `UnixListener.Close` unlinks a Go-created socket; `api.Stop` calls `removeDaemonSocket`; SIGTERM on the old daemon runs both.
**How to avoid:** `SetUnlinkOnClose(false)` before any close and a `handedOff` guard on the shutdown path. Test: after a handoff in-process, `os.Stat(socketPath)` still names the same inode, and the new daemon's `socketLost` returns false over 2 intervals.

### Pitfall 5: Offsets drift so deltas become wrong
**What goes wrong:** viewers resume at the wrong byte: duplicates or gaps.
**Why it happens:** re-framing the ring, transferring `Snapshot()` without `end`, or appending the seed to the ring.
**How to avoid:** transfer `buf`, `end` and `truncated` exactly. The seed goes only into the emulator, never into the ring. Test: `Join(sub, X, true)` on the imported hub equals it on a reference hub for X across the ring window.

### Pitfall 6: Freeze waits forever on an idle shell
**What goes wrong:** the handoff hangs at FREEZE.
**Why it happens:** the master is blocking (today's creack/pty open), so the deadline is unsupported.
**How to avoid:** Pattern 2 for every session a v1.2+ daemon creates. Adopted sessions inherit non-blocking through the shared OFD. Freeze must fail (abort the handoff, not hang) if `SetReadDeadline` returns `os.ErrNoDeadline`.

### Pitfall 7: The seed mutates the old emulator, then the handoff rolls back
**What goes wrong:** after a rolled-back handoff the old daemon's emulator is out of alt screen or carries probe cells.
**Why it happens:** the main-screen read uses `?1049l` and the probes print cells (hub.go:731-748), as the existing rebuild does.
**How to avoid:** on rollback, the old hub rebuilds its emulator from its own exported seed (same `applySeedLocked`). The alternative is reading the main screen read-only through vtalt.go's reflected `scrs[0]` (vtalt.go:28-38). That keeps the old emulator intact but widens the reflect/unsafe surface that `TestVTLayout_AltScreenScrollback` pins. Prefer the re-import.

### Pitfall 8: The kqueue registration races the exit
**What goes wrong:** a shell that exits between fd receipt and registration gets ESRCH, so no status.
**How to avoid:** treat ESRCH as status unknown (-1). EOF on the master still drives the session to stopped. Register as the first import step.

### Pitfall 9: PID reuse on kill of an adopted session
**What goes wrong:** `KillSession` on an adopted session whose shell already exited signals an unrelated process group.
**How to avoid:** signal only while the exit watcher has not fired. After NOTE_EXIT, mark the session's pid dead. This matches `killSession`'s existing nil-cmd early return (cleanup.go:21-27).

### Pitfall 10: Local viewers on the upgrading Mac do not reconnect
**What goes wrong:** the office local tab shows nothing after the upgrade; `portty attach` on the office Mac prints "Detached."
**Why it happens:** host-path RelayClients are built without `reconnect` (TerminalPanel.tsx:498 `reconnect: isGuest`; the comment at :454-455 says "host-path tabs never reconnect (a local relay drop means the daemon and session died)"). Local attach uses one-shot `AttachSession` (cmd_attach.go:364-366).
**How to avoid:** Chesterton's Fence is satisfied, because the premise "a local drop means the session died" stops being true with handoff. Give host tabs `reconnect` with a `beforeReconnect` that re-reads the relay port and checks the session is still listed (else stop with `ended`). Run local attach through `attach.Run` with a `Reauth` that checks the session still exists. A real daemon crash then still ends the tab: the respawned daemon lists no such session.

### Pitfall 11: GUI startup waits on a slow handoff
**What goes wrong:** the frontend's 90 s binding wait or the DEBT-06 15 s daemon wait trips.
**How to avoid:** bound the takeover wait (proposed 10 s) inside the existing `daemonReadyWait` budget (process.go:265 `const daemonReadyWait = 15 * time.Second`).

### Pitfall 12: Firewall prompts from integration tests
**What goes wrong:** every freshly built daemon binary listens on 127.0.0.1 (relay) and prompts the user, per the user's note on test binaries.
**How to avoid:** gate the two-binary integration test behind an env var (pattern: `PORTTY_LIVE_TAILNET_TESTS=1`, TESTING.md Phase 7 note) and run it in the VM or CI.

### Pitfall 13: Fault-injection hooks in release builds
**How to avoid:** env-driven fault points only under the `vmpreview` build tag (share_vmpreview.go:1-5 shows the pattern; `tests/check-brand.sh` keeps the tag out of release scripts). Unit tests use package-level seams instead.

## Code Examples

### Wails SIGTERM root cause and fix (UPGR-05)

Root cause, read this session:
- Wails registers `gosignal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)` (wails/v2@v2.10.2/internal/signal/signal.go:26). Its goroutine receives once and calls `a.application.Shutdown()` (pkg/application/application.go:66-69).
- That reaches `Frontend.Quit`, which calls `OnBeforeClose` and quits only if it returns false (internal/frontend/desktop/darwin/frontend.go:326-336).
- Portty's `beforeClose` returns true unless `quitting` is set (app.go:554-566: "always prevent default quit — modal owns the decision").
- So SIGTERM shows the quit modal (or nothing visible, when the window is hidden), and later SIGTERMs are swallowed by the still-registered Notify. That is why G-01 needed `kill -9`.

```go
// proposed: in App.startup (after wails.Run installed its handler)
signal.Reset(syscall.SIGTERM)             // undo Wails' Notify (os/signal: "Reset undoes the effect of any prior calls to Notify for the provided signals")
term := make(chan os.Signal, 1)
signal.Notify(term, syscall.SIGTERM)
go func() {
    <-term
    a.quitting.Store(true)                // beforeClose now allows the quit; shutdown() leaves the daemon running
    runtime.Quit(a.ctx)
}()
```
Leave SIGINT to Wails (dev Ctrl-C) or treat it the same; the requirement names only SIGTERM. Put the handler body behind a seam so a unit test can call it without signals.

### Quit All cancels an in-flight EnsureDaemon (IN-07)
```go
// proposed: EnsureDaemon(ctx, socketPath, policy); App keeps the cancel of the running one
func (a *App) QuitAll() {
    a.quitting.Store(true)
    a.cancelEnsure()        // startDaemon/waitForDaemon loops check ctx every 50 ms
    a.daemonMu.Lock()       // now returns promptly
    _ = a.client.ShutdownDaemon() // retry on 503 for a few seconds: a frozen old daemon answers 503 until it commits or thaws
    a.daemonMu.Unlock()
    runtime.Quit(a.ctx)
}
```
Rules:
- Cancel before spawn: never spawn.
- Cancel after spawning a fresh (non-takeover) daemon: kill it, since it owns no sessions yet.
- Cancel during a takeover: stop waiting but never kill the takeover child.

### Cmd-Q under the restart dialog
App.tsx renders `QuitConfirmModal` (lines 1297-1305) before the DEBT-08 `RegenerateKeyModal` (1307-1328), so the restart dialog paints on top and its overlay eats the Quit dialog's clicks (G-04 note). Fix: one modal at a time. Render the quit modal only when `restartPrompt === 0`, and keep `showQuitModal` pending so it appears once the restart dialog closes. Vitest: emit `daemon:restart-needed 2`, then `app:quit-requested`; assert exactly one `role="dialog"` and that it is the restart dialog; answer Not Now; assert the quit dialog is now the only dialog.

### API version on /health (UPGR-04)
Today (types.go:61-64, verbatim):
```go
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}
```
Proposed additions: `API int \`json:"api,omitempty"\`` (absent = 1, the v1.0/v1.1 route set, which 08-06-REVIEW-FIX IN-03 found unchanged since v1.0) and `Handoff int \`json:"handoff,omitempty"\`` (absent = cannot hand off). Client rule: `daemonAPI < MinDaemonAPI` → "update needed" (GUI banner; CLI `portty: …` plus exit 1). Phase 9 keeps `MinDaemonAPI = 1`, so a v1.1 daemon kept with Not Now keeps working, as success criterion 4 requires. Test the banner through a seam that raises the minimum.

## State of the Art / Prior Art

| Approach | Who | How | Fit for Portty |
|----------|-----|-----|----------------|
| No live upgrade; client refuses with "protocol version mismatch", user runs `kill-server` | tmux | Server keeps running the old binary; sessions die on kill-server | This is today's DEBT-08 behaviour [CITED: tmux CHANGES via raw.githubusercontent.com/tmux/tmux; nickjanetakis.com/blog/fix-open-terminal-failed-error-with-tmux] |
| Per-session server process that never upgrades | abduco, dtach | Each session has its own long-lived holder; clients come and go | Avoids handoff but is a re-architecture [ASSUMED: from knowledge of abduco's design] |
| Layout "resurrection" (re-run commands) | zellij, tmux-resurrect | Serializes layout, not processes | Loses running programs; not acceptable for Claude Code sessions [ASSUMED] |
| fork+exec with inherited listening fds; old and new both accept; rollback by HUP old / QUIT new | nginx `USR2` | "the old master does not close its listen sockets" | Same rollback idea as ours [CITED: nginx.org/en/docs/control.html] |
| Listening fds fetched from the old process over a unix socket via SCM_RIGHTS | HAProxy `-x` / `expose-fd listeners`; Envoy hot restart | New process asks the old for fds | Direct precedent for the pull design [CITED: haproxy.com/blog/hitless-reloads-with-haproxy-howto] |
| Service manager stores fds across restarts | systemd `FDSTORE=1` | fds "will later be handed back using the usual file descriptor passing logic at the next start or restart" | No launchd equivalent for arbitrary fds; we act as our own fd store during the handoff [CITED: freedesktop.org sd_pid_notify_with_fds] |
| Go library for fd-inheritance upgrades | cloudflare/tableflip | `Upgrade()` / `Ready()` / `Exit()`; "works on Linux and macOS" | Good reference for the ready handshake; we pull instead of inherit, so it is not a dependency [CITED: github.com/cloudflare/tableflip] |

**Self-exec vs socket passing on macOS, measured here:**
- Both work: P2 output `after exec: pid=9467 (same=true)` / `wait4=9468 err=<nil> exited=true code=5`, from a process with 8 busy goroutines.
- Self-exec is simpler for exit status, since the child stays a child.
- Socket passing is the only one that can roll back after the new code runs. Rock-solid beats simple here.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `NOTE_EXITSTATUS` on a non-child keeps working on future macOS releases (observed on 27.2.1, documented "child only") | Pattern 4 | Exit codes of handed-off sessions read as unknown (0, "exited"). Cosmetic: status badge and notification text. Fallback already designed |
| A2 | gob is the right blob encoding | Standard Stack | Low; JSON works too at a size cost |
| A3 | No macOS per-message SCM_RIGHTS fd limit hit by batched sends (not measured) | Pattern 3 | A large send fails with `MSG_CTRUNC` or EMSGSIZE → abort → DEBT-08. Mitigation: per-session messages |
| A4 | flock held via a passed fd behaves as one shared lock (OFD semantics) | Pattern 7 | Lock could free early → a third daemon could start. Pin with a unit test |
| A5 | A unix listener rebuilt by `net.FileListener` does not unlink on close | Pattern 7 | None in practice; api.Stop unlinks explicitly |
| A6 | Accepting a few lost output bytes in the "new daemon dies after COMMIT, before COMMITTED" window is acceptable | Pattern 8 | If the user wants zero loss there, the old daemon must linger, re-adopt and reconcile, which is much more complex. Confirm in discuss-phase |
| A7 | Users accept that handed-off sessions' exit codes may be unknown (fallback case only) | Pattern 4 | Same as A1 |
| A8 | Handoff is allowed from every EnsureDaemon caller (GUI, CLI, attach reconnect) except `--keep-daemon`, because it is non-destructive | Pattern 9 | If the user wants handoff only from the GUI, restrict the callers |
| A9 | Zero-session mismatches keep the plain restart (no handoff) | Pattern 9 | None functionally |
| A10 | abduco/zellij characterisations | Prior art | Informational only |

## Open Questions (RESOLVED)

All six were decided in 09-CONTEXT.md ("Decided autonomously (reversible)" table and the locked decisions): 1 accepted with the window minimized; 2 the round-trip property test against a reference hub; 3 the "could not be moved" wording; 4 no LaunchAgent special case (hand off the same way); 5 the share server reports `api`; 6 the GUI quits without the dialog on a logout, restart or shutdown. The text below is kept as the research record.

1. **Loss tolerance in the post-COMMIT crash window (A6).** What we know: before COMMIT nothing can be lost; after COMMITTED the new daemon owns everything. What's unclear: whether a crash of the new daemon between COMMIT and COMMITTED (milliseconds) may lose the few output bytes it read. Recommendation: accept and log; ask in discuss-phase.
2. **Seed fidelity bar.** What we know: viewers resume by raw delta, so the seed only affects late joiners' full renders, future history lines and the alt-exit rebuild. Recommendation: the property test "render(imported) == render(reference) and stays equal under the same continuation stream" across the existing replay-fidelity corpus. Any divergence class not on the existing `shortcut:` list blocks the plan.
3. **Copy for the failure case.** The GUI dialog and CLI message need a "could not be moved" variant (UPGR-03). This is a small UI decision, not a UI phase (ROADMAP: "UI hint: no").
4. **Service-mode daemons (LaunchAgent).** Should a launchd-managed daemon hand off (and lose launchd management) or skip to DEBT-08? Recommendation: skip handoff when the daemon reports it runs under the service (add a flag in `/health`), since the default spawn path is what the user runs.
5. **Peer-visible API version.** Phases 11 and 13 need "an older daemon or viewer meets the new message with update needed" across Macs. Recommendation: expose `api` in the share server's meta response now (one field) so Phase 11 has it. Confirm scope.
6. **macOS logout and restart.** Wails' `applicationShouldTerminate` returns `NSTerminateCancel` and routes to the quit modal (AppDelegate.m:26-29). A system restart is likely blocked by Portty's modal. Related to UPGR-05 but not required by it; flag for the user.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain go1.26.11 | build, probes, tests | ✓ (used for probes via `GOTOOLCHAIN=go1.26.11`) | go1.26.11 | — |
| macOS kqueue / SCM_RIGHTS / ptmx | handoff | ✓ | macOS 27.2.1 (26B131) | — |
| golang.org/x/sys in module cache | handoff | ✓ | v0.46.0 | stdlib `syscall` has the kqueue constants but no `GetsockoptXucred` |
| Wails CLI (`wails build -platform darwin/arm64 -tags wailsassets`) | two-build VM rig | not checked this session (used by 08-VM-CHECKS) | — | — |
| Tart VMs `portty-test` (office) and `portty-test-2` (home), real tailnet | UPGR-01/02/03/05 VM checks | not probed (host must not be touched) | macOS 27.2 per 08-VM-CHECKS | none: success criteria name the VMs |

**Missing dependencies with no fallback:** none known. VM availability is verified at execution time.

## Test Strategy

`workflow.nyquist_validation` is `false` in .planning/config.json, so there is no Nyquist section. The user asked for a test strategy:

**Unit, relay (fake PTY = `os.Pipe`, pollable with deadlines):**
- T-R1 `Freeze`/`Thaw`: Run stops without closing `Done`; bytes written during the freeze are read after thaw; contiguous ticks.
- T-R2 export/import round trip vs a reference hub fed the same stream:
  - Covers the replay-fidelity fixtures, wrapped lines, charset and links, scroll regions, a ring past 256 KiB, a genuine resize (`geomEnd > 0`), in alt screen (vim-like), and alt screen then exit after import (the rebuild fires).
  - Asserts equal `RenderSnapshot`, equal `Join` full and delta replays for every offset in the ring window, and equality after a continuation stream.
- T-R3 fuzz `FuzzHandoffRoundTrip` seeded from `rebuildFuzzStream`.
- T-R4 the refactored in-process alt-exit rebuild still passes `TestAltScreenExit_RebuildMatchesReference` and `TestAltScreenExit_RebuildFuzz` unchanged.

**Unit, pty (real ptys, local processes only):**
- T-P1 pollable master: freeze an idle read; 1 MiB write to a non-reading child blocks, not EAGAIN.
- T-P2 adopt from fds: send master+slave over `unix.Socketpair` to self, adopt, read, write, resize.
- T-P3 exit status of a real orphan: `sh -c '(sleep 0.3; exit 7) & echo $!'` makes a grandchild reparented to launchd; kqueue returns 7, or the EACCES fallback returns -1.
- T-P4 kill of an adopted session: SIGHUP to the group, SIGKILL after 2 s.

**Unit, daemon (two engines and APIs in one process, temp HOME via the existing TestMain, real unix socket in the temp dir, `/bin/cat` or pipe sessions):**
- T-D1 happy path: ids, names, workDirs, share records and grants equal; the socket inode is unchanged; the relay port is unchanged; `/health` reports the new version.
- T-D2 fault injection at each Pattern 8 step on both sides. Each case asserts the old daemon serves (list, create after thaw, output continues contiguously, resume deltas valid) and the takeover process exits non-zero.
- T-D3 old dies after FREEZE: new proceeds as committed.
- T-D4 two concurrent takeovers: one wins, one gets 409.
- T-D5 freeze gate: mutating routes 503 + `Retry-After`; DaemonClient retries.
- T-D6 SIGTERM to the old daemon after COMMIT does not unlink the socket.
- T-D7 lock held across the whole handoff (`lockHeld` true at every step).
- T-D8 EnsureDaemon: handoff when `handoff >= 1`, DEBT-08 when the field is absent (v1.1) or the takeover fails, plain restart at 0 sessions, `--keep-daemon` skips.

**Unit, GUI/CLI/frontend:**
- SIGTERM seam; QuitAll cancel (an ensure blocked in a lock wait returns within 100 ms of cancel); vitest for one modal at a time.
- Host-tab reconnect (`TerminalPanel.reconnect` pattern); local attach reconnect (`internal/attach/loop_test.go` pattern).
- `api` too old → banner / CLI exit 1.

**Integration (env-gated, e.g. `PORTTY_HANDOFF_IT=1`):**
- Build the root package twice: `-ldflags "-X main.Version=vA"` and `vB` into `t.TempDir()`, temp HOME. Run A `daemon`, create 2 ticker sessions with A's CLI, hold a WS viewer counting ticks, run B `portty list`.
- Assert `/health` is vB, the same ids and shell pids (`ps -o pid=`), contiguous ticks on the reconnected viewer, new-viewer history containing the pre-upgrade `HIST-` lines.
- Fault run: a `vmpreview`-tagged B with a fault env var → A still serving, B CLI prints the "could not be moved" refusal, rc 1.

**VM (extend M-104 rig; new M-NN items in TESTING.md Section 5):**
- UPGR-01: two ticking sessions (one in `vim`), share on, install B over A, launch: no dialog, `/health` B, same shell PIDs, tickers gapless, new viewer gets history, share link works with the same token and refuses a wrong one.
- UPGR-02: office local tab + home peer tab + browser share + home `portty attach` attached during the upgrade; each reconnects within seconds with no repeated or missing tick.
- UPGR-03: kill B mid-handoff (vmpreview fault) → old serving, GUI and CLI say the sessions could not be moved, DEBT-08 offered; v1.1 → B gets DEBT-08.
- UPGR-05: `kill -TERM <gui>` quits within ~2 s with the daemon and sessions alive; Quit All during an approved restart quits within a few seconds; Cmd-Q with the restart dialog open shows one dialog (AX dump).
- Use the U-01 poller and watcher (corrected `ps -ax` poller, 08-VM-CHECKS.md:173).

## Recommended Plan Split (4-5 plans)

| Plan | Wave | Scope | Requirements |
|------|------|-------|--------------|
| 09-01 Lifecycle groundwork | 1 | `HealthResponse.api/handoff`; EnsureDaemon `ctx` + RestartPolicy reason struct; Quit All cancel; GUI SIGTERM handler; one-modal-at-a-time; "update needed" banner + CLI exit; DaemonClient 503 retry | UPGR-04, UPGR-05 |
| 09-02 PTY layer | 1 | Pollable master for all I/O; uniform `pid`; `AdoptSession` from fds; kqueue exit watcher with fallback; adopted kill path; T-P1..P4 | UPGR-01 (substrate) |
| 09-03 Relay state | 1 | `Freeze`/`Thaw`; `HubState` export/import; seed refactor of `rebuildAfterAltExitLocked` + alt-screen extension; staged hub creation; T-R1..R4 | UPGR-01, UPGR-02 (offset fidelity) |
| 09-04 Daemon handoff | 2 (after 01-03) | `/handoff` + `daemon --takeover`; SCM_RIGHTS helpers + peer-cred; freeze gate; listener/lock passing (webserver `tls.NewListener` refactor); share records/grants; commit/rollback state machine with fault seams; old-daemon exit path (1012 kick, no unlink, SIGTERM guard); EnsureDaemon integration and DEBT-08 fallback copy; T-D1..D8 | UPGR-01, UPGR-03 |
| 09-05 Viewers, integration, VM | 3 | Host-tab and local-attach reconnect; env-gated two-binary integration test; vmpreview fault hook; TESTING.md (Sections 2, 4, 5); VM run of all five criteria | UPGR-02, UPGR-03, UPGR-05 (VM) |

09-01..09-03 touch disjoint files (process.go/app.go/frontend, internal/pty, internal/relay) and can run in parallel. Merge 09-05's reconnect work into 09-04 if the planner prefers fewer plans.

## Security Domain

`security_enforcement: true`, ASVS level 1.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (handoff peer) | Same-uid check via `LOCAL_PEERCRED` on the hijacked conn, plus the existing 0600 socket in the user's config dir. No shared secret |
| V3 Session Management | yes (share tokens) | Tokens and grants move unchanged; key reloaded from `capability.key`; wrong tokens still refused (success criterion 1) |
| V4 Access Control | yes | `/handoff` only on the unix socket mux, never on the relay TCP port or the share server; one handoff at a time; mutating routes gated during freeze |
| V5 Input Validation | yes | Versioned HELLO; bounded blob size; every session record validated (ids, fd count matches, ring length ≤ max, history caps) before import; abort on any error (CLAUDE.md "let it crash" toward the old daemon) |
| V6 Cryptography | no new crypto | Capability HMAC unchanged |
| V7 Error handling and logging | yes | No tokens, caps or terminal contents in daemon.log; log only step names, counts, pids (matches 08-VM-CHECKS logging discipline) |
| V8 Data protection | yes | State blob is memory-to-memory only, never a temp file (history.go:18-19) |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Another local process impersonates a takeover daemon to steal PTY fds (and so shell write access) | Spoofing / Elevation | uid check via LOCAL_PEERCRED; socket 0600; same trust boundary as the existing API, whose `POST /sessions` already grants a shell |
| Handoff route reachable from the tailnet | Elevation | Register `/handoff` only on `a.mux` (unix socket), never in `RelayHandler` or the webserver mux; add a test that the relay port and share server 404 it |
| Malformed or hostile blob crashes or corrupts the new daemon | Tampering / DoS | Size caps, field validation, abort before COMMIT; old keeps serving |
| Fault-injection env var in release builds | Tampering | `vmpreview` tag only; brand gate keeps the tag out of release scripts |
| Share link outlives the share after handoff | Info disclosure | Records and grants move exactly; share-off and exit cleanup run in the new daemon (onExit re-attached on adopt) |
| CSP or token gate regression | Tampering | Webserver changes limited to listener construction; CSP middleware and `requireCapability` untouched; existing CSP tests stay green |

## Sources

### Primary (HIGH confidence)
- Codebase, read this session: internal/daemon/{process.go, process_unix.go, api.go, share.go, types.go, engine.go, remote_ws_proxy.go, socket.go, service.go}; internal/pty/{native.go, session.go, cleanup.go, reap_other.go, registry.go}; internal/relay/{hub.go, scrollback.go, history.go, vtalt.go, wraps.go, charstate.go, manager.go, close_reasons.go}; internal/webserver/server.go; internal/capability/keystore.go; app.go; main.go; cmd_attach.go; cmd_cli.go; cmd_daemon.go; frontend/src/{App.tsx, components/TerminalPanel.tsx, lib/relayClient.ts, lib/closeReasons.ts}
- Module sources: go-pty v0.2.3 (pty_unix.go, cmd_unix.go, pty.go), creack/pty v1.1.24 (pty_darwin.go, README), charmbracelet/x/vt (emulator.go, screen.go), wails v2.10.2 (internal/signal/signal.go, pkg/application/application.go, internal/frontend/desktop/darwin/{frontend.go, AppDelegate.m}), golang.org/x/sys v0.46.0, coder/websocket close.go
- Probes run this session (scratchpad fdprobe, execprobe, nbprobe), outputs quoted verbatim above
- `man 2 kqueue` on macOS 27.2.1
- .planning: ROADMAP.md Phase 9, REQUIREMENTS.md UPGR-01..05, PROJECT.md, 08-06-PLAN/SUMMARY/REVIEW-FIX, 08-VM-CHECKS.md (U-01, U-02, G-01..G-06), the three pending todos

### Secondary (MEDIUM confidence)
- https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/kqueue.2.html (EVFILT_PROC visibility rule)
- https://nginx.org/en/docs/control.html (binary upgrade and rollback)
- https://www.freedesktop.org/software/systemd/man/latest/sd_pid_notify_with_fds.html (FDSTORE)
- https://github.com/cloudflare/tableflip (README: "works on Linux and macOS")
- https://www.haproxy.com/blog/hitless-reloads-with-haproxy-howto (expose-fd listeners, `-x`)

### Tertiary (LOW confidence)
- tmux mismatch behaviour: https://nickjanetakis.com/blog/fix-open-terminal-failed-error-with-tmux, https://raw.githubusercontent.com/tmux/tmux/9a4b45dc0fdefead3fe9d471e144da78163860d0/CHANGES
- abduco/dtach/zellij characterisations (training knowledge)

## Metadata

**Confidence breakdown:**
- Mechanism (fd passing, process survival, freeze, exit status): HIGH, probed on the target OS. One undocumented behaviour (A1) has a fallback.
- Architecture (two-phase commit, what moves): HIGH, derived from the code paths that produce today's failures.
- Emulator re-seed fidelity: MEDIUM, built on the proven alt-exit rebuild but needs the alt-screen extension and the round-trip property test.
- UPGR-05 root causes: HIGH, read in Wails source and App.tsx.

**Research date:** 2026-11-19
**Valid until:** 2026-12-19 (stable stack; re-check A1 on any macOS major update)
