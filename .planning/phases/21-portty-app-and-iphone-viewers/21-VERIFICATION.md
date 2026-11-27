---
phase: 21-portty-app-and-iphone-viewers
verified: 2026-11-27T12:00:00Z
status: human_needed
score: 4/5 must-haves verified
covered_files:
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-01-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-01-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-02-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-02-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-03-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-03-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-04-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-04-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-05-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-05-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-06-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-06-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-07-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-07-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-08-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-08-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-09-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-09-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-10-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-10-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-11-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-11-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-12-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-12-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-13-PLAN.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-13-SUMMARY.md"
  - ".planning/phases/21-portty-app-and-iphone-viewers/21-VM-CHECKS.md"
  - "app_clipboard_paste.go"
  - "app_peer_desktop.go"
  - "build/darwin/Info.plist"
  - "frontend/src/App.tsx"
  - "frontend/src/components/DesktopPhoneBars.tsx"
  - "frontend/src/components/DesktopPhoneLayer.tsx"
  - "frontend/src/components/DesktopTab.tsx"
  - "frontend/src/components/DesktopView.tsx"
  - "frontend/src/components/Sidebar.tsx"
  - "frontend/src/components/TabBar.tsx"
  - "frontend/src/lib/controlMode.ts"
  - "frontend/src/lib/desktopPhoneInput.ts"
  - "frontend/src/lib/peerDesktop.ts"
  - "frontend/src/lib/sidebarPeers.ts"
  - "frontend/src/lib/touchTrackpad.ts"
  - "internal/daemon/desktop_caps.go"
  - "internal/daemon/desktop_peer_token.go"
  - "internal/daemon/peer_desktop.go"
covered_digest: "v4:sha256:a91fbacf1cc9c45f9efd8885b4cdfcfcb21bbee012c17efe7b2ef6a072f0cd27"
behavior_unverified: 1
overrides_applied: 0
behavior_unverified_items:
  - truth: "On an iPhone the viewer offers trackpad mode, pinch-zoom, long-press right-click, two-finger scroll and the soft keyboard, and picks up again after the phone sleeps (DVWR-03)"
    test: "M-194..M-200 (TESTING.md Section 5 Category BL) on a real iPhone in Safari and from the Home Screen"
    expected: "One finger moves the pointer, tap and double-tap click, press-and-hold right-clicks with no iOS callout, two fingers scroll, pinch zooms to 5x, the soft keyboard types, and the picture returns after a 2-minute sleep"
    why_human: "The gesture state machine (touchTrackpad.ts, desktopPhoneInput.ts) is unit-tested with a fake clock and the phone layer is wired into DesktopView, but no test or VM drives a real iOS touch screen, Safari callouts, the soft keyboard or iOS sleep and resume. DVWR-03 has no VM evidence at all."
human_verification:
  - test: "M-194 iPhone one-finger trackpad, tap, double-tap, press-and-hold right click with no iOS callout"
    expected: "Pointer moves like a trackpad; tap clicks; double-tap double-clicks and selects a word; long press right-clicks with no callout"
    why_human: "Real touch input and iOS Safari callout behaviour"
  - test: "M-195 iPhone tap-then-drag a window, two-finger scroll direction"
    expected: "Window moves by its title bar; two fingers scroll in the direction the fingers move"
    why_human: "Real touch input"
  - test: "M-196 iPhone pinch to 5x, pan-follow, reset, no Safari tab overview"
    expected: "Pinch zooms to 5x and back; two-finger double-tap and Fit to Screen reset; Safari tab overview never opens"
    why_human: "Real touch input and Safari gesture conflicts"
  - test: "M-197 iPhone soft keyboard, Backspace, Return, armed modifiers"
    expected: "Typing, Backspace and Return work; an armed modifier sends a chord; the form-assist bar gap is the accepted one"
    why_human: "iOS soft keyboard behaviour"
  - test: "M-198 iPhone Send Keys, Lock Screen confirm, surface picker"
    expected: "Cmd+Tab goes to the Mac; Lock Screen asks first; picker lists the office Mac's displays"
    why_human: "Real phone UI against a real Mac"
  - test: "M-199 iPhone sleep and resume"
    expected: "After 2 minutes asleep the picture comes back (Reconnecting, or the Tap to Start card only when iOS needs the tap)"
    why_human: "iOS sleep/resume and autoplay policy"
  - test: "M-200 iPhone Home Screen relaunch and CSP"
    expected: "Icon keeps the same link; Safari console shows no CSP violation with the video element"
    why_human: "Home Screen web app and Safari console"
  - test: "M-201 two real Macs: Desktop row, Local Network prompt, sharp picture, direct tailnet path"
    expected: "Row opens the office Mac in a tab; first-run prompt appears and Allow heals the tab; picture is sharp; path is direct"
    why_human: "Real Local Network permission, Retina sharpness and tailnet path (VM proved only the flow, not sharpness or direct path)"
  - test: "M-202 control mode on real Macs: Cmd+W, Cmd+Q, Cmd+C, release chord, plain click after"
    expected: "Chords act on the office Mac only; Ctrl+Option+Cmd+Shift+Return gives Portty its shortcuts back; the next click is a plain click (VM K6 saw a Control key held after the release chord once, not reproduced, suspected rig artefact)"
    why_human: "Real keyboards and the unresolved K6 observation"
  - test: "M-203 Cmd+V in control mode including a password-manager item"
    expected: "Home clipboard text pastes into the office Mac; a concealed item is refused with the hidden-item notice; no macOS paste alert"
    why_human: "Real pasteboard and real password manager (VM used a synthetic concealed item)"
  - test: "M-204 hidden window lets the office display sleep after about 2 minutes, on battery and on power"
    expected: "Display sleeps about 2 minutes after the window is hidden; reopening brings the picture back"
    why_human: "Real display sleep and power state (VM proved the viewer release and assertion drop only)"
  - test: "M-205 label, chips and cards legible in Light and Dark over the translucent window"
    expected: "Toolbar label, chip and cards legible in both appearances"
    why_human: "Visual judgement; VM measured contrast only (4.77 to 16.83 Light, 4.97 to 11.73 Dark)"
  - test: "Re-check the five code-review fixes WR-01..WR-05 in the real app"
    expected: "Refused share port retries instead of showing the Desktop sharing is off card; Cmd+V after control ends sends nothing; the bar Paste to Mac refuses a concealed item; a later rights downgrade removes Control; closing a Desktop tab clears its sidebar line"
    why_human: "Unit-tested, marked requires human verification by the fixer; they landed after the VM rows ran"
---

# Phase 21: Portty App and iPhone Viewers Verification Report

**Phase Goal:** The office Mac's desktop opens in a Desktop tab of the Portty app on the home Mac, straight from the sidebar, and on an iPhone with touch controls. Both use the same viewer and protocol as the browser link.
**Verified:** 2026-11-27T12:00:00Z
**Status:** human_needed
**Re-verification:** No, initial verification

## Goal Achievement

All code for the goal exists, is wired and passes its unit tests. The Mac half (DVWR-01, DVWR-04) also has real-app proof in two Tart VMs. The iPhone half (DVWR-03) has no run on a real device or VM, and 12 real-device items (M-194..M-205) are deferred to user UAT. No implementation is missing or broken that I could find, so this is not `gaps_found`.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 (DVWR-01): the sidebar lists a peer's desktop next to its sessions; clicking opens one Desktop tab that shows and controls it, with the browser viewer's state and reconnect behaviour | VERIFIED | Sidebar renders the row only for a reachable peer with `desktop === true` (`Sidebar.tsx:207-236`); `App.tsx:1040-1045` opens or focuses one tab per peer and mounts `DesktopTab` (`App.tsx:1511`); `DesktopTab` calls `ConnectPeerDesktop` (binding in `wailsjs/go/main/App.js:101`, Go in `app_peer_desktop.go:55`) and renders the shared `DesktopView`. VM rows D1 (row, tab, Connecting card, Local Network prompt), D2 (denied path, cards, Open in Browser fallback), D3 (Allow heals the tab; picture composited in the translucent web view), D4 (hidden tab keeps decoding), D5 (hidden window releases the office after about 2 min, reopen reconnects in about 1.3 s), D6 (one tab per peer) all PASS (`21-VM-CHECKS.md`, `DVWR01_VM: pass`). The WebRTC-unusable fallback did not trigger: WebRTC works in the Wails web view (D3). |
| 2 | SC2 (DVWR-04): in control mode in a Desktop tab, Cmd+W, Cmd+Q and Cmd+C go to the remote Mac; leaving control mode gives Portty its shortcuts back | VERIFIED | 21-02 probe `KEYROUTE: page` (page-level `preventDefault` stops every Cmd chord from reaching the menu; K1-page 0 leaks, K1-ctl proves the detectors work). `controlMode.ts` and `DesktopTab.tsx` implement capture and the release chord. Real app K6 PASS: Cmd+C filled the office pasteboard and left home untouched, Cmd+W and Cmd+Q acted on office TextEdit while Portty kept its tab and pid, the release chord (Ctrl+Option+Cmd+Shift+Return) returned the label to "Click the picture to control" and a following Cmd+W closed the Desktop tab, a dropped stream ended control mode. H2 held-key check and P2 Cmd+V (including the concealed item notice, no paste alert) PASS (`DVWR04_VM: pass`, `PASTE_ALERT: none`). |
| 3 | SC3 (DVWR-03): on an iPhone, Safari or Home Screen, the viewer offers trackpad mode, pinch-zoom, long-press right-click, two-finger scroll and the soft keyboard, and picks up after the phone sleeps | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Present and wired: `touchTrackpad.ts` (357 lines; tap, double-tap, drag, `longPressMs: 500`, two-finger scroll vs pinch decided once, teardown releases held buttons), `desktopPhoneInput.ts`, `DesktopPhoneLayer.tsx` (435 lines, attaches the trackpad, armed modifiers, hidden-field Backspace/Enter), `DesktopPhoneBars.tsx` (Show Keyboard, Send Keys), `visualViewportInset.ts`; `DesktopView.tsx:1081` mounts the layer when `phoneMode`; `playsInline` and the Tap to Start card at `DesktopView.tsx:748,1132`; resume via `desktopClient.ts` visibility, pageshow and online wake. Unit tests pass. No test, VM or device exercises real iOS touch, the soft keyboard, callouts, the Home Screen or sleep and resume; all of M-194..M-200 are DEFERRED. Routed to human verification. |
| 4 | The Desktop tab and the iPhone use the same viewer and protocol as the browser link | VERIFIED | `DesktopTab.tsx:269` embeds the same `DesktopView` the `/desktop/` page uses, over the same `desktopClient.ts` WebSocket protocol; the app reaches the peer through the home daemon's loopback proxy. D2 shows the app tab and Safari as two viewers on the same office daemon. |
| 5 | The peer's desktop token never reaches the web view, and the same-account gate holds | VERIFIED | `POST /api/peer/desktop/cap` behind `requireSameUser`; token held in daemon memory only (`desktop_caps.go`, `TestDesktopCapStore_NoDiskWriteAPIs`); the binding returns a loopback URL with no token (`TestConnectPeerDesktop_LoopbackURLHasNoToken`); D2 `ax2` dump holds no `cap=`, token or URL. 21-SECURITY.md: 48/48 threats closed, `threats_open: 0`. |

**Score:** 4/5 truths verified (1 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `frontend/src/components/DesktopTab.tsx` (309 lines) | App Desktop tab, control mode, Cmd+V | VERIFIED | Imports `ConnectPeerDesktop`, `ReadHomeClipboard`, `controlReducer`, `DesktopView`; wired from `App.tsx:1511` |
| `frontend/src/components/DesktopView.tsx` (1233 lines) | Shared viewer plus phone layer | VERIFIED | Wired by both the app tab and the browser page |
| `frontend/src/components/DesktopPhoneLayer.tsx`, `DesktopPhoneBars.tsx`, `lib/touchTrackpad.ts`, `lib/desktopPhoneInput.ts`, `lib/visualViewportInset.ts` | iPhone touch layer | VERIFIED (wired, behaviour unverified) | See truth 3 |
| `frontend/src/lib/peerDesktop.ts`, `lib/controlMode.ts`, `lib/sidebarPeers.ts` | Connect, cards, control and paste logic, row states | VERIFIED | Exercised by tests and the VM rows |
| `app_peer_desktop.go`, `app_clipboard_paste.go` | Wails bindings | VERIFIED | Registered in `wailsjs/go/main/App.js`; `go test . -run 'PeerDesktop|ClipboardPaste|ReadHome'` passes |
| `internal/daemon/peer_desktop.go`, `desktop_caps.go`, `desktop_peer_token.go` | Home-daemon cap fetch, in-memory store, WS proxy | VERIFIED | `go test ./internal/daemon ./internal/tailnet` pass |
| `build/darwin/Info.plist` (and `Info.dev.plist`) | `NSLocalNetworkUsageDescription` | VERIFIED | Present in both; the VM prompt shows the exact string (D1) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| Sidebar Desktop row | App tab list | `onOpenDesktop` to `handleOpenDesktop` | WIRED | `Sidebar.tsx:60,208`; `App.tsx:1040,1218` |
| App tab | Desktop panel | `DesktopTab` mounted for `type: 'desktop'`, never a terminal | WIRED | `App.tsx:1495-1515`; `App.desktop-tab.test.tsx` |
| DesktopTab | Go bridge | `ConnectPeerDesktop` | WIRED | `App.js:101` to `app_peer_desktop.go:55` to `client.ConnectPeerDesktop` |
| Go bridge | peer cap route | home daemon fetches the token, serves a loopback WS proxy | WIRED | `TestPeerDesktop_Tracer`, `TestRemoteDesktopWS_*`; D1 to D6 on real daemons |
| DesktopView | phone layer | `phoneMode && phoneClient` | WIRED | `DesktopView.tsx:1081` |
| Window hidden | viewer release | `portty:window-hidden` event feeds the tab | WIRED | D5 on the real app |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| DesktopView video | media stream | WebRTC from the office daemon over the loopback WS proxy | Yes (D3: framesDecoded rising at 2560x1440) | FLOWING |
| Sidebar Desktop row | `peer.desktop` | `X-Portty-Desktop: always` on `/api/sessions/meta` | Yes (D1 row appears for the office guest) | FLOWING |
| Desktop tab label | control state | `controlReducer` | Yes (K6 label transitions) | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Phase 21 frontend suites (15 files) | `vitest run` on DesktopTab, DesktopView, DesktopPhoneBars, DesktopPhoneLayer, App.desktop-tab, Sidebar, TabBar, controlMode, desktopPhoneInput, peerDesktop, sidebarPeers, touchTrackpad, visualViewportInset, tabs, desktopCoords | 15 files, 551 tests passed | PASS |
| Daemon and tailnet Go packages | `GOTOOLCHAIN=go1.26.11 go test ./internal/daemon ./internal/tailnet` | ok (cached) | PASS |
| Root Go bindings | `GOTOOLCHAIN=go1.26.11 go test . -run 'PeerDesktop|ClipboardPaste|ReadHome'` | ok | PASS |
| Traceability paths | `bash tests/check-traceability-paths.sh` | OK: 313 paths | PASS |
| Review-fix and rig commits exist | `git cat-file -t` on 9ef402e6, 0af1d672, d23310b1, 64cdb045, 307277be, b982e560, b845c93f | all `commit` | PASS |

Nothing was run that starts Portty, captures the screen, injects input or contacts a tailnet address.

### Probe Execution

SKIPPED: the phase's probes are the Tart VM rig (`rig/p21-rig.sh`, `rig/keyprobe`), which only run inside the guests. Their results are recorded in `21-VM-CHECKS.md` with evidence files in `evidence/`; I read them and did not re-run them.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| DVWR-01 | 21-01, 21-04, 21-05, 21-06, 21-07, 21-08, 21-11, 21-12, 21-13 | Portty app on another Mac opens a peer's desktop in a Desktop tab from the sidebar | SATISFIED in code and VM (`DVWR01_VM: pass`); real-Mac items M-201, M-204, M-205 deferred | Truths 1, 4, 5. REQUIREMENTS.md still `[ ]` Pending, which is correct until the deferred items pass |
| DVWR-03 | 21-03, 21-10, 21-13 | iPhone viewer: trackpad mode, pinch-zoom, long-press right-click, two-finger scroll, soft keyboard, tap to start video | NEEDS HUMAN | Truth 3. No VM row exists for it; M-194..M-200 deferred |
| DVWR-04 | 21-02, 21-09, 21-12, 21-13 | In control mode in the app, Cmd+W, Cmd+Q, Cmd+C go to the remote Mac | SATISFIED in code and VM (`DVWR04_VM: pass`); real-Mac items M-202, M-203, M-205 deferred | Truth 2 |

All three IDs in the phase requirements appear in plan frontmatter and in REQUIREMENTS.md. No orphaned requirements: REQUIREMENTS.md maps only DVWR-01, DVWR-03 and DVWR-04 to Phase 21 (DVWR-02 belongs to Phase 19).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| Phase 21 source files | - | TBD, FIXME and XXX markers | none found | Debt-marker gate clear |
| `frontend/src/components/DesktopView.tsx` | 204 | `RELEASE_CHORD_HINT` hard-codes "Ctrl+Option+Cmd+Return" and ignores the `releaseChord` prop | Info | Correct for the browser page; the app tab hides this notice (`hideCaptureNotices`), so no user impact today (UI-REVIEW fix 1) |
| `README.md` | 30 | Copy defects noted by the UI review | Info | Addressed by commit 596b413d "README iPhone desktop copy names the Tap to Start and Show Keyboard controls" |
| `frontend/src/App.tsx` (WR-05 residual) | - | A refused share port reports `desktop: false` and can prune the sidebar's muted line | Info | Documented in `21-REVIEW-FIX.md`; needs a new binding field to separate |

### Gate Results Recorded

- Code review: 0 critical, 5 warnings WR-01..WR-05, all fixed test-first (`21-REVIEW-FIX.md`, `all_fixed`); each marked "requires human verification".
- Security: 48/48 threats closed, `threats_open: 0`. AR-21-01 (accepted, pending user review): the viewing side does not check the peer's account, so a different-account node on the tailnet that serves a valid share port could announce a Desktop row and receive control-mode keystrokes and, on Cmd+V, the home clipboard's plain text once the user opens that row and clicks into it. Same trust model as the terminal peer path; revisit in Phase 22 when guest sharing changes the account model.
- UI review: 18/24 (all pillars 3/4), code-only; light/dark and iPhone gestures unobserved.
- Decided: the release chord is Ctrl+Option+Cmd+Shift+Return (K3 found the unshifted chord is taken by the system); UI-SPEC text naming the unshifted chord is stale, not a gap.

### Human Verification Required

See the `human_verification` list in the frontmatter: 12 device items (M-194..M-205 in TESTING.md Section 5 Category BL) plus a real-app re-check of WR-01..WR-05. The VM proof does not stand in for these: a real iPhone, real Retina sharpness and direct path, real display sleep on battery and power, a real password manager, and a visual Light and Dark judgement. One VM observation to look at on a real Mac in M-202: after the release chord once, the home guest's OS key state read Control held; it was not reproduced and the key tool is the suspect, but confirm the next click is a plain click.

### Gaps Summary

No gaps. Every artifact the three requirements need exists, is substantive and is wired, and tests pass. What remains is evidence a code check cannot give: the iPhone behaviour (DVWR-03, no VM coverage at all) and the real-Mac items. DVWR-01, DVWR-03 and DVWR-04 should stay Pending in REQUIREMENTS.md until the owner runs M-194..M-205 (`21-UAT.md`).

---

_Verified: 2026-11-27T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
