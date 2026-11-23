# Phase 13: Remote Localhost URLs - Research

**Researched:** 2026-11-22
**Domain:** Two-hop authenticated HTTP/WebSocket reverse proxy over a tailnet (Go `net/http/httputil`), loopback listener hygiene on macOS, capability-token design, React consent sheet
**Confidence:** HIGH on stack, wire design and macOS socket behaviour (each verified by a local experiment this session); MEDIUM on browser resolver order and dev-server specifics (documentation only, no browser or dev server was run)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Proxy wire contract**
- Per-request HTTP reverse proxy on both hops: browser → the viewing daemon's listener on 127.0.0.1:PORT → HTTPS to a new share-server route keyed by session and port → the host proxies to its own loopback port. `httputil.ReverseProxy` handles `Upgrade: websocket`. The port token travels in a request header, never in a URL. — **Reversibility:** one-way — cross-Mac route shape tied to API version 5; both daemons must agree.
- The browser uses the **same port number** as on the remote Mac (http://localhost:8000 stays http://localhost:8000), so absolute URLs, redirects, OAuth callbacks and dev-server Host/Origin checks need no rewriting. Research must settle macOS SO_REUSEADDR shadowing (127.0.0.1:P bindable while *:P is held) and whether browsers resolve `localhost` to ::1 first. If the port is genuinely taken on this Mac, refuse with a notice, never pick another port silently. — **Reversibility:** costly — URL shape and origin the user's browser and dev servers see.

**Security contract**
- The host mints a **port-scoped signed token** on request (HMAC; claims: session, port, grant tied to the share), so stopping the share kills it. The host refuses any proxy request whose token doesn't name that port, before dialling anything. The port token must NOT verify on any existing session route (today `verifyCapability` accepts any verified claims for the session, capability_mw.go:93-104, and `Verify` ignores unknown fields, capability.go:99-101). Use a distinct token kind or audience so it can't be confused with a session capability. — **Reversibility:** one-way — auth model and cross-Mac token format.
- **Anti-scan printed check:** the host grants a port only if `http://localhost:PORT` (or one of the accepted host forms below) was printed in that session's retained output, reusing the Phase 12 printed-text machinery (relay/pathclick.go `pathSnapshot`/`findPrinted`) with URL token boundaries. Every refusal looks identical whatever the port's state, and no connection is made before all checks pass. — **Reversibility:** one-way — security contract, same as Phase 12's.
- **Who:** `requireSameUser` (WhoIs) plus a valid write capability, on both the grant route and the proxy route. A share link leaked to another tailnet user can't reach ports. Consistent with Phase 14 MOB-05. — **Reversibility:** costly — auth model.
- **Viewing-side listener:** binds 127.0.0.1 only (lsof check in the VM run). Accept only `Host: localhost:P` or `127.0.0.1:P` on every request, which blocks DNS rebinding. No token in browser URLs; port tokens never leave the viewing daemon. URL-02's "token-gated" applies to the tailnet hop. — **Reversibility:** costly — auth model.

**Consent (Claude's discretion, user said "You decide")**
- Surface: the existing in-window sheet style (`.dialog`, RegenerateKeyModal pattern: copy overrides, focus on Cancel, Esc = cancel). Follows "one macOS sheet style for every modal" and is testable in vitest.
- Copy: title "Allow access to {host}'s localhost:{port}?"; body "Pages from this port open in your browser through Portty over your tailnet. Other ports stay closed."; buttons "Don't Allow" / "Allow". Declining opens nothing.
- Lifetime: memory only, per (Mac, port). Ends when that Mac stops sharing, the session's grant is revoked, or this Mac's daemon restarts. Nothing on disk (Phase 4 rule).
- Revocation: implicit only (stop sharing, session end, daemon restart). No indicator or settings list this phase (radical simplicity); revisit if the user asks.

**Scope and failures (Claude's discretion)**
- URLs: `http://localhost:PORT`, `http://127.0.0.1:PORT`, plus `http://[::1]:PORT`, `http://0.0.0.0:PORT` and `http://[::]:PORT`, which servers like `python3 -m http.server` print. The host always dials its own loopback (127.0.0.1, or ::1 for the IPv6 forms), never another address. The browser always opens `http://localhost:PORT`. Desktop peer tabs only; the browser share view and https://localhost keep today's behaviour.
- Failures: before opening, a transient notice in the clicking pane (Phase 12 notice pattern; e.g. "{host} needs a Portty update to open its local pages", "Couldn't reach {host}", "Port {port} is in use on this Mac"). After opening, a plain error page served by the viewing daemon in the browser (e.g. "Nothing is answering on {host}'s port {port}", "{host} stopped sharing"). Never fall back to opening this Mac's own localhost.

**Decided autonomously**
- [Proxy] Q: "Where does the viewing-side listener live?" → Selected: "The viewing Mac's daemon (RemoteCapStore, the verified peer client and the remote_ws_proxy template already live there)" — internal to one Mac, no cross-Mac contract.
- [Proxy] Q: "WebSockets, streaming and stopping the share" → Selected: "Track proxied upgrades and long responses per session; close them on ClearGrants/revokeSignal because hijacked conns survive Stop; the upstream client uses a dial timeout only, not peerClient's 10 s total timeout" — internal plumbing.
- [Proxy] Q: "Version gating and upgrades" → Selected: "APIVersion 5 with PortProxyAPI on both daemons; the viewing daemon learns host support at click time and returns a typed error; an old local daemon shows update needed; forwards end on a Phase 9 handoff and the next click reopens them" — additive gate.
- [URL] Q: "Rewriting Host, Origin and bodies" → Selected: "Host sets Host (and Origin on upgrades) to localhost:PORT; no body rewriting; cookies untouched; Location rewritten only if the local port ever differs" — host-side header logic.

### Claude's Discretion
Consent surface, copy, approval lifetime, revocation visibility, URL scope and failure handling, as recorded above. The planner may adjust details within these choices.

### Deferred Ideas (OUT OF SCOPE)
- An open-ports indicator or a Settings list for revoking ports (chose implicit revocation for now).
- Persisting approvals across restarts.
- Proxying for the browser share view (same-origin hazard with /app/).
- Carrying forwards across a Phase 9 handoff.
- Pending from Phase 12: WR-01 ("last occurrence" path resolution) accept vs refuse-ambiguous, and Option-click `altClickMovesCursor` keep vs turn off: both await the user.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| URL-01 | Clicking `http://localhost:PORT` or `http://127.0.0.1:PORT` in a remote session opens that page from the remote Mac in the user's normal browser, proxied over the tailnet by Portty, including WebSocket upgrades so dev-server hot reload works | Two-hop `ReverseProxy` verified end to end (HTTP, WebSocket echo, SSE first chunk 1 ms) in a local experiment; header-carried target keeps `//`, `%2e%2e`, `%2F` and the query byte-exact; host dials `::1` then `127.0.0.1` (Node 24 `listen(port,'localhost')` binds `::1` only, verified); python 3.15 prints `http://[::]:PORT/` (verified) |
| URL-02 | The proxy is token-gated and tailnet-only, asks consent per port, binds only on the viewing Mac's loopback, and never allows arbitrary port scanning | Derived-key port token (HKDF) cannot verify on session routes; check order refuses before any dial; printed check with URL boundaries; both-loopback bind with a dial-probe because macOS lets a specific bind shadow a wildcard holder (verified); host deny list for the Portty ports; consent sheet via `RegenerateKeyModal` copy overrides |
| URL-03 | Local URLs keep today's behaviour: confirm popover, then the default browser | New branch only when `remote && !wsURL` and the URL host is one of the five loopback forms; every other URL reaches the unchanged `getRisk`/`openLink` path; list of link tests to update deliberately |
</phase_requirements>

## Summary

The phase is one new capability built almost entirely from parts that already exist: the peer client and RemoteCapStore (viewing side), `requireSameUser`, capability HMAC and `ClearGrants`/`revokeSignal` (host side), and the Phase 12 printed-text snapshot. No new dependency is needed; `net/http/httputil.ReverseProxy` (stdlib) does both hops, including WebSocket upgrades, and `crypto/hkdf` (stdlib, Go 1.24+) gives the token domain separation. A two-hop prototype (browser-side client → loopback listener → TLS/HTTP1.1 share-server stand-in → loopback "dev server") passed plain GET, a WebSocket echo with the dev server's Origin check satisfied, and an SSE stream whose first chunk arrived after 1 ms. Cancelling the host request context closed both the upgraded socket and the SSE stream across both hops in about 1 ms.

Four findings change the shape of the plan relative to CONTEXT.md and need planner attention. (1) Do not carry the target path in the mux path: `ServeMux` answers 307 with the internal route prefix for any `//` or `..` path, and a method-less subtree pattern panics at startup against the existing `GET /` catch-all. Carry the original request target in a header and use a fixed route keyed by session and port. (2) The host must try `[::1]` and `127.0.0.1`, not 127.0.0.1 alone for the `localhost` form: Node 24 `server.listen(port, 'localhost')` binds `::1` only on this Mac, so any Node dev server (Vite, Next, webpack) that prints `http://localhost:PORT` refuses IPv4. This stays inside "own loopback only". (3) macOS lets `127.0.0.1:P` and `[::1]:P` bind while a foreign `*:P` holds the port (silent shadowing, verified), and a strict `SO_REUSEADDR=0` bind detects it but then fails for ~30 s after the forward's own connections close (TIME_WAIT, verified). Detect "taken" with a dial-probe of both loopbacks (anything but connection-refused means taken), then bind both families with the default options. (4) The xterm link regex returns `http://[::]:19106/)` for python's real output line (trailing `)` included), so the proxy branch needs URL tail trimming that today's path does not.

**Primary recommendation:** Build `internal/daemon/portforward.go` (viewing-side loopback forwards, approvals, error page) and `internal/webserver/portproxy.go` (grant and proxy routes), add `capability.SignPort/VerifyPort` under an HKDF-derived key, add `Hub.PortPrinted(port)` next to `PathClick`, bump `APIVersion` to 5, and branch the existing TerminalPanel click handler only for desktop peer tabs with a loopback-form host.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Detect and classify a clicked loopback URL | Browser / Client (TerminalPanel, `lib/`) | API / Backend (viewing daemon revalidates in Go) | The click lands in the webview; the daemon re-parses with strict string compare so a JS/Go parser difference cannot widen scope |
| Consent sheet and notices | Browser / Client (React) | — | Same sheet style as RegenerateKeyModal; vitest-testable |
| Approvals (Mac, port) and forward lifetime | API / Backend (viewing daemon, memory) | — | CONTEXT: ends on daemon restart, nothing on disk |
| Loopback listener, Host check, error page | API / Backend (viewing daemon) | — | Binds only 127.0.0.1 and [::1]; the webview never talks to the host |
| Reverse proxy hop 1 (to the host) | API / Backend (viewing daemon) | — | Uses the verified peer transport (stored tailscaled IP, cert checked against MagicDNS name) |
| Authenticate caller (same user, write cap, port token) | API / Backend (host share server) | — | Fails closed before any dial |
| Printed-text anti-scan check | API / Backend (host relay Hub) | — | Only the host holds the session's retained output |
| Reverse proxy hop 2 (to loopback) | API / Backend (host share server) | — | Dials only the host's own loopback |
| Revoke on stop-share | API / Backend (host `ClearGrants`/`revokeSignal`) | Viewing daemon (reacts to upstream refusal) | Hijacked conns survive `srv.Close`, so context cancel is the closing mechanism |
| Page content, cookies, service workers | Browser (user's normal browser) | — | Origin `http://localhost:P`; Portty never touches bodies |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http/httputil` (stdlib) | Go 1.27.4 on this Mac; go.mod `go 1.26.4` [VERIFIED: go version; go.mod:3] | Both proxy hops, WebSocket upgrade, SSE flush | `handleUpgradeResponse` hijacks and copies, ties backend close to `req.Context().Done()` [VERIFIED: GOROOT/src/net/http/httputil/reverseproxy.go:822-888]; SSE and unknown length flush immediately (`flushInterval` returns -1) [VERIFIED: reverseproxy.go:681-696] |
| `crypto/hkdf` (stdlib) | Go 1.24+ [VERIFIED: GOROOT/src/crypto/hkdf/hkdf.go:59 `func Key[Hash hash.Hash](h func() Hash, secret, salt []byte, info string, keyLength int) ([]byte, error)`] | Derive the port-token key from the session signing key | Domain separation without a second key file |
| `github.com/coder/websocket` | v1.8.15 (already required) [VERIFIED: go.mod:14] | Tests only (WS echo through two hops) | Already the project's WebSocket library; the proxies themselves do not use it (they copy raw upgraded conns) |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@xterm/addon-web-links` | 0.12.0 (installed) [VERIFIED: node_modules/@xterm/addon-web-links/package.json] | Existing URL link detection | No change; its matches include trailing punctuation, so trim before routing |
| `RegenerateKeyModal` + `useDialogFocus` | in repo | Consent sheet | Reuse with copy overrides (`title`, `body`, `confirmLabel`, `cancelLabel`, `actingLabel`); focuses Cancel, Esc cancels |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Header-carried target (recommended) | Target path embedded in the host route (`.../ports/{port}/{rest...}`) | Embedded path hits ServeMux cleaning: `a//b` and `a/../b` answer 307 with the internal prefix in `Location` [VERIFIED: local experiment exp3]; also needs RawPath bookkeeping. Rejected. |
| HKDF-derived token key | Same key plus an `aud` claim | `capability.Verify` unmarshals into `Claims` and ignores unknown fields, and `verifyCapability` never reads `Perms`, so a same-key port token with `sid` and `grant_id` would pass on `/api/sessions/{id}/info` [VERIFIED: capability_mw.go:78-104 read this session]. Separate key makes confusion impossible regardless of claim shape. |

**Installation:** none. No new Go or npm packages.

**Version verification:** nothing to install. `coder/websocket` v1.8.15 is already in go.mod and in the module cache (`websocket@v1.8.15` present) [VERIFIED: ls GOMODCACHE].

## Package Legitimacy Audit

No external packages are added by this phase (stdlib plus already-vendored `coder/websocket` and existing npm deps), so the legitimacy gate has nothing to run.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none new) | — | — | — | — | — | Approved: nothing installed |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
 VIEWING MAC (home)                                   HOST MAC (office)
 ------------------------------------------           --------------------------------------------------
 xterm in desktop peer tab (remote && !wsURL)
   Cmd-click "http://[::]:8000/)"  --trim/parse-->  loopback form? no -> existing getRisk/openLink (URL-03)
        | yes
        v
 Wails binding OpenRemotePort(session, url, consent)
        v
 viewing daemon  POST /peers/ports (unix socket)
   1 caps = RemoteCapStore.Get(sid)         -- none -> no_cap
   2 POST host .../sessions/{id}/ports ------------------------------------> requireSameUser (WhoIs, fail closed)
      (X-Portty-Cap header, JSON {port})                                      write cap in header (sid, grant, enabled, "write")
      405 w/o X-Portty marker => host_outdated                                deny list {0, share port, relay port}
      403 + marker => refused (all causes identical)                          Hub.PortPrinted(port)  (snapshot + URL boundaries)
      200 {token}  <----------------------------------------------------      mint PortClaims under HKDF key   [NO dial anywhere]
   3 dial-probe 127.0.0.1:P and [::1]:P : need ECONNREFUSED on both  -- else port_in_use
   4 approvals[(host,P)] missing and !consent -> return consent_required (GUI shows sheet, calls again with consent)
   5 bind 127.0.0.1:P and [::1]:P ; start forward(sid,P,token) ; return http://localhost:P + path
        v
 GUI: openLink("http://localhost:8000/...") -> BrowserOpenURL -> default browser
        v
 browser GET/WS  http://localhost:8000/x?y   (Host localhost:8000)
        v
 forward listener (127.0.0.1:P, [::1]:P)
   Host in {localhost:P, 127.0.0.1:P} else 421/403   (bare "localhost" accepted when P==80)
   ReverseProxy.Rewrite: URL -> host route, headers X-Portty-Cap / -Port-Token / -Target set (inbound copies stripped)
        |  HTTPS, HTTP/1.1, tailscaled IP, cert vs MagicDNS name, dial+TLS timeouts only
        v ------------------------------------------------------------>  route ANY /api/peer/sessions/{id}/ports/{port}
                                                                           same-user -> cap -> port token(kind,sid,port==path,grant active)
                                                                           -> deny list -> parse X-Portty-Target
                                                                           ctx := cancel on revokeSignal(sid) | server Stop
                                                                           ReverseProxy.Rewrite: Host=localhost:P, Origin on Upgrade,
                                                                             strip X-Portty-*  ; Transport dials ::1 then 127.0.0.1 only
                                                                                      v
                                                                           dev server on host loopback :P
 ErrorHandler on viewing side: host 502 marker -> "Nothing is answering"; 403/ECONNREFUSED -> "stopped sharing",
   close forward, drop approval
```

### Recommended Project Structure
```
internal/
├── capability/porttoken.go          # PortClaims, SignPort, VerifyPort, derived key
├── relay/portprinted.go             # Hub.PortPrinted + URL token boundaries (generalise pathSnap search)
├── webserver/portproxy.go           # grant route, proxy route, deny list, revoke-bound context
├── daemon/portforward.go            # PortForwards: approvals, forwards, probe+bind, error page
├── daemon/portforward_api.go        # POST /peers/ports handler, typed errors (or fold into peer_connect.go)
├── daemon/types.go                  # APIVersion 5, PortProxyAPI const, request/response/error codes
└── daemon/peer_client.go            # extract peerTransport(fqdn, addr) used by peerHTTPClient and the forward
app.go                               # OpenRemotePort binding (never throws, typed errorCode)
frontend/src/lib/localhostUrl.ts     # classify + trim; shared JSON fixture with the Go parser
frontend/src/components/TerminalPanel.tsx   # one new branch in the existing handler
```

### Pattern 1: Header-carried target, fixed host route
**What:** The viewing daemon sets `X-Portty-Target: <In.URL.RequestURI()>` and sends every request to one fixed path. The host parses it with `url.ParseRequestURI` and copies `Path`, `RawPath`, `RawQuery` onto the loopback request.
**When to use:** Always for this phase; it is the only way to keep the page's own paths byte-exact and keep the page's query string out of the share server's URL (a dev app's own `?cap=` can never collide with `verifyCapability`'s `r.URL.Query().Get("cap")` [VERIFIED: capability_mw.go:78]).
**Example:**
```go
// Source: local experiment scratchpad/exp2 (passed: "GET /a//b/%2e%2e/c%2Fd" reached the dev server as
// path=/a//b/%2e%2e/c%2Fd, Host="localhost:<port>", X-Portty-* absent, no X-Forwarded-For)
hostProxy := &httputil.ReverseProxy{
    Rewrite: func(pr *httputil.ProxyRequest) {
        pr.SetURL(&url.URL{Scheme: "http", Host: "127.0.0.1:" + port}) // host part is ignored by the custom dialer
        t, err := url.ParseRequestURI(pr.In.Header.Get("X-Portty-Target")) // validate before this point; "/" prefix required
        if err != nil { /* unreachable: validated earlier, refuse before dialling */ }
        pr.Out.URL.Path, pr.Out.URL.RawPath, pr.Out.URL.RawQuery = t.Path, t.RawPath, t.RawQuery
        pr.Out.Header.Del("X-Portty-Target"); pr.Out.Header.Del("X-Portty-Cap"); pr.Out.Header.Del("X-Portty-Port-Token")
        pr.Out.Host = "localhost:" + port
        if strings.EqualFold(pr.In.Header.Get("Upgrade"), "websocket") {
            pr.Out.Header.Set("Origin", "http://localhost:"+port)
        }
    },
}
```
Use `Rewrite`, not `Director`: with `Rewrite` the stdlib removes inbound `Forwarded`/`X-Forwarded-*` before calling you [CITED: pkg.go.dev/net/http/httputil#ReverseProxy, reverseproxy.go:111-135 read this session], so a client cannot smuggle `X-Forwarded-For` to the dev server.

### Pattern 2: Close proxied conns on revoke with a derived context
**What:** Wrap each proxy request's context so `revokeSignal(sid)` (closed by `ClearGrants`) and a server-wide stop channel cancel it. `ReverseProxy` already closes the backend and the hijacked conn when `req.Context()` is done.
**Example:**
```go
// Source: local experiment scratchpad/exp2 ("WS after revoke: closed in 0s", "SSE after revoke: ended in 1ms"),
// mechanism: reverseproxy.go:847-857 (backConn closed on req.Context().Done()), then defer conn.Close() at :863
ctx, cancel := context.WithCancel(r.Context())
defer cancel()
sig := ws.revokeSignal(claims.SID)          // existing helper, server.go:257
if !ws.isGrantActive(claims.SID, claims.GrantID) { refuse(w); return } // re-check after taking the signal (see its doc comment)
go func() {
    select {
    case <-sig:       cancel()
    case <-ws.stopCh: cancel()              // new: closed once in Stop(), so handoff/shutdown also close hijacked conns
    case <-ctx.Done():
    }
}()
proxy.ServeHTTP(w, r.WithContext(ctx))
```
`ClearGrants` runs first in `unshareLocked` (share.go order: `ws.ClearGrants(id)` then `DisableSession`, `RevokeWebViewers`, `Stop`) [VERIFIED: share.go:342-357 read this session], and also on session exit (api.go:450 calls `unshareLocked`). Two other `Stop` callers (api.go:339 shutdown, handoff.go:402) do not call `ClearGrants`, hence the `stopCh`.

### Pattern 3: Probe, then bind both loopbacks (viewing side)
**What:** Refuse unless dialing both `127.0.0.1:P` and `[::1]:P` fails with connection-refused (or `[::1]` is unavailable); then bind both with default options.
**Why not a strict bind:** `SO_REUSEADDR=0` correctly returns EADDRINUSE against a foreign `*:P` holder (B1/B2 below) but also fails for ~30 s after this forward's own server-side-closed connections (E1), so a re-click after Stop would wrongly say "in use".
**Example:**
```go
// Source: local experiment scratchpad/exp1 (macOS 27.2.1, Go 1.27.4)
func portFree(port string) error {
    for _, addr := range []string{"127.0.0.1:" + port, "[::1]:" + port} {
        c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
        if err == nil { c.Close(); return errPortInUse }
        switch {
        case errors.Is(err, syscall.ECONNREFUSED):          // free on this family
        case errors.Is(err, syscall.EADDRNOTAVAIL), errors.Is(err, syscall.ENETUNREACH), errors.Is(err, syscall.EAFNOSUPPORT):
            // IPv6 loopback absent: only acceptable for the [::1] probe
        default:
            return errPortInUse                              // timeout, reset, anything else: fail closed
        }
    }
    return nil
}
// then net.Listen("tcp4","127.0.0.1:"+port); net.Listen("tcp6","[::1]:"+port) (EADDRNOTAVAIL on the second is tolerated);
// any EADDRINUSE closes whatever was opened and returns port_in_use.
```

### Pattern 4: Viewing-side Host check
Accept `r.Host` equal to `localhost:P` or `127.0.0.1:P`; when `P == 80` browsers omit the port, so also accept bare `localhost` and `127.0.0.1`. Everything else gets a fixed refusal with no reflection. `[::1]:P` is deliberately not accepted (CONTEXT lists two forms; the browser always opens `localhost`).

### Anti-Patterns to Avoid
- **Embedding the target path in the host mux path:** 307 redirects leak the internal prefix and change the path (see Alternatives).
- **A method-less subtree pattern on the share mux:** `mux.HandleFunc("/api/peer/sessions/{id}/ports/{port}/{rest...}", h)` panics at registration: "matches more methods than GET /, but has a more specific path pattern" [VERIFIED: exp3, against the real `GET /` registration at server.go:495]. Register per method (GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD), or intercept before the mux.
- **Reusing `peerHTTPClient` as is:** its `http.Client{Timeout: 10 * time.Second,` (peer_client.go:27) cuts every long response and every WebSocket after 10 s. Extract the `*http.Transport` and use it as the `ReverseProxy.Transport`.
- **Wrapping the proxy route in `cspHeaders` or `porttyMarker`:** the proxied app's own CSP must reach the browser untouched, and Portty headers must not (strip any `X-Portty*` response header on the viewing side).
- **Using `Authorization` for Portty's cap:** dev apps use that header themselves. Use `X-Portty-Cap`, `X-Portty-Port-Token`, `X-Portty-Target`, and `Header.Set`/`Del` them on every hop so a page cannot inject its own.
- **Logging request targets on either hop:** Vite's HMR URL carries `?token=`. Log session id8 and port only.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP proxying with upgrade, hop-by-hop stripping, trailers, SSE flush | A copy loop over raw conns | `httputil.ReverseProxy` with `Rewrite` | Handles `Connection`/`Upgrade` rules, 101 hijack, flush rules, request-context cancel of the backend (all in the stdlib source read above) |
| Token domain separation | A second key file, or an `aud` claim on the shared key | `hkdf.Key(sha256.New, signingKey, nil, "portty/port-token/v1", 32)` | Rotation (`regenerate-key`) kills port tokens automatically; no new state |
| Same-user check | New WhoIs code | `ws.requireSameUser` (whois.go:88, fails closed) | Already tested with `SetWhoIsClientForTest` |
| Verified tailnet dial | New dialer | the `peerHTTPClient` dial and `TLSClientConfig{ServerName: fqdn}` setup, extracted as `peerTransport` | CR-02: dials tailscaled's IP, never system DNS |
| Printed-text search | A new scrollback scan | `pathSnapshot()` plus the soft-wrap join passes (`firstPass`/`fallbackPass`), parameterised on the match predicate | Wrap marks, ANSI strip and the live screen are subtle; Phase 12 pinned them |
| Consent dialog | New modal component | `RegenerateKeyModal` with overrides | One sheet style, focus and Esc handled by `useDialogFocus` |
| URL parsing (Go) | Regex on the click text | `net/url.Parse` then strict string compare of `Hostname()` against the five canonical forms, reject userinfo | `http://localhost:8000@evil.com/` has hostname `evil.com` |

**Key insight:** every dangerous edge (header smuggling, rebinding, token confusion, port scanning) is closed by ordering and exact-match checks, not by clever parsing. Keep each check a pure function with a table test.

## Runtime State Inventory

Not a rename/refactor/migration phase. Omitted.

## Research Findings by Topic (the seven questions in CONTEXT "Specific Ideas")

### 1. macOS sockets, SO_REUSEADDR, localhost resolution

Experiment: `scratchpad/exp1/main.go`, macOS 27.2.1, Go 1.27.4. Results (verbatim lines):

| Case | Result |
|------|--------|
| A0 foreign `:19101` (dual-stack, REUSEADDR on), A1 bind `127.0.0.1:P` REUSEADDR=1 | `err=<nil>`: **silent shadow**. `GET 127.0.0.1:P => OURS4`, `GET [::1]:P => OURS6` (we also bound `[::1]` fine) |
| A3 rebind `127.0.0.1:P` with REUSEADDR=0 while we hold it | `bind: address already in use` |
| B foreign `:P`, we bind `127.0.0.1:P` / `[::1]:P` with REUSEADDR=0 | both `bind: address already in use` (strict bind detects a wildcard holder) |
| C foreign `[::1]:P` only, we bind `127.0.0.1:P` | OK. `GET [::1]:P => FOREIGN6`, `GET localhost:P => FOREIGN6`: **Go's `localhost` resolved to `::1` first** and reached the foreign server, not ours. Binding `[::1]:P` too fails with EADDRINUSE |
| D foreign `127.0.0.1:P`, we bind `127.0.0.1:P` (both REUSEADDR) | `address already in use` |
| E server-side close then rebind `127.0.0.1:P` | REUSEADDR=0: `address already in use`; REUSEADDR=1: OK (TIME_WAIT) |

So: yes, `127.0.0.1:P` is bindable while `*:P` is held and it silently shadows the foreign listener for IPv4 loopback; SO_REUSEADDR (Go sets it on by default) is what allows it [VERIFIED: exp1]. Conclusion: bind both families (otherwise a `localhost` request can reach a `::1` holder instead of the forward, case C) and detect "taken" with the dial-probe in Pattern 3.

`/etc/hosts` has `127.0.0.1 localhost` then `::1             localhost`, and `dscacheutil -q host -a name localhost` listed `::1` first on this Mac [VERIFIED: command output this session]. **Safari and Chrome order:** not verified. Chromium resolves `localhost` internally to both families without DNS [CITED: codereview.chromium.org/1177933002, "Resolve RFC 6761 localhost names to loopback": plain localhost yields IPv4 and IPv6 endpoints]; I found no authoritative statement of which is tried first, or that Safari retries IPv4 after a refused `::1` (forum reports only). [ASSUMED] The design does not depend on it: with both loopbacks bound, either order lands on the forward.

### 2. Dev servers behind a same-port proxy with Host/Origin rewritten to `localhost:PORT`

Because the browser is on `http://localhost:P` and the host rewrites Host to `localhost:P` and Origin to `http://localhost:P` on upgrades, the dev server sees exactly what it sees with a local browser. Evidence per server:
- **Vite:** `server.allowedHosts` default allows `localhost`, `*.localhost` and all IPs; `server.cors` default allows localhost, 127.0.0.1 and ::1 origins [CITED: vite.dev/config/server-options]. The CVE-2025-24010 fix (6.0.9, 5.4.12, 4.5.6) added Origin validation and a WebSocket token; the opt-out is `legacy.skipWebSocketTokenCheck` [CITED: NVD/Snyk snippets via search, mechanism details not fetched]. The token is embedded in the page JS the same dev server serves, and the proxy does not rewrite bodies or strip `Sec-WebSocket-Protocol`, so HMR should work unmodified. [ASSUMED: not run against Vite]
- **webpack-dev-server:** `allowedHosts: 'auto'` always allows `localhost`, `host` and `client.webSocketURL.hostname` [CITED: webpack.js.org/configuration/dev-server]. The docs say nothing about the WebSocket Origin check. [ASSUMED] An Origin of `http://localhost:P` is accepted.
- **Next.js dev:** blocks cross-origin requests to dev-only assets/endpoints by default; only the **hostname** of Origin is matched, port ignored; `localhost` and its subdomains are allowed [CITED: nextjs.org/docs/app/api-reference/config/next-config-js/allowedDevOrigins]. CVE-2026-27977 (fixed 16.1.7) concerns `Origin: null` bypass, not this path.
- **Multi-port apps:** an app whose page calls another port (frontend 5173 calling API 3001) fetches `http://localhost:3001` from the browser. That is a separate (Mac, port): it works only if 3001 was printed and the user clicked and approved it; otherwise the browser hits this Mac's own `localhost:3001` or nothing. Document as a limitation, do not auto-approve.
- **Typed 127.0.0.1:** the browser always opens `localhost`, but if the user types `http://127.0.0.1:P` the viewing listener accepts it, the browser's Origin is `http://127.0.0.1:P`, and the host rewrites Host to `localhost:P`; CSRF checks that compare Origin to Host (Django) then fail for POSTs. Recommend rewriting `Origin` (and the origin part of `Referer`) to `http://localhost:P` whenever it equals one of the accepted forms, on all request types, not only upgrades.
- **Cookies, storage, service workers** are per-host not per-port for cookies and per-origin for the rest; the remote app's cookies for `localhost` are shared with every local dev server in the same browser profile. Known limitation of the same-port design; mention in the security review.

### 3. `httputil.ReverseProxy` across two hops

Verified in `scratchpad/exp2` (two `ReverseProxy` instances; host side an `httptest` TLS server with `EnableHTTP2=false`; viewing side a `Transport` with `DialContext` to the server's address and `TLSClientConfig.ServerName` set, the same shape as `peerHTTPClient`):
- Host saw `proto HTTP/1.1 tls true`, `Upgrade: websocket` on the WS request. [VERIFIED: exp2 output]
- WS echo through both hops with the dev server using `coder/websocket`'s default same-host Origin check passed once Origin was rewritten. [VERIFIED]
- SSE first chunk after 1 ms (no buffering). [VERIFIED]
- Revoke: cancelling the host request context ended the WS client read ("failed to read frame header: EOF") and the SSE body ("unexpected EOF") within ~1 ms across both hops. [VERIFIED] `httputil: ReverseProxy read error during body copy: unexpected EOF` is logged on the host: set `ErrorLog` to a filtered logger.
- HTTP/1.1 only: the share server's `tls.Config` sets no `NextProtos` (server.go:392-395 builds `&tls.Config{GetCertificate: getCert, MinVersion: tls.VersionTLS12}`), so no ALPN `h2`; Go's `Transport` with a custom `DialContext`/`TLSClientConfig` does not auto-enable HTTP/2. Do not set `ForceAttemptHTTP2`; add a test asserting `resp.ProtoMajor == 1` on the proxy hop, because WebSocket upgrades fail over h2.
- **Timeouts:** the share server has only `ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second` (server.go:401), no Read/WriteTimeout, so streams are not cut by the server. Viewing-side transport: `Dialer.Timeout` 10 s, `TLSHandshakeTimeout` 10 s, no `ResponseHeaderTimeout` (long polls), `DisableCompression: true` (be a faithful pipe: with it unset the Transport adds `Accept-Encoding: gzip` when the browser sent none and transparently decodes [CITED: net/http transport.go:199-207]), `IdleConnTimeout` below the server's 60 s (30 s) so a reused conn is not raced against the server's idle close [ASSUMED], `MaxIdleConnsPerHost` about 8 (browsers open 6 per origin; the default 2 churns TLS handshakes) [ASSUMED]. Host loopback transport: dial 3 s, no response timeout.
- **Client disconnect:** a browser tab closing cancels hop 1's outbound request, which closes the TLS conn, which cancels the host request context. Not exercised in exp2; make it a test with a goroutine-leak check.
- The viewing daemon's `ErrorHandler` must emit a fixed page and log `id8` and port only (the default handler logs the dial error text).

### 4. Token design

Existing facts [VERIFIED: files read this session]: `Claims` is `{SID, Perms, IAT, GrantID, V}` signed as `b64(payload).b64(hmac)`; `Verify` unmarshals into `Claims` and ignores unknown fields (capability.go:99-101); `verifyCapability` checks only token validity, `claims.SID` vs path `id`, `isGrantActive` and `IsSessionEnabled`, not `Perms` (capability_mw.go:78-104); read-only status is decided later by `readonly := !capability.HasPerm(claims.Perms, "write")` (server.go:821). A same-key token whose payload carries `sid` and `grant_id` therefore authenticates on `/api/sessions/{id}/info` and `/sessions/{id}/ws` as a read-only viewer. Hence:

```go
// internal/capability/porttoken.go (new). Verify() on the session key can never accept these.
type PortClaims struct {
    SID     string `json:"sid"`
    Port    int    `json:"port"`
    IAT     int64  `json:"iat"`
    GrantID string `json:"grant_id"`
    Kind    string `json:"kind"` // "port"
    V       int    `json:"v"`
}
func portKey(sessionKey []byte) ([]byte, error) { return hkdf.Key(sha256.New, sessionKey, nil, "portty/port-token/v1", 32) }
func SignPort(c PortClaims, sessionKey []byte) (string, error)   // json.Marshal + HMAC under portKey
func VerifyPort(tok string, sessionKey []byte) (PortClaims, error) // constant-time compare, Kind=="port", V==1
```

Verification order on the proxy route, all before any dial: same-user, then `X-Portty-Cap` (session cap: valid, `sid` equals path id, `HasPerm(...,"write")`, grant active, session enabled; refactor `verifyCapability` into `verifyCapabilityToken(token, pathID)` so the `?cap=` routes are unchanged), then `VerifyPort` (`Kind`, `sid` equal, `Port` equals the path port, `GrantID` equal to the cap's, grant active), then the deny list, then target parse. Sending both credentials keeps CONTEXT's "write capability on both routes" literal at no cost; if the user prefers one credential the port token alone is sufficient because it is only minted against a write cap and dies with the grant and the key. Add no expiry claim: lifetime is the grant, a leaked token is useless without same-user WhoIs and the cap.

**Deny list on the host (defence in depth, not in CONTEXT):** refuse port 0, the share server's own port (`tailnet.DefaultProbePort = 7444`, tailnet.go:29, or `ws.Addr()`), and the daemon relay port (`a.relayPort`, api.go:28). The relay accepts WebSocket Origins only for its own port (`loopbackOriginPatterns` appends `http://localhost:<port-of-Host>`, server.go:251-272) and the host rewrites Origin to `http://localhost:PORT`, so proxying to the relay port would defeat that guard. The webserver needs a `SetDeniedPorts(func(int) bool)` seam set by the daemon.

### 5. Printed-text check for URLs

`hasToken` uses the path alphabet (`isPathChar`, pathclick.go:~313-318) so `http://localhost:8000` followed by `/` is not a token (`/` is a path char, not a sentence-ending `.`), and `localhost:80` would match inside `localhost:8000` without boundaries. Add a URL matcher and generalise the two search passes to take a predicate (and the needle length for `fallbackPass`'s bound):

- Needles: `http://` + host + `:` + decimal port, host in `localhost`, `127.0.0.1`, `[::1]`, `0.0.0.0`, `[::]`. Port from a validated integer 1..65535, formatted with `strconv.Itoa` (no leading zeros).
- Before the needle: start of line, whitespace, or one of `( [ { < " ' ` , = ;`. Not a letter, digit, `.`, `-`, `_`, `/`, `:`.
- After the needle: end of line; or `/ ? # ) ] } > " ' , ;` or whitespace; or `.` followed by end or a non-alphanumeric. Refuse a digit, letter, `@`, `:`, `-`, `_`, `%`, `~`, `+` (so `:80` never matches `:8000`, and `localhost:8000@evil` never matches).
- Search over the same `pathSnapshot()` rows (ANSI-stripped history plus live screen, soft-wrap joins), so a Vite line with a bold port (`http://localhost:\x1b[1m5173\x1b[22m/`) matches after `ansi.Strip` in `finish`.
- Guard with a `portPrintedBusy` CAS like `pathClickBusy` (hub.go:103) so a write-cap holder cannot stall `emuMu` with repeated 10,000-line snapshots; a busy hub answers the same refusal.

Real banners that must match (from the tools' own output formats; the python one is verified here): python `Serving HTTP on :: port 19106 (http://[::]:19106/) ...`, Vite `Local:   http://localhost:5173/`, Next `- Local:        http://localhost:3000`, Django `Starting development server at http://127.0.0.1:8000/`, Puma `* Listening on http://127.0.0.1:3000` and `http://[::1]:3000`, uvicorn `running on http://127.0.0.1:8000 (Press CTRL+C to quit)`, webpack `Loopback: http://localhost:8080/, http://[::1]:8080/`. Only python was run; the rest are [ASSUMED] from memory and belong in the VM matrix as sample inputs for the Go table test.

### 6. What `python3 -m http.server 8000` prints on current macOS

Run here (Python 3.15.2, macOS 27.2.1, empty directory, port 19106, killed afterwards):
```
Serving HTTP on :: port 19106 (http://[::]:19106/) ...
```
`lsof` showed `TCP *:19106 (LISTEN)` on an IPv6 socket, and both `127.0.0.1` and `[::1]` answered 200 [VERIFIED: command output]. So the `[::]` form is the one SC1 will actually print. The VM's `/usr/bin/python3` version is unknown (no SSH key from this session); Python 3.8+ binds the dual-stack `::` by default, older prints `0.0.0.0` [ASSUMED], and both forms are in scope. The xterm link for that line is `http://[::]:19106/)`: `new URL()` gives hostname `[::]`, port `19106`, pathname `/)` [VERIFIED: node check with the project's `urlRegex`]. A line ending `http://127.0.0.1:8000.` yields no link at all today (`new URL` throws on port `8000.`); that is an existing limitation, outside this phase. The proxy branch must trim trailing `) ] } > , ; . ' "` (and unbalanced `)`) from the matched text before parsing, and must send the cleaned path/query to the browser.

### 7. Validation architecture

See the Validation Architecture section.

## Common Pitfalls

### Pitfall 1: Silent shadowing hides the user's real local server
**What goes wrong:** Forward binds `127.0.0.1:8000` over a user's own `*:8000` server (verified possible); the user's local page disappears from their browser until the forward ends, and the foreign server receives nothing.
**How to avoid:** dial-probe both loopbacks first (Pattern 3). Test with a foreign wildcard holder and a `::1`-only holder (Node `listen(port,'localhost')`).
**Warning signs:** `lsof -nP -iTCP:P -sTCP:LISTEN` shows our daemon on `127.0.0.1` and `::1` plus another PID on `*`.

### Pitfall 2: Node dev servers are IPv6-only on this Mac
**What goes wrong:** `listen(port,'localhost')` binds `{"address":"::1","family":"IPv6"}` and `127.0.0.1` gets `ECONNREFUSED` (Node v24.22.1) [VERIFIED]. A host that dials only 127.0.0.1 yields "Nothing is answering" for Vite/Next banners that say `http://localhost:PORT`. Vite's default `server.host` being `localhost` is [ASSUMED].
**How to avoid:** host Transport `DialContext` ignores the URL host and tries `[::1]:P` then `127.0.0.1:P` (macOS resolver order seen here). This refines CONTEXT's "127.0.0.1, or ::1 for the IPv6 forms"; it still dials only the host's own loopback. Confirm with the user (Open Question 1).

### Pitfall 3: ServeMux cleans paths and rejects method-less subtree patterns
See Anti-Patterns; verified in exp3.

### Pitfall 4: `Host` has no port when P is 80
Browsers send `Host: localhost` for `http://localhost:80`. A strict `localhost:P` compare refuses it. Accept the bare form only when P == 80.

### Pitfall 5: Per-request WhoIs cost on asset-heavy pages
`requireSameUser` makes two LocalAPI calls per request (`StatusWithoutPeers`, `WhoIs`) under a 3 s timeout (whois.go:88-100). A Vite page loads hundreds of modules; a tailscaled hiccup becomes a broken page (503). [ASSUMED: not measured]. If measurement shows it matters, add a positive-only verdict cache of at most ~5 s keyed by the remote IP inside the proxy route only; never cache a refusal's cause, never extend past the TTL.

### Pitfall 6: Trailing punctuation in the matched URL
`(http://[::]:19106/)` yields path `/)`. Trim before parsing or the browser opens a broken URL (see Finding 6).

### Pitfall 7: Hijacked conns outlive `Stop`
`srv.Close()` does not close hijacked conns (share.go comment on `unshareLocked`, handoff.go:402 comment). Revoke must cancel contexts (Pattern 2), including from `Stop()` itself.

### Pitfall 8: Default error handlers and logs leak
`ReverseProxy`'s default handler logs `http: proxy error: <dial error>` (addresses, not tokens, but the viewing side's error may include the host URL). Set `ErrorHandler` and `ErrorLog`; log id8, port, outcome code. `remote_caps.go` has a source-grep test that nothing secret reaches disk; extend it to the new files (forwards and tokens are memory-only).

### Pitfall 9: Old host looks like a refusal
An old host answers `POST` to the new path with **405 and `Allow: GET, HEAD`**, no `X-Portty` marker [VERIFIED: exp3 against an `GET /` plus `POST .../cap` mux]. Every new-host refusal must carry `tailnet.MarkerHeader` (`X-Portty`: `1`, tailnet.go:35-36). Viewing daemon: 405 or 404 without the marker means `host_outdated`; 403 with the marker means refused. This replaces an extra `/info` round trip (which would also put the cap in a URL query).

### Pitfall 10: Browser state collides across hosts and ports
Cookies for `localhost` are shared with any local dev server; a service worker or localStorage from an earlier local app on the same port can serve stale content. Document in the security review and VM notes; no code fix.

## Code Examples

### Viewing-side request rewrite
```go
// Source: derived from local experiment exp2 (viewProxy); names are proposals
viewProxy := &httputil.ReverseProxy{
    Transport: fwd.transport, // peerTransport(...) + DisableCompression, TLSHandshakeTimeout, MaxIdleConnsPerHost
    Rewrite: func(pr *httputil.ProxyRequest) {
        pr.SetURL(&url.URL{Scheme: "https", Host: fwd.fqdn})
        pr.Out.URL.Path, pr.Out.URL.RawPath, pr.Out.URL.RawQuery = "/api/peer/sessions/"+url.PathEscape(fwd.sid)+"/ports/"+strconv.Itoa(fwd.port), "", ""
        pr.Out.Header.Set("X-Portty-Target", pr.In.URL.RequestURI())
        pr.Out.Header.Set("X-Portty-Cap", fwd.currentCap()) // read from RemoteCapStore per request
        pr.Out.Header.Set("X-Portty-Port-Token", fwd.token)
        pr.Out.Host = ""
    },
    ModifyResponse: stripPorttyHeaders, // delete every X-Portty* response header
    ErrorHandler:   fwd.errorPage,      // fixed text, no reflected input
}
```

### Pure matcher with table test (host side)
```go
// hasURLPort reports whether line holds http://<loopback form>:<port> with URL boundaries.
func hasURLPort(line string, port int) bool { /* needles x boundaries as specified in Finding 5 */ }
```
Table cases to pin: `(http://[::]:8000/)` true; `http://localhost:8000` true; `http://localhost:80` in `http://localhost:8000/` false; `xhttp://localhost:8000` false; `http://localhost:8000.evil.com` false; `http://localhost:8000@evil.com/` false; `http://localhost:8000.` true; `http://localhost:8000:12` false; `https://localhost:8000` false; `http://LOCALHOST:8000` false (printed text must be canonical lower case; the click side lowercases via the URL parser, the host never case-folds).

### Daemon API shape (proposal)
`POST /peers/ports` on the unix socket (api.go registerRoutes, next to `a.mux.HandleFunc("POST /peers/connect", a.handlePeerConnect)`, api.go:206). Request `{sessionId, url, consent}`. Success `{status:"open", url:"http://localhost:P/path?q", host, port}` or `{status:"consent", host, port}`. Typed errors (stable strings): `daemon_outdated` (GUI checks `DaemonAPI(HealthInfo()) >= PortProxyAPI`, client.go:86-97, before calling), `no_cap`, `host_outdated`, `refused`, `unreachable`, `port_in_use`, `bad_url`. Wails binding `OpenRemotePort(sessionID, rawURL string, consent bool) RemotePortResult` mirrors `ConnectPeerSession` (app.go:1461-1474: never throws, fills `ErrorCode`). The binding returns the URL; the frontend calls the existing `openLink` (keeps the 95-06 "no current-tab navigation sink" grep test green).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `ReverseProxy.Director` | `ReverseProxy.Rewrite` | Go 1.20 | Inbound `X-Forwarded-*` stripped for you; use it |
| Hand-rolled HKDF | `crypto/hkdf` in stdlib | Go 1.24 | No `golang.org/x/crypto` import |
| Dev servers trusting any Host/Origin | Host allowlists, Origin checks, WS tokens | Vite 6.0.9/5.4.12/4.5.6 (CVE-2025-24010), Next 15.2.x and 16.1.7 | Same-port Host/Origin rewrite to `localhost:P` is what keeps them working |

**Deprecated/outdated:** `server.hmr.host/clientPort` guidance on the Vite page now points at `server.ws`; irrelevant because same-port needs no override.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Safari/Chrome try `::1` first for `localhost` | Finding 1 | Low: both loopbacks are bound, so either order works |
| A2 | Vite's default `server.host` is `localhost` and binds `::1` on Node 24 | Pitfall 2 | Medium: if Vite binds `0.0.0.0`/`::`, dialing `127.0.0.1` alone would have been fine; trying both is still correct |
| A3 | Vite/webpack WS handshakes pass with Origin and Host rewritten to `localhost:P`, no body rewrite | Finding 2 | Medium: HMR fails; needs the VM matrix row with a real Vite or webpack server |
| A4 | VM `/usr/bin/python3` is 3.8+ and prints the `[::]` form | Finding 6 | Low: both forms are accepted |
| A5 | Banners of Next, Django, Puma, uvicorn, webpack match the strings listed | Finding 5 | Low: only affects table-test fixtures |
| A6 | Per-request WhoIs is fast enough; a short positive cache may be needed | Pitfall 5 | Medium: slow or flaky pages |
| A7 | `IdleConnTimeout` 30 s and `MaxIdleConnsPerHost` 8 are right | Finding 3 | Low: tuning only |
| A8 | An in-flight cap (about 128 per session) on the host is enough as a DoS guard | Security Domain | Low |
| A9 | The VMs have node/npm and outbound internet (for a Vite or webpack row) | Environment Availability | Medium: else use a small Node `ws` or Go echo helper |
| A10 | `Sec-Fetch-Site: cross-site` refusal for non-navigations is an acceptable optional hardening | Security Domain | Low: optional, not in CONTEXT |

## Open Questions

1. **Dial order on the host (refines a locked line).**
   - Known: CONTEXT says "127.0.0.1, or ::1 for the IPv6 forms". Node 24 `listen(port,'localhost')` binds `::1` only here, so a Vite or Next banner `http://localhost:PORT` needs `::1`.
   - Unclear: whether the user accepts "try `::1` then `127.0.0.1` for every form" as within "own loopback only".
   - Recommendation: do it, record it in the security review, and put the evidence in the confirmation.
2. **Host-side port deny list beyond the Portty ports.** Recommend only {0, share port, relay port}. Ports below 1024 or well-known sensitive ports (2375 Docker, 9222 DevTools, 5432) are not denied; consent plus the printed check is the control, but a malicious file could print `http://localhost:9222`. Ask whether to add a hard-deny set or a stronger sheet warning. Not decided in CONTEXT.
3. **Exact route and header names** (`/api/peer/sessions/{id}/ports[/{port}]`, `X-Portty-*`) are one-way once API 5 ships; confirm in plan review.
4. **VM rig:** does the office VM have node/npm and internet for a real Vite/webpack row, and what python3 does it carry? No SSH key was available from this session.
5. **`https://localhost:PORT` and `*.localhost` in a remote tab** keep today's behaviour (CONTEXT), which opens this Mac's own localhost. Accepted by the decision; flag in the security review as a known gap.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build, tests | yes | 1.27.4 (Phase 12 gate used `GOTOOLCHAIN=go1.26.11`) | — |
| Node | frontend tests, Node-based experiments | yes | v24.22.1 | — |
| pnpm / vitest | frontend tests | yes | `node_modules/.bin/vitest` present | — |
| Python 3 | SC1 sample server | yes (host) | 3.15.2 | — |
| lsof, curl | SC2 `lsof` check, curl probes | yes | system | — |
| tart + VMs `portty-test` (office), `portty-test-2` (home) | VM rows | yes, both running | macOS 27.2 per Phase 12 notes | — |
| SSH access to the VMs | VM rows | no key in this session (`Permission denied (publickey)`) | — | executor uses the Phase 12 rig (`~/p907`, `~/p12`) |
| Node/npm and internet inside the VMs | Vite/webpack row | unknown | — | tiny WS echo helper or `python http.server` plus a Node `ws` script |

**Missing dependencies with no fallback:** none found.
**Missing dependencies with fallback:** VM tool inventory unknown (see Open Question 4).

## Validation Architecture

`workflow.nyquist_validation` is `false` in `.planning/config.json` [VERIFIED: config read this session], so this section is not required; it is included because the invocation asked for the tests and VM rows for SC1-4.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (race, short) + vitest 4.1 + Playwright (existing, not touched) |
| Config file | go.mod; `frontend/vitest` config in repo |
| Quick run command | `go test -race -short -count=1 ./internal/capability ./internal/relay ./internal/webserver ./internal/daemon` and `cd frontend && pnpm vitest run src/lib/__tests__/localhostUrl.test.ts` |
| Full suite command | the Phase 12 gate: `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./... && go test -race -short -count=1 ./...`, `pnpm install --frozen-lockfile && pnpm vitest run`, `tsc --noEmit`, `tests/check-brand.sh`, `tests/check-share-removal.sh`, `bash tests/check-traceability-paths.sh`, `tests/build-script.test.sh` [VERIFIED: 12-VM-CHECKS.md gate paragraph] |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| URL-02 | Port token is not accepted by `capability.Verify` and a session cap is not accepted by `VerifyPort`; tamper, wrong kind, wrong port, fuzz | unit + fuzz | `go test ./internal/capability -run Port` | no, Wave 0: `internal/capability/porttoken_test.go` |
| URL-02 | `hasURLPort` boundary table; `Hub.PortPrinted` over history, live screen, soft wrap, ANSI-bold port, busy refusal | unit | `go test ./internal/relay -run PortPrinted` | no, Wave 0: `internal/relay/portprinted_test.go` |
| URL-02 | Grant route: other user (WhoIs stub) 403 + marker; read cap 403; no/other-session cap 403; not printed 403; printed 200; refusals byte-identical for unprinted, printed-but-closed and printed-and-open ports; **tripwire loopback listener sees 0 accepts on every refusal** | integration | `go test ./internal/webserver -run PortGrant` | no, Wave 0: `internal/webserver/portproxy_test.go` |
| URL-02/01 | Proxy route: missing token, wrong port, port token of another session, revoked grant, session disabled, denied ports (share, relay, 0): all refused with 0 accepts; success proxies path/query byte-exact (`//`, `%2e%2e`, `%2F`), Host and Origin rewritten, `X-Portty-*` and `X-Forwarded-For` absent at the dev server; WS echo, SSE first chunk, `ProtoMajor == 1`; `::1`-only and `127.0.0.1`-only upstreams both reachable | integration | `go test ./internal/webserver -run PortProxy` | no, Wave 0 (same file) |
| URL-02 | `ClearGrants`, session exit and `Stop()` each close an open WS and an open SSE within a bound; client disconnect cancels two hops; goroutine count returns to baseline | integration | `go test ./internal/webserver -run PortProxyRevoke` | no, Wave 0 |
| URL-01/02 | Viewing daemon: forward listens only on 127.0.0.1 and `[::1]` (inspect `net.Listener.Addr`), Host check table (including P == 80), foreign `*:P` holder and `::1`-only holder give `port_in_use` and are left untouched, approvals per (Mac, port), consent flow (`consent` then `open`), error page text and headers, forward closes on upstream 403/ECONNREFUSED and drops the approval, token never in a URL, nothing on disk (extend remote_caps source-grep test) | integration | `go test ./internal/daemon -run PortForward` | no, Wave 0: `internal/daemon/portforward_test.go` |
| URL-01 | Version gate: `APIVersion == 5`, `PortProxyAPI == 5`, 405-without-marker maps to `host_outdated`, old local daemon maps to `daemon_outdated` | unit | `go test ./internal/daemon -run 'TestHealthInfo|TestAPIHealth|PortGate'` | edit in place: client_test.go:47, api_test.go:171 |
| URL-02 | Consent sheet: copy, focus on Cancel, Esc cancels, Don't Allow opens nothing | vitest | `pnpm vitest run RemotePortConsent` | no, Wave 0 |
| URL-01/03 | `localhostUrl.ts`: five hosts accepted, userinfo rejected, `127.1`, `localhost.` and `https` not matched, tail trimming, shared fixture with the Go parser | vitest + Go | `pnpm vitest run localhostUrl` and `go test ./internal/daemon -run LocalhostURL` | no, Wave 0 plus a shared fixture JSON (Phase 12 pattern) |
| URL-03 | Click handler: remote desktop peer tab with loopback form routes to the binding; local tab, browser share view (`wsURL`), non-loopback and `https://localhost` still reach `getRisk`/`openLink`; failure paths never call `openLink` with a local URL | vitest | `pnpm vitest run TerminalPanel` | edit in place + new |

**Link tests to update deliberately (SC4):** `frontend/src/components/__tests__/TerminalPanel.web-links.test.tsx` (source-gate greps of the handler; the Plan B test pins that `lib/pathLinks.ts` is the only non-test file registering a link provider, so register nothing new), `frontend/src/lib/__tests__/openLink.test.ts`, `frontend/src/components/__tests__/App.webLink.test.tsx`, `frontend/src/components/__tests__/WebShareSessionView.peerLinks.test.tsx`, `frontend/src/components/__tests__/LinkConfirmPopover.test.tsx`, `frontend/src/lib/__tests__/urlSafety.test.ts`, `internal/webserver/web_links_test.go`. Do not weaken assertions; add the new branch's expectations beside them.

### VM rows (new Category BD in TESTING.md, next free ids M-137 onward; highest existing is M-136)
Rig as Phase 12 (office `portty-test`, home `portty-test-2`, never stop or log out the ephemeral nodes; real CGEvent clicks; Safari via WebDriver).
- **M-137 (SC1):** office runs `python3 -m http.server 8000` in a directory with `index.html`, an asset and a subfolder. Confirm and record the exact banner it prints. Home peer tab Cmd-click on the printed URL, Allow, default browser opens `http://localhost:8000/`; page, subfolder, query string and asset load; the `http://127.0.0.1:8000/` form also works (type it with `echo`). Second click on the same port shows no sheet.
- **M-138 (SC2):** sheet names the office Mac and the port; Don't Allow and Esc open nothing and leave no listener; another port asks again. On home, `lsof -nP -iTCP:8000 -sTCP:LISTEN` lists only `127.0.0.1:8000` and `[::1]:8000`, no `*`.
- **M-139 (SC3):** from the home VM, `curl` the office share route with no token, a wrong token, a token for another port, no cap; all 403, and an office-side listener counter shows no connection; a printed-never port is refused with the same notice as a closed one; from the Tart host (not on the tailnet) the share port is unreachable. Stop sharing on the office: an open page, an open WebSocket and an SSE stream stop within seconds, the home listener is gone, the next click asks again.
- **M-140 (dev servers):** a WebSocket and SSE page through the proxy (Vite or webpack-dev-server with an edit triggering HMR if node and internet exist, else a minimal Node `ws` echo plus an SSE endpoint); a `::1`-only Node server (`listen(port,'localhost')`) works.
- **M-141 (port taken):** home runs a foreign server on `*:8000` (python) and, separately, on `::1` only (Node); a click gives "Port 8000 is in use on this Mac", the foreign server still answers `curl`, nothing was bound by Portty.
- **M-142 (gates):** office on build A (no Phase 13) and home on the new build gives "needs a Portty update"; old home daemon with new GUI shows update needed (the M-104 two-build rig).
- **M-143 (SC4):** a local tab and a remote tab with `https://example.com` and an IDN lookalike keep the confirm popover then browser; `http://localhost:8000` in a local tab opens this Mac's own server as today.

### Sampling Rate
- **Per task commit:** the quick Go command for the touched package, or the single vitest file.
- **Per wave merge:** the two quick commands above across all four Go packages and `pnpm vitest run`.
- **Phase gate:** full suite green, `check-traceability-paths.sh` clean, then the VM rows, before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/capability/porttoken_test.go` and a fuzz case beside `capability_fuzz_test.go`
- [ ] `internal/relay/portprinted_test.go`
- [ ] `internal/webserver/portproxy_test.go` (tripwire listener helper, two-hop WS and SSE helpers; mirror `SetWhoIsClientForTest`)
- [ ] `internal/daemon/portforward_test.go` (uses `SetPeerClientForTest` and `tailnet.LiveAllowed` seams; tests never touch the tailnet)
- [ ] `frontend/src/lib/__tests__/localhostUrl.test.ts` and `fixtures/localhost-url.json`; consent sheet test; TerminalPanel branch test
- [ ] TESTING.md: Section 2 manifest groups, Section 4 rows for URL-01..03 (repo-relative `.go`/`.ts`/`.tsx` paths only), Section 5 Category BD (M-137..M-143), the header count note; run `bash tests/check-traceability-paths.sh` (CLAUDE.md standing rule)

## Security Domain

`security_enforcement` is not set to false in config, so this section applies. The Phase 12 security review (`12-SECURITY-REVIEW.md`: scope, why the contract holds, honest scope, accepted risks, threat register, tests that pin it, VM evidence) is the template for `13-SECURITY-REVIEW.md`; SC3 requires it.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Tailscale WhoIs same-user (`requireSameUser`), session capability HMAC |
| V3 Session Management | yes | Port token bound to the share grant; revoked by `ClearGrants`, key rotation (HKDF) and `Stop` |
| V4 Access Control | yes | Per-port consent, printed-text grant, exact Host check, deny list, check order before any dial |
| V5 Input Validation | yes | Strict URL parse and canonical-host compare, port range, `X-Portty-Target` parse, header size caps |
| V6 Cryptography | yes | HMAC-SHA256 via existing `capability`, `crypto/hkdf`, `hmac.Equal`; never hand-roll |
| V9 Communications | yes | HTTPS to peers with cert pinned to the MagicDNS name against tailscaled's IP |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Port scanning through the host proxy | Information disclosure | Printed-text grant, identical refusals, no dial before all checks (tripwire test) |
| Token confusion (port token as session cap) | Elevation of privilege | HKDF-derived key plus distinct claims and `Kind`; `Verify` fails on the session key |
| Share link leaked to another tailnet user | Spoofing | `requireSameUser` on both routes, fail closed |
| DNS rebinding to the viewing listener | Spoofing | Exact Host allowlist (`localhost:P`, `127.0.0.1:P`), loopback bind only |
| Other website CSRFs the forward (no-cors request to `localhost:P`) | Tampering | Residual, same as any local dev server; optional: refuse `Sec-Fetch-Site: cross-site` unless `Sec-Fetch-Mode: navigate` (OAuth callbacks are cross-site navigations) [A10] |
| Port shadowing hides or hijacks a local server | Tampering | Probe both loopbacks, refuse unless connection-refused |
| Proxying to Portty's own ports (relay accepts `http://localhost:<its port>` Origins) | Elevation of privilege | Host deny list {0, share port, relay port}; viewing probe refuses its own relay port |
| Attacker-printed URL for a sensitive port (9222, 2375) social-engineers approval | Elevation of privilege | Consent names the port; open decision on a hard-deny set (Open Question 2) |
| Hijacked WS/SSE outlives stop-share | Elevation of privilege | Context cancel on `revokeSignal` and `stopCh`; tests on every `Stop` path |
| Header injection and smuggling (`X-Forwarded-*`, `X-Portty-*`, hop-by-hop) | Tampering | `Rewrite`, `Set`/`Del` of every Portty header on each hop, `ReverseProxy` hop-by-hop stripping |
| Host-side resource exhaustion by a same-user write-cap holder | Denial of service | `portPrintedBusy` CAS, an in-flight cap per session (about 128 [A8]), forwards per daemon cap |
| Token or target in logs (Vite `?token=`) | Information disclosure | Log id8, port, outcome only; fixed error pages; source-grep test |
| Reflected content in the daemon's error page | Tampering | Static copy plus escaped host label and numeric port; `Content-Security-Policy: default-src 'none'`, `nosniff`, `no-store` |

**CSP and tailnet-only constraints (project CLAUDE.md):** do not wrap the proxy route in `cspHeaders`, and do not touch the strict policy of `/dashboard`, `/sessions/{id}` or `/app/`; existing `csp_mw_test.go` and the browser CSP e2e tests must stay green. The proxy route lives on the tailnet-bound share listener only; the only new loopback listeners are the forwards.

## Project Constraints (from CLAUDE.md)

- MIT licence and root `LICENSE` stay; no new third-party code is added.
- macOS only; keep the inherited Go/Wails/React/tsnet stack; remote access only over the tailnet; no relay servers or cloud dependencies.
- Keep token-gated, tailnet-only share links and the CSP intact through the change; remote write access to a shell is powerful.
- Chesterton's Fence: read each piece before removing; no removal is needed here. `verifyCapability` is refactored, not removed; keep its `?cap=` behaviour and tests.
- TESTING.md is the regression home: add suites to Section 2, requirement rows to Section 4 (repo-relative paths only, then `bash tests/check-traceability-paths.sh`), manual items to Section 5, and re-apply branch protection only if `build.yml` matrix entries change.
- Python env rule (global CLAUDE.md): any Python helper runs from a venv; this phase needs none beyond `python3 -m http.server` as a sample server inside the VMs.
- Go: `go fmt`, `golangci-lint`, `ctx context.Context` in functions that block; TS: ESLint and Prettier; 80%+ coverage in critical components; integration tests for routes.
- Silent fallbacks are forbidden: refuse and say so ("never pick another port silently", fixed refusals, `ErrorCode` instead of `or {}`).
- "Never drive GUI on the host Mac": all clicks and app launches happen in the Tart VMs; research ran only Go and Node socket experiments and a short `python3 -m http.server` in an empty scratch directory on the host.
- GSD workflow: this file is the only repo change from research.

## Sources

### Primary (HIGH confidence)
- Local experiments this session (scratchpad `exp1` sockets, `exp2` two-hop proxy, `exp3` ServeMux): macOS 27.2.1, Go 1.27.4, Node v24.22.1, Python 3.15.2.
- Repo files read this session: `internal/daemon/{remote_ws_proxy,peer_client,remote_caps,peer_connect,api,share,types,client,handoff}.go`, `internal/webserver/{server,capability_mw,whois,origin_mw}.go`, `internal/capability/capability.go`, `internal/relay/{pathclick,server}.go`, `internal/tailnet/{tailnet,sessions}.go`, `app.go`, `frontend/src/components/{TerminalPanel,RegenerateKeyModal}.tsx`, `frontend/src/lib/openLink.ts`, `frontend/src/App.tsx`, `TESTING.md`, `.planning/{REQUIREMENTS,ROADMAP,STATE,config.json}`, `12-VM-CHECKS.md`.
- Go standard library source (GOROOT): `net/http/httputil/reverseproxy.go`, `crypto/hkdf/hkdf.go`, `net/http/transport.go`.

### Secondary (MEDIUM confidence)
- https://vite.dev/config/server-options (allowedHosts, cors defaults)
- https://webpack.js.org/configuration/dev-server/ (allowedHosts `auto`, `client.webSocketURL`)
- https://nextjs.org/docs/app/api-reference/config/next-config-js/allowedDevOrigins
- https://codereview.chromium.org/1177933002 (Chromium resolves localhost internally to IPv4 and IPv6)

### Tertiary (LOW confidence)
- Search-result summaries of CVE-2025-24010 (NVD page fetch returned no body), CVE-2026-27977, and forum reports on Safari and `::1`. Used only to name versions; mechanisms not verified.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, stdlib only, behaviour confirmed by experiment.
- Architecture: HIGH for the proxy chain, token separation and loopback handling; MEDIUM for UI branching details (not prototyped).
- Pitfalls: HIGH for the five that were reproduced (shadowing, `::1`-only Node, ServeMux 307, mux conflict panic, 405 old host); MEDIUM for dev-server and browser behaviours.

**Research date:** 2026-11-22
**Valid until:** 2026-12-22 (stable stdlib and repo seams); re-check dev-server behaviour if Vite, webpack-dev-server or Next majors change.
