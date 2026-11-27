---
phase: 20-remote-control
reviewed: 2026-11-27T03:58:32Z
depth: deep
files_reviewed: 75
files_reviewed_list:
  - app_desktop_test.go
  - app_desktop.go
  - frontend/src/components/__tests__/ContextMenu.test.tsx
  - frontend/src/components/__tests__/DesktopClipboardPopover.test.tsx
  - frontend/src/components/__tests__/DesktopControlBar.test.tsx
  - frontend/src/components/__tests__/DesktopSendKeysMenu.test.tsx
  - frontend/src/components/__tests__/DesktopSetupSheet.test.tsx
  - frontend/src/components/__tests__/DesktopView.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx
  - frontend/src/components/ContextMenu.tsx
  - frontend/src/components/DesktopClipboardPopover.tsx
  - frontend/src/components/DesktopControlBar.tsx
  - frontend/src/components/DesktopSendKeysMenu.tsx
  - frontend/src/components/DesktopSetupSheet.tsx
  - frontend/src/components/DesktopView.tsx
  - frontend/src/components/SettingsSheet.tsx
  - frontend/src/lib/__tests__/desktopClient.test.ts
  - frontend/src/lib/__tests__/desktopInput.test.ts
  - frontend/src/lib/__tests__/peerDesktop.test.ts
  - frontend/src/lib/desktopClient.ts
  - frontend/src/lib/desktopInput.ts
  - frontend/src/style.css
  - frontend/src/wailsjs/go/main/App.d.ts
  - internal/daemon/desktop_api_test.go
  - internal/daemon/desktop_api.go
  - internal/daemon/desktop_permissions_test.go
  - internal/daemon/desktop_probe_vmdesktop.go
  - internal/desktop/clipboard_export_test.go
  - internal/desktop/clipboard_test.go
  - internal/desktop/clipboard.go
  - internal/desktop/control_golden_test.go
  - internal/desktop/control_test.go
  - internal/desktop/control.go
  - internal/desktop/export_input_test.go
  - internal/desktop/export_test.go
  - internal/desktop/fanout_test.go
  - internal/desktop/gates.go
  - internal/desktop/input_map_test.go
  - internal/desktop/input_map.go
  - internal/desktop/input_test.go
  - internal/desktop/input.go
  - internal/desktop/manager.go
  - internal/desktop/probe_vmdesktop.go
  - internal/desktop/protocol_test.go
  - internal/desktop/surface_export_test.go
  - internal/desktop/surface_test.go
  - internal/desktop/surface.go
  - internal/desktop/testdata/input-v1.jsonl
  - internal/desktop/testdata/signal-v1.jsonl
  - internal/desktop/viewer.go
  - internal/desktop/window_confine_test.go
  - internal/desktop/window_export_test.go
  - internal/desktop/window_test.go
  - internal/desktop/window.go
  - internal/screencap/input_darwin_test.go
  - internal/screencap/input_darwin.go
  - internal/screencap/input_darwin.m
  - internal/screencap/keytable_test.go
  - internal/screencap/keytable.go
  - internal/screencap/media_darwin.go
  - internal/screencap/media_darwin.m
  - internal/screencap/pasteboard_darwin_test.go
  - internal/screencap/pasteboard_darwin.go
  - internal/screencap/pasteboard_darwin.m
  - internal/screencap/screencap_darwin_test.go
  - internal/screencap/screencap.go
  - internal/screencap/surfaces_test.go
  - internal/screencap/surfaces.go
  - internal/screencap/sys_darwin.m
  - internal/screencap/window_darwin_test.go
  - internal/screencap/window_darwin.go
  - internal/screencap/window_darwin.m
  - TESTING.md
  - tray_common_test.go
  - tray_common.go
findings:
  critical: 0
  high: 2
  medium: 7
  low: 6
  info: 2
  warning: 13
  total: 17
status: issues_found
---

# Phase 20: Code Review Report

**Reviewed:** 2026-11-27T03:58:32Z
**Depth:** deep (cross-file: viewer -> data channel -> input goroutine -> gates -> cgo bridge; daemon shutdown order)
**Files Reviewed:** 75 in scope
**Status:** issues_found

## Summary

I read every line of the files that carry the security and reliability load: `internal/desktop/{input,gates,control,window,surface,clipboard,input_map,viewer}.go`, the Phase 20 diff of `manager.go`, `internal/screencap/{input,pasteboard,window}_darwin.{m,go}`, `keytable.go`, the Phase 20 diff of `media_darwin.m`, `frontend/src/lib/{desktopInput,desktopClient}.ts`, how `DesktopView.tsx` wires releases, and the Phase 20 diffs of `desktop_api.go`, `app_desktop.go` and `tray_common.go`. I also traced the daemon's shutdown path (`internal/daemon/api.go`, `desktop_api.go`) and the `/desktop/ws` route gating (`internal/webserver/desktop_routes.go`). I only skimmed the test files, `style.css`, `TESTING.md`, `SettingsSheet.tsx`, `DesktopSetupSheet.tsx`, `ContextMenu.tsx` and the control-bar components, looking for dangerous patterns (no `innerHTML`, no logging of clipboard text, no use of `console`).

Checks run: `GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 ./internal/desktop/ ./internal/screencap/` passes. `vitest run` on desktopInput, desktopClient and DesktopView passes (265 tests). Nothing was run against the host's input, AX, capture or pasteboard.

Most of the design holds up when traced through:
- Releases skip every gate.
- Held state is recorded before the post, under the same lock that release snapshots take.
- The epoch bump plus flush ordering in `applySurface` is correct.
- `/desktop/ws` is Origin, then same-user, then token, before the upgrade.
- Clipboard reassembly is bounded.
- The JSON-escape budget on both sides keeps frames under 32768 bytes.
- `pcEncodeStill`'s release handshake is correct.

The problems are at the edges.

The most serious: **the daemon's real shutdown order kills the input goroutine before `Manager.Close` queues its releases, so held keys and buttons are never let go on quit or handoff (H-01).** Second, **the viewer can keep a key held forever because of the macOS Cmd key-up quirk, and its heartbeat defeats the dead-man (H-02).** Both break the standing rule that no key or button may stay held.

Necessity review (#4100) was not performed: the reference was not loaded within this run's budget, so `necessity:` is omitted.

Severity mapping for GSD consumers: high counts as critical-tier for blocking purposes, at the caller's discretion. Medium and low are `warning`. Info is `info`.

## High

### H-01: Daemon shutdown cancels the input goroutine before Close posts the releases, so held input stays held on quit and handoff

**Files:** `internal/daemon/desktop_api.go:579-587`, `internal/desktop/manager.go:487`, `internal/desktop/input.go:885-911`
**Severity:** high (BLOCKER: breaks the rule that nothing may stay held)
**Evidence:**
- `stopDesktop()` runs `a.desktopCancel()` and only then calls `mgr.Close()`. Both shutdown paths call it (`api.go:400`, `api.go:454`).
- `Manager.Start(a.desktopCtx)` derives the input loop's context from that same context: `ictx, icancel := context.WithCancel(ctx)` (`manager.go:487`). So `inputLoop` returns as soon as `desktopCancel()` runs.
- `Close()` then calls `releaseAllViewers("closing")`. That queues one release per held key or button into `m.in.rels`, plus a flush marker. It sees `m.in.done != nil` (still set), waits `flushWait` (1 s), and logs "input releases not posted before close". No goroutine is left to post the releases.
- Viewers' `leave()` calls during `CloseAll` queue into the same dead loop.
- Result: a Cmd, Shift or mouse button held by a viewer at the moment of Quit All, an upgrade handoff or a daemon stop stays down in the WindowServer. On an unattended Mac that means a stuck modifier or drag state that nobody can clear.
- Tests miss this because they start with a context that is never cancelled before `Close`.

**Fix:** Close should be the only thing that ends the input goroutine. Either:
```go
// manager.go Start
ictx, icancel := context.WithCancel(context.Background()) // Close cancels it after the final flush
```
or, in `Close`, after `<-idone`, drain whatever is left in `m.in.rels` synchronously with `m.postRelease(r)`. Add a test that cancels the Start context before `Close` while a key is held and asserts that the key-up was posted.

### H-02: A key pressed before Cmd and released while Cmd is down is never released, and the heartbeat keeps the dead-man from firing

**Files:** `frontend/src/lib/desktopInput.ts:198-204`, `:317-334`
**Severity:** high (BLOCKER: a key stays held with no time limit)
**Evidence:**
- `onKeyDown` puts every non-tap key into `forwarded`. Only keys pressed *while* `ev.metaKey` is true are sent as a down+up tap (`:317-321`).
- macOS (`-[NSApplication sendEvent:]`) drops the keyUp of any key released while Command is down, so the browser never fires `keyup`. Sequence: A down (forwarded, daemon holds kVK_ANSI_A), Cmd down, A up (swallowed), Cmd up. `forwarded` still holds `KeyA`.
- `held() > 0`, so `syncHeartbeat` keeps sending `hb` every second. The daemon's `inputTick` dead-man (`input.go:396-425`) only fires after 3 s of silence, so it never does.
- `reconcileModifiers` (`input.go:758-780`) only repairs the four modifier bits, never ordinary keys.
- The key stays down on the Mac until the viewer blurs, presses the same key again, or uses the release chord. This is common in fast typing, for example "...s" followed quickly by Cmd+S.

**Fix (viewer):** when a `MetaLeft` or `MetaRight` keydown is forwarded (and again on its keyup), send a key-up for every forwarded non-modifier code and remove it from `forwarded`. Belt and braces on the daemon: in `handleKey`, when a Command key-up is handled, release every held non-modifier key of that viewer. Add a vitest for the A, Cmd, (no A up), Cmd-up sequence.

## Medium

### M-01: A failed surface switch can leave the mapping on a surface the viewer is not seeing

**Files:** `internal/desktop/manager.go:791-796`, `internal/desktop/surface.go:323-343`
**Severity:** medium
**Evidence:**
- (a) `startSource`: when a select ran during the start, `src.SelectSurface(cur)` is tried after the start. If it fails, the code only logs. `m.surf.sel` stays at `cur` (for example a window), but the stream shows `cfg.Surface` (for example the main display). No epoch bump and no `surface` message follow, so viewers keep sending the current epoch, and `target()` maps their clicks onto the window's frame while they look at the display. Clicks land in the wrong place.
- (b) `applySurface` with a non-empty `reason` records the fallback even when `SelectSurface` fails (`err == nil || reason != ""`) and broadcasts the main display. The source is still on the vanished surface (frozen or black), and normalized clicks map onto the main display.

**Fix:** For (a), on failure call `m.setSurface(cfg.Surface, …)`, `m.bumpEpoch()` and broadcast `surfaceMsg("")` (or fail the start). For (b), when a fallback switch fails, either restart the source on the main display or refuse all presses (`target().ok = false`) until a switch succeeds, rather than mapping onto a surface that is not on screen.

### M-02: The same-app pop-up carve-out also lets clicks through to the app's other document windows, and to UI the capture does not show

**File:** `internal/desktop/window.go:225-238` (commit 78ea4fc8)
**Severity:** medium
**Evidence:**
- The comment and commit say "the app's other document windows … still drop as covered". But `sameAppPopup` accepts any layer-0 window of the same pid that lies entirely inside the shared frame. A smaller second Terminal or TextEdit window sitting over the shared one passes. The click lands in a window the viewer cannot see (`initWithDesktopIndependentWindow:` captures only the shared window).
- `top.Layer != 0` also accepts negative layers, which contradicts "any layer above 0".
- 20-VM-CHECKS W3 records that sheets and pop-ups are separate windows, and that single-window capture likely excludes them (not confirmed). So the carve-out lets clicks reach menus the viewer may not see at all. X1 was not re-run after the fix.

**Fix:** Decide with AX, not geometry. Use `AXUIElementCopyElementAtPosition(app, x, y)`, walk up to the `AXWindow`, and accept only when it is the shared window or its role or subrole is `AXSheet`, `AXDialog`, `AXSystemDialog`, `AXMenu` or `AXPopover` (or the element is a menu). Use `top.Layer > 0`. Add an M-NN or VM row that confirms what the decoded frame shows for a pop-up before shipping the carve-out.

### M-03: Resizing the shared window releases held input mid-drag, so a remote drag-resize is cut within a second

**File:** `internal/desktop/surface.go:438-463`
**Severity:** medium
**Evidence:**
- `surfaceTick` treats a change of 1 pt or more in W or H as a reason to call `applySurface(ctx, sel, cur, "")`. That bumps the epoch and calls `releaseHeldAll("surface")`, which posts MouseUp for the held button.
- When a viewer drags the corner of the shared window, the next 1 s poll ends the drag. Every move after that is dropped as `stale_epoch` until the new `surface` message arrives, and the viewer shows "Held keys were released".
- Coordinates are normalized, so a size-only change on the same surface does not invalidate them.

**Fix:** For a same-surface resize, reconfigure the encoder without bumping the epoch or releasing held input. At minimum, defer the resize apply while any viewer holds a button.

### M-04: The control-effect check falsely trips `needs_accessibility` when moves land on off-screen parts of the shared window

**Files:** `internal/desktop/control.go:134-183`, `internal/desktop/input.go:579-580`, `internal/desktop/surface.go:193-202`
**Severity:** medium
**Evidence:**
- In a window share, `target()` is the full CGWindow frame, and `mapPoint` maps onto it even where the window extends past a display edge (a common position).
- The WindowServer clamps the cursor to the display. The 60 ms read-back then differs from the posted point and counts as a miss. Three misses in a row call `setControlInput(ControlNeedsAccessibility)`.
- After that, `pressAllowed` drops every press as `no_accessibility` and the viewer pauses, until the next `recoverControl` nudge (up to 5 s later). Hovering near the off-screen edge repeats this.
- A physical user moving the mouse during the 60 ms window has the same effect.

**Fix:** Skip the effect check, or clamp the posted point, when the mapped point is outside the union of `CGGetActiveDisplayList` bounds. Better, intersect the window target with the visible display area before mapping moves.

### M-05: The window-share key gate can be bypassed by global hotkeys and non-activating panels, and its blocklist is narrow

**Files:** `internal/desktop/gates.go:73-98`, `internal/desktop/window.go:142-147`, `internal/screencap/window_darwin.m:63-96`
**Severity:** medium (unverified on a VM)
**Evidence:**
- Keys pass when the AX `kAXFocusedApplicationAttribute` pid matches and its focused (or main) window frame is within 2 pt of the shared frame.
- The chord blocklist covers only Cmd+Tab, Cmd+Space and Ctrl+Cmd+Q. Option+Space (Alfred or Raycast), Cmd+Shift+5, Ctrl+arrows (Spaces) and Cmd+Opt+Esc all pass.
- Launchers open non-activating `NSPanel`s, which take key focus without changing the frontmost or focused application. Every keystroke after that can pass the gate yet land in a panel the window capture does not show. This is the PITFALLS 11 risk this gate exists for.
- The `kAXMainWindowAttribute` fallback (`:84-86`) and frame-only matching also accept a different same-app window with an identical frame.

**Fix:** Also read the system-wide `kAXFocusedUIElementAttribute`, walk to its `AXWindow`, and require that window to match the shared frame and title (fail closed on no value). Consider refusing presses while any window above layer 0 from another pid is key. Add a VM row: window share, then Option+Space with a launcher installed, then type.

### M-06: The pasteboard concealed check is check-then-read (TOCTOU)

**File:** `internal/screencap/pasteboard_darwin.m:54-79`
**Severity:** medium
**Evidence:**
- `[pb types]` is checked for the Concealed, Transient and 1Password markers.
- Then `[pb stringForType:]` reads the current contents.
- If a password manager writes a concealed secret between the two calls, the secret is read and sent to the viewer. The window is small, but the whole point of the check is to never read concealed content.

**Fix:** Snapshot `NSInteger cc = pb.changeCount` before the type check and compare after `stringForType:`. On a mismatch, discard the string and answer failed or retry once. Reading `changeCount` is not polling. Alternatively, take `pb.pasteboardItems.firstObject` once and check `item.types` and `[item stringForType:]` on that same item.

### M-07: No `@autoreleasepool` in cgo ObjC entry points that run on long-lived Go threads

**Files:** `internal/screencap/pasteboard_darwin.m:41-103`, `internal/screencap/window_darwin.m:137-302`, `internal/screencap/media_darwin.m` (`pc_list_surfaces`, `pc_stream_select`, `pcEncodeStill`)
**Severity:** medium (an unbounded leak in a daemon meant to run for weeks)
**Evidence:**
- Go threads have no autorelease pool. Objects autoreleased on them go to the lazily created per-thread pool, which drains only when the thread exits, and Go threads do not exit.
- Paths that autorelease, and how often they run:
  - `pc_list_surfaces` (`SCShareableContent`, collection and dictionary literals, `NSJSONSerialization`): every second from `surfaceTick` while a window or non-main display is shared.
  - `pc_window_minimized` (`stringWithUTF8String:`): 5 times a second from `windowTick` while the shared window is minimized.
  - `pc_window_raise` (`NSRunningApplication`, `NSWorkspaceOpenConfiguration`).
  - `pc_pb_read` (pasteboard types and up to 1 MiB of string internals per Copy).
- The repo's own ObjC files (`tray_objc_darwin.m`, `trash_darwin.m`) use `@autoreleasepool`. None of the `internal/screencap/*.m` files do.

**Fix:** Wrap the body of every exported `pc_*` function in `@autoreleasepool { … }`, inside the `@try`.

## Low

### L-01: `win.mu` is documented as a leaf lock but is held across control pushes and data-channel sends

**File:** `internal/desktop/window.go:254-279` (also `:66` comment)
**Severity:** low
**Evidence:** The `startRaise` timer and goroutine call `setControlWindow` and `clearWindowState` while holding `m.win.mu`. That takes `ctl.mu` and `vmu` and calls `dc.SendText` for every viewer. Meanwhile `cachedFrame` and `clampToWindow` (called for every move and every button release on the input goroutine) wait on `win.mu`. There is no deadlock today, but the release path waits behind network I/O, and the invariant the comment states is false.
**Fix:** Compute `fail := m.win.raising && m.win.gen == gen` (and the result in the goroutine) under the lock, unlock, then call `setControlWindow` or `clearWindowState`.

### L-02: Held Cmd chords do not auto-repeat

**Files:** `frontend/src/lib/desktopInput.ts:316-320`, `internal/desktop/input.go:691-694`
**Severity:** low
**Evidence:** A tap is sent as a down with `r: ev.repeat`, immediately followed by an up. On repeats the daemon sees `R=true` for a key it no longer holds and drops it as `not_held`. Holding Cmd+Z, Cmd+Delete or Cmd+= fires once.
**Fix:** Send `r: false` on the tap's down. Each tap is a complete press.

### L-03: Slow select and clipboard handlers block the signalling read loop and can trip the viewer's wake ping

**Files:** `internal/desktop/manager.go:695-708`, `frontend/src/lib/desktopClient.ts:765-783`
**Severity:** low
**Evidence:**
- `handleSelect` runs inline on the viewer's read loop: up to 5 s for the list, plus `pc_stream_select` (up to 15 s to resolve, 5+5 s to update, 1 s wait, 5 s still).
- `handleClipGet` waits up to 2 s.
- A wake ping during that time (3 s `pingTimeoutMs`) tears down a healthy session.

**Fix:** Run select and clipboard work on a per-viewer worker goroutine, keeping pings on the read loop. Or have the client skip or extend the ping deadline while a switch or clipboard transfer is pending.

### L-04: Release-type messages bypass the per-viewer queue cap

**File:** `internal/desktop/input.go:337-347`
**Severity:** low
**Evidence:** `!msg.isRelease() && len(vi.fifo) >= maxFifo` exempts every key-up, button-up and `release` from the 1024 cap. A misbehaving same-user viewer can grow the queue without bound. Only own-account viewers can open the channel today. Phase 22 guests make this more relevant.
**Fix:** Collapse consecutive `release` messages, and at enqueue time drop key or button ups for things the viewer does not hold. Keep a hard upper cap (for example 4×maxFifo) even for releases.

### L-05: Logout chords are not guarded, though they are as unrecoverable as Lock Screen

**File:** `internal/desktop/gates.go:48-71`
**Severity:** low
**Evidence:**
- Lock Screen gets a confirm dialog because a remote unlock is not possible yet.
- Cmd+Opt+Shift+Q (log out immediately) and Cmd+Shift+Q then Return end the Aqua session and the launchd agent with it. That is the same "nobody can get in to fix the remote Mac" outcome, and nothing guards these chords in a display share.

**Fix:** Refuse Cmd+Opt+Shift+Q on the daemon (counted as `window_chord`-style `guarded_chord`), or route it through the Send-keys confirm like Lock Screen.

### L-06: `gConfig` is mutated during a switch while the capture handler reads it

**File:** `internal/screencap/media_darwin.m` (`pcApplySurface` ~628-650, handler ~476-480, `pc_stream_select` ~717-721)
**Severity:** low
**Evidence:**
- `pcApplySurface` writes `gConfig.width/height` on the Go thread, while `didOutputSampleBuffer` reads them on `gQueue` to update `gEncoded`. This is a data race, and the "new surface delivered a frame" test can miscount, forcing or skipping the still frame.
- If the rollback `rebuildWidth:oldW` fails, `gConfig` keeps the new size while the stream runs the old filter.

**Fix:** Keep the expected width and height in `_Atomic` ints set by the switch, and compare against those in the handler. Restore `gConfig` dimensions on every rollback path.

## Info

### I-01: `clipResult` slices the viewer ID without a bound

**File:** `internal/desktop/clipboard.go:109`
**Severity:** info
**Evidence:** `string(v.ID)[:8]` panics on an ID shorter than 8 characters. `endViewerInput` uses `min(8, len(v.ID))`. IDs are generated today, so this is latent.
**Fix:** Use the same `min(8, len)` form.

### I-02: Composed text is posted on virtual key 0 (`kVK_ANSI_A`)

**File:** `internal/screencap/input_darwin.m:91-96`
**Severity:** info
**Evidence:** `CGEventCreateKeyboardEvent(src, 0, …)` with a Unicode override. Apps that read the key code, and an active IME on the Mac (for example Japanese input), may treat it as "a".
**Fix:** Document it and add a VM or M-NN row with a non-Latin input source active. Consider a key code that has no character, such as `kVK_F20` (0x5A), as the carrier.

---

_Reviewed: 2026-11-27T03:58:32Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
