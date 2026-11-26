# Phase 14: Mobile Web App - Pattern Map

**Mapped:** 2026-11-22
**Files analyzed:** 27 (new and modified)
**Analogs found:** 25 / 27

All analog paths below were read in this pass and exist as tracked source (no `.gsd` capability mirrors are involved in this repo). Line numbers are from the working tree on `gsd/v1.2`.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/webserver/whois.go` (add `requireSameUserPage`, optional verdict cache) | middleware | request-response | `internal/webserver/capability_mw.go` `requireCapabilityPage` + existing `requireSameUser` | exact |
| `internal/webserver/server.go` (route wrapping, `/app/` cache header) | route | request-response | its own routes :558-615 | exact |
| `internal/webserver/csp_mw.go` (+`manifest-src 'self'`) | middleware/config | request-response | its own amendment blocks (Amendment 3, Phase 11) | exact |
| `internal/webserver/whois_gate_test.go` (new) | test | request-response | `internal/webserver/whois_test.go` | exact |
| `internal/webserver/webservertest/` (new fake WhoIs) | test utility | request-response | `fakeWhoIs` / `SameUserWhoIsForTest` in `whois_test.go` :25-70 | role-match (lift out of package-private) |
| `web/user-refused.html` (new) | static page | request-response | `web/link-invalid.html` | exact |
| `web/service-retry.html` (new) | static page | request-response | `web/link-invalid.html` | exact |
| `internal/relay/oscabsorb.go` (lone-ESC flush) | utility (input filter) | streaming | itself, `Filter` :52-80 | exact |
| `internal/relay/oscabsorb_test.go` (P9 changed) | test | streaming | itself, P9 :55-70 | exact |
| `cmd/playwright-fixture/main.go` (same-user fake, drain `inputCaptureR`) | config/fixture | request-response | itself; `SetWhoIsClientForTest` whois.go:40 | exact |
| `tests/check-brand.sh` (+ `webserver[t]est` guard) | test/guard script | batch | itself :1-30 (bracket-class pattern) | exact |
| release-exclusion Go test (repo root) | test | batch | none in repo for `go list -deps`; use RESEARCH snippet | no analog |
| `frontend/index.html` (manifest, icons, `viewport-fit=cover`) | config | static | itself :1-12 | exact |
| `frontend/public/manifest.json` + 3 PNG icons | config/asset | static | `frontend/public/portty-title-logo.png` (placement only) | partial |
| `frontend/src/lib/ctrlMap.ts` (new, pure) | utility | transform | `frontend/src/lib/terminalScale.ts` (+ `.test.ts`) | role-match |
| `frontend/src/lib/pinchZoom.ts` (new) | utility/hook | event-driven | `frontend/src/lib/touchScrollHandler.ts` | exact |
| `frontend/src/lib/visualViewportInset.ts` (new) | hook/utility | event-driven | `frontend/src/lib/useColorScheme.ts` (hook shape) | partial |
| `frontend/src/components/KeyBar.tsx` (new) | component | event-driven | `frontend/src/components/LinkNoLongerWorks.tsx` (props-light component), `SessionEndedBanner.tsx` | partial |
| `frontend/src/components/TerminalPanel.tsx` (attach pinch, onData) | component | event-driven | its own effect :800-812 | exact |
| `frontend/src/components/WebShareSessionView.tsx` (font state, Reload) | component | request-response | itself :136-210 | exact |
| `frontend/src/App.tsx` (bootstrap marker branch, web font state) | component | request-response | itself :936-960 | exact |
| `frontend/src/components/UserRefused` text (or extend `LinkNoLongerWorks.tsx`) | component | static | `LinkNoLongerWorks.tsx` | exact |
| `frontend/src/lib/closeReasons.ts` (`shareInfoVerdict` marker branch) | utility | transform | itself :28-36 | exact |
| `frontend/src/lib/relayClient.ts` (+`pageshow`, connect-timeout counter) | service | event-driven | itself :296-304 | exact |
| `frontend/src/lib/shareCopy.ts`, `SettingsSheet.tsx:394`, `cmd_share.go:91`, `README.md:22` + tests | copy | static | themselves | exact |
| `frontend/playwright.config.ts` (iPhone project), `e2e/*.spec.ts` | test | request-response | `playwright.config.ts` projects :36-45, `e2e/web-csp.spec.ts` | exact |
| `docs/launcher-app.md` (new) | docs | static | none (`docs/` holds only images) | no analog |
| `TESTING.md` (suite manifest, traceability rows, M-145..M-150) | docs | static | TESTING.md Phase 13 note :36, M-140..M-143 :1723-1743 | exact |

## Pattern Assignments

### `internal/webserver/whois.go` and `server.go` route wrapping (middleware, request-response)

**Analog:** `internal/webserver/capability_mw.go` (page-variant pattern) and `whois.go` (gate).

**Existing gate** (`whois.go` :88-116, unchanged for API routes):
```go
func (ws *WebServer) requireSameUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		st, err := ws.whois.StatusWithoutPeers(ctx)
		if err != nil || st == nil || st.Self == nil { refuseLocalFault(w); return }
		who, err := ws.whois.WhoIs(ctx, r.RemoteAddr)
		if err != nil && !errors.Is(err, local.ErrPeerNotFound) { refuseLocalFault(w); return }
		if err != nil || !sameUser(who, st.Self) { refuseOtherUser(w); return }
		next(w, r)
	}
}
```
Refusals: `refuseOtherUser` (:72-75) sets `tailnet.MarkerHeader`/`MarkerValue` then 403; `refuseLocalFault` (:80-82) is a bare 503.

**Page variant to copy** (`capability_mw.go` :52-73): write a static embedded page on refusal; nothing from the request is rendered:
```go
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.Header().Set("X-Content-Type-Options", "nosniff")
w.WriteHeader(status)
_, _ = w.Write(linkInvalidPage)
...
var linkInvalidPage = func() []byte {
	b, err := webfs.WebFS.ReadFile("link-invalid.html")
	if err != nil { panic("webserver: embedded link-invalid.html: " + err.Error()) }
	return b
}()
```
For `requireSameUserPage`: factor the check into a shared `(status int)` helper so both wrappers share the decision, then write `userRefusedPage` (403, plus the marker header, per CONTEXT) or `serviceRetryPage` (503). Do not duplicate the decision logic.

**Wiring** (`server.go` :558-615). Current lines to change:
```go
mux.HandleFunc("GET /api/sessions/{id}/info", ws.requireCapability(ws.handleSessionInfo))          // :588
mux.HandleFunc("GET /api/plugin-config", ws.requireCapability(ws.handleGetPluginConfig))           // :594
mux.HandleFunc("GET /api/plugin-config/stream", ws.requireCapability(ws.handleStreamPluginConfig)) // :599
ws.cspHeaders(ws.requireCapabilityPage(ws.handleTerminalPage))                                      // :606
ws.requireAllowedOrigin(ws.requireCapability(ws.handleWSSRelay))                                    // :615
mux.HandleFunc("GET /app/", ws.cspHeaders(func(w http.ResponseWriter, r *http.Request) {...}))     // :667
```
Target shapes (from RESEARCH Pattern 1): `requireSameUser(requireCapability(h))` for the API routes, `cspHeaders(requireSameUserPage(requireCapabilityPage(h)))` for `/sessions/{id}`, `cspHeaders(requireSameUserPage(appHandler))` for `/app/`, `requireAllowedOrigin(requireSameUser(requireCapability(handleWSSRelay)))` for the WebSocket. Leave `GET /api/sessions` (:583, `porttyMarker(...)`), `/assets/`, `/`, `/dashboard` untouched. Existing already-gated examples to mirror: `:558` `ws.requireSameUser(ws.handleSessionsMeta)`, `:563`, `:575`.

**`/app/` cache-header fix** (`server.go` ~:700-705): the existing `w.Header().Del("Cache-Control")` runs for every non-index file. Narrow it:
```go
if strings.HasPrefix(rel, "assets/") { w.Header().Del("Cache-Control") } else { w.Header().Set("Cache-Control", "no-cache") }
```
Keep the Phase 176 WR-02 comment; update it rather than removing it (Chesterton: it explains why the Del exists).

**Tests never touch the tailnet:** `NewWebServer` (server.go :178-181) installs `noLiveWhoIs{}` when `!tailnet.LiveAllowed()`; tests that hit newly gated routes must install the same-user fake.

---

### `internal/webserver/webservertest/` and `whois_gate_test.go` (test utility and test, request-response)

**Analog:** `internal/webserver/whois_test.go` :25-70.

```go
const testSelfUser tailcfg.UserID = 42

type fakeWhoIs struct {
	self     *ipnstate.Status
	byAddr   map[string]*apitype.WhoIsResponse
	err      error
	whoisErr error // fails WhoIs only
}
func (f *fakeWhoIs) WhoIs(_ context.Context, remoteAddr string) (*apitype.WhoIsResponse, error) { ... local.ErrPeerNotFound for unknown hosts ... }
func (f *fakeWhoIs) StatusWithoutPeers(context.Context) (*ipnstate.Status, error) { ... }

func whoIsAs(peerUser tailcfg.UserID) *fakeWhoIs {
	return &fakeWhoIs{
		self:   &ipnstate.Status{Self: &ipnstate.PeerStatus{UserID: testSelfUser}},
		byAddr: map[string]*apitype.WhoIsResponse{"127.0.0.1": {Node: &tailcfg.Node{User: peerUser}}},
	}
}
func SameUserWhoIsForTest() *fakeWhoIs { return whoIsAs(testSelfUser) }
```
Note an exported `SameUserWhoIsForTest` already exists in a `_test.go` file for the `webserver_test` package. The new `webservertest` package is what the fixture and daemon tests can import (a `_test.go` symbol is not importable). Move the fake out of `whois_test.go`, keep the `whoIsClient` interface (whois.go :20-23) as the contract, and make `whois_test.go` import it. Test matrix for `whois_gate_test.go`: route x identity (same user, other user, tagged, shared-in, ErrPeerNotFound, status error, WhoIs error) with the existing `TestSameUser` table style (whois_test.go :72+).

**Release guard:** use the `go list -deps` snippet in RESEARCH "Release-exclusion guard", and extend `tests/check-brand.sh` using its header's bracket-class convention (`webserver[t]est`) so the script never matches itself.

---

### `web/user-refused.html`, `web/service-retry.html` (static page)

**Analog:** `web/link-invalid.html` (whole file, 15 lines).
```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Portty</title>
  <link rel="stylesheet" href="/assets/dashboard.css">
</head>
<body>
  <div class="container">
    <h1>Portty</h1>
    <h2 class="error">Link No Longer Works</h2>
    <p>This link was turned off or is incomplete. ...</p>
  </div>
</body>
</html>
```
Copy structure exactly. Refusal copy (CONTEXT): h2 "This link only works on your own devices", body "Open it on a device signed in to the same Tailscale account as the Mac that shared it." Name no account. Embed via the same `webfs.WebFS.ReadFile` + `panic` pattern; confirm `web/embed.go` embed directive covers the new files.

---

### `internal/relay/oscabsorb.go` lone-ESC fix (utility, streaming)

**Analog:** itself. State machine in `Filter` (:52-80): `stateOutside` -> `stateGotEsc` on 0x1b; `stateGotEsc` + `]`/`[` enter OSC/CSI; default flushes `0x1b, b`. Add after the byte loop, before `return out` (RESEARCH Pattern 2):
```go
if a.state == stateGotEsc { // a bare ESC ends this MsgInput frame
	out = append(out, 0x1b)
	a.state = stateOutside
}
```
Touch only `stateGotEsc`. States `stateInOSC`/`stateInCSI` stay absorbing across frames (tests S1-S5). `IsUserInput` (:209-234) returns false for a trailing lone ESC; leave it (a lone Esc does not claim, by design).

**Test to change deliberately:** `oscabsorb_test.go` :55-70, `TestInputAbsorber_P9_BareEscThenColon`. New contract: call 1 `Filter("\x1b")` returns `"\x1b"`; call 2 `Filter(":")` returns `":"`. Keep the table-driven `bytes.Equal` assertion style; update the doc comment above it.

---

### `frontend/src/lib/pinchZoom.ts` (utility, event-driven)

**Analog:** `frontend/src/lib/touchScrollHandler.ts` (`attachTouchScroll(container, term): () => void`, :1-60+). Copy: the header doc block style with phase/requirement refs, module-level threshold constants, `readCellHeight` live reads, and a returned cleanup function; sibling attach in the same effect.

**Attach seam** (`TerminalPanel.tsx` :800-812):
```tsx
useEffect(() => {
  const container = containerRef.current
  const term = termRef.current
  if (!container || !term) return
  const cleanup = attachTouchScroll(container, term)
  return cleanup
}, [sessionId])
```
Add `attachPinchZoom(container, onSteps)` here (combine cleanups). Register `touchmove` with `{ passive: false }`; two touches only; keep the single-finger path untouched (touchScroll releases multi-touch to the browser today). Font plumbing: `App.tsx` :1328-1329 passes `fontSize`/`onFontSizeChange` only when `mode !== 'web'`; add in-memory state for web mode using the existing `fontSizes: Record<string, number>` (App.tsx :124). Claims go through the existing font effect (TerminalPanel.tsx :1180-1187); debounce one claim at pinch end.

**Test style analog:** `frontend/src/lib/terminalScale.test.ts` (vitest `describe/it/expect`, comment-per-case with the arithmetic).

---

### `frontend/src/lib/ctrlMap.ts` (utility, transform)

**Analog:** `frontend/src/lib/terminalScale.ts` (pure functions, separate `.test.ts` beside it). Use the `ctrlByte` and Ctrl+arrow (`\x1b[1;5A..D`) mapping from RESEARCH Pattern 5 verbatim. Arrow sequences: `\x1bO A..D` when `term.modes.applicationCursorKeysMode`, else `\x1b[ A..D`.

### `frontend/src/components/KeyBar.tsx` (component, event-driven)

**Analog:** `LinkNoLongerWorks.tsx` for the minimal functional-component form (`React.ReactElement` return, BEM-ish `link-dead__heading` class naming, doc comment with requirement ID). No close analog for a toolbar; planner should check `TabBar.tsx` for button/aria conventions. Behaviour from RESEARCH Pattern 5: `term.input(seq, true)` (typings `input(data, wasUserInput?)`); `onPointerDown={e => e.preventDefault()}` keeps the helper textarea focused; show only when `matchMedia('(pointer: coarse) and (hover: none)')`; hide for read-only caps. Ctrl is one-shot state consumed by both the bar and the iOS keyboard path (hook the `onData` site, TerminalPanel.tsx ~:675).

### `frontend/src/lib/visualViewportInset.ts` (hook, event-driven)

**Analog:** `frontend/src/lib/useColorScheme.ts` for the hook-with-listener-cleanup shape (read it before writing). Listeners on `visualViewport` `resize` and `scroll`, read inside `requestAnimationFrame`; add the stuck-viewport heal from Pattern 3.

---

### Bootstrap and reconnect marker branch (App.tsx, closeReasons.ts, relayClient.ts)

**Bootstrap analog** (`App.tsx` :936-960):
```tsx
fetch(`/api/sessions/${encodeURIComponent(sessionId)}/info?cap=${encodeURIComponent(capToken)}`)
  .then(async (res) => {
    if (cancelled) return
    if (res.status === 401 || res.status === 403) { setWebLinkDead(true); return }
    ...
```
Add before it: `if (res.status === 403 && res.headers.get('X-Portty') === '1') { setWebOtherUser(true); return }` and render the refusal text (reuse the `LinkNoLongerWorks` structure with the `link-dead` classes).

**Verdict analog** (`closeReasons.ts` :31-36): `shareInfoVerdict(status)` maps 401/403 to `{kind:'stop', reason:'share-stopped'}`. Extend the signature with a marker flag (or accept headers) and add a new stop reason for the other-account case; extend the closeReasons tests.

**Wake listeners** (`relayClient.ts` :298-303):
```ts
window.addEventListener('online', this.onWake)
window.addEventListener('portty:wake', this.onSystemWake)
document.addEventListener('visibilitychange', this.onWake)
```
Add `window.addEventListener('pageshow', this.onWake)` and remove it in the matching teardown. Unit-test with the existing `relayClient.test.ts` fake-timer style (simulate a timer-gap suspend).

---

### `internal/webserver/csp_mw.go` (config)

**Analog:** itself. The file documents each policy change as a numbered "Amendment" with rationale, date and phase; add "Amendment 4 (Phase 14, MOB-02)" the same way, add `manifest-src 'self'` to the policy string, and update the doc policy and the existing CSP tests (`csp_mw_test.go`, incl. the token-aware `NoUnsafeEval` guard). Security-review sign-off is a CONTEXT requirement.

### `frontend/index.html` and manifest (config)

**Analog:** `frontend/index.html` :1-12 (shared with Wails; carries a meta CSP with only `frame-src`, `child-src`, `worker-src`, so it does not restrict `manifest-src`). Append `viewport-fit=cover` to the existing viewport meta; add relative-href links (Vite `base: './'`): `<link rel="manifest" href="./manifest.json">`, `<link rel="apple-touch-icon" href="./apple-touch-icon.png">`. Use the manifest JSON in RESEARCH Pattern 6 (no `start_url`, no `id`, `scope: "./"`). Add a vitest that reads index.html to assert those tags.

### Playwright (test, request-response)

**Analog:** `frontend/playwright.config.ts` projects array (chromium, firefox entries of the form `{ name, use: { ...devices['Desktop Chrome'] } }`); add `{ name: 'iphone', use: { ...devices['iPhone 15'] } }` (newest profile in 1.59.1). Spec analog: `e2e/web-csp.spec.ts`; use the Chromium CDP `Page.getAppManifest` snippet in RESEARCH. Fixture: `cmd/playwright-fixture/main.go` is not a test binary (LiveAllowed true), so call `ws.SetWhoIsClientForTest(webservertest...)` and drain `inputCaptureR` for key-bar tests. Wave 0 needs `pnpm exec playwright install webkit`.

### `TESTING.md` (docs, standing rule)

**Analog:** Phase 13 note (TESTING.md :36) and the M-NN items (:1723-1743). Per `CLAUDE.md`: add new test files to Section 2, traceability rows to Section 4 (repo-relative `.go/.ts/.tsx/.sh` paths only), M-145..M-150 to Section 5; run `bash tests/check-traceability-paths.sh` before committing.

## Shared Patterns

### Fail-closed refusal codes
**Source:** `internal/webserver/whois.go` :72-82. 403 + `X-Portty: 1` for another account, 503 with no marker for a local tailscaled fault. **Apply to:** every newly gated route; the SPA branches on the marker (a cap 403 from `requireCapability` has none, capability_mw.go :38-47).

### Static embedded pages with no request data
**Source:** `capability_mw.go` :52-73 and `web/link-invalid.html`. **Apply to:** both new pages; `cspHeaders` stays outermost so the page loads `/assets/dashboard.css` under the CSP.

### Tests never touch the tailnet
**Source:** `server.go` :178-181 (`noLiveWhoIs`), `whois_test.go` fakes. **Apply to:** all Go tests on gated routes (about 30 files, 18 `NewWebServer` call sites), the fixture, and any VM-only fake (`vmpreview` tag, guarded by `tests/check-brand.sh`).

### Pure-module-plus-colocated-test
**Source:** `terminalScale.ts` / `terminalScale.test.ts`. **Apply to:** `ctrlMap.ts`, `pinchZoom.ts` maths, `visualViewportInset.ts`.

### INPT-06 claim rule
**Source:** TerminalPanel.tsx :1132-1187. Typing, pinch or rotation claims; joining never does. Key-bar bytes go through `term.input(seq, true)` so they count as typing.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| Release-exclusion `go list -deps` test | test | batch | No existing Go test shells out to `go list`; use the RESEARCH snippet, with `check-brand.sh` as the shell-side guard analog |
| `docs/launcher-app.md` | docs | static | `docs/` holds only images; planner picks the path (RESEARCH: `docs/launcher-app.md`); copy spec is in CONTEXT |
| Icon PNGs (`apple-touch-icon.png`, `icon-192.png`, `icon-512.png`, 1024 source) | asset | static | Generated once from `build/appicon.png` using the RESEARCH `magick` recipe |

## Metadata

**Analog search scope:** `internal/webserver`, `internal/relay`, `web/`, `cmd/playwright-fixture`, `tests/`, `frontend/src/{lib,components}`, `frontend/e2e`, `frontend/playwright.config.ts`, `frontend/index.html`, `TESTING.md`.
**Files scanned:** about 30 read in whole or in part (RESEARCH.md was read to line 404 of 555; the remainder was not needed for file selection).
**Pattern extraction date:** 2026-11-22
