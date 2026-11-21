# Requirements: Portty

**Defined:** 2026-11-19
**Core Value:** A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.
**Source:** .planning/seeds/SEED-261119-nus-v1-2-preview-and-remote-urls.md (scope approved by the user, 2026-11-19)

## v1.2 Requirements

### Upgrades keep sessions (first, rock-solid priority)

- [x] **UPGR-01**: Upgrading Portty while sessions are running keeps every session alive. The old daemon hands its PTYs and session state (ring, history, offsets, share records) to the new daemon. Shells, and the programs running in them (e.g. Claude Code), never notice. No consent dialog is needed for a successful handoff.
- [x] **UPGR-02**: Viewers attached during an upgrade (local tabs, peer tabs, browser share links, `portty attach`) reconnect to the new daemon on their own and resume with a delta, with no duplicated or missing output.
- [x] **UPGR-03**: If a handoff fails partway, no session is lost: the old daemon keeps serving, the user is told, and the DEBT-08 consent guard remains the fallback.
- [x] **UPGR-04**: The daemon reports an API/protocol version. A GUI or CLI facing an incompatible daemon shows a clear "update needed" state instead of failing calls (08-06 IN-03).
- [x] **UPGR-05**: The GUI quits cleanly on SIGTERM. Quit All can cancel an in-flight daemon restart instead of waiting behind it (08-06 IN-07). Cmd-Q while the restart dialog is open never stacks a second dialog underneath.

### Input path reliability (scheduled v1.1 todos)

- [x] **INPT-01**: A long paste into a slow program never gets a healthy viewer dropped. Heartbeats on the server, the daemon proxy and the browser keep working while input is backpressured, via a bounded input queue (08 IN-05).
- [x] **INPT-02**: The "paste may have been cut short" notice never appears for a paste that fully arrived (08 IN-06).
- [x] **INPT-03**: The browser share viewer shows the same partial-paste notice as the app when a socket drops mid-paste.
- [x] **INPT-04**: Peer-tab snapshots are keyed by host plus session everywhere, including the Retry/daemon-reconnect tab replacement (08 IN-07).
- [x] **INPT-06**: The viewer you're typing on sets the session's size (user decision 2026-11-19, reported in v1.1.0). Zooming (Cmd+/Cmd−) or resizing in a peer tab, the browser viewer or `portty attach` makes the shell reflow to that viewer's grid. When the host, or another viewer, types again, it takes over the size. Today viewers never drive the grid and the peer tab's zoom handler is a no-op (WebShareSessionView.tsx:197).
- [x] **INPT-05**: `TestExitEvent_ListSessions_ExitCodePopulatedForStopped` is deterministic under the full `-race` suite.

### File preview

- [x] **PREV-01**: Running `portty open <file>` inside any Portty session, local or remote, opens a preview tab for that file in every app viewer of that session.
- [x] **PREV-02**: Images (PNG, JPEG, GIF, WebP, SVG rendered safely), PDFs and Markdown (prettified, sanitized, no script and no remote fetches) render in the preview tab with a macOS-native look in light and dark mode.
- [x] **PREV-03**: File bytes travel only over the session's existing authenticated stream, chunked under a size cap with a clear "too large" message. There is no generic file-read API on the daemon or the share server.
- [x] **PREV-04**: Browser share viewers get the same previews, with the share page CSP changed only as much as the security review allows.
- [x] **PREV-05**: `portty` is on the PATH without manual setup (user decision 2026-11-20). Inside every Portty session it works automatically (the daemon puts the app bundle's binary directory on the session's PATH, and it survives the login shell's path_helper). For other terminals, a Portty menu item "Install Command-Line Tool…" shows the standard macOS administrator prompt and creates `/usr/local/bin/portty` (creating the folder if needed); it says so if the link already exists and points elsewhere instead of overwriting it silently. The website's install step 2 then points at the menu item.

### File path click

- [ ] **PATH-01**: The terminal knows each session's working directory through OSC 7. Portty's zsh setup emits it, and other shells that emit it work too.
- [ ] **PATH-02**: Cmd-clicking a file path printed in the terminal (absolute, or relative to the working directory) previews it, using the PREV path for local and remote sessions alike. The host previews only paths that session itself printed (user decision 2026-11-19), so there is still no general file-read API. Paths that don't exist do nothing.

### Remote localhost URLs

- [ ] **URL-01**: Clicking `http://localhost:PORT` or `http://127.0.0.1:PORT` in a remote session opens that page from the remote Mac in the user's normal browser, proxied over the tailnet by Portty, including WebSocket upgrades so dev-server hot reload works (user decision 2026-11-19).
- [ ] **URL-02**: The proxy is token-gated and tailnet-only, asks consent per port, binds only on the viewing Mac's loopback, and never allows arbitrary port scanning.
- [ ] **URL-03**: Local URLs keep today's behaviour: confirm popover, then the default browser.

### Mobile web app

- [ ] **MOB-01**: The browser share view works on an iPhone: a key bar above the keyboard (Esc, Tab, Ctrl, arrows, `|`), the session fitted to the phone's width, the keyboard inset handled, and pinch to change the font size.
- [ ] **MOB-02**: The share view can be added to the Home Screen (manifest and icon) and opens full screen, served by the host Mac over the tailnet.
- [ ] **MOB-03**: After the phone sleeps or switches networks, the view reconnects and resumes, checked on a real iPhone running the Tailscale app.
- [ ] **MOB-04**: This repo supplies the copy and link format for the static launcher page at portty.app/app. That page holds no token, no WebSocket and no shell access, and it is built by the website repo.
- [ ] **MOB-05**: Browser share links work only for devices signed in to the host's Tailscale account (user decision 2026-11-19). Every share-server request (page, info, WebSocket) gets the same WhoIs same-user check that peer tabs already use, and it fails closed. Your own devices, including an iPhone running the Tailscale app, see no prompt. A valid link opened by another tailnet user or a shared node is refused with a clear page. A pairing code for invited users is deferred (an opt-in later).

## Future Requirements

- Clicking the "update available" notification shows Portty (needs a notarized build).
- Overnight two-Mac test (VERF-01, M-84), M-85, and the first real release checks (M-93).
- Native iOS app (only if the mobile web app proves too awkward).
- Opt-in invited devices for a browser share (user decision 2026-11-19: future milestone, not v1.2). Per session, "Allow invited devices" shows a one-time code like `K7QM-2XPA` on the host. A browser outside the host's Tailscale account must enter it: 5 tries, then lockout; one use, short expiry. The paired device is remembered until sharing is turned off. Builds on MOB-05.

## Out of Scope

| Feature | Reason |
|---------|--------|
| In-app web view tab for arbitrary sites | Wails v2 has a single web view; iframes are blocked; a native WKWebView would be extra code to maintain. The user chose option 2 (proxy plus normal browser). |
| Hosting the real app on portty.app | Every Mac would have to trust the website's origin, so a website compromise would mean shell access. The user chose host-served. |
| Generic remote file browsing | Only files explicitly opened from inside the shell are sent. |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| UPGR-01 | Phase 9 | Complete |
| UPGR-02 | Phase 9 | Complete |
| UPGR-03 | Phase 9 | Complete |
| UPGR-04 | Phase 9 | Complete |
| UPGR-05 | Phase 9 | Complete |
| INPT-01 | Phase 10 | Complete |
| INPT-02 | Phase 10 | Complete |
| INPT-03 | Phase 10 | Complete |
| INPT-04 | Phase 10 | Complete |
| INPT-05 | Phase 10 | Complete |
| INPT-06 | Phase 10 | Complete |
| PREV-01 | Phase 11 | Complete |
| PREV-02 | Phase 11 | Complete |
| PREV-03 | Phase 11 | Complete |
| PREV-04 | Phase 11 | Complete |
| PREV-05 | Phase 11 | Complete |
| PATH-01 | Phase 12 | Pending |
| PATH-02 | Phase 12 | Pending |
| URL-01 | Phase 13 | Pending |
| URL-02 | Phase 13 | Pending |
| URL-03 | Phase 13 | Pending |
| MOB-01 | Phase 14 | Pending |
| MOB-02 | Phase 14 | Pending |
| MOB-03 | Phase 14 | Pending |
| MOB-04 | Phase 14 | Pending |
| MOB-05 | Phase 14 | Pending |

**Coverage:**
- v1.2 requirements: 26 total
- Mapped to phases: 26
- Unmapped: 0 ✓

---
*Requirements defined: 2026-11-19*
