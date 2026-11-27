---
phase: 21-portty-app-and-iphone-viewers
reviewed: 2026-11-27T00:00:00Z
depth: standard
files_reviewed: 64
files_reviewed_list:
  - app_clipboard_paste_test.go
  - app_clipboard_paste.go
  - app_peer_desktop_test.go
  - app_peer_desktop.go
  - app.go
  - build/darwin/Info.dev.plist
  - build/darwin/Info.plist
  - frontend/src/App.tsx
  - frontend/src/components/__tests__/App.desktop-tab.test.tsx
  - frontend/src/components/__tests__/DesktopPhoneBars.test.tsx
  - frontend/src/components/__tests__/DesktopPhoneLayer.test.tsx
  - frontend/src/components/__tests__/DesktopTab.test.tsx
  - frontend/src/components/__tests__/DesktopView.test.tsx
  - frontend/src/components/__tests__/Sidebar.test.tsx
  - frontend/src/components/__tests__/TabBar.test.tsx
  - frontend/src/components/ContextMenu.tsx
  - frontend/src/components/DesktopControlBar.tsx
  - frontend/src/components/DesktopPhoneBars.tsx
  - frontend/src/components/DesktopPhoneLayer.tsx
  - frontend/src/components/DesktopSendKeysMenu.tsx
  - frontend/src/components/DesktopTab.tsx
  - frontend/src/components/DesktopView.tsx
  - frontend/src/components/Sidebar.tsx
  - frontend/src/components/TabBar.tsx
  - frontend/src/lib/__tests__/controlMode.test.ts
  - frontend/src/lib/__tests__/desktopCoords.test.ts
  - frontend/src/lib/__tests__/desktopPhoneInput.test.ts
  - frontend/src/lib/__tests__/peerDesktop.test.ts
  - frontend/src/lib/__tests__/sidebarPeers.test.ts
  - frontend/src/lib/__tests__/tabs.test.ts
  - frontend/src/lib/__tests__/touchTrackpad.test.ts
  - frontend/src/lib/__tests__/visualViewportInset.test.ts
  - frontend/src/lib/controlMode.ts
  - frontend/src/lib/desktopCoords.ts
  - frontend/src/lib/desktopInput.ts
  - frontend/src/lib/desktopPhoneInput.ts
  - frontend/src/lib/peerDesktop.ts
  - frontend/src/lib/remoteSession.ts
  - frontend/src/lib/sidebarPeers.ts
  - frontend/src/lib/tabs.ts
  - frontend/src/lib/touchTrackpad.ts
  - frontend/src/lib/visualViewportInset.ts
  - frontend/src/style.css
  - frontend/src/wailsjs/go/main/App.d.ts
  - frontend/src/wailsjs/go/main/App.js
  - internal/daemon/api.go
  - internal/daemon/client.go
  - internal/daemon/desktop_caps_test.go
  - internal/daemon/desktop_caps.go
  - internal/daemon/desktop_peer_token_test.go
  - internal/daemon/desktop_peer_token.go
  - internal/daemon/peer_desktop_test.go
  - internal/daemon/peer_desktop.go
  - internal/daemon/relay_remote_files.go
  - internal/daemon/remote_desktop_ws_proxy_test.go
  - internal/daemon/remote_desktop_ws_proxy.go
  - internal/tailnet/desktop_meta_test.go
  - internal/tailnet/sessions.go
  - internal/webserver/desktop_peer_test.go
  - internal/webserver/server.go
  - internal/webserver/whois.go
  - window_occlusion_darwin_test.go
  - window_occlusion_darwin.go
  - window_occlusion_darwin.m
findings:
  critical: 0
  warning: 5
  info: 3
  total: 8
status: issues_found
---

# Phase 21: Code Review Report

**Reviewed:** 2026-11-27
**Depth:** standard
**Files Reviewed:** 64
**Status:** issues_found

## Summary

Reviewed the Go daemon/webserver/tailnet side of the peer desktop path (token store, `/peers/desktop`, loopback WS proxy, `/api/peer/desktop/cap`), the Wails bindings, the occlusion observer, and the frontend Desktop tab, shared viewer, input/trackpad engines and iPhone layer. Test files were only checked for registration in TESTING.md (`tests/check-traceability-paths.sh` passes, 313 paths) and the changed Go packages vet clean and pass their desktop/peer tests.

The security-critical path holds up. The token stays in `DesktopCapStore` (in memory, https-only baseURL enforced in `Put`), `/peers/desktop` returns no token, the `/peers/desktop/link` route is on the daemon socket only (the relay handler never mounts the daemon mux), the proxy strips the query and the raw token from every log line, close reasons are fixed literals, and `handlePeerDesktopCap` sits behind `requireSameUser` with the same JSON-content-type and no-CORS rules as `handlePeerCap`. The release path (`controlReducer`, `releaseAll` before every exit, layout-effect `tab-close` running before the viewer's passive cleanup) is sound. The findings are reliability and invariant gaps, not leaks. No BLOCKER-class defect was found.

## Warnings

### WR-01: A refused peer share port is classified as the terminal "Desktop sharing is off" state, so a peer daemon restart strands the tab

**File:** `internal/daemon/peer_desktop.go:90-95` (also `frontend/src/lib/peerDesktop.ts:38-49`, `frontend/src/components/DesktopTab.tsx:14-15`, `frontend/src/App.tsx:1039-1046`)
**Issue:** `connectPeerDesktop` maps both a 404 and `syscall.ECONNREFUSED` to `PeerErrDesktopOff` (409). On the web-view side `connectVerdict('desktop_off')` is `'off'`, and `desktopClient.classify()` turns `'off'` into `final('desktop_off')`, which calls `shutdown()` with no retry and no wake listeners. A connection refusal also happens whenever the peer's share server is simply not listening: the peer daemon restarting (Portty upgrade), or the first seconds after a Mac wake or boot. Sequence: the stream drops, the client redials, the proxy's dial to the peer is refused (closes 1013), `hello` is false so `classify()` runs `check()`, `check()` calls `ConnectPeerDesktop`, the daemon answers `desktop_off`, and the tab is final.

The `desktop_off` card has no action button (`DESKTOP_TAB_CARDS.desktop_off` has no `action`; `DesktopView` only renders buttons for `unreachable` and `token_rejected`). Clicking the sidebar Desktop row does not recover it either: `handleOpenDesktop` only calls `setActiveId` when the tab exists, although the UI-SPEC says "a click is the retest". The only way out is closing and reopening the tab.

The daemon already draws this distinction on the serving side (`desktop_api.go:295-301`: "On with no Manager yet ... is not off: 4410 is final for the viewer, 1013 makes it retry"). Project priority is reliability when nobody can reach the remote Mac, and VM-CHECKS row 10 (peer restart) was not run for this path. The sidebar poll only removes the row when the peer stops announcing `desktop`, which does not help an already-open tab.
**Fix:** Only a 404 is "off". Map `ECONNREFUSED` to a retryable code:
```go
case errors.Is(err, tailnet.ErrPeerDesktopOff):
    a.desktopCaps.Delete(fqdn)
    return peer, &peerDesktopFail{http.StatusConflict, PeerConnectErrorResponse{Error: PeerErrDesktopOff, Detail: short}}
case errors.Is(err, syscall.ECONNREFUSED):
    return peer, &peerDesktopFail{http.StatusBadGateway, PeerConnectErrorResponse{Error: PeerErrUnreachable, Detail: short}}
```
Also make a click on an existing Desktop row call the tab's reconnect (or give the final card a Reconnect action), so a final state is recoverable without closing the tab.

### WR-02: Cmd+V in control mode can still push clipboard text to the Mac after control mode has ended

**File:** `frontend/src/components/DesktopTab.tsx:117-142`
**Issue:** In `paste()`, `controlling.current` is checked only after `pasteToMac` completes, and only to decide whether to replay the key. Between the keydown and `clientRef.current?.pasteToMac(d.send)` there is an `await ReadHomeClipboard()` (a Go call that does a native pasteboard read of up to 1 MiB). If the user leaves control mode in that window (release chord, window blur, Cmd+Tab, tab switch, or the hidden-pause), the home clipboard text is still sent to the remote Mac's clipboard. That contradicts the stated goal that after control mode exits nothing is acted on at the Mac, and it is silent. The window is short, but the Mac is unattended and the payload is the user's clipboard.
**Fix:** Re-check right before the send, and again after it:
```ts
const d = pasteDecision(read, lastSent.current)
if (d.notice) return notice(d.notice)
if (!controlling.current) return            // control ended while the read was in flight
if (d.send !== null) { ... }
```

### WR-03: The bar's "Paste to Mac" reads the home clipboard with `navigator.clipboard.readText()` inside the app, bypassing the concealed-item check the app deliberately added

**File:** `frontend/src/components/DesktopView.tsx:814-836` (reached from `DesktopControlBar.tsx:334-338`); contrast `app_clipboard_paste.go:5-9`, `frontend/src/components/DesktopTab.tsx:94-95`
**Issue:** `app_clipboard_paste.go` and the `DesktopTab` comment state that the types-first Go bridge is the only home-clipboard read because a plain-text read "cannot see" concealed or transient pasteboard items, and that "this is the only place the home clipboard is read". But `DesktopView` is shared and embedded, and `showBar` is true in the app tab (`canControl && cardKey === null ...`; `embedded` does not suppress it). So Clipboard > Paste to Mac in the app calls `navigator.clipboard.readText()` and sends the text with no concealed check. A password-manager item on the home clipboard would be sent to the office Mac's clipboard on that menu click. It is an explicit menu action, so this is lower risk than the Cmd+V path, but the app now has two different home-clipboard read paths with different protections, and the documented invariant is false.
**Fix:** Give `DesktopView` an optional `readClipboard?: () => Promise<{outcome, text}>` prop. `DesktopTab` passes a wrapper over `ReadHomeClipboard` and `pasteToMac()` uses it when present (honouring `concealed` and `too_large` with `PASTE_NOTICES`). Or, embedded, hide the bar's Paste item and leave Cmd+V as the single path.

### WR-04: `rights` after the first hello are ignored, so a downgrade on reconnect leaves Control, surface and clipboard UI live

**File:** `frontend/src/components/DesktopView.tsx:337-390`
**Issue:** `client.on('rights')` returns early when `inputCtl.current` already exists, and `setCanControl`, `setCanSurface` and `setCanClipboard` are only ever set to `true` inside the one-time creation branch. The client emits `rights` on every hello (`desktopClient.ts:652`), so after a reconnect where the grant lost control, surface or clipboard (for example the host changed what is allowed), the viewer keeps the input controller with `control = true`, the Control segment, the picker and the Clipboard menu. Input is still enforced by the daemon, so this is not an authorization bypass. The result is a viewer that offers actions that silently do nothing, with no card or notice.
**Fix:** Handle every `rights` event. When `!r.control` and a controller exists, call `inputCtl.current.setControl(false)` and `setCanControl(false)`. Update `setCanSurface(r.surface)` and `setCanClipboard(r.clipboard)` on every event, not only at creation.

### WR-05: Sidebar Desktop row state outlives its tab and is erased by a single failed probe

**File:** `frontend/src/App.tsx:811-819, 1048-1059`
**Issue:** `desktopStates` is only written by `DesktopTab` while mounted. Closing the tab never clears its entry (`DesktopView` cleanup does not call `onDesktopState(null)`, and `App` has no close hook for it), so the row keeps a stale muted line ("Needs permission", "Locked") for as long as the peer announces `desktop`, with no connection behind it. In the other direction, the poll prunes any entry whose peer is not currently `desktop === true`. `FetchAllPeerSessionsMeta` reports `Desktop: false` for any probe that is not `MetaOK` (a 5 s timeout is enough), so one flaky poll drops a still-true "Locked" line, and it is not re-sent because the Mac only sends state on change. `desktopLabels[tabId]` has the same leak: a reopened tab with the same id briefly shows the previous label.
**Fix:** In the tab-close path (and `DesktopTab`'s unmount) call `handleDesktopState(connectHost, null)` and delete `desktopLabels[tabId]`. Prune in the poll only when the peer is reachable and `desktop !== true`, not when the probe failed.

## Info

### IN-01: `useReveal` keeps document-level pointer listeners and a flash timer alive in a background (inert, opacity-0) Desktop tab

**File:** `frontend/src/components/DesktopControlBar.tsx:209-228`, `frontend/src/App.tsx:1494-1517`
**Issue:** A Desktop tab behind another tab stays mounted and streaming (by design), including its control bar. `useReveal` registers `document` `pointermove` and a capture-phase `pointerdown` that calls `preventDefault()` and `stopPropagation()` for presses inside the hidden bar's box (`y` from 0 to bar bottom plus 8, so it covers the app's own tab strip and toolbar over that horizontal span). While the bar is not "shown" (`shownRef` false), a press there that is not preceded by a pointermove in the region (touch, pen, a synthetic or keyboard-driven click) is swallowed in whatever tab is active. A mouse normally produces a preceding move that sets `hot`, so the practical risk is small, but the listener should not be live for a tab that is not the active one.
**Fix:** Pass an `active` flag (the tab's `active` prop through `DesktopView` to the bar) and register the document listeners only while active, or bail out of `onDown` and `onMove` when inactive.

### IN-02: Two independent implementations of the peer-host key normalisation

**File:** `internal/daemon/desktop_caps.go:42-44` and `app_peer_desktop.go:30-32`
**Issue:** `desktopCapKey` (daemon) and `desktopHostKey` (app) both lower-case, trim whitespace and strip one trailing dot, and the app builds the loopback URL from its copy while the daemon looks up with its own. They agree today. If either drifts (for example one starts collapsing more than one trailing dot), the proxy lookup fails with "no desktop token registered", which the web view reports only as unreachable.
**Fix:** Export one helper from `internal/daemon` (for example `daemon.DesktopHostKey`) and call it from `app_peer_desktop.go`.

### IN-03: `desktopInput.destroy()` does not release held input

**File:** `frontend/src/lib/desktopInput.ts:488-513`
**Issue:** `destroy()` removes listeners and clears the heartbeat timer but does not call `releaseAll`. With buttons or keys held, the 1 s heartbeat stops and the Mac relies on the daemon's dead-man timeout. The Desktop tab does release first (the `tab-close` layout cleanup runs before the viewer's passive cleanup), so this is not reachable in the app. It is a latent hole for any other caller (the browser page's unmount, an `ErrorBoundary` remount) given the "nothing left held" requirement.
**Fix:** Call `releaseAll('disconnect')` at the top of `destroy()` when `held() > 0` (or unconditionally, since a release message is idempotent).

---

_Reviewed: 2026-11-27_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
