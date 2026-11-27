# Phase 22: Click-to-Share for Tailnet People - Pattern Map

**Mapped:** 2026-11-26
**Files analyzed:** 31 new or modified
**Analogs found:** 29 / 31 (2 partial: the physical-input tap, the GUI liveness stream)

All analog paths are git-tracked source (checked with `git ls-files`); no gitignored mirror paths appear. All people and addresses are placeholders. Line numbers are as of 2026-11-26 on branch `gsd/phase-17-desktop-feasibility-spike`.

## Current-state ledger (read this before citing any symbol)

| Status | Symbols |
|--------|---------|
| ON DISK (Phase 19) | `internal/desktop` Manager (`ServeDesktopViewer`, `join`, `leave`, `CloseAll` (concurrent close), `ViewerList`, `Full`, `Viewers`), `Viewer{ID, Rights, JoinedAt}`, `Rights` bit set (`RightsViewOnly`, `RightsControl`, `RightsClipboard`, `RightsSurface`, `RightsOwnAccount`) in `viewer.go`; `capability.DesktopKey/SignDesktop/VerifyDesktop`; `webserver.DesktopBackend` + `desktopTokenOutcome` + `setupDesktopRoutes` in `desktop_routes.go`; daemon `desktopAdapter`, `registerDesktopRoutes`, `serverNeededLocked`; `desktop.json`; hello `Caps`/`Rights` map; tray `setTrayDesktopTitle`; Settings Desktop group |
| ON DISK (Phase 20 plan 20-01) | `Manager.handleInput(v *viewer, msg inputMsg)` single entry point (`input.go:390`), `enqueueInput`, `gate()` (target/epoch/lock only, no rights), `ReleaseViewer(id, reason)`, `endViewerInput`, `releaseAllViewers`, `takeHeldLocked`, `acceptDataChannel` (requires `vw.inputCap == 1`), `Deps.Input` bridge, `screencap.EventMarker` tagging, hello `Rights` all true |
| NOT ON DISK: `Viewer.Rights` is set once at join (`manager.go:548 rights := RightsOwnAccount`, `:594`) and never mutated; nothing in `input.go` reads `Rights` today | |
| PLANNED (Phase 20, not yet on disk) | 20-02: `pressAllowed(v, msg)` in `gates.go` checking `RightsControl` first (drop reason `no_right`), releases bypass gates, dead-man 3 s, per-viewer token bucket. 20-03: `handleSelect` checks `RightsSurface` per viewer. 20-05: `handleClipGet` / `handleClipPut` check `RightsClipboard` per action, answer `forbidden`. 20-06: `desktopInput.ts` (`createDesktopInput`, `setControl`, `releaseAll`), `DesktopClient.sendInput`, events `rights`, `input`, `surface`, `control`. **There is no runtime "set rights", "lease hook" or "live rights message" in any 20-xx plan**: Phase 22 must add them (see Manager additions below) |
| PLANNED (Phase 21, executing concurrently) | `desktopClient.ts` / `DesktopView.tsx` copy-override table and Phase 21 viewer states; Portty-app Desktop tab. Treat any symbol from `.planning/phases/21-portty-app-and-iphone-viewers/21-*-PLAN.md` as not on disk; re-read `git log` before planning touches `DesktopView.tsx` or `desktopClient.ts` |
| NOT BUILT (Phase 22 creates) | `requireTailnetPeer(Page)`, guest token, guest share registry, admission queue, arbiter, audit appender, GUI liveness stream, tray indicator/line, all sheets |

## File Classification

| New / Modified File | Role | Data Flow | Closest Analog | Match |
|---------------------|------|-----------|----------------|-------|
| `internal/webserver/whois.go` (+ `requireTailnetPeer`, `requireTailnetPeerPage`, `tailnetPeerStatus`, `tailnetPeer`) | middleware | request-response | same file: `sameUser` / `sameUserStatus` / `requireSameUser(Page)` | exact |
| `internal/webserver/webservertest/whois.go` (extend fake) | test fake | request-response | same file | exact (extension) |
| `internal/capability/desktopguesttoken.go` (+ test) | utility | transform | `internal/capability/desktoptoken.go`, `porttoken.go` | exact |
| `internal/webserver/desktop_routes.go` or new `desktop_guest_routes.go` | route | request-response + WebSocket | `desktop_routes.go` `setupDesktopRoutes`, `handleDesktopWS` | exact |
| `internal/webserver/server.go` `setupRoutes` (one call line) | route | request-response | `ws.setupDesktopRoutes(mux)` call at `:744` | exact |
| `internal/webserver/desktop_guest_routes_test.go` (route-table pin test, cross-token) | test | request-response | `whois_gate_test.go`, `crosstoken_test.go`, `desktop_routes_test.go` | exact |
| `web/desktop-guest-link-invalid.html`, `web/desktop-guest-refused.html` | static page | request-response | `web/user-refused.html`, `web/desktop-link-invalid.html` | exact |
| `internal/desktop/share.go` (guest share registry, `share_id`) | service | CRUD in memory | `internal/daemon/share.go` `mintShareRecord`, `unshareLocked` | role-match |
| `internal/desktop/admission.go` | service | event-driven queue | `webserver.AddGrant/ClearGrants/revokeSignal/isGrantActive` (`server.go:278-321`) | role-match |
| `internal/desktop/arbiter.go` (grant gate, 2 s lease, physical-wins) | service | request-response (per message) | `internal/desktop/input.go` `gate()` + 20-02 `pressAllowed` (PLANNED) | partial |
| `internal/desktop/viewer.go` + `manager.go` (live rights, identity on `Viewer`, join/leave hooks, release-on-change) | model/service | CRUD | `viewer.go` `Rights`; `manager.go` `join`/`leave`/`CloseAll` | exact (extension) |
| `internal/desktop/revoke.go` (ordered concurrent teardown) | service | batch | `Manager.CloseAll` (`manager.go:980`), `unshareLocked` (`daemon/share.go:357`) | exact |
| `internal/desktop/audit.go` (+ test) | service | file-I/O append | `internal/daemon/daemon_log.go` `daemonLog` | role-match |
| `internal/screencap/inputtap_darwin.m` (+ Go wrapper) | native utility | event-driven | none for taps; ObjC/cgo shape from existing `internal/screencap` bridges | partial |
| `internal/daemon/desktop_share_api.go` (start/stop, viewers, admit/deny, grant/revoke, events stream) | controller | request-response + streaming | `internal/daemon/desktop_api.go` `registerDesktopRoutes`, `desktopAdapter` | role-match |
| `internal/daemon/client.go` (new `DaemonClient` methods) | client | request-response | `DesktopLink`, `DesktopPermissions` (`client.go:542,555`) | exact |
| `app_desktop_share.go` (Wails bindings) + `app.go` notification routing | controller | request-response | `app_desktop.go`; `emitEvent` (`app.go:1060`); `app_update.go` `handleNotificationClick` | exact |
| `tray.go`, `tray_objc_darwin.m` (line, Stop item, icon variant) | UI native | event-driven | `setTrayDesktopTitle`, `setTrayUpdateItem`, `trayIconBytesForState` | exact |
| `frontend/src/components/ShareDesktopSheet.tsx` | component | request-response | `DesktopSetupSheet.tsx`, `SharePopover.tsx` | exact |
| `frontend/src/components/AdmitSheet.tsx` | component | event-driven | `QuitConfirmModal.tsx`, `DesktopSetupSheet.tsx` | role-match |
| `frontend/src/components/ViewerList.tsx` | component | CRUD | `SettingsSheet.tsx` `Row` + `Switch` | role-match |
| `frontend/src/components/ShareBanner.tsx` | component | event-driven | `TailscaleBanner.tsx` | exact |
| `frontend/src/lib/shareDesktopCopy.ts` | utility | transform | `frontend/src/lib/shareCopy.ts` | exact |
| `frontend/src/lib/useDialogFocus.ts` (+ `initialFocus: 'dialog'`) | hook | event-driven | same file | exact (extension) |
| `frontend/src/components/DesktopView.tsx`, `lib/desktopClient.ts` (guest mode) | component | streaming | same files (Phase 19 built; Phase 20/21 PLANNED edits) | exact, merge risk |
| `frontend/src/components/SettingsSheet.tsx`, `QuitConfirmModal.tsx` (Row 6, quit sentence, Security string) | component | request-response | same files | exact |
| `TESTING.md` (+ `tests/check-traceability-paths.sh` run) | docs | n/a | `TESTING.md` Sections 2, 4, 5, 6 | exact |

## Pattern Assignments

### `internal/webserver/whois.go`: `requireTailnetPeer` (middleware, request-response)

**Analog:** same file, `sameUser` (`:54-68`), `sameUserStatus` (`:88-111`), `requireSameUser` (`:117-128`), `requireSameUserPage` (`:133-150`). Do not touch `sameUser`.

**Fail-closed status shape to copy** (`whois.go:88-111`):
```go
func (ws *WebServer) sameUserStatus(r *http.Request) int {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	st, err := ws.whois.StatusWithoutPeers(ctx)
	if err != nil || st == nil || st.Self == nil {
		return http.StatusServiceUnavailable
	}
	who, err := ws.whois.WhoIs(ctx, r.RemoteAddr)
	if err != nil && !errors.Is(err, local.ErrPeerNotFound) {
		return http.StatusServiceUnavailable
	}
	if err != nil || !sameUser(who, st.Self) {
		// numeric IDs only in the debug log
		return http.StatusForbidden
	}
	return 0
}
```
New predicate differs only here: do NOT compare `who.Node.User` to `self.UserID`; require `who.Node.User != 0`, not tagged, `Sharer == 0`, `who.UserProfile != nil` (the existing fake returns nil, so the fake must change first), and keep the self-sanity lines from `sameUser` (`:61-66`: tagged self, `AltSharerUserID`, `UserID == 0`). Return a `peerIdentity{UserID, LoginName, DisplayName, NodeName (Node.ComputedName), StableID (Node.StableID), IP}` for the admission; sanitize names in the daemon, not here. Because the guest WS handler needs the identity, `tailnetPeerStatus` should return `(int, peerIdentity)` and the wrappers pass the identity via a request-context value (no analog for context passing in `whois.go`; the `capability/context.go` file is the repo precedent for context values).

**Page wrapper to copy** (`whois.go:133-150`): same code but pick `desktopGuestRefusedPage` for 403 (keeps `tailnet.MarkerHeader` set) and `serviceRetryPage` for 503 (no marker). `mustWebFile` at `:159` loads the embedded page; add the two new pages to the `web` embed (check the `webfs.WebFS` embed pattern covers `*.html`).

**Narrowness proof:** only guest routes call it. The route-table test (below) is the proof.

---

### `internal/webserver/webservertest/whois.go`: extend the WhoIs fake

**Which fake exists:** `webservertest.WhoIs` (package `internal/webserver/webservertest`), installed with `ws.SetWhoIsClientForTest(webservertest.New(id))` (`whois.go:41` for the setter; used in `capability_helpers_test.go:128,159`, `whois_gate_test.go`). A second seam, `noLiveWhoIs` (`whois.go:30`), fails tests closed when no fake is installed. Keep it.

**What it models today** (`webservertest/whois.go:27-35, 83-110`): one `Identity` for every caller (`Same`, `Other`, `Tagged`, `SharedIn`, `NotFound`, `StatusFault`, `WhoIsFault`); self is `UserID: SelfUser (42)`; the answer is `&apitype.WhoIsResponse{Node: node}` with only `Node.User`, `Node.Tags`, `Node.Sharer` set.

**What Phase 22 needs it to model (all missing):**
1. `UserProfile{ID, LoginName, DisplayName}`: today nil. The new gate refuses nil, and the Admit prompt, audit and sanitizer tests need names (including bidi/control characters).
2. `Node.ID`, `Node.StableID`, `Node.ComputedName` per identity, for the (share_id, user ID, stable node ID) admission key and the "same person, second device" case.
3. Per-remote-address identity: today `WhoIs(context.Context, string)` ignores the address. Add `SetFor(addr string, ident Identity)` / a map keyed by `host` of `RemoteAddr`, so two guests (and the ICE-pair remote check, which calls WhoIs on a different address of the same node) can coexist in one test.
4. Other-account, tagged and shared-in identities exist (`Other` user 7, `Tagged`, `SharedIn` Sharer 9) but each must now return the profile and node IDs. Add a second "other" user (a colleague), a "same node, other address family" case, and a "tailnet peer with nil profile" case for the fail-closed test.
5. A tagged or shared-in **self** (`StatusWithoutPeers` with `Self.Tags` or `Self.AltSharerUserID`): today `Self` is fixed; add a `SelfTagged` / `SelfSharedIn` identity, since `tailnetPeer` copies those refusals.
6. `ParseIdentity` `names` map (`:37-45`) and the Playwright fixture consume names: new constants must be added there or the fixture cannot select them.

Keep the package rule from its header: imports only `tailscale.com` types, never `internal/webserver`.

---

### `internal/capability/desktopguesttoken.go` (utility, transform)

**Analog:** `internal/capability/desktoptoken.go` (whole file, 105 lines), itself modelled on `porttoken.go`.

**Copy exactly:** `desktopKeyInfo` const -> new info string `portty/desktop-guest-token/v1` (`desktoptoken.go:23`); `DesktopKey` HKDF (`:48-53`, `hkdf.Key(sha256.New, sessionKey, nil, info, 32)`); `SignDesktop` (`:57-69`); `VerifyDesktop` body (`:75-105`) including the `strings.SplitN(token, ".", 2)` parse and `hmac.Equal`.

**Claims, fixed field order** (mirror `desktoptoken.go:34-40`):
```go
type DesktopGuestClaims struct {
	Kind    string `json:"kind"`     // "desktop-guest"
	ShareID string `json:"share_id"` // 128-bit random, hex
	IAT     int64  `json:"iat"`
	V       int    `json:"v"`        // 1
}
```
No `exp`, no `mode`. Errors: reuse `ErrMalformedToken`, `ErrInvalidSignature`, `ErrMalformedClaims`; add `ErrNotDesktopGuestToken` beside `ErrNotDesktopToken` (`:27`). Test file mirrors `desktoptoken_test.go` and extends the cross-key matrix: a guest token must fail `Verify`, `VerifyPort`, `VerifyDesktop`; an always token, session cap and port token must fail `VerifyDesktopGuest`.

---

### Guest routes (route, request-response and WebSocket)

**Analog:** `internal/webserver/desktop_routes.go`, `setupDesktopRoutes` (`:105-170`) and `handleDesktopWS` (`:172-205`).

**Gate order to copy** (CONTEXT: page = `cspHeaders` -> `requireTailnetPeerPage` -> token page check; WS = `requireAllowedOrigin` -> `requireTailnetPeer` -> token), from `desktop_routes.go:109,152-153`:
```go
mux.HandleFunc("GET /desktop/", ws.cspHeaders(ws.requireSameUserPage(func(w http.ResponseWriter, r *http.Request) { ... })))
mux.HandleFunc("GET /desktop/ws",
	ws.requireAllowedOrigin(ws.requireSameUser(ws.handleDesktopWS)))
```
New routes (register them in a `setupDesktopGuestRoutes` called right after `ws.setupDesktopRoutes(mux)` at `server.go:744`):
- `GET /desktop/guest/` -> `cspHeaders(requireTailnetPeerPage(...))`; entry document requires the guest token, hashed assets do not (copy the `rel`/`fs.Stat`/`strings.HasPrefix(rel, "assets/")` caching branch at `:122-138`, with `http.StripPrefix("/desktop/guest/", ...)`). Research Finding 5: the assets must sit behind `requireTailnetPeerPage`, because `/app/` and the `/desktop/` bundle are same-user only.
- `GET /desktop/guest/ws` -> `requireAllowedOrigin(requireTailnetPeer(handleDesktopGuestWS))`.
- ServeMux gotcha, already solved at `:141-150`: `GET /desktop/guest` (no slash) needs the explicit 301 with the query kept; `/desktop/` is a subtree pattern, and `/desktop/guest/` is more specific so it wins, but add the pinning test.

**Token outcome helper to mirror** (`desktop_routes.go:43-64`): `guestTokenOutcome(r)` -> `VerifyDesktopGuest(token, key)`, then ask the backend whether `claims.ShareID` is the active share; stopped/old share -> invalid. Reuse `writeDesktopRefusal`'s reason-header idea (`:70-100`, `X-Portty-Desktop`; 401 token, 410 off, never 403 so only the user refusal carries the marker) but with `desktop-guest-link-invalid.html`.

**WS handler skeleton** (`desktop_routes.go:172-205`): token check before `websocket.Accept`, `Accept` with `OriginPatterns: ws.allowedOrigins()`, a re-check closure (`admit func() bool`) passed to the backend so a key regenerated mid-handshake cuts the viewer (CR-01 comment at `:14-19`). For guests the backend call also carries the `peerIdentity` from the gate. Extend `DesktopBackend` additively (or add a sibling `DesktopGuestBackend` interface; the existing `Full() bool` type assertion at `:178` is the precedent for optional interface methods) rather than changing `ServeDesktopViewer`'s signature, so Phase 19/20 tests keep compiling.

**Test fake to copy:** `desktop_routes_test.go:20-47` `fakeDesktopBackend` + `newDesktopEnv`.

---

### Route-table pin test and cross-token tests (test, request-response)

**Analogs:** `internal/webserver/crosstoken_test.go` `TestCrossToken_Matrix` (tokens map, routes x tokens, matching cells asserted so a dead route does not pass as "refused"); `internal/webserver/whois_gate_test.go` `gateRefusals` table and `checkGateAnswer` (`:19-47`).

**Core pattern to copy** (`whois_gate_test.go:23-47`):
```go
var gateRefusals = []gateCase{
	{"other", webservertest.Other, http.StatusForbidden, true},
	{"tagged", webservertest.Tagged, http.StatusForbidden, true},
	{"shared-in", webservertest.SharedIn, http.StatusForbidden, true},
	{"not-found", webservertest.NotFound, http.StatusForbidden, true},
	{"status-fault", webservertest.StatusFault, http.StatusServiceUnavailable, false},
	{"whois-fault", webservertest.WhoIsFault, http.StatusServiceUnavailable, false},
}
```
Phase 22 additions: (1) a table of every registered pattern with its expected gate (enumerate with `mux` introspection or an explicit list that fails when `go vet`-style drift occurs: the test should walk the registered patterns, so a new route cannot appear ungated); assert that exactly `GET /desktop/guest/`, `GET /desktop/guest/ws` and the guest assets use `requireTailnetPeer`; (2) the Other identity gets 403 on `/app/`, `/sessions/*`, `/api/*`, port routes and `/desktop/`, and **200/101 only on the guest routes**; (3) matrix columns add `guest` token: refused on `/desktop/`, `/sessions/*`, `/api/sessions*`, port routes; `always`, `session`, `port` tokens refused on `/desktop/guest/*`; stopped share and pre-regenerate token refused.

---

### `internal/desktop/share.go`, `admission.go` (service, in-memory CRUD / event-driven)

**Analogs:** `internal/daemon/share.go` `mintShareRecord` (`:332-350`) for the 128-bit ID and signing; `unshareLocked` (`:357-371`) for teardown order; `webserver/server.go` `AddGrant`/`ClearGrants`/`revokeSignal`/`isGrantActive` (`:278-321`) for the TOCTOU-safe revoke idiom.

**128-bit random ID** (`daemon/share.go:337-342`):
```go
var gid [16]byte
if _, err := rand.Read(gid[:]); err != nil { return shareRecord{}, err }
... GrantID: hex.EncodeToString(gid[:])
```
**Revoke idiom to copy** (`server.go:290-321`): `revokeSignal` returns the channel the next clear closes; callers re-check `isGrantActive` after taking it. Use the same shape for "admission pending -> wait on (admitted | denied | share ended | timeout)": subscribe, then re-check state, then block. Guest `ServeDesktopViewer` must re-check admission after subscribing and before writing any frame (same as `handleWSSRelay`, `server.go:901`).

Keys and limits (CONTEXT + research): admission key `(share_id, user ID, stable node ID)`; deny-list keyed by user ID for the share's life; at most 1 pending per (user, node), 2 per user, 8 per share, 5 min timeout; a reconnect with the same key rebinds grants and closes the older socket. State lives in daemon memory; nothing goes in `desktop.json` (the `Config` type at `desktop/config.go` is for always-available only; do not extend it).

---

### `internal/desktop/viewer.go` and `manager.go`: live rights, identity, hooks (model/service)

**Analog:** `viewer.go` (whole, 50 lines) and `Manager.join` (`manager.go:669-715`), `leave` (`:767`), `ViewerList` (`:965`).

Facts the plan must start from:
- `Viewer.Rights` is read by nobody and written once (`manager.go:548,594`). The ServeDesktopViewer hello always announces `RightsOwnAccount` (`:547-557`), and the `viewer` struct has no identity or mode fields.
- Mutation of `v.Viewer.Rights` after join must happen under `m.vmu` (viewers map and `ViewerList` copy are guarded by `vmu`, `manager.go:965-974`). The input path (`handleInput`, `input.go:390`) is on the input goroutine; read rights through a small `m.rightsOf(v)` that takes `vmu` or use an `atomic.Uint32` on `viewer` to avoid lock-order issues (documented order: `mu`, then `vmu`, `manager.go:253`).
- Phase 22 additions (new symbols, no analog yet): `Viewer` gains `Mode` ("own" | "guest"), `Identity` (UserID, sanitized login/display/node names, StableID, IP); `Manager.SetRights(id, Rights)` that releases held input via the existing `ReleaseViewer` when `RightsControl` drops and pushes a live `rights` signalling message (additive message type, same `send(ctx, c, v any)` helper at `manager.go:464`); `Deps` hooks `OnViewerJoin/OnViewerLeave func(Viewer)` (the `Deps.Notify` func field at `:175` is the pattern for an optional func seam, nil-safe).
- Guest viewers start at `RightsViewOnly` and send `rights` map all false in the hello; PLANNED 20-02 `pressAllowed` then drops everything with `no_right`, so the arbiter only needs to add admission, lease and physical-wins in front of or inside it (`gate()` at `input.go:410` is today's only gate).
- `acceptDataChannel` (`input.go:246-249`) requires `vw.inputCap == 1`: a guest page without control must not open the channel, but a granted guest needs it live. Decide at plan time whether the channel opens at admit (rights gate drops messages) or at grant (needs a signalling re-offer). Opening at admit with the rights gate is simpler and matches "Phase 22 changes rights at runtime without touching the protocol" (20-02 T-20-13).

**Concurrent close** is already the pattern (`CloseAll`, `manager.go:980-996`):
```go
for _, c := range conns {
	wg.Add(1)
	go func() { defer wg.Done(); _ = c.Close(code, reason) }()
}
wg.Wait()
```
Research Finding 7: `Close` can wait up to 10 s per unresponsive peer; keep it concurrent and add a `CloseNow` backstop after a ~2 s ceiling. Add `Manager.CloseViewer(id, code, reason)` (per-viewer) with the revoke order: mark input ended (`ReleaseViewer`), clear admission, `pc.Close()`, then socket close.

---

### `internal/desktop/arbiter.go` (service, per-message decision)

**Analog (partial):** `Manager.gate` (`input.go:410-426`) for the "count the drop reason, return false" shape, and PLANNED 20-02 `pressAllowed` (not on disk: `gates.go` does not exist) for the ordered checks:
```go
func (m *Manager) gate(msg inputMsg) (inputTarget, bool) {
	t := m.target()
	switch {
	case !t.ok:
		m.countInput("no_target")
	case msg.E != t.epoch:
		m.countInput("stale_epoch")
	case m.Locked():
		m.countInput("locked")
	default:
		return t, true
	}
	return t, false
}
```
Use `m.countInput(name)` (`input.go:226`) with new reasons `not_admitted`, `no_clipboard_right`, `lease_held`, `physical`. Releases bypass every gate (20-02 contract) so physical-wins, lease change and revoke call `releaseHeld`/`endViewerInput` first (`input.go:502-558`). Injected clock: `Deps.Now` (`manager.go:168`) is the existing seam; `PhysicalSource` is a new `Deps` func field (`func() time.Duration`), defaulting to the tap, replaced by a fake in tests (the `Deps` comment at `:180-182` states this rule). Constants `physicalHold = 1s`, `leaseHold = 2s` per RESEARCH Pattern 5 (the code sketch there is the design; it is not on disk). Tests: copy the loopback harness in `internal/desktop/input_test.go` / `fakes_test.go` and `loopback_test.go` (fake `InputBridge`, synthetic source).

---

### `internal/desktop/audit.go` (service, file-I/O append)

**Analog:** `internal/daemon/daemon_log.go` `daemonLog` (`:67-112`) for mutex-guarded size-capped rotate-to-`.1`; open flags at `:21-23`.

**Copy:**
```go
os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
...
func (d *daemonLog) Write(p []byte) (int, error) {
	d.mu.Lock(); defer d.mu.Unlock()
	if d.size+int64(len(p)) > d.max { d.rotate() }
	n, err := d.out.Write(p); d.size += int64(n); return n, err
}
func (d *daemonLog) rotate() {
	d.size = 0
	if err := os.Rename(d.path, d.path+".1"); err != nil { ... return }
	f, err := os.OpenFile(d.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	...
```
**Drop** the `syscall.Dup2` loop and the `adoptNullStdio` machinery; the audit file is not stdio. `daemonLogMax = 10 << 20` (`:14`), so the audit cap (5 MiB per CONTEXT) is a new constant. Initial size: `Stat` the file on open (daemonLog starts at 0 and rotates on startup; for an append-only audit file do not rotate at startup, seed `size` from `Stat`). One `Write` per line (single marshalled line plus newline). Directory: `daemonConfigDir()` in `internal/daemon/engine.go:61-69` (the audit appender takes the dir as a parameter, as `Deps.ConfigDir` does at `manager.go:166`). Write failures: log to the standard logger once per minute and never block. Sanitizer: a pure function (drop `unicode.Cc`/`Cf`, collapse whitespace, cap 128 runes) in the same package, used by the Manager when it builds `Viewer.Identity`, so the GUI, notifications, signalling and log all get sanitized text.

---

### `internal/screencap/inputtap_darwin.m` (native utility, event-driven): PARTIAL

No existing event tap in the repo. Use the verified shape in `22-RESEARCH.md` Pattern 1 (listen-only `kCGSessionEventTap`, atomic `g_last_untagged`, check `CGEventTapIsEnabled`, re-enable on timeout, mask includes drag types). For the tag constant and the cgo build conventions, read the Phase 20-01 bridge in `internal/screencap` (`EventMarker`; the file that defines it is the analog for constants shared between posting and listening). Test seam: the Go wrapper must return `0/unavailable` under tests the same way "every real posting call refuses under tests" (`Deps.Input` comment, `manager.go:171-172`). Create the tap only while a controller exists (research Pattern 1); poll about once a second.

---

### `internal/daemon/desktop_share_api.go` (controller, request-response + streaming)

**Analog:** `internal/daemon/desktop_api.go` `registerDesktopRoutes` (`:125-131`), the `desktopAdapter` (`:255-291`), `handlePutDesktop` (`:192`), lifecycle fields guarded by `a.mu` (`api.go:46-67`).

**Route registration pattern** (`desktop_api.go:125-131`):
```go
func (a *API) registerDesktopRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /desktop", a.handleGetDesktop)
	mux.HandleFunc("PUT /desktop", a.handlePutDesktop)
	mux.HandleFunc("GET /desktop/link", a.handleGetDesktopLink)
```
Add `POST /desktop/share`, `DELETE /desktop/share`, `GET /desktop/share/viewers`, `POST /desktop/share/admit|deny`, `PUT /desktop/share/viewers/{id}/rights`, `DELETE /desktop/share/viewers/{id}`, `GET /desktop/share/events` (NDJSON, close = GUI liveness). These are on the daemon unix socket (all routes in `api.go:215-253` are request/response; there is **no streaming endpoint to copy**: the events stream is new, plan it from RESEARCH Pattern 8).

**Server lifetime:** `serverNeededLocked` (`desktop_api.go:117-119`) is `len(a.shares) > 0 || a.desktopCfg.AlwaysAvailable`; a click-to-share must also keep the share server up. Add `|| a.guestShare != nil`, because `stopServerIfIdleLocked` (`share.go:373`) consults it.

**Share end paths to wire:** Stop (explicit), GUI stream closed, `remintSharesLocked` / Regenerate Signing Key (`share.go:393`; ends the guest share), daemon exit (`stopDesktop`, `desktop_api.go:557`). Follow `unshareLocked`'s comment-documented order (`share.go:352-356`): clear grant, kick viewers, then stop the server.

**Response/error style:** `writeCode(w, status, code)` (`desktop_api.go:713`) and `ShareError` with `shareStatus(code)` (`share.go:63`). Reuse the CT acknowledgement: `recordCTDisclosure` (`share.go:221`) is what `desktopOn` uses for `FirstShare`.

**Test seam analog:** `SetDesktopSeamsForTest` (`desktop_api.go:73`) and `desktop_api_test.go`.

---

### `internal/daemon/client.go`, `app_desktop_share.go`, `app.go` (client + Wails bindings)

**Analog:** `DaemonClient.DesktopLink` / `DesktopPermissions` (`client.go:542,555`) and `app_desktop.go` (`GetDesktopPermissions`, `:38-48`):
```go
func (a *App) GetDesktopPermissions() DesktopPermissionsResult {
	if a.client == nil {
		return DesktopPermissionsResult{ErrorCode: "unavailable"}
	}
	sr, ax, err := a.client.DesktopPermissions()
	if err != nil {
		return DesktopPermissionsResult{ErrorCode: "unavailable"}
	}
	return DesktopPermissionsResult{SR: sr, AX: ax}
}
```
Bindings never throw, return an `ErrorCode` string, detail comes only from the local binding. Events to React: `a.emitEvent(name, data)` (`app.go:1060-1072`; tests inject `emitEventFunc`, see `app_exit_test.go:20,87`). Notifications: `a.sendNotificationFunc(id, title, body)` (`app.go:98-101`, example at `:1873` `"portty.desktop-approval-lapse"`). Click routing: `handleNotificationClick` currently returns for every ID but `updateNotificationID` (`app_update.go:258-260`); add the three `portty.desktop-admit|join|leave` IDs there. Quit warning: `QuitConfirmModal.tsx` and the quit path near `app.go:996` (planner to verify per UI-SPEC).

---

### `tray.go`, `tray_objc_darwin.m` (native UI, event-driven)

**Analog:** the Phase 19 desktop line and the update item.
- ObjC globals (`tray_objc_darwin.m:25-28`): add `NSString *trayShareTitle`, `BOOL trayShareActive`, and a `shareClicked:` / `stopShareClicked:` selector pair.
- `menuWillOpen` (`:35-75+`) builds the menu on every open; insert after the `openItem` block (`:40-46`) using the `updateItem` block as the template (`:48-56`, MRC: `[item release]` after `addItem`). Do **not** copy the desktop line's inert style (`action:NULL`, `:68-75`): the UI-SPEC makes "Desktop shared with N" an enabled item.
- Setter pattern (`:281-300`): `setTrayUpdateItem(const char *title)` / `setTrayDesktopTitle`; add `setTrayShareState(int active, int count)`; declare it in the cgo preamble (`tray.go:18-19`) and wrap it like `setTrayDesktopLine` (`tray.go:134-138`: `C.CString` + `C.free(unsafe.Pointer(...))`).
- Icon variant: `trayIconBytesForState` (`tray.go:147-166`) is a precedence switch; insert "sharing" after the `!connected` error return and before the quartile switch, and add `trayIconSharingBytes`/`2x` next to `trayIconErrorBytes` (embedded PNG vars; find with `grep -n "trayIconErrorBytes" *.go`). Two new PNGs (18 and 36 px).
- `BuildMenuItems` (`tray_common.go`) has no non-test caller (research Finding 9): update only to match or leave alone knowingly. Test seam analog: `app_set_tray_progress_test.go`.

---

### Frontend sheets (components)

**`ShareDesktopSheet.tsx`**: analog `frontend/src/components/DesktopSetupSheet.tsx` (sheet shell `.dialog.dialog--setup`, `useDialogFocus`, content mounted only while open) and `SharePopover.tsx` (Switch, Copy/Copied!, CT line, error copy keyed by `errorCode`). Copy strings live in one lib file (below). Do not read the link into a field or a QR (UI-SPEC).

**`AdmitSheet.tsx`**: analog `QuitConfirmModal.tsx` (modal with two choices, topmost via DOM order) plus `useDialogFocus` with `data-dialog-cancel` on Deny. New behaviours with no analog: `initialFocus: 'dialog'` (add to `useDialogFocus.ts`; the option type is `DialogInitialFocus` at `:8`, default `'first-input'` at `:40`, focus applied at `:49` via `initialTarget(dialog, initialFocus)`), no `data-dialog-primary` on either button, Esc is a no-op (`onCancel` empty; Esc handling at `:59` calls `onCancelRef.current()`), 600 ms click guard with `aria-disabled`. Render names in `<bdi>`.

**`ViewerList.tsx`**: analog `SettingsSheet.tsx` `Row` (private; UI-SPEC says move to its own file, not copy) and the `Switch` component (`saving`, `blocked`, `describedBy`).

**`ShareBanner.tsx`**: analog `TailscaleBanner.tsx` (full file, 27 lines): `<div className="banner banner--warning" role="status">`, icon with `banner__icon`, `banner__message`, optional `banner__action`; the Secure Event Input variant has no dismiss (like TailscaleBanner), the Share stopped variant adds `banner__dismiss`. Mounted by `App` beside `TailscaleBanner`.

**`lib/shareDesktopCopy.ts`**: analog `lib/shareCopy.ts` (exported const strings; `clean(detail)` helper strips `Error:` and trailing periods; error copy chosen by code, never parsed from text, `:19-31`). Copy every UI-SPEC string verbatim here and nowhere else.

**Tests:** Vitest per component (`pnpm --dir frontend vitest run <file>`); analogs are the existing `*.test.tsx` beside `SharePopover` and `DesktopSetupSheet` (find with `ls frontend/src/components | grep test`).

**Guest mode in `DesktopView.tsx` / `lib/desktopClient.ts`**: Phase 19 built; Phase 20 (20-06) and Phase 21 plans also edit these files. Guest mode must be a prop, per UI-SPEC, and states selected by a fixed close-reason code (`waiting`, `denied`, `admit_timeout`, `revoked`, `share_ended`, `token_invalid`). Existing close codes to extend additively: `desktop.CloseTokenRejected` 4401, `CloseDesktopOff` 4410, `CloseFull` 4429 (`manager.go:49-52`); allocate 44xx codes for guest reasons, or carry the reason in the close reason string. Merge-risk: read the latest `git log -- frontend/src/components/DesktopView.tsx` at planning time.

---

### Static pages (`web/desktop-guest-link-invalid.html`, `web/desktop-guest-refused.html`)

**Analog:** `web/user-refused.html` and `web/desktop-link-invalid.html` (UI-SPEC: copy `web/link-invalid.html` structure exactly: `<html lang="en">`, viewport meta, `<title>Portty</title>`, `dashboard.css` link, `<div class="container">`, `<h1>Portty</h1>`, `<h2 class="error">`, one `<p>`; no script, nothing rendered from the request). Load with `mustWebFile` (`whois.go:159`); tests analog: `brand_pages_test.go` (add both files and `tests/check-brand.sh` compliance: no home paths or real tailnet addresses).

---

### `TESTING.md` (docs)

**Standing rule** (project `CLAUDE.md`): add new test files to the Section 2 suite manifest, a Section 4 traceability row per new test file covering DSHR-03..07/DCTL-04 (repo-relative `.go`/`.ts`/`.tsx`/`.sh` path only), manual rows in Section 5 (M-NN: two-account tailnet SC1; Appendix B 2-minute physical-input check on a real Mac; VM rows for physical-input detection and banner suppression while captured; secure-input keyboard blind spot), then run `bash tests/check-traceability-paths.sh`. No `build.yml` matrix change expected.

## Shared Patterns

### Fail closed, in the documented order
**Source:** `internal/webserver/whois.go:88-111`; `desktop_routes.go:152-153`
**Apply to:** every guest route, the admission wait, the daemon share endpoints
Order is Origin, then WhoIs, then token, then (new) admission; any error refuses (403 with marker for another account or no peer, 503 without marker for a local tailscaled fault). Refusals are fixed static pages or fixed codes; no name from the request is ever rendered into a served page.

### Key separation by HKDF, never by claim
**Source:** `internal/capability/desktoptoken.go:42-53`
**Apply to:** the guest token only (own info string); never reuse `DesktopKey`.

### Grants clear before the kick; re-check after subscribe
**Source:** `internal/daemon/share.go:352-371`, `webserver/server.go:278-321`
**Apply to:** Stop, per-viewer revoke, deny, key regeneration, GUI-gone
```go
ws.ClearGrants(id)            // 1. grants first
hub.RevokeWebViewers(reason)  // 2. kick (hijacked sockets survive Stop, so close them explicitly)
a.stopServerIfIdleLocked()    // 3. then the server
```

### Releases bypass gates; every transition that drops a viewer releases first
**Source:** `internal/desktop/input.go:502-558` (`releaseHeld`, `ReleaseViewer`, `endViewerInput`, `releaseAllViewers`); 20-02 plan "releases bypass gates" (PLANNED)
**Apply to:** physical takeover, lease change, grant revoke, revoke, Stop, lock.

### Optional func seams with `...ForTest` setters; tests never reach live tailscaled
**Source:** `webserver/whois.go:30-41`, `desktop.Deps` (`manager.go:161-190`), `daemon.SetDesktopSeamsForTest` (`desktop_api.go:73`)
**Apply to:** `PhysicalSource`, tap creation, audit file path/clock, GUI-liveness, notification sender.

### Counters with no sensitive content
**Source:** `Manager.countInput` / `InputCounters` (`input.go:226-245`)
**Apply to:** all arbiter drops; never key codes, coordinates, clipboard, tokens or SDP in counters, logs or the audit file.

### Local-binding-only error detail in the UI
**Source:** `app_desktop.go:38-48`, `lib/shareCopy.ts:19-31`
**Apply to:** every Wails binding and every sheet error string.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/screencap/inputtap_darwin.m` | native utility | event-driven | No `CGEventTap` exists; use RESEARCH Pattern 1 (VM-proven) and the Phase 20-01 bridge for cgo/marker conventions |
| `GET /desktop/share/events` (GUI liveness stream) | controller | streaming | Every daemon API route is request/response; design from RESEARCH Pattern 8 (NDJSON, stream close is the signal, one GUI holder) |

## Metadata

**Analog search scope:** `internal/webserver`, `internal/webserver/webservertest`, `internal/capability`, `internal/desktop`, `internal/daemon`, repo-root `*.go` and `tray_objc_darwin.m`, `frontend/src/components`, `frontend/src/lib`, `web/`, `.planning/phases/20-remote-control/20-*-PLAN.md`, `.planning/phases/21-portty-app-and-iphone-viewers/21-*-PLAN.md`
**Files scanned:** about 45 (read in full or by targeted ranges)
**Pattern extraction date:** 2026-11-26
**Plan-time re-verification list:** (1) whether 20-02..20-11 have landed (`ls internal/desktop/gates.go`, `grep -n RightsControl internal/desktop/input.go`); (2) Phase 21 edits to `DesktopView.tsx` and `desktopClient.ts`; (3) the `web` embed pattern includes new `*.html`; (4) tracked-ness of any newly named analog (`git ls-files -- <path>`).
