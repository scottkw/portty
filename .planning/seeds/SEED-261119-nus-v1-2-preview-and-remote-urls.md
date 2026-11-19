---
id: SEED-261119-nus
status: dormant
planted: 2026-11-19
planted_during: v1.1 Hardening, Phase 8
trigger_when: when v1.1 is complete; start milestone v1.2 with this scope (user approved 2026-11-19)
scope: medium (about 4-6 days of agent work)
---

# SEED-261119-nus: v1.2 — file preview tabs and remote localhost URLs

## Why This Matters
The user runs a terminal on the office Mac (often with Claude Code inside) and controls it from home. Two gaps:
- Images, PDFs and markdown produced in a session can't be viewed without copying them off the remote Mac.
- `localhost:PORT` URLs printed by a dev server on the remote Mac can't be opened from home.

The user approved this as the v1.2 milestone on 2026-11-19 and chose option 2 for URLs: no embedded browser.

## Approved Scope (v1.2 requirements to write at /gsd-new-milestone)
0. **FIRST PHASE: keep sessions alive across a Portty upgrade.** User decision, 2026-11-19 ("Guard now, keep later").
   - Today a version-mismatch restart ends every live session (handleShutdown, since 3d7fd1a2).
   - v1.1 adds only a consent guard (DEBT-08).
   - v1.2: the old daemon hands its PTY master file descriptors and session state (ring, history, offsets) to the new daemon over the unix socket (SCM_RIGHTS). Viewers reconnect with resume.
   - Needs a VM upgrade test with live sessions and attached viewers.
   - Also in this phase:
     - the GUI ignores SIGTERM (found in U-01);
     - pending todos: paste-heartbeat-drop (IN-05), paste-notice-false-positive (IN-06), peer-snapshot-tab-replace (IN-07), web-viewer-paste-notice. These share the ack/input-queue path.
1. **File preview via `portty open <file>`**, run inside any session, local or remote.
   - The CLI talks to the daemon on the Mac where the shell runs. That daemon sends a preview message over the session's existing authenticated relay stream, chunked with a size cap. Viewers open a preview tab next to the terminal tabs.
   - Renderers:
     - images;
     - PDF, using WKWebView's native rendering (verify inside Wails v2 first);
     - markdown, prettified with a renderer plus a sanitizer, because remote content is untrusted.
   - No generic remote file-read API: only files someone explicitly opened from inside the shell are sent.
   - CSP update and a security review. The share page CSP today is `img-src 'self' data:`.
2. **Click a file path printed in the terminal to preview it.** Needs the shell's working directory, via OSC 7 (zsh hook) tracking.
3. **Remote localhost URLs.** A `localhost:PORT` / `127.0.0.1:PORT` URL clicked in a remote session is proxied over the tailnet by Portty and opened in the user's normal browser.
   - Local URLs keep the existing confirm popover, then open in the default browser.
   - Security review: this exposes the remote Mac's local ports. Keep it token-gated and tailnet-only, ask consent per port, and allow no arbitrary port scanning.

4. **Mobile-aware web app** (user approved for v1.2, 2026-11-19; estimate about 2-3 days):
   - a key bar above the iOS keyboard: Esc, Tab, Ctrl, arrows, `|`;
   - Add to Home Screen: manifest and icon, full screen;
   - a phone layout: fit to width, keyboard inset, pinch to change the font size;
   - reconnect after the phone sleeps, checked on a real iPhone running the Tailscale app.
   - The existing browser view already has touch scroll (`attachTouchScroll`).
   - **User request:** host it as a subfolder of the Portty website, https://portty.app/app. The website lives in a separate repo, handled by another session.
   - **DECIDED (user, 2026-11-19): option (a).**
     - Each host Mac serves the mobile UI itself over the tailnet, with no change to the origin check, CSP or tokens.
     - portty.app/app is a static landing/launcher page only: what the app is, how to open your Mac's tailnet link, how to add it to the home screen. It holds no WebSocket, no token and no shell access, so it can stay static on Cloudflare.
     - The website repo and session own that page; this repo only supplies the copy and the link format.
   - Background on the decision: today each host Mac serves its own browser view, and `internal/webserver/origin_mw.go` accepts WebSockets only from the host's own origin (the defence against cross-site WebSocket hijacking). For a page on portty.app to talk to a host, every host would have to trust the portty.app origin. Anyone who could change the JavaScript on portty.app (a compromised site, CDN or deploy) would then get the share token and full shell access on every user's Macs. That conflicts with the constraints "no cloud dependencies" and "keep token-gated tailnet-only links and CSP intact". Options:
     - (a) host Macs serve the mobile UI over the tailnet, and portty.app/app is only a landing page or launcher with no shell access;
     - (b) portty.app/app is the real app, with the origin allowlist widened, subresource integrity, a strict CSP and signed releases.

## Not in scope
- An in-app web view tab for arbitrary sites. Wails v2 has a single web view; iframes are blocked by X-Frame-Options and the CSP; a native WKWebView would be extra Go/Objective-C to maintain.

## When to Surface
**Trigger:** run /gsd-new-milestone v1.2 right after v1.1 completes (after `main` is fast-forwarded and tagged v1.1).

## Scope Estimate
Preview about 2-3 days, path click about 0.5 day, URL proxy about 1-2 days. Likely 3 phases: preview, path click (could merge into preview), URL proxy.

## Breadcrumbs
- Link handling: frontend/src/components (LinkConfirmPopover, `ALLOWED_SCHEMES` in frontend/src/lib).
- CSP: internal/webserver/csp_mw.go.
- Relay stream and message types: internal/relay/protocol.go, frontend/src/lib/relayClient.ts.
- CLI: cmd_cli.go and cmd_attach.go; daemon API in internal/daemon/api.go.
- Peer proxying: internal/daemon/remote_ws_proxy.go, peer_client.go; the tailnet guard is in internal/tailnet/live.go (keep tests off the tailnet).
- Wails v2.10.2 (single window). No markdown or PDF dependencies installed today.

## Notes
- User constraints: radical simplicity, macOS-native look, rock-solid first.
- The estimates come from reading the code; no prototype has been built.
