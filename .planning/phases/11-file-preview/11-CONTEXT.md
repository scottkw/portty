# Phase 11: File Preview — Context

**Gathered:** 2026-11-21 (autonomous smart discuss; reversible technical choices decided and logged; product decisions from the user noted)
**Status:** Ready for UI spec and planning

<domain>
`portty open <file>`, run inside any Portty session (local or remote), shows an image, PDF or Markdown file in a preview tab beside that session's terminal tab, in every app viewer and in browser share viewers. File bytes travel only over the session's existing authenticated stream; there is no route that reads a file by path (PREV-01..04). `portty` is on the PATH inside Portty sessions with no setup, and a Portty menu item installs it for other terminals (PREV-05). Source: .planning/seeds/SEED-261119-nus-v1-2-preview-and-remote-urls.md, REQUIREMENTS.md PREV-01..05, ROADMAP Phase 11 success criteria 1-5.
</domain>

<decisions>

### Locked by the user and the roadmap
- **Preview tab focus (user, 2026-11-21: "Only the viewer you typed in"):** the viewer that last typed into the session (the Phase 10 grid owner / last `NoteTyping` viewer) switches to the new preview tab; every other viewer (e.g. the unattended office window) gets it as a background tab.
- A preview tab sits beside its session's terminal tab; closing it leaves the terminal and the session untouched.
- No generic file-read API on the daemon or the share server (PREV-03); only files explicitly opened from inside the shell are sent.
- Markdown is untrusted: sanitized, no script, no remote fetches. SVG renders safely (no script, no external fetch).
- Share page CSP changes only as far as this phase's security review approves; the token gate, tailnet-only listener and `frame-ancestors 'none'` stay unchanged.
- PREV-05 (user, 2026-11-20): automatic PATH inside Portty sessions plus a menu item "Install Command-Line Tool…" (admin prompt, `/usr/local/bin/portty`, refuse to overwrite a foreign link, cancel changes nothing).
- macOS-native look, light and dark mode live; radical simplicity.

### Decided autonomously (reversible)
| Question | Decision | Why |
|---|---|---|
| Where the file is read | The `portty open` CLI reads the file itself (it runs as the user, in the shell, on the host Mac), then sends the bytes to its local daemon over the unix socket together with the session it runs in (from an env var the daemon sets in each session, e.g. `PORTTY_SESSION_ID`; research confirms or adds it). The daemon never opens a path given to it. | Keeps "no file-read API": the daemon only relays bytes a same-UID local process handed it for a session. |
| Transport to viewers | A new relay message type on the session hub, chunked under the existing frame limits (and under `relay.ViewerReadLimit` for remote viewers), with a header (preview id, file name, MIME type, total size) and an end marker; delivered to viewers connected at that moment, through the peer proxy unchanged. Interleaves with terminal output so the terminal keeps updating (SC3). | Reuses the authenticated stream; peer tabs and browsers get it for free. |
| Size cap | 20 MiB per file (named constant). Over the cap: the CLI prints a one-line "too large to preview" error and the viewers get nothing, or (if the cap is crossed mid-stream) a "too large to preview" placeholder, never a partial render. | Big enough for typical PDFs and screenshots; bounded memory. |
| Type detection | By content sniffing plus extension (PNG, JPEG, GIF, WebP, SVG, PDF, Markdown `.md`/`.markdown`); anything else → CLI error "can't preview this file type". A missing file → CLI error. | One-line CLI errors per SC1. |
| Image rendering | `<img>` from a `blob:` URL (SVG included: an SVG in an `<img>` runs no script and fetches nothing). | Browser-enforced safety for SVG; minimal CSP change (`blob:` in img-src) for the security review. |
| PDF rendering | WKWebView's built-in PDF view from a `blob:` URL (iframe/embed/object). **Proven first** in the real Wails v2 web view (a spike task before anything is built on it, per SC2). If it doesn't work in Wails, stop and report options (e.g. pdf.js) rather than improvising. | User asked for native look; the spike guards the risk. |
| Markdown rendering | A small Markdown renderer plus a sanitizer (e.g. `marked` + `DOMPurify`; research picks, checking maintenance, size and supply-chain risk; exact pinned versions). Remote images and media are not fetched (dropped or shown as a placeholder link); links go through the existing LinkConfirmPopover/ALLOWED_SCHEMES path; no raw HTML passthrough beyond the sanitizer's safe set. Styled with the app's native look in light/dark. | Remote content is untrusted; reuse existing link handling. |
| Browser share viewer | Same React viewer (the share page is `/app/`); the CSP change is the minimum the renderers need (`blob:` for img and for the PDF frame, if required), recorded in the security review. | PREV-04. |
| Older peers | Viewers that don't know the new message type must ignore it (research confirms what old viewers do with unknown frame types); `portty open` against a daemon too old to relay previews prints the existing "update needed" style message (API version from Phase 9; bump `api` if the planner needs a capability check). | Mixed versions degrade cleanly. |
| Preview lifetime | Ephemeral: held in the viewer's memory only; not persisted across app restart; a viewer that joins later doesn't receive earlier previews. Closing a preview tab is local to that viewer. | Simplest; no storage of remote content. |
| PREV-05 PATH | The daemon adds the app bundle's binary directory (`…/Portty.app/Contents/MacOS` via a symlink or the binary's own dir) to each new session's PATH, in a way that survives zsh's login `path_helper`; existing sessions are unaffected. The CLI name inside sessions is `portty`. | Works with no setup in new sessions. |
| PREV-05 menu item | "Install Command-Line Tool…" in the Portty app menu: uses the standard macOS admin prompt (e.g. `osascript … with administrator privileges`) to create `/usr/local/bin` if needed and the symlink to the app's binary; if `/usr/local/bin/portty` exists and points elsewhere, show what it points to and don't overwrite; cancel → no change; success/failure shown in a native dialog. | Matches the user's decision; no installer package. |

### Decided after research and UI spec (2026-11-21, autonomous; see 11-RESEARCH.md and 11-UI-SPEC.md)
| Question | Decision | Why |
|---|---|---|
| UI-SPEC defaults | Accepted as written: at most 10 preview tabs per viewer (the 11th closes the oldest one not being viewed), plus a 100 MiB retained cap; reopening the same file (matched by full path, never shown) refreshes its tab; the install command refuses every existing link that isn't to this copy of Portty; the typing viewer switches only if it is on that session's tab or one of its previews; the cap is 20,000,000 bytes so "20 MB" is exact. Where research and UI-SPEC differ (8 vs 10 tabs; previews after session end), UI-SPEC wins. | Small, reversible, consistent with simplicity and reliability. |
| Paced delivery | Previews go to each subscriber through a paced per-subscriber sender (`Hub.SendPreview`) that uses only a small window of the 256-slot queue, never trips the slow-viewer kick, skips viewers that are still replaying, allows one in-flight preview per hub, and doesn't hold the daemon's write gate during streaming (the route buffers, returns 202 and paces asynchronously, so a Phase 9 handoff drain isn't blocked). | Research: a plain broadcast would drop viewers (SC3) and could fail an upgrade handoff. |
| Over-cap (SC3 wording) | The CLI refuses an over-cap file with its one-line "too large to preview" message, which every viewer sees in the terminal; viewers get no tab. A cap tripped mid-stream (shouldn't happen, since the daemon checks before sending) shows the Too Large state, never a partial render. | Satisfies SC3's "clear message, never a partial render" without placeholder tabs on unattended viewers. |
| Session id | Generate the id before spawn and pass `PORTTY_SESSION_ID` as a required env entry; sessions from older builds (including shells moved over by a Phase 9 handoff) get one clear CLI line. `portty open` never starts or restarts the daemon (`open` added to `cliNeedsDaemon`'s exclusions). `APIVersion` goes to 3, with a `PreviewAPI` gate for the CLI. | Research findings. |
| Markdown dependencies | `marked` + `dompurify`, exact pins, lazy-loaded. Use the newest versions published at least 14 days before install (the executor checks npm publish dates and records them), with DOMPurify no older than the newest release containing the fixes the research lists. No human checkpoint is needed for the age flag. Sanitizer config as in research: html dropped, images to placeholders, explicit ALLOWED_TAGS, http/https/mailto only, no style attribute. | The two-week rule replaces the "too new" checkpoint. |
| Desktop CSP | The desktop app has no CSP. Don't add one in this phase, since the sanitizer is the defence there; note it in the security review as an observation. The share page gets the minimal delta (`blob:` in img-src, plus frame-src or object-src per the spike); `frame-ancestors 'none'` stays. | Adding a desktop CSP is a separate change with wide blast radius. |
| PDF in Safari | If the spike shows Safari can't render the PDF blob under the share CSP without loosening `frame-ancestors`, use the "Open PDF" fallback (opens the blob top-level). Never loosen `frame-ancestors`; pdf.js only with the user's sign-off. If PDF fails in the Wails app itself, stop and report. | Research decision tree. |
| PATH entry | A per-user symlink `<UserConfigDir>/portty/bin/portty` → resolved `os.Executable()`, refreshed atomically on each CreateSession, prepended to the session PATH (it survives zsh `path_helper`, searched after system dirs). | Proper lowercase name; tab completion works. |

### Decided at planning (2026-11-21, autonomous)
| Question | Decision | Why |
|---|---|---|
| DOMPurify version | Pin `dompurify@3.4.17` (the newest at least 14 days old). The only fix in 3.4.18 (GHSA-qm7q-q8h9-jg8q) is in `SAFE_FOR_TEMPLATES` mode, which our config doesn't use; the security review records this. Todo: move to 3.4.18 or later once it is 14 days old (2026-11-28). `marked@18.1.3` meets both rules. | The two-week rule holds, and the advisory doesn't apply to our config. |
| Package legitimacy | The 14-day rule replaces the human checkpoint for "too-new" only; 11-06 still stops on SLOP or any other reason. | Confirmed as planned. |
| Extra copy | Accept the planner's CLI busy line, the Open PDF pane copy (spike verdict b), the old-session wording (verbatim UI-SPEC line), and task lists as ☑/☐ glyphs. | Consistent with UI-SPEC style. |
| Spike gate | 11-01 changes no repo code, so its gate is "only 11-SPIKE.md changed" (no Go suite). | Saves firewall alerts. |

### Claude's Discretion
- Plan split, file names, test names, chunk size, exact MIME sniffing, the preview tab's visual details within the UI spec.
</decisions>

<code_context>
- CLI: cmd_cli.go (subcommand routing), main.go launchMode (lesson dcbe999d: anything touching routing needs a real-launch check), internal/daemon/client.go and api.go (unix-socket routes), internal/daemon/engine.go (session env when spawning)
- Relay: internal/relay/protocol.go (message types), hub.go (broadcast to subscribers), webserver/server.go and relay/server.go write pumps, internal/daemon/remote_ws_proxy.go (peer tabs, copies frames verbatim), relay.ViewerReadLimit
- Frontend: frontend/src/App.tsx (tabs, tab ids from Phase 10 `webSessionTabId(id, host)`), components/TerminalPanel.tsx, WebShareSessionView.tsx, LinkConfirmPopover, lib (ALLOWED_SCHEMES), relayClient.ts (frame parsing)
- CSP: internal/webserver/csp_mw.go and the app's frontend/index.html CSP
- Menu: the Wails native menu builder (BuildMenuItems / app menu), app.go bindings, frontend/src/wailsjs stubs (hand-maintained)
- PATH: internal/daemon/path.go AugmentServicePath (daemon's own PATH), session spawn env
</code_context>

<specifics>
- Toolchain `export GOTOOLCHAIN=go1.26.11`. Gates: `go build ./... && go vet ./...`, `go test -race -short ./...` (nothing skipped beyond the known env/-short gates), `cd frontend && pnpm vitest run` + tsc, check-brand, check-traceability. Each full suite at most once per plan (each test binary fires a firewall alert for the user); outputs to files, never piped through tail. New npm dependencies: exact pinned versions, lockfile updated, `pnpm install --frozen-lockfile` afterwards, and the dependency named in the security review.
- VM checks on the two Tart VMs (office `portty-test` 192.168.64.108, home `portty-test-2` 192.168.64.109; ephemeral tailnet nodes: never stop them, never `tailscale down/logout`). Real Wails builds for the PDF spike and the preview/menu checks; ad-hoc signing only; the admin-prompt menu check runs in a VM, never on the host; never touch the user's Portty.
- Worktrees start at an old commit: fast-forward to gsd/v1.2 first.
</specifics>

<deferred>
- Previews for other types (video, audio, plain text/code with syntax highlight).
- Persisting previews or sending earlier previews to late joiners.
- Path click (Phase 12) builds on this phase's preview path.
</deferred>
