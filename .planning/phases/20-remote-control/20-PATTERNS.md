# Phase 20: Remote Control - Pattern Map

**Mapped:** 2026-11-26
**Files analyzed:** 34 new or modified
**Analogs found:** 32 / 34 (all analog paths verified git-tracked with `git ls-files`)

Reuse note: Phase 19 plans 19-01..19-09 and 19-17 are all MERGED on this branch (19-06 viewer, 19-08 daemon wiring and 19-09 state/Locked cache included, contrary to the brief). Every Phase 19 analog below is real merged code, not "PLANNED". Line numbers are from this branch at mapping time and Phase 19 is still editing; re-grep before relying on exact lines. Cross-cutting items already mapped in `19-PATTERNS.md` (gate order, test seams, crash containment, TESTING.md rule, SettingsSheet `Row`) are not repeated, only cross-referenced.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `internal/screencap/input_darwin.m` + `input_darwin.go` | bridge (cgo ObjC): CGEvent post, AX focus/raise/frame | request-response | `internal/screencap/sys_darwin.m/.go` (`pc_pointer_nudge`, `pcMoveTo`) | exact |
| `internal/screencap/pasteboard_darwin.m` + `.go` | bridge (cgo ObjC): NSPasteboard | request-response | `internal/screencap/sys_darwin.m/.go` (`pc_session_locked` read pattern) | role-match |
| `internal/screencap/input_stub.go` (`!darwin`) | stub | - | `internal/screencap/synthetic.go` + `sys_darwin.go` `testing.Testing()` refusal | role-match |
| `internal/screencap/keytable.go` | utility (closed table) | transform | `frontend/src/lib/ctrlMap.ts` (closed table idiom) | role-match |
| `internal/screencap/media_darwin.m/.go` (modify: surface select, window filter, `updateContentFilter`, size events) | bridge | streaming | itself (`pcMainDisplay` 380-411, `pc_encoder_rebuild` 563-584) | exact |
| `internal/screencap/screencap.go` (modify: `Source` gains surface list, window size/frame) | interface | transform | itself (`Surface`, `Source`, `Rebuild`) | exact |
| `internal/screencap/synthetic.go` (modify: window surfaces, size change on select) | test utility | streaming | itself (`SelectSurface` 146-152, `Rebuild` 162-172) | exact |
| `internal/desktop/input.go` (message types, per-viewer state, gates, held sets, dead-man) | service | event-driven | `internal/desktop/manager.go` (viewer, msg types, Deps seams) + `viewer.go` | role-match |
| `internal/desktop/input_map.go` (normalized to global points, click count) | utility | transform | `internal/screencap/sys_darwin.go` `nudgeDelta` (pure fn beside native call, table-testable) | partial |
| `internal/desktop/input_queue.go` (bounded queue, latest-wins moves) | utility | event-driven | `internal/relay/input.go` (InputMaxFrames/Bytes quota idiom) | role-match |
| `internal/desktop/surface.go` (list, select, epoch, fallback) | service | request-response | `internal/desktop/manager.go` `restartSource` 651 / `noteSize` 793 / `broadcast` 812 | role-match |
| `internal/desktop/clipboard.go` (chunking, concealed, state) | service | request-response | `internal/relay/protocol.go` 137-141 (split around 32768) | partial |
| `internal/desktop/viewer.go` (modify: `Rights` gains Control/Clipboard bits) | model | - | itself (lines 12-26) | exact |
| `internal/desktop/manager.go` (modify: hello `input:1`, new WS message types, Deps seams) | controller (WS) | request-response | itself (msgHello 108, Deps 153-180, ServeDesktopViewer 460-587) | exact |
| `internal/desktop/peer.go` (modify: wire input hook) | service | streaming (pion) | itself (`answer` 98-170: `OnDataChannel` hook) | exact |
| `internal/desktop/*_test.go` (fake poster, fake AX, fake pasteboard, loopback input) | test | - | `internal/desktop/fakes_test.go` `world`; `fanout_test.go` `TestFanout_DataChannelHook` | exact |
| `internal/webserver/desktop_routes.go` (likely unchanged) / `input_limit_test.go` | route/test | request-response | itself; `input_limit_test.go` pins 32768 | exact |
| `internal/daemon/*` (wire Control state to GUI/tray, if any) | service | request-response | 19-08 `desktopAdapter` / `GET /desktop` | role-match |
| `frontend/src/lib/desktopInput.ts` (capture, coalesce, key/text, release-all, heartbeat) | utility | event-driven | `frontend/src/lib/desktopClient.ts` (injectable deps, generation counter) + `lib/ctrlMap.ts` + `lib/pasteGuard.ts` | role-match |
| `frontend/src/lib/desktopKeymap.ts` (optional allow-list) | utility | transform | `frontend/src/lib/ctrlMap.ts` | role-match |
| `frontend/src/lib/desktopClient.ts` (modify: data channel before `createOffer`, surface/clipboard/epoch messages) | utility | streaming (WS+RTC) | itself (`handle` 357-381, `DesktopClient` interface 82-88) | exact |
| `frontend/src/components/DesktopView.tsx` (modify: control layer, bar, chips) | component | event-driven | itself (`onVideoRect`, `videoContentRect` 17-30, `updateRect` 142-153) | exact |
| `frontend/src/components/DesktopBar.tsx` + menus (Picker, Send Keys, Clipboard) | component | event-driven | `frontend/src/components/ContextMenu.tsx` | role-match |
| View Only / Control segmented control | component | event-driven | `SettingsSheet.tsx` Appearance `.share-segbar` 303-321 | exact |
| Lock Screen confirm dialog | component | request-response | `frontend/src/components/RegenerateKeyModal.tsx` + `lib/useDialogFocus.ts` | exact |
| Copy fallback / Paste popovers | component | request-response | `frontend/src/components/SharePopover.tsx` (role="dialog" popover) + `lib/pasteGuard.ts` | role-match |
| Ctrl/Cmd swap persistence (per browser) | utility | CRUD (localStorage) | `NewSessionModal.tsx:15,32` `localStorage.getItem/setItem` (LAST_DIR_KEY) | role-match |
| `frontend/src/components/SettingsSheet.tsx` (Desktop group row; maybe Paste from Other Apps row) | component | CRUD | itself (`Row`, Sharing pane Desktop group) | exact |
| Phase 19 permissions sheet strings (Accessibility wording change) | component | - | 19-UI-SPEC/SettingsSheet onboarding sheet | exact |
| `TESTING.md` | docs/test | - | Section 2/4/5 rules (M-172 is the last item seen) | exact |
| `internal/screencap/*_darwin_test.go` | test | - | `internal/screencap/screencap_darwin_test.go` | role-match |

## Pattern Assignments

### `internal/screencap/input_darwin.m` and `input_darwin.go` (bridge, request-response)

**Analog:** `internal/screencap/sys_darwin.m` / `sys_darwin.go`. This is the closest real code: it already posts CGEvents at `kCGHIDEventTap` from a private source in a launchd-hosted daemon.

**ObjC function shape** (`sys_darwin.m` 14-20, 133-166): every function is `int pc_*` returning 0 or an error code, wrapped in `@try/@catch`, with the shared error enum:
```objc
enum { kSysExceptionErr = -90001, kSysNilErr = -90002 };

static int pcMoveTo(CGEventSourceRef src, CGPoint to) {
	CGEventRef move = CGEventCreateMouseEvent(src, kCGEventMouseMoved, to, kCGMouseButtonLeft);
	if (move == NULL) return 0;
	CGEventPost(kCGHIDEventTap, move);
	CFRelease(move);
	usleep(50 * 1000);
	CGEventRef now = CGEventCreate(NULL);          // live location read-back
	...
	return fabs(p.x - to.x) <= 0.5 && fabs(p.y - to.y) <= 0.5;
}

int pc_pointer_nudge(double dx, int *ok) {
	@try {
		...
		CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStatePrivate);  // private source: RESEARCH Pattern 3
		...
	} @catch (NSException *e) { return kSysExceptionErr; }
}
```
Differences for Phase 20: create the private `CGEventSourceRef` once (static) and reuse it; tag every event with `CGEventSetIntegerValueField(ev, kCGEventSourceUserData, kPorttyMarker)`; post `kCGEventMouseMoved` or the `*Dragged` type, never `CGWarpMouseCursorPosition` (RESEARCH R3); never `usleep` on the post path (RESEARCH Pitfall 9: the 50 ms sleep is acceptable only in the watchdog nudge, not per input event). Control-needs-Accessibility check can reuse the `pcMoveTo` read-back idea on a separate timer. Add `AXUIElementSetMessagingTimeout(systemWide, 0.25)` (R13) and read focus with `AXUIElementCopyAttributeValue(systemWide, kAXFocusedApplicationAttribute)`, never `NSWorkspace` (R2).

**cgo Go side** (`sys_darwin.go` 8-43, 125-141): prototypes in the cgo comment, `import "C"`, `sysErr` helper, and the test refusal:
```go
/*
#cgo darwin LDFLAGS: -framework IOKit -framework CoreGraphics -framework ApplicationServices -framework CoreFoundation
int pc_pointer_state(double *x, double *minX, double *maxX);
int pc_pointer_nudge(double dx, int *ok);
*/
import "C"

func NudgePointer() (bool, error) {
	if testing.Testing() {
		return false, ErrRefusedInTest
	}
	...
	if rc := C.pc_pointer_nudge(...); rc != 0 { return false, sysErr("nudging the pointer", rc) }
```
Copy: `testing.Testing()` refusal on every function that posts an event or touches AX (reads like `SessionLocked` may run anywhere), and `sysErr`-style error wrapping. Add `-framework AppKit` only for `pasteboard_darwin` and the NSWorkspace fallback rung (R1 rung 2). Keep CFLAGS in one file: `media_darwin.go:12` already sets `#cgo darwin CFLAGS: -x objective-c -fobjc-arc`; the other .m files rely on it (ARC on, see `preflight_darwin.m:5`). Do not add a minimum-version CFLAG (media_darwin.go:8 comment: build.sh sets macOS 14).

**Poster seam for tests:** define a Go interface (for example `InputPoster`) in `internal/screencap` or `internal/desktop` with a darwin cgo implementation and a fake in `internal/desktop/fakes_test.go`. Follow the `Deps` func-seam style (manager.go 153-180), not a global.

---

### `internal/screencap/pasteboard_darwin.m` and `.go` (bridge, request-response)

**Analog:** `sys_darwin.m` `pc_session_locked` (lines 19-38): a read that returns an error code for "no answer" instead of a false-looking default (`kSysNilErr`, never "unlocked").
```objc
CFDictionaryRef d = CGSessionCopyCurrentDictionary();
if (d == NULL) return kSysNilErr;
```
Apply: pasteboard read returns distinct codes for concealed, would-prompt (Waiting for approval), empty and ok. Check types first, then read `public.utf8-plain-text`; skip `org.nspasteboard.ConcealedType` and `org.nspasteboard.TransientType`; output buffer pattern for strings: see `media_darwin.go` `C.pc_still_capture(... **out, *n ...)` and its free path. Bound the read with a Go-side timeout goroutine. No in-repo analog for NSPasteboard itself (see No Analog Found).

---

### `internal/screencap/media_darwin.m/.go` (modify: window filter, `updateContentFilter`, size events)

**Analog:** itself.

**Display resolution** (`media_darwin.m` 377-411): `pcMainDisplay` fetches `SCShareableContent`, finds the `SCDisplay`, builds the filter and capture size (`display.width * f.pointPixelScale` capped via `pcCaptureSize`):
```objc
[SCShareableContent getShareableContentExcludingDesktopWindows:NO onScreenWindowsOnly:YES completionHandler:^(...){ ... dispatch_semaphore_signal(sem); }];
if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 15 * NSEC_PER_SEC)) != 0) { ... return kPCTimeoutErr; }
SCContentFilter *f = [[SCContentFilter alloc] initWithDisplay:display excludingWindows:@[]];
```
Generalize into "resolve surface (display by `CGDirectDisplayID`, or window by `windowID`) to filter plus size". Window filter is `initWithDesktopIndependentWindow:`.

**Running-stream reconfigure** (`pc_encoder_rebuild`, 563-584): encoder first, then `updateConfiguration` with a semaphore wait (5 s) and error-code mapping. A surface switch adds `updateContentFilter:completionHandler:` after `updateConfiguration` with the same semaphore and `kPCTimeoutErr` mapping, and the same `@try/@catch` to `kPCExceptionErr`. Drop frames whose size differs from the encoder session until both completions return (RESEARCH R5).

**Go side** (`media_darwin.go` 199-207): `checkSurface` currently accepts only the main display (ID 0) and `SelectSurface` is a thin wrapper; Phase 20 replaces both. `AccessUnit.Width/Height` (`screencap.go` 64-69) is already the per-unit size event; the Manager's `noteSize` (manager.go 793) turns it into the `size` message. `Rebuild` (media_darwin.go 309-324) takes the `d.mu` lock and refuses when not running: copy for a surface-switch method.

**Interface** (`screencap.go` 89-106): the `Source` doc already says "SelectSurface with a SurfaceWindow returns ErrSurfaceUnsupported in Phase 19 (Phase 20 implements it)". Phase 20 needs a way to list surfaces and read window frames; add an optional second interface the Manager type-asserts (as `DisplaySource.HardwareEncoder` is used via assertion, media_darwin.go 326) rather than widening `Source` for every fake.

---

### `internal/screencap/synthetic.go` (modify, test utility)

**Analog:** itself. `SelectSurface` (146-152) rejects windows; `Rebuild` (162-172) sets size and forces a keyframe, exactly what a real switch does:
```go
func (s *SyntheticSource) Rebuild(width, height int) error {
	...
	s.mu.Lock(); s.w, s.h = width, height; s.forceKF = true; s.mu.Unlock()
```
Add: accept `SurfaceWindow`, switch to a per-surface fixture size, force a keyframe, count selects (like `KeyframesForced()` 174). `desktop/fanout_test.go` `TestFanout_SizeEventOnRebuild` (234) is the test to copy for "switch emits a size event".

---

### `internal/screencap/keytable.go` (utility, transform)

**Analog:** `frontend/src/lib/ctrlMap.ts` (header comment, lines 1-2): "A closed table, so a character with no control code goes as typed rather than as a guess." Same rule on the Go side: unknown W3C `code` returns `(0, false)` and is a counted drop, never a guess. Build the table from the `Events.h` kVK values RESEARCH quotes (Pattern 3) and add a Go test that every kVK is unique and every viewer-allowed code resolves. Pure Go, no cgo, so it is testable without the build tag (keep it out of `*_darwin.go`).

---

### `internal/desktop/input.go`, `input_map.go`, `input_queue.go` (service, event-driven)

**Analog for message types and Deps seams:** `internal/desktop/manager.go`.

Wire-message style (lines 108-143): small structs with `json:"type"` tags and string consts for types (`typeHello`, `typeState`, ...), plus a loosely decoded `inbound` struct where "the type decides which fields matter, and unknown fields are ignored" (145-151). Input messages use the data channel `{t, ...}` envelope per CONTEXT; copy the tolerant-decode idea (unknown fields ignored, malformed message is a counted drop) but with `t` as the discriminator and a 4 KiB check before `json.Unmarshal`.

Seam pattern (Deps 153-180, defaults 182-191): func-typed fields, package-var defaults, nil means the real bridge:
```go
Lock func() (bool, error)   // is the session screen locked
Nudge func() (bool, error)
...
var ( defaultLock = screencap.SessionLocked ... )
```
Add input seams to `Deps` the same way (`PostInput`, `Pasteboard`, `AXFocus`), tests replace each. Lock gate source is `Manager.Locked()` from 19-09 (250 ms wall-clock cache, true on a read error, fail closed): call it synchronously on each press (RESEARCH R8); it already matches Phase 20's needs, so do not add a second poller.

**Per-viewer state:** `viewer` struct (manager.go 218-227) is guarded by `Manager.vmu`; `Viewer{ID, Rights, JoinedAt}` (viewer.go 22-26) is the public record, `Rights uint8` with `RightsViewOnly = 0` (viewer.go 15-18). Add `RightsControl`/`RightsClipboard` bits; set both at the `viewer` construction site (manager.go 525, today `Rights: RightsViewOnly`) for own-account viewers. Held sets, last epoch, rate bucket and dead-man deadline go on `viewer` (or a sub-struct) under the existing lock discipline; use one leaf mutex like `smu` in state.go (19-09 pattern) so release-all from `leave`, lock flip and dead-man cannot race.

**Recover and logging:** copy `defer recoverLog()` (manager.go 404, used at 461 and peer.go 131) at the top of every goroutine and every data-channel callback. Log only counts and kinds (CONTEXT [Logging]); copy `slog.Info("desktop: viewer joined", "viewer", id[:8], ...)` style (manager.go 585) of using the 8-char viewer prefix.

**Bounded queue:** analog `internal/relay/input.go` 30-48 (`InputMaxFrames`, `InputMaxBytes`, hub quota, `ErrInputClosed`) and `internal/relay/protocol.go` 137-141:
```go
// MaxInputPayload ... 32768 (coder/websocket's default read limit, kept by every input reader as the DoS guard) minus the type byte. ... senders split longer input
const MaxInputPayload = 32767
```
Reuse the idiom (named consts, frames/bytes bound, drop-and-count at the bound) but the structure differs: moves are latest-wins, presses/releases never dropped, `OnMessage` only enqueues without blocking (RESEARCH R12).

**Existing SCTP bound:** `peer.go` 57-74 already applies `se.SetSCTPMaxMessageSize(8192)` (summary `SCTPMax: 8192`, test at fanout_test.go 295 asserts `a=max-message-size:8192`), so the 4 KiB cap is enforced in two layers; do not re-add the setting.

**`input_map.go`:** pure-function style like `nudgeDelta` (`sys_darwin.go` 116-123): small pure Go function next to native code, table-tested with edge cases (negative origins, right edge). Pointer coordinates are global points; backing scale is not used in posting (RESEARCH Pattern 1).

---

### `internal/desktop/peer.go` and the data-channel hook (modify, streaming)

**Analog:** itself. The hook is already plumbed and the registration happens before the offer is applied:
```go
hook := m.d.OnDataChannel
if hook == nil { hook = CloseDataChannel }
pc.OnDataChannel(func(dc *webrtc.DataChannel) {
	defer recoverLog()
	hook(v.Viewer, dc)
})
```
(peer.go 126-133). Phase 20 sets `Deps.OnDataChannel` (manager.go 160-162) to an `input` handler that checks `dc.Label() == "input"` and closes any other label with the existing `CloseDataChannel` (viewer.go 36). Contract comment on `DataChannelHook` (viewer.go 28-32): "never block inside OnMessage". Release-all must also hook `dc.OnClose` and `OnConnectionStateChange` failed (manager.go ~560 already closes the socket on `PeerConnectionStateFailed`; `leave` (663) is the single place that runs on every exit path, so call `releaseAll` there).

Hello capability: `msgHello.Caps map[string]int` (manager.go 108-113), server sends `Caps: map[string]int{"video": 1}` (line 509). Add `"input": 1` there; `TestFanout_HelloCapsForwardCompatible` (fanout_test.go 322) shows the forward-compat test to extend.

---

### `internal/desktop/manager.go` WS messages: surface list/select, epoch, clipboard, control state (modify, request-response)

**Analog:** itself. Server-to-viewer send: `send(ctx, c, msgX{Type: ...})` (manager.go 411); fan-out to all viewers: `m.broadcast(ctx, v)` (812) and `broadcastState` (824); inbound loop at 570-587 handles only `ping` today and must grow a `switch in.Type` with `select`, `surfaces`, `clip_*` cases, with unknown types still counted not fatal only if forward-compat is wanted (today any bad JSON ends with `reject(errCodeBadMessage)`). Constants `readLimit = 32768` (line 85) is the read limit the clipboard chunking must fit under (RESEARCH R4: cap chunks near 16 KiB of text, measure the serialized frame; CONTEXT post-research agrees); Manager already sets `c.SetReadLimit(readLimit)` (line 463).

State message extension: `msgState{State, Viewers, Locked}` (120-125) is built by `broadcastState`; the Control-needs-Accessibility flag and the shared-surface description should ride as additive fields (older viewer clients ignore unknown fields: `desktopClient.ts` `handle` reads fields by name from a `Record<string, unknown>`).

`Status`/`deriveState` (state.go, 19-09): Locked is an attribute, not a state; Phase 20 control-state ("needs Accessibility") should follow the same attribute pattern, not add to the `State` enum.

---

### `internal/desktop/*_test.go` (test)

**Analog:** `internal/desktop/fakes_test.go` (`world` fake with mutex, counters, injectable errors and a fake clock; `fakeSource`/`sources` for sources) and `fanout_test.go`.

Loopback pattern to copy for input integration (`fanout_test.go` 269-320): `newTracerServer(t, func(_ *tracerEnv, d *desktop.Deps){ d.OnDataChannel = ... })`, `env.join(ctx, t, env.token(t, "g1"), func(lc *loopbackClient){ lc.pc.CreateDataChannel("input", nil) })`. Package `desktop_test` (external) with `export_test.go` for seams such as `EngineSummaryForTest`. Phase 19 synthetic source plus `world` fake wall clock drives gates, dead-man (3 s) and rate-cap tests without sleeps. Tests that must not touch the host: `testing.Testing()` refusal exists in every real bridge (screencap) and `AllowLoopbackForTest` panics outside tests (peer.go 41-46).

Register new test files in TESTING.md Section 2 and Section 4 (standing rule; run `bash tests/check-traceability-paths.sh`).

---

### `frontend/src/lib/desktopClient.ts` (modify)

**Analog:** itself.

Data channel must exist before `createOffer` (RESEARCH Phase-19 dependency 3). Insert next to the transceiver (lines 366-373):
```ts
conn.addTransceiver('video', { direction: 'recvonly' })
await conn.setLocalDescription(await conn.createOffer())
await gathered(conn)
```
Add `const input = conn.createDataChannel('input', {ordered: true})` before the `createOffer` line (a channel in the offer is what yields `m=application`; the loopback test asserts it). Generation counter idiom: every callback checks `g === gen` (lines 363-366) so a superseded socket/peer cannot act; keep for the channel's `onopen/onclose/onmessage`.

Message handler (357-381): `handle(g, raw)` with `switch (m.type)` over `hello/answer/state/...`; add cases for `surfaces`, `epoch`, `clip_*`, `control_state` with the same typed-field narrowing (`typeof m.sdp === 'string'`). Client interface (82-88): `on<K extends keyof DesktopEvents>`; extend `DesktopEvents` (74-80) with `surfaces`, `epoch`, `clipboard`, and expose `sendInput(msg)` plus `send(msg)` for `select`/clipboard. Transport is injectable (`SignalTransport`, `TransportFactory`, lines 35-43), so tests stay fake-driven (see `lib/__tests__/desktopClient.test.ts`).

---

### `frontend/src/lib/desktopInput.ts` (utility, event-driven)

**Analog (structure):** `desktopClient.ts` (injectable deps and clock `DesktopClock`, `DesktopClientOptions` 46-61, gen counter) and `hostReconnect.ts` (deps argument defaulting to real bindings; 19-PATTERNS). Build `createDesktopInput({send, clock, doc, win, getRect, getEpoch, ...})` with real defaults so vitest can drive it with fakes.

**Analog (closed table):** `lib/ctrlMap.ts` 1-2: unknown key is dropped or sent as typed, never guessed. A client-side allow-list (`desktopKeymap.ts`) shared by tests mirrors the Go table.

**Analog (paste guard):** `lib/pasteGuard.ts` 1-17 `guardPasteInsert(textarea)`: `addEventListener('paste', e => e.preventDefault())` returning a remover; apply to the hidden IME textarea (RESEARCH Pattern 4):
```ts
export function guardPasteInsert(textarea: HTMLTextAreaElement | undefined): () => void {
  const stop = (e: Event) => e.preventDefault()
  textarea?.addEventListener('paste', stop)
  return () => textarea?.removeEventListener('paste', stop)
}
```
**Video rect:** normalize against `videoContentRect(elementWidth, elementHeight, videoWidth, videoHeight)` and the `onVideoRect` prop (DesktopView.tsx 17-30, 105-108, 142-153; the `resize` listeners on `<video>` and `window` are already there at lines 147-153).

New logic with no repo analog (use RESEARCH Pattern 4/6/R7): Cmd-chord tap rule, forwarded-down map, 1 s heartbeat while anything is held, blur/visibilitychange/pagehide release-all, rAF coalescing.

---

### `frontend/src/components/DesktopView.tsx` (modify)

**Analog:** itself. "One overlay at a time: the first of errors, state cards, tap card, Reconnecting chip, Connecting card, Locked chip, count chip wins" (19-06 SUMMARY). New chips (Control needs Accessibility, notices) join that precedence list rather than stacking. The component "Owns one DesktopClient and one `<video>` for the life of the page; no Wails imports, and view only: no pointer, key or context-menu handling" (docstring ~109-116): Phase 20 removes the "view only" part; keep no-Wails-imports (the page is served to browsers). Keep `videoRef.current?.setAttribute('muted','')` (Safari reads the attribute) and the `startPlay` Tap-to-start flow (lines ~156-166) untouched.

---

### `frontend/src/components/DesktopBar.tsx` and menus (component, event-driven)

**Analog:** `frontend/src/components/ContextMenu.tsx` (UI-SPEC line 49: "Extract the keyboard handling into a shared hook or reuse the component; do not copy it").

What to reuse:
- Item model (3-11): `{label, onSelect, separatorBefore?, disabled?, describedBy?}` with "Visible but inert: aria-disabled, dimmed, no highlight, skipped by arrows" (the greyed-while-Locked items need exactly this).
- Shell (38-103): portal to `document.body`, flip left/up to stay in viewport in `useLayoutEffect`, focus first non-disabled `[role="menuitem"]`, document mousedown outside closes, Esc closes, ArrowUp/Down roving via `querySelectorAll('[role="menuitem"]:not([aria-disabled="true"])')`, `invoker?.focus()` on close.
- Classes `.context-menu`, `.context-menu__item`, `.context-menu__separator`; add `.desktop-menu` modifier only for width/check column/captions (UI-SPEC 49).
Anchoring differs: `ContextMenu` takes a viewport point; bar menus pass the bottom-left of the trigger button (the component's doc says "or the bottom-left of the invoker for keyboard"; `isMenuKey` at 14-16 is the keyboard-open helper). Check-mark items (Display/Window picker selection, Swap toggle) need a small extension of the item type, not a fork.

Icons: `@heroicons/react/24/outline` only (UI-SPEC line 38).

---

### View Only / Control segmented control (component)

**Analog:** `SettingsSheet.tsx` 303-321 (copy verbatim shape):
```tsx
<div className="share-segbar settings-appearance" role="radiogroup" aria-label="Appearance" onKeyDown={handleAppearanceKeyDown}>
  {APPEARANCES.map(({ id, label }) => (
    <button key={id} ref={...} type="button" role="radio" aria-checked={appearance === id}
      tabIndex={appearance === id ? 0 : -1}
      className={appearance === id ? 'share-seg is-active' : 'share-seg'}
      onClick={() => ...}>{label}</button>
  ))}
</div>
```
Reuse `.share-segbar` / `.share-seg`, roving tabindex (`tabIndex -1` for the unselected; `useDialogFocus`'s focusables filter deliberately skips `tabindex=-1`), and arrow handling from `handleAppearanceKeyDown` (find in SettingsSheet.tsx; do not re-implement). Do not add `settings-appearance` (a Settings-specific modifier).

---

### Lock Screen confirm dialog (component, request-response)

**Analog:** `RegenerateKeyModal.tsx` (props 4-23: `title`, `body`, `confirmLabel`, `cancelLabel`, `titleId`, `destructive`; shell classes `.dialog-scrim`/`.dialog`) plus `lib/useDialogFocus.ts`:
```ts
useDialogFocus(ref, { onCancel, initialFocus: 'cancel', active })   // safe button focused first, Tab trapped, Esc = cancel
```
`ref` must point at the `aria-modal="true"` element; mark the primary with `data-dialog-primary` and the safe button with `data-dialog-cancel` (useDialogFocus.ts 27-28). The component supports copy overrides precisely so other guarded actions reuse it: instantiate with Lock Screen copy (UI-SPEC Surface D, "Don't Lock" cancel label) instead of a new component. Per UI-SPEC line 427, place it after the viewer in the DOM.

---

### Copy fallback and Paste popovers (component)

**Analog:** `SharePopover.tsx` (`role="dialog"` popover shell around line 273, `SharePopoverProps` 21+; `ReactPortal` return) for placement, focus and outside-click behaviour. Paste popover with its focused textarea additionally uses `guardPasteInsert`-style logic only to avoid expensive insertion of huge pastes, but here the user's paste into the textarea is the point (it exposes `clipboardData` without a permission prompt), so cap length at 1 MiB and read via the `paste` event instead of calling `guardPasteInsert` on it. Clipboard writes: call `navigator.clipboard.write([new ClipboardItem({'text/plain': promise})])` synchronously in the click handler (RESEARCH Pattern 8, Safari). No in-repo analog for ClipboardItem (see No Analog Found).

---

### Ctrl/Cmd swap persistence (utility, CRUD)

**Analog:** `NewSessionModal.tsx:15,32`:
```ts
const [selectedDir, setSelectedDir] = useState(() => localStorage.getItem(LAST_DIR_KEY) ?? '')
...
localStorage.setItem(LAST_DIR_KEY, path)
```
Use a namespaced key following `desktopClient.ts:455` (`'portty:desktop-debug'`): for example `portty:desktop-swap-ctrl-cmd`. Note: `desktopClient.ts` reads storage through the injected `win` (`win.localStorage?.getItem(...)`) so tests do not need a global; copy that injection. Missing key means off (CONTEXT: swap off by default). Wrap access like line 455 does, because Safari private mode can throw.

---

### `SettingsSheet.tsx` Desktop group and permissions strings (modify)

**Analog:** itself; follow `19-PATTERNS.md` section "SettingsSheet.tsx Desktop group" (`Row` at line 56, `HasCTDisclosure` ack, `settings-group`). Phase 20 adds one row showing what is being shared (UI-SPEC Surface E, 489-501) and updates pinned Phase 19 strings that change (UI-SPEC 518-530: update the tests asserting the old text, grep the test files for the old strings before editing). If the VM pasteboard row shows the paste-privacy alert, add the conditional "Paste from Other Apps" row to the permissions sheet (UI-SPEC 514).

---

### Tray line and daemon status (modify)

**Analog:** 19-08 `GET /desktop` status (`DesktopStatusResponse`) and the tray state line from 19-PATTERNS.md (`tray.go`/`tray_common.go`). Add the shared-surface text to the existing status payload as additive JSON fields; no new endpoint.

## Shared Patterns

### Fail closed, releases exempt from gates
**Source:** `internal/desktop/manager.go` + `state.go` `Manager.Locked()` (19-09: "true on a read error (fail closed for Phase 20)").
**Apply to:** `input.go` press and move paths. Releases of held items bypass lock, epoch, window and rate gates (RESEARCH Pattern 6, R8).

### Native bridge discipline
**Source:** `internal/screencap/sys_darwin.m` (`@try/@catch` to typed code, `kSysExceptionErr`), `sys_darwin.go` (`testing.Testing()` refusal, `sysErr`).
**Apply to:** `input_darwin.*`, `pasteboard_darwin.*`, any new SCK call. Crash containment: the daemon holds PTY masters, so an uncaught ObjC exception ends every terminal session (19-PATTERNS "Crash containment").

### Package-var seams and `SetXForTest`
**Source:** `internal/desktop/manager.go` 153-191 (`Deps` func fields, `default*` vars, `holdFunc` adapter).
**Apply to:** every new native read or action (poster, AX focus, pasteboard, surface list).

### Recover in every goroutine and callback
**Source:** `manager.go` 404 `recoverLog()`, used at 461 and `peer.go` 131.
**Apply to:** data-channel `OnMessage`/`OnClose`, dead-man timer, input goroutine, clipboard timeout goroutine.

### Read limit and split-around-it
**Source:** `internal/webserver/input_limit_test.go` 14-15 ("senders split around it (HARD-02)"), `internal/relay/protocol.go` 137-141, `internal/desktop/manager.go` 85 `readLimit = 32768`.
**Apply to:** clipboard chunks (cap by serialized frame length, about 16 KiB of text), viewer side splitting in `desktopClient.ts`.

### Closed tables
**Source:** `frontend/src/lib/ctrlMap.ts` header comment 1-2.
**Apply to:** `keytable.go`, `desktopKeymap.ts`, the button-index map, the window-list filter.

### Gate order on the share server
**Source:** `internal/webserver/desktop_routes.go` 149-152: `requireAllowedOrigin(requireSameUser(handleDesktopWS))`, token checked before Accept. No change expected: input rides the existing authenticated socket and the data channel of the already admitted viewer. A test should pin that an unauthenticated peer cannot open an `input` channel (it cannot join at all).

### TESTING.md standing rule
**Source:** repo `CLAUDE.md` and TESTING.md Section 6; `19-PATTERNS.md` "TESTING.md standing rule". Continue M-NN numbering after Phase 19's last row (M-172 was the last seen on this branch; re-check the tail of Section 5 at plan time).

## No Analog Found

| File / piece | Role | Data Flow | Reason |
|---|---|---|---|
| NSPasteboard read/write (types check, concealed skip) | bridge | request-response | No NSPasteboard code in repo; use RESEARCH Pattern 8. Style from `sys_darwin.m` |
| AX focus/raise ladder, window frame match (`AXUIElement*`, `NSWorkspace openApplication`) | bridge | request-response | Only `AXIsProcessTrusted` exists (`preflight_darwin.m`); use RESEARCH R1, R2, R13, Pattern 7 |
| Browser IME/composition capture, Cmd-chord tap rule, Keyboard Lock, `ClipboardItem` promise | utility | event-driven | No keyboard-capture code for a non-xterm surface; xterm owns key handling in the terminal. Use RESEARCH Patterns 4, 8 and Pitfall 7 |

## Metadata

**Analog search scope:** `internal/screencap`, `internal/desktop`, `internal/webserver` (desktop routes, input limit), `internal/relay` (input caps), `frontend/src/{lib,components}`, repo-root cgo files, 19-01..19-09 and 19-17 SUMMARY files.
**Not fully read** (grep and targeted ranges only): `media_darwin.m` (647 lines), `manager.go` (919 lines), `desktopClient.ts` (528 lines), `SettingsSheet.tsx`, `SharePopover.tsx`, `20-RESEARCH.md` lines 365-522 (validation and open-question tail), `20-UI-SPEC.md` (headings and reuse table only). Planner should open the cited ranges before copying.
**Pattern extraction date:** 2026-11-26
