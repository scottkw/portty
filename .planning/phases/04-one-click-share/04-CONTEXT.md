# Phase 4: One-Click Share - Context

**Gathered:** 2026-11-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Sharing is one on/off switch per session. Shared means both: visible in the user's tailnet peers' Portty sidebars and reachable through a tailnet-only web link, always with full control. Turning Share off disconnects every remote viewer (peer GUI, browser, CLI). `portty attach <host>:<session>` works from any terminal. Funnel, public links, the risk panel, consent gates, read-only links, join codes and the LAN fallback server are removed. Requirements: SHAR-01..06.

Not in this phase: 12h+ idle survival and reconnect (SHAR-07, Phase 5); update download flow (Phase 6).

</domain>

<decisions>
## Implementation Decisions

### Who can use a share
- Peer Macs get access automatically when Tailscale WhoIs (via the local tailscaled client, `tailscale.com/client/local`) shows the requesting node is logged in as the same tailnet user as this Mac. No join code. Portty still mints a capability behind the scenes, so the capability-gated model stays intact (only issuance is automated).
- The web link carries the full-control capability token. Anyone on the tailnet who has the link can use it (treat it like a password); the server stays tailnet-only.
- Turning Share off revokes every capability for that session and disconnects all viewers (peer GUI, browser, CLI). Turning it back on mints a new link.
- Share state persists across daemon and app restarts.

### The Share control
- The toolbar Share button opens a small popover with one "Share this session" switch. The sidebar row context menu also has "Share" / "Stop Sharing".
- When shared, the popover shows the link in one field with a Copy button and one line: "Anyone on your tailnet with this link has full control."
- Join codes, the QR code, and the "Open" button are removed (radical simplicity; same-user access replaces join codes).
- The shared indicator stays as today: the signal icon on the sidebar row and the blue "Shared" toolbar state.

### Cleanup and the no-Tailscale case
- Remove the password-gated LAN web server fallback (Phase 1 review WR-01/WR-02: `internal/daemon/process.go` LAN auto-start, `app.go` StartWebServer LAN fallback). With Tailscale off, Share is dimmed with its reason, as now.
- Settings ▸ Sharing shrinks to Tailscale status plus a short explanation. The web server starts automatically the first time a session is shared (and stops when nothing is shared, at Claude's discretion).
- Remove the unused public-write consent-gate state (`SetRWGate` / `isRWGated`) with Funnel.
- Remove CLI `serve` / `web start`. Add `portty share <session>` and `portty unshare <session>`, which flip the same switch.
- Remove all Funnel, public-link, risk-panel, hold-to-confirm, read-only link code, UI, settings and tests (SHAR-02, SHAR-05).

### portty attach from any terminal
- Target syntax `portty attach <host>:<session>`: host is the tailnet machine's MagicDNS short name; session is the session name, or an ID prefix when names clash.
- Bare `portty attach <host>` lists that Mac's shared sessions and attaches straight away if exactly one is shared.
- CLI access uses the same automatic same-user WhoIs check as the GUI; no code or token to paste.
- When the host stops sharing mid-attach: restore the terminal, remove the status bar, print "Portty: <host> stopped sharing <session>." and exit 0.

### Claude's Discretion
- Whether the web server stops when the last share ends; exact popover layout within Phase 2/3 tokens; how capabilities are persisted and revoked; handling of a non-same-user tailnet peer (not listed / not authorized); session naming collisions; test strategy for WhoIs (fake local client).

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `tailscale.com/client/local` is already used in `internal/tailnet/tailnet.go`, `internal/webserver/server.go`, `internal/webserver/tailscale.go` (WhoIs is available there).
- Capability store and middleware: `internal/capability/`, `internal/webserver/capability_mw.go`, `requireCapability`; peer probe with `X-Portty` marker (Phase 3).
- CLI attach with tmux-style status bar: `cmd_attach.go`, `internal/attach/`, `internal/statusbar/`.
- Share UI today: `frontend/src/components/Share/` (SessionShareModal, FunnelRiskPanel, ShareSegmentedControl) and `frontend/src/components/SessionShare/` (TailnetTab, InternetReadOnlyTab, InternetFullAccessTab, HoldToConfirmButton, ShareLinkCard); `RemoteJoinCodeModal.tsx`; QR via `github.com/skip2/go-qrcode` in `app.go` and `cmd_cli.go`.

### Established Patterns
- Daemon-persisted settings with PATCH routes and validation (appearance, terminal font).
- Sidebar context menu (Phase 2) and Settings sheet panes.

### Integration Points
- Peer sidebar list comes from `DiscoverAndProbe` (port 7444, X-Portty). Peers currently need a saved capability from a join code; that flow becomes automatic issuance after a same-user WhoIs.
- Funnel loopback listener serves the same mux (server.go ~475) — removed with Funnel.

</code_context>

<specifics>
## Specific Ideas

- Both Macs need the new build at the same time (wire contract changed in Phase 3 and changes again here).
- Native-GUI checks go in the Tart VM; the VM has no Tailscale, so tailnet flows need fakes in tests plus the deferred two-Mac UAT.

</specifics>

<deferred>
## Deferred Ideas

- Phase 2 UAT items 2-5 and Phase 3 M-70/M-73/M-74 (need both Macs, a signed build, a public repo).
- Restored tabs replaying the old first-prompt % mark (261116-ku9 follow-up).
- Phase 5: accelerated idle tests instead of a real overnight wait.

</deferred>
