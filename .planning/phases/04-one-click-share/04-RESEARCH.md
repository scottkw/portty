# Phase 4: One-Click Share - Research

**Researched:** 2026-11-16
**Domain:** Go daemon + webserver capability model, Tailscale LocalAPI WhoIs, Wails/React share UI, CLI remote attach. Mostly removal, with one new access path (same-user WhoIs) and one new state (a per-session share record).
**Confidence:** HIGH for the code inventory and the Tailscale API (read from source this session). MEDIUM for the design recommendations. LOW only where tagged [ASSUMED].

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Who can use a share
- Peer Macs get access automatically when Tailscale WhoIs (via the local tailscaled client, `tailscale.com/client/local`) shows the requesting node is logged in as the same tailnet user as this Mac. No join code. Portty still mints a capability behind the scenes, so the capability-gated model stays intact (only issuance is automated).
- The web link carries the full-control capability token. Anyone on the tailnet who has the link can use it (treat it like a password); the server stays tailnet-only.
- Turning Share off revokes every capability for that session and disconnects all viewers (peer GUI, browser, CLI). Turning it back on mints a new link.
- Share state persists across daemon and app restarts.

#### The Share control
- The toolbar Share button opens a small popover with one "Share this session" switch. The sidebar row context menu also has "Share" / "Stop Sharing".
- When shared, the popover shows the link in one field with a Copy button and one line: "Anyone on your tailnet with this link has full control."
- Join codes, the QR code, and the "Open" button are removed (radical simplicity; same-user access replaces join codes).
- The shared indicator stays as today: the signal icon on the sidebar row and the blue "Shared" toolbar state.

#### Cleanup and the no-Tailscale case
- Remove the password-gated LAN web server fallback (Phase 1 review WR-01/WR-02: `internal/daemon/process.go` LAN auto-start, `app.go` StartWebServer LAN fallback). With Tailscale off, Share is dimmed with its reason, as now.
- Settings ▸ Sharing shrinks to Tailscale status plus a short explanation. The web server starts automatically the first time a session is shared (and stops when nothing is shared, at Claude's discretion).
- Remove the unused public-write consent-gate state (`SetRWGate` / `isRWGated`) with Funnel.
- Remove CLI `serve` / `web start`. Add `portty share <session>` and `portty unshare <session>`, which flip the same switch.
- Remove all Funnel, public-link, risk-panel, hold-to-confirm, read-only link code, UI, settings and tests (SHAR-02, SHAR-05).

#### portty attach from any terminal
- Target syntax `portty attach <host>:<session>`: host is the tailnet machine's MagicDNS short name; session is the session name, or an ID prefix when names clash.
- Bare `portty attach <host>` lists that Mac's shared sessions and attaches straight away if exactly one is shared.
- CLI access uses the same automatic same-user WhoIs check as the GUI; no code or token to paste.
- When the host stops sharing mid-attach: restore the terminal, remove the status bar, print "Portty: <host> stopped sharing <session>." and exit 0.

### Claude's Discretion
- Whether the web server stops when the last share ends; exact popover layout within Phase 2/3 tokens; how capabilities are persisted and revoked; handling of a non-same-user tailnet peer (not listed / not authorized); session naming collisions; test strategy for WhoIs (fake local client).

### Deferred Ideas (OUT OF SCOPE)
- Phase 2 UAT items 2-5 and Phase 3 M-70/M-73/M-74 (need both Macs, a signed build, a public repo).
- Restored tabs replaying the old first-prompt % mark (261116-ku9 follow-up).
- Phase 5: accelerated idle tests instead of a real overnight wait.

### Already decided in 04-UI-SPEC.md "Decisions Made Here" (approved; the planner treats these as settled unless the user overrides)
- The web server starts on the first share and stops when no session is shared. No UI shows it.
- A peer signed in to a different account is not listed. The CLI prints a clear refusal.
- The one-time share warning banner and its Settings switch are removed.
- The CT disclosure is kept but does not block: it shows in the off popover until the first share, and that share records the acknowledgement (the CLI does too).
- A peer tab that loses its share keeps its scrollback and shows an info banner. It does not close itself.
- Bare `portty attach <host>` with several shared sessions lists them and exits 0.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SHAR-01 | One on/off state per session. Shared means both: visible to tailnet peers and reachable through a tailnet-only web link | Daemon share record (§Pattern 1), `PUT /sessions/{id}/share`, WhoIs-gated `GET /api/sessions/meta` (§Pattern 2), popover (UI-SPEC) |
| SHAR-02 | A share always grants full control. Read-only link type and UI removed. Links stay token-gated and tailnet-only | Mint only `"read,write"` (§Pattern 1). Removal inventory rows R-RO-* |
| SHAR-03 | The web link opens a working interactive terminal in a browser on the tailnet | Existing path `/sessions/{id}?cap=` → 302 `/app/?session&cap` → WS with Origin = BaseURL (server.go:849-859, 1184-1192). No change needed beyond auto-start. "Link No Longer Works" page (§Pattern 6) |
| SHAR-04 | Stopping a share disconnects remote viewers and removes the session from peers' sidebars | `ClearGrants` + revoke with fixed close reason + re-check after subscribe (§Pattern 3). Proxy forwards the close reason (§Pattern 4) |
| SHAR-05 | Funnel removed: risk panel, consent gates, hold-to-confirm, public links, Funnel config and their tests | Removal inventory (§Removal Inventory) |
| SHAR-06 | `portty attach <host>:<session>` from any terminal, with the status bar | Daemon `POST /peers/connect` + relay proxy (§Pattern 5). CLI copy in UI-SPEC |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

From `/Users/ken/dev/CLAUDE.md`, `/Users/ken/dev/portty/CLAUDE.md` and `/Users/ken/dev/portty/.claude/CLAUDE.md`:

- **TESTING.md standing rule.** Every plan that adds, renames or removes tests updates TESTING.md: Section 2 manifest counts, Section 4 traceability rows (path column is a repo-relative `.go/.ts/.tsx/.sh` path only), and Section 5 manual items for things that can't be automated. Run `bash tests/check-traceability-paths.sh` before committing. It fails on any path that no longer exists, so **a plan that deletes a test file must delete its Section 4 rows in the same commit**.
- **Chesterton's Fence.** Read each piece and know why it exists before removing it, especially daemon, relay and capability code that kept and removed features share. The "Shared with kept features" column in the removal inventory exists for this.
- **Security constraint.** Keep the capability-gated share model, the CSP and the consent gates intact through the refactor. (The Funnel consent gates are removed by decision. The capability gate and CSP stay.)
- **Tech stack.** Go/Wails/React/tsnet stays. Strip-down, not rewrite. No new UI libraries (UI-SPEC).
- **Silent fallbacks are forbidden.** `or {}` turns a hard failure into silent corruption. Let it crash. This applies to WhoIs errors (fail closed) and to the random-port fallback (see Pitfall 6).
- **Go conventions.** `go fmt`, context-aware functions. JS/TS: camelCase, TypeScript types.
- **Testing.** 80%+ coverage in critical components, plus integration tests for routes.
- **Workflow.** Work goes through GSD. Native-GUI checks run in the Tart VM `portty-test` (memory). The VM has no Tailscale, so tailnet flows need fakes plus the deferred two-Mac UAT.
- **Ponytail** is active for executor agents: smallest change that fully solves the task.
- **Memory: radical simplicity.** Default to cutting AgentHub features.

## Summary

Most of Phase 4 is deletion. Funnel (about 1,200 lines of daemon and webserver code plus 2,000 lines of tests), join codes, QR, the LAN fallback server, the RW gate and the three-tab share modal all go. What replaces them is small: one in-memory share record per session in the daemon (`grantID` + signed token), a WhoIs middleware on the webserver that admits only nodes logged in as the same Tailscale user, and one daemon route that fetches a peer's capability on behalf of the GUI and the CLI.

The capability model needs no new primitive. The webserver already checks every token against a per-session grant set (`isGrantActive`, server.go:260-268) and a web-enabled set (`IsSessionEnabled`). `ClearGrants` already revokes every token for a session, and a fresh grant ID on the next share already makes the old link dead. No epoch or generation counter is needed. Three things are missing today and must be added. (1) Live WebSockets are never cut when grants clear, because `requireCapability` runs only at upgrade time. `DisconnectWebViewers` closes them, but with code 1008 `"too slow"` (hub.go:197-210 reuses `CloseSlow`), so no client can tell "share stopped" from "slow consumer". (2) A viewer that passes the cap check just before revocation subscribes just after it and stays connected (TOCTOU). (3) The daemon's remote WS proxy drops the upstream close reason, so a peer tab can't show "stopped sharing".

Two existing remote paths are broken today and get fixed by this phase as a side effect. `portty attach host:id` dials the peer's WS with no `?cap=` and no `Origin` header, so it gets 401/403 every time (cmd_attach.go:231, origin_mw.go:44-50). `portty list` and `App.GetRemoteSessions` call the cap-gated `/api/sessions` with no cap, so they list nothing (sessions.go:83-113).

**Primary recommendation:** Keep the webserver's grant model unchanged and put the share record in the daemon (`API`). Gate peer discovery and peer capability issuance with a `requireSameUser` WhoIs middleware behind an injectable `whoIsClient` seam. Route GUI and CLI remote access through one daemon route (`POST /peers/connect`) and the existing relay proxy. Revoke with a fixed close reason (`"share stopped"`) and re-check the grant after subscribe.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Share on/off state, link token, web-server lifecycle | Daemon (`internal/daemon/api.go`) | — | The daemon outlives the GUI and owns the PTYs. GUI, CLI and sidebar menu all flip the same daemon state (UI-SPEC Interaction Contract) |
| Capability verification, grant set, revoke-on-connect | Webserver (`internal/webserver`) | Capability pkg (Sign/Verify only) | Unchanged model: `requireCapability` + `isGrantActive` + `IsSessionEnabled` |
| Same-user check (WhoIs) | Webserver middleware on the **host** Mac | tailscaled LocalAPI | Only the host can trust the source address of an inbound tailnet connection |
| Peer capability fetch + storage | Daemon on the **viewing** Mac (`RemoteCapStore`) | — | The relay proxy that uses the cap lives in the daemon, and the CLI already talks to the daemon. One path serves GUI and CLI |
| Live disconnect with reason | Relay hub (`internal/relay/hub.go`) | Webserver WS handler | Hub tracks subscribers by Origin. The WS handler owns the close call |
| Close-reason forwarding | Daemon remote WS proxy | — | The proxy sits between the peer's webserver and the local webview/CLI |
| Peer list in sidebar | App process (Go, `app.go`) | Host webserver `GET /api/sessions/meta` | Existing 30 s poll in App.tsx:786-810 |
| Share popover, menu items, banners | Browser/Wails webview (React) | — | Display only. Branches on fixed close-reason constants, never wire text |
| `portty share/unshare/attach` | CLI (main package) | Daemon socket | Thin clients of the daemon routes |

## Standard Stack

No new dependencies. One dependency leaves.

### Core (already pinned, verified in go.mod this session)
| Library | Version | Purpose | Notes |
|---------|---------|---------|-------|
| `tailscale.com` | v1.98.3 | `client/local.Client` WhoIs, StatusWithoutPeers, GetCertificate, CertPair | [VERIFIED: go.mod line `tailscale.com v1.98.3`; API read from `/Users/ken/go/pkg/mod/tailscale.com@v1.98.3/client/local/local.go:319-361`] |
| `github.com/coder/websocket` | v1.8.15 | WS server/client, `CloseError{Code, Reason}` | [VERIFIED: go.mod; `close.go:65-68` `type CloseError struct { Code StatusCode; Reason string }`] |
| Go | go.mod `go 1.26.4`; local toolchain go1.27.4 | — | [VERIFIED: `go version`] |

### Removed
| Library | Why it can go | Verification |
|---------|---------------|--------------|
| `github.com/skip2/go-qrcode` | Only importers are `app.go:19`, `cmd_cli.go:17`, `internal/webserver/server.go:25`, all removed by this phase | [VERIFIED: `git grep -l "go-qrcode\|qrcode\." -- '*.go'` returns exactly those three files]. After removal, `go mod tidy` drops it from go.mod and go.sum (go.sum lines 168-169). Run `go mod tidy` and check `git diff go.mod` shows only that removal (T-03-SC precedent: no other module churn) |

`tailscale.com/ipn` (imported at server.go:27 for Funnel's `ipn.ServeConfig`/`CheckFunnelAccess`) leaves the webserver package's import list but stays in the module graph.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| WhoIs via `local.Client` | `tsnet.Server` embedded node | Would give Portty its own node identity, but the project uses the system tailscaled everywhere today. Rewrite, not strip-down. Rejected |
| Comparing `UserProfile.ID` | Comparing `LoginName` strings | LoginName is "for display purposes only" per tailcfg.go:287. Use the numeric `UserID` |
| One token per share (link token reused for peers) | A fresh grant per peer request | Per-peer grants add map growth and no revocation benefit (share-off clears all anyway). One token per share is simpler. See Pattern 1 |

**Installation:** none. `go mod tidy` after removing the qrcode imports.

## Package Legitimacy Audit

This phase installs no external packages (Go or npm). It removes `github.com/skip2/go-qrcode`. The legitimacy gate does not apply.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                       HOST MAC (office-air)                                     VIEWING MAC (home-air)
 ┌───────────────────────────────────────────────────────┐        ┌────────────────────────────────────────────────┐
 │ Share switch / menu / `portty share`                  │        │ Sidebar poll (App.go, 30 s)                    │
 │        │  PUT /sessions/{id}/share {shared:true}      │        │   GET https://office-air:7444/api/sessions/meta│
 │        ▼  (daemon unix socket)                        │        │        │                                       │
 │ Daemon API ── health ok? certs on? ──no──► typed error │        │        │                                       │
 │        │yes                                           │        │        ▼                                       │
 │        ├─ web server nil? → start on 100.x:7444       │◄───────┼── requireSameUser(WhoIs RemoteAddr)            │
 │        ├─ mint grant + "read,write" token             │  tailnet│     200 [shared sessions] | 403 other user     │
 │        ├─ ws.EnableSession + ws.AddGrant              │        │                                                │
 │        └─ return link https://fqdn:7444/sessions/id?cap│        │ Click peer row / `portty attach h:s`           │
 │                                                       │        │   → local daemon POST /peers/connect {host,sid}│
 │ Webserver (100.x:7444, LE cert via tailscaled)        │        │        │ resolve host in tailscale Status       │
 │   GET  /api/sessions          (marker, cap)           │        │        ▼                                       │
 │   GET  /api/sessions/meta     (requireSameUser)       │◄───────┼── POST /api/peer/sessions/{sid}/cap            │
 │   POST /api/peer/sessions/{id}/cap (requireSameUser)  │        │     → {cap}  → RemoteCapStore.Put(sid,base,cap)│
 │   GET  /sessions/{id}?cap → 302 /app/ (HTML 401/403)  │        │                                                │
 │   GET  /sessions/{id}/ws  (origin + cap) ─────────────┼──┐     │ GUI tab / CLI → ws://127.0.0.1:relay           │
 │        │ subscribe, re-check grant                    │  │     │   /api/relay/remote/{sid}/ws  (proxy)          │
 │        ▼                                              │  └─────┼── wss://office-air:7444/sessions/{sid}/ws?cap  │
 │ Relay Hub (subscribers: local | web)                  │        │     Origin: https://fqdn:7444                  │
 │                                                       │        │                                                │
 │ Share off: ClearGrants → DisableSession →             │        │ Proxy forwards close (code, "share stopped")   │
 │   hub.RevokeWebViewers("share stopped") → if none     │───────►│   → tab banner "<host> stopped sharing…"       │
 │   shared: stop web server                             │        │   → CLI prints locked line, exit 0             │
 └───────────────────────────────────────────────────────┘        └────────────────────────────────────────────────┘
        Browser on any tailnet device: link → /app/ SPA → wss …/ws?cap (Origin = page origin = BaseURL)
```

### Recommended file layout (changes only)
```
internal/webserver/
├── whois.go            # NEW: whoIsClient seam, requireSameUser middleware, sameUser()
├── whois_test.go       # NEW: fake client table tests (tagged, shared-in, other user, error)
├── server.go           # minus Funnel/join/QR/local mode; plus peer-cap route, revoke re-check
├── capability_mw.go    # unchanged logic; HTML variant for GET /sessions/{id}
├── funnel_client.go    # DELETE
├── auth.go, localip.go # DELETE (LAN fallback only)
internal/daemon/
├── share.go            # NEW: share registry, PUT/GET /sessions/{id}/share, web-server start/stop
├── peer_connect.go     # NEW: POST /peers/connect (resolve host, fetch cap, RemoteCapStore.Put)
├── remote_ws_proxy.go  # forward upstream close code + reason
├── api.go              # minus Funnel/join/web-serve/capabilities/webserver routes
internal/tailnet/
├── sessions.go         # FetchPeerSessionsMeta returns tri-state (ok | forbidden | unreachable); FetchPeerCap
internal/relay/hub.go   # RevokeWebViewers(reason) via a per-subscriber Kick func
cmd_share.go            # NEW: portty share / unshare
cmd_attach.go           # host[:session] resolution, daemon connect, revoke message
frontend/src/components/SharePopover.tsx   # NEW (replaces Share/ and SessionShare/)
```

### Pattern 1: Daemon-owned share record, webserver grant model unchanged

**What:** `API` gains `shares map[string]shareRecord` (guarded by `a.mu`), where `shareRecord{grantID, token string}`. It is the single source of truth for "is this session shared" and "what is its link". The webserver keeps `webEnabled` + `grants` as its enforcement cache. Every time a WebServer is created, the daemon replays `EnableSession` + `AddGrant` for each record.

**Why replay instead of a callback:** `requireCapability` (capability_mw.go:37-80) checks `isGrantActive` and `IsSessionEnabled`. Keeping those calls unchanged keeps the capability middleware and its fuzz target (`FuzzRequireCapability`) untouched. The webserver is recreated on every start (stop-when-none-shared), so the record cannot live in it: `handleWebServerStart` today builds a fresh `WebServer` and loses every grant (api.go:984-1037).

**Token:** perms are always `"read,write"`. Claims field order is load-bearing (capability.go:15-25). Reuse the existing mint shape from api.go:1194-1204:

```go
// Source: internal/daemon/api.go:1177-1204 (existing mint), reduced to one token
var gid [16]byte
if _, err := rand.Read(gid[:]); err != nil { return shareRecord{}, err }
claims := capability.Claims{SID: sessionID, Perms: "read,write", IAT: time.Now().Unix(),
    GrantID: hex.EncodeToString(gid[:]), V: 1}
tok, err := capability.Sign(claims, key)
```

The verbatim existing values: `rPerms := "read"` / `wPerms := "read,write"` (api.go:1196-1197) and `capability.Claims{SID: sessionID, Perms: wPerms, IAT: now, GrantID: hex.EncodeToString(wgid[:]), V: 1}` (api.go:1199). [VERIFIED: api.go:1196-1199]

**Peers use the same token.** `POST /api/peer/sessions/{id}/cap` returns the share's token. Share-off clears the grant, so the peer's stored cap dies with the link. This satisfies "Portty still mints a capability behind the scenes".

**Share on (daemon, `PUT /sessions/{id}/share {"shared":true}`):**
1. Session exists and is running, else 404.
2. `webserver.CheckHealth`: `!Connected` → 503 code `tailscale_off`. `!HasCerts` → 409 code `https_off`. Map these to the UI-SPEC copy in the Go binding, not by parsing text in React.
3. Already shared → return the existing link (idempotent, so the UI-SPEC rule "the link is the same while shared" holds).
4. Web server not running → create it on `h.IP:7444` with `FQDN: h.Domain`. Wire resolver, plugin provider, signing key, static FS. Replay records.
5. Optional cert warm-up: `lc.CertPair(ctx, h.Domain)` with about a 20 s timeout, so a certificate failure shows up as "Couldn't start sharing: <detail>" while the popover reads "Getting a link…", not later as a browser TLS error. The daemon client timeout is 30 s (client.go:36). [VERIFIED: `local.Client.CertPair(ctx, domain)` at cert.go:56]
6. Mint the record, `EnableSession`, `AddGrant`, store. On first success write the CT acknowledgement file (see Pitfall 10).
7. Return `{shared:true, url: BaseURL()+"/sessions/"+id+"?cap="+token}`.

**Share off (`{"shared":false}`), and also session exit and kill:**
1. Delete the record. `ws.ClearGrants(id)` **then** `ws.DisableSession(id)`.
2. `hub.RevokeWebViewers(relay.CloseReasonShareStopped)` (Pattern 3).
3. If no records remain, `ws.Stop()` and set `a.webServer = nil`.
4. Not shared → 200 with `{shared:false}` so the CLI prints `"<name>" isn't shared.` (exit 0).

**Persistence ("Share state persists across daemon and app restarts").** An app restart is covered with no disk writes: the daemon keeps running when the GUI quits (`QuitGUIOnly`), so the in-memory record and its link survive. A daemon restart kills every PTY: there is no session restore (no restore code in `internal/daemon` or `internal/pty`; `runDaemonCore` shuts the hub manager down at exit, process.go:150-153). So no shared session can exist after a daemon restart, and nothing is left to restore. **Recommendation:** in-memory only. Do not write tokens to disk. See Open Question 1, which asks the user to confirm this reading of the locked decision.

**Signing-key regeneration** (Settings ▸ Security, kept; api.go:1439-1461) makes every record's token unverifiable. Recommendation: for each shared session, revoke viewers with the same close reason, clear grants, and mint a fresh record. Shares stay on with new links. Otherwise the popover shows a dead link.

### Pattern 2: `requireSameUser` WhoIs middleware

**API facts (read from module source this session):**
- `func (lc *Client) WhoIs(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error)`. `remoteAddr` "must be an IP or IP:port". Not found → `local.ErrPeerNotFound`. [VERIFIED: tailscale.com@v1.98.3/client/local/local.go:312-332]
- `apitype.WhoIsResponse{ Node *tailcfg.Node; UserProfile *tailcfg.UserProfile; CapMap tailcfg.PeerCapMap }`, package `tailscale.com/client/tailscale/apitype`. [VERIFIED: client/tailscale/apitype/apitype.go:35-42]
- `tailcfg.Node` has `User UserID`, `Sharer UserID` ("if non-zero, the user who shared this node, if different than User"), `Tags []string`, `func (n *Node) IsTagged() bool { return len(n.Tags) > 0 }`. [VERIFIED: tailcfg/tailcfg.go:348-364, 594-596]
- `tailcfg.UserProfile{ID UserID; LoginName string // "for display purposes only"; DisplayName …}`. [VERIFIED: tailcfg.go:285-297]
- This Mac's own user: `StatusWithoutPeers(ctx)` → `Status.Self *PeerStatus` with `UserID tailcfg.UserID`, `AltSharerUserID tailcfg.UserID`, `Tags *views.Slice[string]`. [VERIFIED: ipn/ipnstate/ipnstate.go:49, 225-246]

**Rule (fail closed):**
```go
// internal/webserver/whois.go (sketch)
type whoIsClient interface {
    WhoIs(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error)
    StatusWithoutPeers(ctx context.Context) (*ipnstate.Status, error)
}
// production: ws.whois = &ws.lc   (local.Client satisfies both methods)

func sameUser(who *apitype.WhoIsResponse, self *ipnstate.PeerStatus) bool {
    if who == nil || who.Node == nil || self == nil { return false }
    if who.Node.IsTagged() || who.Node.Sharer != 0 { return false }      // tagged or shared-in
    if self.Tags != nil && self.Tags.Len() > 0 { return false }           // this Mac is tagged
    if self.AltSharerUserID != 0 || self.UserID == 0 { return false }
    return who.Node.User == self.UserID
}

func (ws *WebServer) requireSameUser(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
        defer cancel()
        st, err := ws.whois.StatusWithoutPeers(ctx)
        if err != nil || st.Self == nil { forbidden(w); return }
        who, err := ws.whois.WhoIs(ctx, r.RemoteAddr)
        if err != nil || !sameUser(who, st.Self) { forbidden(w); return }
        next(w, r)
    }
}
```
`forbidden` writes 403 with the `X-Portty: 1` marker so the viewer can tell "Portty, other account" apart from "not Portty". Use one generic body (no account names, T-87-08 precedent).

**Edge cases:**
| Case | Behavior | Basis |
|------|----------|-------|
| Same user, another Mac | allow | `Node.User == Self.UserID` |
| Different user, same tailnet | 403, peer group hidden, CLI "didn't allow access" | UI-SPEC |
| Tagged node (CI box, server) | 403 | Tagged nodes' `User` is the tailnet's shared tagged-devices identity, not a person [ASSUMED]. Checking `IsTagged()` explicitly also blocks two tagged nodes "matching" each other |
| This Mac is tagged | refuse everyone | Same reason. Document in Settings explanation if it ever matters |
| Node shared in from another tailnet | 403 (`Sharer != 0`) | tailcfg.go:363-364. Strict on purpose; the user's two Macs are in one tailnet |
| Node with no user / WhoIs 404 (`ErrPeerNotFound`) | 403 | fail closed |
| LocalAPI down mid-request | 403 | fail closed. The listener is useless without tailscaled anyway |

**Where to apply it:** `GET /api/sessions/meta` (currently open, server.go:823) and the new `POST /api/peer/sessions/{id}/cap`. Not on `/sessions/{id}` or the WS route: those stay capability-only, because the link must work from any tailnet device (locked decision).

**CSRF hardening for the cap route:** a browser page on the same user's other Mac passes WhoIs. It cannot read a cross-origin response unless the server sends CORS headers, so the token can't leak. Still, make the route POST, require `Content-Type: application/json` (forces a CORS preflight that the server never answers), and **never add CORS headers to WhoIs-gated routes**. DNS rebinding is blocked because the TLS listener only presents a certificate for the `.ts.net` name (`lc.GetCertificate`, server.go:413-416).

### Pattern 3: Revoke with a fixed reason, and close the subscribe race

Today `Hub.DisconnectWebViewers` calls `go sub.CloseSlow()` for each web subscriber (hub.go:197-210), and the webserver sets `sub.CloseSlow = func() { conn.Close(websocket.StatusPolicyViolation, "too slow") }` (server.go:1277-1279). [VERIFIED: both read this session]

Recommended change:
- Add `Kick func(reason string)` to `relay.Subscriber`. The webserver sets it to `conn.Close(websocket.StatusNormalClosure, reason)`.
- Add `func (h *Hub) RevokeWebViewers(reason string)`. Same unlock-before-I/O shape as `DisconnectWebViewers` (collect under `h.mu`, release, then call; calling under the lock self-deadlocks, hub.go:186-192).
- Define the reason once: `const CloseReasonShareStopped = "share stopped"` in `relay` (next to the existing `"session ended"` literal used at server.go:1366 and relay/server.go:191). The frontend keeps a matching constant. Banner text never renders the wire reason (T-175-06-02).
- **Re-check after subscribe** in `handleWSSRelay`: right after `hub.Subscribe(sub)`, `if !ws.isGrantActive(claims.SID, claims.GrantID) { conn.Close(StatusNormalClosure, CloseReasonShareStopped); return }`. Share-off clears grants before it kicks subscribers, so any connection either subscribed before the kick (and gets kicked) or sees the grant gone.
- The plugin-config SSE stream (`handleStreamPluginConfig`, plugin_config_stream.go:34-90) also outlives revocation. Re-check `isGrantActive` before each send and return when revoked. Low impact (plugin settings only), but it makes "disconnect every viewer" literally true.
- `http.Serve(ln, mux)` (server.go:439) means `ws.Stop()` closes the listener only. Active connections, including hijacked WebSockets, keep running. So **revoke first, then stop**. Do not rely on `Stop()` to disconnect anyone.

`handleDisconnectViewers` and the "Disconnect all viewers" button go away with the share modal. Their purpose (kick without revoking) no longer exists once there is one switch.

### Pattern 4: Proxy forwards the upstream close

`copyWS` (remote_ws_proxy.go:120-131) returns on the first error, and the deferred `CloseNow()` sends an abnormal close with no reason. Forward it:

```go
// internal/daemon/remote_ws_proxy.go (sketch)
err := <-errc
var ce websocket.CloseError
if errors.As(err, &ce) {
    _ = clientConn.Close(ce.Code, ce.Reason)   // e.g. 1000 "share stopped" / "session ended"
}
```
[VERIFIED: coder/websocket@v1.8.15/close.go:65-84 defines `CloseError` and `CloseStatus`]

Without this, neither the peer tab nor the CLI can tell "share stopped" from a network drop. This is the hinge for SHAR-04's peer half and SHAR-06's exit message.

### Pattern 5: One remote-connect path for GUI and CLI

**Daemon route `POST /peers/connect {"host": "...", "sessionId": "..."}`** on the Unix socket (0600, same trust boundary as every other owner route):
1. Resolve `host` against **all online tailnet peers** (`tailnet.DiscoverPeers`, not the probed list). Match the first DNS label of `DNSName` case-insensitively first, then `HostName`. The decision says host is "the MagicDNS short name". Today's code matches `HostName` only (cmd_attach.go:209-213), and `HostName` is the machine's local hostname, which can differ in case and form from the MagicDNS label.
2. Build `baseURL = "https://" + strings.TrimSuffix(DNSName, ".") + ":7444"` from Tailscale's own status. Never use a URL a peer supplied.
3. `GET {baseURL}/api/sessions/meta` → resolve the session (exact name, then unique ID prefix). `POST {baseURL}/api/peer/sessions/{id}/cap` → token.
4. `RemoteCapStore.Put(id, baseURL, token)` (remote_caps.go:62-91, keyed by session ID).
5. Return `{sessionId, name, host}`, or a typed error code: `tailscale_off`, `unknown_host`, `not_sharing` (connection refused: with stop-when-none-shared the host's port is closed), `no_such_session`, `ambiguous`, `forbidden` (403), `unreachable`.

**GUI:** a peer-row click calls a new binding `ConnectPeerSession(host, sessionId)`, then opens the existing in-app tab with `remote: true`, which routes through `ws://127.0.0.1:<relay>/api/relay/remote/{sid}/ws` (relayClient.ts:117-124). On error the tab opens with the "Couldn't connect to <host>…" banner (UI-SPEC).

**CLI:** `portty attach <host>[:<session>]` calls the same daemon route, then dials the local relay proxy. The relay's inbound Origin allowlist accepts the CLI's empty Origin (relay/server.go:212-214). The CLI no longer dials the peer directly, so the missing-cap and missing-Origin bugs disappear.

**Why not have the CLI dial the peer directly:** it would need its own cap fetch, Origin injection (byte-exact `https://fqdn:7444`) and TLS/IP fallback. The daemon proxy already does all three for the GUI. WhoIs sees the same source address either way, because both processes run on the same Mac.

### Pattern 6: Web-facing states

- `GET /sessions/{id}` with a bad or revoked cap: an HTML page instead of `http.Error` plain text, same status codes (401/403), CSP headers applied (it is already wrapped in `ws.cspHeaders`, server.go:849-850). Add a page variant of `requireCapability` used **only** on this route. API routes keep plain text. The page must not echo host, session name or ID.
- React `/app/` (web mode only): on load, `GET /api/sessions/{id}/info?cap=` (same origin, no CORS issue). 401/403 → the "Link No Longer Works" block. Mid-session close with reason `share stopped` → the info banner, no reconnect. `TerminalPanel` already sets `sessionEnded` on any guest close (TerminalPanel.tsx:367-374). Branch on the reason constant there.
- Dashboard (`web/dashboard.html:17-19` form, `web/assets/dashboard.js`): replace the join form with the UI-SPEC paragraph.
- Add `Referrer-Policy: no-referrer` in `cspHeaders` (csp_mw.go:123-124 sets only CSP and Cache-Control). The `/app/` URL now carries a full-control token. Browsers default to `strict-origin-when-cross-origin`, which already strips the query cross-origin [ASSUMED], so this is hardening, not a fix.

### Routes: keep / remove / add

**Daemon socket (`internal/daemon/api.go:150-220`)**
| Route | Disposition | Note |
|-------|-------------|------|
| `POST /webserver/start`, `POST /webserver/stop`, `GET /webserver/status` | REMOVE | Server lifecycle becomes internal to share on/off |
| `GET /webserver/local-password` | REMOVE | LAN fallback |
| `POST /sessions/{id}/web-serve` | REPLACE with `PUT /sessions/{id}/share` | |
| `POST /sessions/{id}/capabilities` | REMOVE | Folded into share on (returns the link) |
| `POST /sessions/{id}/disconnect-viewers` | REMOVE | Share off revokes and disconnects |
| `POST /sessions/{id}/funnel`, `POST`/`DELETE /sessions/{id}/funnel-write` | REMOVE | Funnel |
| `POST /join/exchange` | REMOVE | Join codes |
| `GET/PATCH /settings/shell-web-share-warned`, `…-warning-enabled` | REMOVE | Banner and its Settings switch removed (UI-SPEC) |
| `POST /api/remote-files/caps`, `GET /api/remote-files/caps/{id}/open-url` | REMOVE (recommended) | Existed so the GUI could deposit a join-code cap. `POST /peers/connect` writes the store directly. Keep `RemoteCapStore` itself |
| `POST /capability/regenerate-key` | KEEP + re-mint shares | |
| `GET /tailnet/peers` | KEEP | Sidebar and CLI "Found: …" list |
| `GET /sessions/{id}/share` | ADD | `{shared, url}` for the popover on open |
| `PUT /sessions/{id}/share` | ADD | Pattern 1 |
| `POST /peers/connect` | ADD | Pattern 5 |

**Webserver (`internal/webserver/server.go:782-958`)**
| Route | Disposition |
|-------|-------------|
| `GET /`, `GET /dashboard` | KEEP (copy change) |
| `GET /join`, `POST /join/exchange` | REMOVE |
| `GET /api/sessions/meta` | KEEP, wrap in `requireSameUser` |
| `GET /api/sessions` (marker + cap) | KEEP. Peer discovery probe depends on its 401 + `X-Portty: 1` (tailnet.go:90-100) |
| `GET /api/sessions/{id}/info`, `/api/plugin-config`, `/api/plugin-config/stream` | KEEP |
| `GET /sessions/{id}` | KEEP, HTML error variant |
| `GET /sessions/{id}/ws` | KEEP, re-check after subscribe, `Kick` |
| `GET /api/sessions/{id}/qr` | REMOVE |
| `GET /assets/…`, `GET /app/` | KEEP |
| `POST /api/peer/sessions/{id}/cap` | ADD (`requireSameUser`, session must be shared) |

**CLI (`main.go:215-237`, `cmd_cli.go`)**
| Command | Disposition |
|---------|-------------|
| `serve`, `unserve`, `web`, `qr` | REMOVE |
| `share <session>`, `unshare <session>` | ADD |
| `attach <session>` / `<host>:<session>` / `<host>` | REWORK; drop `--readonly` |
| `list` | FIX remote section to use the WhoIs-gated meta (today it calls the cap-gated `/api/sessions` with no cap, cmd_cli.go:142, and lists nothing) |

### Anti-Patterns to Avoid
- **Trusting `X-Portty` or peer-supplied URLs for access.** The marker is an identity hint (AR-02). The daemon builds peer base URLs from Tailscale status.
- **Matching users by `LoginName`.** Display-only field. Use `UserID`.
- **Relying on `ws.Stop()` to disconnect viewers.** It closes listeners only.
- **Rendering `CloseEvent.reason` or daemon error text in banners.** Fixed copy plus local names only (UI-SPEC Accessibility, T-175-06-02).
- **A random-port fallback for the share server.** See Pitfall 6.
- **Re-adding CORS headers to any WhoIs-gated route.**

## Removal Inventory

Line numbers are from this session's reads. "Shared" marks code a kept feature also uses: read it before cutting (Chesterton).

### Go — webserver
| ID | Target | Location | Shared with kept features? |
|----|--------|----------|----------------------------|
| R-FN-1 | Funnel fields `funnelClient`, `funnelActive`, `funnelBaseURL`, `funnelPort`, `loopbackListener` | server.go:90-96, 101 | `ws.lc` (server.go:88) **stays**: GetCertificate (server.go:414) and the new WhoIs seam use it |
| R-FN-2 | Loopback plain-HTTP listener serving `ws.mux` (Funnel hop 2) | server.go:441-476; Stop 527-542 | No. Removing it closes the Phase 3 residual T-03-06 / IN-07 (marker visible on the internet) |
| R-FN-3 | `EnableFunnel`, `DisableFunnel`, `FunnelBaseURL`, `ClearLingeringFunnel` | server.go:577-766; `Stop()` call at 525 | No |
| R-FN-4 | `funnel_client.go` (whole file) | internal/webserver/funnel_client.go | No |
| R-FN-5 | Funnel origin branch | origin_mw.go:61-65 (middleware), 89-91 (`allowedOrigins`) | Middleware itself **stays** (CSWSH defense, Phase 88). Keep the tailnet-only branch |
| R-RW-1 | `rwGated` map, `SetRWGate`, `isRWGated` | server.go:109-114, 285-310 | No production reader since 01-03 (STATE) |
| R-RW-2 | `RemoveGrant` | server.go:270-283 | Only caller is `revokeFunnelWriteLocked` (api.go:1743). Remove with it |
| R-JC-1 | `joinCodes` field, `SetJoinCodes` | server.go:121-125, 323-331 | No |
| R-JC-2 | `GET /join`, `POST /join/exchange`, `handleJoin`, `handleJoinExchange` | server.go:798-807, 985-1064 | No |
| R-QR-1 | `GET /api/sessions/{id}/qr`, `handleSessionQR`, qrcode import | server.go:861-863, 1194-1211, 25 | No |
| R-LAN-1 | `Config.Mode`, `Config.Password`, `Start()` dispatch, `Mode()`, `startLocal`, local branch of `BaseURL` | server.go:40-45, 344-356, 480-517, 571-573 | `Config` stays. `BindIP()` (361) is used only by the LAN upgrader → remove. Playwright fixture sets `Mode: "tailscale"` (cmd/playwright-fixture/main.go:105) and must change in the same plan |
| R-LAN-2 | `auth.go` (BasicAuthMiddleware), `localip.go` (GetLANIP) + their tests | whole files | No production callers outside the LAN path (verified by git grep) |
| R-LAN-3 | `selfcert.go` (`GenerateSelfSignedCert`) | whole file | **Shared: KEEP.** Tests use it as a TLS helper: app_test.go:346, 477; internal/daemon/api_test.go:1273 |
| R-LAN-4 | `assertTailnetBindIP` comments about local mode | server.go:371-403 | The warning itself stays (defense in depth for the meta route) |

### Go — capability
| ID | Target | Location | Shared? |
|----|--------|----------|---------|
| R-JC-3 | `joincode.go`, `joincode_test.go`, `export_test.go` (only `SetClockForTest` on `JoinCodeManager`) | internal/capability/ | No |
| R-JC-4 | `ErrCodeNotFound`, `ErrCodeExpired` | errors.go:24-34 | No. Keep the three token errors |
| keep | `Claims`, `Sign`, `Verify`, `HasPerm`, `KeyStore`, `WithClaims` | capability.go, keystore.go, context.go | Core model. `HasPerm` is still the write gate in `handleWSSRelay` (server.go:1236-1237) |

### Go — daemon
| ID | Target | Location | Shared? |
|----|--------|----------|---------|
| R-FN-6 | All `funnel*` fields, `mintRaceHookForTest`, `funnelReadCodeMaxTTL` | api.go:58-136 | No |
| R-FN-7 | Funnel handlers and helpers | api.go:1482-1772 (`handleSetSessionFunnel`, `handleSetSessionFunnelWrite`, `handleDisableSessionFunnelWrite`, `disableFunnelForSession`, `revokeFunnelWriteLocked`, `disableFunnelWriteForSession`) | `runSessionExitCleanup` (api.go:602-618) and `handleDeleteSession` (636-659) call `disableFunnelForSession`: keep those functions, drop the call, add the share-record cleanup |
| R-FN-8 | Funnel snapshot in `handleListSessions` | api.go:524-555 | `WebEnabled` enrichment stays (now from the share record) |
| R-JC-5 | `joinCodes` field, bootstrap, `JoinCodes()`, `handleExchangeJoinCode`, `mintFunnelReadCodeLocked` | api.go:42-46, 264-268, 277-281, 1371-1432, 1311-1331 | `BootstrapCapabilityState` stays (signing key) |
| R-WS-1 | `handleWebServerStart/Stop/Status`, `RestartWebServer`, `AutoStartWebServer` (rework into share helper), `WebServerMode`, `WebServerBindIP`, `SetLocalPassword`, `localPassword`, `handleGetLocalPassword` | api.go:33, 382-495, 959-1081 | `AutoStartWebServer`'s wiring (resolver, plugin provider, listener, key, static FS) is the template for the new start helper |
| R-WS-2 | `handleWebServe`, `issueCapabilitiesForSession`, `handleIssueCapabilities`, `handleDisconnectViewers` | api.go:1095-1365 | Replaced by share on/off |
| R-WS-3 | Shell web-share warning routes | api.go:165-168, 787-832 | No |
| R-LAN-5 | `upgradeToTailscale`, local password, LAN auto-start, `generateLocalPassword`; **and the Tailscale auto-start at daemon boot** | process.go:22-58, 107-147, 155-164 | The web server must start only on first share. `BootstrapCapabilityState`, relay and socket startup (process.go:83-104) stay |
| R-WS-4 | Engine `shellWebShareWarned`, `shellWebShareWarningEnabled` (fields, get/set, settings keys, constructor default) | engine.go:87-88, 130-139, 165-166, 200-215, 729-776 | Legacy keys in an existing settings.json are ignored by `json.Unmarshal` and dropped on the next save (engine.go:79-81 documents this). No schema bump needed |
| R-TY-1 | `WebServerStartRequest/Response`, `WebServerStatusResponse`, `WebServeRequest`, `SetSessionFunnel*`, `SetSessionFunnelWrite*`, `IssueCapabilitiesResponse`, `ExchangeJoinCode*`; `SessionInfo.FunnelActive`, `.FunnelWriteActive` | types.go:15-20, 76-176 | `SessionInfo.WebEnabled` stays (wire name kept; it now means "shared") |
| R-CL-1 | Client methods `StartWebServer`, `GetLocalNetworkPassword`, `StopWebServer`, `GetWebServerStatus`, `ToggleWebServing`, `DisconnectViewers`, `SetSessionFunnel`, `SetSessionFunnelWrite`, `DisableSessionFunnelWrite`, `IssueCapabilities`, `ExchangeJoinCode`; `ExchangeJoinCodeAtURL`; `RegisterRemoteCap`/`RemoteSessionOpenURL` if those routes go | client.go:250-372; client_remote_files.go:67, 184, 201 | `ListTailnetPeers`, `RegenerateSigningKey` stay |
| R-RO-1 | Local relay `?readonly=` query | relay/server.go:48 | `Subscriber.ReadOnly` **stays**: the webserver sets it from the cap's perms (server.go:1237, 1273) |

### Go — main package
| ID | Target | Location |
|----|--------|----------|
| R-APP-1 | `StartWebServer`, `GetLocalNetworkPassword`, `GetWebServerMode`, `StopWebServer`, `ToggleWebServing`, `DisconnectViewers`, `SetSessionFunnel`, `SetSessionFunnelWrite`, `DisableSessionFunnelWrite`, `GetWebServerURL`, `IsWebServerRunning`, `GetWebServerQRCode`, `IssueCapabilities`, `ExchangeJoinCode`, `GetCapabilityQRCode`, `ExchangeJoinCodeAtURL`, `RegisterRemoteCap`, `OpenRemoteSessionURL`, `GetShellWebShare*`/`SetShellWebShare*`, `GetRemoteSessions` (unused by the frontend and cap-broken) | app.go:516-560, 835-1050, 1177-1213, 1254-1303 |
| R-APP-2 | Funnel fields in `ListSessions` mapping | app.go:455-463 |
| R-APP-3 | qrcode import | app.go:19 |
| keep | `HasCTDisclosure`, `AcknowledgeCTDisclosure`, `RegenerateSigningKey`, `GetTailscaleStatus`, `OpenTailscale`, `GetRemoteSessionsWithMeta` (made tri-state) | app.go:960-975, 1029-1036, 1133-1175, 1220-1252 |
| R-CLI-1 | `serve`, `unserve`, `web`, `qr`, `cmdWeb*`, `cmdServe`, `cmdUnserve`, `cmdQR`, usage text, qrcode import | cmd_cli.go:17, 20-50, 205-304, 330-349; main.go:225-234 |
| R-RO-2 | `attach --readonly` | cmd_attach.go:35-41, 98-100 |
| R-FIX | `playwright-fixture`: `Mode`, the `"read"` viewer cap, the `/join/exchange` mock peer | cmd/playwright-fixture/main.go:101-106, 161-172, 175-178, 271-300 (build tag `playwrightfixture`; plain `go vet ./...` does not compile it) |

### Frontend
| ID | Target | Location |
|----|--------|----------|
| R-UI-1 | Delete `Share/SessionShareModal.tsx`, `Share/FunnelRiskPanel.tsx`, `Share/ShareSegmentedControl.tsx`, `SessionShare/{TailnetTab,InternetReadOnlyTab,InternetFullAccessTab,ShareLinkCard,shared}.tsx`, `ShellWebShareBanner.tsx`, `RemoteJoinCodeModal.tsx` | frontend/src/components/ |
| R-UI-2 | Delete tests `FunnelRiskPanel`, `HoldToConfirmButton`, `InternetFullAccessTab`, `InternetReadOnlyTab`, `SessionShareModal`, `SessionShareModal.disconnect`, `ShareLinkCard`, `ShareSegmentedControl`, `ShellWebShareBanner`, `TailnetTab`, `RemoteJoinCodeModal` (**two copies**: `components/RemoteJoinCodeModal.test.tsx` and `components/__tests__/RemoteJoinCodeModal.test.tsx`), `funnelBinding.contract`, `App.shellWebShare`, `SettingsSheet.shell-warn-toggle`, `SettingsSheet.web-link-ux` | `frontend/src/components/__tests__/` |
| R-UI-3 | Rewrite `App.open-remote.test.tsx` for auto-connect | same |
| R-UI-4 | App.tsx: imports 18-50; shell-warn state 95-104; share-modal and join-modal state 156-176; web-server init and poll 360-424; share toggle and shell-warn handlers 659-740; modal sync 770-784; remote open and modal exchange 948-1014; TabBar Funnel props 1118-1131; Settings web-server callbacks 1250-1290; modal renders 1344-1420 | frontend/src/App.tsx |
| R-UI-5 | TabBar Funnel exposure icons (`GlobeAltIcon`, `LockOpenIcon`) and props | TabBar.tsx:60-73, 107-108, 205-230 |
| R-UI-6 | Settings ▸ Sharing web-server row, port, URL, QR, LAN password, "Ask before sharing" switch; Security CT copy change | SettingsSheet.tsx:3-20, 160-300, 375-460 |
| R-UI-7 | `remoteAdapter.ts` `funnelActive` | lib/remoteAdapter.ts:27 |
| R-UI-8 | Hand-maintained Wails stubs: `wailsjs/go/main/App.d.ts`, `App.js`, `wailsjs/go/models.ts`. The `wailsjs/wailsjs/…` tree is not what the app imports (funnelBinding.contract.test.tsx header documents this trap) | frontend/src/wailsjs/ |
| R-UI-9 | Funnel/share-modal CSS (keep all tokens, UI-SPEC) | style.css |
| R-WEB-1 | `web/join.html`, `web/assets/join.js`, `web/assets/join.css`; `embed.go` list | web/embed.go:5 |
| R-WEB-2 | Dashboard join form and copy; `dashboard.js` join logic | web/dashboard.html:17-19, web/assets/dashboard.js |
| R-WEB-3 | READ ONLY badge and read-only caret in `terminal.js` (still tested by several Go e2e tests; keep the file) | web/assets/terminal.js:93-113 |

### Go tests affected
- **Delete:** internal/daemon/funnel_test.go (22 tests), internal/webserver/funnel_test.go (7), join_test.go (2), rwgate_test.go (6), internal/capability/joincode_test.go (16), auth_test.go, localip_test.go, internal/daemon/api_shell_warn_test.go, engine_shell_warn_test.go.
- **Edit:** process_test.go (drop `TestRestartWebServer_StopsAndStarts`, `TestUpgradeToTailscale_ExitsOnCancel`), api_test.go, api_disconnect_test.go (becomes the share-off revoke test), client_test.go, app_test.go, app_remote_files_test.go, cmd_cli_test.go, cmd_attach_test.go, server_test.go, brand_pages_test.go (`/join` copy), capability_test.go, csp_integration_test.go, browser_csp_e2e_test.go, fuzz_test.go (`FuzzWebServerRoutes` seeds hitting `/join`), origin_mw_test.go (Funnel origin cases), session_ended_test.go, inject_test.go, relay/server_test.go and server_inject_test.go (`readonly` query).
- Every deleted or renamed file also needs its TESTING.md Section 4 rows removed and Section 2 counts updated. Sections 5 "Category I — Tailscale Funnel", "T — Funnel Frontend", "X — Public Share Access Codes", "Y — Public Full-Access", "J — Shell Web-Share Warning Toggle", "A — Share Modal" and "G — Live Tailnet Remote Session Open" need retiring or rewriting (TESTING.md:485-707).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Who is calling over the tailnet | Parsing IPs, keeping an allowlist of 100.x addresses | `local.Client.WhoIs(ctx, r.RemoteAddr)` | tailscaled already maps the WireGuard-authenticated source to a node and user |
| HTTPS certificate | Self-signed certs, ACME client | `lc.GetCertificate` (already wired, server.go:413-416), `lc.CertPair` for warm-up | Tailscale issues and renews the LE cert |
| Revocation | Token expiry, blocklists, epochs | Existing grant set + `ClearGrants` + new grant ID on re-share | Already the model; capability_mw.go:68-71 |
| Close-reason parsing | String matching on errors | `errors.As(err, &websocket.CloseError{})` | Typed code and reason |
| Session-name matching | Ad hoc in each command | One `resolveSession(list, arg)` used by share, unshare, attach (local and remote) | The same three error copies (no such, clash, not shared) appear in all of them |

**Key insight:** the capability model already has the revocation primitive. What it lacks is enforcement on live connections and a reason on the close.

## Runtime State Inventory

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | `settings.json` may hold `shellWebShareWarned` and `shellWebShareWarningEnabled` (engine.go:87-88). `ct_disclosed` file in the config dir (app.go:960-974) stays and keeps its meaning. `capability.key` unchanged. Share records are new and in-memory | Code edit only. Unknown keys are ignored on load and dropped on next save. No migration |
| Live service config | **tailscaled serve config.** Funnel wrote `ipn.ServeConfig` (TCP 443 + web proxy to a loopback port + AllowFunnel) into tailscaled, which persists across process restarts. `ClearLingeringFunnel` (server.go:745-766) existed precisely to clear stale config at daemon start. Once removed, any Funnel config left by an AgentHub or pre-Phase-4 Portty run stays in tailscaled and publicly proxies to a dead port | No code (removing Funnel means not touching serve config). Add a UAT/manual step on both Macs: `tailscale funnel status` / `tailscale serve status`; if anything is listed, `tailscale serve reset`. Record it in TESTING.md Section 5 |
| OS-registered state | LaunchAgent `portty-daemon` (service.go:55): unchanged. No Tailscale registrations by Portty other than the serve config above | None |
| Secrets/env vars | Local-mode password: generated per daemon lifetime, memory only (process.go:107-113); disappears with the code. `capability.key` unchanged | None |
| Build artifacts | Untracked `agenthub` binary in the repo root and `/agenthub` no longer gitignored (Phase 3 IN-08; git status shows `?? agenthub`). go.sum lines 168-169 for go-qrcode | `go mod tidy` removes the go.sum lines. Do not `git add -A` (would commit the binary). The binary is outside this phase's scope |

## Common Pitfalls

### Pitfall 1: Live viewers survive revocation
**What goes wrong:** Share off, but the browser tab keeps typing into the shell.
**Why:** `requireCapability` runs only at upgrade; hijacked WebSockets outlive `ClearGrants` and `ws.Stop()`.
**How to avoid:** Pattern 3: clear grants, then `RevokeWebViewers(reason)`, then re-check the grant after `Subscribe`.
**Warning signs:** a test that opens a WS, flips share off, and can still write to the PTY.

### Pitfall 2: Every disconnect looks like "too slow"
**What goes wrong:** Peer tabs and the CLI can't show "stopped sharing".
**Why:** `DisconnectWebViewers` reuses `CloseSlow` = `StatusPolicyViolation, "too slow"` (server.go:1277-1279), and the proxy drops close reasons (remote_ws_proxy.go:120-131).
**How to avoid:** `Kick(reason)` plus proxy close forwarding (Patterns 3-4). Test the reason end to end through the proxy.

### Pitfall 3: Tagged nodes look like the same user
**What goes wrong:** Every tagged device gets full control of the user's shells, or two tagged nodes trust each other.
**Why:** tagged nodes don't carry the person's user identity [ASSUMED], and equality of two tagged identities is meaningless.
**How to avoid:** reject `who.Node.IsTagged()`, `Sharer != 0`, and a tagged Self, before comparing `UserID`.

### Pitfall 4: CLI host names don't match
**What goes wrong:** `portty attach office-air:build` says "no Mac named office-air" while the sidebar shows it.
**Why:** current code matches `tailnet.Peer.Hostname` (HostInfo hostname, e.g. `Kens-MacBook-Air`) (cmd_attach.go:209-213), while the decision says MagicDNS short name (`kens-macbook-air`).
**How to avoid:** match the first label of `DNSName` case-insensitively, then fall back to `HostName`. The sidebar's `shortHost` (sidebarPeers.ts:20-22) splits `hostname` on `.`, which is the HostInfo name. Keep the CLI hint and the sidebar label consistent.

### Pitfall 5: "Not sharing" is reported as "unknown host"
**What goes wrong:** With the server stopped when nothing is shared, the host's port 7444 is closed, so `DiscoverAndProbe` doesn't list it and the CLI says "no Mac named <host> is running Portty".
**How to avoid:** resolve the host against all online peers (`DiscoverPeers`), and map connection-refused to `not_sharing`. The "Found: …" list in the unknown-host message comes from the probed list (`GET /tailnet/peers`).

### Pitfall 6: Random-port fallback breaks peers silently
**What goes wrong:** Share works locally but no peer ever sees it.
**Why:** `startTailscale` falls back to `:0` on any `*net.OpError` (server.go:419-433), but peers probe only `tailnet.DefaultProbePort` = `7444` (tailnet.go:29 `const DefaultProbePort = 7444`) [VERIFIED].
**How to avoid:** for the share server, fail with "port 7444 is in use" instead of falling back. Also `cmdWebStart` used port 443 (cmd_cli.go:228). It goes away with `web start`.

### Pitfall 7: Deleting a test file without its TESTING.md row
**What goes wrong:** `tests/check-traceability-paths.sh` fails (`MISSING traceability path: …`).
**How to avoid:** same commit removes Section 4 rows. Run the script at every plan boundary.

### Pitfall 8: Editing the wrong Wails stub tree
**What goes wrong:** TypeScript compiles but App imports stale signatures, or the reverse.
**Why:** the app imports `frontend/src/wailsjs/go/main/App` and `wailsjs/go/models`. `frontend/src/wailsjs/wailsjs/go/models.ts` is a second generated tree. The stubs are hand-maintained ("AUTO-GENERATED … replaced by `wails build`" header, but commits edit them).
**How to avoid:** update `App.d.ts`, `App.js` and `go/models.ts` together with every binding change.

### Pitfall 9: The playwright fixture stops compiling unnoticed
**What goes wrong:** `go build ./...` and `go vet ./...` pass, then the e2e workflow fails.
**Why:** `cmd/playwright-fixture/main.go` has `//go:build playwrightfixture`, and it sets `Config.Mode` (main.go:105) and mounts a `/join/exchange` mock.
**How to avoid:** add `go vet -tags playwrightfixture ./cmd/playwright-fixture/` to the plan's verify step.

### Pitfall 10: CT acknowledgement written by two processes
**What goes wrong:** The GUI writes `ct_disclosed` (app.go:971-974); the CLI must record it too (UI-SPEC). Two writers is fine (same file, same content), but only if the share itself succeeded.
**How to avoid:** the daemon writes the file on the first successful share-on (it shares `os.UserConfigDir()/portty` with the app: engine.go:64-71 and app.go:825-833 both build that path). The GUI keeps reading via `HasCTDisclosure`. The CLI prints the CT sentence when the share response says it was the first.

### Pitfall 11: Close-reason text drift
**What goes wrong:** The server says `share stopped`, the frontend checks `sharing stopped`.
**How to avoid:** one Go constant, one TS constant, and one cross-language test (Go test asserts the literal; vitest asserts the TS constant equals the same literal), mirroring how `"session ended"` is used today.

### Pitfall 12: `go mod tidy` churn
**What goes wrong:** tidy upgrades or drops more than go-qrcode.
**How to avoid:** run it with the pinned toolchain and check `git diff go.mod go.sum` shows only go-qrcode lines removed (T-03-SC precedent).

## Code Examples

### Hub revoke with reason (sketch)
```go
// Source: modelled on internal/relay/hub.go:197-210 (DisconnectWebViewers)
const CloseReasonShareStopped = "share stopped"

func (h *Hub) RevokeWebViewers(reason string) {
    h.mu.Lock()
    targets := make([]*Subscriber, 0, len(h.subscribers))
    for s := range h.subscribers {
        if s.Origin == "web" { targets = append(targets, s) }
    }
    h.mu.Unlock() // never call Kick under h.mu: it re-enters Unsubscribe
    for _, s := range targets {
        if s.Kick != nil { go s.Kick(reason) } else { go s.CloseSlow() }
    }
}
```

### CLI exit on revoke (sketch)
```go
// After AttachSession returns, BEFORE printing anything, undo raw mode and the bar
// explicitly (the deferred term.Restore / bar.Stop would run after the print).
err = attach.AttachSession(ctx, conn, os.Stdin, stdout, detachKey, bar, onFrame)
var ce websocket.CloseError
revoked := errors.As(err, &ce) && ce.Reason == relay.CloseReasonShareStopped
term.Restore(int(os.Stdin.Fd()), oldState)
if bar != nil { bar.Stop() }
if revoked {
    fmt.Printf("Portty: %s stopped sharing %s.\n", host, session.Name) // locked copy, stdout, exit 0
    return nil
}
printDetachMessage(os.Stderr)
```
`main.go` prints any returned error and exits 1 (main.go:244-247), so the revoke path must return `nil`. Errors also need the `portty: ` prefix that UI-SPEC requires. Today they print raw (`attach: …`).

### Fake WhoIs for tests
```go
type fakeWhoIs struct {
    self *ipnstate.Status
    byAddr map[string]*apitype.WhoIsResponse // key: host part of RemoteAddr, e.g. "127.0.0.1"
    err error
}
func (f *fakeWhoIs) StatusWithoutPeers(context.Context) (*ipnstate.Status, error) { return f.self, f.err }
func (f *fakeWhoIs) WhoIs(_ context.Context, addr string) (*apitype.WhoIsResponse, error) {
    host, _, _ := net.SplitHostPort(addr)
    if w, ok := f.byAddr[host]; ok { return w, nil }
    return nil, local.ErrPeerNotFound
}
```
The existing `testServer` binds `127.0.0.1` with a self-signed TLS config (capability_test_helpers.go:43-80), so every test request arrives from `127.0.0.1:<port>`. Expose `SetWhoIsClientForTest`, mirroring the existing `SetFunnelClientForTest` pattern (funnel_client.go:44-50) that is being deleted.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Join codes exchanged out of band | Same-user WhoIs issuance | This phase | No code to paste. Only same-account Macs get peer access |
| Two links (read, read/write) + public Funnel links | One full-control tailnet link | This phase (SHAR-02, SHAR-05) | One token per share |
| Web server always on (auto-start at daemon boot, LAN fallback) | On while anything is shared | This phase | Port 7444 closed when idle. Closes WR-01/WR-02 |

**Deprecated/outdated:** `FetchAllPeerSessions` / `GetRemoteSessions` (cap-gated listing with no cap): remove. `FetchPeerSessions` used by `cmdAttachRemote`: replaced by the daemon connect route.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Tagged nodes' `Node.User` is a shared tagged-devices identity, not the human owner | Pattern 2, Pitfall 3 | Low: the code rejects tagged nodes explicitly regardless |
| A2 | `Node.User` and `Status.Self.UserID` are the same numeric ID for one person's two Macs in one tailnet | Pattern 2 | High: if wrong, same-user peers are refused. Covered by the two-Mac UAT; add a debug log of both IDs on refusal (IDs only, no names) |
| A3 | Browsers default to `strict-origin-when-cross-origin`, so the `?cap=` query is not sent cross-origin today | Pattern 6 | Low: the recommended `Referrer-Policy: no-referrer` covers it either way |
| A4 | Rejecting `Sharer != 0` never blocks the user's own Macs (both in one tailnet, nothing shared in) | Pattern 2 | Medium: if a Mac were shared in from another tailnet, access is refused with the "didn't allow access" copy |
| A5 | A listener bound to the tailnet IP keeps working after a Tailscale reconnect with the same IP | Interaction Contract ("share state is kept") | Medium; Phase 5's reconnect work owns this. Note it for the two-Mac UAT |
| A6 | Reading "persists across daemon restarts" as satisfied by in-memory records, because sessions die with the daemon | Pattern 1 | Medium: the user may want literal disk persistence. Open Question 1 |

## Open Questions

1. **Disk persistence of share records** (locked decision "persists across daemon and app restarts").
   - What we know: app restarts keep the daemon (and the record) alive. A daemon restart kills every PTY (no restore code), so a persisted record could never match a live session.
   - What's unclear: whether the user meant something else, e.g. the daemon being restarted by an update while sessions survive (they don't today).
   - Recommendation: in-memory records. Confirm with the user at plan check. If the user insists, persist `{sessionID: grantID, iat}` (not the token) in a 0600 file and prune at load, knowing it is always pruned today.

2. **Peer group for a same-account Mac with nothing shared.** The UI-SPEC says a peer with 0 sessions is hidden, but quick task 261116-c9f changed the sidebar to show "Nothing shared" for reachable empty peers (sidebarPeers.ts:28-34). With stop-when-none-shared, a not-sharing Mac's port is closed, so it isn't discovered at all and the question mostly resolves itself. A different-account Mac answers 403: recommend the meta fetch return a third state (`forbidden`) and `peerView` drop those groups, matching the UI-SPEC.

3. **`RemoteCapStore` keyed by session ID only** (remote_caps.go:39-43). Two peers can't realistically collide (32-hex random IDs), so keep it. Phase 5 reconnect may want `(host, id)`.

4. **InsecureSkipVerify in the proxy client** (remote_files.go:89-100) was justified by self-signed LAN certs, which this phase removes. Tightening to verify against the FQDN is possible now, but the proxy has no IP fallback for clients with MagicDNS off. Recommend leaving it as is in Phase 4 (WireGuard already authenticates the peer IP) and listing it for Phase 5.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/tests | ✓ | go1.27.4 (go.mod 1.26.4) | — |
| Node | frontend tests | ✓ | v24.22.1 | — |
| pnpm | frontend tests | ✓ | 12.6.0 | — |
| wails CLI | full app build | ✓ | /Users/ken/go/bin/wails | — |
| tailscale CLI/daemon (host) | manual checks only | ✓ | /usr/local/bin/tailscale | Tests use fakes |
| Tart VM `portty-test` | native popover check | per memory; no Tailscale inside | — | Fake Tailscale health in VM |
| Second Mac on same tailnet account | SHAR-01/03/04/06 end-to-end | ✗ in this environment | — | Deferred two-Mac UAT (TESTING.md Section 5) |

Baseline: `go build ./... && go vet ./...` passes on the branch today (run this session).

## Test Strategy (no real tailnet)

(`workflow.nyquist_validation` is `false`, so the full Validation Architecture section is omitted. This section answers the orchestrator's test-strategy question.)

| Layer | What to test | How |
|-------|--------------|-----|
| `webserver` whois | `sameUser` truth table: same user, other user, tagged peer, tagged self, `Sharer != 0`, nil node, WhoIs error, Status error | `fakeWhoIs` (above), pure table test |
| `webserver` routes | `/api/sessions/meta` 200 vs 403 + marker; `POST /api/peer/sessions/{id}/cap` returns the share token only when shared and same user; requires JSON content type | `testServer` + `SetWhoIsClientForTest` |
| `webserver` revoke | Open WS with cap → `ClearGrants`+`RevokeWebViewers` → client gets `CloseError{1000, "share stopped"}`; input after revoke never reaches the PTY; connect-after-revoke race closed by the post-subscribe check | `testServerWithHub` + `dialWebServerWS` (capability_test_helpers.go) |
| `webserver` page | `/sessions/{id}` bad cap → HTML, 401/403, CSP header, no session name/ID in body | existing CSP test helpers |
| `daemon` share | On: health fakes (`tailscale_off`, `https_off`), idempotent link, server started once; Off: record gone, grants cleared, server stopped when last share ends; session exit/kill cleans up; key regen re-mints | `SetWebServerForTest` exists (api.go:376). Add an injectable health func and a webserver-factory seam so tests don't bind 100.x |
| `daemon` proxy | Upstream close `(1000, "share stopped")` arrives at the client unchanged | `httptest.NewTLSServer` upstream + `SetRemoteFilesClientForTest` (api.go:239) |
| `daemon` connect | Host resolution (DNS label, HostName fallback, case), error codes, `RemoteCapStore.Put` | Injectable discover func (`tailnetCache` already takes a `discoverFunc`, tailnet_cache.go:39) and an injected HTTP client |
| CLI | `resolveSession` (name, clash, ID prefix), host resolution, output copy, exit codes, revoke path restores terminal before printing | Existing `cmd_attach_test.go` / `cmd_cli_test.go` patterns; `cmdAttachRemoteWithClient` already takes an injected client |
| Frontend | Popover states (off, starting, on, failures, Tailscale off), Copy/Copied!, context menu `disabled` item, peer tab banners by close reason, web "Link No Longer Works" | vitest with mocked `wailsjs/go/main/App` |
| Removal gates | No `funnel`, `join code`, `qrcode`, `readonly`, `read-only` link UI left in shipped code (SC4) | A `git grep` assertion test or a TESTING.md gate script, excluding `.planning/` and historical comments the planner chooses to keep |
| Manual | Popover native feel in Tart VM; two-Mac flow; `tailscale serve status` clean on both Macs | TESTING.md Section 5 new M-items |

Gate commands at every plan boundary: `go build ./... && go vet ./... && go vet -tags playwrightfixture ./cmd/playwright-fixture/ && go test -race -short ./...`, `cd frontend && pnpm exec tsc --noEmit && pnpm test`, `bash tests/check-traceability-paths.sh`.

## Suggested Plan Split (sequential, build green at each boundary)

Granularity is coarse. The stubs are hand-maintained, so Go bindings can change before the React side without breaking `tsc`. Between plans 1 and 4 the GUI's share UI is non-functional at runtime but everything builds and tests pass.

1. **04-01 Backend removal (Go only).** R-FN-*, R-RW-*, R-JC-*, R-QR-1, R-LAN-1/2/4/5, R-CLI-1, R-RO-*, R-FIX, R-APP-3, `go mod tidy`. Reduce `issueCapabilitiesForSession` to the write link only (no codes) so the old Tailnet tab still compiles against the bindings. Delete Go tests and their TESTING.md rows. Gate: build/vet (incl. playwright tag), Go tests, traceability.
2. **04-02 Share core (Go).** WhoIs seam + `requireSameUser`; share record; `GET`/`PUT /sessions/{id}/share`; start on first share (no random port, HasCerts error, optional cert warm-up), stop when none; `Kick`/`RevokeWebViewers` + post-subscribe re-check + SSE re-check; peer-cap route; meta gating; HTML error page; dashboard copy; `Referrer-Policy`; key-regen re-mint; remove `web-serve`/`capabilities`/`disconnect-viewers`/`webserver/*` routes and the shell-warn settings. New App bindings `GetSessionShare`, `SetSessionShared`. Tests per the strategy table.
3. **04-03 Remote access + CLI (Go).** `tailnet` tri-state meta + `FetchPeerCap`; `POST /peers/connect`; proxy close forwarding; `ConnectPeerSession` binding; `GetRemoteSessionsWithMeta` tri-state; `portty share/unshare`, `attach <host>[:<session>]` with resolver, list fix, error prefixes and copy, usage text; remove `RegisterRemoteCap`/open-url routes and bindings.
4. **04-04 Frontend.** Share popover; sidebar menu with `disabled`; delete R-UI-1/2/5/6/9 and R-WEB-*; peer auto-connect and banners; web-mode states; Settings panes; Wails stubs; vitest; TESTING.md (counts, rows, new M-items: popover in VM, two-Mac flow, serve-config check).
5. **(Optional, or fold into 04-04) 04-05 Removal gate and docs.** SC4 grep gate, TESTING.md Section 5 retirements, final sweep of comments that mention removed features.

## Security Domain

`security_enforcement` is enabled (ASVS level 1, block on high).

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Tailscale WireGuard node identity + WhoIs same-user check (peers, CLI); capability token (link holders) |
| V3 Session Management | yes | Capability token bound to session ID and grant ID; revoked by `ClearGrants`; new grant on re-share |
| V4 Access Control | yes | `requireSameUser` on discovery and cap issuance; `requireCapability` + Origin allowlist on terminal routes; owner routes on the 0600 Unix socket only |
| V5 Input Validation | yes | Session resolution against the daemon list; host resolved from Tailscale status, never peer-supplied URLs; JSON body limits (8 KiB precedent, remote_files.go:118) |
| V6 Cryptography | yes | HMAC-SHA256 via existing `capability.Sign/Verify` (constant-time compare); LE certs via tailscaled. Never hand-roll |
| V9 Communications | yes | TLS ≥1.2 on the share server (server.go:415); tailnet-only bind (assertTailnetBindIP) |

### Known Threat Patterns
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Different-account or tagged tailnet node requests a cap | Spoofing / EoP | `requireSameUser` fail-closed rules (Pattern 2) |
| Viewer keeps control after share off | EoP | Revoke with reason + post-subscribe re-check (Pattern 3) |
| Cross-site page on the user's Mac fetches the cap | Info disclosure | POST + JSON content type + no CORS headers; TLS blocks DNS rebinding |
| Cross-site WebSocket hijack | Tampering | Existing exact Origin allowlist (origin_mw.go) |
| Link leaks via Referer | Info disclosure | `Referrer-Policy: no-referrer` |
| Peer-controlled URL used for outbound requests | Tampering (SSRF-ish) | Daemon builds base URL from Tailscale status |
| Lingering public Funnel config after code removal | Info disclosure | Manual `tailscale serve status`/`reset` check |
| Revoke page leaks session identity | Info disclosure | Fixed copy, no host/name/ID (UI-SPEC) |
| Banner renders wire text | Tampering (UI spoof) | Fixed copy keyed on constants (T-175-06-02) |

**Residual, unchanged by this phase:** the relay loopback port (127.0.0.1) accepts any local process, including other macOS users on the same Mac, for local sessions and the remote proxy. This predates Phase 4 and is outside its scope. Note it in the phase SECURITY.md.

## Sources

### Primary (HIGH confidence)
- `tailscale.com@v1.98.3` module source: `client/local/local.go:300-361, 685-700`, `client/local/cert.go:56-108`, `client/tailscale/apitype/apitype.go:35-42`, `tailcfg/tailcfg.go:285-310, 348-420, 594-601`, `ipn/ipnstate/ipnstate.go` (Status, PeerStatus)
- `github.com/coder/websocket@v1.8.15/close.go:65-84`
- Repo files read this session: `internal/webserver/{server.go, capability_mw.go, origin_mw.go, tailscale.go, funnel_client.go, auth.go, capability_test_helpers.go}`, `internal/capability/{capability.go, keystore.go, joincode.go, errors.go, context.go}`, `internal/daemon/{api.go, process.go, engine.go, types.go, remote_caps.go, remote_ws_proxy.go, relay_remote_files.go, remote_files.go, tailnet_cache.go, client.go}`, `internal/relay/{hub.go, server.go}`, `internal/tailnet/{tailnet.go, sessions.go}`, `internal/attach/attach.go`, `cmd_attach.go`, `cmd_remote.go`, `cmd_cli.go`, `main.go`, `app.go`, `frontend/src/{App.tsx, lib/relayClient.ts, lib/remoteSession.ts, lib/sidebarPeers.ts, lib/tailscale.ts, components/WebShareSessionView.tsx, components/TerminalPanel.tsx, components/ContextMenu.tsx, components/Share/SessionShareModal.tsx}`, `cmd/playwright-fixture/main.go`, `tests/check-traceability-paths.sh`, `TESTING.md`, `wails.json`, `go.mod`

### Secondary (MEDIUM confidence)
- Phase docs: 04-CONTEXT.md, 04-UI-SPEC.md, STATE.md, 03-SECURITY.md, 03-REVIEW.md

### Tertiary (LOW confidence)
- Behavior of tagged-node identities and browser Referer defaults (Assumptions A1, A3)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH. No new deps. API shapes read from the pinned module source.
- Architecture: MEDIUM-HIGH. Built on verified code paths. The share-record placement and single connect path are recommendations.
- Removal inventory: HIGH. File:line from reads and `git grep` this session. Line numbers drift as plans edit files, so plans should anchor on symbol names.
- Pitfalls: HIGH for 1, 2, 4, 6-9 (observed in code); MEDIUM for 3 and 5.

**Research date:** 2026-11-16
**Valid until:** 2026-12-16 (stable: pinned deps; the code itself changes with each plan)
