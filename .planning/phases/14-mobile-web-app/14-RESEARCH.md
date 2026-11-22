# Phase 14: Mobile Web App - Research

**Researched:** 2026-11-22
**Domain:** iPhone-grade browser share view (`/app/`) served by the host Mac over the tailnet: same-account gate, Home Screen install, touch input, reconnect
**Confidence:** MEDIUM overall. The server-side gate, CSP, input-filter and test-harness findings are HIGH (source read and run this session). iOS runtime behaviour (Home Screen launch URL, standalone keyboard, suspend/roaming events) is MEDIUM/LOW: no iPhone was available, so it is cited from WebKit source and bug reports and must be closed by the real-phone check (M-145..M-148).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Same-account gate (MOB-05)**
- `requireSameUser` runs outermost, before any capability work, on: `GET /sessions/{id}`, all of `GET /app/` (SPA, chunks, manifest, icons), `/api/sessions/{id}/info`, `/api/plugin-config`, `/api/plugin-config/stream`, and `/sessions/{id}/ws` (order there: origin → WhoIs → cap). Left open: the cap-less discovery probe `GET /api/sessions`, `/assets/`, `/` and `/dashboard`, since none carries session data and gating the probe would hide peers whenever tailscaled blips. Per-route wrapping keeps a per-session hook for invited devices later. — **Reversibility:** one-way — auth model; tests, the Playwright fixture and CI need a same-user WhoIs fake (the fixture is not a test binary, server.go:178-181).
- Refusal page: a static embedded page in the link-invalid.html style (403 + X-Portty marker; names no account). Heading "This link only works on your own devices"; body "Open it on a device signed in to the same Tailscale account as the Mac that shared it." (Claude's discretion)
- Share copy that MOB-05 makes false (shareCopy.ts, SettingsSheet.tsx, cmd_share.go, README.md and their tests) becomes: "Treat this link like a password. It only opens on devices signed in to your Tailscale account." (Claude's discretion)

**Input: lone Esc**
- Bug found: `InputAbsorber.Filter` holds a bare ESC until the next byte (oscabsorb.go:52-75; pinned by test P9), so the key bar's Esc and Claude Code's "Esc to interrupt" lag in every viewer, local tabs included. Fix: pass a bare ESC through when it ends a MsgInput frame. Research must confirm xterm never splits an auto-reply across onData calls; P9 is changed deliberately. Verify the current behaviour on a VM first (vim, a TUI). — **Reversibility:** costly — an input filter shared by every viewer and both pumps.

**Home Screen (MOB-02)**
- The token stays where it is today, in `/app/?session=…&cap=…`. Ship a static manifest with no `start_url` (it defaults to the document URL) and scope `./`, so the icon opens the exact link. The icon stops working when sharing stops, the key is regenerated, or the daemon restarts (share state lives in memory); the user re-adds it. SC2's "Link No Longer Works" comes from the existing 403 on `/info`. — **Reversibility:** one-way — URL shape and auth.
- CSP: add only `manifest-src 'self'` to the share CSP (csp_mw.go), with security-review sign-off. The icon is already covered by `img-src 'self'`. Serve the manifest (`.json`, or an explicit content type) and icons from frontend/public with no-cache. `display: standalone`, `viewport-fit=cover`, safe-area insets. — **Reversibility:** costly — CSP change.

**Key bar (MOB-01, Claude's discretion)**
- Keys: exactly the SC1 set: Esc, Tab, Ctrl, ←, ↑, ↓, →, `|`, in one row of ~44 pt keys.
- Ctrl is one-shot: tap highlights it, the next key (from the bar or the iOS keyboard) gets Ctrl, then it releases; tap again to cancel. Map a-z and A-Z to 0x01-0x1A (iOS auto-capitalises), `@ [ \ ] ^ _` and space to C0, and Ctrl+arrow to `CSI 1;5`.
- Placement: always shown at the bottom on touch-only devices, docked above the keyboard when it is up, so Esc or Ctrl-C is one tap with no keyboard (e.g. interrupting Claude Code).

**Phone layout (MOB-01, Claude's discretion)**
- Reflow: keep INPT-06 as it is. Typing, pinch or rotation claims the grid; joining and just looking never reflow the office Mac. Until then the phone shows the owner's grid scaled, with the 0.7 floor and horizontal scroll.
- Pinch-to-zoom changes the terminal font, not the page. The size is kept in memory only: a relaunch returns to the host's font, and nothing is stored on the phone. One claim when the pinch ends (debounced).
- Chrome: hide the tab strip on phones while only the session tab exists; show it when a preview arrives. Tighter padding.

**Icon and launcher (MOB-02, MOB-04, Claude's discretion)**
- Icon: a full-bleed iOS version derived once from build/appicon.png (gradient square with the glyph), committed as one source PNG and exported at 180 px (apple-touch-icon), 192 and 512 px (manifest).
- Home Screen name: "Portty".
- Launcher (portty.app/app): static copy only. It covers what Portty is, how to get the link from the Mac's Share button onto the phone (AirDrop, Universal Clipboard), the Add to Home Screen steps, and the link format `https://<mac>.<tailnet>.ts.net:7444/sessions/…`. No inputs, no token, no WebSocket. It lives as a markdown file in this repo for the website repo to use. The copy warns that sending the link to yourself in Messages stores it in iCloud.

**Decided autonomously**
- [Gate] Q: "Fail-closed codes and SPA reaction" → Selected: "Keep 503 for a local tailscaled fault (clients retry; page routes get a static retry page); the SPA tells a WhoIs 403 (marker) apart from a cap 403 and shows the refusal text at bootstrap and in the reconnect probe; live sockets are not re-checked (the Phase 9 handoff reconnect re-runs the gate)" — one client header branch; no wire change.
- [Key bar] Q: "How keys reach the shell and which devices get the bar" → Selected: "term.input(seq, true) so bytes take the onData → sendInput path and count as typing; arrows follow applicationCursorKeysMode; preventDefault on pointerdown keeps the keyboard up; shown only when (pointer: coarse) and (hover: none)" — frontend only.
- [Layout] Q: "Keeping the cursor line above the keyboard" → Selected: "On touch devices, size the app root to visualViewport height and offset; ResizeObserver reports, and the next keystroke claims at the reported size" — frontend only, existing report/claim paths.
- [Reconnect] Q: "Reconnect after sleep and standalone recovery" → Selected: "No new mechanism; add pageshow as a wake trigger; unit-test an iOS-like suspend; give the stopped, ended, unreachable and ErrorBoundary states a Reload button on the phone; real-phone check recorded as M-145" — client only.

### Claude's Discretion
Refusal and share copy, key set, Ctrl behaviour, bar placement, reflow trigger, font persistence, phone chrome, icon art, icon name and launcher scope, as recorded above. The planner may adjust details within these choices.

### Deferred Ideas (OUT OF SCOPE)
- Invited devices / pairing code for a browser share (future milestone; builds on MOB-05).
- More key-bar keys (Shift-Tab, ~, /, -, Alt).
- Saving the pinch font size per device.
- A "Fit" control or first-tap claim on phones.
- A launcher "check my connection" field, or a QR code in the Share popover.
- Pre-filled per-session Home Screen names.
- An icon that survives re-sharing (token-less start_url plus same-account token fetch).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MOB-05 | Browser share links work only for devices signed in to the host's Tailscale account; every page/info/WebSocket request gets the WhoIs same-user check, fail closed | Gate wiring pattern (Pattern 1), WhoIs cost measurement and cache recommendation, host-own-IP resolution verified, fixture/VM fake design, threat register T-14-01..T-14-08 |
| MOB-01 | Phone share view: key bar, width fit, keyboard inset, pinch font size | Key bar via `term.input`, `visualViewport` findings (Pattern 3), pinch handler (Pattern 4), 16 px auto-zoom rule, lone-Esc fix, Playwright media-query evidence |
| MOB-02 | Add to Home Screen, full screen, all assets host-served | WebKit manifest parsing (start_url default), CSP experiment (manifest-src), MIME finding, `/app/` cache-header interaction, icon recipe |
| MOB-03 | Reconnect after sleep or network change on a real iPhone | relayClient wake paths, iOS WebSocket CONNECTING bug (WebKit 308073), pageshow trigger, Reload escalation, M-148 |
| MOB-04 | Repo supplies launcher copy and link format | Launcher file placement, link format, iCloud/Messages warning, copy-drift list (MOB-05 makes six strings false) |
</phase_requirements>

## Summary

The server work (MOB-05) is small and well-bounded: `requireSameUser` already exists, fails closed, and is used on five routes. Phase 14 wraps seven more route groups with it. Three findings change how it should be built. First, WhoIs cost is unmeasured and may matter: each gated request makes two LocalAPI calls (`StatusWithoutPeers` and `WhoIs`) and `/app/` has about ten requests per cold load, so a short positive-verdict cache per source IP is a cheap hedge; the real cost must be measured in a VM (M-149). Second, every existing Go test and the Playwright fixture that touches a newly gated route will fail 503 (`noLiveWhoIs` in test binaries; a real WhoIs against `127.0.0.1` in the fixture), so a shared same-user fake and a release-exclusion guard are Wave 0 work. Third, the `/app/` handler strips `Cache-Control` for every non-index file, so the manifest and icons would be served with no cache header unless the Del is narrowed to `assets/`.

Two decisions in CONTEXT hold up under evidence. The lone-Esc fix is safe: every `triggerDataEvent` call site in xterm 6.0.0 sends a whole key or reply as one string, and the only frame splitting is `encodeInputFrames` at 32767 bytes, which only large pastes reach. And `manifest-src 'self'` is required: Chromium blocks the manifest under today's `default-src 'none'` (reproduced) and WebKit's loader checks manifests and icons against CSP (source read). One nuance: a lone Esc will pass through but will not claim the grid, because `IsUserInput` returns false for a trailing bare ESC. That is consistent with INPT-06's conservative rule and needs no change.

iOS runtime behaviour cannot be proven from here. WebKit's manifest parser sends a missing `start_url` to the document URL (source read), which supports the no-`start_url` design, but the Add-to-Home-Screen flow on a real iOS 26/27 device must confirm the query string survives. Standalone-mode `visualViewport` has documented bugs, and a WebKit bug filed in 2026 shows new WebSockets hanging in CONNECTING after a resume. The existing 15 s connect timeout and the timer-gap wake detector already cover that shape; the plan should add a Reload escalation after repeated connect timeouts.

**Primary recommendation:** Build MOB-05 first, with a per-IP positive-verdict cache only if the VM measurement (M-149) shows WhoIs cost matters, a shared same-user fake in a non-release package plus a `go list -deps` release guard, and `manifest-src 'self'` as the only CSP change; then build the phone UI on `term.input`, `visualViewport` and touch-distance pinch, and close the iOS unknowns with M-145..M-150.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Same-account gate | API / Backend (share server, `requireSameUser`) | Frontend (renders refusal from marker) | Identity comes from the local tailscaled WhoIs of the TCP source IP; the browser cannot be trusted to assert it |
| Refusal and retry pages | API / Backend (embedded static HTML) | — | Must render before any SPA or terminal data is sent |
| Manifest, icons, CSP | CDN / Static (served by the share server from the embedded bundle) | API / Backend (CSP header) | All assets host-served (MOB-02); CSP is a response header |
| Home Screen launch URL | Browser / Client (iOS WebKit) | — | iOS decides the launch URL from the page URL at add time |
| Key bar, Ctrl one-shot, pinch font size | Browser / Client | — | Pure UI; bytes reuse `onData → sendInput` |
| Keyboard inset, safe areas | Browser / Client | — | `visualViewport` and CSS `env()` |
| Lone-Esc passthrough | API / Backend (relay `InputAbsorber`) | — | The filter lives server-side in both pumps (`relay/server.go`, `webserver/server.go`) |
| Reconnect after suspend/roam | Browser / Client (`relayClient`) | API / Backend (gate re-run on redial) | Client detects and redials; server re-runs WhoIs and cap on the new socket |
| Launcher copy and link format | Docs in this repo (markdown) | Website repo (renders it) | MOB-04: static, no token, no WebSocket |

## Standard Stack

### Core
No new runtime dependencies are needed. Everything uses what is already pinned.

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `@xterm/xterm` | 6.0.0 (installed) [VERIFIED: frontend/node_modules/@xterm/xterm/package.json] | `term.input()`, `modes.applicationCursorKeysMode`, helper textarea | Already the terminal; typings declare `input(data: string, wasUserInput?: boolean): void` (typings/xterm.d.ts:1025) and `readonly applicationCursorKeysMode: boolean` (:1911) |
| `tailscale.com` | v1.98.3 [VERIFIED: go.mod:20] | `client/local` WhoIs / StatusWithoutPeers | Already used by `requireSameUser` |
| `@playwright/test` | 1.59.1 (installed) [VERIFIED: frontend/node_modules/@playwright/test/package.json] | iPhone WebKit project | `devices['iPhone 15']` is the newest iPhone profile in this version (measured: list ends at iPhone 15 Pro Max) |

### Supporting
| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| ImageMagick `magick` | present at /opt/homebrew/bin/magick [VERIFIED: command -v] | Derive the full-bleed icon once | Icon task (executor runs it once, commits the PNGs) |
| `sips` | /usr/bin/sips [VERIFIED: command -v] | Export 180/192/512 px from the committed source | Alternative to magick for resizing |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Touch-distance pinch | `gesturestart`/`gesturechange` (`scale`) | Gesture events are WebKit-proprietary, absent in Playwright WebKit (measured `'ongesturestart' in window === false`), so untestable. Use touch distance as the mechanism; add a `gesturestart` `preventDefault` only as a belt against page zoom |
| `manifest.json` | `.webmanifest` + explicit `Content-Type` | Go on macOS has no `.webmanifest` entry: `FileServerFS` served it as `text/plain; charset=utf-8` (measured). `manifest.json` serves as `application/json`; either works, `.json` needs no code |
| Per-IP verdict cache | Per-request WhoIs | Uncached cost is unmeasured (two LocalAPI calls per request); a cache bounds the load and the staleness to the TTL (see Pitfall 1) |

**Installation:** none. **Version verification:** no packages added; `npm view` not applicable. Playwright's WebKit for 1.59.1 (`webkit-2272`) is not installed on this Mac (the cache holds `webkit-2336`); run `cd frontend && pnpm exec playwright install webkit` before the first iPhone e2e run [VERIFIED: launch error and ls ~/Library/Caches/ms-playwright].

## Package Legitimacy Audit

No external packages are installed by this phase. `Packages removed due to [SLOP] verdict:` none. `Packages flagged as suspicious [SUS]:` none. The new Go package `internal/webserver/webservertest` is in-repo code, not a dependency.

## Architecture Patterns

### System Architecture Diagram

```
iPhone (Tailscale app) ──tailnet──▶ host share listener (BindIP:7444, TLS)
        │                                   │
        │  GET /app/?session&cap            ▼
        │                          cspHeaders (outermost, adds CSP + no-store + no-referrer)
        │                                   │
        │                                   ▼
        │                    requireSameUser[Page]  ◀── LocalAPI (tailscaled)
        │                       │ cache hit (≤TTL, per source IP, positives only)?
        │                       │ else StatusWithoutPeers + WhoIs(remote IP)
        │                       ├─ error / no self ─▶ 503 (retry page for pages)
        │                       ├─ other user / tagged / shared-in ─▶ 403 + X-Portty (refusal page for pages)
        │                       ▼ same user
        │         ┌─────────────┴───────────────────────────────┐
        │   /app/ static (index no-store;        /api/sessions/{id}/info, /api/plugin-config(+/stream)
        │   assets/ hashed cacheable;                    │ requireCapability (401/403 no marker)
        │   manifest.json, icons no-cache)               ▼
        │                                         handlers
        │   /sessions/{id}/ws: requireAllowedOrigin → requireSameUser → requireCapability → handleWSSRelay
        ▼
 SPA in browser: bootstrap fetch /info
        ├─ 403 + X-Portty ─▶ refusal text ("This link only works on your own devices")
        ├─ 401 / 403 (no marker) ─▶ LinkNoLongerWorks
        └─ ok ─▶ WebShareSessionView ─▶ TerminalPanel
                    ├─ KeyBar ──term.input(seq,true)──▶ onData ─▶ relayClient.sendInput ─▶ WS MsgInput
                    ├─ pinch (touch distance) ─▶ font delta ─▶ debounced claim
                    ├─ visualViewport ─▶ app-root height/offset ─▶ ResizeObserver report
                    └─ relayClient wake: online | visibilitychange | pageshow | 1 s timer-gap ─▶ probe ─▶ redial
 WS MsgInput ─▶ InputAbsorber.Filter (lone trailing ESC now passes) ─▶ IsUserInput ─▶ PTY
```

### Recommended Project Structure
```
internal/webserver/
├── whois.go                 # add requireSameUserPage + verdict cache here
├── whois_gate_test.go       # NEW: MOB-05 route x identity matrix
├── webservertest/           # NEW: SameUser/OtherUser fakes. Never imported by non-test code.
├── csp_mw.go                # + manifest-src 'self'
└── server.go                # route wrapping; /app/ Cache-Control Del narrowed to assets/
web/
├── user-refused.html        # NEW embedded refusal page (link-invalid.html style)
└── service-retry.html       # NEW embedded 503 retry page (page routes only)
frontend/
├── index.html               # + manifest, apple-touch-icon, viewport-fit=cover, meta (inert in Wails)
├── public/                  # + manifest.json, apple-touch-icon.png, icon-192.png, icon-512.png
├── src/lib/ctrlMap.ts       # NEW pure Ctrl/arrow mapping
├── src/lib/pinchZoom.ts     # NEW pure pinch maths + handler attach
├── src/lib/visualViewportInset.ts  # NEW hook/util
└── src/components/KeyBar.tsx       # NEW
cmd/playwright-fixture/main.go      # same-user fake + /__test__/whois + drain inputCaptureR
docs/launcher-app.md                # NEW MOB-04 copy for the website repo (path is a planner choice)
```

### Pattern 1: Gate wiring (MOB-05)
**What:** Wrap, innermost last: `cspHeaders(requireSameUserPage(requireCapabilityPage(h)))` for `/sessions/{id}`; `cspHeaders(requireSameUserPage(appHandler))` for `/app/`; `requireSameUser(requireCapability(h))` for `/info` and `/api/plugin-config*`; `requireAllowedOrigin(requireSameUser(requireCapability(handleWSSRelay)))` for the WebSocket.
**When to use:** every route in the locked list. The page variant differs only in what it writes on refusal.
**Why `cspHeaders` stays outermost for pages:** the refusal page loads `/assets/dashboard.css` (web/link-invalid.html line 7: `<link rel="stylesheet" href="/assets/dashboard.css">`), so it must be served under the CSP that allows `style-src 'self'`, and `/assets/` stays open per the locked decision.
**Refusal codes (existing, quoted from whois.go):** `refuseOtherUser` sets `tailnet.MarkerHeader` to `tailnet.MarkerValue` and calls `http.Error(w, "forbidden", http.StatusForbidden)`; `refuseLocalFault` calls `http.Error(w, "unavailable", http.StatusServiceUnavailable)` with no marker [VERIFIED: internal/webserver/whois.go:72-82]. `MarkerHeader = "X-Portty"`, `MarkerValue = "1"` [VERIFIED: internal/tailnet/tailnet.go:35-36].
**Distinguishing the two 403s in the SPA:** `requireCapability` writes `http.Error(w, msg, status)` with no marker [VERIFIED: capability_mw.go:38-47], so `res.headers.get('X-Portty') === '1' && res.status === 403` is exactly the WhoIs refusal. Today App.tsx bootstrap treats any 401/403 as a dead link (App.tsx:937-958 region, read) and `shareInfoVerdict` maps 401/403 to `stop`/`share-stopped` (closeReasons.ts:31-36, read), so both need the marker branch.

### Pattern 2: Lone-Esc fix
**What:** after the byte loop in `InputAbsorber.Filter`, if the state is `stateGotEsc`, emit the ESC and reset.
```go
// Source: derived from internal/relay/oscabsorb.go Filter; xterm sends each key and reply whole (see Findings Q10)
if a.state == stateGotEsc { // a bare ESC ends this MsgInput frame: nothing follows it in this frame
	out = append(out, 0x1b)
	a.state = stateOutside
}
return out
```
Only `stateGotEsc` is touched. A split after the `ESC ]` introducer (tests S1-S5) stays absorbed because that state is `stateInOSC`/`stateInCSI`, not `stateGotEsc`. P9 is the only test that pins the hold [VERIFIED: grep for `Filter([]byte("\x1b"))` finds only oscabsorb_test.go:62]; its new contract is call 1 returns `"\x1b"`, call 2 returns `":"`.
**Side effect to accept:** `IsUserInput` returns false for a trailing lone ESC (comment: "a lone trailing ESC: unterminated"; code `if i+1 >= len(b) { return false }`) [VERIFIED: internal/relay/oscabsorb.go IsUserInput], so Esc alone does not claim the grid. Arrows (`ESC [ A`), Tab and Ctrl-C do.
**Known cost:** a raw terminal behind `portty attach` that splits an auto-reply exactly between ESC and `]` would now leak `]11;…` text into the shell. xterm.js never does; the existing `shortcut:` comment above `IsUserInput` already records this for the claim side.

### Pattern 3: Keyboard inset and safe areas
**What:** on touch devices, set the app root's height to `visualViewport.height` and its top to `visualViewport.offsetTop`, recomputed on `visualViewport` `resize` and `scroll`, and read the values inside `requestAnimationFrame`.
**Why:** Safari does not implement the `interactive-widget` viewport key (caniuse lists iOS Safari unsupported through 26.5; WebKit bug 259770 is NEW) nor the VirtualKeyboard API (WebKit bug 230225), and iOS resizes only the visual viewport [CITED: caniuse.com/mdn-html_elements_meta_name_viewport_interactive-widget_resizes-visual; developer.chrome.com/blog/viewport-resize-behavior]. In standalone mode there are open bugs: `visualViewport.height` unreliable (WebKit 218983), and a report that the viewport shrinks on first keyboard open and never grows back until force-quit, healed by toggling `display` on a full-height element after the input blurs (iOS 17/18 reports) [CITED: dev.to/cederhook/fixing-the-ios-standalone-pwa-keyboard-bug-that-shrinks-your-viewport-for-good-63d; bugs.webkit.org/show_bug.cgi?id=218983]. The rAF read is the workaround reported on WebKit bug 254861.
**Plan consequence:** implement the hook plus a "stuck viewport" heal (max `innerHeight` tracking, check ~140 ms after the helper textarea blurs); mark both `[ASSUMED]` working on iOS 26/27 until M-146 passes.
Safe areas: `viewport-fit=cover` in the viewport meta, `padding-bottom: env(safe-area-inset-bottom)` on the key bar, `env(safe-area-inset-top)` on the root. In Playwright WebKit the insets are `0px` (measured), so test the CSS presence, not the value.

### Pattern 4: Pinch for font size, not page zoom
**What:** a `touchstart/touchmove/touchend` handler on the terminal container, registered `{ passive: false }`. With exactly two touches, `preventDefault()` on `touchmove`, track `d = hypot(dx, dy)`, compute `ratio = d / d0`, and convert to integer font-size steps via the existing `onFontSizeChange(delta)` API; on `touchend`, debounce one claim. Add `document.addEventListener('gesturestart', e => e.preventDefault())` as a belt.
**Block page zoom:** `user-scalable=no` and `maximum-scale` are ignored by iOS Safari since iOS 10 [CITED: quirksmode.org/blog/archives/2020/12/userscalableno.html; WebKit bug 186970 thread], so rely on `touch-action` on the root (`html { touch-action: pan-x pan-y }`) plus the handler's `preventDefault`. Whether iOS 26/27 honours `touch-action` for two-finger zoom was not confirmable from sources: [ASSUMED], covered by M-147.
**Existing seam:** `attachTouchScroll(container, term)` releases multi-touch to the browser today (touchScrollHandler.ts header comment, read), and `.terminal-session-container` has `touch-action: pan-y` (style.css:39) which already withholds pinch-zoom from that element. The new handler should be a sibling module attached in the same `useEffect` (TerminalPanel.tsx:805-811) so the single-finger path stays untouched.
**Web-mode font plumbing is missing:** App.tsx passes `fontSize={mode !== 'web' && activeWebTab ? … : undefined}` and `onFontSizeChange={mode !== 'web' && activeWebTab ? … : undefined}` [VERIFIED: App.tsx:1328-1329], and `WebShareSessionView` falls back to `onFontSizeChange ?? (() => {})`. So today a browser viewer cannot change its font at all. The pinch task must add in-memory font state for `mode === 'web'` (already `fontSizes: Record<string, number>` at App.tsx:124 exists for desktop tabs).

### Pattern 5: Key bar through `term.input`
`term.input(seq, true)` fires `onData` via `coreService.triggerDataEvent(data, wasUserInput)` [VERIFIED: xterm src common/CoreTerminal.ts:166-168 and common/services/CoreService.ts:61-81], so the bytes take `sendInput` unchanged. It returns early when `disableStdin` is set (CoreService.ts:62-65), so a read-only cap viewer gets an inert bar for free; still hide the bar for read-only caps. Arrow sequences: `ESC O A..D` when `term.modes.applicationCursorKeysMode`, else `ESC [ A..D`. Keep `preventDefault()` on `pointerdown` (not `click`) so the helper textarea keeps focus and the keyboard stays up.
```ts
// Source: CONTEXT key-bar decision; byte values are the standard C0 mapping
export function ctrlByte(ch: string): string | null {
  if (ch.length !== 1) return null
  const c = ch.toUpperCase().charCodeAt(0)
  if (c >= 0x41 && c <= 0x5a) return String.fromCharCode(c - 0x40) // A-Z / a-z -> 0x01-0x1A
  if (ch === ' ' || ch === '@') return '\x00'
  if (ch === '[') return '\x1b'; if (ch === '\\') return '\x1c'
  if (ch === ']') return '\x1d'; if (ch === '^') return '\x1e'; if (ch === '_') return '\x1f'
  return null
}
```
Ctrl arrows: `CSI 1;5 A..D` (`\x1b[1;5A`), regardless of application mode.

### Pattern 6: Manifest and head tags
`frontend/public/manifest.json` (served at `/app/manifest.json` because Vite `base: './'` [VERIFIED: vite.config.ts:3-12 read] and the SPA is mounted at `/app/`):
```json
{ "name": "Portty", "short_name": "Portty", "display": "standalone", "scope": "./",
  "background_color": "#000000", "theme_color": "#000000",
  "icons": [ { "src": "icon-192.png", "sizes": "192x192", "type": "image/png" },
             { "src": "icon-512.png", "sizes": "512x512", "type": "image/png" } ] }
```
No `start_url`, no `id`. WebKit defaults `start_url` to the document URL when absent and `id` to the start URL (`parsedManifest.id = parseId(manifest, parsedManifest.startURL)`), and it rejects a cross-origin `start_url`/`scope` [CITED: WebKit ApplicationManifestParser.cpp, parseStartURL/parseScope/parseId]. `scope: "./"` resolves against the manifest URL to `/app/`, and the launch URL `/app/?session=…&cap=…` is inside it. Head additions in `index.html` (relative hrefs, as Vite `base: './'` requires): `<link rel="manifest" href="./manifest.json">`, `<link rel="apple-touch-icon" href="./apple-touch-icon.png">`, `viewport-fit=cover` appended to the existing viewport meta, `<meta name="apple-mobile-web-app-capable" content="yes">`, `<meta name="apple-mobile-web-app-title" content="Portty">`. The meta tag is optional since iOS 11.3 and "not needed to open as an app" since 26.0 [CITED: firt.dev/notes/pwa-ios], and every site opens as a web app by default on iOS/iPadOS 26 [CITED: webkit.org/blog/16993]; keep it as a one-line hedge. An `apple-touch-icon` link overrides manifest `icons` on iOS 15.4+ [CITED: firt.dev/notes/pwa-ios]. Maskable and monochrome are ignored; SVG icons only via manifest [CITED: same]. Desktop impact: index.html is shared with Wails; WKWebView does not fetch a manifest unless the host app asks for it, and the existing meta CSP names only `frame-src`, `child-src`, `worker-src` so it does not restrict `manifest-src`. [ASSUMED] inert; add a vitest that reads index.html and a quick `wails dev` smoke in the VM.

### Anti-Patterns to Avoid
- **Gating by `ws.cspHeaders` inside the gate:** refusal pages would ship without CSP. Keep `cspHeaders` outermost.
- **Detecting touch with `'ontouchstart' in window`:** true even on a desktop Playwright WebKit context (measured). Use `(pointer: coarse) and (hover: none)`.
- **`page.setContent` with `addInitScript` stubs:** init scripts run on navigation; `setContent` did not run mine (measured). Use `page.goto('data:…')` or the fixture URL.
- **Caching refusals or errors in the verdict cache:** only a positive same-user verdict may be cached.
- **Leaving the `/app/` `Del("Cache-Control")` as-is:** it strips caching headers from every file that is not index.html, including the manifest and icons.
- **`start_url` or `id` in the manifest:** either would pin a token or a path; omit both.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Same-user decision | A new identity check | Existing `sameUser(who, self)` + `requireSameUser` | Already handles numeric IDs, tagged, shared-in (`Sharer != 0`), `AltSharerUserID`, missing data [VERIFIED: whois.go:48-63] |
| WhoIs fake | Per-package ad-hoc fakes | One `webservertest` package used by webserver, daemon tests and the fixture | 18 `NewWebServer` test call sites plus `loopbackShareServer` need it; whois_test.go's `fakeWhoIs` is package-private |
| Reconnect | A new reconnect mechanism | `relayClient` wake paths + `pageshow` listener + Reload escalation | Timer-gap detection already covers a missing `visibilitychange` |
| Keyboard layout detection | UA sniffing | `matchMedia('(pointer: coarse) and (hover: none)')` | Verified true under Playwright `hasTouch` and false without it |
| Font-size claim | New claim path | `onFontSizeChange` → existing font effect → claim (TerminalPanel.tsx:1180-1187) | INPT-06 already claims on a user font change |
| Icon pipeline | A build step | One-off `magick` run, commit the PNGs | CONTEXT: "derived once"; `tests/verify-icons.sh` style gate optional |

**Key insight:** every piece of this phase reuses an existing seam; the new code is wiring, one pure mapping module, one pinch module, one hook, one component and static assets.

## Runtime State Inventory

Not a rename/refactor/migration phase. Omitted. One related runtime fact: share state (token, grants) lives in daemon memory, so Home Screen icons die on daemon restart (locked decision); nothing is stored on the phone by the app.

## Common Pitfalls

### Pitfall 1: WhoIs cost is unmeasured and `/app/` makes many requests
**What goes wrong:** Each gated request does two LocalAPI calls (`StatusWithoutPeers`, then `WhoIs`) [VERIFIED: internal/webserver/whois.go:88-116]. A cold `/app/` load is about ten requests (index, chunks, CSS, manifest, icon), plus `/info`, `/api/plugin-config` and the WebSocket. If the LocalAPI is slow or serializes calls, the load is slow and the same queue starves peer discovery and port-proxy traffic.
**Measurement status:** NOT measured in this research. An ad-hoc timing program was run against the host's live tailscaled during this session; the user's firewall flagged it, which breaks the project rule that research and tests never touch the tailnet. It was stopped and deleted at the coordinator's request, and its numbers are withdrawn: do not plan from them. The only remaining basis is code reading: `WhoIs` holds `b.mu` while it looks up the node [CITED: tailscale.com@v1.98.3/ipn/ipnlocal/local.go WhoIs], and each `local.Client` call is a separate HTTP request to the daemon. Anything about latency beyond that is [ASSUMED].
**How to avoid:** plan a measurement as a VM-check item (M-149 step) on the Tart VM's own tailscaled, not the host's. Independent of the result, a positive-only per-IP verdict cache (key the source IP, not IP:port; WhoIs accepts port 0 for tailnet IPs [CITED: tailscale.com@v1.98.3/ipn/ipnlocal/local.go WhoIs doc comment]) with a short TTL (suggest 5 s), bounded size, never caching refusals or errors, is cheap insurance. Staleness is bounded by the TTL, and live sockets are not re-checked anyway (locked). [ASSUMED] the TTL value and the cache itself are a design recommendation; the security reviewer should sign off (T-14-07). If the VM measurement shows negligible cost, drop the cache (Chesterton: do not add it without evidence).
**Warning signs:** `/app/` cold load over 1 s in the VM; peer rows flickering during an `/app/` load.

### Pitfall 2: Existing tests and the fixture fail 503
**What goes wrong:** `NewWebServer` installs `noLiveWhoIs{}` in test binaries (`!tailnet.LiveAllowed()`) [VERIFIED: server.go:178-181], so any test that GETs a newly gated route gets 503. The Playwright fixture is not a test binary (LiveAllowed true) so it uses the real client and a loopback client IP, which WhoIs cannot resolve, so it gets 403.
**How to avoid:** Wave 0 delivers the shared fake, then every affected test installs it. Grep of Go tests naming `/app/`, `/info`, `/sessions/`, `plugin-config`, `/ws` finds about 30 files; 18 `NewWebServer` call sites [VERIFIED: grep this session]. The fixture calls `ws.SetWhoIsClientForTest(...)` (an exported method on a non-test file, whois.go:40) so no fixture-only hook is needed.

### Pitfall 3: `/app/` strips caching headers from the manifest and icons
**What goes wrong:** the `/app/` handler calls `w.Header().Del("Cache-Control")` before `FileServerFS` for every file that exists, because `cspHeaders` sets `no-store` and hashed bundles must be cacheable [VERIFIED: server.go `/app/` handler]. An embedded FS has no modtime, so the manifest and icons end up with no validators and no `Cache-Control`. CONTEXT wants no-cache.
**How to avoid:** only Del when `strings.HasPrefix(rel, "assets/")` (Vite's hashed output dir) and otherwise set `Cache-Control: no-cache`. Add a test over every non-`assets/` file in the bundle.

### Pitfall 4: `.webmanifest` served as text/plain
Measured: Go on this Mac maps `.json` to `application/json` and `.webmanifest` to nothing, so `FileServerFS` sniffs `text/plain; charset=utf-8`. Use `manifest.json`, or set `application/manifest+json` explicitly. No evidence Safari rejects a non-manifest MIME, but the spec expects `application/manifest+json`; [ASSUMED] Safari is lenient.

### Pitfall 5: iOS WebSocket stuck in CONNECTING after a resume
WebKit bug 308073 (filed 2026-02-17, iOS 26.3, still NEW): after a background resume the old socket claims OPEN, pings go unanswered, `onclose` 1006 fires, then every `new WebSocket()` stays at CONNECTING while `fetch` works; only a full page reload fixes it; reproduced in both a Safari tab and Add-to-Home-Screen mode; the reporter used plain `ws://` on a LAN IP, and a commenter says connecting by IP fails while a hostname works [CITED: bugs.webkit.org/show_bug.cgi?id=308073]. Portty serves `wss://` on a MagicDNS hostname, so the exact trigger may not apply, but the shape is the one to defend against.
**How to avoid:** `RECONNECT.connectMs` (15 000) already drops a socket with no host frame; add a counter: after 2 consecutive connect timeouts (about 30 s) show a Reload action on the phone (`location.reload()` keeps the token URL). Include this as a fifth Reload state alongside the four CONTEXT lists.

### Pitfall 6: Wake events are unreliable in standalone mode
The same bug report says `visibilitychange` was not reported firing and used a timer-drift check instead; Apple's developer forums describe wrong or missing visibility signals for Home Screen web apps [CITED: bugs.webkit.org/show_bug.cgi?id=308073; developer.apple.com/forums/thread/123606 as surfaced by search]. `relayClient` already has the right backstop: `checkLiveness` runs every 1 s and a gap over `wakeGapMs` (5 000) calls `onSystemWake` [VERIFIED: relayClient.ts:164-176 and :472-530 read]. Add `pageshow` next to `online`/`visibilitychange` (listeners at :300-303) pointing at `onWake`; do not rely on any single event. [ASSUMED] which of the three fire on iOS 26/27 standalone: not documented anywhere I found, so M-148 records it.

### Pitfall 7: iOS auto-zoom on focus
iOS zooms on focus for form controls under 16 px. The xterm helper textarea sets `autocorrect="off"`, `autocapitalize="off"`, `spellcheck="false"` [VERIFIED: xterm src browser/CoreBrowserTerminal.ts:448-450] and is sized to one cell, but the app's own inputs (FindBar, InlineSessionName, SettingsSheet, SharePopover, PluginsSection) use 11-13 px fonts (style.css has `font-size: 13px` and `12px` rules). Add `@media (pointer: coarse) { input, textarea, select { font-size: 16px } }` (the helper textarea included via `.xterm-helper-textarea`). [ASSUMED] the helper textarea's computed size triggers zoom; M-146 checks that focusing does not zoom.

### Pitfall 8: Smart punctuation and predictive composition
`autocorrect="off"` is set, but iOS smart quotes/dashes and the double-space period are keyboard features. [ASSUMED] they are disabled when autocorrect is off; M-146 types `--`, `"`, `'` and double-space and reads the bytes with `cat -v`. xterm 6 handles IME with `CompositionHelper` (emits the diff, or `DEL` on shrink) [VERIFIED: xterm src browser/input/CompositionHelper.ts:136-202].

### Pitfall 9: Open PDF in a standalone app
`PdfPreview.tsx` opens the PDF with `window.open(url, '_blank', 'noopener,noreferrer')` (PdfPreview.tsx:52). Whether a top-level `blob:` navigation in a Home Screen web app has a way back is unknown. [ASSUMED] it opens in an in-app sheet with Done; M-145 checks. Fallback: hide Open PDF in standalone (`navigator.standalone`) and say so.

### Pitfall 10: Typing on the phone narrows the office grid
Locked behaviour (INPT-06) and README known issue. Put the warning in the launcher copy only if the executor confirms it in the VM; do not change reflow.

### Pitfall 11: Playwright WebKit version mismatch
`pnpm exec playwright test` for 1.59.1 needs `webkit-2272`; only `webkit-2336` is cached. Wave 0 includes `playwright install webkit`. The probes in this research used `executablePath` pointing at 2336 and ran headless; WebKit builds differ from iOS Safari, so emulation proves media queries and layout hooks only.

## Code Examples

### Same-user fake for tests, fixture and VM (non-release package)
```go
// Source: whoIsClient interface in internal/webserver/whois.go:20-23 and SetWhoIsClientForTest :40
package webservertest

import (
	"context"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
)

// WhoIs answers StatusWithoutPeers with a self node of SelfUser and WhoIs with
// the user mapped for the caller's host (default SelfUser). Never imported by release code.
type WhoIs struct {
	SelfUser tailcfg.UserID
	ByHost   map[string]tailcfg.UserID // remote host -> user; missing = SelfUser
}
```
Behaviour and field names are the planner's to finalise; the interface to satisfy is exactly the two methods in `whoIsClient`.

### Release-exclusion guard (Go-native, no grep)
```go
// A repo-root test: the release binary graph must not contain the fake.
// go list -deps with no tags excludes //go:build playwrightfixture and vmpreview files.
out, _ := exec.Command("go", "list", "-deps", ".", "./cmd/...").Output()
if bytes.Contains(out, []byte("internal/webserver/webservertest")) { t.Fatal("fake WhoIs reachable from a release build") }
```
Plus extend `tests/check-brand.sh` with one `git grep` that `webservertest` is imported only from `_test.go`, `cmd/playwright-fixture/` and `*_vmpreview.go`. The existing guard pattern spells words with a bracket class so the script does not match itself (tests/check-brand.sh header, read): copy that (`webserver[t]est`). The `vmpreview` tag already has a guard: check-brand.sh fails on that word in `.github`, `build.sh` and `scripts` (tracked or untracked) [VERIFIED: tests/check-brand.sh:8-13 and the `vm-tag` check].

### Chromium e2e check that the CSP now allows the manifest
```ts
// Source: reproduced this session with Playwright 1.59.1 chromium; today's policy fails, +manifest-src 'self' passes
test.skip(browserName !== 'chromium')
const cdp = await page.context().newCDPSession(page)
await cdp.send('Page.enable')
const m = await cdp.send('Page.getAppManifest')
expect(m.data).toContain('"display":"standalone"')
```

### Icon recipe (run once, commit the outputs)
`build/appicon.png` is 1024x1024 RGBA with a macOS squircle, a shadow and transparent margins (tile roughly x/y 100-924) [VERIFIED: sips and image read]. A full-bleed iOS icon must be opaque (iOS paints transparent areas black): build a 1024 square with the tile's vertical gradient (about `#3067ff` at top to `#204ee6` at bottom), composite the glyph (the white "p" and dark underscore, roughly x 338-686, y 295-730) scaled for the larger tile, `-alpha off`, then export 180 (apple-touch-icon), 192 and 512. Keep one committed 1024 source and a `magick` one-liner in the task.

## Findings by Research Question

| # | Question | Finding | Tag |
|---|----------|---------|-----|
| Q1 | Launch URL with no `start_url` | WebKit defaults `start_url` to the document URL (and `id` to `start_url`); a missing `display` means `browser` | [CITED: WebKit ApplicationManifestParser.cpp]. The query string surviving the real iOS 26/27 Add flow is unverified: older reports conflict (some say iOS cuts the path, others that it uses the page URL). [ASSUMED] works; M-145 gates the design |
| Q1b | `apple-mobile-web-app-capable` | Optional since iOS 11.3, "not needed to open as an app" since 26.0; every site opens as a web app by default on iOS/iPadOS 26 and the user can turn "Open as Web App" off. Keep the meta as a hedge | [CITED: firt.dev/notes/pwa-ios; webkit.org/blog/16993]. Safari 26.0 release notes list "Added support for any website to become a web app on iOS or iPadOS" [CITED: developer.apple.com Safari 26.0 release notes]. Safari 27 notes (read to the end) list no web-app change |
| Q1c | Home Screen storage separation | firt.dev marks "Storage shared with Browser" unsupported (separate), no iOS version given. The app stores nothing on the phone; the token lives only in the Home Screen URL | [CITED: firt.dev/notes/pwa-ios], impact low |
| Q2 | Page CSP vs manifest/icon fetch; MIME | WebKit `CachedResourceLoader::allowedByContentSecurityPolicy` checks `Icon` with `allowImageFromSource` (img-src) and `ApplicationManifest` with `allowManifestFromSource` (manifest-src, which falls back to `default-src` under CSP3), no exemption. Chromium reproduction: manifest blocked under `default-src 'none'`, parsed with `manifest-src 'self'` | [CITED: WebKit CachedResourceLoader.cpp]; Chromium [VERIFIED: experiment]. The fallback-to-default-src step in WebKit was not read directly: [ASSUMED] |
| Q2b | Manifest MIME | `manifest.json` serves `application/json`; `.webmanifest` serves `text/plain` on this Go build | [VERIFIED: experiment] |
| Q3 | WebSocket across suspend and roaming | Old socket dies; reports of new sockets stuck CONNECTING (WebKit 308073, 2026); the 1 s timer-gap detector and probe in `relayClient` cover a missing visibility event; `connectMs` 15 s covers a stuck connect; no Tailscale-specific doc found | [CITED: bugs.webkit.org/show_bug.cgi?id=308073]; Tailscale roaming [ASSUMED], M-148 |
| Q3b | Which of visibilitychange / online / pageshow fire in standalone | Not documented; visibilitychange reported unreliable; pageshow no evidence either way; do not depend on one | [ASSUMED], M-148 records it |
| Q4 | visualViewport, interactive-widget, safe areas | `interactive-widget` unsupported in Safari; visualViewport resize fixed in iOS 13 (WebKit 198347); standalone has open bugs (218983, 254861/254868 comments on iOS 18.3.1) | [CITED] see Pattern 3; fix is [ASSUMED] until M-146 |
| Q5 | Blocking page pinch while capturing it | `user-scalable=no` ignored on iOS; use `touch-action` plus a non-passive `touchmove` handler with touch distance; `gesturestart` absent in Playwright WebKit | [CITED] + [VERIFIED: experiment for gesture absence]; iOS 26/27 behaviour [ASSUMED] |
| Q6 | iOS auto-zoom <16 px; xterm textarea | Textarea has autocorrect/autocapitalize/spellcheck off; auto-zoom and smart punctuation need a device | [VERIFIED: xterm source] + [ASSUMED], M-146 |
| Q10 | Does xterm 6 split a reply or key across `onData` calls? | No. All 27 `triggerDataEvent` sites in xterm src send one whole string (keys, DA1/DA2 `ESC [ ? 6 c`, OSC color replies ending `ESC \`, DSR, mouse, focus). Composition emits the text diff or `DEL`. Paste is one call, bracketed markers included. The only split is client-side: `encodeInputFrames` cuts at `MAX_INPUT_PAYLOAD = 32767` bytes (large pastes) | [VERIFIED: xterm src grep and reads of CoreService.ts, CoreBrowserTerminal.ts, CompositionHelper.ts, Clipboard.ts; relayClient.ts:24 and :80-91] |
| Q7 | Playwright WebKit iPhone emulation | `hasTouch` (iPhone profile or plain) gives `(pointer: coarse)` and `(hover: none)` true; without it false. `visualViewport` exists. `'ontouchstart' in window` true even on desktop. No `gesturestart`. `navigator.standalone` false; safe inset `0px`. `visualViewport` stub works through `addInitScript` + `page.goto` (not `setContent`). Newest profile is iPhone 15 (iOS 17.5 UA) | [VERIFIED: experiment on WebKit build 2336 via executablePath] |
| Q8 | WhoIs cost; host resolving own IP | Cost: not measured (see Pitfall 1); VM item. Own IP: code reading only. Self addresses are indexed in `nodeByAddr` (`updateNodeByAddrLocked` adds `nm.SelfNode` addresses) and `WhoIs` returns that node through `NodeByID`, which also matches `SelfNode`, so a host dialing its own tailnet IP should resolve to itself with the same user. An ad-hoc live check was run and withdrawn (firewall flag), so confirm in the VM | [CITED: tailscale.com@v1.98.3/ipn/ipnlocal/node_backend.go:203-227 and :484-510, read]; behaviour [ASSUMED] until M-149 |
| Q9 | Fake WhoIs without reaching releases | `webservertest` package + `go list -deps` test + check-brand `git grep`; fixture uses `SetWhoIsClientForTest`; VM check uses a `vmpreview`-tagged file (existing guard) | Design, [VERIFIED] guard mechanics read |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `apple-mobile-web-app-capable` required for standalone | Manifest `display` honoured; any site opens as a web app by default | iOS 11.3 / iOS 26 | Meta tag is a hedge only |
| Manifest icons ignored by iOS | Manifest `icons` used; `apple-touch-icon` link overrides | iOS 15.4 | Provide both; PNG only for the touch icon |
| Safari ignored WebSocket over HTTP/2 | Safari 26 added WebSocket over HTTP/2 and HTTP/3 | Safari 26.0 | No action; note for troubleshooting [CITED: Safari 26.0 release notes] |

**Deprecated/outdated:** `user-scalable=no` / `maximum-scale` as a zoom block on iOS.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The query string survives Add to Home Screen when the manifest has no `start_url` on iOS 26/27 | Q1 | The icon opens `/app/` with no token: SC2 fails. M-145 is the gate. Fallback: a launcher/Add flow change, which is a CONTEXT decision |
| A2 | `visibilitychange`, `online` and `pageshow` firing in standalone is unreliable, so the timer-gap path is load-bearing | Pitfall 6 | If all fire reliably the extra trigger is harmless |
| A3 | The standalone keyboard viewport heal works on iOS 26/27 | Pattern 3 | Keyboard may cover the cursor line or leave a gap |
| A4 | `touch-action` blocks two-finger page zoom on iOS 26/27 | Pattern 4 | Page zooms instead of font; add a `gesturestart` `preventDefault` and re-test |
| A5 | The helper textarea triggers iOS focus auto-zoom below 16 px | Pitfall 7 | 16 px rule is harmless if not needed |
| A6 | Smart punctuation and double-space period are off with `autocorrect="off"` | Pitfall 8 | Typed `--` or `"` reaches the shell as Unicode |
| A7 | WebKit's `manifest-src` falls back to `default-src` | Q2 | `manifest-src 'self'` is still needed for Chromium (verified); low risk |
| A8 | WhoIs per-request cost is high enough to justify a 5 s positive-verdict cache; also that a host dialing its own tailnet IP resolves to itself | Pitfall 1, Q8 | Unneeded cache (drop it), or a removed device keeps access up to 5 s; own-IP failure would lock the host out of its own link. Needs a VM measurement and security-review sign-off |
| A9 | `.webmanifest` content type is not required by Safari | Pitfall 4 | Avoided by serving `manifest.json` |
| A10 | WKWebView in the Wails desktop app ignores `<link rel=manifest>` | Pattern 6 | Desktop app behaviour changes; check once in the VM |
| A11 | `window.open(blob)` in a standalone app is escapable | Pitfall 9 | Strands the user; hide Open PDF in standalone |
| A12 | Tailscale iOS app keeps the tunnel through sleep so the socket dies half-open rather than cleanly | Q3 | Different recovery timing; M-148 measures |

## Open Questions (RESOLVED)

1. **Does the Add to Home Screen flow keep `?session=…&cap=…` on iOS 26/27?**
   - What we know: WebKit's parser uses the document URL; some reports on older iOS say the path or query is dropped.
   - What's unclear: current behaviour. No iPhone was available.
   - Recommendation: do M-145 first in the phase (before polishing the launcher copy). If it fails, stop and re-discuss; the token-less `start_url` option is deferred in CONTEXT.
   - **RESOLVED (planning, 2026-11-22):** an early headless iOS Simulator spike, 14-07 (row H-01), runs right after the manifest plan (14-05) and before the key bar, phone layout and launcher plans; its blocking checkpoint offers continue, redesign (route `/gsd-discuss-phase 14`) or continue with the real phone as the primary check. The full simulator run repeats it (14-13, S-04) and 14-14's checkpoint asks for one real-iPhone look, again with the redesign route.
2. **Should the verdict cache exist?** The locked text says "measure the cost"; the measurement says it matters on this rig. Recommend yes, with security-review sign-off; alternative is no cache plus a note that `/app/` load is ~0.5 to 1 s slower.
   - **RESOLVED (planning, 2026-11-22):** no cache now (the earlier host measurement was withdrawn, see Pitfall 1, so there is no evidence for it). 14-12's VM row P-04 measures `StatusWithoutPeers` and `WhoIs` on the VM's own tailscaled and a cold `/app/` load, with the rule "worth discussing above 300 ms per cold load or a p95 over 50 ms"; the user decides at 14-14's checkpoint (UI-B), and a cache would be a gap plan with the TTL signed off in the security review.
3. **Does a lone Esc need to claim the grid?** Currently no. Recommend no change.
   - **RESOLVED (planning, 2026-11-22):** no change; `IsUserInput` keeps returning false for a trailing lone ESC (14-02 pins it).
4. **Where does `docs/launcher-app.md` live?** CONTEXT says a markdown file in this repo; `docs/` currently holds only logos.
   - **RESOLVED (planning, 2026-11-22):** `docs/launcher-app.md`, pinned by the repo-root `launcher_copy_test.go` (14-09).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go (pinned toolchain) | all Go work | yes | go1.27.4 installed; use `GOTOOLCHAIN=go1.26.11` | none needed; guard in toolchain_guard.go |
| Node / pnpm | vitest, Playwright | yes | node v24.22.1 (default), pnpm at ~/Library/pnpm | none |
| Playwright WebKit for 1.59.1 | iPhone e2e project | no (`webkit-2272` missing; `webkit-2336` cached) | — | `pnpm exec playwright install webkit` (network) |
| Playwright Chromium | manifest CSP e2e | yes | cached chromium builds | — |
| Tailscale on host | not used: research and tests must not touch the tailnet | — | — | measure WhoIs cost and own-IP resolution on the VM's tailscaled (M-149) |
| Tailscale in CI (ubuntu) | — | no | — | fixture fake (required by design) |
| Tart VMs | M-149/M-150 | yes | `portty-test`, `portty-test-2` running | `tart` at ~/.local/bin/tart |
| Second Tailscale user | M-149 | no (one account) | — | `vmpreview` fake or user supplies a second login |
| Real iPhone with Tailscale | M-145..M-148 | not available to the agent | — | human check; recorded in TESTING.md Section 5 |
| ImageMagick / sips | icons | yes | magick, sips | either |

**Missing with no fallback:** a real iPhone (human-only checks). **Missing with fallback:** Playwright WebKit build (install), second Tailscale user (fake).

## Validation Architecture

> `workflow.nyquist_validation` is `false` in `.planning/config.json`; this section is included because the caller asked for it.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go test (toolchain go1.26.11), vitest ^4.1.0 (jsdom), Playwright 1.59.1 |
| Config file | `frontend/vite.config.ts` (vitest), `frontend/playwright.config.ts` (add an iPhone project) |
| Quick run command | `GOTOOLCHAIN=go1.26.11 go test -race -short ./internal/webserver/... ./internal/relay/...` and `cd frontend && pnpm exec vitest run <file>` |
| Full suite command | `GOTOOLCHAIN=go1.26.11 go test -race -short ./...`, `cd frontend && pnpm test`, `cd frontend && pnpm exec playwright test` (each full suite runs once per plan, per CONTEXT) |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MOB-05 | Each gated route x {same user, other user, tagged, shared-in, WhoIs error, peer-not-found, tailscaled down} gives 200 / 403+marker / 503; open routes stay open; WS order origin then WhoIs then cap; refusal page names no account | Go integration | `go test -race -short ./internal/webserver -run SameUserGate` | Wave 0: `internal/webserver/whois_gate_test.go` |
| MOB-05 | Verdict cache: positives cached per IP, refusals and errors never, TTL expiry | Go unit | `go test ./internal/webserver -run VerdictCache` | Wave 0: same file |
| MOB-05 | Fake cannot reach a release build | Go + shell | `go test . -run NoFakeWhoIsInRelease`; `bash tests/check-brand.sh` | Wave 0: repo-root test, check-brand edit |
| MOB-05 | SPA shows refusal on 403+marker (bootstrap and probe), LinkNoLongerWorks on plain 403 | vitest | `pnpm exec vitest run App.refusal` | Wave 0: `frontend/src/__tests__/App.sameUser.test.tsx`; edit `closeReasons` test |
| MOB-05 | Playwright: second-user fixture flips to refusal before any terminal data | e2e | `pnpm exec playwright test web-same-user` | Wave 0: `frontend/e2e/web-same-user.spec.ts`, fixture edit |
| MOB-02 | CSP has `manifest-src 'self'` and nothing else changed; no `unsafe-eval` | Go | `go test ./internal/webserver -run CSPHeaders` | edit `csp_mw_test.go` (`TestCSPHeaders_ManifestDelta`) |
| MOB-02 | Manifest MIME, no-cache on non-`assets/` files, hashed bundles still cacheable, no `start_url`/`id`, icons exist at declared sizes | Go + vitest | `go test ./internal/webserver -run AppStatic`; `pnpm exec vitest run manifest` | Wave 0: `internal/webserver/app_static_test.go`, `frontend/public/manifest.test.ts` |
| MOB-02 | Chromium parses the manifest under the real CSP | e2e (chromium only) | `pnpm exec playwright test web-csp --project=chromium` | edit `frontend/e2e/web-csp.spec.ts` |
| MOB-01 | Key bar bytes: Esc, Tab, arrows in both cursor modes, `|`, Ctrl one-shot incl. A-Z, `@[\]^_`, space, Ctrl+arrow `CSI 1;5` | vitest | `pnpm exec vitest run ctrlMap KeyBar` | Wave 0: `frontend/src/lib/__tests__/ctrlMap.test.ts`, `frontend/src/components/__tests__/KeyBar.test.tsx` |
| MOB-01 | Bar shown only under `(pointer: coarse) and (hover: none)`; bytes reach the PTY | e2e iPhone WebKit | `pnpm exec playwright test mobile-iphone --project=iphone` | Wave 0: `frontend/e2e/mobile-iphone.spec.ts`; fixture drains `inputCaptureR` and exposes the captured bytes |
| MOB-01 | Pinch maths and handler (two-touch preventDefault, debounce, one claim, no page zoom call) | vitest | `pnpm exec vitest run pinchZoom` | Wave 0: `frontend/src/lib/__tests__/pinchZoom.test.ts` |
| MOB-01 | visualViewport inset with a stub (keyboard up, stuck-viewport heal) | vitest + e2e stub | `pnpm exec vitest run visualViewportInset` | Wave 0: `frontend/src/lib/__tests__/visualViewportInset.test.ts` |
| MOB-01 | Lone Esc passes at frame end; Esc then `:` in two frames; S1-S5 still absorb; `IsUserInput` bare ESC false | Go | `go test ./internal/relay -run InputAbsorber\|IsUserInput` | edit `oscabsorb_test.go` (P9 rewritten) |
| MOB-01 | 16 px inputs on coarse pointers; phone chrome hides single-tab strip | vitest (CSS source gate) | `pnpm exec vitest run style` | Wave 0 add to an existing CSS gate test |
| MOB-03 | iOS-like suspend: timer gap, no visibilitychange, `pageshow` triggers probe; 2 connect timeouts show Reload; Reload present on stopped/ended/unreachable/ErrorBoundary | vitest | `pnpm exec vitest run relayClient ErrorBoundary` | edit `relayClient.test.ts`, `ErrorBoundary` test; Wave 0 new cases |
| MOB-03 | Real phone: sleep, Wi-Fi to cellular, app switcher | manual | M-148 | Section 5 |
| MOB-04 | Launcher markdown has the link format, Add to Home Screen steps, iCloud warning, no input/token | Go or vitest string pin | `go test . -run LauncherCopy` | Wave 0: `launcher_copy_test.go` or a vitest reading `docs/launcher-app.md` |
| MOB-05, MOB-04 | Share copy updated in all six places | Go + vitest | `go test . -run Share`; `pnpm exec vitest run shareCopy SharePopover` | edit `cmd_share_test.go:108`, `SharePopover.test.tsx:140`, shareCopy and SettingsSheet tests |

### Sampling Rate
- **Per task commit:** the quick command for the touched area (Go package or one vitest file).
- **Per wave merge:** package-level Go + `pnpm test` for the touched directories.
- **Phase gate:** each full suite once per plan, then `/gsd-verify-work`; run `bash tests/check-traceability-paths.sh` and `bash tests/check-brand.sh` before commit.

### Wave 0 Gaps
- [ ] `internal/webserver/webservertest/` fake + migrate the ~18 `NewWebServer` test sites, `loopbackShareServer` in `internal/daemon/share_test.go`, and any test hitting a newly gated route
- [ ] `internal/webserver/whois_gate_test.go`, `internal/webserver/app_static_test.go`
- [ ] Repo-root `NoFakeWhoIsInRelease` test and the `tests/check-brand.sh` line
- [ ] `cmd/playwright-fixture/main.go`: install the same-user fake, add `/__test__/whois` to switch user, drain `inputCaptureR` (currently `_ = inputCaptureR`, main.go:93-94), expose captured bytes
- [ ] `frontend/playwright.config.ts`: add `{ name: 'iphone', use: { ...devices['iPhone 15'] } }`
- [ ] `cd frontend && pnpm exec playwright install webkit`
- [ ] New vitest files listed in the map above

### TESTING.md updates (standing convention)
- **Section 2:** update counts from `git ls-files` (baseline after Phase 13: Go 168, vitest 110, Playwright 8, build-script 1, total 287) and add a dated note line like the existing Phase 13 note.
- **Section 4:** add rows for MOB-01..MOB-05 for every new test file; the path column must be a bare repo-relative path ending in `.go`, `.ts`, `.tsx` or `.sh`, test names in Notes. Run `bash tests/check-traceability-paths.sh`.
- **Section 5:** add `Category BE — Mobile Web App (MOB-01..MOB-05)` after Category BD, with M-145 onward:
  - **M-145** Home Screen install opens the exact link full screen (SC2): Add from Safari on the real iPhone, close, relaunch from the icon, stop sharing and relaunch (Link No Longer Works); also Open PDF has a way back, icon art, name "Portty", whether the query survived.
  - **M-146** Key bar and keyboard (SC1): Esc interrupts Claude Code, Ctrl-C, Ctrl one-shot, arrows in vim and `less` (application cursor), `|`, `cat -v` of `--`, `"`, double-space, no zoom on focus, keyboard inset keeps the cursor line visible, rotation, hardware-keyboard Ctrl.
  - **M-147** Pinch changes the font and not the page; one claim; a relaunch returns to the host font.
  - **M-148** Sleep and roaming (SC3): lock 10 minutes, Wi-Fi to cellular, airplane toggle, app switcher; record time to resume and which of `online`, `visibilitychange`, `pageshow` fired (console via a debug build or Web Inspector over cable); Reload appears when stuck.
  - **M-149** Same-account gate on two users in the VM (SC5), plus on the VM's own tailscaled: time `StatusWithoutPeers` and `WhoIs` per request and a cold `/app/` load, and confirm the host dialing its own tailnet IP resolves to itself (decides whether the verdict cache is kept): other-user device sees the refusal before any terminal data (check the socket never opens), same user passes, host opens its own link, tailscaled stopped gives the retry page, tagged and shared-in refused; uses the `vmpreview`-tagged fake or a second login.
  - **M-150** Lone Esc lag gone in vim and a TUI, in a local tab and in a viewer, before and after (VM first, per CONTEXT).
  - **M-151** Launcher copy read-through against the real Share, AirDrop and Universal Clipboard flow (MOB-04).
- **Section 3:** no `build.yml` matrix change expected; if one changes, re-apply branch protection.

## Security Domain

### Applicable ASVS Categories (level 1)

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Device identity via Tailscale WhoIs numeric user ID plus the existing HMAC capability (second factor) |
| V3 Session Management | yes | Capability token in the URL; `Referrer-Policy: no-referrer` and `no-store` already set by `cspHeaders`; Home Screen bookmark stores the token (T-14-04) |
| V4 Access Control | yes | `requireSameUser` outermost on every session-bearing route; per-route wrapping |
| V5 Input Validation | yes (small) | Static assets only; refusal pages render nothing from the request; Ctrl mapping is a closed table |
| V6 Cryptography | no | None added; TLS and HMAC unchanged |
| V13 API / Web Service | yes | Fixed status codes (403 + marker, 503), WS order origin then WhoIs then cap |
| V14 Configuration | yes | CSP: add `manifest-src 'self'` only; no `unsafe-eval` regression test stays |

### Threat register for 14-SECURITY-REVIEW.md

| ID | Threat | STRIDE | Mitigation / disposition |
|----|--------|--------|--------------------------|
| T-14-01 | Another user's device presents a valid link (leaked or forwarded) | Spoofing / Elevation | WhoIs numeric user ID must equal the host's; tagged, shared-in (`Sharer != 0`), `AltSharerUserID`, self-tagged all refused [VERIFIED: whois.go:48-63]. Gate precedes cap work, so the other device cannot probe cap validity |
| T-14-02 | Source-IP spoofing to borrow a same-user IP | Spoofing | Source IP is a WireGuard-authenticated tailnet address; accepted trust in tailscaled [ASSUMED, same trust as the existing peer routes] |
| T-14-03 | tailscaled fault turns the gate open | Elevation | Fail closed: any error from `StatusWithoutPeers` or `WhoIs` (except peer-not-found, which is a 403) gives 503, never `next` [VERIFIED: whois.go:88-116]. Test every error class |
| T-14-04 | Token in the Home Screen URL is stored in the WebClip and may sync or back up, and sharing the link via Messages stores it in iCloud | Information disclosure | Gate makes a leaked link useless off the host's account (MOB-05); launcher copy warns about Messages/iCloud (locked); icon dies on stop/regenerate/daemon restart; accept residual: a same-account device that syncs the bookmark can open it |
| T-14-05 | CSP widening for the manifest | Tampering | Only `manifest-src 'self'`; manifest is static, same-origin; WebKit rejects a cross-origin `start_url` and `scope` [CITED: WebKit parser]; icons stay under `img-src 'self'`. Tests: `TestCSPHeaders_ManifestDelta` (exact policy), existing no-`unsafe-eval` guard |
| T-14-06 | Refusal page leaks account or host details | Information disclosure | Static embedded page, names no account; carries the `X-Portty` marker (already public to the tailnet) |
| T-14-07 | WhoIs queue starvation by unauthenticated tailnet peers (per-request LocalAPI cost, unmeasured) or stale allow from the verdict cache | Denial of service / Elevation | Positive-only per-IP cache with a short TTL and size bound; refusals and errors never cached; WS and `/info` unaffected by live-socket policy; accept staleness up to TTL, reviewer to approve the TTL [ASSUMED] |
| T-14-08 | Ordering lets an unauthorised caller reach HMAC work or the WS upgrade | Spoofing | WS order origin, WhoIs, cap; assert with a test that a refused caller never reaches `websocket.Accept` |
| T-14-09 | Lone-Esc passthrough weakens the absorber (a reply split leaks text to the shell) | Tampering | xterm never splits (Q10); only a raw terminal behind `attach` could; covered by tests S1-S5 and a new P9; low severity, not a security boundary |
| T-14-10 | Key bar gives a read-only viewer a way to type | Elevation | Bar reuses `term.input`, a no-op under `disableStdin` (CoreService.ts:62-65); hide the bar for read-only caps; server still discards read-only input (`if !sub.ReadOnly`, server.go MsgInput case) |
| T-14-11 | Test fake compiled into a release | Elevation | `webservertest` imported only by tests, the `playwrightfixture`-tagged fixture and `vmpreview`-tagged files; `go list -deps` release test plus the check-brand `git grep` |
| T-14-12 | Stale manifest or icon served from cache after a change | Tampering | `Cache-Control: no-cache` on all non-`assets/` files |
| T-14-13 | Open PDF strands or leaks in standalone | Denial of service (UX) | M-145 check; hide in standalone if no way back |

## Sources

### Primary (HIGH confidence)
- Repo files read this session: `internal/webserver/whois.go`, `capability_mw.go`, `csp_mw.go`, `server.go` (routes, `/app/` handler, `handleTerminalPage`), `internal/relay/oscabsorb.go` and `_test.go`, `relay/server.go`, `internal/tailnet/tailnet.go`, `frontend/src/lib/relayClient.ts`, `closeReasons.ts`, `App.tsx`, `WebShareSessionView.tsx`, `touchScrollHandler.ts`, `style.css`, `index.html`, `vite.config.ts`, `cmd/playwright-fixture/main.go`, `tests/check-brand.sh`, `TESTING.md`
- `graft callers` and `graft skeleton`/`ask` for `requireSameUser` (call site `GET /api/sessions/meta` at server.go:558), `requireCapabilityPage` (server.go:606), `refuseOtherUser` (`refusePort` portproxy.go:47), `SetWhoIsClientForTest` (callers fuzz_test.go:39 and portproxy_test.go:156)
- xterm 6.0.0 source in `frontend/node_modules/@xterm/xterm/src` (CoreService, CoreBrowserTerminal, CompositionHelper, Clipboard, InputHandler) and `typings/xterm.d.ts`
- tailscale.com@v1.98.3 source in the Go module cache (`ipn/ipnlocal/local.go` WhoIs, `node_backend.go`)
- Experiments run this session: Go `FileServerFS` MIME, Playwright WebKit media/touch/visualViewport probes, Chromium manifest CSP reproduction

### Secondary (MEDIUM confidence)
- WebKit source: `Source/WebCore/loader/cache/CachedResourceLoader.cpp`, `Source/WebCore/Modules/applicationmanifest/ApplicationManifestParser.cpp` (github.com/WebKit/WebKit, main)
- webkit.org/blog/16993 (Safari 26 "every site can be a web app"); developer.apple.com Safari 26.0 and 27 release notes
- firt.dev/notes/pwa-ios (iOS PWA support matrix)
- bugs.webkit.org/show_bug.cgi?id=308073 (WebSocket CONNECTING after resume), 198347 (visual viewport resize, fixed iOS 13), 218983 (standalone visualViewport.height), 259770 (interactive-widget NEW), 230225 (VirtualKeyboard API)
- dev.to/cederhook standalone keyboard viewport article; caniuse interactive-widget tables; quirksmode.org userscalable post; developer.chrome.com viewport-resize-behavior

### Tertiary (LOW confidence)
- Search-surfaced forum threads on standalone visibility events and Home Screen `start_url` behaviour on older iOS; no iOS 26/27 primary source found

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, nothing new added, versions read from disk
- Architecture: HIGH for the gate, CSP, filter and tests (source read and experiments); MEDIUM for iOS runtime behaviour
- Pitfalls: MEDIUM, the MIME, CSP and Playwright ones were run locally (no tailnet); WhoIs cost is unmeasured; iOS ones are cited reports needing the real-phone checks

**Research date:** 2026-11-22
**Valid until:** 2026-12-22 for the server findings; re-check the iOS bug links before M-145..M-148 if the phone check slips

## Project Constraints (from CLAUDE.md)

- `TESTING.md` is the regression-suite home: add new test files to Section 2, an M-NN item for anything that cannot be automated (Section 5), a Section 4 row per new test file covering a v4.0 or later requirement (path only, `.go`/`.ts`/`.tsx`/`.sh`), run `bash tests/check-traceability-paths.sh` before committing, and re-apply branch protection if `build.yml` matrix entries change.
- Keep MIT licence text and metadata; macOS-only; keep the inherited Go/Wails/React/tsnet stack; tailnet-only with no new relay or cloud dependency.
- Security: keep token-gated tailnet-only share links and the CSP intact; Chesterton's Fence before deleting anything in shared daemon/relay/capability code (the absorber change touches shared code: the hold is removed deliberately, with P9 rewritten).
- Python venv rules and FastAPI conventions do not apply (no Python in this phase). Go: `go fmt`, `golangci-lint`, `ctx context.Context`. JS/TS: ESLint + Prettier, TypeScript types, `pnpm`.
- Silent fallbacks are forbidden (no `or {}`-style defaults); fail closed in the gate.
- Project rules from memory: GUI checks run in the Tart VM, never on the host; reliability first; new npm deps are pinned exactly (none planned); each full suite runs once per plan; `GOTOOLCHAIN=go1.26.11`.
- GSD workflow enforcement: file-changing work goes through a GSD command; this document is a planning artifact only.
