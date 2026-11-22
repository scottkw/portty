---
phase: 13-remote-localhost-urls
fixed_at: 2026-11-22T00:00:00Z
review_path: .planning/phases/13-remote-localhost-urls/13-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 13: Code Review Fix Report

**Fixed at:** 2026-11-22
**Source review:** .planning/phases/13-remote-localhost-urls/13-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (CR-01, WR-01, WR-02, WR-03, WR-04)
- Fixed: 5
- Skipped: 0

Each fix was test-first: the new test or row was run against the unfixed code and failed for the stated reason, then passed after the fix. Edits and verification ran in the main checkout (no worktree), so the numbers are reproducible from this tree. No new test file was added, so TESTING.md is unchanged. No CSP file was touched and no CORS handling was added.

## Fixed Issues

### CR-01: Relay port is not denied on a daemon that came up through a handoff

**Files modified:** `internal/daemon/handoff.go`, `internal/daemon/handoff_test.go`, `internal/webserver/server.go`
**Commit:** 32a2b551
**Applied fix:** `takeover.finalize` now sets `a.relayPort` from the received relay listener before `wireShareServer`, so the deny set is right when the share server is wired (commit serves the relay later). `SetDeniedPorts` ignores 0. New `TestTakeover_FinalizeDeniesRelayPort` builds a takeover, runs `finalize`, starts the share server, and checks both routes. It uses a printed-port hub so only the deny list can refuse, and a second printed port as a positive control (grant 200, proxy reaches the dialer). The relay port gets no grant, a 403 on the proxy, and no dial.

### WR-01: `X-Portty-Target` is not validated for bytes the request line cannot carry

**Files modified:** `internal/webserver/portproxy.go`, `internal/webserver/portproxy_test.go`
**Commit:** f8ddd52b
**Applied fix:** Before `url.ParseRequestURI`, the proxy refuses a target with any byte `<= 0x20` or `>= 0x7f`, through `refusePort` (the identical refusal). Two rows were added to `TestPortProxy_Refusals` (`/a?x y HTTP/1.0` and a raw non-ASCII byte). That test also checks the answer matches the first refusal and the tripwire counts no dial.

### WR-02: Cross-site defence on the forward fails open when the browser sends no `Sec-Fetch-*` headers

**Files modified:** `internal/daemon/portforward.go`, `internal/daemon/portforward_test.go`
**Commit:** f9b06ba2
**Applied fix:** `portForward.ServeHTTP` also refuses a request that is not a navigation (`Sec-Fetch-Mode != navigate`, including no header) whose `Origin` is present and whose hostname is not `localhost`, `127.0.0.1` or `::1` (any port). `null` and unparseable origins are refused. A cross-site navigation (OAuth callback) still passes. Rows were added to `TestPortForward_CrossSite`: evil origins, lookalike hosts and `null` are refused with no stub hit. Loopback origins on other ports, `[::1]`, and a cross-origin navigation reach the stub.
**Not done (out of this fix's reach):** the review's separate ask to record a Safari `Sec-Fetch-Site` check in the VM evidence, or to downgrade T-13-07's disposition to "partial: header-dependent". That is a documentation and evidence decision for the security verdict, not a code change.

### WR-03: A marked 403 from the host closes the forward and drops the approval

**Files modified:** `internal/daemon/portforward.go`, `internal/daemon/portforward_test.go`
**Commit:** e68387a7
**Status:** fixed: requires human verification (logic change)
**Applied fix:** The host's refusals stay byte-identical, so there is no new marker. On a marked 403, `ModifyResponse` calls the new `refusedOrTransient`, which re-runs `requestPortGrant` once with the held capability. Only a refusal of that (or no held capability) gives `errStopped`, which closes the forward. A fresh token is rebound and answered with the "Couldn't reach, reload" page. Any other grant failure also gives that page. New `TestPortForward_TransientRefusalKeepsForward` checks one marked 403, then exactly one re-grant, then the forward still open, then the next request succeeding with the re-granted token. `TestPortForward_StoppedSharing` now also makes the re-grant refuse, which is the real gone-share case.
**Limit to confirm:** the re-grant runs immediately after the refusal. If tailscaled still has not learned the viewer node, the re-grant is also refused and the forward closes as before. The fix removes the single-miss case, not a sustained one. A delayed retry or a "N consecutive refusals" rule would cover that, at the cost of a slower "stopped sharing" page.

### WR-04: Re-clicking a live forward reuses a transport pinned to the old host IP

**Files modified:** `internal/daemon/portforward.go`, `internal/daemon/portforward_api.go`, `internal/daemon/portforward_test.go`
**Commit:** 82811b55
**Applied fix:** `portForward` now records the `ip` its transport was built for. `handlePortOpen` reuses a live forward only when `ip` and `base.Host` still match. Otherwise it closes the forward and falls through to the existing rebuild path. `live` stays true, so the user's approval is carried across and no new consent is asked. New `TestPortForward_ReclickRebuildsOnNewAddress` checks that an unchanged address reuses the forward, and that a changed address rebuilds it with the new ip, status `open` without consent, and a working request.

## Skipped Issues

None.

## Out of scope (left alone, per the request)

- IN-01: three hand-copied loopback host lists.
- IN-02: `firstPass` and `fallbackPass` have no non-test caller.
- IN-03: unchecked type assertion in the `peerClientForTest` seam (`forwardTransport`).

## Verification

Run in the main checkout with `GOTOOLCHAIN=go1.26.11`:

- `go test -race -short ./internal/daemon`: ok
- `go test -race -short ./internal/webserver`: ok
- `go test -race -short ./internal/relay`: ok
- `go vet ./internal/daemon ./internal/webserver ./internal/relay`: clean
- `gofmt -l internal`: clean

---

_Fixed: 2026-11-22_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
