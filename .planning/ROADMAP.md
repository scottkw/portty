# Roadmap: Portty

## Milestones

- ✅ **v1.0 Portty v1.0** — Phases 1-6 (shipped 2026-11-18)
- ✅ **v1.1 Hardening** — Phases 7-8 (shipped 2026-11-19)
- ✅ **v1.2 Previews, Remote URLs, Mobile** — Phases 9-16 (shipped 2026-11-23)
- 🚧 **v2.0 Desktop Sharing** — Phases 17-23 (in progress)

## Phases

<details>
<summary>✅ v1.0 Portty v1.0 (Phases 1-6) — SHIPPED 2026-11-18</summary>

- [x] Phase 1: Plain macOS Terminal (12/12 plans) — completed 2026-11-15
- [x] Phase 2: Tabs + Sidebar Navigation (13/13 plans) — completed 2026-11-16 (two-Mac UAT deferred)
- [x] Phase 3: Portty Brand (6/6 plans) — completed 2026-11-16 (M-70/M-73/M-74 deferred)
- [x] Phase 4: One-Click Share (10/10 plans) — completed 2026-11-17 (live two-Mac repro deferred)
- [x] Phase 5: Overnight Survival (16/16 plans) — completed 2026-11-18 (M-84/M-85, VERF-01 deferred)
- [x] Phase 6: Release Updates (4/4 plans) — completed 2026-11-18 (M-93 first real release deferred)

Full details: `.planning/milestones/v1.0-ROADMAP.md`. Audit: `.planning/milestones/v1.0-MILESTONE-AUDIT.md`.

</details>

<details>
<summary>✅ v1.1 Hardening (Phases 7-8) — SHIPPED 2026-11-19</summary>

- [x] Phase 7: Hardening (5/5 plans) — completed 2026-11-18
- [x] Phase 8: Debt Cleanup (6/6 plans) — completed 2026-11-19

Full details: `.planning/milestones/v1.1-ROADMAP.md`. Audit: `.planning/milestones/v1.1-MILESTONE-AUDIT.md`.

</details>

<details>
<summary>✅ v1.2 Previews, Remote URLs, Mobile (Phases 9-16) — SHIPPED 2026-11-23</summary>

- [x] Phase 9: Upgrades Keep Sessions (7/7 plans) — completed 2026-11-20
- [x] Phase 10: Input Path Reliability (7/7 plans) — completed 2026-11-21
- [x] Phase 11: File Preview (10/10 plans) — completed 2026-11-21
- [x] Phase 12: File Path Click (9/9 plans) — completed 2026-11-22
- [x] Phase 13: Remote Localhost URLs (7/7 plans) — completed 2026-11-22
- [x] Phase 14: Mobile Web App (14/14 plans) — completed 2026-11-22
- [x] Phase 15: v1.2 Hardening (9/9 plans) — completed 2026-11-23
- [x] Phase 16: v1.2 Cleanup (4/4 plans) — completed 2026-11-23

Full details: `.planning/milestones/v1.2-ROADMAP.md`. Audit: `.planning/milestones/v1.2-MILESTONE-AUDIT.md`. dompurify 3.4.18 follows in a v1.2.1 patch (on or after 2026-11-28).

</details>

### 🚧 v2.0 Desktop Sharing

**Milestone Goal:** Use the office Mac's whole desktop from home (in the Portty app, a browser, or an iPhone) as easily as its terminal today, and let Portty updates install themselves.

**Holds for every phase:** Terminal sessions and their upgrade handoff never regress. Reliability comes first, because nobody can get to the unattended office Mac to fix it. Links stay token-gated and tailnet-only. There is no STUN, TURN, relay or cloud dependency. The CSP stays unchanged except for what a recorded security review approves. The stack stays Wails v2, the look stays macOS-native, and simplicity is the default. Every phase that adds, renames or removes tests updates TESTING.md: the Section 2 manifest, Section 4 traceability rows (repo-relative paths, checked with `bash tests/check-traceability-paths.sh`), and Section 5 M-NN items for anything the Tart VMs cannot prove. Capture, hardware encode, TCC, lock-screen, display and iPhone checks run on the user's real Macs as M-NN rows. The Tart VMs have no hardware encoder or TCC automation, and research subagents never touch this host's live tailscaled.

- [x] **Phase 17: Desktop Feasibility Spike** - A throwaway capture → encode → pion → browser path on a real Mac settles every risky assumption with a recorded go/no-go, and the capture-host architecture is decided from that evidence (completed 2026-11-25)
- [x] **Phase 18: Self-Installing Updates** - Update verifies, replaces `/Applications/Portty.app`, relaunches, cleans up the DMG and rolls back a version that won't start; a first launch from a mounted DMG moves itself into place; sessions survive (completed 2026-11-26; human verification deferred, see 18-UAT.md)
- [x] **Phase 18.1: Apple Silicon Release Pipeline** (INSERTED) - One arm64-only build published under both DMG names, inside-out signing with the v1.2 designated requirement, every nested binary signed, notarized and verified in CI, minimum macOS 14 enforced at build time (completed 2026-11-26; SC2 runtime proof deferred to Phase 23 rc.1, see 18.1-UAT.md)
- [x] **Phase 19: Unattended Desktop Viewing** - Always-available desktop for your own Macs: guided permissions, live view in Chrome and Safari with nobody at the Mac, visible state, display kept awake, automatic reconnect, media on the tailnet only (completed 2026-11-26; real-Mac verification deferred, see 19-UAT.md)
- [x] **Phase 20: Remote Control** - Pointer, keyboard, Send-keys menu and explicit clipboard from any viewer; the sharer picks a display or a single window and switches displays mid-session (completed 2026-11-27; real-Mac verification deferred, see 20-UAT.md)
- [x] **Phase 21: Portty App and iPhone Viewers** - A Desktop tab opened from the sidebar in the Portty app, with Cmd shortcuts forwarded in control mode, and an iPhone viewer with trackpad mode and touch controls (completed 2026-11-27; real-device verification deferred, see 21-UAT.md)
- [ ] **Phase 22: Click-to-Share for Tailnet People** - A desktop link other tailnet people can open, an Admit prompt by tailnet identity, view-only until granted, instant stop and revoke, a "shared with N" indicator, an audit log, and soft control hand-off
- [ ] **Phase 23: Audio, Adaptive Quality, Unlock and Release** - System audio (starts muted), sharp-when-still and smooth-when-moving video, remote unlock shipped or cut per the spike, real-Mac hardening, and the v2.0 release (rc.1 dry run, then the real tag) on the Phase 18.1 pipeline

## Phase Details

### Phase 17: Desktop Feasibility Spike
**Goal**: Before any production desktop code is written, a throwaway end-to-end path on a real Mac shows the desktop live over the tailnet and settles each risky assumption with a recorded go or no-go. The capture-host architecture is decided from that evidence, so every later phase plans against what was proven rather than what was hoped.
**Depends on**: Nothing (first v2.0 phase)
**Requirements**: FEAS-01, FEAS-02
**Success Criteria** (what must be TRUE):
  1. On a real Apple Silicon Mac, a throwaway page opened from another device over the tailnet shows that Mac's desktop moving live (ScreenCaptureKit → VideoToolbox H.264 → pion → browser), with no STUN, TURN or cloud server involved. (FEAS-01)
  2. A recorded go/no-go matrix gives a measured result, with its evidence, for every FEAS-01 question:
     - which process holds Screen Recording and Accessibility: a launchd helper or the daemon, including a daemon respawned over SSH, and with the GUI closed
     - WebRTC video and data channel inside the Wails web view
     - connections over the Tailscale interface from Chrome, Safari, the Portty app and iPhone Safari, with full ICE vs ICE-lite per browser
     - lock-screen frames and CGEvent typing on the office Mac's macOS
     - display sleep and wake
     - monthly Screen Recording re-approval
     - CPU, latency and sharpness on the MacBook Air at the default 2560 px, 30 fps cap
     - Opus audio encode

     (FEAS-01)
  3. The capture-host decision (bundled helper or in-daemon) is written down with the evidence behind it before Phase 19 is planned. Every no-go names the requirement it changes or cuts, for example UNLK-01, or DVWR-01 falling back to a browser link. (FEAS-02)
  4. No spike code reaches the shipping build. Release builds and the full terminal test suite are unchanged by this phase.
**Plans**: 12/12 plans complete

Plans:
**Wave 1**
- [x] 17-01-PLAN.md — Tracer: synthetic frames → VideoToolbox → pion → receiver on loopback; matrix skeleton with the FEAS-02 decision rule; SC4 baseline

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 17-02-PLAN.md — ScreenCaptureKit capture and vmprobe; Spike.app signed inside-out (Developer ID); permission-probe kit for every launch path

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 17-03-PLAN.md — R00 (SCK in the Tart guest) and row routing; R10 grant across a same-identity rebuild and bundle swap

**Wave 4** *(blocked on Wave 3 completion)*
- [x] 17-04-PLAN.md — Helper shape (SMAppService): attribution, Settings entries, AX, GUI closed, SSH kickstart

**Wave 5** *(blocked on Wave 4 completion)*
- [x] 17-05-PLAN.md — In-daemon shape: LaunchAgent (ProcessType reference rows), GUI spawn, SSH spawn, SSH kickstart

**Wave 6** *(blocked on Wave 5 completion)*
- [x] 17-06-PLAN.md — Lock-screen frames, typing and continuity per shape; provisional FEAS-02 pick

**Wave 7** *(blocked on Wave 6 completion)*
- [x] 17-09-PLAN.md — AudioConverter Opus and SCK audio; monthly re-approval store and expiry behaviour

**Wave 8** *(blocked on Wave 7 completion)*
- [x] 17-07-PLAN.md — VM re-key (user); live desktop over the tailnet in Safari (full vs lite), join latency, binding

**Wave 9** *(blocked on Wave 8 completion)*
- [x] 17-08-PLAN.md — Chrome over the tailnet; WebRTC in a throwaway Wails v2 shell

**Wave 10** *(blocked on Wave 9 completion)*
- [x] 17-10-PLAN.md — host-run (test chart, perf timeline, scheduled sleep/wake) and a VM rehearsal

**Wave 11** *(blocked on Wave 10 completion)*
- [x] 17-11-PLAN.md — User session: iPhone row, then the user-started run on the MacBook Air; revoke

**Wave 12** *(blocked on Wave 11 completion)*
- [x] 17-12-PLAN.md — FEAS-02 decision, requirement changes, TESTING.md Category BH, VM cleanup, SC4 proof, archive

**Cross-cutting constraints:** <!-- gsd:generated -->
- Probe runs never overlap: before each run no other spike process runs in the guest, and each record carries the pid used to filter its tccd lines

**Notes**: File Apple's persistent-content-capture entitlement request alongside the spike. Until Apple grants it, someone at the office Mac has to approve Screen Recording about once a month, and DPRM-03 makes that visible.

### Phase 18: Self-Installing Updates
**Goal**: Clicking Update installs the new Portty by itself. The verified DMG replaces `/Applications/Portty.app`, Portty relaunches, the DMG is cleaned up, and terminal sessions survive through the v1.2 takeover. A version that will not start is rolled back. A first launch from a mounted DMG moves Portty into place by itself.
**Depends on**: Nothing new in v2.0. It reuses the v1.0 verify chain and the v1.2 session takeover, is independent of Phase 17, and can run alongside it. It must ship before or with the first release that bundles a capture helper.
**Requirements**: UPD-05, UPD-06, UPD-07, UPD-08, UPD-09
**Success Criteria** (what must be TRUE):
  1. With two ticking terminal sessions and a share link on, clicking Update on build N installs build N+1 once the DMG passes the existing SHA-256, Team ID, signing identifier and notarization checks. `/Applications/Portty.app` is replaced, Portty relaunches as N+1, each shell keeps its PID and history, the share link still works, and the update DMG is ejected and deleted. This is shown on a clean VM with the two-build rig and once on the office Mac. (UPD-05, UPD-06)
  2. When N+1 fails to start or never reports healthy, Portty puts build N back in `/Applications` and runs it by itself, with sessions intact, and says the update did not take. (UPD-07)
  3. Opening Portty from a mounted, quarantined DMG moves it to `/Applications` (or `~/Applications` when `/Applications` is not writable), relaunches from there, ejects the DMG and moves the downloaded DMG to the Trash. Two copies never end up running. (UPD-08)
  4. Every install failure (target not writable, App Management refusal, mount or copy failure, disk full) falls back to today's Reveal-in-Finder flow with a clear message. No relaunch ever stops at a Gatekeeper dialog. (UPD-09)
**Plans**: 10/10 plans executed

Plans:
**Wave 1**
- [x] 18-01-PLAN.md — Tracer: Update on N stages, hands off to a detached installer copy, swaps, relaunches, waits for health, deletes the DMG (in-process); staging-seam and installer-seam hardening

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 18-08-PLAN.md — VM rig and R1-lite (the tracer on real macOS); probes P1, P2, P7, P8 before any plan that consumes them
- [x] 18-04-PLAN.md — Banner and alert copy: Update button, installing and restarting states, revealed-with-error warning, no tray offer after a failed install

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 18-10-PLAN.md — Installer runs from a whole-bundle ditto copy of the running Portty.app (P2d CONTRADICTED the lone-executable copy); signed VM re-run P2d-bundle, which must pass before rollback work

**Wave 4** *(blocked on Wave 3 completion)*
- [x] 18-03-PLAN.md — UPD-07 rollback: GUI-alive health, guarded kills, relaunch retry (A9), keep-daemon relaunch, "did not take" message, failed version remembered; VM rows R2-A..R2-D and A9-fix

**Wave 5** *(blocked on Wave 4 completion)*
- [x] 18-02-PLAN.md — UPD-09 GUI side: preflight, disk space, full app trust chain, one text per failure cause; a failed version (or an unreadable skip record) gets no automatic offer; asset names universal plus portty-vX.Y.Z-darwin-arm64.dmg (user-confirmed 2026-11-25)
- [x] 18-05-PLAN.md — UPD-06: old app to the Trash after health, startup sweep of interrupted installs; P3, R1-trash, S1

**Wave 6** *(blocked on Wave 5 completion)*
- [x] 18-06-PLAN.md — UPD-08 mover: probes P4-P6, disk-image launch detection, installer move kinds (DMG trashed on every install path), mover mode with the Replace question; R4-smoke

**Wave 7** *(blocked on Wave 6 completion)*
- [x] 18-07-PLAN.md — Final VM matrix on one build: R1 with history and share link, R2-A/R2-B in derived windows, R3, F1-F7, every row backed by a rig capture; HealthTimeout tuning

**Wave 8** *(blocked on Wave 7 completion)*
- [x] 18-09-PLAN.md — Final VM mover matrix R4-a..R4-g on the same build; TESTING.md Category BI M-165..M-169; closure record with M-169 and SC1's office-Mac half human_needed

### Phase 18.1: Apple Silicon Release Pipeline (INSERTED)

**Goal**: The release pipeline builds Portty once for Apple Silicon only, with minimum macOS 14, and publishes that build under both DMG names, so installed v1.2.x copies and builds from Phase 18 on both find, verify and install it. CI signs every Mach-O inside out without changing the TCC identity, and notarizes and verifies the result. Pulled forward from Phase 23 (user approved 2026-11-25) because none of it depends on Phases 19-22.
**Depends on**: Phase 18 (its updater accepts the `portty-vX.Y.Z-darwin-arm64.dmg` name alongside the universal one)
**Requirements**: REL-01
**Success Criteria** (what must be TRUE):
  1. release.yml and build.yml build with `wails build -platform darwin/arm64`, and release builds set both CGO_CFLAGS and CGO_LDFLAGS to `-mmacosx-version-min=14.0` (build.sh too). build.yml checks every Mach-O's minos is 14.0 or later and that it has only an arm64 slice, so a PR catches a regression. build/darwin/Info.plist has LSMinimumSystemVersion 14.0. (REL-01)
  2. One arm64 build is published as both `portty-vX.Y.Z-darwin-universal.dmg` and `portty-vX.Y.Z-darwin-arm64.dmg`. Each DMG is signed with `--identifier` equal to its name without .dmg, notarized, stapled and checked the way the updater checks it, and both are in checksums.txt. An installed v1.2.x copy still finds, verifies and installs the release through the universal name. (REL-01)
  3. Signing is inside out per Mach-O (no `codesign --deep`), keeps identifier `app.portty.Portty`, hardened runtime, timestamp and the unchanged entitlements file, and CI fails on unsigned code, the wrong team, a missing runtime flag, a non-arm64 slice, a minos below 14.0 or a designated requirement that differs from v1.2's. The signing step is written so a provisioning profile and the persistent-content-capture entitlement can be added later without other changes; the profile is added only once Apple grants request T558S9Q5B3. (REL-01)
  4. release_workflow_test.go and the other release tests pin both asset names, identifier equals name for each, the arm64-only build flag, no `--deep` and the verify loop. If the build.yml matrix context changes, TESTING.md Section 3 and the branch protection on `main` move to the new check name in the same change. (REL-01)
**Plans**: 4/4 plans executed

Plans:
**Wave 1**
- [x] 18.1-01-PLAN.md — Tracer: build.sh builds arm64-only for macOS 14 (both CGO flags) and the new scripts/verify-bundle.sh proves it on the real bundle; Info.plist 14.0; build.yml builds darwin/arm64 and runs the gate and its fixture tests on every PR (context build (portty, darwin/arm64, macos-latest))

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 18.1-02-PLAN.md — Inside-out signing (scripts/sign-bundle.sh, entitlement-ready, no deep signing) and the --signed/--notarized gate pinning team, hardened runtime, timestamp, entitlements and the v1.2 designated requirement; proven with the host Developer ID on a scratch copy; build.sh signs through both

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 18.1-03-PLAN.md — release.yml: arm64 build with macOS 14 flags, bundle gates, inside-out signing; one build published as both DMG names from one suffix list (identifier = name, notarized, stapled, verified, checksums, release-notes line, 210-min timeout); Go pins against DMGAssetNames (needs Phase 18's 18-02)

**Wave 4** *(blocked on Wave 3 completion)*
- [x] 18.1-04-PLAN.md — TESTING.md (REL-01 rows, build-script group of 2, renamed check context and one-check protection payload, Category BI.1 first-tag-run item); merge checkpoint, then the delegated branch protection on main, recorded in Section 3

### Phase 19: Unattended Desktop Viewing
**Goal**: Turn on always-available desktop on the office Mac once, then open its live desktop from home in Chrome or Safari any time, with nobody at the Mac and the GUI closed. Permissions are guided, the desktop's state is always visible, and the display stays awake while someone is watching. Viewers reconnect by themselves, desktop media never leaves the tailnet, and terminal sessions and their upgrade handoff are untouched.
**Depends on**: Phase 17 (the capture-host decision, FEAS-02); Phase 18.1 (macOS 14 minimum, arm64 build and signing pipeline)
**Requirements**: DPRM-01, DPRM-02, DPRM-03, DPRM-04, DESK-01, DESK-02, DESK-03, DESK-04, DSHR-01, DSHR-02, DSHR-08, DVWR-02
**Success Criteria** (what must be TRUE):
  1. Portty guides the user through granting Screen Recording and Accessibility, and a test capture confirms both work. With always-available desktop on, the user's own devices (same Tailscale account) open the Mac's live desktop from a link in desktop Chrome and Safari. The picture is sharp on Retina and fitted to the window, with the Portty GUI quit and nobody at the Mac. (DPRM-01, DSHR-01, DESK-01, DVWR-02)
  2. The host and every viewer see the desktop's real state: permission missing, waiting for approval at the Mac, display asleep, locked, or streaming to N viewers. When the Screen Recording approval lapses, Portty shows "waiting for approval at the Mac" and sends a native notification. While any viewer is connected the display stays awake (display-awake and system-sleep assertions, lid-closed sleep excepted), and a stream ended by display sleep restarts by itself after the wake, tested on battery. (DPRM-02, DPRM-03, DPRM-04)
  3. Several viewers can watch at once, and a new viewer gets a picture right away. Each viewer comes back by itself after a network drop, Mac sleep and wake, or a Portty upgrade, showing "Reconnecting" in the meantime. Always-available links, including one saved to an iPhone Home Screen, keep working across daemon restarts and upgrades. (DESK-03, DESK-04, DSHR-02)
  4. Desktop media stays on the tailnet. No STUN, TURN or cloud server is contacted, and no desktop socket binds a non-tailnet address; tests assert both. The office Mac and the home Mac reach each other on a direct tailnet path (tailscale ping reads direct). A desktop link never opens a terminal session and a terminal link never opens the desktop; cross-token tests refuse both. (DESK-02, DSHR-08)
  5. Terminal sessions never regress. With the desktop share on, a Portty upgrade still hands every terminal session to the new daemon with PIDs and history intact. Turning the desktop share on or off never takes the share server away from terminal viewers, and the reverse holds too. (DSHR-02, DESK-03; guards the validated UPGR-01..05)
**Plans**: 17/17 plans executed

Plans:
**Wave 1**
- [x] 19-01-PLAN.md — Tracer: always-available link on loopback (desktop token under its own HKDF key, /desktop/ and /desktop/ws signalling v1, synthetic source into a pion answer); screencap.Source interface and State enum
- [x] 19-02-PLAN.md — Daemon under launchd: own LaunchAgent, kickstart-only start path (no -k, no detached spawn), typed repair errors, legacy agent cleanup, re-home after upgrade

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 19-03-PLAN.md — Media server hardening: tailnet-only bind, no STUN/TURN/mDNS, multi-viewer with IDR on join, viewer cap, size events plus rebuild, data-channel hook (8192-byte SCTP), per-viewer rights
- [x] 19-04-PLAN.md — Route gates: desktop-off and rejected-link pages, /desktop/check, DSHR-08 cross-token matrix, CSP pin, manifest without start_url, /desktop redirect
- [x] 19-05-PLAN.md — Native capture: ScreenCaptureKit main display, rebuildable VideoToolbox encoder with hardware read-back, power/lock/AC reads, pointer nudge, preflight confined to one file
- [x] 19-17-PLAN.md — GUI on the launchd start path: agent install from installed bundles, repair state with Open Login Items, re-home retry

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 19-06-PLAN.md — Browser viewer: desktopview mode, desktopClient (pluggable transport, pre-redial hook, wake events, backoff), DesktopView states and video rect, CSP e2e
- [x] 19-07-PLAN.md — VM launchd rows (install, kickstart, repair state, re-home, stop, legacy end state)
- [x] 19-08-PLAN.md — Daemon wiring: desktop.json, /desktop API, serverNeeded(), health poller, userspace refusal, revocation, production DisplaySource
- [x] 19-09-PLAN.md — State and recovery: precedence and push, Locked(), stall watchdog, lapse signals and one-sender claim, viewer and AC-only assertions, restart after wake, crash-loop breaker

**Wave 4** *(blocked on Wave 3 completion)*
- [x] 19-10-PLAN.md — Handoff with the desktop on: listener passed under serverNeeded, viewers reconnect, HandoffVersion stays 1
- [x] 19-11-PLAN.md — Onboarding: perm-probe subcommand, permission and test-capture endpoints, Set Up Desktop Sharing sheet, daemon notifier wiring

**Wave 5** *(blocked on Wave 4 completion)*
- [x] 19-12-PLAN.md — VM proof of the real media path: Safari and Chrome over the tailnet with the GUI quit, CSP, binds and packets, assertions, probe, notification path

**Wave 6** *(blocked on Wave 5 completion)*
- [x] 19-13-PLAN.md — Settings ▸ Sharing Desktop group (switch, status, Copy Link, permissions, footnote) and security copy
- [x] 19-14-PLAN.md — Tray Desktop line and the GUI half of the lapse-notification claim

**Wave 7** *(blocked on Wave 6 completion)*
- [x] 19-15-PLAN.md — Final VM matrix: upgrade with desktop on and live terminal sessions (SC5), GUI flow, multi-viewer, restarts, closure

**Wave 8** *(blocked on Wave 7 completion)*
- [x] 19-16-PLAN.md — TESTING.md: Section 2/4/5 updates for all new tests and M-NN rows
**Notes**: The largest change to existing code is the share server's lifetime: it must stay up while either a terminal share or the desktop share needs it. Its handoff path changes too. Desktop sessions are not handed off; they reconnect.
**UI hint**: yes

### Phase 20: Remote Control
**Goal**: A viewer drives the shared Mac as if sitting at it, with pointer, keyboard, system shortcuts and clipboard. The sharer chooses a whole display or a single window, and displays can be switched mid-session.
**Depends on**: Phase 19
**Requirements**: DCTL-01, DCTL-02, DCTL-03, DCTL-05, DCTL-06, DESK-05, DESK-06
**Success Criteria** (what must be TRUE):
  1. A viewer with control moves, clicks, scrolls and drags, and the pointer lands on the right spot on Retina and non-Retina displays at any viewer window size. (DCTL-01)
  2. A viewer types text with shortcuts and non-ASCII characters (accents, emoji, input-method text) and it arrives as typed. No key or modifier stays held after the viewer loses focus, closes or disconnects. A Send-keys menu sends Cmd+Tab, Cmd+Space and Lock Screen, and an optional Ctrl→Cmd swap serves viewers on non-Mac keyboards. (DCTL-02, DCTL-03)
  3. The sharer picks a whole display or a single window to share, and displays can be switched mid-session without the viewer reconnecting. In a window share, the window comes to the front when controlled and clicks never land outside it. (DESK-05, DESK-06, DCTL-05)
  4. Text copies from the shared Mac to the viewer, and pastes from the viewer to the shared Mac, only by an explicit action and never in the background. Items a password manager marks as concealed never sync. (DCTL-06)
**Plans**: 11/11 plans executed

Plans:
**Wave 1**
- [x] 20-01-PLAN.md — Tracer: a click on the `input` data channel reaches the event bridge at the right global point (stale epoch and locked presses dropped); tagged private-source bridge, closed key table, text chunking, own-account rights in hello; wire contract frozen in golden files

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 20-02-PLAN.md — Daemon input: keys and text, release-all on every trigger plus 3 s dead-man, press-only gates, optimistic Lock Screen, click count, wheel, relative move with cursor report, rate limits, "Control needs Accessibility" state
- [x] 20-03-PLAN.md — Display and window picker and mid-session switch on the running stream (no renegotiation): epoch bump, fallback to main display, resize, still-frame fallback
- [x] 20-06-PLAN.md — Viewer input layer: channel before offer, pointer and keyboard capture, IME, Cmd-chord tap rule, Ctrl/Cmd swap, release chord, release triggers and heartbeat

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 20-04-PLAN.md — Window-share confinement: AX focus gate, raise-then-drop with verified raise ladder, clamped moves, topmost hit test, raise-failed and minimized states
- [x] 20-07-PLAN.md — Control bar: picker, standalone Send Keys menu with Lock Screen confirm, swap toggle, View Only / Control, Full Screen with Keyboard Lock, availability table

**Wave 4** *(blocked on Wave 3 completion)*
- [x] 20-05-PLAN.md — Daemon clipboard: read on request only, types checked first (concealed/transient never read), 16 KiB chunks under the read limit, strict reassembly, per-viewer rights
- [x] 20-08-PLAN.md — Clipboard UI (Safari-safe write, Copy and Paste fallback popovers, notices) and persistent state chips

**Wave 5** *(blocked on Wave 4 completion; needs 19-12 and 19-15 merged)*
- [x] 20-09-PLAN.md — Tart VM proof of all real-macOS control rows and the PASTE_ROW verdict (user checkpoints for VM permission clicks)

**Wave 6** *(blocked on Wave 5 completion; needs 19-13 and 19-14 merged)*
- [x] 20-10-PLAN.md — Host side: Settings Showing row, Control needs Accessibility, changed Phase 19 strings, tray window-only suffix, conditional Paste from Other Apps row

**Wave 7** *(blocked on Wave 6 completion; needs 19-16 merged)*
- [x] 20-11-PLAN.md — TESTING.md: Sections 2 and 4 plus a Phase 20 Section 5 category numbered after Phase 19's last M-NN
**UI hint**: yes

### Phase 21: Portty App and iPhone Viewers
**Goal**: The office Mac's desktop opens in a Desktop tab of the Portty app on the home Mac, straight from the sidebar, and on an iPhone with touch controls. Both use the same viewer and protocol as the browser link.
**Depends on**: Phase 20
**Requirements**: DVWR-01, DVWR-03, DVWR-04
**Success Criteria** (what must be TRUE):
  1. In the Portty app on another Mac, the sidebar lists a peer's desktop next to its sessions. Clicking it opens a Desktop tab that shows and controls that desktop, with the same state and reconnect behaviour as the browser viewer. If Phase 17 found WebRTC unusable in the Wails web view, the row opens the browser link instead and the docs say so. (DVWR-01)
  2. In control mode in a Desktop tab, Cmd+W, Cmd+Q and Cmd+C go to the remote Mac instead of closing the tab, quitting Portty or copying locally. Leaving control mode gives Portty its shortcuts back. (DVWR-04)
  3. On an iPhone, in Safari or from the Home Screen, the viewer offers trackpad mode, pinch-zoom, long-press for right-click, two-finger scroll and the soft keyboard, and it picks up again after the phone sleeps. (DVWR-03)
**Plans**: 13/13 plans executed

Plans:
**Wave 1**
- [x] 21-01-PLAN.md — Office side: `X-Portty-Desktop: always` on the sessions meta answer, same-user `POST /api/peer/desktop/cap`, desktop token provider, tailnet header reader and `FetchPeerDesktopCap`
- [x] 21-02-PLAN.md — Key-routing probe in the real Portty.app in portty-test-2 (page-level, then blank-in-place, then menu rebuild); writes KEYROUTE, CMDKEYUP, RELEASE_CHORD, HIDDEN_VISIBILITY verdicts (user checkpoint)
- [x] 21-03-PLAN.md — iPhone gesture engine and zoom/pan math as pure code (relative moves, host-side pointer)
- [x] 21-04-PLAN.md — Sidebar Desktop row with muted state line and the `desktop` tab type and tab chrome (frontend only)
- [x] 21-05-PLAN.md — Desktop tab connection logic: key refresh before redial, check mapping, Local Network watcher, 2-minute hidden pause, app card copy

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 21-06-PLAN.md — Home daemon: in-memory desktop key store, `POST /peers/desktop`, loopback signalling proxy `GET /api/relay/remote-desktop/{host}/ws` with one automatic key refresh

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 21-07-PLAN.md — Bindings (`ConnectPeerDesktop`, `OpenPeerDesktopInBrowser`, `OpenLocalNetworkSettings`), desktop flag in the peer poll, `NSLocalNetworkUsageDescription`

**Wave 4** *(blocked on Wave 3 completion; gated on 20-06, 20-07, 20-08 SUMMARY.md)*
- [x] 21-08-PLAN.md — The Desktop tab: additive DesktopView props, DesktopTab, Local Network and Refused cards, hidden tab and window rules, toolbar label

**Wave 5** *(blocked on Wave 4 completion; gated on Phase 20 SUMMARY.md files)*
- [x] 21-09-PLAN.md — Control mode: exits with release-all, Cmd+V through the explicit paste message (concealed never sent), native fallback only if KEYROUTE calls for it (gated on 20-05, 20-06, 20-07, 20-08)
- [x] 21-10-PLAN.md — iPhone viewer: touch layer, bottom bar and key bar, soft keyboard, Tap to start after sleep (gated on 20-01, 20-02, 20-06, 20-07, 20-08)

**Wave 6** *(blocked on Wave 5 completion; gated on 20-06, 20-09)*
- [x] 21-11-PLAN.md — Tart VM rows: Desktop row and tab in the real app, Local Network denied state (user checkpoints)

**Wave 7** *(blocked on Wave 6 completion; gated on 20-02, 20-05, 20-06, 20-08, 20-09)*
- [x] 21-12-PLAN.md — Tart VM rows: Local Network allowed and heal, hidden tab and window, one tab per peer, Cmd+W/Q/C forwarded and given back, no held keys, Cmd+V (user checkpoint)

**Wave 8** *(blocked on Wave 7 completion; gated on 20-02, 20-11)*
- [x] 21-13-PLAN.md — TESTING.md Sections 2, 4, 5 (M-NN at run time), README Help paragraphs, real iPhone and real-Mac sessions (user checkpoint)
**UI hint**: yes

### Phase 22: Click-to-Share for Tailnet People
**Goal**: The sharer shows their desktop to a colleague on the tailnet with one click. Anyone on the tailnet can open the link, but each guest waits at an Admit prompt that names them, watches view-only until the sharer grants control for that person, and can be cut off at once. The shared Mac always shows who is watching and keeps a local audit log. Terminal links stay limited to the host's own account.
**Depends on**: Phase 20 (control and clipboard, which are granted per person). It builds on the desktop token and share server from Phase 19.
**Requirements**: DSHR-03, DSHR-04, DSHR-05, DSHR-06, DSHR-07, DCTL-04
**Success Criteria** (what must be TRUE):
  1. Share Desktop creates a link that a person signed in to a different Tailscale account on the same tailnet can open. That same person still gets the own-devices refusal on any terminal link. (DSHR-03)
  2. Each guest waits at an Admit prompt on the shared Mac that shows their tailnet identity. An admitted guest watches view-only: their pointer, keys and clipboard actions do nothing until the sharer allows control, and separately clipboard, for that person. (DSHR-04)
  3. The sharer can stop the share or revoke one viewer at any time, which ends that connection at once, and the link stops working when the share ends. The shared Mac shows join and leave notifications and a persistent "shared with N" indicator with one-click stop. (DSHR-05, DSHR-06)
  4. Every join, leave and control grant is written to a local audit log on the shared Mac. (DSHR-07)
  5. With several viewers allowed to control, whoever typed last holds control for about 2 seconds while the others wait. The person physically at the shared Mac always wins over remote input. (DCTL-04)
**Plans**: 9/13 plans executed

Plans:
**Wave 1**
- [x] 22-01-PLAN.md — Guest token (own HKDF key), `requireTailnetPeer(Page)`, `/desktop/guest/` page, assets and ws routes, two static pages, extended WhoIs fake, route-table and cross-token tests
- [x] 22-02-PLAN.md — JSON Lines audit appender (0600, 5 MiB rotation, never blocks a join) and the guest name sanitizer / Identity type
- [x] 22-03-PLAN.md — Listen-only event tap and Secure Event Input probe; pure arbiter: physical wins (fails closed on an unhealthy tap), 2 s lease

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 22-04-PLAN.md — Manager guest path: admission queue, limits, 5-minute timeout, deny by user ID, re-admit, no PeerConnection before Admit, media bound to the admitted node (gated on 20-04, 20-05)

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 22-05-PLAN.md — Live grants (`SetRights`), revoke and Stop in the safe order with concurrent close, lease and physical-wins in `pressAllowed`, lease messages

**Wave 4** *(blocked on Wave 3 completion)*
- [x] 22-06-PLAN.md — Daemon guest share lifecycle, GUI events stream (doubles as GUI liveness), admit/deny/rights/disconnect on the daemon socket only, audit wiring (gated on 20-10)
- [x] 22-07-PLAN.md — Guest page mode in DesktopView and desktopClient: guest cards, chips, close reasons, bar restrictions (gated on 20-07, 20-08, 21-08, 21-10)

**Wave 5** *(blocked on Wave 4 completion)*
- [x] 22-08-PLAN.md — GUI holds the events stream; tray slot (Share Desktop…, Desktop shared with N, Stop Sharing) and sharing icon variant

**Wave 6** *(blocked on Wave 5 completion)*
- [x] 22-09-PLAN.md — Wails bindings, Admit brings the window forward, coalesced join/leave notifications, Quit GUI Only ends the share

**Wave 7** *(blocked on Wave 6 completion)*
- [ ] 22-10-PLAN.md — Share Desktop sheet, Watching list with grant switches, Settings row, Security string

**Wave 8** *(blocked on Wave 7 completion)*
- [ ] 22-11-PLAN.md — Admit sheet (no default button, Esc inert, 600 ms guard), Secure Event Input and Share-stopped banners, quit sentence

**Wave 9** *(blocked on Wave 8 completion)*
- [ ] 22-12-PLAN.md — Tart VM rows G-01..G-10 and SC1 on portty-test/portty-test-2 (user checkpoints: VM permission clicks, optional second Tailscale user, real-Mac items; Appendix B recorded PARTIAL)

**Wave 10** *(blocked on Wave 9 completion)*
- [ ] 22-13-PLAN.md — TESTING.md (M-NN at run time), README access-rules paragraph, recorded security review 22-SECURITY.md

Every plan opens with a hard `test -f` gate on the upstream 20-/21-/22- SUMMARY.md files it needs and stops if one is missing.
**Notes**: This phase widens who can reach a desktop. MOB-05 still holds for terminal links. The phase therefore gets a recorded security review covering admit, revoke, cross-token and wrong-identity cases.
**UI hint**: yes

### Phase 23: Audio, Adaptive Quality, Unlock and Release
**Goal**: Viewers hear the shared Mac, and the picture adapts to what is on screen and to the network. Remote unlock either ships, if Phase 17 proved it, or is cut and documented. Hardening on real Macs shows v2.0 is safe to leave running unattended, and v2.0 ships through the Phase 18.1 release pipeline after a v2.0.0-rc.1 dry run.
**Depends on**: Phase 22 and Phase 18.1 (the release pipeline). The UNLK-01 outcome is set by Phase 17's lock-screen results.
**Requirements**: DCTL-07, DESK-07, UNLK-01 (REL-01 moved to Phase 18.1 on 2026-11-25; this phase runs the v2.0 release on that pipeline)
**Success Criteria** (what must be TRUE):
  1. A viewer hears the shared Mac's audio along with the picture. Audio starts muted on the viewer, with one tap or click to unmute. (DCTL-07)
  2. The picture is sharp, with small text readable, when the screen is still, and stays smooth while scrolling or dragging windows. It adapts as the network gets worse or better without stalling other viewers. (DESK-07)
  3. Remote unlock has one of two outcomes, set by the Phase 17 evidence. If it is feasible, a viewer on the host's own account sees the lock screen of a locked, logged-in Mac and types the password to unlock it. The password is never stored or logged, and click-to-share guests never get unlock. If it is not feasible, the requirement is cut and the docs say "unattended desktop works while the Mac is awake and unlocked". (UNLK-01)
  4. A `v2.0.0-rc.1` draft release (never offered to installed copies) runs the Phase 18.1 pipeline end to end and both DMGs verify, then the real v2.0 release ships. An installed v1.2.x copy updates to v2.0 through the universal name, and a Phase 18 build updates through self-install. (release run of the REL-01 pipeline built in Phase 18.1)
  5. On a real Mac, a two-build upgrade with a live desktop viewer and live terminal sessions keeps every terminal session and brings the desktop viewer back by itself. A one-hour desktop soak on the office Mac ends with no crash, leak or stuck state. (DESK-03, final hardening)
**Plans**: 0/16 plans executed

Plans:
**Wave 1**
- [ ] 23-01-PLAN.md — Daemon audio: same SCStream, AudioConverter Opus ~64 kbps stereo track, audio failure never touches the picture (DCTL-07)
- [ ] 23-06-PLAN.md — Browser sound: audio transceiver + stereo=1 munge, DesktopSoundButton starting muted, hint, host strings (DCTL-07)

**Wave 2** *(blocked on Wave 1 completion)*
- [ ] 23-02-PLAN.md — Adaptive bitrate: pion TWCC/GCC starting at max, minimum-of-viewers with hysteresis, bitrate→fps→resolution ladder via encoder rebuild, slow-viewer exclusion (DESK-07)

**Wave 3** *(blocked on Wave 2 completion)*
- [ ] 23-03-PLAN.md — Sharp when still: refine after ~300 ms of no new content, idle refresh (DESK-07)
- [ ] 23-04-PLAN.md — Daemon unlock: binary 0x01 message (never a Go string), own-account `unlock` cap, wake/type/check/retry once, 5 s rate limit, 15 min pause after 3 failures (UNLK-01)

**Wave 4** *(blocked on Wave 3 completion)*
- [ ] 23-05-PLAN.md — Unlock audit events, re-lock fail-secure after last own-account viewer leaves, lock-time input drop (UNLK-01; gated on 22-02, 22-03)
- [ ] 23-07-PLAN.md — Unlock This Mac panel (11 states), guest Locked chip, blocked-input notice, CSP e2e check (UNLK-01)

**Wave 5** *(blocked on Wave 4 completion)*
- [ ] 23-08-PLAN.md — Slow connection chip, iPhone Sound item, app-tab playing indicator, phone locked-state bar rules (gated on 21-08, 21-10)

**Wave 6** *(blocked on Wave 5 completion)*
- [ ] 23-09-PLAN.md — VM proof of audio and adaptive quality (checkpoint: VM permission clicks)

**Wave 7** *(blocked on Wave 6 completion)*
- [ ] 23-10-PLAN.md — VM proof of remote unlock (VM admin password only)

**Wave 8** *(blocked on Wave 7 completion)*
- [ ] 23-11-PLAN.md — SC5 VM rehearsal: two-build upgrade with live viewer and sessions, one-hour VM soak

**Wave 9** *(blocked on Wave 8 completion)*
- [ ] 23-12-PLAN.md — TESTING.md (Sections 2, 4, 5; M-NN at run time) and README

**Wave 10** *(blocked on Wave 9 completion)*
- [ ] 23-13-PLAN.md — v2.0.0-rc.1 and rc.2 dry run on the Phase 18.1 pipeline (checkpoint: user pushes tags)

**Wave 11** *(blocked on Wave 10 completion)*
- [ ] 23-14-PLAN.md — Owner real-Mac session: M-161, M-162, M-169..M-179 incl. M-177 rc.1→rc.2 upgrade, office soak (checkpoint; conditional libopus decision)

**Wave 12** *(blocked on Wave 11 completion)*
- [ ] 23-15-PLAN.md — Go/no-go table, publish decision (one-way checkpoint), user pushes v2.0.0 tag

**Wave 13** *(blocked on Wave 12 completion)*
- [ ] 23-16-PLAN.md — Verify published v2.0.0 and upgrade paths (v1.2 universal name, Phase 18 self-install), M-172
**UI hint**: yes

## Progress

**Execution Order:** Phases run in numeric order: 17 → 18 → 18.1 → 19 → 20 → 21 → 22 → 23. Phase 18.1 depends only on Phase 18. Phase 18 does not depend on Phase 17 and can run alongside it.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 17. Desktop Feasibility Spike | v2.0 | 12/12 | Complete    | 2026-11-25 |
| 18. Self-Installing Updates | v2.0 | 10/10 | Complete    | 2026-11-26 |
| 18.1. Apple Silicon Release Pipeline (INSERTED) | v2.0 | 4/4 | Complete    | 2026-11-26 |
| 19. Unattended Desktop Viewing | v2.0 | 17/17 | Complete    | 2026-11-26 |
| 20. Remote Control | v2.0 | 11/11 | Complete    | 2026-11-27 |
| 21. Portty App and iPhone Viewers | v2.0 | 13/13 | Complete    | 2026-11-27 |
| 22. Click-to-Share for Tailnet People | v2.0 | 9/13 | In Progress | - |
| 23. Audio, Adaptive Quality, Unlock and Release | v2.0 | 0/16 | Not started | - |
