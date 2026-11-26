# Phase 15: v1.2 Hardening - Pattern Map

**Mapped:** 2026-11-22
**Files analyzed:** 30 (all modifications; no new source files, a few new test files/cases)
**Analogs found:** 30 / 30 (every file edits an existing file or sits beside a same-package test that is its analog)

All analog paths below were checked as git-tracked source (no gitignored mirrors). Line numbers are from the working tree on `gsd/v1.2` as of 2026-11-22. They may drift if other work lands first. Re-grep the symbol named in each heading.

Post-research decisions in CONTEXT.md override RESEARCH.md where they differ:
- B2 (sidebar): no 35 s time grace. Use "refused vs timeout" at the meta fetch site (see Item B2 below), or split a todo.
- `--status-top`: ship rows-1 for both positions, document the Top limit.
- G-1 `TestKillClean_NoOrphans`: make deterministic with `testutil.WaitFor`.
- O-2 extra: best-effort chmod 0600 after a successful takeover COMMIT.

## File Classification

| File (new/modified) | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `internal/attach/attach.go` (add `BarRows`) | utility | transform | same file `MakeClientResizeFrame`/`MakeClientClaimFrame` (L155-173) | exact |
| `internal/attach/attach_unix.go` (`WatchResize` reserve) | service | event-driven (SIGWINCH) | same file `WatchResize`/`watchResize` (L15-45) | exact |
| `cmd_attach.go` (onConnect L394-401), `cmd_attach_resize.go` | controller (CLI) | request-response | same call site | exact |
| `internal/statusbar/bar.go` (`ReservedRows`) | utility | transform | same file `Start()` L103-117 (`b.mu`, `b.rows`) | exact |
| `internal/attach/attach_test.go`, `internal/statusbar/bar_test.go`, `cmd_attach_test.go`, `internal/relay/hub_grid_owner_test.go` | test | event-driven | `TestWatchResize_SendsClaim` (attach_test.go L60-117); `TestHub_GridTypingClaims` | exact |
| `frontend/src/lib/closeReasons.ts` (`peerConnectVerdict` sawEnded) | utility | transform | same file L20-32 | exact |
| `frontend/src/components/{TabBar,TerminalPanel,WebShareSessionView}.tsx`, `App.tsx` (`sessionEnded` flag, `onSessionEnded`) | component | event-driven | existing `shareStopped`/`onShareStopped` plumbing | exact |
| `frontend/src/lib/__tests__/closeReasons.test.ts`, `TerminalPanel.guestClose.test.tsx` | test | transform | `describe('peerConnectVerdict')` L22-32 | exact |
| `internal/tailnet/sessions.go` (+ `App.tsx`/`sidebarPeers.ts` consumer) | service/utility | request-response | `MetaStatus` enum L44-55 + `doFetchSessionsMeta` L67-94 | role-match |
| `internal/pty/cleanup.go`, `session.go`, `reap_other.go` (`reaped`, route via `signalGroup`) | service | event-driven | `killAdopted` + `signalGroup` seam (cleanup.go L87-125) | exact |
| `internal/pty/adopt_test.go` or `native_test.go` (reaped-session test) | test | event-driven | `TestKillAdopted` (adopt_test.go L270-310) + `recordSignals` (L229-250) | exact |
| `internal/pty/native_test.go` `TestKillClean_NoOrphans` (G-1) | test | event-driven | `internal/testutil/wait.go` `WaitFor` | exact |
| `internal/daemon/ipc.go` (`listenDaemonSocket` chmod) | utility | file-I/O | same file L10-12; test in `socket_test.go` | exact |
| `internal/fdpass/fdpass.go` (`Recv`, new `rightsFromOOB`) | utility | file-I/O | same file `Recv` L62-100; tests `fdpass_test.go` L92-124 | exact |
| `internal/daemon/handoff.go` (O-6 seam + check; post-COMMIT chmod) | service | request-response | same file L873-890 and `peerPID` seam L130 | exact |
| `internal/daemon/handoff_test.go`/`handoff_fault_test.go` (mismatch test) | test | request-response | `handoffScene.failAt` (handoff_test.go L250), `checkOldServes` (handoff_fault_test.go L32) | exact |
| `tray_objc_darwin.m` (`didReceiveNotificationResponse`, register at launch) | component (native) | event-driven | same file `PorttyNotificationDelegate` L84-95, `registerNotificationDelegate` L104-117 | exact |
| `tray.go` (`//export onNotificationClick`) | controller | event-driven | `onTrayUpdate` L109-115 | exact |
| `app_update.go` (`handleNotificationClick`) | service | event-driven | `showWindow` L229-238, `notifyUpdateOnce` (L128-134) | exact |
| `app_update_test.go` | test | event-driven | harness L160-175 (`showWindowFunc` counter) | exact |
| `frontend/src/App.tsx` (`update:show` handler) | component | event-driven | `update:status` subscription L865-878 | exact |
| `frontend/src/App.update.test.tsx` | test | event-driven | existing file | exact |
| `internal/relay/history.go` (`Skip`), `internal/relay/hub.go` (`J` handler) | service | transform | `History.Clear` L71-73 and the `J` handler L1297-1302 | exact |
| `internal/relay/cwd_test.go`, `internal/relay/pathclick_test.go` | test | transform | `cwdHub`/`osc7`/`wantCwd` (cwd_test.go L18-60); `TestPathClick_Relative` (pathclick_test.go L384-433) | exact |
| `frontend/src/lib/pathLinks.ts` (`onLeave` calls `invalidate`) | utility | event-driven | same file L113, L134-150 | exact |
| `frontend/src/lib/__tests__/pathLinks.linkifier.test.ts` (F-2 case + `vi.waitFor` flake fix) | test | event-driven | same file `renderOnce` L93-98 | exact |
| `internal/webserver/input_queue_test.go`, `capability_helpers_test.go` (H2) | test | event-driven | `joinPasteViewer` L32-45; `dialWebServerWS` (capability_helpers_test.go L213-229) | exact |
| `internal/relay/manager.go`, `internal/daemon/engine.go` (doc comments only, C items 2 and 3) | docs | n/a | `killAdopted` doc comment style | role-match |
| `TESTING.md`, `.planning/todos/*` | config/docs | n/a | Phase 14 TESTING.md note lines | exact |

## Pattern Assignments

### Item A: attach sends rows-1

**Analog (frames):** `internal/attach/attach.go` L155-173. Add `BarRows` here, right after these two functions.

```go
func MakeClientResizeFrame(cols, rows uint16) []byte {
	return []byte{
		relay.MsgResize2,
		byte(cols >> 8), byte(cols),
		byte(rows >> 8), byte(rows),
	}
}

func MakeClientClaimFrame(cols, rows uint16) []byte {
	return append(MakeClientResizeFrame(cols, rows), relay.ResizeClaim)
}
```

**Resize watcher** (`internal/attach/attach_unix.go` L15-45). Keep `watchResize(ctx, conn, winch, size)` unchanged and apply the reserve in the closure that `WatchResize` passes:

```go
func WatchResize(ctx context.Context, conn *websocket.Conn) {
	winchCh := make(chan os.Signal, 1)
	signal.Notify(winchCh, syscall.SIGWINCH)
	go func() {
		defer signal.Stop(winchCh)
		watchResize(ctx, conn, winchCh, func() (int, int, error) { return term.GetSize(int(os.Stdin.Fd())) })
	}()
}
// watchResize: size() error => continue; else
//   conn.Write(ctx, websocket.MessageBinary, MakeClientClaimFrame(uint16(cols), uint16(rows)))
```

**Call site** (`cmd_attach.go` L394-401):

```go
onConnect := func(ctx context.Context, conn *websocket.Conn) {
	if cols, rows, err := term.GetSize(int(os.Stdin.Fd())); err == nil {
		_ = conn.Write(ctx, websocket.MessageBinary, attach.MakeClientResizeFrame(uint16(cols), uint16(rows)))
	}
	watchResize(ctx, conn)
}
```

**Bar reserve analog** (`internal/statusbar/bar.go` L103-117). Start stores `b.rows` under `b.mu` only when `getSize` succeeded, so `ReservedRows()` is `b.rows > 0 ? 1 : 0` under `b.mu`, and a nil `*Bar` returns 0. Take the reserve after `Start`, not from `bar != nil` (Pitfall 3). Test with `Options.FallbackCols/FallbackRows` (bar.go L36-37).

**Test analog** (`internal/attach/attach_test.go` L60-117): `watchResize` driven with a `sizes` channel and `winch` channel, asserted with `bytes.Equal(got, MakeClientClaimFrame(...))`. Add a sibling where the size closure subtracts `BarRows(rows, 1)` and expects `MakeClientClaimFrame(90, 29)` from 90x30, plus reserve 0 expecting 30. Table-test `BarRows` for rows 0, 1, 2 and reserve 0 (Pitfall 2, uint16 wrap).

**Hub-level pin:** copy the shape of `TestHub_GridTypingClaims` in `internal/relay/hub_grid_owner_test.go`. Sequence: GUI owner 120x40, CLI reports 80x23 (grid stays), CLI claims 80x23 (grid 80x23), GUI reclaims (120x40).

**`--status-top`:** add a comment at the call site that Top ships rows-1 and absolute-CUP apps can still overdraw row 1 (no DECOM). That is the accepted, documented limit (Pitfall 1).

---

### Item B1: ended peer tab (frontend verdict mapping)

**Analog:** `frontend/src/lib/closeReasons.ts` L20-32.

```ts
export function peerConnectVerdict(errorCode: string): ReconnectVerdict {
  return errorCode === 'not_sharing' || errorCode === 'no_such_session' || errorCode === 'forbidden'
    ? { kind: 'stop', reason: 'share-stopped' }
    : { kind: 'retry' }
}
```

Change: add `sawEnded = false`. For `no_such_session && sawEnded` return `{ kind: 'stop', reason: 'ended' }`. Keep `not_sharing` and `forbidden` as share-stopped even when `sawEnded`. `shareInfoVerdict` (same file, ~L44) already returns `'ended'` for 404, so copy its return shape.

**Flag plumbing:** copy `shareStopped?: boolean` on `Tab` (`components/TabBar.tsx` ~L37), `onShareStopped` in `TerminalPanel.tsx` (guest `onClose` branch ~L663-665) and `WebShareSessionView.tsx` (~L63, 113, 258), and the host-plus-session match in `App.tsx` `dropPeerSession` (~L1038-1046). `reconnectPeerTab` (~L1022-1031) reads the tab's flag at call time.

**Test analog** (`frontend/src/lib/__tests__/closeReasons.test.ts` L22-32):

```ts
describe('peerConnectVerdict', () => {
  it.each(['not_sharing', 'no_such_session', 'forbidden'])('%s stops with share-stopped', (code) => {
    expect(peerConnectVerdict(code)).toEqual({ kind: 'stop', reason: 'share-stopped' })
  })
  it.each(['unreachable', ...])('%s retries', (code) => {
    expect(peerConnectVerdict(code)).toEqual({ kind: 'retry' })
  })
})
```

Extend with `peerConnectVerdict('no_such_session', true)` equal to `{ kind: 'stop', reason: 'ended' }`. Pin the copy strings from `15-UI-SPEC.md` (`SessionEndedBanner.tsx` L128-129).

---

### Item B2: sidebar "Can't reach this Mac" (refused vs timeout)

No time-based grace (post-research decision). The closest analog is the fetch path in `internal/tailnet/sessions.go`:

```go
// L44-55
type MetaStatus int
const (
	// MetaUnreachable: transport error, timeout, or an unexpected status.
	MetaUnreachable MetaStatus = iota
	MetaOK
	MetaForbidden
)
// doFetchSessionsMeta L67-94: client.Do(req) error -> return nil, MetaUnreachable, err
// fetchAllPeerSessionsMeta L216-224: Reachable: status == MetaOK
```

Pattern to follow, if refused is distinguishable: in `doFetchSessionsMeta`'s `client.Do` error branch, test `errors.Is(err, syscall.ECONNREFUSED)`. Do not treat a timeout (`os.IsTimeout`, `context.DeadlineExceeded`) as refused. Return a new status, for example `MetaRefused`. In `fetchAllPeerSessionsMeta` map `MetaRefused` to `Reachable: true` with an empty sessions slice (the same `sessions == nil -> []ShareableSessionMeta{}` guard at L213). Note that `FetchPeerSessionsMeta` (L102-147) retries the IP fallback only `if status == MetaUnreachable`. Decide deliberately whether a refused DNS-name attempt should still try the IP. Over tsnet, refusal may surface as a different error, so verify the error type with a test against a closed local port (`FetchPeerSessionsMetaWithClient` L153 takes a client and baseURL, so tests can point at `127.0.0.1:<closed>`). If it cannot be told apart reliably, write a new pending todo and do not add a grace. That is the locked fallback.

Frontend consumer is unchanged: `App.tsx` ~L798-806 `setRemotePeers(peers ?? [])` and `sidebarPeers.ts` `peerView` already render a reachable peer with nothing shared as "Nothing shared".

---

### Item C: stale pgid on dismiss

**Analog:** `internal/pty/cleanup.go`. Created path (L20-45) signals without a guard, and `killAdopted` (L87-125) has the guard and the seam.

```go
// L36 (created path, the unguarded signal; SIGKILL at ~L68 is the same)
_ = syscall.Kill(-pgid, syscall.SIGHUP)

// L89 seam and L97-125 guard to mirror
var signalGroup = syscall.Kill
...
if alive() {
	_ = signalGroup(-s.pid, syscall.SIGHUP)
}
...
if alive() {
	_ = signalGroup(-s.pid, syscall.SIGKILL)
}
```

Change: add `reaped` (an `atomic.Bool` is simplest) on `Session`. Set it after `cmd.Wait()` returns in `ReapNaturalExit` (`reap_other.go` L19-41) and in `killSession`'s own wait goroutine. Gate both created-path signals on `!s.reaped.Load()`, and route them through `signalGroup`. Do not infer reaped from `ExitCode() != -1` (Research Item C). Only one `Wait` caller at a time (Pitfall 5).

**Test analog:** `internal/pty/adopt_test.go` L229-250 (`recordSignals`) and the second half of `TestKillAdopted` L296-309:

```go
before := len(sent())
if err := b3.Kill(gone.ID); err != nil { t.Fatalf("Kill: %v", err) }
if got := sent(); len(got) != before {
	t.Fatalf("Kill of an exited adopted session sent %v", got[before:])
}
```

Build the created session with `startSh(t, b, "read x; exit 0")` (`native_test.go` ~L342), `readAll(sess)`, write `"\n"`, wait for eof, then `sess.ReapNaturalExit()`. Add a control that a running `sleep 30` session still gets `SIGHUP`.

**Docs-only items 2 and 3:** comment on `HubManager.Shutdown` (`internal/relay/manager.go` ~L73) and on `closeSessionFiles` (`internal/daemon/engine.go` ~L376-380), in the explanatory-comment style of the `killAdopted` doc block above. No code change.

---

### Item G-1: `TestKillClean_NoOrphans` deterministic

**Analog (helper):** `internal/testutil/wait.go`: `WaitFor(t testing.TB, timeout, cond func() bool, msg string, args ...any)`, polling every 10 ms. Replace the two sleeps in `internal/pty/native_test.go` L251-290:

```go
time.Sleep(200 * time.Millisecond)   // -> WaitFor on the sleep child being up
...
time.Sleep(300 * time.Millisecond)   // -> WaitFor killSignal0(-pgid) != nil
```

Keep the final `killSignal0(-pgid)` assertion. For the "child up" condition, use a marker the shell can prove (for example echo through the PTY, or `pgrep`-free: wait until the group exists with `killSignal0(-pgid) == nil`). Check how other `internal/pty` tests import `testutil` before adding the import.

---

### Item D: socket hardening

**O-2** (`internal/daemon/ipc.go` L10-12):

```go
func listenDaemonSocket(path string) (net.Listener, error) {
	return net.Listen("unix", path)
}
```

Change: after a successful listen, `os.Chmod(path, 0o600)`. On failure close the listener and return the error. `os` is already imported. Test in `socket_test.go` style: `shortSocketPath(t, "mode")` (`/tmp/dtestN_name`, avoids the 103-char sun_path limit), listen, `os.Stat(path)` mode `& 0o777 == 0o600`. Post-COMMIT chmod in `handoff.go`: best-effort `os.Chmod(d.socketPath, 0o600)`, with the error `log.Printf`'d and never returned. Follow the existing `log.Printf("handoff: ...")` style.

**O-1** (`internal/fdpass/fdpass.go` L76-100). The gap is `ParseSocketControlMessage` returning `nil, err` and dropping the fds parsed so far:

```go
var fds []int
var perr error
if oobn > 0 {
	var msgs []unix.SocketControlMessage
	msgs, perr = unix.ParseSocketControlMessage(oob[:oobn])
	for i := range msgs { ... fds = append(fds, got...) }
}
switch {
case flags&unix.MSG_CTRUNC != 0: ...
case perr != nil: err = fmt.Errorf("fdpass: parse control message: %w", perr)
case len(fds) != nfds: ...
}
if err != nil { for _, fd := range fds { _ = unix.Close(fd) } ...
```

Change: factor `rightsFromOOB(oob []byte) (fds []int, err error)` that walks with `unix.ParseOneSocketControlMessage` and appends SCM_RIGHTS fds as it goes. An error mid-buffer still returns the earlier fds so the existing close loop frees them. Each fd is closed exactly once (Pitfall 7).

**Test analog** (`fdpass_test.go` L92-124), same `openFds(t)` before/after count, with helpers `socketpair(t)` and `pipe(t)`:

```go
before := openFds(t)
...
if after := openFds(t); after != before { t.Fatalf("open fds %d after ..., want %d (leak)", after, before) }
```

For `rightsFromOOB`, feed `unix.UnixRights(fdA, fdB)` followed by garbage bytes. Assert err != nil, then the returned fds are closed by the caller and `fcntl(F_GETFD)` gives EBADF. Keep `TestRecv_TruncatedClosesFds` and `TestRecv_CountMismatch` green.

**O-6** (`internal/daemon/handoff.go` L873-890; seams at ~L128-131):

```go
var ok hsHelloOK
if err := t.read(frameHelloOK, &ok, maxFrame); err != nil { ... }
t.oldPID = ok.PID
if code, done := t.preStep("after-hello"); done {
```

```go
peerUID       = fdpass.PeerUID
peerPID       = fdpass.PeerPID      // old side's lookup of the takeover; tests replace it
watchPeerExit = pty.WatchExit
```

Add a NEW seam beside them (for example `takeoverPeerPID = fdpass.PeerPID`; do not reuse `peerPID`, Pitfall 6). Replace `t.oldPID = ok.PID` with the research snippet:

```go
kernelPID, err := takeoverPeerPID(t.c)
if err != nil || kernelPID != ok.PID {
	return t.fail(takeoverRefused, fmt.Sprintf("HELLO-OK pid %d does not match the socket peer (%d, %v)", ok.PID, kernelPID, err))
}
t.oldPID = kernelPID
```

A `PeerPID` error refuses (fails toward the old daemon keeping its sessions). Check `t.fail`'s signature at ~L1200-1206 before copying. Test with the `handoffScene` helpers (`handoff_test.go` `failAt` L250 and `installSeams` ~L195-250). Replace the new seam with a function returning another pid and assert outcome `takeoverRefused`. Then assert the old API still serves via `checkOldServes` (`handoff_fault_test.go` L32).

---

### Item E: update-notification click

**ObjC analog** (`tray_objc_darwin.m` L84-95, L104-117). Add a second delegate method inside the same `@implementation`, keeping the `willPresent` form:

```objc
@implementation PorttyNotificationDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
        willPresentNotification:(UNNotification *)notification
          withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    completionHandler(UNNotificationPresentationOptionBanner | ... );
}
@end
```

New method: `userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:`. If `response.actionIdentifier` equals `UNNotificationDefaultActionIdentifier`, call `onNotificationClick((char *)[response.notification.request.identifier UTF8String])` inside `@autoreleasepool`, and always call `completionHandler()`. Declare `extern void onNotificationClick(char *identifier);` beside the existing forward declarations (~L13-16). Call `registerNotificationDelegate()` from tray init, not only from `sendNotification` (~L296). It is idempotent and bundle-guarded (see the `hasValidBundleIdentifier` early return and the `@try/@catch` swallow pattern at L104-117). The Apple selector names are [ASSUMED] until the VM check.

**Go export analog** (`tray.go` L109-115):

```go
//export onTrayUpdate
func onTrayUpdate() {
	app := trayCallbackApp // captured before the goroutine, like onTrayQuit
	if app != nil {
		go app.offerUpdate() // the alert blocks; never on the main thread
	}
}
```

`onNotificationClick(id *C.char)` copies this: capture `app := trayCallbackApp`, convert `C.GoString(id)` on the callback thread, then `go app.handleNotificationClick(idStr)`.

**Go handler** (`app_update.go`, near `showWindow` L229-238). Unexported so Wails does not bind it:

```go
func (a *App) handleNotificationClick(id string) {
	if id != "portty.update-available" { return }
	a.showWindow()
	a.emitEvent("update:show", nil)
}
```

It never calls `DownloadUpdate`. The id string must match `notifyUpdateOnce`'s id at ~L128-134 exactly. Use that constant if one exists, otherwise grep for the literal.

**Go test analog** (`app_update_test.go` L160-175): the harness `showWindowFunc` increments `h.shows` under `h.mu`. Add: click with the update id gives `shows == 1` and one `update:show` emit (see the `emitEventFunc` capture used elsewhere in that file); an unknown id does nothing; no download started.

**Frontend** (`frontend/src/App.tsx` L865-878), add beside the `update:status` subscription:

```tsx
const offUpdate = EventsOn('update:status', (s: UpdateStatus | null) => { gotEvent = true; apply(s) })
...
return () => { offUpdate() }
```

New: `const offShow = EventsOn('update:show', () => setDismissedUpdate(null))`, and call `offShow()` in the same cleanup. Vitest case in `App.update.test.tsx`: dismiss, emit `update:show`, banner visible in "available" state, `DownloadUpdate` not called.

**Manual:** TESTING.md Section 5 item M-152 (notarized VM build, hide the window, click the notice). Per the standing rule in `CLAUDE.md`.

---

### Item F: F-1 relative path after `ESC[3J`

**Analog (fix site):** `internal/relay/hub.go` L1297-1302 and `internal/relay/history.go` L71-73:

```go
emu.RegisterCsiHandler('J', func(params ansi.Params) bool {
	n, _, _ := params.Param(0, 0)
	if n == 3 {
		h.history.Clear()
		h.wraps.clear()
	}
```

```go
func (h *History) Clear() {
	h.lines, h.wrapped, h.head, h.bytes = nil, nil, 0, 0
}
```

Change: add `func (h *History) Skip(n int) { h.pushed += uint64(n) }`, and in the handler after `Clear()` call `h.history.Skip(emu.ScrollbackLen())`. vt's own handler runs after ours and drops exactly those lines. This is prototyped and validated in RESEARCH Item F. Do not change `Hub.PathClick`/`cwdLog.at`. The defect is row numbering, not the lookup.

**Test analog** (`internal/relay/cwd_test.go` L18-60): `cwdHub(t, cols, rows)`, `osc7(dir)`, `cwdRowOfLocked`, `cwdOf`, `wantCwd(t, h, marker, want)`. Feed the repro bytes:

`osc7("/start")+"% cd proj\r\n"+osc7("/proj")+"% clear; ls\r\n"+"\x1b[H\x1b[2J\x1b[3J"+"docs-x\r\ndocs-y\r\n"+osc7("/proj")+"% "`

Expect `docs-x` and `docs-y` both resolve to `/proj`. Add an `ED3`-alone variant with undrained scrollback. Keep `TestCwd_SurvivesResizeAndClear` (cwd_test.go ~L305) green. Click-level version in `pathclick_test.go`: copy `TestPathClick_Relative` (L384-433) with `pcHub`/`pcClick`/`pcPreviewed` and real temp dirs. Assert the click on row 0 previews the file under `/proj`.

---

### Item G: F-2 first Cmd-click on the same cell

**Analog:** `frontend/src/lib/pathLinks.ts` L113 and L134-150.

```ts
const onLeave = () => { pointerInside = false }
...
const invalidate = () => queueMicrotask(() => {
  if (disposed) return
  const lk = (term as any)._core?.linkifier
  if (!lk || !('_activeLine' in lk)) return
  lk._lastBufferCell = undefined
  lk._activeLine = -1
  if (!pointerInside || !lastMove) return
  ...
```

Change: `onLeave` sets `pointerInside = false; invalidate()`. `invalidate` is a `const` declared after `onLeave`, so move `onLeave` (and its `addEventListener('mouseleave', ...)`) below `invalidate`, or hoist, to avoid `no-use-before-define`. Test: add the leave, change row text, return to same cell, Cmd-click case to `pathLinks.linkifier.test.ts`, copying the existing hover and click helpers there and the `pathClicks()` collector (L85-92). The case must fail before the fix. Repro: hover (3,1), click, dispatch `mouseleave` on `.xterm-screen`, `write('\x1b[1;1H\x1b[2Kdocs/other.png')`, `renderOnce()`, hover (3,1), click. Expect `['docs/diagram.png','docs/other.png']`.

---

### Item H1: vitest flake (`renderOnce`)

**Analog:** `frontend/src/lib/__tests__/pathLinks.linkifier.test.ts` L93-98:

```ts
const renderOnce = async () => {
  const before = renders
  term.refresh(0, term.rows - 1)
  await sleep(100)
  expect(renders, 'term.onRender fired').toBeGreaterThan(before)
}
```

Replace with `await vi.waitFor(() => expect(renders, 'term.onRender fired').toBeGreaterThan(before), { timeout: 5000, interval: 10 })`. `vi` is already imported at L23. Convert the other wall-clock assertions the same way (`sleep(100)` at ~L143, 169, 234; `sleep(20)` at ~L165, 252; `resizeTo`'s `sleep(150)` at ~L202). Keep `sleep` only for "nothing happened" windows. Sequence this with Item G, since both edit this file.

### Item H2: Go flake `TestShare_PasteBacklogKeepsViewer`

**Analog:** `internal/webserver/input_queue_test.go` L28-45 (`joinPasteViewer`: `ws.heartbeatEvery, ws.heartbeatTimeout = 50*time.Millisecond, timeout`) and the test at L94-155 (`const timeout = 500 * time.Millisecond`, hold `3 * timeout`). Change the per-ping ceiling to 5 s (keep `heartbeatEvery` 50 ms), and replace the wall-clock hold with an event wait counting client-received pings through `websocket.DialOptions.OnPingReceived` (coder/websocket v1.8.15). Add the optional dial variant next to `dialWebServerWS` in `capability_helpers_test.go` (~L213-229). Event-wait pattern: `testutil.WaitFor(t, timeout, cond, msg)` as in `internal/testutil/wait.go`. Leave `TestShare_DeadViewerReapedBacklogDelivered` (100 ms timeout) untouched. Product code (`relay.Heartbeat`) is untouched.

---

## Shared Patterns

### Test seams over mocks
**Source:** `internal/pty/cleanup.go` L89 (`var signalGroup = syscall.Kill`) with `recordSignals` (`adopt_test.go` L229-250); `internal/daemon/handoff.go` L128-135 seam vars with `installSeams` (`handoff_test.go`); `app_update_test.go` `showWindowFunc`, `emitEventFunc`.
**Apply to:** Items C, D (O-6), E. Add a package-level `var` seam and restore it with `t.Cleanup`, as in `recordSignals`:
```go
prev := signalGroup
signalGroup = func(pid int, sig syscall.Signal) error { ...; return prev(pid, sig) }
t.Cleanup(func() { signalGroup = prev })
```

### Events, not wall-clock, in tests
**Source:** Go `internal/testutil/wait.go` `WaitFor`; vitest `vi.waitFor`.
**Apply to:** H1, H2, G-1, any new test that waits on a render, redial, ping or process exit.

### Fail toward the old daemon / never silently fall back
**Source:** `handoff.go` `t.fail(takeoverRefused, ...)`; CLAUDE.md "Silent Fallbacks".
**Apply to:** O-6 (a `PeerPID` error refuses), O-2 (a chmod failure closes the listener and returns the error). The post-COMMIT chmod is the one exception: log only, never fatal.

### Go error and logging style in daemon code
**Source:** `handoff.go` L876-889: `log.Printf("handoff: ...: %v", err)` then `t.cleanup()` and return a typed outcome (`takeoverFailed`, `takeoverRefused`, `takeoverBusy`). Wrapped errors use `fmt.Errorf("fdpass: ...: %w", err)`.

### Frontend event subscription
**Source:** `App.tsx` L865-878: `EventsOn(name, handler)` returns an unsubscribe closure, collected in the `useEffect` cleanup. Apply to the `update:show` handler.

### TESTING.md standing rule (every task that adds or changes tests)
**Source:** `CLAUDE.md`. For new test files, update Section 2 counts and add a `> Note: Portty Phase 15 ...` line, and add a Section 4 row per new test file (repo-relative path only, reuse existing IDs such as INPT-01, INPT-06, PATH-01, PATH-02, UPGR-01, UPD-01). Add M-NN items from M-152 (notification click, attach full-screen app check, `--status-top` limit) to Section 5. Run `bash tests/check-traceability-paths.sh` (baseline `OK: 228 traceability paths exist`).

### Commands (from RESEARCH)
`GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 ./internal/<pkg>/...` (root package: `.`). Frontend: `cd frontend && pnpm exec vitest run <file>`, `pnpm exec tsc --noEmit`. Phase gate: one clean `go test -race -short -count=1 ./...` and `pnpm test`.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `tray_objc_darwin.m` `didReceiveNotificationResponse` method body | native delegate | event-driven | No existing click-response handler anywhere in the repo. Copy the surrounding delegate and `registerNotificationDelegate` structure only. The Apple API names are [ASSUMED] (RESEARCH A2) and confirmed only by the VM check M-152. |
| Refused-vs-timeout classification in `internal/tailnet/sessions.go` | service | request-response | No code currently distinguishes ECONNREFUSED from timeout. The `MetaStatus` enum is the structural analog, but the error classification is new. Verify against a closed local port, and over tsnet in a VM row (never the host tailnet, per memory). Split a todo if unreliable. |

## Ordering and conflict notes for the planner

- Parallel-safe Go groups: A (attach, statusbar, cmd_attach), C plus G-1 (`internal/pty`), D (`internal/daemon`, `internal/fdpass`), F (`internal/relay`), H2 (`internal/webserver`), B2 Go side (`internal/tailnet`).
- Frontend serialization: B1 and E and B2 all touch `App.tsx`. G and H1 both edit `pathLinks.linkifier.test.ts`. Run each pair in sequence.
- Item C and G-1 share `internal/pty`, so one plan or sequential plans.
- Item I (todo moves to `.planning/todos/completed/`, TESTING.md) goes last.

## Metadata

**Analog search scope:** repo root Go files, `internal/{attach,statusbar,pty,daemon,fdpass,relay,webserver,tailnet,testutil}`, `frontend/src/{lib,components}` and `App.tsx`
**Files read for excerpts:** about 20 (targeted line ranges, no re-reads)
**Pattern extraction date:** 2026-11-22
