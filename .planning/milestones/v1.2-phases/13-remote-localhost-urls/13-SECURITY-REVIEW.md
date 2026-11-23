# Phase 13 Security Review: Remote Localhost URLs

## Scope and parts

Scope: what Phase 13 (URL-01, URL-02) changes at the security boundary. In a desktop peer tab, a Cmd-click on a `localhost` URL the remote session printed opens that page from the remote Mac in this Mac's own browser. Portty proxies it over the tailnet, WebSocket upgrades included. That is the first time one Mac can reach another Mac's loopback ports through a share, so the contract below is new, not an amendment.

This file is written in two parts:

- **13-06 (this part):** the contract, the check order on both Macs, why it holds, the honest scope, the accepted risks, what stays unchanged, version behaviour, the consent and notice behaviour, the threat register and the tests that pin each point.
- **13-07 (appended):** `## VM evidence and verdict`, from the two-VM run over a real tailnet and the `lsof`, Vite, `python3 -m http.server` and stop-sharing checks.

No share link, capability, port token or tailnet host appears in this file. Hosts are written as `<host>`. Where a page's own query parameters matter, they are named in words (a `cap` query parameter, Vite's HMR token).

## What Phase 13 adds at the boundary

- **On the host Mac (the one sharing):** two routes on the share server, both behind the existing same-user check. A grant route mints a port token for a port the session printed. A proxy route reverse-proxies one request, or one WebSocket upgrade, to this Mac's own loopback. The host never dials anything else.
- **On the viewing Mac:** one route on the daemon's owner-only unix socket (`POST /peers/ports`) and a loopback forward per approved (Mac, port): a listener on `127.0.0.1:<P>` and `[::1]:<P>` whose requests the daemon carries to the host's proxy route. A new Wails binding, a consent sheet and five notices in the clicking pane.
- **In between:** a port token. It is signed under a key derived from the share's signing key (HKDF, a distinct info string) and carries its own claims kind, so no session route accepts it and no port route accepts a session capability.
- **Versions:** `APIVersion` 5 and `PortProxyAPI` 5. `MinDaemonAPI` stays 1.

## Refinements of CONTEXT decisions

Four places where what was built is narrower or different from what CONTEXT first said, and why.

1. **The viewing listener binds both loopbacks, `127.0.0.1` and `[::1]`.**
   - CONTEXT said: bind this Mac's loopback only, `127.0.0.1`.
   - Built: both `127.0.0.1:<P>` and `[::1]:<P>`, with explicit `tcp4` and `tcp6` binds (13-02). 13-CONTEXT now records this (updated after commit 5ffe4fa8; RESEARCH Finding 1).
   - Why: a browser resolves `localhost` to `[::1]` first on some setups. If only `127.0.0.1` were bound, a program holding `[::1]:<P>` on this Mac would answer the browser's `localhost` request instead of the forward, and the user would see another program's page. With both bound, a `localhost` request can never reach another program. Both are loopback, so the "loopback only" rule holds. Before binding, the daemon dial-probes both and refuses unless both refuse the connection (a foreign wildcard or `[::1]`-only holder gives `port_in_use`); an absent IPv6 loopback is tolerated.
2. **The host rewrites Origin only from exactly `http://127.0.0.1:<P>` to `http://localhost:<P>`, never on all upgrades.**
   - CONTEXT said: rewrite Origin so dev-server checks pass.
   - Built: the rewrite fires only when the Origin is the typed `http://127.0.0.1:<P>` form of this very port (13-03). Every other Origin, a cross-site page's included, reaches the dev server unchanged.
   - Why: the browser opens `http://localhost:<P>`, so its Origin already matches the Host the host sends (`localhost:<P>`). Only a user who typed `127.0.0.1` needs the fix. A blanket rewrite would turn a cross-site page's WebSocket into a same-origin one at the dev server's own check (Origin laundering, T-13-20).
3. **The forward refuses cross-site requests that are not navigations (13-02).**
   - CONTEXT did not require this (RESEARCH A10 called it optional hardening).
   - Built: a request with `Sec-Fetch-Site: cross-site` is refused unless its `Sec-Fetch-Mode` is `navigate`.
   - Why: once the Host and Origin look local, any website could drive the dev server through the forward with no-cors requests or a WebSocket. A cross-site navigation, such as an OAuth callback, still passes.
4. **Approvals and forwards end on the session stream's terminal close or a changed capability, with no request needed; `open` is answered only with a live approval (13-02, checker B1).**
   - CONTEXT said approvals end when the Mac stops sharing.
   - Built: a peer tab's session stream closed by the host with 1000 `share stopped` or `session ended` makes `handleRemoteSessionWS` see `relay.IsTerminalClose` on the peer side and call `RevokeSession`, which closes the Mac's forwards and drops their approvals. A different capability at `POST /peers/connect` does the same. `handlePortOpen` answers `open` only for an approval recorded after its forward was installed and tied to the SHA-256 of the capability it was given under.
   - Why: a request-time check alone leaves a forward listening, and approved, after the share ends, until something happens to hit it. The stream close removes that wait. A 1012 close or a dropped network is not terminal: the tab reconnects and the forward stays.

## The contract

The host dials its own loopback port for a viewing Mac only when every row holds, and it checks them all before the first dial.

| Condition | Enforced in | Pinned by |
|-----------|-------------|-----------|
| Same Tailscale user as the host | `requireSameUser` (WhoIs, fails closed) wrapping both routes in `setupRoutes` (`internal/webserver/server.go`) | `TestPortGrant_Refusals` and `TestPortProxy_Refusals` ("another user" rows) |
| A write capability with an active grant, on a session that is shared | `portCapability` in `internal/webserver/portproxy.go`, over `verifyCapabilityToken` (signature, session, grant active, session enabled) plus the `write` permission | `TestPortGrant_Refusals`, `TestPortProxy_Refusals` (read-only, other session, revoked grant, session disabled rows) |
| The port was printed by that session, as an `http` loopback URL, as a whole token | `Hub.PortPrinted` over the Phase 12 printed-text snapshot with URL token boundaries (`internal/relay/portprinted.go`), called by `handlePortGrant` | `TestHasURLPort_Table`, `TestPortPrinted_*`, `TestPortGrant_Refusals` ("port never printed" row), `TestPortGrant_IdenticalOpenOrClosed` |
| A port token naming that session, that port and that grant, under the derived key | `capability.VerifyPort` and the SID, port and GrantID comparisons in `handlePortProxy` | `TestPortToken_NotASessionCapability`, `TestPortToken_SessionCapNotAPortToken`, `TestPortToken_Tamper`, `TestPortToken_KeyRotation`, `TestPortProxy_Refusals` (wrong port, other session, other grant rows), `TestPortProxyRevoke_KeyRegeneration` |
| Not a Portty port | `portDenied`: 0, the share server's own listener port, 7444, and the relay port set by `SetDeniedPorts` in `wireShareServer`; the dialer re-checks | the deny rows of `TestPortGrant_Refusals` and `TestPortProxy_Refusals` |
| The viewer's consent for that Mac and port | `handlePortOpen` (`internal/daemon/portforward_api.go`): `consent` is returned until an approval exists; the sheet answers it | `TestPortForward_ConsentFirst`, `TestPortOpen_Approvals`, `TestPortOpen_ApprovalLiveCheck`, `TestPortForward_StopReshareClickAsksConsent`, `TestPortForwardE2E_StopSharing`, `TestPortForwardE2E_ReshareWithoutStream`; the sheet in `frontend/src/components/__tests__/TerminalPanel.remotePort.test.tsx` |
| The host dials only its own loopback, after every check above | `dialLoopback`: the URL host is ignored, `[::1]` then `127.0.0.1`, validated port only | `TestPortProxy_DialOrder`, and the tripwire (zero connections on the requested port, both families) in every refusal row |

## Check order

The order is fixed and no check moves after a dial.

**Viewing Mac, `POST /peers/ports` (`handlePortOpen`):**

1. The request is a JSON body of at most 8 KiB; the URL is classified (the five loopback forms, a port 1..65535, no userinfo; the same fixture pins the TypeScript and Go classifiers) and the URL to open is built from the port alone.
2. The daemon holds a capability for the session (`no_cap` otherwise).
3. The host grant (below) runs. A refused grant gives `refused`, an old host gives `host_outdated`, anything else `unreachable`. The grant runs before consent so the sheet is never shown for a port the host would refuse.
4. A stale approval is dropped (an approval is live only while the session still holds the capability it was given under).
5. Another Mac holding the port gives `port_in_use`.
6. A live approval with a forward rebinds it to the new port token and answers `open`.
7. Otherwise any forward with no live approval is closed, the port is dial-probed on both loopbacks (`port_in_use` unless both refuse), and with no approval and no `consent` flag the answer is `consent`.
8. With consent: bind both loopbacks, install the forward, record the approval, answer `open`.

**Host Mac, grant route (`POST /api/peer/sessions/{id}/ports`, `handlePortGrant`):** same user, JSON content type, body at most 1 KiB with an integer port in 1..65535, capability with write permission, deny list, the session's hub and `PortPrinted`, then mint. A grant never connects to anything. Every refusal is the same answer (`refusePort`, the 403 and the `X-Portty` marker), whatever the cause or the port's state.

**Host Mac, proxy route (`<METHOD> /api/peer/sessions/{id}/ports/{port}`, `handlePortProxy`):** same user, `{port}` as decimal digits without a leading zero in 1..65535, capability with write permission, `VerifyPort` with session, port and GrantID all matching, deny list, the `X-Portty-Target` header (non-empty, at most 8192 bytes, starts with `/`, parses), the revoke-bound context (the grant is re-checked after taking the revoke signal), then the proxy, which dials `[::1]` then `127.0.0.1`.

**Viewing Mac, the forward's own checks (`portForward.ServeHTTP`), per request:** the forward is not closed; the Host header is exactly `localhost:<P>` or `127.0.0.1:<P>` (the bare forms only for port 80); a `Sec-Fetch-Site: cross-site` request is a navigation; the target starts with `/` and is at most 8192 bytes (414 and 400 with fixed bodies, because the host would answer a long target with the same 403 as a stopped share and the forward would close). Then the request goes to the host with `X-Portty-Target`, `X-Portty-Cap` and `X-Portty-Port-Token` set by the daemon from its own store. Every inbound `X-Portty*` header from the page is deleted first.

## Why the contract holds

1. **Nothing reaches an unprinted port.** The grant is the only route to a token and it requires the session's own printed output to contain the URL as a whole token (`:80` is not a token of `:8000`, userinfo and suffix domains do not count, https and upper case are refused). Pinned by `TestHasURLPort_Table`, `TestPortPrinted_*` and `TestPortGrant_Refusals`.
2. **A caller cannot tell what is listening.** Every refusal is byte-identical and carries a tripwire: a listener on the requested port, on both families, that must count zero connections. A printed-but-closed port and a printed-and-open port give the same grant answer. Pinned by `TestPortGrant_IdenticalOpenOrClosed`, and the tripwires in `TestPortGrant_Refusals` and `TestPortProxy_Refusals`.
3. **A port token cannot pass as a session capability, and the reverse.** The token is signed under an HKDF-derived key with its own info string and carries a claims kind, so `capability.Verify` fails its HMAC and `VerifyPort` rejects a session capability. `hmac.Equal` is used and the kind is checked after the signature. Pinned by `TestPortToken_NotASessionCapability`, `TestPortToken_SessionCapNotAPortToken`, `TestPortToken_Tamper`, `FuzzVerifyPort`.
4. **A token is tied to one session, one port and one grant, under the current key.** Another port, another session or another grant is refused. A signing-key regeneration changes the derived key, so a token minted before it is refused with the identical 403, even next to a capability that is good under the new key with the same grant. Pinned by `TestPortProxy_Refusals`, `TestPortToken_KeyRotation`, `TestPortProxyRevoke_KeyRegeneration`.
5. **Another tailnet user gets nothing.** `requireSameUser` fails closed on both routes, so a share link leaked to another user cannot reach ports, and a read-only capability cannot either. Pinned by the "another user" and "read-only" rows of both refusal tables.
6. **Portty's own ports are never proxied.** The share server's port, 7444 and the relay port are denied at the handler and again at the dialer; the relay accepts Origins that name its own port, which the rewritten Origin would otherwise satisfy. Pinned by the deny rows.
7. **The host dials only its own loopback.** The proxy URL host is always `localhost`, the transport's dialer ignores it and takes only the validated port, and it connects to `[::1]` then `127.0.0.1`. Pinned by `TestPortProxy_DialOrder`.
8. **The page cannot forge Portty's headers or marker in either direction.** The viewing daemon deletes every inbound `X-Portty*`; the host deletes every `X-Portty*`, `Forwarded` and `X-Forwarded*` header before the dev server sees the request, and deletes every `X-Portty*` response header so a dev server cannot forge the marker the viewing Mac trusts. Pinned by `TestPortForward_HeaderHygiene` and `TestPortProxy_PathAndHeaders`.
9. **The request the dev server sees is the request the page made.** The path and query travel byte-exact in `X-Portty-Target` on a fixed route, so the share mux never cleans `//` or `..` and the page's own query never reaches the capability lookup. The Host the dev server sees is `localhost:<P>`. Pinned by `TestPortProxy_PathAndHeaders` (`RequestURI` byte for byte) and `TestPortForwardE2E_HostDevServer`.
10. **The forward is reachable only from this Mac, by a request that looks local.** It binds only `127.0.0.1` and `[::1]`; the Host check blocks DNS rebinding; the cross-site check blocks other websites. Pinned by `TestPortForward_OpenThenGET`, `TestForwardHostAllowed`, `TestPortForward_HostRefused`, `TestPortForward_CrossSite`, and the VM `lsof` row in 13-07.
11. **Port shadowing is refused, never done.** The daemon will not take a port another program holds on either loopback. Pinned by `TestPortOpen_PortInUse` (a wildcard holder and a `[::1]`-only holder) and `TestPortOpen_NoApprovalAfterPortInUse`.
12. **Stopping the share ends everything open, on both Macs.** On the host, the proxied request's context is cancelled by `revokeSignal` (closed by `ClearGrants`, which share-off, session exit and the key regeneration's re-mint all call) and by `stopCh` (closed by `Stop`, which shutdown and a handoff call without `ClearGrants`), so an upgraded connection, which `Server.Close` does not reach, ends too, and a client that goes away cancels the upstream request. On the viewing Mac, the session stream's terminal close (`IsTerminalClose`) or a changed capability closes the forward and drops the approval with no request sent to it. Pinned by `TestPortProxyRevoke_ClearGrants`, `TestPortProxyRevoke_Stop`, `TestPortProxyRevoke_KeyRegeneration`, `TestPortProxyRevoke_ClientGone`, `TestPortForwardE2E_StopSharing` (share-off with and without another shared session, and a killed session) and `TestPortForward_StreamCloseRevokes`. In these tests the streams and the forward ended within a few milliseconds of the trigger (bound: 2 s), and the goroutine count returned to its baseline.
13. **A re-share always asks again.** Each share mints a fresh GrantID, and the approval is tied to the capability it was given under, so an approval from before cannot revive. Pinned by `TestPortForward_StopReshareClickAsksConsent`, `TestPortOpen_ApprovalLiveCheck`, `TestPortForwardE2E_StopSharing` and `TestPortForwardE2E_ReshareWithoutStream`.
14. **Nothing secret is logged, shown or written.** The daemon logs one outcome line with the first 8 characters of the session id, the port and the outcome; the host logs the same shape; error pages and notices carry fixed copy, an escaped host label and a numeric port, with `default-src 'none'`, `nosniff` and `no-store` on the pages; the binding's result carries status, URL, host, port and an error code only; approvals and tokens live in memory. Pinned by `TestPortForward_LogsNoSecrets`, `TestPortGrant_LogsNoSecrets`, `TestPortForward_NoDiskWriteAPIs`, `TestOpenRemotePort_NoSecrets`.
15. **The relay port cannot reach the new route.** `POST /peers/ports` is registered on the unix socket mux only, so a web page that can reach the loopback relay cannot ask for a forward. Pinned by `TestPortForward_NotOnRelayHandler`.

## Honest scope

A holder of a write capability already controls the shell on the host (a share always grants full control) and could fetch a local port from inside it with `curl`. So the printed check and the consent sheet are defence in depth against scanning, look-alike links and viewer-side mistakes, not the boundary.

The boundary that is new, and real:

- Nothing reaches an unprinted, unapproved or Portty-internal port.
- Nothing reaches any port for another tailnet user.
- Nothing reaches any port for a read-only capability.
- Nothing stays reachable after the share ends, on either Mac.

The viewing Mac's side is a different kind of exposure: a page the user approved is served from this Mac's loopback port, so anything that can reach that loopback port while the forward is open can use it (see the accepted risks).

## Accepted risks

Each with its source.

- **No hard-deny list for sensitive ports such as 9222, 2375 or 5432.** A session can print any URL, including one for a debugging or database port, and a user who clicks and allows it reaches that port through the host's share. The controls are the printed check, same-user, write access and consent, and the sheet names the port. Source: CONTEXT "Post-research refinements"; RESEARCH Open Question 2.
- **localhost cookies and storage are shared with this Mac's other local apps.** The page runs at `http://localhost:<P>` in this Mac's browser, so the remote dev server receives this Mac's cookies for `localhost` and the page shares storage with any local app on the same origin. Source: RESEARCH Pitfall 10.
- **A multi-port app reaches this Mac's own localhost for an unapproved second port.** A page on an approved port that calls another port on `localhost` reaches this Mac's own program on that port, or nothing, not the host's. Each port needs its own click and consent. Source: RESEARCH Finding 2.
- **`https://localhost`, portless forms, upper-case and other non-canonical loopback forms in a remote tab keep today's behaviour,** which opens this Mac's own localhost. They are not routed through the proxy, and nothing is proxied for them. Source: CONTEXT "Scope and failures"; RESEARCH Open Question 5.
- **Any local process or other local user can use an approved forward while it is open.** It is a loopback listener, like a local dev server; the Host check and the cross-site check stop websites, not programs on this Mac. Source: RESEARCH Security Domain ("Other website CSRFs the forward"), the loopback design in CONTEXT.
- **The host dials `[::1]` then `127.0.0.1` for every accepted form.** CONTEXT's "own loopback only" is kept, but a page printed as `localhost` is reached on whichever family the dev server bound. Source: CONTEXT refinement; RESEARCH Open Question 1.
- **No in-flight cap on proxied streams for a same-user write-capability holder (T-13-34).** Such a holder already controls the shell. Source: RESEARCH A8, the 13-06 threat model.
- **Per-request WhoIs may turn a tailscaled hiccup into a 503 on asset-heavy pages.** `requireSameUser` fails closed on every proxied request, so a slow `tailscaled` is felt as a failed asset. Source: RESEARCH Pitfall 5.
- **Approvals are memory-only, with no indicator and no list.** A restart of the viewing daemon, a handoff or the end of the share ends them; there is no way to see or revoke one by hand. Source: CONTEXT "Consent" and the deferred list.
- **The no-stream window.** With no peer-tab session stream open and no reconnect on the viewing Mac, a share-off sends the daemon no signal, so the forward keeps listening. The next time the browser opens it (a reload, a link or the page's own retry, with no consent prompt because no Portty click is involved) the first request gets the host's stopped page and the forward then closes. A Portty click in that window is refused at the grant and opens nothing. This is safe because each share mints a fresh GrantID: the host refuses the old capability and its port tokens on both routes, so nothing is proxied in the window. A reconnect (`POST /peers/connect` with the new capability) closes the forward at once, and the next click asks for consent. Source: 13-02 (checker B1); pinned by `TestPortForwardE2E_ReshareWithoutStream` (`PeerConnectAsksConsent` and `NoConnectStoppedPage`) and `TestPortForward_StoppedSharing`; seen on real machines in 13-07 L-06.

## What stays unchanged

- **The share page CSP and the desktop CSP.** `internal/webserver/csp_mw.go` and `frontend/index.html` are byte-identical to commit 9574fd49 (the 13-06 gate checks it). The proxy route is outside the CSP wrapper on purpose: it carries the dev server's own response headers, including its own CSP, and adds none.
- **No CORS.** No `Access-Control-Allow-Origin` exists in non-test `internal/webserver` code, and a desktop peer tab never calls the host from the webview; all host traffic goes through the viewing daemon. The refusals carry no `Access-Control-*` header (checked in the refusal tables).
- **The token gate on existing routes.** `verifyCapability` now calls `verifyCapabilityToken` with the same sequence, statuses and bodies; `internal/webserver/capability_test.go` and `whois_test.go` are identical to 9574fd49 and pass.
- **The tailnet-only listener and the WhoIs gate.** Both new routes sit behind `requireSameUser` on the same listener; no new listener exists on the host. The only new listeners are the viewing Mac's loopback forwards.
- **Phase 12 path clicks.** `internal/relay/pathclick_test.go` is identical to 9574fd49; the printed-text search was generalised to take a predicate and the old entry points remain as thin wrappers.
- **The existing link tests.** No `expect(` line was removed from the six link test files and `internal/webserver/web_links_test.go` is unchanged. Local tabs, the browser share view and non-loopback URLs keep today's behaviour (URL-03).
- **Forwards are not carried across a Phase 9 handoff.** They close on the handoff and the next click reopens them, asking consent again.

## Version compatibility

- `APIVersion` is 5 and `PortProxyAPI` is 5; `MinDaemonAPI` stays 1, so no "Update needed" banner appears for a daemon on API 4, which simply has no remote localhost clicks. Pinned by `TestHealthInfo` (`internal/daemon/client_test.go`) and `TestAPIHealth` (`internal/daemon/api_test.go`).
- **An old local daemon (API 4):** the binding checks `DaemonAPI(h) < PortProxyAPI` and answers `daemon_outdated` without calling the route; a 404 or 405 with no JSON error from the local mux is `daemon_outdated` too. Pinned by `TestOpenRemotePort_DaemonOutdated` and `TestOpenRemotePortClient`.
- **An old host (API 4):** its share server has no port routes, so the grant gets a plain 404 or 405 with no `X-Portty` marker, which the viewing daemon maps to `host_outdated`. Every refusal a new host gives carries the marker, so a refusal is never read as an old host. Pinned by `TestPortOpen_ErrorCodes` (the `host_outdated` row) and `TestOpenRemotePort_Codes`.
- **A new host, an old viewing Mac:** the old Mac never calls the routes; nothing changes for it.
- **The click on an old pair:** it shows one of the fixed notices, opens nothing, and never falls back to opening this Mac's own localhost.

## Viewer, consent and notices

- **Where it fires.** Only in a desktop peer tab (`remote && !wsURL`) and only when the classifier matches one of the five loopback forms (`localhost`, `127.0.0.1`, `[::1]`, `0.0.0.0`, `[::]`) over `http`. The local tab, the browser share view, `https://` loopback URLs, non-loopback URLs and non-canonical forms keep today's `getRisk` and `openLink` path. Trailing punctuation an xterm link match keeps (a `)`) is trimmed first. Pinned by `frontend/src/lib/__tests__/localhostUrl.test.ts` and `TestClassifyLoopbackURL_Fixture` over the shared fixture, and by the regressions in `frontend/src/components/__tests__/TerminalPanel.remotePort.test.tsx`.
- **What opens.** Only the URL the binding returns with status `open`, which the daemon builds as `http://localhost:<P>` plus the path, query and fragment. Every error and every consent decline opens nothing. The tests assert `openLink` is never called on a failure.
- **Consent.** On status `consent`, an in-window sheet (the RegenerateKeyModal pattern in a portal): "Allow access to {host}'s localhost:{port}?", with the body "Pages from this port open in your browser through Portty over your tailnet. Other ports stay closed." and the buttons Don't Allow and Allow. Focus starts on Don't Allow, Esc and Don't Allow make no second call. Allow calls again with consent true; if that answer is not `open` the user gets the unreachable notice, never a second sheet, so a misbehaving binding cannot loop the user through sheets. The sheet is shown only after the host has already granted the port.
- **Approval lifetime.** Per (Mac, port), in memory only. It ends when that Mac stops sharing, the session ends, the capability changes, the daemon stops or hands off, or the viewing daemon restarts.
- **Notices.** Five fixed copies in the clicking pane only (host needs an update, this Mac's daemon needs an update, port in use, refused, unreachable), each with the host label and a numeric port and nothing from the page. They share the existing notice slot and timer. After a page is open, the viewing daemon serves one of three fixed error pages in the browser (stopped sharing, nothing answering, couldn't reach).
- **Pinned by** `TerminalPanel.remotePort.test.tsx`, `SessionEndedBanner.test.tsx`, `TestOpenRemotePort_Codes`, `TestOpenRemotePort_NoSecrets`, and for the pages `TestPortForward_StoppedSharing`, `TestPortForward_NothingAnswering`, `TestPortForward_HostGone`.

## Threat register

Rows from the threat models of 13-01 to 13-06. T-13-SC (package installs) is in every plan: no package was installed in any of them (the stdlib `crypto/hkdf` and `httputil.ReverseProxy` only), so the package-legitimacy gate never triggered. Pinned-by entries without a path are in the package named in the contract table.

| ID | Threat | Severity | Disposition | Pinned by |
|----|--------|----------|-------------|-----------|
| T-13-01 | Information disclosure: `hasURLPort` port scanning by look-alike text | high | mitigate | `TestHasURLPort_Table`, `TestHasURLPort_Banners` |
| T-13-02 | Denial of service: `Hub.PortPrinted` snapshots under the emulator lock | medium | mitigate | `TestPortPrinted_Busy` |
| T-13-03 | Elevation: a port token accepted as a session capability | high | mitigate | `TestPortToken_NotASessionCapability`, `TestPortToken_SessionCapNotAPortToken`, `TestPortKey_Derived` |
| T-13-04 | Spoofing: a forged or altered port token | high | mitigate | `TestPortToken_Tamper`, `TestPortToken_KeyRotation`, `FuzzVerifyPort` |
| T-13-05 | Tampering: Phase 12 path-click search changed by the predicate refactor | medium | mitigate | `internal/relay/pathclick_test.go` byte-identical to 9574fd49 and passing |
| T-13-06 | Spoofing: DNS rebinding to the forward listener | high | mitigate | `TestForwardHostAllowed`, `TestPortForward_HostRefused` |
| T-13-07 | Tampering: cross-site pages and WebSocket hijacking of the forward (`Sec-Fetch-Site`) | medium | mitigate | `TestPortForward_CrossSite` |
| T-13-08 | Tampering: probe and bind shadowing a local server | high | mitigate | `TestPortOpen_PortInUse`, `TestPortOpen_NoApprovalAfterPortInUse` |
| T-13-09 | Information disclosure: the forward reachable off-host | high | mitigate | `TestPortForward_OpenThenGET`; the VM `lsof` row in 13-07 |
| T-13-10 | Information disclosure: capability, port token and request targets in logs or on disk | high | mitigate | `TestPortForward_LogsNoSecrets`, `TestPortForward_NoDiskWriteAPIs` |
| T-13-11 | Tampering: header injection by a page | high | mitigate | `TestPortForward_HeaderHygiene` |
| T-13-12 | Elevation: `POST /peers/ports` from a web page through the relay port | high | mitigate | `TestPortForward_NotOnRelayHandler` |
| T-13-13 | Elevation: a forward or approval outliving the share, the daemon or a handoff | high | mitigate | `TestPortForward_StreamCloseRevokes`, `TestPortForward_NewCapRevokes`, `TestPortForward_StopReshareClickAsksConsent`, `TestPortOpen_ApprovalLiveCheck`, `TestPortForward_StoppedSharing`, `TestPortForward_CloseAll` |
| T-13-14 | Tampering: reflected content in the error pages | medium | mitigate | `TestPortForward_StoppedSharing`, `TestPortForward_NothingAnswering` (fixed copy, escaped label, `default-src 'none'`, `nosniff`, `no-store`) |
| T-13-15 | Spoofing: another tailnet user with a leaked link | high | mitigate | `TestPortGrant_Refusals`, `TestPortProxy_Refusals` ("another user" rows) |
| T-13-16 | Information disclosure: port scanning through the host | high | mitigate | `TestPortGrant_IdenticalOpenOrClosed`, `TestPortGrant_Refusals` and `TestPortProxy_Refusals` (tripwires) |
| T-13-17 | Elevation: proxying to Portty's own ports | high | mitigate | the deny rows of `TestPortGrant_Refusals` and `TestPortProxy_Refusals` |
| T-13-18 | Tampering: header smuggling to the dev server | medium | mitigate | `TestPortProxy_PathAndHeaders` |
| T-13-19 | Spoofing: a dev server forging the `X-Portty` marker | medium | mitigate | `TestPortProxy_PathAndHeaders` |
| T-13-20 | Tampering: Origin laundering (a cross-site WebSocket made same-origin) | high | mitigate | `TestPortProxy_PathAndHeaders` (five Origin rows); `TestPortForwardE2E_WebSocketAndSSE` (the typed `http://127.0.0.1:<P>` form passes the dev server's default check) |
| T-13-21 | Elevation: the host dialling beyond its own loopback | high | mitigate | `TestPortProxy_DialOrder` |
| T-13-22 | Tampering: path cleaning or query collision on the share mux | medium | mitigate | `TestPortProxy_PathAndHeaders` (byte-exact `RequestURI`), `TestPortForwardE2E_HostDevServer` |
| T-13-23 | Elevation: the `verifyCapability` refactor weakening existing routes | high | mitigate | `internal/webserver/capability_test.go` byte-identical to 9574fd49 and passing |
| T-13-24 | Spoofing: version confusion (an old daemon read as a refusal or as success) | medium | mitigate | `TestOpenRemotePort_DaemonOutdated`, `TestOpenRemotePortClient`, `TestHealthInfo`, `TestAPIHealth` |
| T-13-25 | Information disclosure: the binding result carrying a capability or port token to the webview | high | mitigate | `TestOpenRemotePort_NoSecrets` |
| T-13-26 | Denial of service: the binding throwing into the click handler | low | mitigate | `TestOpenRemotePort_NoDaemon`, `TestOpenRemotePort_Codes` |
| T-13-27 | Spoofing: classifier look-alike forms routed to the proxy, or loopback forms missed | medium | mitigate | `localhostUrl.test.ts`, `TestClassifyLoopbackURL_Fixture` |
| T-13-28 | Tampering: a failure path opening this Mac's own localhost | high | mitigate | the notice tests in `TerminalPanel.remotePort.test.tsx` (`openLink` never called on a failure) |
| T-13-29 | Elevation: consent bypass | high | mitigate | the consent tests in `TerminalPanel.remotePort.test.tsx` (Don't Allow and Esc make no second call; focus on Don't Allow) |
| T-13-30 | Tampering: URL-03 regression (local or browser-view links routed through the proxy) | medium | mitigate | the regressions in `TerminalPanel.remotePort.test.tsx`; the existing link tests unchanged |
| T-13-31 | Information disclosure: a notice or the sheet reflecting attacker text | low | mitigate | `SessionEndedBanner.test.tsx`; fixed copy, host label and numeric port, no raw HTML |
| T-13-32 | Elevation: hijacked WebSocket and SSE, forwards or approvals outliving share-off, session exit or Stop | high | mitigate | `TestPortProxyRevoke_ClearGrants`, `TestPortProxyRevoke_Stop`, `TestPortForwardE2E_StopSharing`, `TestPortForwardE2E_ReshareWithoutStream` |
| T-13-33 | Elevation: port tokens surviving a signing-key regeneration | high | mitigate | `TestPortProxyRevoke_KeyRegeneration` |
| T-13-34 | Denial of service: many proxied streams from a same-user write-capability holder | low | accept | none (accepted; see Accepted risks) |
| T-13-35 | Repudiation: the security contract for host port access | medium | mitigate | this file, with the VM evidence and verdict added by 13-07; the 13-06 gate greps it for link, token and tailnet URL patterns |

## Tests that pin it

- Printed check: `TestHasURLPort_Table`, `TestHasURLPort_Banners`, `TestPortPrinted_*` (`internal/relay/portprinted_test.go`)
- Port token: `TestPortToken_NotASessionCapability`, `TestPortToken_SessionCapNotAPortToken`, `TestPortToken_Tamper`, `TestPortToken_KeyRotation`, `TestPortKey_Derived`, `FuzzVerifyPort` (`internal/capability/porttoken_test.go`)
- Host grant and proxy: `TestPortGrant_Granted`, `TestPortGrant_Refusals`, `TestPortGrant_IdenticalOpenOrClosed`, `TestPortGrant_LocalFault`, `TestPortGrant_LogsNoSecrets`, `TestPortProxy_Refusals`, `TestPortProxy_PathAndHeaders`, `TestPortProxy_DialOrder`, `TestPortProxy_Methods` (`internal/webserver/portproxy_test.go`)
- Host revoke: `TestPortProxyRevoke_ClearGrants`, `TestPortProxyRevoke_Stop`, `TestPortProxyRevoke_KeyRegeneration`, `TestPortProxyRevoke_ClientGone` (`internal/webserver/portproxy_revoke_test.go`)
- Viewing forward: `TestPortForward_*`, `TestPortOpen_*`, `TestForwardHostAllowed`, `TestClassifyLoopbackURL_Fixture` (`internal/daemon/portforward_test.go`)
- Two daemons together: `TestPortForwardE2E_HostDevServer`, `TestPortForwardE2E_WebSocketAndSSE`, `TestPortForwardE2E_StopSharing`, `TestPortForwardE2E_ReshareWithoutStream` (`internal/daemon/portforward_e2e_test.go`)
- Binding and versions: `TestOpenRemotePort`, `TestOpenRemotePort_DaemonOutdated`, `TestOpenRemotePort_Codes`, `TestOpenRemotePort_NoSecrets`, `TestOpenRemotePort_NoDaemon` (`app_test.go`), `TestOpenRemotePortClient`, `TestHealthInfo` (`internal/daemon/client_test.go`), `TestAPIHealth` (`internal/daemon/api_test.go`)
- Viewer: `frontend/src/lib/__tests__/localhostUrl.test.ts`, `frontend/src/components/__tests__/TerminalPanel.remotePort.test.tsx`, `frontend/src/components/__tests__/SessionEndedBanner.test.tsx`, and the shared fixture `frontend/src/lib/__tests__/fixtures/localhost-url.json`
- Unchanged, still passing: `internal/webserver/capability_test.go`, `internal/webserver/whois_test.go`, `internal/relay/pathclick_test.go`, `internal/webserver/web_links_test.go`

## VM evidence and verdict

Evidence from 13-07 on the two Tart VMs over the real tailnet (office `portty-vm-office` is the host, home `portty-vm-home` the viewing Mac), 2026-11-22, all in `13-VM-CHECKS.md`. Builds A13 (pre-phase, API 4), B13 and HEAD (API 5), ad-hoc signed. No link, capability, port token or tailnet URL is written here.

- **Old host and upgrade (L-01, PASS).** Against an API 4 office the click showed exactly `portty-vm-office needs a Portty update to open its local pages.` with no sheet, nothing bound and no browser (`outcome=host-outdated`). After the office's upgrade by handoff the session survived, the tab reconnected with `mode=delta`, and the same pre-upgrade URL asked for consent (`outcome=consent`): the printed check read output carried across the handoff, and no approval carried over. Don't Allow opened nothing.
- **The page through the forward (L-02, PASS).** The printed `http://[::]:8000/` opened the office page in home Safari after one sheet: title `p13 site`, `GET /`, `/app.js` and `/logo.png` each 200 at the dev server with `host=localhost:8000` and no `X-Portty` or forwarded header; the `127.0.0.1` form with `sub/page.html?q=a%20b` opened with no second sheet and the target arrived byte-exact. The home listener was the daemon's, with Safari's WebKit networking process as its client; no in-app browser.
- **Consent and the loopback-only listener (L-03, PASS).** The sheet read exactly `Allow access to portty-vm-office's localhost:8001?` with the fixed body. Return and Esc opened and bound nothing; Allow opened the page. `lsof` listed exactly `127.0.0.1:8001` and `[::1]:8001` for the daemon and no `*:8001`. Approval of one port did not cover another.
- **Refusals before any connection, scan resistance and tailnet-only (L-04, PASS).** A grant for a printed port returned 200; six refusals (no capability, garbage capability, a never-printed port with a tripwire, a closed port, the share port, the relay port) were all 403 with `X-Portty: 1` and one normalised hash; the proxy route refused a missing token and another port's token the same way. **The tripwire count stayed 0.** From the home VM the office's LAN address on 7444 failed to connect (curl exit 7) and `lsof` showed 7444 only on the tailnet address.
- **Streams and hot reload (L-05, PASS).** WebSocket and SSE counters rose through both hops, a touch of the hot file reloaded the page in about 1.2 s, the WebSocket request carried `host=localhost:8100 origin=http://localhost:8100` with no `X-Portty` or forwarded header, and a `::1`-only server worked.
- **Revoke on stop-sharing (L-06, PASS).** `portty unshare` produced the home log line `outcome=revoked` 50 ms later with nothing done on the home VM; the page's WebSocket and SSE closed and the listener was gone in under 0.7 s (poll granularity), and the peer tab showed `portty-vm-office stopped sharing this session.` After a re-share and reopening the tab, a click asked for consent again. The no-stream window behaved as recorded below.
- **The taken port (L-07, PASS).** A wildcard holder and a `::1`-only holder each gave `Port N is in use on this Mac.` with no sheet, and the home servers kept answering.
- **Unchanged links and the `https://localhost` gap (L-08, PASS).** A local tab opened `localhost:8600` directly with no sheet and no forward; the punycode link showed the confirm popover in a local tab and a peer tab; `https://example.com/` opened with no dialog; `https://localhost:8600/` asked home Safari for its own `https://localhost:8600` with no sheet and no forward, the accepted gap.
- **UI-1, another tailnet user.** "Accept automated-only" (the user, 2026-11-22). One account serves every machine in the rig, so a refusal for another user was not shown on real machines; `TestPortGrant_Refusals` and `TestPortProxy_Refusals` (other-user rows) pin it.
- **UI-2, a real Vite or webpack HMR project.** "Check on my Macs" (the user, 2026-11-22). Neither VM has node, so L-05 used the `p13dev` helper; a real project's HMR is manual check M-140 for the user's own Macs.
- **Gate finding G-1.** One `internal/pty` `TestKillClean_NoOrphans` FAIL in the single gate run, recorded by the user as the known G-1 flake; a re-run alone with `-count=5` passed and `internal/pty` has no diff since `9574fd49`. It does not touch this feature.

### Residual risks

The accepted risks listed in "Accepted risks" above stand. In addition, from the VM run:

- **The no-stream window.** With no peer-tab session stream open, a share-off sends the viewing daemon no signal, so the forward keeps listening. On real machines it stayed open about 4.1 s after the re-share, until the page's own EventSource retry made the first request; that request got the stopped page (`outcome=stopped`), the forward closed within 0.6 s, and the office dev server logged no request, so nothing was proxied. Safe because each share mints a fresh grant id.
- **`https://localhost`, `*.localhost` and other non-canonical forms keep today's behaviour.** `https://localhost:8600/` opened this Mac's own localhost in L-08. Names such as `app.localhost`, portless forms and upper-case hosts are not classified as remote localhost URLs and are not proxied either.
- **No deny list for well-known ports.** A session that prints a URL for a debugging or database port can be clicked and approved; the sheet names the port, and the printed check, same-user, write access and consent are the controls.
- **A `::1`-only page starts more slowly.** In L-05 the `::1`-only server's page was still `Untitled` at +2 s and counting by +7 s, while IPv4 pages counted by +2 s. Observed once, not investigated, and not a refusal or a leak.
- **UI-1 and UI-2 are not shown on real machines** (see above): another tailnet user is automated-only, and a real Vite or webpack HMR project is a check for the user's own Macs.

### Threat register for 13-07

| ID | Threat | Severity | Disposition | Result |
|----|--------|----------|-------------|--------|
| T-13-36 | Tampering: the host machine | high | mitigate | every GUI, daemon and app action ran only in the VMs; the host ran the gate, builds, the helper build and SSH; ad-hoc signing only |
| T-13-37 | Information disclosure: evidence and this review | high | mitigate | links, capabilities and port tokens only in 0600 files in deleted private directories; evidence holds codes, header names and hashes; both files grep clean for link, token and tailnet URL patterns |
| T-13-38 | Denial of service: ephemeral VM tailnet nodes | high | mitigate | no network drop; the VMs were never stopped or re-authenticated and are still on the tailnet |
| T-13-39 | Tampering: VM settings and dotfiles after the run | medium | mitigate | settings.json restored (`cmp` clean); dotfile sha256 equal to L-00 (the office history restored from the L-00 copy); HEAD deployed on both |
| T-13-40 | Tampering: temporary worktrees and helper source left behind | medium | mitigate | the A13 and helper worktrees were removed; none is registered; the helper source and binary were deleted from the host and never committed |
| T-13-41 | Elevation of privilege: helper servers and tripwires left listening | low | mitigate | every `p13dev` process was killed and the sessions stopped; only the p13dev binary stays in `~/p13` |
| T-13-SC | Tampering: package installs | high | mitigate | only `pnpm install --frozen-lockfile`; no package installed in the VMs; the Command Line Tools install prompt opened by a stray `strings` call was killed with nothing installed |

**Verdict:** approved. L-03 (consent per Mac and port, loopback-only listener), L-04 (refusals before any connection, tripwire 0, tailnet-only) and L-06 (revoke on stop-sharing with no request, consent again after a re-share, the recorded no-stream window) are each PASS, as are L-00..L-02, L-05, L-07 and L-08; UI-1 is accepted as automated-only and UI-2 is a check on the user's own Macs, both in the user's words above (2026-11-22).
