---
phase: 04-one-click-share
reviewed: 2026-11-17T06:35:40Z
depth: standard
iteration: 2
files_reviewed: 22
files_reviewed_list:
  - TESTING.md
  - app.go
  - cmd_attach.go
  - cmd_attach_test.go
  - frontend/src/App.tsx
  - frontend/src/components/TabBar.tsx
  - frontend/src/lib/remoteAdapter.ts
  - frontend/src/lib/remoteSession.ts
  - frontend/src/wailsjs/go/main/App.d.ts
  - internal/daemon/api.go
  - internal/daemon/peer_client.go
  - internal/daemon/peer_client_test.go
  - internal/daemon/peer_connect.go
  - internal/daemon/remote_caps.go
  - internal/daemon/remote_ws_proxy.go
  - internal/daemon/share.go
  - internal/tailnet/sessions.go
  - internal/webserver/server.go
  - internal/webserver/whois.go
  - tests/check-brand.sh
  - tests/check-share-removal.sh
  - tests/check-traceability-paths.sh
findings:
  critical: 0
  warning: 2
  info: 7
  total: 9
  necessity: 0
status: issues_found
---

# Phase 4: Code Review Report (iteration 2)

**Reviewed:** 2026-11-17T06:35:40Z
**Depth:** standard
**Files Reviewed:** 22 (fix range `ee25e7a3~1..e2dc549a`, 11 commits)
**Status:** issues_found

## Summary

This pass re-checks the iteration-1 fixes (CR-01, CR-02, WR-01..WR-09) and looks for bugs the fixes introduced. I read the full diff and the surrounding code in `share.go`, `peer_connect.go`, `remote_ws_proxy.go`, `whois.go`, `server.go`, `cmd_attach.go`, `App.tsx` and the three gate scripts. I also checked two library behaviours: coder/websocket v1.8.15 removing `http.Client.Timeout` at dial, and tailscale v1.98.3 `local.Client.WhoIs` returning `ErrPeerNotFound` only on a LocalAPI 404.

What I ran (all exit 0):
- `go build ./...`, `go vet ./...` and `go vet -tags vmpreview ./...`
- `pnpm -C frontend exec tsc --noEmit`
- `bash tests/check-share-removal.sh`, run from the repo root and from `tests/`
- `bash tests/check-brand.sh` and `bash tests/check-traceability-paths.sh` (109 paths)
- Targeted tests that need no daemon spawn and no real config dir: `go test ./internal/webserver -run 'WhoIs|TestStop_'` and `go test ./internal/daemon -run 'TestPeerHTTPClient|TestRemoteCap'`

I did not run the full daemon suite, so it would not touch `~/Library/Application Support/portty`.

**Status of the iteration-1 findings:**

| ID | Verdict | Evidence |
|---|---|---|
| CR-01 | Fixed | The gate prints `OK: share removal gate clean`, from the root and from `tests/`. |
| CR-02 | Fixed | No non-test Go file contains `InsecureSkipVerify` (repo-wide grep). `peerHTTPClient` always dials `ip:7444`, ignores the URL host, sets `ServerName` to the FQDN with TLS 1.2 minimum, and fails every dial when there is no IP (no DNS fallback). The WS proxy uses the IP stored in `RemoteCapStore` and `u.Hostname()` from the same `peerBaseURL`, so the name it verifies matches `/peers/connect`. A peer with no IP or no DNS name fails closed: "unreachable", or Go's "ServerName or InsecureSkipVerify must be specified". coder/websocket strips the client's 10 s `Timeout` before the handshake (`dial.go:78-84`), so long-lived terminals are not cut off. |
| WR-01 | Fixed | `Stop` calls `srv.Close()`. Calling it twice is harmless (the error is discarded by `_ = ws.Stop()`), and Close does not wait on handlers, so calling it under `a.mu` cannot deadlock with handlers that take `a.mu.RLock` through the token provider. |
| WR-02 | Fixed | Lock order is always `startMu` then `a.mu`, and nothing takes `startMu` while holding `a.mu`. Every early return unlocks `startMu`. See new WR-01 below for a side effect of the related WR-04 fix. |
| WR-03 | Fixed | `CurrentSigningKey` uses `signingKeyMu`, not `a.mu`, so there is no re-entrancy. Regenerate writes the key before it takes `a.mu`, and install reads it under `a.mu`, so one of the two always updates the new server. |
| WR-04 | Fixed for the reported case. Introduces new WR-01 below. | |
| WR-05 | Fixed | The FQDN pass runs only when the input contains a dot, so CLI labels behave as before. |
| WR-06 | Fixed | A reconnect replaces the tab object, which clears `shareStopped`. The key `sessionId:capToken` forces a remount on a new cap. |
| WR-07 | Fixed | All three gates `cd` to the top level and exit 2 outside a repo. Share-removal exits 2 when TESTING.md or Section 4 is missing, and traceability exits 1 (no longer 0) when TESTING.md is missing. git grep status >1 still exits 2. No silent-pass path remains. |
| WR-08 | Fixed | A `StatusWithoutPeers` failure, or any `WhoIs` error other than `ErrPeerNotFound`, now gives 503 with no marker. The viewer maps that to unreachable, both in the sidebar (which then tries the IP fallback) and in `/peers/connect`. It still refuses in every case. |
| WR-09 | Partly fixed | Local-first, colon names, empty host and the non-TTY order are all correct. The typo mapping is too broad (new WR-02 below). |
| IN-01 | Fixed with CR-02 | Two other stale comments remain (IN-06). |

**New findings:**
- WR-01: a race in the WR-04 fix.
- WR-02: a regression from WR-09, caused by my own iteration-1 suggestion, which was wrong.
- Two new Info items, plus one Info item the fixer reported itself.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: A share-on that gives up (dead session or failed mint) can stop the server another share-on just started

**File:** `internal/daemon/share.go:156-177` (with `unshareLocked`, `share.go:296-300`)
**Issue:** `startMu` is released at line 156, before the `a.mu` section that re-checks and mints. The WR-04 fix calls `unshareLocked(id)` on refusal, and `unshareLocked` stops the server whenever `len(a.shares) == 0`. It does not check whether this call started the server.

How it happens:
1. Share-on A (session X) takes `startMu`, warms the cert, starts and installs the server, and releases `startMu`.
2. Share-on B (session Y) was waiting on `startMu`. Y was alive at B's first check (line 127) but has since exited. B takes `startMu`, sees `ws != nil`, releases it, and wins `a.mu` before A.
3. B finds Y not shareable and calls `unshareLocked(Y)`. `a.shares` is still empty, because A has not minted, so B stops A's server and sets `a.webServer = nil`.
4. A then takes `a.mu`, sees `ws == nil` and returns `start_failed: the share server stopped while starting`.

A failed mint in B has the same effect. The window is the warm-up (up to 20 s) plus contention on `a.mu`, and it needs two concurrent share-ons. That is rare, but the fix created this path: before, a refused share-on never stopped anything.

**Fix:** Keep `startMu` held through the re-check and mint, so no other share-on can sit between "server installed" and "record minted". The lock order stays `startMu` then `a.mu`:
```go
a.startMu.Lock()
defer a.startMu.Unlock()
a.mu.RLock(); ws := a.webServer; a.mu.RUnlock()
if ws == nil {
    if serr := a.startShareServer(ctx, h, newServer, warmCert); serr != nil {
        return ShareResponse{}, serr
    }
}
a.mu.Lock()
defer a.mu.Unlock()
// ...re-check, mint, unshareLocked on refusal (unchanged)
```
Share-on is already serialized during start, so holding the lock for the extra few microseconds of minting costs nothing.

### WR-02: `portty attach <host>` on a real Mac that is not sharing, uses another account, or is unreachable now says "no session named <host>"

**File:** `cmd_attach.go:87-89,129-130`
**Issue:** The WR-09 fix passes `nse` as `onRefused`, and `attachRemote` returns it for every `PeerConnectError` except `multiple_sessions`. Iteration 1 recommended exactly that, and it was too broad. The usage string on line 64 documents `portty attach <host>`. When the host is found but answers `not_sharing`, `forbidden`, `unreachable` or `no_such_session`, the user now sees:

`portty: no session named "office-air". Run 'portty list' to see your sessions.`

The diagnostics the daemon already resolved are thrown away. Before the fix the user saw "office-air isn't sharing any sessions. Turn on Share…" or the same-account message. That is the same kind of wrong-cause message WR-08 fixed on the host side. A workaround exists (`portty attach office-air:`), but nothing points users to it.

**Fix:** Swap in the local error only when the argument did not resolve to a Mac, meaning the typo case:
```go
case errors.As(err, &pe) && onRefused != nil &&
    (pe.Code == daemon.PeerErrUnknownHost || pe.Code == daemon.PeerErrTailscaleOff):
    return onRefused
```
All other codes fall through to `peerConnectMessage`. Extend `TestCmdAttach_Resolution` with a matched peer whose fake meta returns `not_sharing`, and assert the not-sharing message.

## Info

### IN-02: The vmpreview tag gate covers only `.github`, `build.sh` and `scripts` (still open)

**File:** `tests/check-brand.sh:65`
**Issue:** Unchanged from iteration 1. The gate would miss the tag in `wails.json`, a Makefile or Taskfile, or `GOFLAGS` set somewhere other than `.github`.
**Fix:** Widen the pathspec (`-- .github build.sh scripts wails.json Makefile '*.mk' 'Taskfile*'`). Optionally print a stderr banner from the vmpreview `init()`.

### IN-03: Key regeneration closes viewers with "share stopped" while the share stays on (still open)

**File:** `internal/daemon/share.go:311`; `cmd_attach.go:215-221`
**Issue:** Unchanged. The CLI prints "<host> stopped sharing <session>" and exits. In the GUI, WR-06 now makes a row click reconnect, so the GUI recovers, but the message is still false.
**Fix:** Add a distinct close reason such as `"link changed"` that the CLI treats as reconnectable.

### IN-04: Production `internal/daemon/process.go` imports `testing` (still open)

**File:** `internal/daemon/process.go:9,111`
**Issue:** Unchanged. `testing.Testing()` links the `testing` package into the release binary.
**Fix:** This is acceptable as is. Alternatively, use an env var set in `TestMain`.

### IN-05: A remote attach that ends with "session ended" exits 1 with a raw websocket error (still open)

**File:** `cmd_attach.go:215-221`; `main.go:241-244`
**Issue:** Unchanged. `remoteAttachEnd` maps only `CloseReasonShareStopped`.
**Fix:** Also map `relay.CloseReasonSessionEnded` to a fixed message and exit 0.

### IN-06: Two comments now contradict the fixed code

**File:** `internal/daemon/share.go:285`; `cmd_attach.go:58-59`
**Issue:**
- The `unshareLocked` doc still says "Stop closes only the listener (hijacked WebSockets survive it)". Since WR-01, Stop closes every non-hijacked connection. The kick-before-Stop order is still right, but the stated reason is half wrong.
- The `cmdAttach` doc still says "<host>:<session> is always remote". Since WR-09, a local session name containing a colon wins.

**Fix:** Change the first to "Stop closes the listener and plain connections; hijacked WebSockets survive it". Change the second to "<host>:<session> is remote unless a local session has that exact name".

### IN-07: The share server sets no timeouts, and every `/peers/connect` leaves an idle TLS connection open on both Macs

**File:** `internal/webserver/server.go:345`; `internal/daemon/peer_client.go:24-38`, `peer_connect.go:148`
**Issue:** The WR-01 fix built `&http.Server{Handler: ws.mux}` without the `ReadHeaderTimeout` and `IdleTimeout` the iteration-1 fix suggested. `peerClient` builds a new `http.Transport` on every call and never calls `CloseIdleConnections`. After a connect, the keep-alive connection to the peer stays open with read and write goroutines on the viewer, and a served connection on the host. Neither side ever times it out, so these connections pile up until the share server stops or the daemon restarts. A peer on the same tailnet could also hold header reads open indefinitely. The old client did the same, so this is not a regression, but the fix was the natural place to close it.
**Fix:** Add `ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second` to the server, and add `defer client.CloseIdleConnections()` after building the client in `handlePeerConnect`.

### IN-08: Sidebar peer groups and the share-stopped drop are still keyed by HostName (reported by the fixer)

**File:** `frontend/src/App.tsx:808-812` (`dropPeerSession`); `sidebarPeers.ts`
**Issue:** The fixer reported this. WR-05 fixed the connect, but two Macs with the same HostName still share one React group key. A share-stopped close from one Mac also filters that session ID out of both groups' rows until the next poll. Session IDs are random, so in practice only the first part matters.
**Fix:** Key groups and `dropPeerSession` on `dnsName` (falling back to `hostname`).

---

_Reviewed: 2026-11-17T06:35:40Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
