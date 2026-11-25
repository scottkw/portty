# Phase 19: Unattended Desktop Viewing - Pattern Map

**Mapped:** 2026-11-25
**Files analyzed:** 27 new or modified
**Analogs found:** 25 / 27 (all analog paths verified git-tracked)

Note: line numbers below come from this branch at mapping time. Phase 18 is editing concurrently, so re-grep before relying on exact lines.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `internal/capability/desktoptoken.go` | utility (token) | transform | `internal/capability/porttoken.go` | exact |
| `internal/screencap/screencap.go` (+ `!darwin` stub) | interface/config | transform | `spike/desktop/bridge_darwin.go` (decls) | role-match |
| `internal/screencap/capture_darwin.m/.go`, `encode_darwin.m/.go` | bridge (cgo ObjC) | streaming | `spike/desktop/media_darwin.m` + `bridge_darwin.go` | exact |
| `internal/screencap/power_darwin.m/.go`, `session_darwin.m/.go` | bridge | request-response | `spike/desktop/sys_darwin.m` (`spk_display_asleep`, `spk_session_dict`) | exact |
| `internal/screencap/nudge_darwin.m/.go`, `preflight_darwin.m/.go` | bridge | request-response | `spike/desktop/sys_darwin.m` (`spk_mouse_nudge` lines 62-76, `spk_sr_preflight`, `spk_ax_trusted`) | exact (nudge must return the pointer) |
| `internal/screencap/synthetic.go` | test utility | streaming | 17-01 tracer pattern, `spike/desktop/media_darwin.m` `spk_synthetic_start` | role-match |
| `internal/desktop/peers.go` | service | streaming (pion) | `spike/desktop/server.go` `newAPI` (~312-340) | exact |
| `internal/desktop/manager.go` | service | event-driven | `internal/wakewatch/wakewatch.go` + `internal/daemon/share.go` seams | partial |
| `internal/desktop/signalling.go` | controller (WS) | request-response | `spike/desktop/server.go` handlers + `internal/webserver/server.go` `/sessions/{id}/ws` | role-match |
| `internal/desktop/config.go` (desktop.json) | model/config | file-I/O | `internal/daemon/engine.go` settings save, `internal/capability/keystore.go` (0600, LoadOrGenerate) | role-match |
| `internal/desktop/permissions.go` + `cmd_daemon.go` `perm-probe` | service | request-response | `cmd_daemon.go` dispatch, `internal/daemon/process_unix.go` exec pattern | role-match |
| `internal/daemon/launchd.go` | service | file-I/O + exec | `internal/daemon/service.go` + `process_unix.go` | role-match |
| `internal/daemon/process.go`, `process_unix.go` (kickstart start path) | modify | request-response | itself (`startDetachedDaemon`, `ensureDaemon`) | exact |
| `internal/daemon/share.go`, `handoff.go` (serverNeeded) | modify | event-driven | itself | exact |
| `internal/daemon/desktop_api.go` | controller | request-response | `internal/daemon/api.go` routes | exact |
| `internal/webserver/desktop_routes.go` | route | request-response | `internal/webserver/server.go` `/app/` and `/sessions/{id}` routes | exact |
| `internal/webserver/csp_mw.go` (test pin only) | test | - | itself | exact |
| `frontend/src/components/DesktopView.tsx` | component | streaming | `frontend/src/components/WebShareSessionView` pattern + `lib/webMode.ts` | role-match |
| `frontend/src/lib/desktopClient.ts` | utility | streaming (WS+RTC) | `frontend/src/lib/relayClient.ts` + `hostReconnect.ts` | role-match |
| `frontend/src/lib/webMode.ts`, `main.tsx` | modify | - | itself | exact |
| `frontend/src/components/SettingsSheet.tsx` (Desktop group + onboarding sheet) | component | CRUD | itself, Sharing pane `Row` | exact |
| `app.go` bindings, `tray.go`/`tray_common.go` state line | modify | request-response | `GetSessionShare`, `HasCTDisclosure`; existing tray items | exact |
| lapse notification | service | event-driven | `notification_darwin.go` (`sendNotification`, `hasAppBundleID`) | exact |
| `build/darwin/Info.plist`, cgo flags | config | - | `spike/desktop/bundle/Info.plist`, `build.sh` | role-match |
| `TESTING.md`, `*_test.go` | test | - | existing suite manifest (standing rule) | exact |
| `internal/desktop/*_test.go` (bind refusal, STUN absence) | test | - | `spike/desktop/server.go` `checkBind` 59-78 | role-match |

## Pattern Assignments

### `internal/capability/desktoptoken.go` (utility, transform)

**Analog:** `internal/capability/porttoken.go` (copy nearly whole file; lines 14-19, 34-39, 66-96)

Imports (lines 3-12): `crypto/hkdf, crypto/hmac, crypto/sha256, encoding/base64, encoding/json, errors, fmt, strings`.

Key derivation and separation (lines 14-19, 49-54):
```go
const PortTokenKind = "port"
const portKeyInfo = "portty/port-token/v1"
func PortKey(sessionKey []byte) ([]byte, error) {
	if len(sessionKey) == 0 { return nil, errors.New("capability: empty session key") }
	return hkdf.Key(sha256.New, sessionKey, nil, portKeyInfo, 32)
}
```
Desktop version: `DesktopTokenKind = "desktop"`, info `portty/desktop-token/v1`, claims (field order is load-bearing, HMAC signs marshalled bytes): `Kind, Mode ("always"), GrantID json "grant_id", IAT, V`. No SID. Verify tail (lines 88-96): `hmac.Equal`, unmarshal, `if c.Kind != X || c.V != 1 { return ..., ErrWrongKind }`. Reuse `ErrMalformedToken/ErrInvalidSignature/ErrMalformedClaims`; declare a distinct `ErrNotDesktopToken` or reuse ErrWrongKind with message tweak (planner choice). Note the comment at 37-48: HKDF separation, not Kind, is what blocks cross-acceptance. Tests: cross-token matrix (session cap, port, desktop on every route family).

---

### `internal/screencap/*_darwin.{m,go}` (cgo ObjC bridges)

**Analog:** `spike/desktop/media_darwin.m` (SCK ~166-173 sizing, VT 380-382, size cap 915-920), `spike/desktop/bridge_darwin.go` (Go decls lines 18-58: `spk_sck_start`, `spk_encoder_force_keyframe`, `spk_request_refresh`, `spk_display_asleep`, `spk_sr_preflight`, `spk_ax_trusted`, `spk_post_event_preflight`, `spk_session_dict`), `spike/desktop/sys_darwin.m`.
Repo-root style for cgo ObjC: `notification_darwin.go` header:
```go
//go:build darwin
/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework UserNotifications
...
*/
import "C"
```
Phase 19 changes versus spike (do not copy blindly):
- Rename `spk_` prefix; add `-mmacosx-version-min=14.0`; frameworks: ScreenCaptureKit, VideoToolbox, CoreMedia, CoreVideo, CoreGraphics, ApplicationServices, IOKit.
- Wrap every bridge call in ObjC `@try`/`@catch`; Go side `recover()`; errors become typed codes (-3801, -3815), never exit.
- SPS/PPS on every IDR; real keyframe ceiling instead of spike `fps*3600`; 8 Mbps / 30 fps / queueDepth 5 / `showsCursor=YES` / RealTime / `AllowFrameReordering=false` / `kVTProfileLevel_H264_ConstrainedBaseline_AutoLevel` as named constants.
- `preflight_darwin.m` is the only file calling `CGPreflightScreenCaptureAccess`, `AXIsProcessTrusted`, `CGPreflightPostEventAccess`, and only from the `perm-probe` child (never prompt variants).
- Nudge: spike moves +1 and does not return, fails at right edge; production returns the pointer and picks the direction away from the edge.
- Power: `IOPMAssertionCreateWithName` for `PreventUserIdleDisplaySleep` and `PreventUserIdleSystemSleep`; `IOPMAssertionDeclareUserActivity(..., kIOPMUserActiveRemote, ...)` on join; release on every shutdown path. Add AC-power-gated system-sleep assertion (user decision).
- Provide `//go:build !darwin` stub so `go vet ./...` stays green.

---

### `internal/desktop/peers.go` (service, streaming)

**Analog:** `spike/desktop/server.go` lines 312-340 (`newAPI`):
```go
se := webrtc.SettingEngine{}
se.SetICEUDPMux(s.mux)
se.SetLite(lite)
se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
se.SetIPFilter(func(ip net.IP) bool { return ip.Equal(s.bind.AsSlice()) })
```
then `newMediaEngine()` and `interceptor.Registry`. Use server ICE-lite, `iceServers` empty, one shared track, coalesced PLI/FIR to forced IDR. Pins: pion/webrtc/v4 v4.2.29, interceptor v0.1.51, ice/v4 v4.4.9 (see `spike/desktop/go.mod`). The spike's SPS patch (server.go ~305-308, `0x42 0xc0` -> `0xe0`) is NOT needed under 42e01f.

**Bind refusal** (spike `server.go:50-78`): `cgnat = netip.MustParsePrefix("100.64.0.0/10")`, `errBindRefused`, `checkBind` refuses `IsUnspecified()` always. Production version: accept only 100.64/10 (loopback only under test hook); tests assert refusal of 0.0.0.0, ::, non-CGNAT.

---

### `internal/desktop/manager.go` (state machine, viewer registry)

**Analogs:** `internal/wakewatch/wakewatch.go` (`Watch(ctx) <-chan Reason`, wall-clock gap, no monotonic timers, no run loop) for restart-after-wake; seam style from `internal/daemon/share.go:30-45`:
```go
var (
	defaultShareHealth = webserver.CheckHealth
	defaultNewShareServer = func(h webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {...}
	defaultWarmCert = func(ctx context.Context, domain string) error {
		if !tailnet.LiveAllowed() { return tailnet.ErrLiveTailnetInTest }
```
Copy: package-var defaults copied into struct by constructor, `SetXForTest` setters, `tailnet.LiveAllowed()` guard, Runner seam for exec. State enum `off, needs_permission, awaiting_approval, display_asleep, locked, idle, streaming(n)`.

---

### `internal/desktop/config.go` (desktop.json)

**Analogs:** `internal/capability/keystore.go` (0600, LoadOrGenerate), `internal/daemon/engine.go` settings save (atomic write plus SchemaVersion migration). Fields: `alwaysAvailable, grantID, iat, schemaVersion`. Do NOT put in settings.json (engine drops unknown fields).

---

### `internal/daemon/launchd.go` and start-path changes

**Analogs:** `internal/daemon/service.go:45-70` (old kardianos config: label `portty-daemon`, `"KeepAlive": false`, `Arguments: []string{"daemon"}`; reuse only for legacy `uninstall`), `internal/daemon/process_unix.go:25-57` (`startDetachedDaemon`: `exec.Command(exe,"daemon")`, `Setsid`, `openDaemonLog(daemonConfigDir())`, reaped via done channel). Replace spawn with `launchctl kickstart gui/<uid>/app.portty.Portty.daemon` through a Runner seam; keep `testing.Testing()` refusal. Render own plist (kardianos can't express `KeepAlive {SuccessfulExit:false}`): `RunAtLoad true`, no `ProcessType`. Never `kickstart -k`. CLI `EnsureDaemon` only kickstarts, never execs. Re-home: in `portty daemon` under launchd, if healthy same-version non-launchd daemon holds the lock, run `runTakeover`; exit 0 on refusal or abort, non-zero on real failure (`process.go:102-146` `runDaemonCore` returns normally on fatal, so audit those exit paths). Entry points: `process.go` `EnsureDaemon:232`, `ensureDaemon:259`, `startDaemon:414`.

---

### `internal/daemon/share.go` and `handoff.go` (serverNeeded)

**Edit sites:**
- `share.go:355-359` `unshareLocked`: `if ws != nil && len(a.shares) == 0 { _ = ws.Stop(); ...; a.webServer = nil }` becomes `!a.serverNeededLocked()`.
- `handoff.go:647-653`: `if ws != nil && len(g.Shares) > 0 {` pass listener, `g.ShareListener = true`, change to serverNeeded (gob already carries `ShareListener` + `Shares`, HandoffVersion stays 1).
- `handoff.go:1148-1156` `takeover.finalize`: `if len(shares) == 0 { t.dropShares(); return nil }` must instead build the server when desktop.json says always-available.
- Keep `API.Stop` (`api.go:343`) and handoff old-side `ws.Stop()` (`handoff.go:402`) unconditional. Lock order startMu, then a.mu; never hold a.mu across network or cert work. Tests pin both directions.

---

### `internal/webserver/desktop_routes.go` (route, request-response)

**Analog:** `internal/webserver/server.go`
Page gate (line ~622):
```go
mux.HandleFunc("GET /sessions/{id}",
	ws.cspHeaders(ws.requireSameUserPage(ws.requireCapabilityPage(ws.handleTerminalPage))))
```
SPA static handler (lines 691-725): `cspHeaders(requireSameUserPage(func...))`, `appFS := ws.staticAppFS`, nil -> 503, `serveIndex`, directory-listing guard (trailing slash falls to index), `fs.Stat` SPA fallback, and `w.Header().Del("Cache-Control")` for `assets/` (cspHeaders sets no-store; do not remove the Del). Mirror for `/desktop/`, but only the entry document checks the desktop token (hashed assets fetch without `?cap=`). WS route: copy `/sessions/{id}/ws` composition `requireAllowedOrigin(requireSameUser(<desktop token check>))`. Setter injection: `server.go:131` `shareToken func(sessionID string) (string, bool)` "Set once before Start via SetShareTokenProvider"; add `SetDesktopProvider`-style setters. Off state: static page "desktop sharing is off on this Mac" with `X-Portty` marker (copy refusal pages in `whois.go`). Page headers: no-store, `Referrer-Policy: no-referrer`. CSP: no change; add a test pinning /desktop policy equals the existing one (`csp_mw.go`, amendment history in file).

---

### `internal/daemon/desktop_api.go` (controller)

**Analog:** `internal/daemon/api.go` route registration, `handleRegenerateSigningKey` (re-mint pattern; must also invalidate desktop token and viewers). Endpoints: desktop status, enable/disable always-available, permission probe, test capture. Health poller is new (no existing one; `shareHealth` only read on share-on and takeover): start server when Tailscale connected with certs, restart on IP or domain change, detect userspace networking via `net.InterfaceAddrs`.

---

### `frontend/src/lib/webMode.ts` and `main.tsx`

**Current (webMode.ts:46-52):**
```ts
export function detectMode(loc: Location = window.location): AppMode {
  const path = loc.pathname
  if (path === '/app') return 'web'
  if (path.startsWith('/app/')) return 'web'
  return 'desktop'
}
```
Add a `/desktop` and `/desktop/` mode ('desktopview'; keep the `/app` boundary respect), unit test the three paths. `main.tsx:9` `if (detectMode() === 'desktop') ...classList.add('is-desktop')` must not fire for the viewer; branch before `<App />` renders `DesktopView` inside the existing `ErrorBoundary`. `readWebModeParams` already parses `?cap=`.

### `frontend/src/lib/desktopClient.ts` and `DesktopView.tsx`

**Analog:** `frontend/src/lib/hostReconnect.ts` (injectable deps argument defaulting to real bindings, verdict union `{kind:'retry'|'stop'|'ok'}`, any read failure is retry) and `lib/relayClient.ts` `ReconnectVerdict`. Build pluggable connection, a hook before redial, and a wake event (visibilitychange/pageshow). Capped backoff 1-10 s. Muted autoplay with tap-to-start overlay; verify video plays under the strict CSP (no `media-src` today).

### `frontend/src/components/SettingsSheet.tsx` Desktop group

**Analog:** same file, `Row({label, htmlFor, description, descriptionId, children})` at line 56; CT ack state at 110-111 and `HasCTDisclosure().then(...)` at 126, `handleAcknowledgeCT` at 170; Tailscale row ~384 with disabled reason (`tailscaleOff ? TAILSCALE_OFF_REASON : undefined`). Reuse `ct_disclosed` ack, `settings-group`, footnote. Bindings are imported from `../wailsjs/go/main/App`; add app.go bindings next to `GetSessionShare`/`HasCTDisclosure`.

### Notifications and tray

`notification_darwin.go`: `hasAppBundleID()` guard (go-test binary returns false) and `sendNotification(identifier, title, body)`; unique identifier per notification to avoid collapse. One notification per lapse, none on recovery. Daemon-side sender needs the same bundle identity; one sender at a time. Tray state line: add item in `tray.go`/`tray_common.go` following existing menu items; no new icon state.

## Shared Patterns

### Gate order for every share-server route
Origin, then WhoIs same-user, then capability. Pages use `requireSameUserPage` (static refusal 403/503 pages with `X-Portty` marker); sockets use `requireSameUser`. Source: `server.go` `/sessions/{id}` and `/ws` routes.

### Test seams
Package-var defaults, copied into the struct, `SetXForTest`; exec through Runner; `tailnet.LiveAllowed()` and `testing.Testing()` refusals so tests never touch tailscaled or launchd. Source: `share.go:30-45`.

### Crash containment
ObjC `@try`, Go `recover`, errors become state. Daemon holds PTY masters, so a crash ends all terminal sessions.

### Handoff compatibility
HandoffVersion stays 1; desktop sessions are not handed off (viewers reconnect); UDP mux port is not inheritable, new owner retries bind until old exits.

### TESTING.md standing rule
New test files go in Section 2 suite groups; add M-NN rows in Section 5 for non-automatable checks (M-157, M-159, M-160, M-161, M-163, M-164 already referenced); Section 4 traceability row per new test file; run `bash tests/check-traceability-paths.sh`.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| Daemon health poller (tailnet IP/cert watcher) | service | event-driven | No existing poller; `shareHealth` only read on share-on and takeover. Use RESEARCH lines ~334 |
| Crash-loop breaker (counter file) | service | file-I/O | Nothing similar; RESEARCH line ~394 (ASSUMED design) |

## Metadata

**Analog search scope:** `internal/capability`, `internal/daemon`, `internal/webserver`, `internal/wakewatch`, `spike/desktop`, `frontend/src/{lib,components,main.tsx}`, repo-root cgo files.
**Not fully read** (grep only): `media_darwin.m`, `sys_darwin.m`, `relayClient.ts`, `tray.go`, `api.go`; planner should open exact line ranges before copying.
**Pattern extraction date:** 2026-11-25
