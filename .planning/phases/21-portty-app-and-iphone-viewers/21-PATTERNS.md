# Phase 21: Portty App and iPhone Viewers - Pattern Map

**Mapped:** 2026-11-26
**Files analyzed:** 27 new/modified (plus tests)
**Analogs found:** 25 / 27 (all analog paths verified in the working tree; wailsjs and rig files confirmed git-tracked)

Phase 19 is BUILT (real code, cited with lines). Phase 20 is PLANNED, NOT BUILT (see the separate section; never treat those symbols as existing).

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `internal/webserver/server.go` (handleSessionsMeta: add `X-Portty-Desktop` header; route `POST /api/peer/desktop/cap`) | controller | request-response | same file, `handleSessionsMeta` L804-825, route L569-574 | exact |
| `internal/webserver/whois.go` (new `handlePeerDesktopCap`) | controller | request-response | `handlePeerCap` L167-190 | exact |
| `internal/tailnet/sessions.go` (`PeerSessionMetaGroup.Desktop`, header parse, `FetchPeerDesktopCap`) | service | request-response | `doFetchSessionsMeta` L74-103, `FetchPeerCap` | exact |
| `internal/daemon/desktop_caps.go` (new: in-memory desktop cap store keyed by host) | store | CRUD (memory) | `remote_caps.go` (RemoteCapStore) + `remote_caps_test.go` source-grep | exact |
| `internal/daemon/peer_desktop.go` (new: `POST /peers/desktop`) | controller | request-response | `peer_connect.go` `handlePeerConnect` L133+ | exact |
| `internal/daemon/remote_desktop_ws_proxy.go` (new: `GET /api/relay/remote-desktop/{host}/ws`) | controller/proxy | streaming (WS) | `remote_ws_proxy.go` `handleRemoteSessionWS` L47+, `copyWS` L209 | exact |
| `internal/daemon/relay_remote_files.go` (register route) | config | routing | same file L29 | exact |
| daemon client method `ConnectPeerDesktop` (internal/daemon client) | service | request-response | `DaemonClient.ConnectPeer` (used app.go L1748) | exact |
| `app.go` (`ConnectPeerDesktop`, `SetControlMode` bindings; `RemotePeerSessions.Desktop`) | controller/binding | request-response | `ConnectPeerSession` L1744-1757, `RemotePeerSessions` L58-63 | exact |
| `frontend/src/wailsjs/go/main/App.{js,d.ts}`, `models.ts` (regenerate, committed) | generated | n/a | tracked at `frontend/src/wailsjs/` | exact |
| `main.go` (control-mode menu fallback only if probe fails) | config | event-driven | `appMenu` L183-214 | role |
| `build/darwin/Info.plist` (`NSLocalNetworkUsageDescription`) | config | n/a | NSAppTransportSecurity block L66-70 | role |
| `frontend/src/components/DesktopView.tsx` (accept transport/check/beforeRedial/peer props) | component | streaming | itself, L105-117, L168-195 | exact |
| `frontend/src/lib/desktopClient.ts` (consume options; add `wsUrl` helper only if needed) | utility | streaming | itself L52-60, L100-127, L507-528 | exact |
| `frontend/src/components/DesktopTab.tsx` (new wrapper: loopback transport, control mode, Local Network state) | component | streaming | `TerminalPanel` mounting + `DesktopView` | role |
| `frontend/src/lib/controlMode.ts` (new: control-mode state machine, release chord, hidden-window 2 min timer) | utility | event-driven | `lib/hostReconnect.ts` (+ `hostReconnect.test.ts`) | role |
| `frontend/src/lib/desktopTouch.ts` (new: trackpad math, gestures) | utility | event-driven | `lib/pinchZoom.ts` L21-60, `lib/touchScrollHandler.ts` L46 | role |
| `frontend/src/components/DesktopPhoneBar.tsx` (new: keyboard button, key bar) | component | event-driven | `components/KeyBar.tsx` L22 | role |
| `frontend/src/components/Sidebar.tsx` (Desktop row) | component | request-response | itself (peer groups, PeerDot) | exact |
| `frontend/src/lib/sidebarPeers.ts` (carry `desktop`) | utility | transform | itself L5-60 | exact |
| `frontend/src/lib/remoteSession.ts` (`RemotePeerSessions.desktop`) | model | transform | itself | exact |
| `frontend/src/components/TabBar.tsx` (`type: 'desktop'`) | component | request-response | `Tab` L19-33, `isLocal` L109 | exact |
| `frontend/src/lib/tabs.ts` (`peerTabsRemoved`, `tabLabel`, `desktopTabId`) | utility | transform | `webSessionTabId`, `peerTabsRemoved` | exact |
| `frontend/src/App.tsx` (open/focus desktop tab) | component | request-response | `handleOpenRemoteSession` L986, `onOpenPeer` L1165 | exact |
| `frontend/src/lib/webMode.ts` (read-only; phone/standalone detection) | utility | n/a | `detectMode` L51-57; `phoneQuery.ts` | reuse |
| `TESTING.md` | docs | n/a | Sections 2, 4, 5 (Category pattern) | exact |
| `.planning/phases/21-.../21-VM-CHECKS.md` + rig subcommands | docs/rig | n/a | `19-VM-CHECKS.md`, `rig/p19-rig.sh` | exact |
| Go test files for proxy/header/cap route/store | test | n/a | `remote_ws_proxy_test.go`, `remote_caps_test.go`, `peer_connect_test.go`, `desktop_routes_test.go` | exact |
| Native keyboard monitor (ObjC) | native | event-driven | none | none |

## Pattern Assignments

### `internal/webserver/whois.go` new `handlePeerDesktopCap` (controller, request-response)

**Analog:** `handlePeerCap` (whois.go L167-190), route at server.go L569-574 (`requireSameUser`). Copy exactly: JSON content-type gate, 8 KiB body cap, nil-backend -> 404, no-store, `{"cap": token}`.

```go
mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
if err != nil || mt != "application/json" {
	http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
	return
}
_, _ = io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, 8<<10))
if ws.shareToken == nil { http.NotFound(w, r); return }
...
w.Header().Set("Content-Type", "application/json")
w.Header().Set("Cache-Control", "no-store")
json.NewEncoder(w).Encode(map[string]string{"cap": token}) //nolint:errcheck
```
Differences: token comes from the Phase 19 `DesktopBackend` (`ws.desktop`, server.go L134-136, `SetDesktopBackend` desktop_routes.go L28); 404 when always-available is off. Register beside L574: `mux.HandleFunc("POST /api/peer/desktop/cap", ws.requireSameUser(ws.handlePeerDesktopCap))`.

**Header on meta** (`handleSessionsMeta` server.go L804-825): set `w.Header().Set("X-Portty-Desktop", "always")` before `Content-Type`/encode, only while always-available is on; the body stays `json.NewEncoder(w).Encode(items)` (bare array).

**Refusal marker pattern:** `desktop_routes.go` `writeDesktopRefusal` L82 and `desktopTokenOutcome` L43; `/desktop/ws` L154 and `/desktop/check` L160 (`requireSameUser`) are what the proxy dials. Test that a desktop token is refused on `/sessions/*` (crosstoken_test.go is the analog).

### `internal/tailnet/sessions.go` (service, request-response)

**Analog:** `PeerSessionMetaGroup` L38-42 (add `Desktop string` json tag), `doFetchSessionsMeta(ctx, url, client, host)` L74-103 returns `([]ShareableSessionMeta, MetaStatus, error)`. The header is on `resp` inside this function; extending the return signature touches `FetchPeerSessionsMeta` L112 and `FetchPeerSessionsMetaWithClient` L162 and their callers. Readers ignore unknown values (only `"always"` sets true). `MetaForbidden` (403 + `X-Portty: 1`) hides other-account peers; keep that branch first.

### `internal/daemon/desktop_caps.go` (store)

**Analog:** `remote_caps.go` (whole file, ~80+ lines read). Copy: header comment with the no-disk invariant, `sync.RWMutex`, `New...Store()`, `Put` rejecting empty inputs and non-https baseURL (L66-90), `Get`/`Delete`. Key by peer host instead of session id. Extend the source-grep no-disk guard in `remote_caps_test.go` to cover the new file. Kill on always-available off / Regenerate Signing Key: add a Delete-all path wired where `RemoteCapStore` entries are purged.

### `internal/daemon/peer_desktop.go` `POST /peers/desktop` (controller)

**Analog:** `handlePeerConnect` (peer_connect.go L133-200+). Copy the skeleton: 8192-byte `MaxBytesReader`, decode `{host}`, `context.WithTimeout(20s)`, `a.discoverPeers(ctx)` -> 503 `PeerErrTailscaleOff`, `matchPeer` -> 404 `PeerErrUnknownHost`, `a.peerBaseURL(peer)`, `peerIP(peer)`, per-request `a.peerClient(strings.TrimSuffix(peer.DNSName, "."), ip)` with `defer client.CloseIdleConnections()`, error switch (Forbidden 403, Unreachable 502, never log the token: `redactCapTokenFromError`). Replace the meta/session resolve with a POST to `/api/peer/desktop/cap`; map 404 to a new "desktop off" code, 403+marker to `PeerErrForbidden`. Seams reuse `SetPeerSeamsForTest` (L33). Route registration follows wherever `/peers/connect` is registered (grep `handlePeerConnect`).

### `internal/daemon/remote_desktop_ws_proxy.go` (proxy, streaming)

**Analog:** `handleRemoteSessionWS` (remote_ws_proxy.go L47+). Header comment L1-27 states the two gotchas to copy: (1) inject `Origin: <baseURL>` byte-exact on the upstream dial, (2) run the copy loop on the long-lived request ctx, not the 10 s dial ctx; ping upstream every `relay.PingEvery`; abnormal close with no close frame. Cap lookup happens BEFORE `websocket.Accept` so a no-cap path returns the JSON 404 marker. Inbound Origin: `relay.LoopbackOriginPatterns`. Differences: path value is `{host}`; upstream URL `wss://<peer>:7444/desktop/ws?cap=<held token>`; signalling only. Use `copyWS(ctx, dst, src, side, errc)` (L209) unchanged. Distinct close/HTTP outcomes must map onto Phase 19's distinct proxy errors so `check()` can classify.

**Route registration** (relay_remote_files.go L21-33):
```go
mux.HandleFunc("GET /api/relay/remote/{sessionID}/ws", a.handleRemoteSessionWS)
// add: mux.HandleFunc("GET /api/relay/remote-desktop/{host}/ws", a.handleRemoteDesktopWS)
mux.Handle("/", relayServer)
```
No CORS wrapper, no OPTIONS.

### `app.go` bindings

**Analog:** `ConnectPeerSession` L1744-1757 and `PeerConnectResult` L1729-1738 ("never throws; ErrorCode"). `ConnectPeerDesktop(host string) PeerDesktopResult{Host, URL, ErrorCode}`: URL is the loopback ws URL with NO token. `client == nil` -> `PeerErrUnreachable`; `errors.As(err, &pe)` with `*daemon.PeerConnectError`. Add `Desktop bool` to `RemotePeerSessions` L58-63 (`json:"desktop"`). `SetControlMode(bool)` only if the probe in 21-RESEARCH shows the in-handler key block is insufficient. After editing bindings, regenerate `frontend/src/wailsjs/go/main/App.js`, `App.d.ts`, `models.ts` (tracked; commit them).

### `main.go` (only if fallback needed)

`appMenu` L183-214: `appSub.AddText("Quit Portty", keys.CmdOrCtrl("q"), ...)`, `fileMenu.AddText("Close Tab", keys.CmdOrCtrl("w"), emitMenuEvent("menu:close-tab"))`, `m.Append(menu.EditMenu())`, `viewMenu.AddText("Toggle Sidebar", keys.Combo("s", keys.CmdOrCtrlKey, keys.ControlKey), ...)`, Settings L190. First-wave probe decides; do not edit unless the probe fails.

### `build/darwin/Info.plist`

Add next to (L66-70):
```xml
<key>NSAppTransportSecurity</key>
<dict><key>NSAllowsLocalNetworking</key><true/></dict>
```
a sibling `<key>NSLocalNetworkUsageDescription</key><string>...</string>`.

### `frontend/src/components/DesktopView.tsx` (Phase 19, BUILT)

Real surface (282 lines):
- `export interface VideoRect`, `videoContentRect(elementWidth, elementHeight, videoWidth, videoHeight)` L17-30.
- `export interface DesktopViewProps { onVideoRect?: (r: VideoRect) => void; onReload?: () => void }` L105-110.
- `export function DesktopView({ onVideoRect, onReload }: DesktopViewProps = {})` L117.
- Mount effect L168-195: `const client = createDesktopClient()` with NO options; subscribes `track`, `state`, `size`, `status`; `client.start()`; cleanup `off()` + `client.stop()`.
- Sets `document.title` and `apple-mobile-web-app-title` meta (L129+) and has a Tap to start path (`needsTap`, `startPlay`).
- View only: no pointer, key or context-menu handling.

**Not exposed yet (Phase 21 must add):** props for `clientOptions` (transport, check, beforeRedial, createPeerConnection), a way to get the `DesktopClient` handle (for `wake('external')`), peer name, status-to-host callbacks (`onStatus`), embedding in a tab (document.title/meta side effects are page-level and should be skipped when embedded), local-network-denied state. Copy the effect structure; keep `createDesktopClient(opts)` creation inside the effect keyed on stable option identity (use refs like `rectCb`, L119-120).

### `frontend/src/lib/desktopClient.ts` (Phase 19, BUILT) exact exports Phase 21 consumes

```ts
export type DesktopState = ...                       // L14
export type WakeReason = 'visibility' | 'pageshow' | 'online' | 'server-restart' | 'external'  // L24
export type ClientStatus = ... | {kind:'unreachable'} | {kind:'token_rejected'} | {kind:'desktop_off'} | {kind:'other_account'}  // L26-33
export interface SignalTransport { send(msg: object): void; close(code?: number): void;
  onOpen(cb: () => void): void; onMessage(cb: (msg: unknown) => void): void; onClose(cb: (code: number) => void): void }  // L35
export type TransportFactory = () => SignalTransport   // L43
export type CheckVerdict = 'ok' | 'token' | 'off' | 'other_account' | 'retry'  // L44
export interface DesktopClientOptions { transport?; check?: () => Promise<CheckVerdict>;
  beforeRedial?: () => Promise<void | 'stop'>;   // awaited before every redial, never before first connect
  createPeerConnection?: () => RTCPeerConnection; clock?; doc?; win? }  // L52-61
export interface DesktopClient { start(); stop(); wake(reason: WakeReason);
  on<K extends keyof DesktopEvents>(event: K, cb): () => void }  // L82-88
export const RECONNECT = { delaysMs: [1000,2000,4000,8000,10000], unreachableAfterMs: 30000, pingTimeoutMs: 3000, ... }  // L90
export function defaultDesktopWsUrl(loc = window.location): string  // L100
export function webSocketTransport(url: string): SignalTransport    // L105
export function createDesktopClient(opts: DesktopClientOptions = {}): DesktopClient  // L134
export async function fetchCheck(loc = window.location): Promise<CheckVerdict>  // L507
```
`fetchCheck` hits same-origin `/desktop/check?cap=` (204 ok, 401 token, 410 off, 403+`X-Portty` other_account, else retry; 5 s abort). In the Wails tab (origin `wails://`, no `/desktop/check`) supply a custom `check` that asks the loopback proxy/daemon and returns the same verdicts: copy its status-to-verdict mapping and AbortController timeout. `webSocketTransport(url)` takes the loopback `ws://127.0.0.1:<relayPort>/api/relay/remote-desktop/<host>/ws`. Use `beforeRedial` to re-run `ConnectPeerDesktop` (token refresh through `POST /peers/desktop` on every connect, returning `'stop'` when tab closed or desktop off).

### `frontend/src/lib/webMode.ts` (BUILT)

`AppMode = 'desktop' | 'web' | 'desktopview'` L28; `detectMode(loc)` L51 is pathname-based (`/desktop` and `/desktop/*` -> `desktopview`); `readWebModeParams(loc)` L66 returns `{sessionId, capToken}`. The Wails tab is mode `'desktop'` (root), so `DesktopTab` is mounted by `App`, not by mode detection. Phone/standalone: `lib/phoneQuery.ts` `PHONE_QUERY = '(pointer: coarse) and (hover: none)'`, `isPhone()`, `usePhone()`, `isStandalone()` (L1-40); never the user agent.

### Tab type (`TabBar.tsx`, `lib/tabs.ts`, `App.tsx`)

`Tab` (TabBar.tsx L19-33): `type?: 'terminal' | 'web-session' | 'preview'`, `peerHost?`, `connectHost?`, `baseURL?`, `capToken?` (do NOT set `capToken` or `baseURL` for desktop). Add `'desktop'`; `isLocal` L109 (`!tab.type || tab.type === 'terminal'`) already excludes it (no rename). `tabs.ts`: `tabLabel` returns `tab.name` for non-terminal types (OK); `peerTabsRemoved` filters `t.type === 'web-session' && t.peerHost` (L~62) so extend it (or add `desktopTabsRemoved`) to disconnect removed desktop tabs; add `desktopTabId(host) => 'desktop:' + host` beside `webSessionTabId` (`__websession__${host}/${sessionId}`, with the '/' uniqueness rationale). `peerTabParts` returns null for non-web-session, so desktop tabs need their own label branch. In `App.tsx`, copy `handleOpenRemoteSession` (L986) for open-or-focus semantics; Sidebar callback is wired at L1165 (`onOpenPeer`). Use the shared nextActiveTabId on close.

### `Sidebar.tsx` / `sidebarPeers.ts` / `remoteSession.ts`

`PeerGroup {key, host, reachable, peer: RemotePeerSessions}` L5-11, `shortHost`, `peerColorIndex(hostname)` (FNV-1a, L33; pass the FULL hostname), `peerView(tailscaleOff, loaded, peers, reconnectingHost)` L39. The Desktop row is the first row in each group; reuse `PeerDot` and the group's row classes; icon `ComputerDesktopIcon` from heroicons (already a dependency). Read Sidebar.tsx's existing session row markup before writing the row (not excerpted here; read in the plan task).

### `KeyBar.tsx`, `pinchZoom.ts`, `touchScrollHandler.ts`, `visualViewportInset.ts`

- `KeyBar({ send, applicationCursor, ctrlArmed, onCtrlChange, keyboardUp })` L22: one-shot Ctrl armed state and pointerdown firing rule (copy for Esc/Tab/Ctrl/Opt/Cmd/arrows; `send` becomes Phase 20 key messages).
- `attachPinchZoom(container, handlers): () => void` (pinchZoom.ts L31) with `PinchHandlers` L21: non-passive listeners on a container, returns a disposer. Adapt for CSS transform scale 1x-5x (not font size; `MIN_FONT_PX`/`MAX_FONT_PX` L16-17 are terminal-specific, do not reuse).
- `attachTouchScroll(container, term)` L46: same attach/detach shape; two-finger drag -> wheel message.
- `useVisualViewportInset(enabled): { keyboardUp }` L17: use directly for the soft-keyboard layout.

### Tests

Analogs: Go `remote_ws_proxy_test.go` (Origin injection, ctx lifetime), `remote_caps_test.go` (no-disk source grep), `peer_connect_test.go` (`SetPeerSeamsForTest`), `desktop_routes_test.go`, `crosstoken_test.go`. Vitest: `hostReconnect.test.ts`, `relayClient.test.ts`, `remoteAdapter.test.ts` (co-located `lib/*.test.ts`, plus `lib/__tests__/`). Test setup stubs `matchMedia`.

### `TESTING.md` and checks

Sections: 2 Suite Manifest (L22), 4 traceability (L386, rows are `| REQ-ID | path | group |` with repo-relative `.go/.ts/.tsx/.sh` paths only), 5 manual checklist (L979, "Category X" headings, next M-NN). `tests/check-traceability-paths.sh` greps `\| [^|]+\.(go|ts|tsx|sh) \|`; run `bash tests/check-traceability-paths.sh` and `bash tests/check-brand.sh` before commit. Add rows for DVWR-01, DVWR-03, DVWR-04 and M-NN rows for iPhone gestures and the Local Network prompt.

### VM GUI checks

`19-VM-CHECKS.md` row format `| id | req | steps | PASS/FAIL | evidence |`, builds with `wails build -tags "wailsassets vmpreview"`, ad-hoc signed, identifier app.portty.Portty; roles portty-test = office, portty-test-2 = home (this phase: real Portty.app on portty-test-2 views portty-test). Rig: `.planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh` (subcommands `enable-desktop`, `safari-stats`, `vm2` ssh helper, `assertions`, `pcap`), plus `readterm.js`, `scroll.swift`, `tray.swift`. Phase 21 should add a `p21-rig.sh` or subcommands copying this. No host tailscaled contact, share link piped through ssh only and kept out of evidence files.

## Phase 20 planned interfaces (UNBUILT - assumption)

Nothing below exists in the tree today. Phase 21 plans must state these as dependencies and not code against them until Phase 20 merges.

| Needed by Phase 21 | Produced by | Planned artifact/name |
|---|---|---|
| Input data channel, `Manager.handleInput`, `ReleaseViewer`, wire parse | 20-01-PLAN | `internal/desktop/input.go`: `InputBridge` interface, `parseInput`, `acceptDataChannel`, `handleInput`, `ReleaseViewer`, `inputLoop` |
| Gates (rights, lock, epoch, rate, control state) | 20-02-PLAN | `internal/desktop/gates.go` `pressAllowed`; `input_map.go` `mapPoint`, `integrateRelative`, `scrollDelta`; `control.go` `ControlState` (relative move + cursor-position report live here, so iPhone trackpad math may be host-side) |
| Display/window switch | 20-03/20-04-PLAN | `surface.go`, `window.go` |
| Explicit paste/copy (`clip_get`, `clip_put`) | 20-05-PLAN, 20-08-PLAN | `clipboard.go`; frontend `copyFromMac`, `pasteToMac` in desktopClient.ts; `DesktopClipboardPopover.tsx` |
| Frontend input layer, Cmd-chord tap rule, release chord, release-all, text split | 20-06-PLAN | `frontend/src/lib/desktopInput.ts` `createDesktopInput`; modifies `desktopClient.ts` (`sendInput`, rights, surface and control events) and `DesktopView.tsx` (stage, hidden input, ring, dot, control state, notice slot) |
| Send Keys menu component, control bar | 20-07-PLAN | `DesktopSendKeysMenu.tsx`, `DesktopControlBar.tsx` (`barAvailability`) |
| Wire golden files | 20-01..20-02 | `internal/desktop/testdata/input-v1.jsonl` (confirm exact producing plan in 20-01/20-02 must_haves before citing) |

Collision warning: 20-06 rewrites `DesktopView.tsx` and `desktopClient.ts`; Phase 21 changes to the same files (options props, client handle) will conflict unless sequenced after Phase 20 or confined to additive props. Phase 20 already owns control mode ring/release chord in DesktopView; verify overlap with Phase 21's ring and `controlMode.ts` before planning (20-UI-SPEC.md, 20-PATTERNS.md).

## Shared Patterns

- **Peer endpoints:** `requireSameUser` twins; refusals carry `X-Portty: 1` (MetaForbidden depends on it). Source: server.go L569-574.
- **Cap hygiene:** token in daemon memory only, never in web view URL, logs (`redactCapTokenFromError`), or disk (source-grep test).
- **Seams:** package vars with `SetXForTest` (`SetPeerSeamsForTest` peer_connect.go L33).
- **Never-throw binding result:** struct with `ErrorCode` (app.go L1729).
- **Touch handlers:** non-passive, container-scoped, return a disposer (`attachPinchZoom` L31).
- **Phone mode:** media query only (`PHONE_QUERY`).
- **Standing rule:** TESTING.md Sections 2, 4, 5 updated in the same plan as the tests.

## No Analog Found

| File | Role | Reason |
|---|---|---|
| Native ObjC NSEvent monitor (fallback 2 of control mode) | native | No key-monitor code; only `internal/screencap/*.m` exist for capture and (planned) input |
| Local Network denied detection (ICE `new` timeout state, Open System Settings) | component | No existing TCC-denial UI; use 21-RESEARCH R13 notes and UI-SPEC |
| iPhone trackpad gesture recognizer (tap, double-tap, long-press, tap-drag, two-finger) | utility | Existing touch code handles terminal pinch/scroll only; use pinchZoom.ts shape |

## Metadata

**Analog search scope:** internal/daemon, internal/webserver, internal/tailnet, app.go, main.go, build/darwin, frontend/src (components, lib, wailsjs), tests, TESTING.md, Phase 19 and 20 planning dirs.
**Pattern extraction date:** 2026-11-26
