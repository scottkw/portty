---
phase: 04-one-click-share
reviewed: 2026-11-17T06:10:14Z
depth: standard
files_reviewed: 44
files_reviewed_list:
  - .github/workflows/build.yml
  - TESTING.md
  - app.go
  - cmd_attach.go
  - cmd_cli.go
  - cmd_remote.go
  - cmd_share.go
  - main.go
  - tailscale_vmpreview.go
  - testmain_test.go
  - tests/check-brand.sh
  - tests/check-share-removal.sh
  - frontend/src/App.tsx
  - frontend/src/components/SharePopover.tsx
  - frontend/src/components/SessionEndedBanner.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/WebShareSessionView.tsx
  - frontend/src/lib/closeReasons.ts
  - frontend/src/lib/remoteAdapter.ts
  - frontend/src/lib/remoteSession.ts
  - frontend/src/lib/webMode.ts
  - internal/daemon/api.go
  - internal/daemon/client.go
  - internal/daemon/engine.go
  - internal/daemon/peer_connect.go
  - internal/daemon/plugin_settings.go
  - internal/daemon/process.go
  - internal/daemon/remote_caps.go
  - internal/daemon/remote_files.go
  - internal/daemon/remote_ws_proxy.go
  - internal/daemon/share.go
  - internal/daemon/share_vmpreview.go
  - internal/daemon/staticapp.go
  - internal/relay/close_reasons.go
  - internal/relay/hub.go
  - internal/relay/server.go
  - internal/tailnet/sessions.go
  - internal/webserver/capability_mw.go
  - internal/webserver/csp_mw.go
  - internal/webserver/origin_mw.go
  - internal/webserver/plugin_config_stream.go
  - internal/webserver/server.go
  - internal/webserver/whois.go
  - web/link-invalid.html
findings:
  critical: 2
  warning: 9
  info: 5
  total: 16
  necessity: 0
status: issues_found
---

# Phase 4: Code Review Report

**Reviewed:** 2026-11-17T06:10:14Z
**Depth:** standard
**Files Reviewed:** 44 (diffed against b0677a02)
**Status:** issues_found

## Summary

I reviewed the phase 4 source diff, security paths first: the WhoIs same-user gate, the peer capability route and `POST /peers/connect`, share on/off and revocation ordering, the share-server lifecycle, the vmpreview fake, the remote WS close forwarding, `portty attach` resolution, the web link pre-check, the SharePopover and peer-tab flows, and both CI gates. Test files, generated `wailsjs` bindings, CSS and docs got only a light look. They are left out of `files_reviewed_list`, except `testmain_test.go` and `TESTING.md`, which I checked against the standing rule.

What I ran:
- `go build ./...`, `go build -tags vmpreview ./...`, `go vet ./...` and `pnpm -C frontend exec tsc --noEmit` all pass.
- `bash tests/check-brand.sh` passes.
- `bash tests/check-share-removal.sh` **fails on the committed tree** (CR-01).
- `bash tests/check-traceability-paths.sh` passes. Every new test file is listed in TESTING.md.

What holds up:
- **WhoIs gate:** compares numeric user IDs, refuses tagged and shared-in callers and a tagged or shared-in self, and refuses (fails closed) on every error.
- **Peer cap route:** POST plus a required `application/json` with no CORS answer, so a cross-site page cannot read the token. Responses are `Cache-Control: no-store`.
- **Revocation order:** ClearGrants, then DisableSession, then RevokeWebViewers, then Stop. With the re-check after subscribe, this closes the subscribe race.
- **Plugin-config SSE stream:** ends when the grant is revoked.
- **Close reasons:** never rendered as text, and the token never appears in errors or close reasons.
- **vmpreview:** binds 127.0.0.1 only, and its build tag is absent from every release path.

The main problems:
1. The new removal gate turns CI red.
2. The new credential path (`/peers/connect` and the WS proxy) uses `InsecureSkipVerify`, even though every real share server now has a valid Tailscale certificate.
3. The share-server lifecycle has several races: `Stop` leaves keep-alive connections open, two first shares started together collide on the fixed port, a key change during start leaves the server on the old key, and a session that dies during start can stay shared.

## Critical Issues

### CR-01: The share removal gate fails on its own header comment, so CI's build job is red on every push

**File:** `tests/check-share-removal.sh:6,19,60` (wired in `.github/workflows/build.yml:29-30`)
**Issue:** The `PUBLIC` pattern includes `public[_-]?wr[i]te` and is scanned with `-i`. The script's own header comments, at line 6 ("…and the public-write gate") and line 19 ("…is not the public-write gate."), spell `public-write` with no bracket class. The scope excludes `.planning`, TESTING.md, vendor and `*_removed_test.go`, but not the script itself. Running it on the committed tree (HEAD cc3ebb44, script unchanged since then) gives:
```
public-sharing: tests/check-share-removal.sh:6:#   - public internet sharing, its consent UI and the public-write gate
public-sharing: tests/check-share-removal.sh:19:# that happens to be named after the write method is not the public-write gate.
FAIL: 2 finding(s)
```
The header claims "this script never matches its own text", and that is false. The new CI step fails, so the branch-protected build check fails.
**Fix:** Reword both comments so they don't spell the word, as the rest of the script does, or exclude the script from scope:
```bash
#   - public internet sharing, its consent UI and the public wr[i]te gate
...
# that happens to be named after the write method is not that gate.
```
Then add a self-test to CI so a regression shows up when the script changes: `bash tests/check-share-removal.sh && echo ok`.

### CR-02: The capability fetch and the token-bearing WS dial skip TLS verification and resolve the host through system DNS

**File:** `internal/daemon/peer_connect.go:137-138,166`, `internal/daemon/remote_files.go:20-29`, `internal/daemon/remote_ws_proxy.go:91-94`
**Issue:**
- **Where the client is used:** `handlePeerConnect` builds `baseURL` from `p.DNSName` (resolved by the OS resolver) and calls `FetchPeerSessionsMetaWithClient` and `FetchPeerCap` with `a.remoteFilesClient()`. That client sets `InsecureSkipVerify: true`. The WS proxy then dials `wss://<DNSName>:7444/sessions/{id}/ws?cap=<token>` with the same client.
- **The justification no longer holds:** the code says peers use "a self-signed cert", but this phase removed the LAN/self-signed server. `shareOn` now refuses to start without `HasCerts`, so every real share server has a valid Tailscale Let's Encrypt certificate. Only the loopback vmpreview build is self-signed, and it has no peers.
- **The sidebar path already verifies:** `tailnet.FetchPeerSessionsMeta` (`internal/tailnet/sessions.go:101-131`) checks the certificate. So the listing is verified but the step that grants control is not.
- **The attack:** if MagicDNS is off on the viewing Mac, or its resolver is spoofed (for example on café Wi-Fi), `office-air.tailXXXX.ts.net` can resolve to an attacker. The attacker answers the meta and cap calls with any session and token, and the viewer then streams keystrokes into a fake "office" shell, including sudo or SSH passwords. This weakens the project constraint "Remote access only over the user's tailnet".

**Fix:** Verify the certificate and skip DNS. Dial the peer's Tailscale IP, with SNI and verification against the FQDN (the same pattern `FetchPeerSessionsMeta` uses):
```go
func peerHTTPClient(p tailnet.Peer) *http.Client {
    fqdn := strings.TrimSuffix(p.DNSName, ".")
    ip := p.TailscaleIPs[0]
    d := &net.Dialer{Timeout: 10 * time.Second}
    return &http.Client{
        Timeout: 10 * time.Second,
        Transport: &http.Transport{
            DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
                return d.DialContext(ctx, network, net.JoinHostPort(ip, strconv.Itoa(tailnet.DefaultProbePort)))
            },
            TLSClientConfig: &tls.Config{ServerName: fqdn, MinVersion: tls.VersionTLS12},
        },
    }
}
```
Store the peer IP (or the client) next to the cap in `RemoteCapStore` so the WS proxy uses the same verified transport. Delete `InsecureSkipVerify` from `remote_files.go`, and keep a test-only override for httptest.

## Warnings

### WR-01: `WebServer.Stop` closes only the listener, so the stopped server keeps answering on keep-alive connections

**File:** `internal/webserver/server.go:348,371-381`; called from `internal/daemon/share.go:288-292`
**Issue:**
- **What Stop does:** `Start` runs `go http.Serve(ln, ws.mux)`, and `Stop` only calls `ln.Close()`. Closing the listener ends the accept loop, but every open HTTP/1.1 keep-alive connection is still served by the old mux. The server has no `IdleTimeout` and no HTTP/2, so browsers keep pooled sockets for minutes.
- **Re-share shows a valid link as dead:** after Share off then on, the daemon creates a new server S2 with a new token. A browser that still holds an idle pooled connection to the same origin sends the new link over the socket owned by the old server S1. S1 had `ClearGrants` and never receives the new grant, so `requireCapabilityPage` shows "Link No Longer Works" for a valid link.
- **The old server keeps running:** its goroutines and memory stay alive, and its closures over `a.shares` keep answering `requireSameUser` routes after "nothing is shared".

**Fix:** Keep an `*http.Server` and close its connections on Stop:
```go
srv := &http.Server{Handler: ws.mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
ws.srv = srv
go srv.Serve(ln)
...
func (ws *WebServer) Stop() error { /* snapshot */ return srv.Close() } // closes listener + idle/active non-hijacked conns
```
Hijacked WebSockets survive `Close`, so the kick-before-Stop order is still needed.

### WR-02: Two first shares started together collide on the fixed port, and the second fails with "port 7444 is in use"

**File:** `internal/daemon/share.go:151-156,222-234`
**Issue:**
- **Both start a server:** `shareOn` reads `a.webServer == nil` under RLock, then calls `startShareServer` without a lock. Two share-ons that overlap during the health check plus the cert warm-up (up to 25 s) both build and `Start` a server on the same fixed `tailnet.DefaultProbePort`.
- **The loser reports an error:** the second `Start` fails with `ErrPortInUse`, and the user gets `port_in_use` for session B even though the share server is running. Ways to hit it: sidebar "Share" on two rows, `portty share a & portty share b`, or the popover plus the CLI.
- **Dead code:** the "lost a race to another share-on" branch at line 230 can only run when two binds succeed, which a fixed port prevents. In production it never runs.

**Fix:** Serialize server start with a dedicated `startMu sync.Mutex` (or `singleflight`), and re-check `a.webServer` after taking it:
```go
a.startMu.Lock()
defer a.startMu.Unlock()
a.mu.RLock(); ws := a.webServer; a.mu.RUnlock()
if ws == nil { if serr := a.startShareServer(...); serr != nil { return ..., serr } }
```
Do not hold `a.mu` across warm-up. `startMu` is separate from `a.mu`.

### WR-03: A signing-key change during the first share's server start leaves the server on the old key, so the link never verifies

**File:** `internal/daemon/share.go:220,229-243,162`; `internal/daemon/api.go:648-657`
**Issue:**
- **Old key installed:** `startShareServer` calls `ws.SetSigningKey(a.CurrentSigningKey())` before `Start` and before installing `a.webServer`.
- **Regeneration misses the new server:** if `POST /capability/regenerate-key` runs in that window (the 20 s cert warm-up makes the window real), it sees `a.webServer == nil` and does not update or re-mint.
- **Mismatched token:** `shareOn` then mints with `a.CurrentSigningKey()`, which is the new key, while the server verifies with the old key. Every request with the link, and every peer connect, fails with 401 or "Link No Longer Works" until sharing is toggled off and on.

**Fix:** Set the key at install time, under `a.mu`, in the same critical section that sets `a.webServer`:
```go
a.mu.Lock()
...
a.webServer = ws
ws.SetSigningKey(a.CurrentSigningKey())
```

### WR-04: A session that exits or is killed during share-on stays shared, and a failed mint leaves an empty server running

**File:** `internal/daemon/share.go:127,158-166`
**Issue:**
- **No re-check before minting:** `sessionShareable` is checked once, before a health check and warm-up that can last up to 25 s. If the session is killed in that window, `handleDeleteSession` runs `runSessionExitCleanup`, which is a no-op because there is no record yet. A natural exit's 10 s cleanup can also fire first. `shareOn` then mints a record for a dead session. Nothing removes it later, so the share server runs until the daemon restarts, and the sidebar and meta keep listing the session.
- **Failed mint keeps the server:** if `mintShareRecord` fails after `startShareServer` installed a server, the server stays up with zero shares.

**Fix:** Inside the final `a.mu.Lock()` section, re-check `a.sessionShareable(id)` before minting and return `ShareErrNotFound` on failure. If the mint fails or the session is gone and `len(a.shares) == 0`, stop the server, as `unshareLocked` does.

### WR-05: The GUI passes the peer's OS HostName, but `matchPeer` matches MagicDNS labels first, so it can connect to the wrong Mac

**File:** `internal/daemon/peer_connect.go:54-66`; `frontend/src/App.tsx:787-790`; `app.go:971-1000`
**Issue:** Sidebar groups are keyed by `tailnet.Peer.Hostname`, and `handleOpenRemoteSession` sends `session.hostname` to `ConnectPeerSession`. `matchPeer` first compares the host to every peer's MagicDNS label and only then to HostName. MagicDNS de-duplicates labels: two machines whose HostName is `macbook-air` get the labels `macbook-air` and `macbook-air-1`, and admins can rename labels. Clicking the second machine's row then sends "macbook-air", which matches the first machine's label. The daemon asks the wrong Mac and reports `no_such_session`. The CLI is fine, because users type the label.
**Fix:** Give the GUI an unambiguous key. Return the peer's DNS name (or node ID or Tailscale IP) in `RemotePeerSessions`, and have `ConnectPeerSession` match on it exactly. Alternatively, have `matchPeer` accept a full FQDN and match it against `DNSName` before the label pass.

### WR-06: After the host stops sharing and shares again, clicking the peer row reopens the dead tab instead of reconnecting

**File:** `frontend/src/App.tsx:781-786,807-811`; `frontend/src/components/TerminalPanel.tsx:384-388`
**Issue:** On a `share stopped` close, the tab stays open (stdin off, banner shown) and `dropPeerSession` removes the row. When the host shares again, the next poll brings the row back. `handleOpenRemoteSession` retries only tabs with `connectFailed`. An ended tab has `connectFailed` false, so the click just activates the dead tab. The daemon also still holds the old, revoked cap. The user has to close the tab manually to reconnect. Key regeneration (`remintSharesLocked`) produces the same state, because it kicks viewers with `share stopped` while the share stays on.
**Fix:** Record the ended or share-stopped state on the Tab (for example, set `connectFailed: true` or `ended: true` from `onShareStopped`), and treat it as retryable in `handleOpenRemoteSession`, so the click calls `ConnectPeerSession` again and replaces the tab's cap.

### WR-07: The removal gate passes without checking anything when run outside the repo root or when TESTING.md headings change

**File:** `tests/check-share-removal.sh:31,87-100,103-106`
**Issue:** The script never runs `cd` to the top level.
- **From a subdirectory:** `git grep` pathspecs scope to the current directory, the `[[ -e path ]]` removed-file checks resolve relative to it and always pass, and `[[ -f TESTING.md ]]` is false, so the TESTING.md scan is skipped. Running from `tests/` scans only `tests/`.
- **Root-relative exclusion breaks:** the `':!tests/fixtures/...'` exclusion is root-relative, and from `tests/` it flags the fixture. I confirmed this.
- **TESTING.md section scan:** if TESTING.md is renamed or missing, or its `## 4.` or `## 6.` heading changes, `awk` prints nothing and the section scan passes.

CI runs from the root, so this does not mask anything today. It does answer "can it pass silently?" with yes.
**Fix:**
```bash
cd "$(git rev-parse --show-toplevel)" || { echo "ERROR: not a git repo"; exit 2; }
[[ -f TESTING.md ]] || { echo "ERROR: TESTING.md missing"; exit 2; }
section=$(awk '/^## 4\./,/^## 6\./' TESTING.md)
[[ -n "$section" ]] || { echo "ERROR: TESTING.md sections 4-6 not found"; exit 2; }
```
Apply the same `cd` to `tests/check-brand.sh`, whose vm-tag check uses relative pathspecs too.

### WR-08: A failure in the host's own tailscaled looks like "different account", so the peer disappears and the CLI blames the account

**File:** `internal/webserver/whois.go:68-84`; consumers `internal/tailnet/sessions.go:79-81`, `internal/daemon/peer_connect.go:143-145`
**Issue:** Refusing on every error is correct. But when the host's own `StatusWithoutPeers` or `WhoIs` call fails (tailscaled restarting, the 3 s timeout, a LocalAPI hiccup), the gate sends the same `403` plus `X-Portty: 1` it uses for another account. The viewer maps that to `MetaForbidden`, which drops the peer from the sidebar entirely, and to `forbidden` in `/peers/connect`, which prints "didn't allow access… same Tailscale account". A transient fault on the host looks like a permanent account mismatch.
**Fix:** Keep refusing, but separate local faults from mismatches. Return `503` without the marker when `StatusWithoutPeers` or `WhoIs` errors. Reserve `403` plus marker for `!sameUser(...)` with a valid answer. The viewer already treats non-200/non-403 as `MetaUnreachable`.

### WR-09: `portty attach` resolution gives misleading results for colon names, typos and non-TTY stdin

**File:** `cmd_attach.go:88-108,129-148`; `cmd_remote.go:17-23`
**Issue:**
- **Colon names:** `parseRemoteID` treats any `:` as `host:session`, so a local session whose name contains `:` can never be attached by name. `portty share` and `unshare` resolve the same name fine. `:sess` sends an empty host and gets a raw `400 invalid request body` error.
- **Typos:** a bare argument that isn't a local session falls through to `attachRemote`. Only `unknown_host` is mapped back to the local "no session named" error. With Tailscale off, a typo prints "Tailscale isn't connected…". Other peer errors (unreachable, forbidden) also replace the local error, and every typo waits on a tailnet probe.
- **Non-TTY stdin:** `attachRemote` checks the terminal before resolving, so a typo from a non-TTY reports "needs an interactive terminal".

**Fix:** In `cmdAttach`, try local resolution first even when the argument contains `:` (an exact name match wins). Reject an empty host. In the bare-argument fallback, map every `PeerConnectError` except `multiple_sessions` and successful resolution back to `nse`.

## Info

### IN-01: Stale security comments now describe a trust model that no longer exists

**File:** `internal/webserver/server.go:63-65`, `internal/daemon/remote_files.go:3-6,15-19`, `internal/daemon/remote_caps.go:58`, `internal/daemon/remote_ws_proxy.go:15,86-89`
**Issue:** The `WebServer` doc says "no application-layer authentication is required", but the server has capabilities plus WhoIs. Several comments justify `InsecureSkipVerify` with self-signed peers, which no longer exist (see CR-02). `remoteFilesClient` and `newRemoteFilesHTTPClient` are named after the removed file browser.
**Fix:** Update the comments and rename to `peerHTTPClient` together with the CR-02 change.

### IN-02: The vmpreview tag gate covers only `.github`, `build.sh` and `scripts`

**File:** `tests/check-brand.sh:62-64`; `internal/daemon/share_vmpreview.go`, `tailscale_vmpreview.go`
**Issue:** I confirmed today's release paths (`release.yml:92`, `build.yml:78`, `build.sh:79`) use only `-tags wailsassets`, and the fake binds loopback only. The gate would miss the tag in `wails.json`, a Makefile or Taskfile, or `GOFLAGS` set in a workflow env block. `.github` covers the last case only if it is spelled out there. Nothing checks at runtime either.
**Fix:** Widen the pathspec (for example `-- .github build.sh scripts wails.json Makefile '*.mk' Taskfile*`). Optionally have the vmpreview `init()` print a loud stderr banner.

### IN-03: Key regeneration closes viewers with "share stopped" while the share stays on

**File:** `internal/daemon/share.go:296-311`; `cmd_attach.go:207-214`
**Issue:** `remintSharesLocked` kicks with `relay.CloseReasonShareStopped`. The CLI then prints "Portty: <host> stopped sharing <session>." and exits 0, and the GUI drops the peer row (then WR-06 applies). The message is false.
**Fix:** Add a distinct reason (for example `"link changed"`) and treat it as reconnectable: run peer connect again, which fetches the new cap.

### IN-04: Production `internal/daemon/process.go` imports `testing`

**File:** `internal/daemon/process.go:111`
**Issue:** `testing.Testing()` (Go 1.21+) works, but it links the `testing` package into the release binary just to guard against spawning a daemon from a test.
**Fix:** This is acceptable as is. A cheaper option is a `//go:build`-free check such as an env var set in `TestMain`, or keeping the guard and accepting the cost.

### IN-05: A remote attach ending with "session ended" exits 1 with a raw websocket error

**File:** `cmd_attach.go:312-323`; `main.go:241-244`
**Issue:** Only `share stopped` gets the clean exit. When the host ends the session, `AttachSession` returns the `CloseError`. `main` then prints `failed to get reader: received close frame: status = StatusNormalClosure and reason = "session ended"` and exits 1. The local attach path behaved this way before this phase, but the new remote path now shows it as well.
**Fix:** In `remoteAttachEnd` (and for local attach), also map `relay.CloseReasonSessionEnded` to a fixed message and exit 0.

---

_Reviewed: 2026-11-17T06:10:14Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
