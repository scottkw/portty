# Phase 13: Remote Localhost URLs - Context

**Gathered:** 2026-11-22
**Status:** Ready for planning

<domain>
## Phase Boundary

In a desktop peer tab (this Mac viewing another Mac's session), Cmd-clicking a `localhost` URL that the remote session printed opens that page from the remote Mac in this Mac's normal browser. The page is proxied over the tailnet by Portty, WebSockets included so dev-server hot reload works (URL-01). Each (Mac, port) needs consent. The proxy is token-gated, tailnet-only and same-user, binds only on the viewing Mac's loopback, and cannot be used to scan ports (URL-02). Local URLs, and non-localhost URLs in remote tabs, keep today's behaviour (URL-03). There is no in-app browser.

</domain>

<decisions>
## Implementation Decisions

### Proxy wire contract
- Per-request HTTP reverse proxy on both hops: browser → the viewing daemon's listener on 127.0.0.1:PORT → HTTPS to a new share-server route keyed by session and port → the host proxies to its own loopback port. `httputil.ReverseProxy` handles `Upgrade: websocket`. The port token travels in a request header, never in a URL. — **Reversibility:** one-way — cross-Mac route shape tied to API version 5; both daemons must agree.
- The browser uses the **same port number** as on the remote Mac (http://localhost:8000 stays http://localhost:8000), so absolute URLs, redirects, OAuth callbacks and dev-server Host/Origin checks need no rewriting. Research must settle macOS SO_REUSEADDR shadowing (127.0.0.1:P bindable while *:P is held) and whether browsers resolve `localhost` to ::1 first. If the port is genuinely taken on this Mac, refuse with a notice, never pick another port silently. — **Reversibility:** costly — URL shape and origin the user's browser and dev servers see.

### Security contract
- The host mints a **port-scoped signed token** on request (HMAC; claims: session, port, grant tied to the share), so stopping the share kills it. The host refuses any proxy request whose token doesn't name that port, before dialling anything. The port token must NOT verify on any existing session route (today `verifyCapability` accepts any verified claims for the session, capability_mw.go:93-104, and `Verify` ignores unknown fields, capability.go:99-101). Use a distinct token kind or audience so it can't be confused with a session capability. — **Reversibility:** one-way — auth model and cross-Mac token format.
- **Anti-scan printed check:** the host grants a port only if `http://localhost:PORT` (or one of the accepted host forms below) was printed in that session's retained output, reusing the Phase 12 printed-text machinery (relay/pathclick.go `pathSnapshot`/`findPrinted`) with URL token boundaries. Every refusal looks identical whatever the port's state, and no connection is made before all checks pass. — **Reversibility:** one-way — security contract, same as Phase 12's.
- **Who:** `requireSameUser` (WhoIs) plus a valid write capability, on both the grant route and the proxy route. A share link leaked to another tailnet user can't reach ports. Consistent with Phase 14 MOB-05. — **Reversibility:** costly — auth model.
- **Viewing-side listener:** binds 127.0.0.1 only (lsof check in the VM run). Accept only `Host: localhost:P` or `127.0.0.1:P` on every request, which blocks DNS rebinding. No token in browser URLs; port tokens never leave the viewing daemon. URL-02's "token-gated" applies to the tailnet hop. — **Reversibility:** costly — auth model.

### Consent (Claude's discretion, user said "You decide")
- Surface: the existing in-window sheet style (`.dialog`, RegenerateKeyModal pattern: copy overrides, focus on Cancel, Esc = cancel). Follows "one macOS sheet style for every modal" and is testable in vitest.
- Copy: title "Allow access to {host}'s localhost:{port}?"; body "Pages from this port open in your browser through Portty over your tailnet. Other ports stay closed."; buttons "Don't Allow" / "Allow". Declining opens nothing.
- Lifetime: memory only, per (Mac, port). Ends when that Mac stops sharing, the session's grant is revoked, or this Mac's daemon restarts. Nothing on disk (Phase 4 rule).
- Revocation: implicit only (stop sharing, session end, daemon restart). No indicator or settings list this phase (radical simplicity); revisit if the user asks.

### Scope and failures (Claude's discretion)
- URLs: `http://localhost:PORT`, `http://127.0.0.1:PORT`, plus `http://[::1]:PORT`, `http://0.0.0.0:PORT` and `http://[::]:PORT`, which servers like `python3 -m http.server` print. The host always dials its own loopback only: ::1 first, then 127.0.0.1, for every accepted form (Node binds `localhost` to ::1 only; research 13-RESEARCH.md), never another address. The host hard-denies port 0, its share port and its relay port (the relay's Origin allowlist would otherwise be bypassed by the rewritten Origin). The browser always opens `http://localhost:PORT`. Desktop peer tabs only; the browser share view and https://localhost keep today's behaviour.
- Failures: before opening, a transient notice in the clicking pane (Phase 12 notice pattern; e.g. "{host} needs a Portty update to open its local pages", "Couldn't reach {host}", "Port {port} is in use on this Mac"). After opening, a plain error page served by the viewing daemon in the browser (e.g. "Nothing is answering on {host}'s port {port}", "{host} stopped sharing"). Never fall back to opening this Mac's own localhost.

### Post-research refinements (Claude's discretion, 2026-11-22)
- Host route carries the original request target in a header (`X-Portty-Target`) on a fixed route `/api/peer/sessions/{id}/ports/{port}`, because ServeMux rewrites `//` and `..` paths and a subtree pattern conflicts with `GET /`. Exact names may be adjusted by the planner before API 5 ships.
- Port tokens use a separate key derived with stdlib `crypto/hkdf` and a distinct claims kind, so they can never authenticate on session routes; regenerating the share key kills them.
- Port-taken detection on this Mac: dial-probe both loopbacks and require connection-refused (IPv6 absent tolerated), then bind 127.0.0.1:P and [::1]:P. Fail closed with the "port in use" notice.
- No extra hard-deny list for well-known sensitive ports (9222, 2375, 5432…): the printed check, same-user, write access and per-port consent are the controls; record it as a residual risk in the security review.
- Every new-host refusal carries the `X-Portty` marker so an old host's plain 405 maps to "needs a Portty update".
- The click branch trims trailing punctuation from the matched URL (xterm's regex keeps a trailing `)`).

### Decided autonomously
- [Proxy] Q: "Where does the viewing-side listener live?" → Selected: "The viewing Mac's daemon (RemoteCapStore, the verified peer client and the remote_ws_proxy template already live there)" — internal to one Mac, no cross-Mac contract.
- [Proxy] Q: "WebSockets, streaming and stopping the share" → Selected: "Track proxied upgrades and long responses per session; close them on ClearGrants/revokeSignal because hijacked conns survive Stop; the upstream client uses a dial timeout only, not peerClient's 10 s total timeout" — internal plumbing.
- [Proxy] Q: "Version gating and upgrades" → Selected: "APIVersion 5 with PortProxyAPI on both daemons; the viewing daemon learns host support at click time and returns a typed error; an old local daemon shows update needed; forwards end on a Phase 9 handoff and the next click reopens them" — additive gate.
- [URL] Q: "Rewriting Host, Origin and bodies" → Selected: "Host sets Host (and Origin on upgrades) to localhost:PORT; no body rewriting; cookies untouched; Location rewritten only if the local port ever differs" — host-side header logic.

### Claude's Discretion
Consent surface, copy, approval lifetime, revocation visibility, URL scope and failure handling, as recorded above. The planner may adjust details within these choices.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- internal/daemon/remote_ws_proxy.go:47-184: template for a daemon-side proxy. Takes the capability from RemoteCapStore, dials through the verified peer client, sets Origin exactly, runs on the long-lived context, keeps the token out of logs and out of the webview's URL.
- internal/daemon/remote_caps.go:61-98: session id → (baseURL, peerIP, capability); filled by POST /peers/connect (peer_connect.go:199) and refreshed on every peer-tab redial (App.tsx:1007-1015).
- internal/daemon/peer_client.go:24-57: dials the tailscaled-reported IP and checks the cert against the MagicDNS name. Its 10 s total timeout (line 27) must not be reused for proxied traffic.
- internal/webserver: `requireSameUser` (whois.go:88-116, fails closed), `verifyCapability` (capability_mw.go:77-106), `requireAllowedOrigin` (origin_mw.go:31-55), `ClearGrants`/`revokeSignal` (server.go:243-266), setupRoutes (server.go:491-648). The server speaks HTTP/1.1 only, so hijack upgrades work.
- internal/capability/capability.go: Sign/Verify, HMAC claims {sid, perms, iat, grant_id, v}.
- internal/relay/pathclick.go:192-253, 315-328: printed-text snapshot and token search to reuse with URL boundaries.
- Frontend: TerminalPanel.tsx:869-906 click handler (urlRegex at 918 already matches localhost URLs; `remote` prop at 97; `hostLabel` at 128); openLink.ts:48-65; RegenerateKeyModal.tsx (sheet); Phase 12 transient notice (TerminalPanel.tsx:79, 339-352).
- App bindings: app.go:1461-1474 `ConnectPeerSession` (typed errorCode, never throws); `runtime.BrowserOpenURL` at app.go:1391.

### Established Patterns
- Feature gate = APIVersion bump + named constant (types.go:60-73); the GUI checks its own daemon with MinDaemonAPI/DaemonAPI (client.go:86-97). No viewing-side code reads a host's `api` yet: Phase 13 is the first cross-Mac api gate.
- A desktop peer tab never calls the host from the webview (no CORS); all host traffic goes through the viewing daemon.
- Nothing secret on disk (remote_caps.go source-grep test).
- Tests never touch the tailnet: `tailnet.LiveAllowed`, `SetPeerClientForTest`, `SetWhoIsClientForTest`.
- Revoke order: ClearGrants → DisableSession → RevokeWebViewers → Stop (share.go:342-357).

### Integration Points
- A new branch in the TerminalPanel click handler (remote tab + localhost-form host) → a new Wails binding → a new daemon unix-socket route (api.go:173-208) → the loopback listener → the host.
- A new host route in setupRoutes, outside `cspHeaders` and outside `/app/` and `/sessions/{id}`, plus a close-on-revoke in unshareLocked/ClearGrants.
- Handoff (handoff.go:190-196) carries only shares and listeners; forwards are deliberately not carried.
- Link tests that pin the click path must be updated deliberately: TerminalPanel.web-links.test.tsx, openLink.test.ts, App.webLink.test.tsx, WebShareSessionView.peerLinks.test.tsx, LinkConfirmPopover.test.tsx, urlSafety.test.ts, internal/webserver/web_links_test.go.

</code_context>

<specifics>
## Specific Ideas

- SC1's `python3 -m http.server 8000` may print `http://[::]:8000/` or `0.0.0.0` rather than localhost. That is why those forms are in scope; the VM check should confirm what it prints.
- Research topics: macOS SO_REUSEADDR and localhost-to-::1 resolution in Safari/Chrome; Vite (allowedHosts, HMR token and Origin), webpack-dev-server and Next.js dev behind a same-port proxy; `httputil.ReverseProxy` upgrades across two hops with a custom tailnet Transport; SSE flushing.
- SC4: "confirm popover, then default browser" means unchanged behaviour (the popover appears only for risky links today).
- With two sessions shared from one Mac, a forward is tied to the session whose output printed the URL; stopping that share ends it.

</specifics>

<deferred>
## Deferred Ideas

- An open-ports indicator or a Settings list for revoking ports (chose implicit revocation for now).
- Persisting approvals across restarts.
- Proxying for the browser share view (same-origin hazard with /app/).
- Carrying forwards across a Phase 9 handoff.
- Pending from Phase 12: WR-01 ("last occurrence" path resolution) accept vs refuse-ambiguous, and Option-click `altClickMovesCursor` keep vs turn off: both await the user.

</deferred>
