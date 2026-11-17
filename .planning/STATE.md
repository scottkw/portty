---
gsd_state_version: "1.0"
current_phase: 04
current_phase_name: One-Click Share
status: verifying
stopped_at: Completed 04-09-PLAN.md
last_updated: "2026-11-17T19:48:44.141Z"
last_activity: 2026-11-17
last_activity_desc: Phase 04 execution started
state_head: 231ee45da5766d8939c2c452e1bcf5c754476659
progress:
  total_phases: 6
  completed_phases: 1
  total_plans: 40
  completed_plans: 40
  percent: 17
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-11-15)

**Core value:** A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.
**Current focus:** Phase 04 — One-Click Share

## Current Position

Phase: 04 (One-Click Share) — EXECUTING
Plan: 9 of 9
Status: Phase complete — ready for verification
Last activity: 2026-11-17 — Phase 04 execution started

Progress: [██░░░░░░░░] 17%

## Performance Metrics

**Velocity:**
- Total plans completed: 12
- Average duration: -
- Total execution time: 0.0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 1 | 12 | - | - |

**Recent Trend:**
- Last 5 plans: -
- Trend: -

*Updated after each plan completion*
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 1 P01 | 8 min | 3 tasks | 103 files |
| Phase 01 P02 | 26 min | 3 tasks | 103 files |
| Phase 01 P03 | 21 min | 3 tasks | 54 files |
| Phase 01 P04 | 17 min | 3 tasks | 35 files |
| Phase 01 P05 | 9 min | 3 tasks | 32 files |
| Phase 01 P06 | 14m | 2 tasks | 40 files |
| Phase 01 P07 | 15m | 2 tasks | 40 files |
| Phase 01 P08 | 8m | 2 tasks | 25 files |
| Phase 01 P09 | 20 min | 3 tasks | 33 files |
| Phase 01 P10 | 19min | 3 tasks | 60 files |
| Phase 01 P11 | 12min | 3 tasks | 53 files |
| Phase 01 P12 | 12min | 3 tasks | 30 files |
| Phase 3 P01 | 7 min | 3 tasks | 71 files |
| Phase 03 P02 | 28 min | 3 tasks | 15 files |
| Phase 03 P03 | 10 min | 2 tasks | 69 files |
| Phase 03 P04 | 25min | 2 tasks | 13 files |
| Phase 3 P05 | 15min | 3 tasks | 21 files |
| Phase 3 P06 | 53min | 3 tasks | 17 files |
| Phase 4 P01 | 35min | 3 tasks | 67 files |
| Phase 4 P02 | 20min | 2 tasks | 39 files |
| Phase 4 P03 | 35min | 2 tasks | 25 files |
| Phase 4 P04 | 45min | 2 tasks | 28 files |
| Phase 04 P05 | 36min | 2 tasks | 57 files |
| Phase 4 P06 | 11min | 3 tasks | 34 files |
| Phase 04 P07 | 11min | 3 tasks | 7 files |
| Phase 04 P08 | 93 min | 3 tasks | 12 files |
| Phase 04 P09 | 14 min | 2 tasks | 9 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Roadmap]: Strip-down before rebrand, so the rename (about 179 files reference "agenthub" today) touches the smallest tree
- [Roadmap]: Navigation before rebrand, so help and in-app text are rewritten once against the final UI
- [Roadmap]: NAV-02 (peer sessions in sidebar) ships with Hub removal so GUI remote attach never regresses (it currently lives in Hub remote cards)
- [Roadmap]: Overnight survival is its own phase so the overnight-length test doesn't hold up same-day share verification
- [Phase 1]: 01-01: Renamed internal/daemon/ipc_nonwindows.go to ipc.go so no *nonwindows* file stays tracked
- [Phase 1]: 01-01: godbus and tailscale/go-winio stay in go.mod as indirect transitive deps (wails linux backend, tailscale safesocket); only direct requires removed
- [Phase 1]: 01-01: release.yml publishes only the signed DMG + checksums.txt; user must re-apply branch protection (build (agenthub, darwin/universal, macos-latest) + playwright) when CI is re-enabled
- [Phase 01]: Web-share guests get no session-menu chevron for any cap; the webFilesEnabled prop and the web bootstrap /info perms probe are removed
- [Phase 01]: RemoteJoinCodeModal and App joinModalForSession keep only 'open-session' and 'hub-modal' intents (dead 'files' intent dropped)
- [Phase 01]: IssueCapabilitiesResponse.homeDir removed from hand-maintained bindings with SessionInfo.homeDir/browseEnabled; Go keeps sending them until 01-03
- [Phase 01]: CodeMirror removed with local pnpm 12; lockfile stays lockfileVersion 9.0, removal-only, frozen install passes
- [Phase 01]: Caps mint only read / read,write; legacy read,files.read tokens stay read-only via whole-token HasPerm(perms,"write")
- [Phase 01]: Kept FilesCORS/FilesPreflight/isAllowedFilesOrigin names (chat CORS), RemoteCapStore + /api/remote-files/caps routes + wrapRelayWithRemoteFiles (remote attach); only comments updated
- [Phase 01]: isRWGated now has no production reader after originAllowedForWrite removal; SetRWGate still written by funnel-write handlers — flagged for later cleanup
- [Phase 01]: wailsapp/mimetype stays in go.mod as // indirect (wails/v2 assetserver imports it)
- [Phase 01]: 01-04: Every session spawns GetShellPath() with exactly -l via an unexported spawnCommand field; tests override with SetSpawnCommandForTest("/bin/cat", nil)
- [Phase 01]: 01-04: CreateRequest drops cli/args; legacy keys are ignored by the plain json.Decoder (T-01-15, asserted by TestHandleCreateSession_LegacyBodyIgnored)
- [Phase 01]: 01-04: Remaining non-test 'opencode' strings (app.go displayNameForCLI, internal/pty/detect.go) left for 01-06 / 01-05 which delete those symbols
- [Phase 01]: 01-04: Tab and hubSessions seed keep cli: 'shell' until 01-07 removes SessionInfo.cli / Tab.cli
- [Phase 1]: 01-05: resolveDefaultShellPath = $SHELL only if absolute, existing, executable, non-directory; else /bin/zsh; shared by GetShellPath display and spawn
- [Phase 1]: 01-05: kept knownShells stale-path filter (drops shell-valued overrides for non-shell keys incl. tailscale), minus Windows names; legacy agent cliPaths keys load as inert data
- [Phase 01]: Status is derived from process state only: running / exited (code 0 or killed) / errored (non-zero); the TS normalizer maps any other value to running
- [Phase 01]: GroupCounts drops the waiting field; Hub attention means errored only
- [Phase 01]: KillSession emits session:status exited (killed sessions read as exited)
- [Phase 01]: 01-07: Frontend reads no cli/cliType; session names fall back to 'shell'; web-share warning gated on enabled && !warned for every session
- [Phase 01]: 01-07: Deleted all agent CSS palettes plus the consumerless .hub-card__chip--agent and .hub-card__badge rules
- [Phase 1]: 01-08: webserver session resolver is (name, status, hostname string, ok bool); /api/sessions/{id}/info 404s on !ok (empty-cliType sentinel removed)
- [Phase 1]: 01-08: pty Session.CLI / CreateRequest.CLI (spawn binary path) kept — internal spawn path, not wire agent identity
- [Phase 01]: 0x35/0x36 are RETIRED relay wire bytes (CHAT-01); read pumps have no case and no default arm, so old clients' inject frames are dropped without a PTY write
- [Phase 01]: ChatPanel isReadOnly state and its /info fetch removed with the inject gesture (sole reader); server-side PTY input gating unchanged
- [Phase 01]: Security fuzz harness (STRP-04): FuzzParseFrame, FuzzInputAbsorberFilter, FuzzSanitizeChatContent, FuzzRequireCapability, FuzzWebServerRoutes plus FuzzVerify; crashers that pin a fixed bug are committed under testdata/fuzz
- [Phase 01]: assetsNoStore returns 404 for non-UTF-8 /assets/ paths (was 500 via fs.Sub ErrInvalid; found by FuzzWebServerRoutes)
- [Phase 01]: 01-10: Exit detection is one app-wide 500 ms watcher (app.go startExitWatcher/checkExits) with no deadline and no cold-start baseline; every session:exit closes the tab and KillSession removes the session, also for tab-less sessions
- [Phase 01]: 01-10: init/retryInit remove sessions already stopped at restore instead of restoring them as tabs
- [Phase 01]: 01-10: SessionCardGrid sort and FLIP reorder deleted with the attention key; the Hub keeps arrival order and group counts are { total } only
- [Phase 01]: 01-10: Daemon derived status and its GetSessionStatus route stay for CLI/web/peers; only the GUI binding and all status display are gone
- [Phase 01]: Every session spawns the constant /bin/zsh -l (sessionShell); $SHELL and legacy shellPath are ignored
- [Phase 01]: loadSettingsFromDisk rewrites settings.json only on an old schemaVersion now that the stale-path filter is gone
- [Phase 01]: sendNotification is the single native notification path; QuitGUIOnly calls it through a.sendNotificationFunc (Phase 6 reuses it)
- [Phase 01]: TailscaleHealth is LocalAPI-only and fail-closed: connected only when tailscaled answers with BackendState Running; the zero value means not connected
- [Phase 01]: The Tailscale poller emits its first result unconditionally (lastHealth nil sentinel) so a launch-time disconnect is reported
- [Phase 01]: tailscaleOff is false while health is null (unchecked, and always in web mode); the NET-01 sentence lives once in frontend/src/lib/tailscale.ts
- [Phase 3]: Module path is github.com/scottkw/portty; all 57 importing .go files rewritten by one sed, zero new gofmt diffs
- [Phase 3]: Portty stores settings, capability.key and daemon.sock in os.UserConfigDir()/portty (0700), fresh start, no migration from the old directory
- [Phase 3]: LaunchAgent label is portty-daemon so it sits beside any old upstream agent
- [Phase 03]: Portty web/peer port is 7444 (tailnet.DefaultProbePort); the daemon binds the same constant the probe dials
- [Phase 03]: Peer discovery requires X-Portty: 1 with 200 or 401 on GET /api/sessions; the probe no longer reads the body, so upstream-app peers are not shown; the header is an identity hint, not authentication
- [Phase 03]: Update check reads scottkw/portty via go-selfupdate Filters matching portty-v<x.y.z>-darwin-universal.dmg; no-release and error paths stay silent
- [Phase 03]: 03-03: tray icons load as one template NSImage with an 18 px and a 36 px rep (18x18 pt each); trayIconBytesForState returns the (1x, @2x) pair, error pair still wins
- [Phase 03]: 03-03: portty-brand/, build/gen_icon.go and build/gen_progress_icons.go deleted; each brand asset has one source
- [Phase 03]: 03-03: scripts/verify-icons.sh uses only sips/cmp/stat; Windows, Linux and ImageMagick checks dropped
- [Phase 03]: 03-04: wails.json name and outputfilename both Portty, giving build/bin/Portty.app; bundle ID app.portty.Portty in both Info.plist templates
- [Phase 03]: 03-04: About Portty uses the native AppKit standard panel via about_darwin.go/.m and the menuShowAbout seam; only credits and a blank build version are passed
- [Phase 03]: 03-04: GitHub Help item and openGitHubCallback removed, not renamed
- [Phase 03]: 03-04: quit-to-menu-bar copy lives in pure quitToMenuBarBody(n) (0/1/n), identifier portty.quit-gui-only, title Portty
- [Phase 3]: 03-05: Web pages change text only (Portty in the system font, no logo or CSS changes); the live /dashboard and /join routes are tested for copy and the CSP header
- [Phase 3]: 03-05: Folder memory key is portty:lastWorkDir with no migration (it resets once to home)
- [Phase 3]: 03-05: Negative test assertions use patterns that never spell the old name out (App.test namespaced theme-key regex)
- [Phase 3]: 03-06: Brand gate is tests/check-brand.sh run from build.yml (bracket-class patterns; exit 0/1/2), not inline negated shell
- [Phase 3]: 03-06: Release DMG is a plain hdiutil UDZO image signed with codesign; no create-dmg or package-manager install; first exercised in Phase 6
- [Phase 3]: 03-06: RELEASE_PUBLISH_TOKEN logic kept, comment only; Phase 6 revisits publishing
- [Phase 3]: 03-06: build.yml context renamed to build (portty, darwin/universal, macos-latest); branch protection re-apply is a manual user step when CI is re-enabled
- [Phase 4]: 04-01: relay.Subscriber.ReadOnly and the webserver HasPerm(claims.Perms, "write") check stay as the capability write gate; only ways to create a view-only link were removed
- [Phase 4]: 04-01: issueCapabilitiesForSession mints one read,write grant; IssueCapabilitiesResponse keeps only writeUrl until 04-02 replaces it with the share switch
- [Phase 4]: 04-01: Section 5 Category J (share warning GUI switch) left for 04-05
- [Phase 4]: 04-02: Share state is one in-memory daemon record per session behind GET/PUT /sessions/{id}/share; the share server runs exactly while a record exists (no boot server, no LAN fallback)
- [Phase 4]: 04-02: A taken share port is ErrPortInUse (port_in_use), never a random port; only EADDRINUSE maps to it
- [Phase 4]: 04-02: App share bindings return ShareResult with errorCode (daemon codes, start_failed/stop_failed, unavailable for GetSessionShare) and never throw
- [Phase 4]: 04-02: Key regeneration re-mints every share link; exit and kill drop the record via unshareLocked
- [Phase 4]: 04-02: vmpreview build tag fakes Tailscale on 127.0.0.1:7444 for the Tart VM; check-brand.sh fails if it reaches .github, build.sh or scripts
- [Phase 4]: Peer capability route returns the share record's existing token (one token per share); GET falls through to 404
- [Phase 4]: Revoke order everywhere: ClearGrants, DisableSession, RevokeWebViewers(relay.CloseReasonShareStopped), then Stop; key regeneration clears grants before kicking
- [Phase 4]: Same-account meta 200 carries no X-Portty marker; only the 403 refusal does
- [Phase 4]: POST /peers/connect is the one viewing-side path for CLI and GUI: host from tailscaled's peer list, base URL from the peer's DNS name, cap stored in the daemon
- [Phase 4]: FetchPeerSessionsMetaWithClient returns (sessions, MetaStatus, error) so a refused port maps to not_sharing
- [Phase 4]: portty attach resolves a bare argument as a local session first, then a host; host:session is always remote
- [Phase 04]: 04-05: SharePopover takes a shared prop (webEnabled from the poll) so it never flashes off and follows shares made from the sidebar or CLI while open
- [Phase 04]: 04-05: The Share popover stays open when Tailscale drops and shows the reason; turning Share off still works
- [Phase 04]: 04-05: The popover VM check is TESTING.md M-81 (M-78 was taken by 04-02); M-82 covers Tailscale-off share controls
- [Phase 04]: 04-05: Sidebar's Other Macs GlobeAltIcon is kept (peer-group icon, not a removed exposure icon)
- [Phase 4]: 04-06: Clicking the row of an open working peer tab only activates it; ConnectPeerSession runs for a new or connect-failed tab
- [Phase 4]: 04-06: Removal gate narrowed after dry run: TS 'public write' fields and the kept read-only capability model are not removed features; legacy settings fixture out of scope
- [Phase 4]: 04-06: No vmpreview WhoIs fake: M-79's browser path is cap-gated, only meta and peer-cap routes use WhoIs
- [Phase 04]: 04-07: UAT gap 1 root cause not pinned in-process; H-grid and H-net need the 04-08 log and M-83 live repro
- [Phase 04]: 04-07: Hub.ReconnectPreamble strips reply-eliciting terminal queries (liveEmuQueryStripPattern, now with XTVERSION and kitty CSI ? u) so a joining terminal never types answers into the shared shell
- [Phase 04]: 04-07: 32 KiB coder/websocket read limit drops proxied joiners on long sessions; deferred (deferred-items.md)
- [Phase 04]: 04-08: the detached daemon logs to daemon.log (0600, one rotation per start) in its config dir; viewer-diag lines on host and viewing Mac give live evidence for UAT gap 1 (M-83)
- [Phase 04]: 04-08: proxy log errors strip the whole upstream query before redacting the raw token (redactCapTokenFromError alone left cap=<redacted>)
- [Phase 04]: 04-08: relay.DiagID shortens session IDs for viewer-diag lines in hub, webserver and proxy
- [Phase 04]: 04-09: share server session info returns magicDNSLabel(h.Domain), the tailnet short name, not the OS hostname, so the browser stop banner matches the GUI and CLI (UAT gap 2)
- [Phase 04]: 04-09: user decision 2026-11-17 (UAT gap 3) Button opens when shared: with Tailscale off, shareDisabled = tailscaleOff && !activeShared; unshared stays aria-disabled, daemon still refuses share-on

### Pending Todos

- Follow-up: restored tabs after an app restart still start at 80×24, so their scrollback replay can show the old % mark in history (see 261116-ku9 SUMMARY).
- [2026-11-17] [docs] Document multi-line prompt resize artifacts in user docs — [todo file](.planning/todos/pending/2026-11-17-document-multi-line-prompt-resize-artifacts.md)

### Blockers/Concerns

- [Phase 4]: User confirmed 2026-11-16 that both Macs are on the same Tailscale account (same login, so the same numeric user ID for the WhoIs gate). Remaining risk: a node that is ACL-tagged is refused by design; check `tailscale status` shows no tags on either Mac during the two-Mac UAT (M-80).


- [Phase 4]: Phase 1 review WR-01/WR-02 (deferred by verifier): the daemon auto-starts a password-gated LAN web server when Tailscale is off (`internal/daemon/process.go:114-145`), StartWebServer falls back to a LAN-only self-signed server when Tailscale is up but tailnet HTTPS certs are off (`app.go:736-743`), and CLI `serve` has no Tailscale check. Phase 4's tailnet-only share must close all three.
- [Phase 2]: Flaky vitest `SessionShareModal.test.tsx:813-821` (SM-02 tabpanel) needs `vi.waitFor`; the Share modal moves in Phase 2 anyway.
- [Phase 2]: LOOK-01 (2026-11-15) — GUI redesign to a macOS-native look, folded into Phase 2 so the UI is redone once with tabs + sidebar and THEM-01. A mockup artifact is being made to feed discuss/planning. UI-REVIEW 01 findings (banner hex colors, disabled-reason tooltips) get fixed as part of this.
- [Phase 2]: THEM-01 (2026-11-15) — remove the theme library/picker; app chrome + xterm follow prefers-color-scheme live (desktop and web share). Check OSC/tray glyphs and the WebGL renderer under both modes.
- [Phase 2]: Remove chat entirely (CHAT-01, decided 2026-11-15) — ChatPanel/ChatBadge/MentionPopover/TerminalChatHost, chat store + routes, relay chat frames, FuzzSanitizeChatContent, chat CORS (FilesCORS naming). Chat panel, Share modal, Funnel risk panel, and web-share session view live under `frontend/src/components/Hub/`. Hub removal must move them, not delete them. Remote-peer polling in `App.tsx` only runs while the Hub tab is active.
- [Phase 3]: NOTF-01 (v1: update check points at Portty's feed) overlaps DIST-02 (v2: auto-update pulls from Portty's feed). The v1 check must cope with no Portty release existing yet.
- [Phase 4]: Remote attach today uses a join-code/capability handshake (`RemoteJoinCodeModal`). SHAR-01's "one action, no separate toggles" must define how peers get capabilities without weakening the capability model.
- [Phase 4]: Since 01-03, the public-write consent gate state (`SetRWGate`/`isRWGated`) is still written by the funnel-write handlers, but no production code reads it (its only reader, `originAllowedForWrite`, was removed). WebSocket write is still enforced by grants. The Share redesign must decide whether to keep or remove the gate.
- [Phase 6]: The update check uses the anonymous GitHub Releases API, which returns 404 for private repos. `scottkw/portty` must be made public before the first release, or installed copies never see an update.
- [Phase 2, post-execution]: Inherited bug, found in a VM check on 2026-11-15. On a fresh install with no settings.json, `loadSettingsFromDisk` returns early (`internal/daemon/engine.go:99-101`) and `e.pluginSettings` stays zero-valued, so every terminal plugin reports off: webgl, search, links, image with storageLimit 0, and the rest. `defaultPluginSettings()` (`plugin_settings.go:96`, WebGL true) is only applied when a file exists. The bug is also present on main at c2f4255d. Fix: initialise pluginSettings (and shellWebShareWarningEnabled=true) in the constructor or in the missing-file branch, plus a test. Run it as a /gsd-quick after Phase 2 or fold it into the code-review fix pass.
- [Phase 5]: User (2026-11-15) finds the real overnight test excessive. At Phase 5 discuss, propose replacing the 12h-idle criteria with accelerated tests:
  - forced connection drop and auto-reconnect
  - VM sleep/wake
  - shortened token TTLs, noting `funnelReadCodeMaxTTL = 8h` in `internal/daemon/api.go`
  - daemon restart
  - scrollback diff
  Rewrite VERF-01 as a short two-Mac sleep/wake check. Real overnight use becomes an informal check, not a gate. The requirement wording is the user's, so confirm at discuss before changing it.
- [Phase 5]: On a remote `portty attach`, the status bar shows `[reconnecting]` after 5 s with no output, because the host sends no heartbeat frame and the cmd_attach.go watcher keys on received frames. That misleads on any idle session (UAT gap 1 diagnosis, 04-07). Phase 5's reconnect/idle work owns the fix.
- [Phase 5]: The office Mac must not sleep overnight. Keep-awake (CONV-01) is v2, so the overnight test needs manual `caffeinate`/Energy settings. The home Mac sleeping overnight needs reconnect-on-wake.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 261115-rc8 | Phase 2 UI polish: sidebar vibrancy material, terminal padding, LAN-mode wording, type/spacing leftovers | 2026-11-15 | 3251a80e | [261115-rc8-phase-2-ui-polish-sidebar-vibrancy-mater](./quick/261115-rc8-phase-2-ui-polish-sidebar-vibrancy-mater/) |
| 261116-c9f | Sidebar "native, alive" refresh: header glyphs, New Session row and +, peer host rows with status dots, compact footer | 2026-11-16 | b501b8f8 | [261116-c9f-sidebar-native-alive-refresh](./quick/261116-c9f-sidebar-native-alive-refresh/) |
| 261116-ku9 | Terminal Font setting (daemon-persisted, live), first-prompt % fix (PTY sized before spawn + measured grid), Settings copy, plugin save-on-change, single empty-state New Session | 2026-11-16 | a363abd6 | [261116-ku9-terminal-font-setting-and-ui-review-fixe](./quick/261116-ku9-terminal-font-setting-and-ui-review-fixe/) |
| 3 | Terminal font: Nerd Font fallbacks after SF Mono so Powerlevel10k icons render | 2026-11-16 | ffe5a118 | — |
| 5 | Menu bar dead at launch: Dock hand-off after Accessory→Regular switch | 2026-11-16 | 3740b04a | — |
| 6 | Share dialog: 12px gap between link cards | 2026-11-16 | 09da9268 | — |

## Deferred Items

Items acknowledged and deferred at milestone close, most recent first:

| Category | Item | Status | Deferred At | Milestone |
|----------|------|--------|-------------|-----------|
| *(none)* | | | | |

## Deferred Verification

| Phase | State | Resume |
|-------|-------|--------|
| 2 | verification_deferred_human | /gsd-verify-work 2 |
| 3 | verification_deferred_human | /gsd-verify-work 3 |
| 4 | verification_deferred_human | /gsd-verify-work 4 |

Phase 2 UAT items 2-5 (peer attach, Share/Tailscale live state, web page appearance, VoiceOver) need both Macs. The user deferred them on 2026-11-16 ("push that UAT until later and move on"). Item 1 passed.

Phase 3 UAT items M-70 (two Macs), M-73 (signed build) and M-74 (README on GitHub; repo private) cannot run in the VM; deferred on 2026-11-16 the same way. M-71 and M-72 passed in the VM.

## Session Continuity

Last session: 2026-11-17T19:48:42.763Z
Stopped at: Completed 04-09-PLAN.md
Resume file: None
