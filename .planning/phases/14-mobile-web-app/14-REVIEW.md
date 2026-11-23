---
phase: 14-mobile-web-app
reviewed: 2026-11-22T00:00:00Z
depth: standard
files_reviewed: 35
files_reviewed_list:
  - cmd/playwright-fixture/main.go
  - cmd_share.go
  - frontend/index.html
  - frontend/playwright.config.ts
  - frontend/public/manifest.json
  - frontend/src/App.tsx
  - frontend/src/components/ErrorBoundary.tsx
  - frontend/src/components/KeyBar.tsx
  - frontend/src/components/PdfPreview.tsx
  - frontend/src/components/SessionEndedBanner.tsx
  - frontend/src/components/SettingsSheet.tsx
  - frontend/src/components/TabBar.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/UserRefused.tsx
  - frontend/src/components/WebShareSessionView.tsx
  - frontend/src/lib/closeReasons.ts
  - frontend/src/lib/ctrlMap.ts
  - frontend/src/lib/phoneQuery.ts
  - frontend/src/lib/pinchZoom.ts
  - frontend/src/lib/relayClient.ts
  - frontend/src/lib/shareCopy.ts
  - frontend/src/lib/visualViewportInset.ts
  - frontend/src/style.css
  - internal/daemon/share_vmwhois.go
  - internal/relay/oscabsorb.go
  - internal/tailnet/tailnet.go
  - internal/webserver/csp_mw.go
  - internal/webserver/server.go
  - internal/webserver/webservertest/whois.go
  - internal/webserver/whois.go
  - tests/check-brand.sh
  - web/assets/dashboard.css
  - web/embed.go
  - web/service-retry.html
  - web/user-refused.html
findings:
  critical: 0
  warning: 3
  info: 7
  total: 10
  necessity: 1
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-11-22
**Depth:** standard
**Files Reviewed:** 35
**Status:** issues_found

## Summary

Reviewed the diff against `2153501a` for every listed file, with the security-sensitive areas traced end to end. I found no blockers. The same-account gate holds under adversarial reading. The three warnings are in the phone UI and none touches the security boundary.

Security results, each traced in code, not taken from the SECURITY-REVIEW document:

- **Same-account gate, check order and fail-closed.** `sameUserStatus` (`internal/webserver/whois.go:88-114`) returns 503 on any status error, nil status, nil `Self`, or any WhoIs error other than `ErrPeerNotFound`. It returns 403 on not-found, a nil `who` with a nil error, a tagged peer, a shared-in peer, a tagged or shared-in self, a zero user ID, or a user mismatch. `next` is only reachable on status 0. Order in `setupRoutes` is as documented. WebSocket runs Origin, then same-user, then capability. The link page runs CSP, same-user, then the capability page. The API routes run same-user, then capability. `/app/` runs CSP, then same-user page gate. The `/app` to `/app/` redirect comes from the mux before any handler and carries no data. I enumerated every `mux.Handle*` registration. Every route that carries session data is gated. The only open routes are `/`, `/dashboard` (static, no session list), `/assets/` and `/assets/xterm/` (embedded static files).
- **Fakes and the `vmwhois` tag.** `webservertest` is imported only by `cmd/playwright-fixture` (tag `playwrightfixture`, which every file in that directory carries) and by `_test.go` files. `share_vmwhois.go` is tag-guarded, and its modes fail closed (unknown or unreadable mode content gives fault, `other` is the default). The release builds (`build.sh`, `build.yml`, `release.yml`) use only `-tags wailsassets`. `go vet` is clean under both `-tags vmwhois` and `-tags playwrightfixture`. See IN-05 for one gap in the guard's reach.
- **Lone-Esc flush (`oscabsorb.go`).** After `Filter` returns, the state is never `stateGotEsc`, so no half-introducer state survives a frame. A split after the introducer is still absorbed, because the OSC and CSI states persist. The only new leak is an Esc, `]` or Esc, `[` split exactly at a frame boundary, which the document already accepts. The leaked bytes carry no newline, so they cannot execute at a shell prompt. A same-account write holder already controls the shell. `IsUserInput` keeps a lone trailing Esc non-claiming. I found no way for an attacker-controlled or split sequence to gain anything beyond the accepted cosmetic leak.
- **CSP delta.** The delta is exactly `manifest-src 'self'` (`csp_mw.go:172-173`), and `default-src 'none'` and the absence of `unsafe-eval` are unchanged. The manifest has no `start_url` or `id`, and its `scope` resolves to `/app/`. Under Vite `base: './'` the head tags are emitted as `./manifest.json` and `./apple-touch-icon.png`, which resolve correctly under `/app/`.
- **Key bar input mapping.** `ctrlByte` and `arrowSeq` are closed tables. The fire rule (pointerdown, plus click with `detail === 0`) does not double-send on touch. The bar is gated by `canWrite !== false`, and `perms` parsing matches the server's `HasPerm`. See WR-02 for an armed-Ctrl consumption bug.
- **`visualViewport` and pinch handlers.** Listeners are added and removed symmetrically. The non-passive `touchmove` is registered only on the terminal container, and the one-finger path never calls `preventDefault`. See WR-01 and WR-03.

## Warnings

### WR-01: Stale `maxInner` makes the viewport "heal" fire in landscape and flicker the whole app

**File:** `frontend/src/lib/visualViewportInset.ts:26-48`
**Issue:** `maxInner` is initialised from `window.innerHeight` at mount and only ever grows (`Math.max`, line 32). `heal` decides "the viewport is stuck shrunk" with `maxInner - vv.height > KEYBOARD_MIN_PX` (line 44). Mount in portrait (about 850 px), then rotate to landscape (innerHeight and `vv.height` about 390 px with the keyboard closed). `maxInner` stays at the portrait height, so the difference is about 460 px, well over 100. After that, every `focusout` of the xterm helper textarea in landscape runs the heal 140 ms later. The heal sets `display: none` on the whole `.app` root, forces layout, and restores it a frame later. This is a visible flash. It also drives a zero-size `ResizeObserver` pass in the terminal. The test file has no rotation case (`grep rotat|orientation|landscape` finds nothing), so nothing pins this. `keyboardUp` (line 35) correctly uses the live `window.innerHeight`, so only the heal is affected.
**Fix:** Compare against the current layout height, or reset the baseline on orientation change.
```ts
const heal = (): void => {
  healTimer = null
  if (document.activeElement?.matches('.xterm-helper-textarea')) return
  // The layout viewport never shrinks for the keyboard on iOS, so the live innerHeight is the baseline.
  if (window.innerHeight - vv.height <= KEYBOARD_MIN_PX) return
  ...
}
```
Drop `maxInner` entirely, or reset it in an `orientationchange` or `resize` handler. Add a test that rotates (innerHeight 850 to 390) and asserts no `display: none` on blur.

### WR-02: An armed Ctrl is consumed by terminal-generated replies, not just keystrokes

**File:** `frontend/src/components/WebShareSessionView.tsx:136-140` (consumed at `frontend/src/components/TerminalPanel.tsx:717`)
**Issue:** `inputTransform` runs on every `term.onData` payload. xterm.js also emits terminal-generated data through `onData`: replies to DA1 and DSR queries (cursor position reports), focus in and out reports (`ESC [ I`, `ESC [ O`) when DECSET 1004 is on, and mouse reports. If any of these lands between the user tapping Ctrl and typing the next key, `setCtrl(false)` fires and `ctrlByte(data)` returns null for the multi-byte reply, so the arm is spent on bytes the user never typed. The user then types `c` and gets a literal `c`, not Ctrl-C. This is the most important key combination for the key bar (stop a running command), and shells and TUIs (Claude Code, vim) query the terminal at unpredictable times. The window is small, but the failure is silent and hits the one gesture the bar exists for.
**Fix:** Only consume the arm for typed input. Terminal replies and reports start with ESC and are longer than one byte; Alt-key sequences are the only typed ESC-prefixed data.
```ts
const inputTransform = (data: string): string => {
  if (!ctrlRef.current) return data
  if (data.length > 1 && data.charCodeAt(0) === 0x1b) return data // a terminal reply or report, not a key
  setCtrl(false)
  return ctrlByte(data) ?? data
}
```
Add a test that arms Ctrl, feeds `\x1b[I` and a DSR reply through `onData`, and asserts Ctrl is still armed.

### WR-03: Page zoom is disabled across the whole phone page, so the in-app PDF cannot be zoomed

**File:** `frontend/src/style.css:2477-2479`, `frontend/src/lib/pinchZoom.ts:79,86`, `frontend/src/components/PdfPreview.tsx:90`
**Issue:** On phones `html { touch-action: pan-x pan-y }` removes pinch-zoom for the entire document. `attachPinchZoom` adds a document-level `gesturestart` `preventDefault` on top of that. The pinch is meant to change the terminal font, but both rules are global: they also apply to the Open PDF layer, which draws the PDF as an `<img>` at `width: 100%` of a phone screen (about 390 px). A PDF page at that width is not legible, and the only zoom path left is "Open full PDF", which leaves the in-app layer. `touch-action` only ever restricts down the tree, so `.pdf-layer` cannot opt back in. This also blocks the user's own accessibility zoom on every other screen of the share page (WCAG 1.4.4). The comment says this is intentional ("a phone never pinch-zooms the page"), so this is a design trade-off the user may not have weighed against the PDF layer.
**Fix:** Scope the restriction to the terminal instead of `html`: set `touch-action: pan-x pan-y` on `.terminal-session-container` only, and register `gesturestart` on that container, not on `document`. Give `.pdf-layer__body` `touch-action: pan-x pan-y pinch-zoom` so the PDF image can be pinched. If the global block is wanted, say so in the 14-UI-SPEC and the PDF layer copy ("Open full PDF to zoom").

## Info

### IN-01: The in-app PDF layer has no fallback when the engine cannot draw a PDF in an `<img>`

**File:** `frontend/src/components/PdfPreview.tsx:90`, `frontend/src/lib/phoneQuery.ts:25-28`
**Issue:** The layer is used whenever `isStandalone()` is true, and the manifest (`display: standalone`) now makes Chrome on Android and desktop offer an installable standalone app too. In those engines an `<img>` with a PDF blob does not render, so the user sees a broken image and the alt text. "Done" and "Open full PDF" still work, so the user is not stranded. There is no `onError` handler. The target (iPhone WebKit) is verified in S-06.
**Fix:** Add `onError` that sets a state showing "Can't show this PDF here. Use Open full PDF.", or gate the layer on `navigator.standalone === true` (iOS only), the platform where the stranding problem exists.

### IN-02: A pending heal `requestAnimationFrame` is not cancelled on cleanup

**File:** `frontend/src/lib/visualViewportInset.ts:47,60-70`
**Issue:** The heal's `requestAnimationFrame(() => { root.style.display = ''; update() })` is not tracked. If the effect cleans up while it is pending (for example `enabled` flips because an iPad gains a trackpad), the callback still runs `update()`. That writes `--vv-height` and `--vv-top` onto the root after cleanup removed them, leaving the root pinned to a stale height.
**Fix:** Store the id (`healFrame`), cancel it in the cleanup, and have `update` return early when a `disposed` flag is set.

### IN-03: Pinch handler calls `preventDefault` without checking `cancelable` and attaches on non-phone touch devices

**File:** `frontend/src/lib/pinchZoom.ts:48`, `frontend/src/components/TerminalPanel.tsx:855`
**Issue:** (a) `e.preventDefault()` on a two-touch `touchmove` is called unconditionally. Once the browser has committed to a scroll the event is not cancelable, and Chrome logs "Ignored attempt to cancel a touchmove event". (b) `attachPinchZoom` is attached for every TerminalPanel, not only on a phone. On a touch-screen desktop browser, `WebShareSessionView` passes a no-op `onFontSizeChange`, so two-finger gestures on the terminal are swallowed and do nothing.
**Fix:** Guard with `if (e.cancelable) e.preventDefault()`. Pass a `enabled` flag (the existing `phone` value) or skip `attachPinchZoom` when `onFontSizeChange` is the no-op.

### IN-04: "Two consecutive connect timeouts" does not reset on other failures

**File:** `frontend/src/lib/relayClient.ts:368,401`
**Issue:** `connectTimeouts` is incremented only in the connect timer and reset only on the first host frame or in `teardown`. A timeout, then an immediate refused or errored attempt (the close path never touches the counter), then another timeout reports "stuck" even though the attempts were not consecutive timeouts. The effect is a Reload button appearing on the reconnecting banner a little early, which is harmless but not what the comment says.
**Fix:** Reset `connectTimeouts = 0` in the `onclose` path when the close did not come from the connect timer (the `socketGen` guard already isolates it).

### IN-05: The release guard covers the fake's package, not the setter that installs any fake

**File:** `internal/webserver/whois.go:41`, `tests/check-brand.sh:73-76`
**Issue:** `SetWhoIsClientForTest` is an exported method on the production type that replaces the gate's identity source. The `fake-whois` guard only fails on an import of `webservertest`. A non-test file that defines its own always-same-user type and calls the setter would pass `check-brand.sh` and the `go list` release-graph test. Today the only non-test callers are the fixture and `share_vmwhois.go`, which are covered by the build tags and the `vm-tag` guard. The gate's integrity therefore rests on convention for any future caller.
**Fix:** Add a third `git grep` to `check-brand.sh` for `SetWhoIsClientForTest` in `*.go` excluding `*_test.go`, `cmd/playwright-fixture/`, `internal/daemon/share_vmwhois.go` and `internal/webserver/whois.go`, reported as `fake-whois`.

### IN-06: Fixture CA is unconstrained and the admin listener has no authentication

**File:** `cmd/playwright-fixture/main.go:451-458,225`
**Issue:** The CA written to `PLAYWRIGHT_FIXTURE_CA_PEM` for the simulator trust store has `IsCA` with no `MaxPathLen` and no name constraint, so a trusted copy could sign a certificate for any host. Its lifetime is 2 hours, its key is memory-only, and it is fixture-only, so the risk is bounded. Separately, the loopback admin listener flips the WhoIs identity (`/__test__/whois`), mints tokens (`/__test__/share-on`) and ends the session with no credential, so any local process or user can drive the fixture.
**Fix:** Set `PermittedIPAddresses: []*net.IPNet{{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(32, 32)}}`, `PermittedDNSDomainsCritical: true` and `MaxPathLenZero: true` on `caTmpl`. Optionally require a per-run random header on `/__test__/*`.

### IN-07: `mustWebFile` panic branch cannot occur, and a stale plan reference remains

**File:** `internal/webserver/whois.go:159-164`, `internal/webserver/webservertest/whois.go:3`
**Category:** necessity
**Issue:** `userRefusedPage` and `serviceRetryPage` read files named in a `//go:embed` directive (`web/embed.go:5`). A missing file fails the build, and `ReadFile` on an embedded name cannot error at run time. The panic branch guards a state the compiler already excludes. Separately, the package comment says "Plan 14-06 adds the guard", which has already shipped (`TestNoFakeWhoIsInRelease`), so the comment is stale.
**Fix:** Keep it if the project wants a belt-and-braces check (it is cheap and matches the existing `fs.Sub` panics), but note the reason in the comment, or use `//go:embed user-refused.html` into `[]byte` variables directly. Update the `webservertest` comment to name the shipped test.

---

_Reviewed: 2026-11-22_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
