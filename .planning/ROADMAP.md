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

- [ ] **Phase 17: Desktop Feasibility Spike** - A throwaway capture → encode → pion → browser path on a real Mac settles every risky assumption with a recorded go/no-go, and the capture-host architecture is decided from that evidence
- [ ] **Phase 18: Self-Installing Updates** - Update verifies, replaces `/Applications/Portty.app`, relaunches, cleans up the DMG and rolls back a version that won't start; a first launch from a mounted DMG moves itself into place; sessions survive
- [ ] **Phase 19: Unattended Desktop Viewing** - Always-available desktop for your own Macs: guided permissions, live view in Chrome and Safari with nobody at the Mac, visible state, display kept awake, automatic reconnect, media on the tailnet only
- [ ] **Phase 20: Remote Control** - Pointer, keyboard, Send-keys menu and explicit clipboard from any viewer; the sharer picks a display or a single window and switches displays mid-session
- [ ] **Phase 21: Portty App and iPhone Viewers** - A Desktop tab opened from the sidebar in the Portty app, with Cmd shortcuts forwarded in control mode, and an iPhone viewer with trackpad mode and touch controls
- [ ] **Phase 22: Click-to-Share for Tailnet People** - A desktop link other tailnet people can open, an Admit prompt by tailnet identity, view-only until granted, instant stop and revoke, a "shared with N" indicator, an audit log, and soft control hand-off
- [ ] **Phase 23: Audio, Adaptive Quality, Unlock and Release** - System audio (starts muted), sharp-when-still and smooth-when-moving video, remote unlock shipped or cut per the spike, and a signed, notarized Apple Silicon release hardened on real Macs

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
**Plans**: 6/12 plans executed

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
- [ ] 17-09-PLAN.md — AudioConverter Opus and SCK audio; monthly re-approval store and expiry behaviour

**Wave 8** *(blocked on Wave 7 completion)*
- [ ] 17-07-PLAN.md — VM re-key (user); live desktop over the tailnet in Safari (full vs lite), join latency, binding

**Wave 9** *(blocked on Wave 8 completion)*
- [ ] 17-08-PLAN.md — Chrome over the tailnet; WebRTC in a throwaway Wails v2 shell

**Wave 10** *(blocked on Wave 9 completion)*
- [ ] 17-10-PLAN.md — host-run (test chart, perf timeline, scheduled sleep/wake) and a VM rehearsal

**Wave 11** *(blocked on Wave 10 completion)*
- [ ] 17-11-PLAN.md — User session: iPhone row, then the user-started run on the MacBook Air; revoke

**Wave 12** *(blocked on Wave 11 completion)*
- [ ] 17-12-PLAN.md — FEAS-02 decision, requirement changes, TESTING.md Category BH, VM cleanup, SC4 proof, archive

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
**Plans**: TBD

### Phase 19: Unattended Desktop Viewing
**Goal**: Turn on always-available desktop on the office Mac once, then open its live desktop from home in Chrome or Safari any time, with nobody at the Mac and the GUI closed. Permissions are guided, the desktop's state is always visible, and the display stays awake while someone is watching. Viewers reconnect by themselves, desktop media never leaves the tailnet, and terminal sessions and their upgrade handoff are untouched.
**Depends on**: Phase 17 (the capture-host decision, FEAS-02)
**Requirements**: DPRM-01, DPRM-02, DPRM-03, DPRM-04, DESK-01, DESK-02, DESK-03, DESK-04, DSHR-01, DSHR-02, DSHR-08, DVWR-02
**Success Criteria** (what must be TRUE):
  1. Portty guides the user through granting Screen Recording and Accessibility, and a test capture confirms both work. With always-available desktop on, the user's own devices (same Tailscale account) open the Mac's live desktop from a link in desktop Chrome and Safari. The picture is sharp on Retina and fitted to the window, with the Portty GUI quit and nobody at the Mac. (DPRM-01, DSHR-01, DESK-01, DVWR-02)
  2. The host and every viewer see the desktop's real state: permission missing, waiting for approval at the Mac, display asleep, locked, or streaming to N viewers. When the Screen Recording approval lapses, Portty shows "waiting for approval at the Mac" and sends a native notification. The display wakes when a viewer connects and stays awake while any viewer is connected, except for lid-closed sleep. (DPRM-02, DPRM-03, DPRM-04)
  3. Several viewers can watch at once, and a new viewer gets a picture right away. Each viewer comes back by itself after a network drop, Mac sleep and wake, or a Portty upgrade, showing "Reconnecting" in the meantime. Always-available links, including one saved to an iPhone Home Screen, keep working across daemon restarts and upgrades. (DESK-03, DESK-04, DSHR-02)
  4. Desktop media stays on the tailnet. No STUN, TURN or cloud server is contacted, and no desktop socket binds a non-tailnet address; tests assert both. A desktop link never opens a terminal session and a terminal link never opens the desktop; cross-token tests refuse both. (DESK-02, DSHR-08)
  5. Terminal sessions never regress. With the desktop share on, a Portty upgrade still hands every terminal session to the new daemon with PIDs and history intact. Turning the desktop share on or off never takes the share server away from terminal viewers, and the reverse holds too. (DSHR-02, DESK-03; guards the validated UPGR-01..05)
**Plans**: TBD
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
**Plans**: TBD
**UI hint**: yes

### Phase 21: Portty App and iPhone Viewers
**Goal**: The office Mac's desktop opens in a Desktop tab of the Portty app on the home Mac, straight from the sidebar, and on an iPhone with touch controls. Both use the same viewer and protocol as the browser link.
**Depends on**: Phase 20
**Requirements**: DVWR-01, DVWR-03, DVWR-04
**Success Criteria** (what must be TRUE):
  1. In the Portty app on another Mac, the sidebar lists a peer's desktop next to its sessions. Clicking it opens a Desktop tab that shows and controls that desktop, with the same state and reconnect behaviour as the browser viewer. If Phase 17 found WebRTC unusable in the Wails web view, the row opens the browser link instead and the docs say so. (DVWR-01)
  2. In control mode in a Desktop tab, Cmd+W, Cmd+Q and Cmd+C go to the remote Mac instead of closing the tab, quitting Portty or copying locally. Leaving control mode gives Portty its shortcuts back. (DVWR-04)
  3. On an iPhone, in Safari or from the Home Screen, the viewer offers trackpad mode, pinch-zoom, long-press for right-click, two-finger scroll and the soft keyboard, and it picks up again after the phone sleeps. (DVWR-03)
**Plans**: TBD
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
**Plans**: TBD
**Notes**: This phase widens who can reach a desktop. MOB-05 still holds for terminal links. The phase therefore gets a recorded security review covering admit, revoke, cross-token and wrong-identity cases.
**UI hint**: yes

### Phase 23: Audio, Adaptive Quality, Unlock and Release
**Goal**: Viewers hear the shared Mac, and the picture adapts to what is on screen and to the network. Remote unlock either ships, if Phase 17 proved it, or is cut and documented. v2.0 ships as a signed, notarized Apple Silicon release, and hardening on real Macs shows it is safe to leave running unattended.
**Depends on**: Phase 22. The UNLK-01 outcome is set by Phase 17's lock-screen results.
**Requirements**: DCTL-07, DESK-07, UNLK-01, REL-01
**Success Criteria** (what must be TRUE):
  1. A viewer hears the shared Mac's audio along with the picture. Audio starts muted on the viewer, with one tap or click to unmute. (DCTL-07)
  2. The picture is sharp, with small text readable, when the screen is still, and stays smooth while scrolling or dragging windows. It adapts as the network gets worse or better without stalling other viewers. (DESK-07)
  3. Remote unlock has one of two outcomes, set by the Phase 17 evidence. If it is feasible, a viewer on the host's own account sees the lock screen of a locked, logged-in Mac and types the password to unlock it. The password is never stored or logged, and click-to-share guests never get unlock. If it is not feasible, the requirement is cut and the docs say "unattended desktop works while the Mac is awake and unlocked". (UNLK-01)
  4. Releases are Apple Silicon only with minimum macOS 14, and CI signs, notarizes and verifies every nested binary in the app. An installed v1.2.x copy still finds, verifies and installs the first v2.0 release. v1.2's updater looks only for `portty-v<ver>-darwin-universal.dmg` and that signing identifier, so the naming must stay compatible or both names must ship. (REL-01)
  5. On a real Mac, a two-build upgrade with a live desktop viewer and live terminal sessions keeps every terminal session and brings the desktop viewer back by itself. A one-hour desktop soak on the office Mac ends with no crash, leak or stuck state. (REL-01, DESK-03)
**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:** Phases run in numeric order: 17 → 18 → 19 → 20 → 21 → 22 → 23. Phase 18 does not depend on Phase 17 and can run alongside it.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 17. Desktop Feasibility Spike | v2.0 | 6/12 | In Progress | - |
| 18. Self-Installing Updates | v2.0 | 0/TBD | Not started | - |
| 19. Unattended Desktop Viewing | v2.0 | 0/TBD | Not started | - |
| 20. Remote Control | v2.0 | 0/TBD | Not started | - |
| 21. Portty App and iPhone Viewers | v2.0 | 0/TBD | Not started | - |
| 22. Click-to-Share for Tailnet People | v2.0 | 0/TBD | Not started | - |
| 23. Audio, Adaptive Quality, Unlock and Release | v2.0 | 0/TBD | Not started | - |
