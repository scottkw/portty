# Roadmap: Portty

## Milestones

- ✅ **v1.0 Portty v1.0** — Phases 1-6 (shipped 2026-11-18)
- ✅ **v1.1 Hardening** — Phases 7-8 (shipped 2026-11-19)
- 🚧 **v1.2 Previews, Remote URLs, Mobile** — Phases 9-14 (in progress)

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

### 🚧 v1.2 Previews, Remote URLs, Mobile

**Milestone Goal:** Upgrades stop costing sessions, and a remote session gets easier to work in: preview files, open the remote Mac's `localhost` pages, and drive a session from an iPhone. Everything stays on the tailnet behind the share token.

**Holds for every phase:** token-gated, tailnet-only links; the CSP unchanged except for what a recorded security review approves; no new cloud dependencies; Wails v2; macOS-native look; radical simplicity. VM evidence comes from two Tart VMs on the real tailnet standing in for the office and home Macs, recorded in the phase's VM-CHECKS file and as M-NN items in TESTING.md.

- [x] **Phase 9: Upgrades Keep Sessions** - The new daemon takes over live PTYs and history on upgrade, viewers resume with a delta, a failed handoff loses nothing, the daemon reports an API version, and quitting is clean (completed 2026-11-20)
- [x] **Phase 10: Input Path Reliability** - A bounded input queue keeps viewers connected through long pastes, the paste notice is honest in the app and the browser, peer snapshots are keyed by host, the viewer you're typing on sets the session's size, and the exit-code test is deterministic (completed 2026-11-21)
- [x] **Phase 11: File Preview** - `portty open <file>` shows images, PDFs and Markdown in a preview tab for app and browser viewers, over the session's own stream, under a size cap and a reviewed CSP (completed 2026-11-21)
- [x] **Phase 12: File Path Click** - OSC 7 working-directory tracking, and Cmd-click on a printed file path previews it, local and remote (completed 2026-11-22)
- [x] **Phase 13: Remote Localhost URLs** - Remote `localhost:PORT` links open in the normal browser through a token-gated tailnet proxy with per-port consent (completed 2026-11-22)
- [x] **Phase 14: Mobile Web App** - Browser links limited to your own Tailscale account, then the share view on an iPhone: key bar, phone layout, Home Screen install, reconnect after sleep, and launcher copy for portty.app/app (completed 2026-11-22)
- [ ] **Phase 15: v1.2 Hardening**

## Phase Details

### Phase 9: Upgrades Keep Sessions
**Goal**: Upgrading Portty never costs a session. The new daemon takes over the running shells and their history, every attached viewer carries on by itself, and if the takeover fails the old daemon keeps serving. Nobody has to be at the remote Mac.
**Depends on**: Phase 8 (the DEBT-08 consent guard stays as the fallback)
**Requirements**: UPGR-01, UPGR-02, UPGR-03, UPGR-04, UPGR-05
**Success Criteria** (what must be TRUE):
  1. On the office Tart VM, with two ticking sessions (one inside a full-screen program such as `vim`) and a share link on, installing build B over build A (the M-104 two-build rig) and launching it ends no session and asks nothing. `/health` reports B, each shell keeps its PID, each ticker counts on with no missing number, a viewer opened afterwards gets the pre-upgrade history, and the share link still works with the same token and still refuses a wrong one. (UPGR-01)
  2. Viewers attached during that upgrade (an office local tab, plus a home-VM peer tab, a browser share link and `portty attach` over the real tailnet) reconnect on their own within seconds and resume with a delta. Each viewer's ticker sequence has no repeated or missing number. (UPGR-02)
  3. A handoff that fails at any step loses nothing. Fault injection at each step in tests, plus one VM run that kills the new daemon mid-handoff, shows the old daemon still serving every session and viewer, the GUI or CLI saying the sessions could not be moved, and the DEBT-08 choice (the dialog, or `--restart-daemon`) offered as before. An upgrade from a v1.1 daemon, which cannot hand off, gets the same DEBT-08 choice. (UPGR-03)
  4. `/health` reports an API version. A GUI or CLI facing a daemon it cannot talk to shows a clear "update needed" state (a banner in the app; a one-line message and a non-zero exit from the CLI) instead of failing calls. A compatible older daemon kept with Not Now keeps working. (UPGR-04)
  5. Quitting is clean. `kill -TERM` quits the GUI and leaves the daemon and its sessions running. Quit All during an approved daemon restart cancels the restart and quits within a few seconds, not ~30 s. Cmd-Q while the restart dialog is open never draws a second dialog underneath it (a vitest plus a VM click check). (UPGR-05)
**Plans**: 7/7 plans complete
Plans:
- [x] 09-01-PLAN.md — Lifecycle groundwork: /health API version and "update needed" in CLI and GUI (UPGR-04); ctx-aware EnsureDaemon with Quit All cancel, couldn't-move restart copy, 503 retry; SIGTERM, logout/restart and one-modal quit fixes (UPGR-05, UPGR-03 copy)
- [x] 09-02-PLAN.md — Kernel substrate: pollable PTY master, fd passing (internal/fdpass, peer uid and pid) with lock and listener pins, adopted sessions with kqueue exit status and safe kill, movable share listener with `api` on /info (UPGR-01, UPGR-04)
- [x] 09-03-PLAN.md — Relay state: Freeze/Thaw, HubState export/import with staged hubs, seed refactor carrying the alt screen, round trip vs a reference hub and fuzz, waitable 1012 restart close (UPGR-01, UPGR-02)
- [x] 09-04-PLAN.md — Handoff protocol: /handoff and in-process takeover with two-phase commit over SCM_RIGHTS, thaw after COMMIT only once the takeover has exited, fault injection at every step, old-dies save, SIGTERM guard (UPGR-01, UPGR-03)
- [x] 09-05-PLAN.md — `portty daemon --takeover` with a status pipe, EnsureDaemon deciding from the takeover's outcome with the DEBT-08 fallback, real launch check (UPGR-01, UPGR-03)
- [x] 09-06-PLAN.md — Env-gated two-binary integration test in the VM, local tab and local attach reconnect, TESTING.md, full gate once (all UPGR)
- [x] 09-07-PLAN.md — Two-build VM checks for SC1-SC5: upgrade, viewers, killed takeover and killed old daemon, v1.1 upgrade, update needed, quit paths (all UPGR)
**UI hint**: no

### Phase 10: Input Path Reliability
**Goal**: Pasting into a shared session is trustworthy everywhere. A long paste never gets a healthy viewer dropped, the "paste may have been cut short" notice shows exactly when a paste may really have been cut short (in the app and in the browser), and peer tabs never mix up two Macs.
**Depends on**: Phase 9 (the input queue and pong changes run through the read pump, daemon proxy and viewer reconnect code that the handoff reworks)
**Requirements**: INPT-01, INPT-02, INPT-03, INPT-04, INPT-05, INPT-06
**Success Criteria** (what must be TRUE):
  1. A multi-megabyte paste into a slow program that doesn't echo (for example `stty raw -echo; pv -L 50k > /tmp/f`) keeps every viewer connected (a home-VM peer tab, a browser share viewer and a local tab), even when draining takes several times the 15 s heartbeat timeout. The file matches the payload byte for byte, and a viewer whose network really dies is still dropped once its backlog drains. Automated with a blocking fake hub, then re-run as M-101 on the two Tart VMs. (INPT-01)
  2. A large paste that fully arrives in an echoing shell shows no "Paste may have been cut short" notice, even when the server's outgoing queue is full as the ping arrives. A paste cut short by a real drop still shows it. (INPT-02)
  3. A browser share viewer whose socket drops mid-paste shows the same "Paste may have been cut short" notice as the app, in its status bar, with no `innerHTML` and no CSP change. Checked in a jsdom harness and with `tailscale down` on the office VM during a paste. (INPT-03)
  4. After a daemon error and Retry, no snapshot of a removed peer tab is left behind. Two Macs that hand out the same session id open as two separate tabs, each rejoining with its own delta. (INPT-04)
  5. `TestExitEvent_ListSessions_ExitCodePopulatedForStopped` passes in repeated full-package `go test -race -count=20 ./internal/daemon/` runs, and the full `-race` suite passes with nothing skipped. (INPT-05)
  6. The viewer you're typing on sets the session's size. On the home VM, Cmd+ in a peer tab on an office session makes the shell reflow to the home grid (`tput cols` changes, the text gets bigger), and so does resizing the home window. Typing in the office local tab hands the size back to the office Mac. The same works for a browser viewer and `portty attach`. Reported on v1.1.0. (INPT-06)
**Plans**: 7/7 plans complete
Plans:
- [x] 10-01-PLAN.md — Wave 1: hermetic exit-code ListSessions test, reproduced and proven with full-package `-race -count=20` (INPT-05)
- [x] 10-02-PLAN.md — Wave 1: peer tabs keyed by host plus session everywhere, Retry drops removed peer tabs' snapshots, host-aware sidebar highlight (INPT-04)
- [x] 10-03-PLAN.md — Wave 1: hub-owned input FIFO with per-connection quotas for both read pumps, owed pongs via a shared write pump, proxy backpressure test, handoff input wait, pong-ack comments, browser-viewer notice vitest (INPT-01, INPT-02, INPT-03)
- [x] 10-04-PLAN.md — Wave 2: grid owner replaces min-among-local, claim bit on MsgResize2, clamp and read-only gate, web claims, settle window removed with a rejoin test, no fallback grid push on local join (INPT-06)
- [x] 10-05-PLAN.md — Wave 2: viewers follow the server grid and report their own fit, claims on window resize and zoom, per-tab Cmd+ for peer tabs (INPT-06)
- [x] 10-06-PLAN.md — Wave 3: typed-input classifier and typing claims in both pumps, `portty attach` SIGWINCH claim, end-to-end attach test through the proxy (INPT-06)
- [x] 10-07-PLAN.md — Wave 4: full gate, TESTING.md, and Tart VM rows for SC1-SC3 and SC6 plus the Phase 9 V-02 re-run (all INPT)
**UI hint**: no

### Phase 11: File Preview
**Goal**: A user in any Portty session, local or remote, can run `portty open <file>` and see an image, PDF or Markdown file in a preview tab beside the terminal, in the app and in a browser share viewer. The bytes never leave the session's authenticated stream.
**Depends on**: Phase 9 (the API version lets an older daemon or viewer meet the new preview message with "update needed" instead of a failure)
**Requirements**: PREV-01, PREV-02, PREV-03, PREV-04, PREV-05
**Success Criteria** (what must be TRUE):
  1. Running `portty open <file>` in an office-VM session opens a preview tab beside that session's terminal tab in every app viewer of it: the office VM's local tab and the home VM's peer tab over the real tailnet. Closing the preview leaves the terminal and the session as they were. A missing or unsupported file gets a one-line CLI error. (PREV-01)
  2. PNG, JPEG, GIF, WebP and SVG images, PDFs and Markdown render in the preview tab with the macOS-native look, following system light and dark mode live. PDF rendering is proven inside the Wails v2 web view before anything is built on it. Hostile fixtures (SVG and Markdown carrying script, event handlers, `javascript:` links or remote images) run nothing and fetch nothing. (PREV-02)
  3. A file over the size cap shows a clear "too large to preview" message, never a partial render. While a file near the cap streams, the terminal keeps updating and no viewer is dropped. Neither the daemon nor the share server gains any route that reads a file by path; tests and the security review confirm it. (PREV-03)
  4. A browser share viewer on the home VM gets the same previews through the share link. The share page CSP (today `img-src 'self' data:`) changes only as far as the phase's security review approves, the review is recorded, and the token gate, the tailnet-only listener and `frame-ancestors 'none'` are unchanged. (PREV-04)
  5. On a VM with nothing on the PATH by hand, `portty open` works in a new Portty session (zsh login shell) with no setup. In Terminal.app, `portty` is not found until Portty menu → "Install Command-Line Tool…" is used; after the macOS administrator prompt, `/usr/local/bin/portty` points into /Applications/Portty.app and `portty list` works there. Cancelling the prompt changes nothing, and an existing link to something else is reported, not overwritten. (PREV-05)
**Plans**: 10/10 plans complete
Plans:
- [x] 11-01-PLAN.md — PDF and SVG spike first: a blob: PDF in the real Wails v2 web view (iframe/object/embed) and on the share page in Safari under the real CSP with candidate directives; verdict, PDF element and CSP delta in 11-SPIKE.md (PREV-02, PREV-04)
- [x] 11-02-PLAN.md — Relay wire format MsgPreview (0x21) and Hub.SendPreview: paced per-viewer sender that never kicks, owner-only focus bit, near-cap test with a ticking terminal, golden fixture for the frontend (PREV-01, PREV-03)
- [x] 11-03-PLAN.md — Session identity and PATH: PORTTY_SESSION_ID set before spawn, per-user `portty` link refreshed atomically and prepended to each new session's PATH, path_helper survival test (PREV-01, PREV-05)
- [x] 11-04-PLAN.md — Daemon route POST /sessions/{id}/preview on the unix socket only: validation, 202 then async streaming (handoff drain not blocked), route absence and source gate, API 3, DaemonClient.SendPreview (PREV-01, PREV-03)
- [x] 11-05-PLAN.md — Frontend data path: MSG_PREVIEW parsing against the Go fixture, bounded assembler, PreviewStore (refresh in place, 10 tabs, 100 MiB, URL revoke), onPreview through TerminalPanel and WebShareSessionView (PREV-01, PREV-03)
- [x] 11-06-PLAN.md — Image and PDF renderers and the share CSP: ImagePreview (fit, Actual Size, checkerboard; hostile SVG fixtures), PdfPreview per the spike, base preview styles, minimal reviewed CSP delta with tests, 11-SECURITY-REVIEW.md (PREV-02, PREV-04)
- [x] 11-07-PLAN.md — Markdown renderer: marked 18.1.3 and dompurify 3.4.17 exact-pinned and lazy-loaded, strict sanitizer, hostile Markdown fixture, gated links, Markdown styles, the review's Dependencies section (PREV-02)
- [x] 11-08-PLAN.md — `portty open` CLI (type detection, cap, UI-SPEC one-line errors, never restarts the daemon, real-launch check) and Portty ▸ Install Command-Line Tool… (lstat first, osascript argv, cancel, foreign link, DMG), README step 2 (PREV-01, PREV-03, PREV-05)
- [x] 11-09-PLAN.md — Preview tabs and pane: tab beside its session, focus only for the typist, keep-mounted peer and browser views, close/refresh/limit rules, browser tab bar, PreviewPanel states and accessibility (PREV-01, PREV-02, PREV-04)
- [x] 11-10-PLAN.md — Gate once, TESTING.md (Sections 2, 4, Category BA), Tart VM checks for SC1-SC5 (beacons, near-cap with four viewers, Safari CSP, admin prompt), upgrade sanity A→B and B→B2 mid-preview, old viewer, security review verdict (all PREV)
**UI hint**: yes

### Phase 12: File Path Click
**Goal**: A user can Cmd-click a file path that a program printed in the terminal and see it previewed, in local and remote sessions alike, because Portty tracks each shell's working directory.
**Depends on**: Phase 11 (a click opens the file through the preview path)
**Requirements**: PATH-01, PATH-02
**Success Criteria** (what must be TRUE):
  1. A new Portty zsh session reports its working directory through OSC 7 after every `cd`, with no setup by the user. A shell the user has set up to emit OSC 7 (bash, fish) is tracked the same way. Without OSC 7, absolute paths still work and relative ones do nothing. (PATH-01)
  2. Cmd-clicking an absolute path, or one relative to the working directory (for example `docs/diagram.png` printed by `ls` after `cd ~/proj`), opens it in a preview tab. This works in a local tab, and in a home-VM peer tab on an office-VM session over the real tailnet. (PATH-02)
  3. Cmd-clicking a path that doesn't exist does nothing: no tab, no dialog, and nothing typed into the shell. A plain click still selects text as before, and URLs keep their own click behavior. (PATH-02)
  4. Path clicks keep the PREV-03 rule. The request rides that session's authenticated stream, the host resolves it against that session's own working directory, and the host previews only a path that the session itself printed, so there is still no way to read a file by arbitrary path. The security review records this. (PATH-02; keeps PREV-03)
**Plans**: 9/9 plans complete
Plans:
**Wave 1**
- [x] 12-01-PLAN.md — Move `portty open`'s file checks into the leaf package internal/previewfile (CLI lines unchanged) and CleanPreviewName into relay (PATH-02)
- [x] 12-02-PLAN.md — zsh ZDOTDIR shim: one embedded .zshenv (restore ZDOTDIR, source the user's .zshenv, chpwd + precmd + initial OSC 7 to /dev/tty), written atomically and verified before ZDOTDIR is set, real-zsh startup matrix (PATH-01)
- [x] 12-03-PLAN.md — Hub cwd tracking: OSC 7 parsed in the live emulator, bounded row-keyed cwd log interleaved with History, carried across upgrade handoffs (PATH-01)

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 12-04-PLAN.md — Hub.PathClick on the session stream: MsgPathClick 0x13, read-only refusal, printed-token check over History + screen, cwd-at-print resolution, previewfile checks, SendPreview with FocusSub, unicast notices, capability bit, source gate, shared fixture (PATH-02)

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 12-05-PLAN.md — Share server arm (peer and browser viewers; read-only capabilities refused), API 4 with PathClickAPI, 12-SECURITY-REVIEW.md recording the PREV-03 amendment (PATH-02)
- [x] 12-06-PLAN.md — Viewer: path link provider in lib/pathLinks.ts (fixture-pinned detection, wrapped rows, modifier-held underline), sendPathClick gated by the host's bit, the two fixed notices, the Phase 95 guard test edited deliberately (PATH-02)

**Wave 4** *(blocked on Wave 3 completion)*
- [x] 12-07-PLAN.md — Gate once, TESTING.md (Sections 2, 4, Category BB M-128..M-135), Tart VM checks for SC1-SC4 (local, peer, Safari, read-only, notices, shell integrity, TUI, wrap, upgrades), security review verdict (PATH-01, PATH-02)

**Gap closure** *(from 12-VERIFICATION.md, 2026-11-22)*
- [x] 12-08-PLAN.md — Viewer gaps: reproduce the reconnect and resize link loss against a real xterm 6 Linkifier (RED), refresh xterm's link state whenever the capability is reported and after every resize (GREEN, paths and URLs), after the selection-offset fix lands; desktop peer tabs take this Mac's own link settings with no cross-origin config request; TESTING.md Sections 2 and 4 (PATH-02)
- [x] 12-09-PLAN.md — Re-run of 12-07 on the gap-closed build: gate once, VM rows C-00R..C-07R, C-09R, U-01R, user checkpoint (FAILs, C-08 ssh and C-06 desktop-app items), TESTING.md Section 5 M-128..M-135, security review gap-closure section and verdict (PATH-01, PATH-02) *(blocked on 12-08)*
**UI hint**: yes

### Phase 13: Remote Localhost URLs
**Goal**: A user viewing a remote session can click a `localhost` URL printed on the remote Mac and see that page in their normal browser, proxied over the tailnet, with no way to reach a port they didn't approve.
**Depends on**: Phase 9 (the API version gates the new proxy route)
**Requirements**: URL-01, URL-02, URL-03
**Success Criteria** (what must be TRUE):
  1. In a home-VM peer tab on an office-VM session running a dev server (for example `python3 -m http.server 8000`), clicking `http://localhost:8000/` or `http://127.0.0.1:8000/` opens the office VM's page in the home VM's default browser through Portty's proxy over the real tailnet. Paths, query strings and the page's own assets load. There is no in-app browser. (URL-01)
  2. The first click on a port asks for consent in a macOS-style sheet that names the remote Mac and the port; declining opens nothing. Approval covers only that Mac and that port, and the viewing Mac's listener binds only to its loopback (checked with `lsof`). (URL-02)
  3. The proxy refuses whatever wasn't asked for. Requests without the token, from outside the tailnet, or for a port nobody approved are refused before the remote Mac connects anywhere. It only ever connects to the remote Mac's own loopback, it cannot be used to scan ports, and once the host stops sharing the proxied page stops loading. The security review records the threat model. (URL-02)
  4. Local URLs, and non-localhost URLs in remote tabs, work as today: the confirm popover, then the default browser. The existing link tests still pass. (URL-03)
**Plans**: 7/7 plans complete
Plans:
- [x] 13-01-PLAN.md — Wave 1: host gate primitives: Hub.PortPrinted (printed loopback URL check over the Phase 12 snapshot, URL boundaries) and port tokens under an HKDF-derived key (URL-02)
- [x] 13-02-PLAN.md — Wave 1: the viewing daemon's loopback forward: POST /peers/ports, grant call, per-(Mac, port) consent, probe then bind on 127.0.0.1 and [::1], Host and cross-site checks, hop-1 proxy, fixed pages, lifetimes; shared classifier fixture (URL-01, URL-02)
- [x] 13-03-PLAN.md — Wave 2: share server grant and proxy routes in check order (same user, write cap, port token, deny list, printed check), loopback-only dialer, header and Origin rules, refusal tables with a tripwire, cross-daemon E2E (URL-01, URL-02)
- [x] 13-04-PLAN.md — Wave 2: API 5 with PortProxyAPI, DaemonClient.OpenRemotePort and the App.OpenRemotePort binding (never throws; daemon_outdated) (URL-01, URL-02)
- [x] 13-05-PLAN.md — Wave 2: TerminalPanel peer-tab branch, consent sheet, five notices, classifier with fixture parity, URL-03 regressions (URL-01, URL-02, URL-03)
- [x] 13-06-PLAN.md — Wave 3: WebSocket and SSE through both hops; streams end on stop-share, session exit, key regeneration and Stop; 13-SECURITY-REVIEW.md parts 1 (URL-01, URL-02)
- [x] 13-07-PLAN.md — Wave 4: full gate, TESTING.md, two-Tart-VM checks L-00..L-08 over the real tailnet, blocking-human checkpoint, Section 5 and the review's verdict (URL-01, URL-02, URL-03)
**UI hint**: no

### Phase 14: Mobile Web App
**Goal**: The user can drive a shared session from an iPhone. The share view fits the phone, adds the keys iOS lacks, installs to the Home Screen and picks up again after the phone sleeps or changes network. The host Mac serves all of it over the tailnet, and portty.app/app is only a static launcher page.
**Depends on**: Phases 10 and 11 (it builds on the browser share viewer and its CSP as the paste-notice and preview work leave them)
**Requirements**: MOB-05, MOB-01, MOB-02, MOB-03, MOB-04
**Success Criteria** (what must be TRUE):
  1. On an iPhone (Playwright's WebKit iPhone emulation first, then the real phone), the share view fits the session to the phone's width, keeps the cursor line above the on-screen keyboard, and shows a key bar above the keyboard (Esc, Tab, Ctrl, arrows, `|`) whose keys reach the shell; Ctrl then C stops a running command. Pinching changes the font size. The desktop browser view is unchanged. (MOB-01)
  2. Add to Home Screen gives a Portty icon that opens the session full screen, with every asset served by the host Mac over the tailnet (nothing from portty.app or a CDN). The token still gates it: after the host stops sharing, the icon opens the "Link No Longer Works" page. The share page CSP changes only as far as the security review approves (for example `manifest-src 'self'`). (MOB-02)
  3. **Human check (real iPhone):** with the Tailscale app connected, after the phone sleeps for several minutes or switches between Wi-Fi and cellular, the view reconnects on its own and resumes with no missing or duplicated output. Recorded as a new M-NN item in TESTING.md. The reconnect logic is unit-tested, but VMs cannot stand in for iOS suspension and the Tailscale app. (MOB-03)
  4. This repo supplies the copy and the link format for portty.app/app (what Portty is, how to open your Mac's tailnet share link, how to add it to the Home Screen) for the website repo. That page holds no token, opens no WebSocket and has no shell access, and nothing in this repo's origin check, CSP or token handling trusts portty.app. (MOB-04)
  5. Built first: a browser share link opens only for devices on the host's own Tailscale account. On the home VM, which is signed in to the same account, the link opens with no prompt. A request whose WhoIs is a different user (a stub in tests, plus a VM check with a second login or a faked WhoIs) is refused with a "This link only works on your own devices" page before any terminal data is sent. Page, info and WebSocket routes are all gated, and the check fails closed when WhoIs errors. (MOB-05)
**Plans**: 14/14 plans complete
Plans:
- [x] 14-01-PLAN.md — Wave 1: MOB-05 first: the shared WhoIs fake (webservertest) in every test-built share server and the Playwright fixture; requireSameUser on the discovery probe, info, plugin-config and the WebSocket (origin, WhoIs, cap); fixture identity switch and input drain; Playwright WebKit install (MOB-05)
- [x] 14-02-PLAN.md — Wave 1: lone Esc: the hold seen on the Tart VMs first (E-00, E-01), then the end-of-frame flush in InputAbsorber with P9 changed on purpose (MOB-01)
- [x] 14-03-PLAN.md — Wave 2: page routes gated (/app/, /sessions/{id}) with the static refusal and tailscaled-retry pages; no-cache for /app/ files outside assets/ (MOB-05, MOB-02)
- [x] 14-04-PLAN.md — Wave 2: the SPA's refusal screen at bootstrap and in the probe, pageshow wake, connect-timeout stuck signal (MOB-05, MOB-03)
- [x] 14-05-PLAN.md — Wave 2: Home Screen: static manifest without start_url, full-bleed icon, head tags, CSP Amendment 4 manifest-src 'self' (MOB-02)
- [x] 14-06-PLAN.md — Wave 2: release guard and check-brand lines; the VM-only vmwhois build; fixture routes for the iOS Simulator (grid, info hits, share off/on, end, PDF preview, CA) (MOB-05, MOB-02)
- [x] 14-07-PLAN.md — Wave 3: early headless iOS Simulator Home Screen spike (H-01: does the launch keep session and cap?) with a blocking continue-or-redesign checkpoint (MOB-02)
- [x] 14-08-PLAN.md — Wave 3: Reload on phone stop, stuck and error states, the phone query, and the standalone Open PDF view with Done (MOB-03)
- [x] 14-09-PLAN.md — Wave 4: share copy made true in seven places; docs/launcher-app.md for portty.app/app with its pin test (MOB-05, MOB-04)
- [x] 14-10-PLAN.md — Wave 4: the key bar (Esc, Tab, Ctrl, arrows, |) through term.input with one-shot Ctrl; iPhone emulation project (MOB-01)
- [x] 14-11-PLAN.md — Wave 5: pinch to font with one claim, visualViewport keyboard inset and safe areas, phone chrome (MOB-01)
- [x] 14-12-PLAN.md — Wave 6: full gate, TESTING.md Sections 2 and 4, Tart VM rows P-00..P-07 over the real tailnet (same account and probe, other account via vmwhois, fault page, WhoIs cost, lone Esc, desktop, launcher) (all)
- [x] 14-13-PLAN.md — Wave 6: headless iOS Simulator rows S-00..S-08 against the loopback fixture (key bar, keyboard inset, pinch, Home Screen launch URL, safe areas, Open PDF, Reload) (MOB-01, MOB-02, MOB-03)
- [x] 14-14-PLAN.md — Wave 7: security review, blocking-human checkpoint (real-phone M-145 look and MOB-03 reconnect; redesign route if the query is dropped), Section 5 and the verdict (all)
**UI hint**: yes

## Progress

**Execution Order:** Phases run in numeric order: 9 → 10 → 11 → 12 → 13 → 14.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 9. Upgrades Keep Sessions | v1.2 | 7/7 | Complete    | 2026-11-20 |
| 10. Input Path Reliability | v1.2 | 7/7 | Complete    | 2026-11-21 |
| 11. File Preview | v1.2 | 10/10 | Complete    | 2026-11-21 |
| 12. File Path Click | v1.2 | 9/9 | Complete    | 2026-11-22 |
| 13. Remote Localhost URLs | v1.2 | 7/7 | Complete    | 2026-11-22 |
| 14. Mobile Web App | v1.2 | 14/14 | Complete    | 2026-11-22 |

### Phase 15: v1.2 Hardening

**Goal:** Close the bugs and test flakes found during v1.2 so the milestone ships clean. Source: the pending todos in .planning/todos/pending/. dompurify-3418 is blocked until 2026-11-28, and v12-release-checks needs the user on a real Mac; both are out of scope here.
**Requirements**: none new (bug fixes and hardening against existing behaviour)
**Success Criteria** (what must be TRUE):
  1. `portty attach` with its status bar shown tells the remote PTY rows-1, initially and on every resize, so a full-screen app (e.g. Claude Code) never draws over the bar. If several viewers are attached, the reduced height wins once the CLI viewer claims or types (INPT-06; a CLI that only watches a taller GUI-owned grid keeps today's behaviour, accepted by the user 2026-11-23). A test pins it. (todo 2026-11-21-attach-statusbar-overwritten)
  2. Clicking an already-ended peer tab shows the "session ended" state, not "stopped sharing". (todo 2026-11-21-ended-peer-tab-banner)
  3. Dismissing a naturally exited session never signals a stale process group, and the other items in todo 2026-11-20-killsession-stale-pgid are resolved.
  4. The Phase 9 audit's daemon socket hardening items are done or recorded as accepted, each with a reason. (todo 2026-11-20-daemon-socket-hardening)
  5. Clicking the "Portty X is available" notification brings Portty forward. (todo 2026-11-18-update-notification-click)
  6. A relative path printed on row 0 of a never-scrolled session resolves against the directory it was printed in (F-1). The first Cmd-click after the pointer returns to the same cell whose text changed sends the path click (F-2).
  7. Flaky tests are made deterministic without loosening product behaviour: pathLinks.linkifier "after a reconnect…" and TestShare_PasteBacklogKeepsViewer. The full Go and vitest suites pass once, clean.
  8. Each fixed todo moves to .planning/todos/completed/, and TESTING.md is updated per the standing rule.
**Depends on:** Phase 14
**Plans:** 5/9 plans executed

Plans:
**Wave 1**
- [x] 15-01-PLAN.md — `portty attach` tells the PTY rows-1 while its bar is shown (initial frame and every SIGWINCH claim, bottom and --status-top), pinned at the frame and the hub grid owner (SC1)
- [x] 15-02-PLAN.md — Dismiss never signals a reaped session's process group (todo items 2-3 documented); TestKillClean_NoOrphans and TestShare_PasteBacklogKeepsViewer wait on events (SC3, SC7)
- [x] 15-03-PLAN.md — Daemon socket mode (O-2): 0600 at listen (fail closed) and a best-effort tighten after a takeover COMMIT (SC4)
- [x] 15-04-PLAN.md — Path click F-1 (rows stay monotonic across ED 3) and F-2 (mouseleave invalidates the Linkifier), linkifier test on vi.waitFor (SC6, SC7)
- [x] 15-05-PLAN.md — An ended peer tab keeps "Session ended" when clicked again (ended flag feeds the reconnect verdict) (SC2)
- [ ] 15-07-PLAN.md — A refused share port reads as reachable with nothing shared; a timeout stays "Can't reach this Mac"; no grace (SC2)

**Wave 2** *(15-06 blocked on 15-05: App.tsx; 15-08 blocked on 15-03: handoff.go)*
- [ ] 15-06-PLAN.md — Clicking the update notification shows Portty with the available banner and starts no download (native delegate, Go handler, React un-dismiss; live click is M-152) (SC5)
- [ ] 15-08-PLAN.md — Takeover refuses a HELLO-OK pid the kernel does not confirm (O-6) and fdpass.Recv frees fds from unparsed control messages (O-1); daemon and fdpass packages pass once (SC4)

**Wave 3** *(blocked on Waves 1-2)*
- [ ] 15-09-PLAN.md — TESTING.md (Phase 15 note, Category BF M-152..M-155, M-155 = TestHandoffIT plus a pre-phase upgrade chain in portty-test before ship), one clean full Go -race and vitest run, nine todos moved to completed (SC7, SC8)
