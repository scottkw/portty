---
phase: 13-remote-localhost-urls
reviewed: 2026-11-22T00:00:00Z
depth: standard
files_reviewed: 21
files_reviewed_list:
  - app.go
  - frontend/src/components/SessionEndedBanner.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/lib/localhostUrl.ts
  - internal/capability/porttoken.go
  - internal/daemon/api.go
  - internal/daemon/client.go
  - internal/daemon/handoff.go
  - internal/daemon/peer_client.go
  - internal/daemon/peer_connect.go
  - internal/daemon/portforward.go
  - internal/daemon/portforward_api.go
  - internal/daemon/remote_ws_proxy.go
  - internal/daemon/share.go
  - internal/daemon/types.go
  - internal/relay/hub.go
  - internal/relay/pathclick.go
  - internal/relay/portprinted.go
  - internal/webserver/capability_mw.go
  - internal/webserver/portproxy.go
  - internal/webserver/server.go
findings:
  critical: 1
  warning: 4
  info: 3
  total: 8
  necessity: 0
status: issues_found
---

# Phase 13: Code Review Report

**Reviewed:** 2026-11-22
**Depth:** standard
**Files Reviewed:** 21
**Status:** issues_found

## Summary

Reviewed the diff against `9574fd49` for the host proxy (`portproxy.go`, `porttoken.go`), the viewing forward (`portforward.go`, `portforward_api.go`), the printed check, and the click/consent UI. Test files were read for intent only.

The core design holds up under adversarial reading:

- Check order on the host is correct. Every credential is verified before the first dial, and a grant never dials.
- The port token is signed under an HKDF-derived key. Token confusion fails in both directions: `Verify` fails the HMAC on a port token, and `VerifyPort` fails the HMAC on a session capability.
- Dialing is loopback-only and ignores the URL host.
- Hop-by-hop headers are removed before `Rewrite` runs, so a page cannot smuggle `X-Portty-*` through a `Connection:` token.
- `X-Portty-Target` is parsed with `ParseRequestURI`, so a `//host` target stays a path and cannot become an authority.
- Revoke is wired through `revokeSignal` and `stopCh`. The goroutine watcher always exits.

Two things do not hold:

1. One concrete hole in the Portty-internal port deny list that appears after an in-place upgrade (handoff). This is a violation of a stated contract row.
2. A handful of robustness and hardening gaps (see Warnings). One of them, the viewer's `Sec-Fetch-Site` defence, is only as strong as the browser's header support. The security review's "mitigate" disposition for it was never shown on the user's browser.

## Critical Issues

### CR-01: Relay port is not denied on a daemon that came up through a handoff

**File:** `internal/daemon/share.go:295` (call site), `internal/daemon/handoff.go:1126-1163` and `:1281` (ordering), `internal/daemon/api.go:296-300`
**Issue:** `wireShareServer` calls `ws.SetDeniedPorts(a.relayPort)`, and `a.relayPort` is only set by `serveRelay` (`api.go:297`).

- On the normal start path, `StartRelay` runs at process start, so the value is right.
- On the takeover path (Phase 9 handoff), the new daemon's share server is built and wired in `takeover.finalize()` (`handoff.go:943` and `:1226`, via `:1162`). That runs before COMMIT.
- The relay listener is only served later, in `commit()` via `a.serveOn(t.ln, t.relayLn)` (`handoff.go:1281`).
- At wire time, `a.relayPort` is therefore `0` in the new process, and `SetDeniedPorts(0)` leaves the deny set as `{0}`.

After any in-place upgrade of a sharing host, the proxy and grant routes will mint a token for, and then dial, the host's relay port. The contract (13-SECURITY-REVIEW "The contract" row 7, "Why it holds" item 6, T-13-17) says the relay port is never proxied. The reason is that the relay's Origin allowlist accepts Origins naming its own port, which the rewritten `http://127.0.0.1:<P>` to `http://localhost:<P>` Origin satisfies.

The dial-time re-check in `newPortTransport` uses the same empty set, so there is no second layer. The 13-07 VM run tested the relay-port refusal only on a freshly started daemon (L-04). The handoff run (L-01) only checked consent. No daemon-level test pins `wireShareServer`'s deny list.

A write-cap holder already has a shell on the host, so this is defence in depth. It is still a stated hard-deny that silently does not apply after every upgrade, so it is classified as a blocker.

**Fix:** Record the relay port before wiring, and pin it with a test. For example, in `finalize()` before `a.wireShareServer(ws, t.health)`:

```go
if tl, ok := t.relayLn.(*net.TCPListener); ok {
    a.relayPort = tl.Addr().(*net.TCPAddr).Port
}
```

Alternatively, have `wireShareServer` derive the port from `a.relayLn` when `a.relayPort == 0`. Better still, give the webserver a deny predicate evaluated per request instead of a value captured at wire time, so ordering can never matter again. Add a test that builds a takeover daemon, runs `finalize`, and asserts `portDenied(relayPort)` on the resulting share server. Also make `SetDeniedPorts` ignore `0`, so a missing value is not silently "denied".

## Warnings

### WR-01: `X-Portty-Target` is not validated for bytes the request line cannot carry (request-line injection)

**File:** `internal/webserver/portproxy.go:166-175` (and `:212`)
**Issue:** The host accepts any target that starts with `/` and survives `url.ParseRequestURI`. That function rejects only control bytes below 0x20 and 0x7f. A space survives in the query, and `RawQuery` is copied verbatim into the outbound request.

Reproduced with Go 1.27: a target of `/a?x y HTTP/1.0` produced the upstream request line `GET /a?x y HTTP/1.0 HTTP/1.1`. Dev servers disagree on how to parse that. A request line with an attacker-chosen "protocol" token is exactly the request-smuggling primitive the review asked about.

The viewing daemon never produces such a target, because Go's server and the browser both encode spaces. A same-user holder of a write capability can set the header directly, though, and the host route is the trust boundary. The contract says "the request the dev server sees is the request the page made", and nothing enforces it for non-daemon callers.

**Fix:** After the existing checks, reject any byte `<= 0x20` or `>= 0x7f` in `target`:

```go
for i := 0; i < len(target); i++ {
    if c := target[i]; c <= 0x20 || c >= 0x7f {
        refusePort(w)
        return
    }
}
```

Add a refusal row for `?x y` and for a raw non-ASCII byte to `TestPortProxy_Refusals`. The viewing forward already emits `RequestURI()`, which percent-encodes both, so legitimate traffic is unaffected.

### WR-02: Cross-site defence on the forward fails open when the browser sends no `Sec-Fetch-*` headers, and nothing else backs it

**File:** `internal/daemon/portforward.go:364-370`
**Issue:** Once Host and Origin look local, any website can drive the remote dev server through the forward (no-cors requests, WebSocket hijacking). The only guard is `Sec-Fetch-Site: cross-site` and `Sec-Fetch-Mode != navigate`. A request that lacks `Sec-Fetch-Site` passes. The plan states this explicitly ("requests without the header pass"). The review's T-13-07 nonetheless reads "mitigate" and "cross-site pages and WebSocket hijacking".

- No evidence in 13-VM-CHECKS or the review shows that the user's browser (Safari, used in L-02) sends the header. The test (`TestPortForward_CrossSite`) only proves the daemon's logic.
- The Host check does not help here. A cross-site `fetch("http://localhost:P")` or `new WebSocket("ws://localhost:P")` carries `Host: localhost:P`.
- On the host, Origin passes through unchanged. That only helps if the dev server enforces it, and Vite's HMR socket check does not cover plain requests.

An `Origin` check does not depend on `Sec-Fetch-*`. Browsers always send `Origin` on a WebSocket handshake and on cross-origin POST.

**Fix:** In `portForward.ServeHTTP`, also refuse a non-navigation request whose `Origin` is present and whose hostname is not one of `localhost`, `127.0.0.1` or `[::1]` (any port). That keeps multi-port apps working (`localhost:3000` to `localhost:8000`) and blocks `https://evil.example`. Add a row to `TestPortForward_CrossSite`. Separately, either record a Safari check in the VM evidence or downgrade T-13-07's disposition to "partial: header-dependent".

### WR-03: A marked 403 from the host closes the forward and drops the approval, so a transient WhoIs miss reads as "stopped sharing"

**File:** `internal/daemon/portforward.go:310-317` and `:326-333`, with `internal/webserver/whois.go:99-117` (`requireSameUser`)
**Issue:** `requireSameUser` answers with the marked 403 (`refuseOtherUser`) whenever `WhoIs` returns `ErrPeerNotFound` or the peer is not the same user. Only other errors give the unmarked 503. `ModifyResponse` maps every marked 403 to `errStopped`, which writes the "stopped sharing" page and calls `closeForward`. That deletes the approval and closes the listener.

If tailscaled has not yet learned the viewer node (after sleep, a path change or a re-key), the next asset request kills a healthy forward. The user sees a false "stopped sharing" page and has to re-click and re-consent. The memory notes say reliability comes first, because nobody can get in to fix the remote Mac. The review accepts "WhoIs may turn into a 503" but not this destructive 403 case.

**Fix:** Close the forward only when the host's answer says the grant is gone. Options:

- Make the host return a distinct marker value or status for the grant-gone cases (inactive grant, bad token, session disabled). `ModifyResponse` closes only on that.
- Alternatively, on a marked 403, re-run the grant (`requestPortGrant`) once. Close only if that is refused too.

Treat a bare `refuseOtherUser` 403 as a retryable "couldn't reach" page.

### WR-04: Re-clicking a live forward reuses a transport pinned to the old host IP

**File:** `internal/daemon/portforward_api.go:254-260` and `internal/daemon/portforward.go:283-286`
**Issue:** `newPortForward` captures `fqdn`, `ip` and `base` once, in `forwardTransport(fqdn, ip)`. A second click while the approval is live does `f.rebind(sid, token)` only. The new grant in the same call used `a.peerClient(fqdn, ip)` with the current IP from `remoteCaps`, so it succeeds. The user is told `open`, but the forward still dials the stale IP and serves "Couldn't reach".

Re-clicking cannot repair it, because the approval is live and the forward is reused. Only ending the share does. This happens whenever the host's tailnet address changes while the capability stays the same: a node re-registered under the same share, or a tab redial that stored a new `ip` with an unchanged capability, since `handlePeerConnect` only revokes on a capability change.

**Fix:** On the live-reuse branch, compare the current `ip` and `base.Host` with the forward's. If they differ, `closeLocked(f)` and fall through to rebuild. The approval is dropped with the forward, so either re-ask consent or carry the approval across explicitly. Alternatively, resolve the address per request from `remoteCaps` instead of baking it into the transport.

## Info

### IN-01: Three hand-copied lists of loopback hosts

**File:** `internal/relay/portprinted.go:14`, `internal/daemon/portforward_api.go:112`, `frontend/src/lib/localhostUrl.ts:7`
**Issue:** The five-host list is repeated in Go (relay), Go (daemon) and TypeScript. The shared JSON fixture pins the two classifiers against each other. Nothing pins the relay's `loopbackHosts` to them, and the printed check and the click classifier must agree for a port to be grantable. If one list gains a form (say `[::ffff:127.0.0.1]`), the click will classify but the grant will refuse, or the reverse.
**Fix:** Export `relay.LoopbackHosts` and use it in `classifyLoopbackURL`. Add one cross-check test that every fixture "accept" case also satisfies `hasURLPort`.

### IN-02: `firstPass` and `fallbackPass` are dead in production code

**File:** `internal/relay/pathclick.go:262-269`
**Issue:** The two thin wrappers have no non-test caller. `findPrinted` calls `findMatch` directly, and only `pathclick_test.go:666-667` uses the wrappers. They exist to keep that test byte-identical to `9574fd49`.
**Fix:** Either move the wrappers into `pathclick_test.go` as test helpers, or leave a one-line comment that they are test-only. They are harmless as they stand.

### IN-03: Test seam in production code can panic

**File:** `internal/daemon/portforward.go:260-261`
**Issue:** `a.peerClientForTest.Transport.(*http.Transport).Clone()` is an unchecked type assertion. A test client built with any other `RoundTripper` (a custom one wrapping `httptest`'s transport, for example) panics inside `handlePortOpen`, with `pf.mu` held and a deferred unlock.
**Fix:** Use the comma-ok form and fall back to a fresh transport that dials through the client, or return a typed error.

---

_Reviewed: 2026-11-22_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
