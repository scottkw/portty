# Phase 13: Remote Localhost URLs - Pattern Map

**Mapped:** 2026-11-22
**Files analyzed:** 14 (9 new, 5+ modified)
**Analogs found:** 14 / 14 (all tracked in git; verified with `git ls-files` for the primary analogs)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/capability/porttoken.go` (new) | utility (token) | transform | `internal/capability/capability.go` (Claims, Sign, Verify) | exact |
| `internal/relay/portprinted.go` (new) | service (printed-text check) | request-response | `internal/relay/pathclick.go` (`pathSnapshot`, `findPrinted`, `hasToken`) | exact |
| `internal/relay/hub.go` (mod: `portPrintedBusy`, `Hub.PortPrinted`) | service | request-response | `pathClickBusy` at hub.go:101-103 | exact |
| `internal/webserver/portproxy.go` (new) | controller + middleware | streaming (HTTP and WS proxy) | `whois.go` `requireSameUser` and `handlePeerCap`; `capability_mw.go` `verifyCapability` | role-match |
| `internal/webserver/capability_mw.go` (mod: extract `verifyCapabilityToken`) | middleware | request-response | itself, lines 75-106 | exact |
| `internal/webserver/server.go` (mod: routes, `stopCh`, `SetDeniedPorts`) | route/config | request-response | server.go:514-517 route registration; `ClearGrants`/`revokeSignal` at 243-266 | exact |
| `internal/daemon/portforward.go` (new) | service | streaming | `remote_ws_proxy.go` (cap lookup, verified dial) plus `remote_caps.go` (memory-only store) | role-match |
| `internal/daemon/portforward_api.go` (new, or fold into `peer_connect.go`) | controller | request-response | `peer_connect.go` `handlePeerConnect` (typed error codes) | exact |
| `internal/daemon/peer_client.go` (mod: extract `peerTransport`) | utility | request-response | itself, lines 22-37 | exact |
| `internal/daemon/types.go` (mod: `APIVersion` 5, `PortProxyAPI`) | config | n/a | types.go:55-73 (`PreviewAPI`, `PathClickAPI`) | exact |
| `internal/daemon/api.go` (mod: register `POST /peers/ports`) | route | request-response | api.go:205-206 (`POST /peers/connect`) | exact |
| `app.go` (mod: `OpenRemotePort` binding) | controller (Wails) | request-response | `ConnectPeerSession` at app.go:1461-1474 | exact |
| `frontend/src/lib/localhostUrl.ts` (new) | utility | transform | `frontend/src/lib/openLink.ts` and `urlSafety.ts` (URL classification) | role-match |
| `frontend/src/components/TerminalPanel.tsx` (mod: click branch, notice) | component | event-driven | click handler at ~869-906; Phase 12 `pathNotice` at 338-352 | exact |
| consent sheet (reuse `RegenerateKeyModal.tsx`, no new file) | component | event-driven | `RegenerateKeyModal.tsx` copy overrides | exact |

## Pattern Assignments

### `internal/capability/porttoken.go` (utility, transform)

**Analog:** `internal/capability/capability.go`

**Claims shape and order rule** (lines 16-24): field declaration order is load-bearing, the HMAC signs the exact marshalled bytes.
```go
type Claims struct {
	SID     string `json:"sid"`
	Perms   string `json:"perms"`
	IAT     int64  `json:"iat"`
	GrantID string `json:"grant_id"`
	V       int    `json:"v"`
}
```
Copy Sign/Verify mechanics from the same file (base64url(payload).base64url(hmac), constant-time compare via `crypto/hmac`). Differences to apply: `PortClaims{SID, Port, IAT, GrantID, Kind:"port", V}`; key = `hkdf.Key(sha256.New, sessionKey, nil, "portty/port-token/v1", 32)` (RESEARCH Pattern, Token design); `VerifyPort` requires `Kind=="port"` and `V==1`. Session-route `Verify` must never accept these, so add a test that a port token fails `Verify` and a session cap fails `VerifyPort`.

---

### `internal/relay/portprinted.go` and `Hub.PortPrinted` in `hub.go` (service, request-response)

**Analog:** `internal/relay/pathclick.go`

**Snapshot and search** (pathclick.go:192-253): reuse `h.pathSnapshot()` (takes `emuMu`, copies rows, strips ANSI in `finish`) and generalise `findPrinted`/`firstPass`/`fallbackPass` to take a match predicate instead of calling `hasToken(run, text)`.
```go
func (s *pathSnap) findPrinted(text string) (uint64, bool) {
	if row, ok := s.firstPass(text); ok { return row, true }
	return s.fallbackPass(text)
}
```
**Token boundary pattern to replace for URLs** (pathclick.go:~313-328): `hasToken` uses `isPathChar`, which treats `/` as a token char. A new `hasURLPort(line, port)` needs the URL boundaries from RESEARCH Finding 5 (before: start, whitespace or `( [ { < " ' , = ;`; after: end, `/ ? # ) ] } > " ' , ;`, whitespace, or `.` followed by non-alphanumeric).
**Busy guard:** copy `pathClickBusy atomic.Bool` (hub.go:101-103) as `portPrintedBusy` with CAS; a busy hub answers the same refusal as any other.
**Test:** table test listed in RESEARCH "Pure matcher with table test".

---

### `internal/webserver/portproxy.go` (controller, streaming)

**Analog:** `internal/webserver/whois.go` (auth wrapper and route handler shape) and `internal/webserver/capability_mw.go`

**Same-user wrapper** (whois.go:88-116, fails closed, marker on refusal): wrap both new routes with it, exactly as server.go:517 does.
```go
mux.HandleFunc("POST /api/peer/sessions/{id}/cap", ws.requireSameUser(ws.handlePeerCap))
```
New registrations go next to this line (server.go:514-517): `POST /api/peer/sessions/{id}/ports` (grant) and the proxy route `/api/peer/sessions/{id}/ports/{port}` registered per method (a method-less subtree pattern panics against `GET /`). Do NOT wrap in `cspHeaders` or `porttyMarker`.

**Capability check to reuse** (capability_mw.go:75-106): `verifyCapability` reads `r.URL.Query().Get("cap")`. Extract `verifyCapabilityToken(token, pathID)` and have `verifyCapability` call it, so `?cap=` routes are unchanged; the new routes pass `r.Header.Get("X-Portty-Cap")` and also require `capability.HasPerm(claims.Perms, "write")`.
```go
claims, err := capability.Verify(token, key)
if err != nil { return capability.Claims{}, http.StatusUnauthorized, "capability required" }
if pathID := r.PathValue("id"); pathID != "" && claims.SID != pathID { ... StatusForbidden }
if !ws.isGrantActive(claims.SID, claims.GrantID) { ... }
if !ws.IsSessionEnabled(claims.SID) { ... }
```
**Revoke-bound context** (server.go:243-266 `ClearGrants`, `revokeSignal`; usage doc: take the signal, then re-check `isGrantActive`): follow RESEARCH Pattern 2; add a `stopCh` closed once in `Stop()` because api.go:339 and handoff.go:402 call `Stop` without `ClearGrants`.
**Refusal rule:** every new-host refusal carries `tailnet.MarkerHeader` and an identical body regardless of cause (match `refuseOtherUser` in whois.go). Check order: same-user, cap, port token, deny list (port 0, share port, relay port via `SetDeniedPorts`), `Hub.PortPrinted`, parse `X-Portty-Target`, then dial. Proxy mechanics: RESEARCH Pattern 1 (`Rewrite`, Host `localhost:P`, Origin on upgrades, dial `[::1]` then `127.0.0.1` only, strip `X-Portty-*`).
**Tests:** use `SetWhoIsClientForTest`; copy structure from `capability_test.go` and the existing peer-cap tests.

---

### `internal/daemon/portforward.go` (service, streaming)

**Analog:** `internal/daemon/remote_ws_proxy.go` (lines 47-100) and `internal/daemon/remote_caps.go` (lines 61-98)

**Cap lookup before anything else, JSON 404 on miss** (remote_ws_proxy.go:56-71):
```go
baseURL, ip, capToken, ok := a.remoteCaps.Get(sid)
if !ok {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "no cap registered for session"})
	return
}
```
**Memory-only rule:** approvals and forwards must hold nothing on disk; extend the source-grep test in `remote_caps.go`'s test to the new files. Per-request cap refresh: read `RemoteCapStore.Get` on every proxied request (peer-tab redial refreshes it). Log id8 and port only, never request targets (Vite HMR `?token=`).
**Transport:** from the extracted `peerTransport` (below), plus `DisableCompression: true`, `IdleConnTimeout: 30s`, `MaxIdleConnsPerHost: 8`, no total timeout. Probe-then-bind per RESEARCH Pattern 3; Host check per Pattern 4; ErrorHandler serves a fixed page with no reflected input.

---

### `internal/daemon/peer_client.go` (utility; extract `peerTransport`)

**Analog:** itself (lines 22-37)
```go
func peerHTTPClient(fqdn, addr string) *http.Client {
	d := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Client{
		Timeout: 10 * time.Second,   // do NOT reuse for proxied traffic
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				if addr == "" { return nil, errors.New("peer has no Tailscale IP") }
				return d.DialContext(ctx, network, addr)
			},
			TLSClientConfig: &tls.Config{ServerName: fqdn, MinVersion: tls.VersionTLS12},
		},
	}
}
```
Extract the `*http.Transport` into `peerTransport(fqdn, addr)`; `peerHTTPClient` wraps it unchanged. Keep the test-binary guard from `a.peerClient` (lines 43-57: `tailnet.LiveAllowed()`, `peerClientForTest`) in the new forward path so tests never dial a tailnet.

---

### `internal/daemon/portforward_api.go` (controller, request-response)

**Analog:** `internal/daemon/peer_connect.go` `handlePeerConnect` plus `daemon.PeerConnectError` typed codes (consumed at app.go:1461-1474). Mirror: request struct, typed error code constants (`daemon_outdated`, `no_cap`, `host_outdated`, `refused`, `unreachable`, `port_in_use`, `bad_url`), `writeJSON`. Re-parse the URL in Go with `net/url` and strict hostname compare against the five loopback forms; reject userinfo. Old host detection: 405/404 without the `X-Portty` marker means `host_outdated`; 403 with marker means `refused`.

**Route registration** (api.go:205-206), add one line after it:
```go
// Connect to a session shared from another Mac (CLI attach and the GUI).
a.mux.HandleFunc("POST /peers/connect", a.handlePeerConnect)
```
Socket only, never on the relay port.

---

### `internal/daemon/types.go` (config)

**Analog:** types.go:55-73
```go
const APIVersion = 4        // bump to 5; extend the doc comment: "5 adds port forwards (PortProxyAPI)"
const PathClickAPI = 4      // add: const PortProxyAPI = 5
```
GUI gate: `MinDaemonAPI`/`DaemonAPI(HealthInfo())` in client.go:86-97.

---

### `app.go` (Wails binding `OpenRemotePort`)

**Analog:** `ConnectPeerSession`, app.go:1461-1474
```go
func (a *App) ConnectPeerSession(host, sessionID string) PeerConnectResult {
	if a.client == nil { return PeerConnectResult{ErrorCode: daemon.PeerErrUnreachable} }
	res, err := a.client.ConnectPeer(host, sessionID)
	if err != nil {
		var pe *daemon.PeerConnectError
		if errors.As(err, &pe) { return PeerConnectResult{ErrorCode: pe.Code} }
		return PeerConnectResult{ErrorCode: daemon.PeerErrUnreachable}
	}
	return PeerConnectResult{SessionID: res.SessionID, ...}
}
```
Never throws; returns the URL and the frontend calls the existing `openLink` (keeps the "no current-tab navigation sink" grep test green). Add a client method next to `ConnectPeer`.

---

### `frontend/src/components/TerminalPanel.tsx` (component, event-driven)

**Analog:** the existing click handler, lines ~869-906 (shown above in `handler`), and Phase 12 notice (lines 78, 338-352).

**Insertion point:** after `isAllowedScheme(uri)` and the modifier gate, before `getRisk(uri, uri)`. Branch only when `remote && !wsURL` and `localhostUrl.classify(trimmed)` is a loopback form; otherwise fall through untouched (URL-03).
```ts
if (!isAllowedScheme(uri)) return
...
if (!isModifierPressed(event, modifier)) return
const risk = getRisk(uri, uri)
```
**Notice pattern** (lines 341-352): copy `pathNotice` state, `pathNoticeTimerRef` and `clearPathNotice`/`showPathNotice` into a sibling notice for `port-*` kinds ("needs a Portty update", "Couldn't reach", "Port N is in use on this Mac").
**Consent:** render `RegenerateKeyModal` with `title`, `body`, `confirmLabel="Allow"`, `cancelLabel="Don't Allow"`, `onConfirm` re-calling the binding with `consent=true`; it focuses Cancel and maps Esc to cancel via `useDialogFocus`.
**Tests to update deliberately:** TerminalPanel.web-links.test.tsx, openLink.test.ts, App.webLink.test.tsx, WebShareSessionView.peerLinks.test.tsx, LinkConfirmPopover.test.tsx, urlSafety.test.ts, internal/webserver/web_links_test.go.

---

### `frontend/src/lib/localhostUrl.ts` (utility, transform)

**Analog:** `frontend/src/lib/openLink.ts` and the urlSafety helpers (URL parse with `new URL`, scheme allowlist). Export `classifyLoopbackUrl(raw)`: trim trailing `) ] } > , ; . ' "` (and unbalanced `)`), parse with `new URL`, accept hostnames `localhost`, `127.0.0.1`, `[::1]`, `0.0.0.0`, `[::]`, reject userinfo and non-http, return `{port, pathAndQuery}`. Share a JSON fixture with the Go parser (RESEARCH "Don't Hand-Roll").

## Shared Patterns

### Fail-closed refusals with the marker
**Source:** `internal/webserver/whois.go` (`refuseOtherUser`, `refuseLocalFault`), `tailnet.MarkerHeader` (tailnet.go:35-36)
**Apply to:** both new host routes. Identical body and status for every refusal cause; 503 only for a local tailscaled fault.

### Capability and grant lifetime
**Source:** `internal/webserver/capability_mw.go:75-106`, `server.go:243-266`
**Apply to:** grant route, proxy route, and the revoke-bound context. Order: take `revokeSignal`, re-check `isGrantActive`.

### Memory-only secrets
**Source:** `internal/daemon/remote_caps.go` and its source-grep test
**Apply to:** `portforward.go`, `porttoken.go`, `portproxy.go`.

### No live tailnet in tests
**Source:** `tailnet.LiveAllowed`, `SetPeerClientForTest`, `SetWhoIsClientForTest`
**Apply to:** every new Go test.

### Version gate
**Source:** `types.go` constants plus `client.go:86-97`
**Apply to:** daemon (`PortProxyAPI`), GUI (daemon_outdated), viewing daemon (host_outdated by marker).

### Regression-suite convention (project CLAUDE.md)
Every new test file gets a row in TESTING.md Sections 2 and 4; add manual M-NN items for the VM checks (lsof loopback bind, Vite HMR, python3 http.server); run `bash tests/check-traceability-paths.sh` before committing.

## No Analog Found

| File or Concern | Role | Data Flow | Reason |
|---|---|---|---|
| `httputil.ReverseProxy` usage (both hops) | proxy | streaming | No `httputil` reverse proxy exists in the repo; use RESEARCH Patterns 1 and 2 (verified by experiment). `remote_ws_proxy.go` is a websocket-library relay, not an HTTP reverse proxy. |
| Probe-then-bind loopback listener | listener | n/a | No loopback-forward listener exists; use RESEARCH Pattern 3. |
| HKDF key derivation | utility | transform | No `crypto/hkdf` use yet; RESEARCH gives the exact call. |

## Metadata

**Analog search scope:** `internal/capability`, `internal/relay`, `internal/webserver`, `internal/daemon`, `app.go`, `frontend/src/components`, `frontend/src/lib`
**Files read for excerpts:** capability.go, capability_mw.go, whois.go, server.go (243-270), pathclick.go, peer_client.go, remote_ws_proxy.go, remote_caps.go, types.go, api.go, app.go, TerminalPanel.tsx, RegenerateKeyModal.tsx
**Not read (cite by line from CONTEXT/RESEARCH):** peer_connect.go body, openLink.ts, urlSafety.ts, client.go. RESEARCH.md lines 451-603 (Validation, Security, Environment sections) were not read.
**Pattern extraction date:** 2026-11-22
