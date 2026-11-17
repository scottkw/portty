---
phase: 04-one-click-share
fixed_at: 2026-11-17T00:00:00Z
review_path: .planning/phases/04-one-click-share/04-REVIEW.md
iteration: 2
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 4: Code Review Fix Report

**Fixed at:** 2026-11-17
**Source review:** .planning/phases/04-one-click-share/04-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 11 (CR-01, CR-02, WR-01..WR-09)
- Fixed: 11
- Skipped: 0
- Info: IN-01 was fixed as part of CR-02 (stale trust-model comments, renamed the client). IN-02..IN-05 are left open.

**Where verification ran:** in the main checkout (`/Users/ken/dev/portty`, branch `gsd/phase-04-one-click-share`), with no worktree, as the orchestrator asked. All of these exit 0 on HEAD `e2dc549a`:
- `go build ./...`, `go vet ./...`, `go vet -tags playwrightfixture ./cmd/playwright-fixture/`, `go vet -tags vmpreview ./...`
- `go test -race -short ./...`
- `pnpm -C frontend exec tsc --noEmit`, `pnpm -C frontend exec vitest run` (82 files, 1038 tests)
- `bash tests/check-traceability-paths.sh`, `bash tests/check-brand.sh`, `bash tests/check-share-removal.sh`
- `pgrep -f '\.test daemon'` prints nothing.

Every new test except WR-03's was checked to fail with the fix reverted.

**STATE.md:** CR-02 resolves the "[Phase 5] The daemon remote WS proxy sets InsecureSkipVerify" concern (STATE.md line 182), so it can be closed. Non-test Go code no longer contains `InsecureSkipVerify`. The client always dials the peer's Tailscale IP from tailscaled's peer list, so a Mac with MagicDNS off also works. The "once an IP fallback exists" condition in that note is covered.

## Fixed Issues

### CR-01: The share removal gate fails on its own header comment

**Files modified:** `tests/check-share-removal.sh`
**Commit:** ee25e7a3
**Applied fix:** I reworded the two header comments so they no longer spell the checked word (`public wr[i]te gate`, `not that gate`). The gate now passes on the committed tree. CI already runs the script as its own build step, so no separate self-test step was added.

### CR-02: The capability fetch and the token-bearing WS dial skip TLS verification and resolve the host through system DNS

**Files modified:** `internal/daemon/peer_client.go` (renamed from `remote_files.go`), `internal/daemon/peer_client_test.go` (new), `internal/daemon/peer_connect.go`, `internal/daemon/remote_caps.go`, `internal/daemon/remote_ws_proxy.go`, `internal/daemon/api.go`, `internal/webserver/server.go`, `internal/daemon/export_test.go`, `internal/daemon/remote_caps_test.go`, `internal/daemon/peer_connect_test.go`, `internal/daemon/remote_ws_proxy_test.go`, `app_test.go`, `TESTING.md`
**Commit:** 5489881d
**Applied fix:**
- **New client:** `peerHTTPClient(fqdn, addr)` dials the peer's Tailscale IP on port 7444, whatever host the URL names. It fully verifies the certificate against the MagicDNS FQDN (`ServerName`, TLS 1.2 minimum). It has no `InsecureSkipVerify`, and a peer with no IP fails rather than falling back to DNS.
- **`/peers/connect`:** uses this client.
- **Cap store:** `RemoteCapStore.Put/Get` now also carries the peer IP, so the WS proxy dials through the same verified transport.
- **Test seam:** `SetRemoteFilesClientForTest` is renamed `SetPeerClientForTest`. Tests still inject an httptest client and never reach tailscaled or a real tailnet address.
- **New test:** `TestPeerHTTPClient_VerifiesCertAndDialsIP` checks three things: a self-signed peer is refused before the handler runs, an unresolvable URL host still reaches the given address (so no DNS lookup happens), and an empty IP fails.
- **IN-01:** the stale "self-signed peer" and "no application-layer authentication" comments are updated.
- **TESTING.md:** Go count is now 110. The stale vitest count of 80 is corrected to 82. A traceability row was added.

### WR-07: The removal gate passes without checking anything when run outside the repo root or when TESTING.md headings change

**Files modified:** `tests/check-share-removal.sh`, `tests/check-brand.sh`, `tests/check-traceability-paths.sh`
**Commit:** b4b69ba7
**Applied fix:**
- **Repo root:** all three gates now `cd` to `git rev-parse --show-toplevel` and exit 2 when not in a git repo. I confirmed they give the same results when run from `tests/` and from `internal/`.
- **share-removal:** exits 2 when TESTING.md is missing or its Sections 4-6 can't be found.
- **traceability:** previously exited 0 ("skipping") when TESTING.md was missing. It now exits 1.
- **Checked:** I tested the missing-file and renamed-heading cases by hand.

### WR-01: `WebServer.Stop` closes only the listener

**Files modified:** `internal/webserver/server.go`, `internal/webserver/share_removed_test.go`
**Commit:** b557e299
**Applied fix:** `Start` now serves through a stored `*http.Server`, and `Stop` calls `srv.Close()`. That closes the listener and every connection that hasn't been taken over by a WebSocket. Test: `TestStop_ClosesKeepAliveConns` (a pooled keep-alive connection gets an error after Stop).

### WR-02: Two first shares started together collide on the fixed port

**Files modified:** `internal/daemon/api.go`, `internal/daemon/share.go`, `internal/daemon/share_test.go`
**Commit:** bb285722
**Applied fix:** A new `startMu` lets only one share-on start the server at a time. Each share-on re-checks `a.webServer` after taking the lock. `a.mu` is still never held during the warm-up. The "lost a race" branch could no longer run, so I removed it. Test: `TestShare_ConcurrentFirstSharesStartOneServer`. With the fix reverted it builds 2 servers; with the fix it builds 1.

### WR-03: A signing-key change during the first share's server start leaves the server on the old key

**Files modified:** `internal/daemon/share.go`
**Commit:** ac7271ef
**Status:** fixed: requires human verification
**Applied fix:** `startShareServer` now calls `ws.SetSigningKey(a.CurrentSigningKey())` again inside the `a.mu` section that installs `a.webServer`. The call before `Start` stays, so `requireCapability` never sees a nil key.
- **Narrower window than reported:** the warm-up runs *before* the first `SetSigningKey`, so a regenerate during the warm-up was already safe. The real window is only `SetSigningKey`, then `Start`, then install, which is a few milliseconds.
- **No test:** no existing seam lands inside that window. A regenerate placed in the warm-up seam passes even without the fix, so I did not add a test that proves nothing. The fix is correct by reading the lock order: regenerate sets the key before it takes `a.mu`, and the install reads the key while holding `a.mu`.

### WR-04: A session that exits or is killed during share-on stays shared, and a failed mint leaves an empty server running

**Files modified:** `internal/daemon/share.go`, `internal/daemon/share_test.go`
**Commit:** ed030881
**Applied fix:** Inside the final `a.mu` section, `shareOn` re-checks `sessionShareable(id)` before minting and returns `not_found` if the session is gone. On that refusal, or when the mint fails, it calls `unshareLocked(id)`. That stops a server this call started for nothing (same order as share-off). Test: `TestShare_SessionKilledDuringStartIsNotShared` (the session is killed in the warm-up seam; with the fix reverted the share returns 200).

### WR-05: The GUI passes the peer's OS HostName, so it can connect to the wrong Mac

**Files modified:** `internal/tailnet/sessions.go`, `app.go`, `internal/daemon/peer_connect.go`, `internal/daemon/peer_connect_test.go`, `frontend/src/wailsjs/go/main/App.d.ts`, `frontend/src/lib/remoteSession.ts`, `frontend/src/lib/remoteAdapter.ts`, `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.open-remote.test.tsx`
**Commit:** 7812ab59
**Applied fix:**
- **New field:** `PeerSessionMetaGroup` and `RemotePeerSessions` now carry `dnsName` (the FQDN, no trailing dot).
- **Matching:** `matchPeer` first tries an exact, case-insensitive FQDN match on `DNSName`. Only then does it try the label and the HostName, so the CLI is unchanged.
- **GUI:** passes `session.peerDNSName || peerHost` to `ConnectPeerSession`.
- **Tests:** Go `TestPeerConnect_FQDNPicksTheRightMac` (two Macs with the same HostName; the FQDN picks `office-air-1`) plus FQDN cases in `TestPeerConnect_HostMatching`. vitest: a peer with `dnsName` is connected by that name.
- **Not regenerated:** `frontend/src/wailsjs/wailsjs/` is the Wails-generated copy and was not touched. It picks up `dnsName` at the next `wails build`.

### WR-06: After the host stops sharing and shares again, clicking the peer row reopens the dead tab

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/TabBar.tsx`, `frontend/src/components/__tests__/App.open-remote.test.tsx`
**Commit:** dc6dcb3d
**Status:** fixed: requires human verification (UI state; needs the two-Mac UAT)
**Applied fix:**
- **Retry on click:** on a `share stopped` close, `dropPeerSession` now also marks the tab `shareStopped: true`. `handleOpenRemoteSession` treats that like `connectFailed`, so the click calls `ConnectPeerSession` again. That replaces the daemon's stored cap and the tab's cap.
- **Remount:** `WebShareSessionView` is now keyed on `sessionId:capToken`, so a new cap remounts the view and dials again. The CR-03 source-grep test was updated to match.
- **Key regeneration:** it uses the same close reason, so it is covered too.
- **Test:** a vitest case where share stops, the next 30 s poll brings the row back, and a click reconnects with `CAP456`. With the fix reverted, connect is called only once.

### WR-08: A failure in the host's own tailscaled looks like "different account"

**Files modified:** `internal/webserver/whois.go`, `internal/webserver/whois_test.go`
**Commit:** 20b7c803
**Applied fix:** The gate still refuses in every failure case. What changed is which answer it gives:

| Situation | Response |
|---|---|
| `StatusWithoutPeers` fails or returns no self | 503, no `X-Portty` marker |
| `WhoIs` fails for any reason other than `local.ErrPeerNotFound` | 503, no marker |
| A valid answer showing another account | 403 plus marker |
| `ErrPeerNotFound` (a real "not one of your nodes" answer) | 403 plus marker |

The viewer already maps a non-200/non-403 answer to unreachable. Test: `TestWhoIs_LocalFaultIs503`. The old `TestWhoIs_MetaRefused` is narrowed to the other-user and peer-not-found cases.

### WR-09: `portty attach` resolution gives misleading results for colon names, typos and non-TTY stdin

**Files modified:** `cmd_attach.go`, `cmd_attach_test.go`
**Commit:** e2dc549a
**Applied fix:**
- **Local first:** `cmdAttach` resolves local sessions before anything else, so a local name containing `:` wins.
- **Empty host:** `:sess` returns the no-session error.
- **Typos:** in the bare-argument fallback, every `PeerConnectError` except `multiple_sessions` maps back to the local "no session named" error. The parameter `onUnknownHost` is renamed `onRefused`.
- **Non-TTY:** `attachRemote` now checks for a terminal after resolving, so a typo or a refusal reports itself and the multi-session listing works without a TTY.
- **Test:** `TestCmdAttach_Resolution` uses the fake tailnet with Tailscale off.

## Left open (out of scope or not changed)

- **IN-02..IN-05:** Info items, not in scope.
- **Duplicate HostNames in the sidebar (related to WR-05):** sidebar groups (`sidebarPeers.ts`) and `dropPeerSession` are still keyed by HostName. Two Macs with the same HostName would share one React key, and a share-stopped drop would remove the row from both groups. The connect itself now picks the right Mac.

---

_Fixed: 2026-11-17_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_

---

# Iteration 2

**Source review:** .planning/phases/04-one-click-share/04-REVIEW.md (iteration 2, commit 495df1f1)
**In scope (coordinator's list):** WR-01, WR-02, IN-06, IN-07. Fixed: 4. Skipped: 0. The frontmatter counts above are for this iteration; iteration 1 was 11 of 11.
**Left as they are, by instruction:** IN-03, IN-05, IN-08 (duplicate-HostName grouping). IN-02 and IN-04 are also still open.

**Where verification ran:** in the main checkout (`/Users/ken/dev/portty`), with no worktree. All of these exit 0 on HEAD `cae3d563`:
- `go build ./...`, `go vet ./...`, `go vet -tags playwrightfixture ./cmd/playwright-fixture/`, `go vet -tags vmpreview ./...`
- `go test -race -short ./...`
- `pnpm -C frontend exec tsc --noEmit`, `pnpm -C frontend exec vitest run` (82 files, 1038 tests)
- the three gates (`check-traceability-paths`, `check-brand`, `check-share-removal`)
- `pgrep -f '\.test daemon'` prints nothing.

No test file was added, so TESTING.md needed no change.

## Fixed Issues

### WR-01: A share-on that gives up can stop the server another share-on just started

**Files modified:** `internal/daemon/share.go`, `internal/daemon/api.go` (field comment)
**Commit:** b494a5c4
**Status:** fixed: requires human verification
**Applied fix:** `shareOn` now holds `startMu` (`defer a.startMu.Unlock()`) from the server check through the session re-check and mint. A share-on that refuses and calls `unshareLocked` can therefore never come between another share-on's "server installed" and "record minted". The lock order is `startMu`, then `a.mu`. Nothing takes `startMu` while holding `a.mu`, and share-off and exit cleanup don't take `startMu` at all.
**Test:** none added. No seam sits between the release of `startMu` and the `a.mu.Lock()` that follows it. Reproducing the race would require the waiting share-on to win `a.mu` against the share-on that just released `startMu`. That would be a timing-dependent test that almost never fails on the old code, so I chose not to add a flaky one. The fix is correct by reading the code: the whole start-to-mint section is now one critical section under `startMu`. The existing WR-02/WR-04 tests from iteration 1 still pass.

### WR-02: Bare `portty attach <host>` on a found Mac said "no session named <host>"

**Files modified:** `cmd_attach.go`, `cmd_attach_test.go`
**Commit:** 6d078421
**Applied fix:** `attachRemote`'s fallback, now called `onNoHost`, replaces only `unknown_host` and `tailscale_off`, the two cases where the argument named no Mac. For not sharing, another account, unreachable and no such session, the found Mac's UI-SPEC message from `peerConnectMessage` is shown again.
**Test:** `TestCmdAttach_BareHostNotSharing` runs bare `portty attach office-air` against a fake peer whose meta returns `[]`. It asserts "office-air isn't sharing any sessions". With the fix reverted it gets "no session named". `TestCmdAttach_Resolution` still covers a typo with Tailscale off.
**Also in this commit:** the IN-06 `cmdAttach` doc fix, because it is the same file.

### IN-06: Two comments contradicted the fixed code

**Files modified:** `internal/daemon/share.go` (commit 7e9c9405), `cmd_attach.go` (in 6d078421)
**Applied fix:**
- The `unshareLocked` comment now reads "Stop closes the listener and plain connections; hijacked WebSockets survive it."
- The `cmdAttach` comment now reads "<host>:<session> is remote unless a local session has that exact name."

### IN-07: No share-server timeouts; every `/peers/connect` left an idle TLS connection open

**Files modified:** `internal/webserver/server.go`, `internal/daemon/peer_connect.go`
**Commit:** cae3d563
**Applied fix:**
- The share server's `http.Server` now sets `ReadHeaderTimeout: 10s` and `IdleTimeout: 60s`. Hijacked WebSockets aren't affected. The plugin-config SSE stream isn't affected either: no `WriteTimeout` is set, and `IdleTimeout` applies only between requests.
- `handlePeerConnect` now calls `defer client.CloseIdleConnections()` on its per-request client, so no keep-alive is left open on either Mac.

---

_Fixed: 2026-11-17_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
