---
gsd_state_version: "1.0"
current_phase: 3
current_phase_name: Portty Brand
status: executing
stopped_at: Phase 3 UI-SPEC approved
last_updated: "2026-11-16T15:48:59.581Z"
last_activity: 2026-11-15
last_activity_desc: Phase 2 execution started
state_head: 293fc1e51d4be43367d5fa3dd7000fc97f372e56
progress:
  total_phases: 6
  completed_phases: 1
  total_plans: 31
  completed_plans: 25
  percent: 17
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-11-15)

**Core value:** A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.
**Current focus:** Phase 2 — Tabs + Sidebar Navigation

## Current Position

Phase: 3 (Portty Brand) — READY TO EXECUTE
Plan: 1 of 12
Status: Ready to execute
Last activity: 2026-11-15 — Phase 2 execution started

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

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 4]: Phase 1 review WR-01/WR-02 (deferred by verifier): the daemon auto-starts a password-gated LAN web server when Tailscale is off (`internal/daemon/process.go:114-145`), StartWebServer falls back to a LAN-only self-signed server when Tailscale is up but tailnet HTTPS certs are off (`app.go:736-743`), and CLI `serve` has no Tailscale check. Phase 4's tailnet-only share must close all three.
- [Phase 2]: Flaky vitest `SessionShareModal.test.tsx:813-821` (SM-02 tabpanel) needs `vi.waitFor`; the Share modal moves in Phase 2 anyway.
- [Phase 2]: LOOK-01 (2026-11-15) — GUI redesign to a macOS-native look, folded into Phase 2 so the UI is redone once with tabs + sidebar and THEM-01. A mockup artifact is being made to feed discuss/planning. UI-REVIEW 01 findings (banner hex colors, disabled-reason tooltips) get fixed as part of this.
- [Phase 2]: THEM-01 (2026-11-15) — remove the theme library/picker; app chrome + xterm follow prefers-color-scheme live (desktop and web share). Check OSC/tray glyphs and the WebGL renderer under both modes.
- [Phase 2]: Remove chat entirely (CHAT-01, decided 2026-11-15) — ChatPanel/ChatBadge/MentionPopover/TerminalChatHost, chat store + routes, relay chat frames, FuzzSanitizeChatContent, chat CORS (FilesCORS naming). Chat panel, Share modal, Funnel risk panel, and web-share session view live under `frontend/src/components/Hub/`. Hub removal must move them, not delete them. Remote-peer polling in `App.tsx` only runs while the Hub tab is active.
- [Phase 3]: Peer discovery probes port 7443 for AgentHub's "capability required" marker. If AgentHub still runs on either Mac, Portty may treat it as a peer or collide on the port. Decide on the port and marker during the rebrand. Both Macs need the rebranded build at the same time.
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
- [Phase 5]: The office Mac must not sleep overnight. Keep-awake (CONV-01) is v2, so the overnight test needs manual `caffeinate`/Energy settings. The home Mac sleeping overnight needs reconnect-on-wake.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 261115-rc8 | Phase 2 UI polish: sidebar vibrancy material, terminal padding, LAN-mode wording, type/spacing leftovers | 2026-11-15 | 3251a80e | [261115-rc8-phase-2-ui-polish-sidebar-vibrancy-mater](./quick/261115-rc8-phase-2-ui-polish-sidebar-vibrancy-mater/) |
| 261116-c9f | Sidebar "native, alive" refresh: header glyphs, New Session row and +, peer host rows with status dots, compact footer | 2026-11-16 | b501b8f8 | [261116-c9f-sidebar-native-alive-refresh](./quick/261116-c9f-sidebar-native-alive-refresh/) |

## Deferred Items

Items acknowledged and deferred at milestone close, most recent first:

| Category | Item | Status | Deferred At | Milestone |
|----------|------|--------|-------------|-----------|
| *(none)* | | | | |

## Deferred Verification

| Phase | State | Resume |
|-------|-------|--------|
| 2 | verification_deferred_human | /gsd-verify-work 2 |

Phase 2 UAT items 2-5 (peer attach, Share/Tailscale live state, web page appearance, VoiceOver) need both Macs. The user deferred them on 2026-11-16 ("push that UAT until later and move on"). Item 1 passed.

## Session Continuity

Last session: 2026-11-16T14:42:46.774Z
Stopped at: Phase 3 UI-SPEC approved
Resume file: .planning/phases/03-portty-brand/03-UI-SPEC.md
