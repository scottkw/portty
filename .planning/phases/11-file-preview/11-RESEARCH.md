# Phase 11: File Preview - Research

**Researched:** 2026-11-21
**Domain:** Go daemon/relay wire extension, React tab model, sandboxed renderers (image/SVG/PDF/Markdown) in Wails v2 WKWebView and a browser share page, macOS PATH/CLI install
**Confidence:** MEDIUM overall (HIGH on repo mechanics and PATH behaviour, which were read or run this session; the PDF-in-Wails and PDF-under-share-CSP questions are genuinely unknown until the spike runs)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Locked by the user and the roadmap**
- **Preview tab focus (user, 2026-11-21: "Only the viewer you typed in"):** the viewer that last typed into the session (the Phase 10 grid owner / last `NoteTyping` viewer) switches to the new preview tab; every other viewer (e.g. the unattended office window) gets it as a background tab.
- A preview tab sits beside its session's terminal tab; closing it leaves the terminal and the session untouched.
- No generic file-read API on the daemon or the share server (PREV-03); only files explicitly opened from inside the shell are sent.
- Markdown is untrusted: sanitized, no script, no remote fetches. SVG renders safely (no script, no external fetch).
- Share page CSP changes only as far as this phase's security review approves; the token gate, tailnet-only listener and `frame-ancestors 'none'` stay unchanged.
- PREV-05 (user, 2026-11-20): automatic PATH inside Portty sessions plus a menu item "Install Command-Line Tool…" (admin prompt, `/usr/local/bin/portty`, refuse to overwrite a foreign link, cancel changes nothing).
- macOS-native look, light and dark mode live; radical simplicity.

**Decided autonomously (reversible)**

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

### Claude's Discretion
- Plan split, file names, test names, chunk size, exact MIME sniffing, the preview tab's visual details within the UI spec.

### Deferred Ideas (OUT OF SCOPE)
- Previews for other types (video, audio, plain text/code with syntax highlight).
- Persisting previews or sending earlier previews to late joiners.
- Path click (Phase 12) builds on this phase's preview path.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PREV-01 | `portty open <file>` inside any Portty session opens a preview tab in every app viewer of that session | Session id env var (Sec. 1), unix-socket route (Sec. 2), relay frame + paced fan-out + focus flag (Sec. 3), tab model with the owner view kept mounted (Sec. 4) |
| PREV-02 | Images, PDF, Markdown render with macOS-native look, light/dark, hostile content inert | Renderers (Sec. 5): `<img blob:>`, marked + DOMPurify strict config, PDF spike; hostile fixture list and beacon-server proof |
| PREV-03 | Bytes only over the session's authenticated stream; size cap; "too large"; no generic file-read API | Cap enforcement in CLI + daemon (Sec. 2), route-absence tests modelled on `files_removed_test.go`, source gate against `os.Open` in daemon/relay preview files |
| PREV-04 | Browser share viewers get previews; CSP changes only as far as the security review allows | Minimal CSP delta (`img-src … blob:`, `frame-src blob:` or `object-src blob:`), blob-inherits-CSP risk to test in Safari (Sec. 6) |
| PREV-05 | `portty` on PATH in sessions automatically; menu item installs `/usr/local/bin/portty` | path_helper verified by running it (Sec. 7), per-user symlink dir recommendation, osascript argv form, test seams |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- `./CLAUDE.md` chain read: `/Users/ken/dev/CLAUDE.md`, `/Users/ken/dev/portty/CLAUDE.md`, `/Users/ken/dev/portty/.claude/CLAUDE.md`. Directives that bear on this phase:
- **TESTING.md is the regression home.** Every phase that adds tests updates it: Section 2 manifest counts, Section 4 traceability rows (path column is a repo-relative file path only, `.go/.ts/.tsx/.sh`), Section 5 manual `M-NN` items for anything not automatable. Run `bash tests/check-traceability-paths.sh` before committing. Last Category is AZ (M-116..M-120); this phase is Category BA starting at M-121. Current counts: Go 152, vitest 95, Playwright 7, build-script 1, total 255 [VERIFIED: TESTING.md:24-36].
- **Chesterton's Fence / deletion discipline**; **security: keep token-gated tailnet-only share links and the CSP intact**; **macOS only**; **keep inherited stack** (Go/Wails/React/tsnet); **no new relay servers or cloud dependencies**.
- Python/FastAPI rules are irrelevant here. Frontend: pnpm, TypeScript, ESLint + Prettier. Go: `ctx` aware, `go fmt`, golangci-lint.
- **Silent fallbacks forbidden** (`or {}` converts hard failures to silent corruption): preview code must surface errors (CLI one-liners, viewer "couldn't load" tile), not default to empty renders.
- **Never run `go test` binaries for the user's firewall**: orchestrator rule for this research; plan gates run each full suite at most once per plan (CONTEXT specifics).
- GSD enforcement: no direct repo edits outside a GSD workflow. MEMORY: radical simplicity, macOS-native look, drive GUI checks in the Tart VMs (never the host), rock-solid first.

## Summary

The phase is feasible with the inherited stack and no new server surface, but three things in the decided design need explicit engineering that the CONTEXT does not mention, and two are genuine unknowns.

**Must-handle design facts (all evidence-backed).** (1) The daemon sets **no** session-id env var today: `pty.CreateRequest` has no `Env` set by `CreateSession`, and the session id is generated *after* `cmd.Start()` [VERIFIED: internal/pty/native.go:59,76,83; internal/daemon/engine.go:228-235], so the id generation must move before the spawn. (2) A naive `broadcast`-style fan-out of a 20 MiB file would **kick every viewer**: each viewer's queue is `make(chan []byte, 256)` and `sendLocked` kicks on a full queue [VERIFIED: internal/relay/server.go:89, internal/webserver/server.go:856, internal/relay/hub.go:398-415], so previews need a paced, per-subscriber, non-kicking sender that holds at most a small window of the 256 slots. (3) **Peer tabs and the browser viewer unmount their relay socket when their tab is not active** (only one `WebShareSessionView` is rendered, for the active `__websession__` tab) [VERIFIED: frontend/src/App.tsx:1178; 1275-1278 skips `web-session` tabs in the local map]. Focusing the new preview tab would therefore tear down the terminal view and its socket mid-stream. The owner session view must stay mounted (hidden) while its preview tab is active.

**Genuine unknowns.** PDF from a `blob:` URL inside the Wails v2 WKWebView, and PDF under the share page CSP in Safari, where WebKit has documented cases of a `blob:` document inheriting the parent's CSP including `frame-ancestors` [CITED: bugs.webkit.org/show_bug.cgi?id=290935 (WebKitGTK; macOS uses Apple's own PDF implementation, so it may not carry over)]. Web search found **no** evidence either way for Wails v2 on macOS. The spike must test both surfaces and has a decision tree with fallbacks.

**Good news verified by running it.** zsh's login `path_helper` keeps pre-existing PATH entries and appends them after the system entries, so a daemon-prepended directory survives (it moves to the end) [VERIFIED: ran `/usr/libexec/path_helper -s` and `zsh -l` on macOS 27.2.1, output quoted in Sec. 7]. Old viewers drop unknown frame types (JS `parseServerFrame` default → `unknown`, Go attach loops have no default arm), and the peer proxy copies frames verbatim, so a new frame type is backward compatible with no proxy change.

**Primary recommendation:** Build in this order: PDF/CSP spike in parallel with the Go relay/daemon/CLI substrate; then the frontend tab model (keeping the owner view mounted); then renderers (`marked` + `DOMPurify`, `<img blob:>`, PDF per spike) and the minimal CSP delta; PREV-05 (per-user `…/portty/bin/portty` symlink dir on the session PATH, plus the menu item with an argv-style `osascript`); gate and VM checks last.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Read the file, sniff type, enforce cap at source | CLI (`portty open`, runs as the user in the shell on the host Mac) | Daemon (re-validates cap and type enum) | The shell's own process is the only thing allowed to open a path; the daemon never receives a path |
| Identify the session (env var) | Daemon spawn env (`internal/pty/native.go`) | CLI reads `PORTTY_SESSION_ID` | Env is inherited by the shell and its children, no lookup needed |
| Accept bytes, fan out to viewers | Daemon unix-socket mux + relay Hub | Peer daemon proxy (verbatim copy), share server write pump | Authenticated stream already exists per session; unix socket dir is `0700` |
| "Focus only the typist" | Relay Hub (knows owner via `NoteTyping`) | Frontend honours a per-viewer flag | The hub is the only place that knows which subscriber typed last |
| Assemble chunks, own preview tabs and blobs | Frontend (App-level store) | TerminalPanel/RelayClient parse frames | Previews must outlive the (possibly unmounted) terminal view and be revoked on close |
| Render image/SVG/PDF/Markdown | Browser/WebView (`<img>`, PDF viewer, sanitized DOM) | — | Browser-enforced isolation is the safety property for SVG; OS parses PDF |
| CSP for the share page | Share server (`csp_mw.go`) | — | Desktop (Wails) page has **no** CSP at all [VERIFIED: grep, see Sec. 6] |
| PATH + CLI install | Daemon (per-user symlink dir, session env) | GUI (menu item, native dialogs, `osascript`) | Session PATH is set at spawn; `/usr/local/bin` needs admin, GUI only |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `marked` | 18.2.1 (pin exact; latest as of 2026-11-16) | Markdown to HTML string | Zero runtime deps, MIT, SLSA provenance on npm, 137.9M weekly downloads, active (weekly releases), repo `markedjs/marked` [VERIFIED: npm registry + `gsd package-legitimacy check`] |
| `dompurify` | 3.4.18 (pin exact; latest as of 2026-11-14) | HTML sanitizer for the rendered Markdown | The standard DOM sanitizer, `(MPL-2.0 OR Apache-2.0)`, ships its own types, only dep is `@types/trusted-types`, maintained by cure53; **but** see advisory churn below [VERIFIED: npm registry; GitHub security advisories] |

Both are flagged `SUS` by the legitimacy seam with reason `too-new` only (the *latest version* was published within the last week; the packages themselves are years old, high-download, with source repos and no postinstall). This is the protocol's `SUS` handling, so the planner must add a `checkpoint:human-verify` before the install (see audit table).

**Why DOMPurify advisory churn is acceptable here.** The 2026 DOMPurify advisories (about 20 since April) are almost all in `IN_PLACE`, `SAFE_FOR_TEMPLATES`, `CUSTOM_ELEMENT_HANDLING`, hook mutation and `PARSER_MEDIA_TYPE` modes; the one `high` is "XSS via `selectedcontent` re-clone" fixed in 3.4.5 [VERIFIED: `gh api repos/cure53/DOMPurify/security-advisories`]. The recommended config uses none of those modes (default string-in/string-out, explicit tag allowlist), and pins >= 3.4.18 which contains every listed fix. Because this library is the last line of defence, the plan must also pin and re-check at install time (`npm view dompurify version`) and tell the user that DOMPurify needs periodic bumps.

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| (none new) | — | Images/SVG use `<img>`; PDF uses the OS viewer; links reuse `LinkConfirmPopover`, `isAllowedScheme`, `getRisk`, `openLink` | — |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `marked` | `markdown-it` 15.0.3 (`html:false`, 8+ deps incl. `linkify-it`, `argparse`, `entities`) | `html:false` skips raw HTML by construction, but more deps and weight; `OK` verdict. Choose only if `marked` fails the hostile-fixture suite |
| `marked` | `micromark` 4.0.3 (core only emits HTML; deps `debug`, `devlop`, many `micromark-util-*`) | Safest raw-HTML default (off), no GFM tables without extension; more packages. Not worth it |
| `DOMPurify` | `rehype-sanitize` (unified pipeline) | Pulls the whole unified tree (remark/rehype, dozens of packages): much larger supply chain |
| PDF OS viewer | `pdf.js` (Apache-2.0) | Last-resort fallback only if the spike fails in Wails; large worker, needs `worker-src`/wasm CSP work on the share page, violates "stop and report" unless the user approves |

**Installation (planner: run from `frontend/`, exact pins, then lockfile + frozen install):**
```bash
pnpm add --save-exact marked@18.2.1 dompurify@3.4.18
pnpm install --frozen-lockfile
```
Existing deps use `^` ranges; CONTEXT requires exact pins for the new ones (`--save-exact`). No `@types/dompurify` is needed (DOMPurify ships types) [VERIFIED: `npm view dompurify@3.4.18 types exports`].

**Version verification:** `npm view marked@18.2.1` = published 2026-11-16, `dist.attestations.provenance` SLSA v1, deps none, `engines.node >= 20`; `npm view dompurify@3.4.18` = published 2026-11-14, deps `@types/trusted-types ^2.0.7`. `marked` 18.0.0 had an OOM DoS (GHSA-6v9c-7cg6-27q7) fixed in 18.0.2, so never pin below that [VERIFIED: gh api advisories].

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| marked | npm | years (v18 line since 2026-04), latest 18.2.1 published 2026-11-16 | 137,857,313/wk | github.com/markedjs/marked | SUS (`too-new`: latest version < 14 days) | Flagged: planner adds `checkpoint:human-verify` before install; SLSA provenance present, no postinstall |
| dompurify | npm | years, latest 3.4.18 published 2026-11-14 | 107,562,344/wk | github.com/cure53/DOMPurify | SUS (`too-new`: latest version < 14 days) | Flagged: planner adds `checkpoint:human-verify`; no postinstall; only dep `@types/trusted-types` |
| markdown-it | npm | years | 41,936,773/wk | github.com/markdown-it/markdown-it | OK | Alternative only, not recommended |
| micromark | npm | years | 102,173,328/wk | github.com/micromark/micromark | OK | Alternative only, not recommended |

**Packages removed due to SLOP:** none.
**Packages flagged SUS:** `marked`, `dompurify` (reason `too-new` only). The human-verify step should check: npm page shows the expected maintainers (`marked`: chjj, styfle, amidknight, tonybrix; `dompurify`: cure53), the tarball integrity matches the lockfile (`marked@18.2.1` sha512-ii4bxCTC…, `dompurify@3.4.18` sha512-ofe3wI/u…), and `pnpm install --frozen-lockfile` is clean. Option if the user prefers a cool-off: `marked@18.2.0` (2026-11-09) is acceptable (>= 18.0.2); do **not** fall back on DOMPurify below 3.4.18 without re-reading the advisory list. Training-data or search-discovered names are `[ASSUMED]` unless confirmed here; both names were confirmed against the registry plus GitHub advisories this session. Package ecosystem is npm; Go side adds no packages (stdlib `net/http` sniffing, `os/exec`).

## Architecture Patterns

### System Architecture Diagram

```
 host Mac (office)                                                       viewer Macs / browsers
 ------------------------------------------------------------------      -----------------------------------
 zsh login shell (PORTTY_SESSION_ID, PATH incl. <cfg>/bin/portty)
   $ portty open report.pdf
        |  1. CLI: os.Open path, stat, cap 20 MiB, sniff -> {name, mime}
        v
   unix socket  <UserConfigDir>/portty/daemon.sock (dir 0700)
   POST /sessions/{id}/preview   (daemon socket mux ONLY; not RelayHandler,
        |                         not share server)
        |  2. handler: session->hub, validate cap/type/name, read body fully
        |     (MaxBytesReader), 202 {viewers:n}; releases the write gate
        v
   relay.Hub.SendPreview (new, internal/relay/preview.go)
        |  3. snapshot subscribers (skip replaying ones; read-only viewers included),
        |     owner := h.owner  -> per-subscriber header {focus: sub==owner}
        |     one paced sender goroutine per target; window <= W slots of 256;
        |     NOT sendLocked/broadcast (would kick)
        v
   sub.Msgs (cap 256) --> relay.WritePump (pong ahead of frames) --> WebSocket
        |        interleaved with MsgOutput terminal frames in the same queue
        +--> local app tab (Origin local, ws://127.0.0.1:relayPort)  ------> TerminalPanel -> RelayClient
        +--> share server conn (Origin web)  --TLS/tailnet-->  browser /app/ (Safari)  -> RelayClient
        +--> share server conn --tailnet--> home daemon proxy (copyWS verbatim,
                                        upstream SetReadLimit 1 MiB) -> home app peer tab
                                                    |
   frontend: RelayClient.handleFrame('preview')  -> previewAssembler (header, chunks, end)
        -> App-level previewStore (Blob, objectURL, ownerTabId, focus)
        -> new Tab {type:'preview', ownerTabId}; setActiveId only if focus
        -> PreviewPanel: image/SVG <img src=blob:>, PDF frame (per spike), Markdown (marked -> DOMPurify)
        close tab -> URL.revokeObjectURL; owner view stays mounted while its preview tab is active
```

### Recommended Project Structure
```
cmd_open.go                         # `portty open` (CLI): sniff, cap, POST via DaemonClient
internal/daemon/preview.go          # POST /sessions/{id}/preview handler (socket mux only)
internal/daemon/types.go            # APIVersion 3, PreviewMaxBytes, PreviewAPI
internal/daemon/clilink.go          # per-user bin dir + symlink refresh (PREV-05 PATH)
internal/relay/preview.go           # MsgPreview, frame builders/parsers, Hub.SendPreview (paced)
internal/relay/protocol.go          # + MsgPreview byte = 0x21
internal/pty/native.go              # PORTTY_SESSION_ID in the spawn env; id generated before Start
app_cli_install.go                  # menu action: inspect link, osascript, native dialogs (seams)
frontend/src/lib/relayClient.ts     # + MSG_PREVIEW, ServerFrame 'preview', onPreview callback
frontend/src/lib/previewAssembler.ts# header/chunk/end -> Blob, bounds, abort rules (pure, tested)
frontend/src/lib/previewStore.ts    # App-level store: tabs' blobs, object URLs, revoke on close
frontend/src/lib/markdown.ts        # lazy-loaded: marked instance + DOMPurify config + hooks
frontend/src/components/PreviewPanel.tsx   # per-type renderers, states (loading/failed/too large)
```

### Pattern 1: Wire format (new `MsgPreview = 0x21`)
**What:** One new server-to-client type in the reserved range, a sub-kind byte, JSON for header/end, binary chunks. Never recorded in the ring, so offsets and resume are untouched (`relayClient.ts` counts only `MSG_OUTPUT` frames toward the offset).
**When to use:** Always for previews; client-to-server frames of this type are already dropped by both read pumps (no `case`, no default arm).
**Verified anchors (verbatim):** `MsgOutput  byte = 0x01 // PTY stdout/stderr → client` and `MsgMeta byte = 0x20` with the note "Reserved range 0x20-0x2F for future server-push frame types." [VERIFIED: internal/relay/protocol.go:13,22-23]; `var ReplayChunk = 16 << 10` and "Any value up to 32767 fits coder/websocket's 32768-byte default read limit" [VERIFIED: internal/relay/protocol.go:100-106]; `const ViewerReadLimit = 4 * DefaultScrollbackBytes` with `DefaultScrollbackBytes = 256 * 1024`, i.e. 1 MiB [VERIFIED: internal/relay/scrollback.go:5,20].
**Proposed layout (planner may rename):**
```
header: [0x21][0x01] JSON {"id":<uint32>,"name":"<basename>","mime":"<enum>","size":<n>,"focus":<bool>}
chunk : [0x21][0x02][id u32 BE][bytes...]            // payload <= PreviewChunk (16 KiB, = ReplayChunk precedent)
end   : [0x21][0x03][id u32 BE][status u8]           // 0 ok, 1 aborted, 2 too large
```
Chunk 16 KiB means 1,280 frames for 20 MiB, far under every read limit (32 KiB default for browser/proxy-client side, 1 MiB `ViewerReadLimit` for Go viewer-side readers, which both the peer proxy upstream (`remote_ws_proxy.go:131`) and `portty attach` (`attach.go:41`, `loop.go:187`) set) [VERIFIED].
**Peer proxy:** `copyWS` reads whole messages and writes them verbatim to the other side, never parsing [VERIFIED: internal/daemon/remote_ws_proxy.go copyWS], so no proxy change and the per-viewer `focus` bit rides inside the header unchanged.
**Old viewers:** the JS client `default: return { type: 'unknown' }` and `// 'unknown' frames are dropped` [VERIFIED: frontend/src/lib/relayClient.ts parseServerFrame/handleFrame]; Go attach (`internal/attach/loop.go:310-350`, `attach.go:134-146`) switches on `MsgOutput`/`MsgMeta`/`MsgPong` with no default arm, so unknown frames are ignored, though `lastFrame` liveness is refreshed (harmless) [VERIFIED].

### Pattern 2: Paced per-subscriber sender (the part CONTEXT does not mention)
**What:** `Hub.SendPreview(ctx, p Preview) (viewers int)`: under `h.mu`, snapshot targets (subscribers with `!replaying`, still subscribed, not revoked) and read `h.owner`; then for each target run one goroutine that, per chunk, takes `h.mu`, rechecks the subscriber is still in `h.subscribers`, and enqueues **only if `len(sub.Msgs) <= window`** (window about 16-32 of the 256 slots), else sleeps a few ms; abort that target (send best-effort `end status=aborted`, never `CloseSlow`) after N seconds without progress or when the hub ends. All producers enqueue under `h.mu` (verified: `sendLocked` is the only `Msgs <-` site), so the check-then-send under `mu` cannot block or race.
**Why not `broadcast`/`sendLocked`:** `sendLocked` is a non-blocking send that kicks on a full queue [VERIFIED: internal/relay/hub.go:398-415, quoted: `select { case sub.Msgs <- frame: default: h.kickSlow(sub) }`]; 1,280 chunks into a 256-slot queue would drop the viewer (SC3: "no viewer is dropped").
**Why a small window:** terminal output frames share the same queue; with window W of 16 KiB chunks the worst added terminal latency is about W x 16 KiB / link rate (32 slots = 512 KiB: about 0.5 s at 1 MB/s). The viewer's `WritePump` writes owed pongs ahead of the queue, so heartbeat liveness is not delayed by the preview [VERIFIED: internal/relay/pong.go WritePump]. Keep the window small (planner constant, e.g. 16).
**Skip replaying subscribers:** `Join` sets `sub.replaying = true`; `sendLocked` diverts frames to `pending`, and `pending` over `ReplayPendingMax` (4 MiB) kicks the viewer [VERIFIED: hub.go:399-406,1820; protocol.go:119]. Preview chunks sent to a joining viewer would do exactly that. Snapshot targets at start, skip `replaying`, never include later joiners (ephemeral per CONTEXT).
**Focus:** `focus = (sub == h.owner)`. `owner` is set by `NoteTyping` (typed input that passes `IsUserInput`, write-capable, has reported a size) and by `ClaimGrid`, and cleared on `Unsubscribe` [VERIFIED: hub.go:563-585, 343-368]. With no owner (nobody typed since the owner left or a handoff), nobody focuses: all get background tabs (recommended; matches "only the viewer you typed in"). Through a peer proxy the home GUI's typing reaches the office share-server pump as that connection's `NoteTyping`, so the home peer tab is the owner and receives `focus:true` [VERIFIED by reading the pumps: webserver/server.go case `relay.MsgInput` -> `hub.NoteTyping(sub)`].
**One in-flight preview per hub**: a second request while one streams returns 429; CLI prints one line. Bounds memory to one 20 MiB buffer per session.
**Handoff:** previews are never exported (not part of `HubState`); a preview in flight at COMMIT ends when viewers get 1012 and reconnect; viewers must discard partial assemblies on socket close.

### Pattern 3: Daemon route on the unix socket mux only
**What:** `a.mux.HandleFunc("POST /sessions/{id}/preview", a.handlePreview)` registered in `registerRoutes` beside `POST /handoff` (api.go:199), never on `RelayHandler()` (127.0.0.1 TCP: reachable by any local process **and by a browser page**) or the share server. The unix socket's parent dir is `drwx------` and the socket file is `srwxr-xr-x` on this machine [VERIFIED: `stat` of `~/Library/Application Support/portty`], i.e. protection is the `0700` directory created by `DefaultSocketPath` (`os.MkdirAll(dir, 0700)`, socket.go:18). Every other mutating route (create/kill session, share) rests on the same property, so a peer-credential check (the `/handoff` pattern at handoff.go:458, `fdpass.PeerUID`) is **not** needed for parity and the stock `http.Serve` has no per-conn context to carry it; adding it is optional hardening, not recommended (simplicity).
**Handler rules (all enforce PREV-03):**
- Accept only a body plus headers (`X-Portty-Name`, `X-Portty-Mime`); **never** a path field. Wrap `r.Body` in `http.MaxBytesReader(w, r.Body, PreviewMaxBytes)`; on overflow reply 413 and send `end status=2` to anyone already given a header (never reached in practice since the header goes out after the body is buffered).
- Validate `mime` against the fixed enum, reduce `name` to a basename with control characters stripped and a 255-byte cap, check hub exists (`a.engine.Manager().Get(id)`), hub not done (409), then buffer fully, respond `202 {"viewers":n}`, and pace in a goroutine detached from the request context.
- **Gate interplay (critical):** `freezeGate` treats every non-GET, non-exempt request as a write: `enterWrite()` takes `writeGate.RLock` for the *whole request* and a handoff drains writes for only `writeDrainTimeout = 1 * time.Second` [VERIFIED: api.go:131-147; handoff.go:76]. A handler that streamed a 20 MiB upload to viewers inside the request would block a handoff drain and fail upgrades. Read the body into memory, return, and pace asynchronously, so the gate is held only for the local copy; add a test that a handoff drain succeeds while a preview is streaming.
- **Frozen daemon:** a POST during a freeze gets `503 Retry-After`; `DaemonClient.doRetrying` retries mutating calls up to `frozenRetryBudget = 20 * time.Second` and needs a fresh body per attempt (use a `bytes.Reader` in the `newReq` closure) [VERIFIED: client.go:308-341].
**API version gate:** bump `APIVersion` to 3 and add `PreviewAPI = 3`; the CLI requires `DaemonAPI(h) >= PreviewAPI` for `open` and otherwise prints the existing update-needed shape. `MinDaemonAPI` stays 1 (so a Not-Now old daemon keeps serving everything else). `internal/daemon/client_test.go:41` pins `APIVersion != 2` and must change; `listener_test.go` covers `/info` `api` (takes a parameter, no change needed) [VERIFIED: types.go:60-62; client.go:84-96; client_test.go:41].

### Pattern 4: `portty open` must not use the restart-capable ensure path
`runCLI` calls `ensureCLIDaemon(daemon.EnsureDaemon, …)` for every daemon command, and a mismatched-build daemon with live sessions *refuses with advice to re-run with `--restart-daemon`* [VERIFIED: cmd_cli.go:134-156 message; main.go:243-249]. For `portty open`, that advice would end the very session the user is typing in. The shell that runs `open` is by definition hosted by a running daemon, so `open` should skip `ensureCLIDaemon` (add `"open"` beside `health`/`version` in `cliNeedsDaemon`, cmd_cli.go:72), dial the socket directly, and map "connection refused" to "Portty's background service isn't running", then read `/health` once for the API gate. `launchMode` already routes any first arg not starting with `-` to the CLI (main.go launchMode), so `open` needs no GUI-routing change; per lesson dcbe999d, still include a real-launch check (`<bin> open --help`-style) in the VM list.

### Pattern 5: Session identity
`pty.NativePTYBackend.Create` builds the env as `cmd.Env = mergeEnv(os.Environ(), req.Env, "TERM=xterm-256color", "COLORTERM=truecolor")` *before* `cmd.Start()`, and calls `generateID()` only after `Start` [VERIFIED: internal/pty/native.go:59,76,83]. `engine.CreateSession` passes no `Env` [VERIFIED: engine.go:228-235]. Recommended: in `Create`, call `generateID()` before building `cmd.Env` and pass `"PORTTY_SESSION_ID="+id` as an extra `required` value (required overrides base and extra, so a `PORTTY_SESSION_ID` inherited from a parent Portty shell that launched the daemon never leaks through). Session ids are 32-hex strings from `crypto/rand`. The id is not a secret (shares use separate capability tokens), and any same-UID process can already list sessions via the socket.
**Remote sessions:** the shell, `portty`, the daemon socket and the hub are all on the host Mac; `DefaultSocketPath` is `os.UserConfigDir()/portty/daemon.sock` and the session env carries the host's `HOME`, so the CLI reaches the *host's* daemon; viewers (local tab, peer proxy, browser) are subscribers of that hub [VERIFIED by reading socket.go and the architecture above; the VM check confirms it end to end].
**Limitations to document:** a session created before this build (including one handed over live by the Phase 9 upgrade) has no `PORTTY_SESSION_ID`; `portty open` there prints one line ("this shell started before Portty updated; open a new session") and exits non-zero. Stale env inside `tmux`/`screen` started in session A and attached from session B carries A's id (preview opens in A): document, no fix. `HOME` rewritten by a tool breaks socket discovery: optional `PORTTY_SOCKET` env is not recommended (simplicity), mention only.

### Pattern 6: Frontend tab model, with the owner view kept mounted
**Facts [VERIFIED: frontend/src/App.tsx; TerminalPanel.tsx:496; TabBar.tsx:19]:**
- `Tab.type` is `'terminal' | 'web-session'`. The only `new RelayClient` is in `TerminalPanel.tsx:496`, so local tabs, peer tabs (proxy via `remote:true`) and the browser share viewer all deliver frames through this one path.
- **Local tabs:** every local session's `TerminalPanel` is mounted all the time, shown with `display: flex|none` (App.tsx:1275-1295, `key={tab.sessionId}`), so a hidden local tab keeps its socket and receives previews.
- **Peer tabs and the browser viewer:** only one `WebShareSessionView` is mounted, for the active `__websession__…` tab (App.tsx:1178, `key={activeId:capToken}`); switching away unmounts the panel, which snapshots the terminal and closes the socket (HARD-05). A hidden peer tab therefore receives **no** previews, and focusing a preview tab unmounts the terminal and **cuts the stream in progress**.
- `TabBar` in `webMode` is a single static pill: no close button, no context menu (`{!webMode && (<button className="tab__close" …`).
**Recommendation:**
1. New `Tab` kind `type: 'preview'` with `ownerTabId` (the session tab id: `sessionId` for local, `webSessionTabId(...)` for peer/web), `previewId`, `name`, `mime`, `state`. Id format `__preview__<ownerTabId>/<previewId>` (must **not** start with `__websession__`, which the render branch matches by prefix). Exclude `preview` in the local-tabs map (`if (tab.type === 'web-session') return null` becomes web-session or preview) so no `TerminalPanel` is created for it, and give it no `sessionId` that aliases the owner (avoid `tabProgress[tab.sessionId]` showing the owner's progress underline and `removeTabState` side effects).
2. **Keep the owner view mounted while one of its previews is the active tab:** compute `sessionViewTabId = activeTab.type==='preview' ? activeTab.ownerTabId : activeId`, render `WebShareSessionView` for it (keyed by that id, not by `activeId`) in a wrapper with `display: none` while the preview is active, and render `PreviewPanel` for the active preview. Without this, peer/browser previews are cut off by their own focus switch. Risk: hidden-mounted peer panel and refit: `measureFit` returns `null` for a zero-size container, so a hidden panel never reports or claims a bogus grid (verified at TerminalPanel.tsx measureFit: `if (!(parentW > 0) || !(parentH > 0)) return null`), but test the re-show refit in the VM.
3. Frames reach App through an `onPreview` callback threaded `App -> TerminalPanel` (local) and `App -> WebShareSessionView -> TerminalPanel` (peer/web), bound to the owner tab id. Assembly and blobs live in an **App-level store** (not in the panel), so closing/unmounting a panel never loses a finished preview and never leaks a URL.
4. **Create the tab only when the end frame verifies** (received bytes == declared size, status ok); at header time show nothing or a lightweight "receiving" indicator on the owner tab. This also avoids focusing a tab whose content never completes.
5. `focus` honoured only if the header says so *and* the owner view is currently the active tab or a sibling preview (do not yank the user from an unrelated session tab; recommend: focus only when the owner session tab (or one of its previews) is active, else background). Flag to UI-SPEC.
6. Same session and same file name: replace the existing tab's content (revoke the old URL) instead of stacking; cap open previews (suggest 8 per window; opening a 9th closes the oldest non-active one) and total retained bytes (suggest 100 MiB). Closing a session tab, `removeTabState`, `session:exit`, and App unmount revoke the owner's previews' URLs (recommended: previews close with their owner; flag).
7. In web mode the bar needs preview tabs to be closable and selectable even though the session pill is static (UI-SPEC to decide visuals).
**Memory bounds:** validate `size <= PREVIEW_MAX (20 MiB)` from the header before accepting; never preallocate from the header; abort if received exceeds declared size; one in-flight assembly per RelayClient (a new header abandons the old partial); discard on socket close/reconnect; drop the chunk array once `new Blob(chunks, {type})` is built (peak about 2x size, transient). Blob `type` comes from the fixed MIME allowlist the *viewer* maps, never the header string verbatim.

### Anti-Patterns to Avoid
- **`hub.broadcast`/`BroadcastMeta` for chunks:** kicks slow viewers (see Pattern 2).
- **A path field, or `os.Open`/`os.ReadFile`/`filepath` use, anywhere in daemon or relay preview code:** breaks PREV-03; enforce with a source gate test.
- **Registering the route on `RelayHandler` or the share server:** the loopback relay is reachable by browser pages; the share server is tailnet-exposed.
- **`<a href>` navigation inside Markdown without `preventDefault`:** an unhandled click can navigate the Wails main frame away from the app [ASSUMED: Wails v2 on macOS delegates external navigations to the system browser, not verified here]; intercept every click, `auxclick` and Enter on anchors, and test `defaultPrevented`.
- **Opening an SVG blob top-level (`window.open(blobUrl)`):** a top-level SVG document runs scripts. Only ever display SVG through `<img>`. A "open PDF in browser tab" fallback is acceptable for PDF only.
- **Trusting header MIME/name for rendering or display:** viewer maps a fixed enum and shows names as React text.
- **Putting `PORTTY_SESSION_ID` lookup behind `EnsureDaemon`'s restart prompt** (Pattern 4).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Markdown parsing | A regex Markdown converter | `marked` (pinned) with a local `new Marked({...})` instance | CommonMark/GFM edge cases, ReDoS-hardened upstream |
| HTML sanitizing | Tag/attr stripping by regex or a hand-written allowlist walker | `DOMPurify` string mode with explicit `ALLOWED_TAGS` and `ALLOWED_URI_REGEXP` | mXSS and parser differentials are exactly what it models |
| Link safety | New scheme checks | `isAllowedScheme`, `getRisk(display, href)`, `LinkConfirmPopover`, `openLink` (reuse) | One audited allowlist `ALLOWED_SCHEMES = ['https:', 'http:', 'mailto:']` [VERIFIED: urlSafety.ts] |
| Content-type detection | Magic-byte tables | `http.DetectContentType` (PNG `\x89PNG\x0D\x0A\x1A\x0A`, JPEG `\xFF\xD8\xFF`, GIF87a/89a, `RIFF….WEBPVP`, `%PDF-`) [VERIFIED: stdlib net/http/sniff.go:89,118-126] | Only SVG (no sniff entry) and Markdown need custom rules |
| Session-hub fan-out | A second pub/sub | The existing `Hub` subscribers and write pumps | Auth, ordering, pongs, handoff already solved |
| Admin elevation | A privileged helper/installer pkg | `osascript` with `with administrator privileges` and `quoted form of` argv | CONTEXT decision; no helper to ship |
| Atomic symlink refresh | Remove-then-create | create temp symlink + `os.Rename` | No window with no link while sessions spawn |
| PDF rendering | pdf.js (unless the spike fails) | WKWebView/Safari native viewer | Keeps native look, no worker/wasm CSP |

**Key insight:** the dangerous parts of this phase are not exotic; they are (a) fan-out backpressure, (b) tab lifecycle for peer/browser views, (c) sanitizer configuration, and (d) CSP inheritance for PDF. Everything else reuses existing, audited paths.

## Common Pitfalls

### Pitfall 1: Viewers kicked mid-preview
**What goes wrong:** viewer socket closes `1008 too slow` during a large preview (SC3 fail).
**Why:** `sendLocked` kicks on a full 256-slot queue; chunks outrun a tailnet/DERP link, and terminal output frames share the queue.
**Avoid:** paced sender with a small window (Pattern 2); a test that streams 20 MiB into a subscriber whose consumer is throttled and asserts `CloseSlow` never fires and a ticking `Run` writer keeps producing frames.
**Warning signs:** `viewer-diag: slow session=… queued=` in `daemon.log`.

### Pitfall 2: Focus switch unmounts the stream (peer tabs, browser)
**What goes wrong:** the home peer tab receives the header, focus moves to the preview tab, the terminal view unmounts and its socket closes, the preview never completes, and the viewer count drops.
**Avoid:** create the tab at the verified end frame and keep the owner view mounted (Pattern 6). Add an App-level test: a peer tab receiving a complete preview with `focus:true` keeps one socket open.

### Pitfall 3: A joining viewer is kicked by preview chunks
**What goes wrong:** chunks go into a replaying subscriber's `pending`; over 4 MiB it is kicked.
**Avoid:** skip `replaying` subscribers and never add late joiners (Pattern 2).

### Pitfall 4: Upload holds the handoff write gate
**What goes wrong:** a 20 MiB request that lives while streaming blocks `drainWrites(1s)` and fails an upgrade handoff.
**Avoid:** buffer, respond 202, pace async (Pattern 3); test it.

### Pitfall 5: `portty open` in an old session
**What goes wrong:** no `PORTTY_SESSION_ID` (sessions from before this build, handed over by Phase 9 upgrades); the user sees a confusing error.
**Avoid:** one clear line; VM check in a session created before the build under test.

### Pitfall 6: `blob:` PDF inherits the page CSP (Safari share viewer)
**What goes wrong:** the PDF iframe/object renders empty under the share page CSP because the blob document inherits `frame-ancestors 'none'`/`default-src 'none'` (documented for WebKitGTK; unknown for Safari).
**Avoid:** spike on the real share page in Safari (Sec. 6). Fallbacks: `<object>`/`<embed>`; for browsers only an "Open PDF" link that opens the blob top-level in a new tab (works because `frame-ancestors` only matters when framed); pdf.js as last resort with user sign-off. Never loosen `frame-ancestors`.

### Pitfall 7: Markdown anchors navigate the app away
**What goes wrong:** a clicked `<a href>` loads in the Wails webview.
**Avoid:** delegated `click`/`auxclick` handler with `preventDefault` for every anchor; links go `isAllowedScheme` -> `getRisk(display, href)` -> `LinkConfirmPopover`/`openLink`; relative/`#` links inert (no file API exists to resolve them). Test every anchor variant.

### Pitfall 8: Markdown "remote images" fetched before sanitize
**What goes wrong:** building the DOM before sanitizing fetches `<img src>`.
**Avoid:** never assign unsanitized HTML to the live DOM or `innerHTML` of an attached node; DOMPurify returns a string, then assign once. In `marked`, override `image` to emit a placeholder span (alt text only) so no `<img>` is ever produced; DOMPurify allowlist excludes `img`.

### Pitfall 9: `<style>`/`style=`/SVG/MathML survive sanitization
**Avoid:** explicit `ALLOWED_TAGS` (not the default profile), `FORBID_ATTR: ['style']`, `ALLOW_DATA_ATTR: false`, no `USE_PROFILES` (it overrides allowlists and enables SVG/MathML by default [CITED: DOMPurify README]), no `ADD_TAGS`/hooks that mutate allowlists.

### Pitfall 10: jsdom has no `URL.createObjectURL`/`revokeObjectURL`
**Avoid:** stub them in the preview tests (spy to assert revoke on close, owner close, unmount); do not add a global to `test-setup.ts` unless several files need it.

### Pitfall 11: PATH entry lost to an rc file that assigns PATH
**What goes wrong:** `~/.zprofile` or `~/.zshrc` doing `PATH=/x:/y` (no `$PATH`) drops the Portty dir.
**Avoid/accept:** rare; documented limit. `path_helper` itself preserves it (Sec. 7). The menu item covers other terminals.

### Pitfall 12: osascript quoting and cancel
**Avoid:** pass the executable path as an argv item and use `quoted form of` inside the AppleScript (Sec. 7); detect cancel (exit status 1, stderr containing `-128`) [ASSUMED: confirm in the VM] and report "no change".

### Pitfall 13: Linking a DMG/translocated path
**What goes wrong:** menu install run from a mounted DMG or Gatekeeper-translocated app links to a path that disappears.
**Avoid:** refuse with "Move Portty to Applications first" when `os.Executable()` is under `/Volumes/` or contains `AppTranslocation` [ASSUMED path shapes; verify in VM].

### Pitfall 14: Test binaries and the firewall alert
Pitfall for the *executor*: each new test binary triggers a macOS firewall alert for the user. Keep new Go tests inside existing packages (relay/daemon/webserver/root), avoid new `httptest` listeners where an in-memory hub suffices, and run each full suite at most once per plan (CONTEXT).

## Code Examples

### Env + id before spawn (internal/pty/native.go)
```go
// Source: internal/pty/native.go:59-83 (current shape; move generateID above the env merge)
id, err := generateID()
if err != nil { /* cancel; close; return */ }
cmd.Env = mergeEnv(os.Environ(), req.Env, "TERM=xterm-256color", "COLORTERM=truecolor", "PORTTY_SESSION_ID="+id)
// ... cmd.Start() ... sess := &Session{ID: id, ...}
```

### PATH extension per session (internal/daemon/engine.go CreateSession)
```go
// Recommended: ensure <configDir>/bin/portty -> os.Executable() (atomic), then
env := []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}
sess, err := e.backend.Create(ctx, pty.CreateRequest{CLI: cliPath, Args: args, Env: env, Cols: cols, Rows: rows, WorkDir: workDir})
```

### Sanitizer configuration (frontend/src/lib/markdown.ts, lazy `import()`)
```ts
// Source: marked docs https://marked.js.org/using_pro (renderer methods take a token object; html/link/image are overridable)
//         DOMPurify README (ALLOWED_TAGS, ALLOWED_URI_REGEXP, FORBID_ATTR, ALLOW_DATA_ATTR)
// NOTE: exact v18 signatures must be confirmed against the pinned version in the hostile-fixture tests.
import { Marked } from 'marked'
import DOMPurify from 'dompurify'

const md = new Marked({ gfm: true, async: false, renderer: {
  html: () => '',                                   // raw HTML dropped (layer 1)
  image: ({ text }) => `<span class="md-image">${escapeHtml(text)}</span>`, // no <img> ever; escapeHtml = a 5-char entity escape (pseudo)
}})
const ALLOWED_TAGS = ['h1','h2','h3','h4','h5','h6','p','br','hr','ul','ol','li','blockquote','pre','code',
  'em','strong','del','a','table','thead','tbody','tr','th','td','span'] // no <input>: GFM task-list checkboxes are dropped (no form controls in previews)
export function renderMarkdown(src: string): string {
  const html = md.parse(src) as string
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS, ALLOWED_ATTR: ['href', 'class', 'title', 'align'],
    ALLOWED_URI_REGEXP: /^(?:https?:|mailto:)/i,    // linear-time; relative links are not allowed (inert)
    ALLOW_DATA_ATTR: false, FORBID_ATTR: ['style'],
  })
}
```

### Anchor handling (PreviewPanel)
```tsx
// Reuse: isAllowedScheme / getRisk / openLink / LinkConfirmPopover (frontend/src/lib/urlSafety.ts, openLink.ts)
function onClick(e: React.MouseEvent) {
  const a = (e.target as HTMLElement).closest('a'); if (!a) return
  e.preventDefault()
  const href = a.getAttribute('href') ?? ''
  if (!isAllowedScheme(href)) return
  const risk = getRisk(a.textContent ?? '', href)       // markdown [apple.com](https://evil.com) is the 'osc8' mismatch case
  risk ? setConfirm({ url: href, risk, x: e.clientX, y: e.clientY }) : openLink(href)
}
// also onAuxClick={onClick}; Enter on a focused anchor fires click.
```

### Install via osascript without string injection (app_cli_install.go)
```go
// argv carries the path; AppleScript quotes it. No Go string is ever spliced into shell text.
script := []string{
  "on run argv",
  `do shell script "/bin/mkdir -p /usr/local/bin && /bin/ln -s " & quoted form of item 1 of argv & " /usr/local/bin/portty" with administrator privileges`,
  "end run",
}
args := []string{"-e", script[0], "-e", script[1], "-e", script[2], exe}
cmd := exec.Command("/usr/bin/osascript", args...)   // exe = EvalSymlinks(os.Executable())
```
(`ln -s` without `-f`: if the link appears between our check and the privileged run, it fails instead of overwriting.)

### CSP delta (internal/webserver/csp_mw.go), minimal
```go
b.WriteString("img-src 'self' data: blob:; ")   // current: "img-src 'self' data:; " [VERIFIED: csp_mw.go]
b.WriteString("frame-src blob:; ")              // only if the PDF frame variant is chosen; or "object-src blob:; " for <object>/<embed>
// unchanged: default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline';
//            connect-src 'self' wss://<host>; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `rehype-sanitize` pipelines for Markdown | `marked` + `DOMPurify` for a single string | n/a | Far smaller supply chain |
| DOMPurify `IN_PLACE`/hook-heavy configs | Default string mode, allowlists | 2026 advisories (3.4.x) | Avoid the modes with advisories |
| PDF via pdf.js everywhere | OS viewer where it works; pdf.js as fallback | n/a | Native look, no worker CSP |

**Deprecated/outdated:** Go `http.DetectContentType` has no SVG signature (SVG always needs custom detection); WKWebView on **iOS** shows only the first PDF page in an iframe (irrelevant: macOS only) [ASSUMED from general knowledge, not needed].

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Wails v2 on macOS routes an unhandled external `<a>` click to the system browser instead of navigating the main frame | Anti-Patterns, Pitfall 7 | Low: we preventDefault anyway; only affects how severe a missed handler would be |
| A2 | `osascript` cancel of the admin prompt exits status 1 with `-128` ("User canceled") in stderr | Pitfall 12 | Cancel detection wrong: show generic failure instead of "no change"; VM check settles it |
| A3 | DMG launches run from `/Volumes/…` and Gatekeeper translocation runs from a path containing `AppTranslocation` | Pitfall 13 | Install link would dangle after eject; VM check |
| A4 | `marked` v18 `new Marked({renderer:{html,image}})` signatures match the docs fetched (token-object methods, `false` falls through) | Code Examples | Hostile-fixture tests fail early; adjust code, not design |
| A5 | Bundle sizes: `marked` about 40 KB min, DOMPurify about 60 KB min (lazy-load keeps the terminal bundle unchanged) | Standard Stack | Larger first-preview latency only; measure after install |
| A6 | A `blob:` PDF in an iframe/embed/object renders in the Wails v2 WKWebView (no CSP on the desktop page) | Pattern PDF, Sec. 5 | Spike decides; fallbacks listed. **Needs the spike, not user confirmation** |
| A7 | A `blob:` PDF frame renders in Safari under the share CSP with `frame-ancestors 'none'` inherited | Sec. 6 | Spike decides; fallback is an "Open PDF" link or pdf.js (user sign-off) |
| A8 | Wails v2 sets no CSP on its own page (only the repo's `index.html` was grepped; none found) | Sec. 6 | If Wails injects one, the desktop might need `blob:` too; spike reveals CSP violations |
| A9 | Preview tabs should close with their owner session tab, replace same-name previews, and cap at 8 open / 100 MiB retained | Pattern 6 | UX, not security: planner/UI-SPEC may choose otherwise |
| A10 | No-owner => nobody focuses (all background) | Pattern 2 | A viewer who never typed since a handoff gets a background tab: acceptable, confirm |

## Open Questions

1. **SC3 wording vs CONTEXT.** ROADMAP SC3 says an over-cap file "shows a clear 'too large to preview' message, never a partial render"; CONTEXT says the CLI prints a one-line error and viewers get nothing (a placeholder only if the cap is crossed mid-stream).
   - Known: the CLI stats first and refuses before contacting the daemon; the daemon also enforces the cap while reading the body.
   - Unclear: whether "shows a message" requires a viewer-side tile for the ordinary over-cap case.
   - Recommendation: keep CONTEXT (the typist sees the CLI line in the terminal they are watching); implement the viewer tile for the daemon-side cap trip (testable in Go) and confirm with the user at plan check.
2. **Stale own link in the install menu.** SC5 says an existing link to something else is reported, not overwritten. A link to a `*/Portty.app/Contents/MacOS/Portty` at another path (app moved) is "ours but stale".
   - Recommendation: follow SC5 literally (report, show the target, tell the user to `rm` it) and note it; offering replace is a small later change if the user wants it.
3. **Hidden-mounted peer view refit** (Pattern 6 item 2): needs a VM check, not a decision.
4. **Where the PATH symlink dir lives:** `<UserConfigDir>/portty/bin` (has a space in "Application Support"; fine for zsh) vs the bundle dir `…/Contents/MacOS` (no space, but relies on case-insensitive APFS to turn `portty` into `Portty` and breaks tab completion). Recommendation: the symlink dir (Sec. 7).
5. **Previews of a session whose owner ends:** recommended to close with the owner; confirm with UI-SPEC.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go (pinned toolchain) | build/tests | yes | `export GOTOOLCHAIN=go1.26.11` (go.mod: `go 1.26.4`) | — |
| Wails CLI | real builds for spike/VM | yes | v2.16.0 CLI (library `github.com/wailsapp/wails/v2 v2.10.2` in go.mod) | — |
| pnpm / node | frontend | yes | pnpm 12.6.0, node v24.22.1 | — |
| Tart + two VMs | VM checks, spike, admin-prompt check | yes | `portty-test`, `portty-test-2` both `running`; macOS 27.2 arm64 per 10-VM-CHECKS.md | — |
| `/usr/libexec/path_helper`, `/bin/zsh`, `osascript` | PREV-05 | yes (host and VMs) | host macOS 27.2.1 | — |
| Safari + safaridriver (home VM) | share-page PDF/CSP spike and preview check | assumed yes (used in Phase 10 R-01b) | — | Playwright WebKit for a non-VM smoke only |
| `gh` | advisories lookup | yes | — | — |

**Missing dependencies with no fallback:** none. **Never touch the host's `/usr/local/bin` or the user's Portty** (CONTEXT).

## Validation Architecture

> `workflow.nyquist_validation` is `false` in `.planning/config.json`, so GSD would normally skip this section; it is included because the orchestrator asked for it explicitly.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (race, `-short`), vitest 4.1 + jsdom 29, Playwright 1.59 (e2e), bash gates |
| Config file | `frontend/vite.config.ts` (vitest block, `test-setup.ts`); TESTING.md Section 2 |
| Quick run command | `GOTOOLCHAIN=go1.26.11 go test -race -short ./internal/relay/ -run Preview` (targeted, one package) and `cd frontend && pnpm vitest run <file>` |
| Full suite command | `GOTOOLCHAIN=go1.26.11 go test -race -short ./...` and `cd frontend && pnpm vitest run` + `tsc`, `bash tests/check-traceability-paths.sh`, check-brand (each at most once per plan, outputs to files) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PREV-01 | Wire frames round-trip; header/chunk/end parse; focus only for owner; replaying/late joiner skipped | unit | `go test -race -short ./internal/relay -run Preview` | ❌ Wave 0 `internal/relay/preview_test.go` |
| PREV-01 | Daemon route: 202, 404 unknown session, 409 ended, 413 cap, 429 busy, bad mime/name | unit | `go test -race -short ./internal/daemon -run Preview` | ❌ `internal/daemon/preview_test.go` (hub via `Manager().Create`, no PTY) |
| PREV-01 | `PORTTY_SESSION_ID` in spawn env, differs per session, overrides inherited | unit | `go test -race -short ./internal/pty -run Env` and an engine test with `SetSpawnCommandForTest("/usr/bin/env")` | ❌ extend `internal/pty/native_test.go` / new `engine_env_test.go` |
| PREV-01 | CLI: missing file, directory, unsupported type, oversize, no env var, old daemon API, success | unit | `go test -race -short . -run Open` | ❌ `cmd_open_test.go` (fake unix-socket daemon) |
| PREV-01 | Frontend: parse 0x21; assembler (size mismatch, overflow, new header abandons, close discards); tab created beside owner; focus only with flag; closing leaves terminal | unit | `pnpm vitest run src/lib/previewAssembler.test.ts src/lib/relayClient.test.ts src/components/__tests__/App.preview.test.tsx` | ❌ Wave 0 |
| PREV-01 | Peer/browser owner view stays mounted while its preview is active (one socket) | component | `pnpm vitest run src/components/__tests__/App.preview.test.tsx` | ❌ |
| PREV-02 | Markdown hostile fixtures inert; links gated; images placeholder; revoke on close/unmount | unit | `pnpm vitest run src/lib/markdown.test.ts src/components/__tests__/PreviewPanel.test.tsx` | ❌ |
| PREV-02 | SVG/Markdown fetch nothing, run nothing (real engine) | e2e / VM | Playwright route-interception spec; VM beacon listener | ❌ `frontend/e2e/preview.spec.ts` (optional) + VM M-item |
| PREV-02 | PDF renders in Wails WKWebView | manual/VM (spike) | VM build + screenshot | ❌ spike evidence file |
| PREV-03 | Near-cap streaming: no `CloseSlow`, terminal ticks continue, queue occupancy bounded | unit | `go test -race -short ./internal/relay -run PreviewNearCap` (pipe-backed hub, throttled subscriber) | ❌ |
| PREV-03 | Handoff drain not blocked by a streaming preview | unit | `go test -race -short ./internal/daemon -run PreviewHandoff` | ❌ |
| PREV-03 | No route reads a path: route absent on `RelayHandler` and the share server; daemon preview source never imports/uses `os.Open`, `os.ReadFile`, `filepath` | unit | extend `files_removed_test.go` pattern in daemon and webserver; source-gate test (`go/parser` imports of `preview*.go`) | ❌ |
| PREV-04 | CSP contains `img-src 'self' data: blob:` (+ frame/object src per spike), `frame-ancestors 'none'`, no `blob:` in script/connect/default | unit | `go test -race -short ./internal/webserver -run CSPHeaders` | ✅ extend `csp_mw_test.go` (substring `img-src 'self' data:` still matches) |
| PREV-04 | Safari share viewer previews image/PDF/Markdown | manual/VM | M-item, safaridriver on home VM | ❌ |
| PREV-05 | Session PATH contains the bin dir; symlink refresh atomic and correct after exe change | unit | `go test -race -short ./internal/daemon -run CLILink` | ❌ `internal/daemon/clilink_test.go` |
| PREV-05 | Install: link states (absent, ours, foreign symlink, regular file, DMG/translocated path), cancel, error, success; menu item present and wired | unit | `go test -race -short . -run 'CLIInstall|AppMenu'` | ❌ `app_cli_install_test.go`; ✅ extend `menu_test.go` |
| PREV-05 | Real zsh login PATH keeps the entry; admin prompt; Terminal.app finds nothing before install | manual/VM | M-items | ❌ |

### Sampling Rate
- **Per task commit:** the targeted package/file command above.
- **Per wave merge:** the affected package(s) plus `pnpm vitest run` for touched frontend dirs.
- **Phase gate:** full Go suite once, vitest once, `tsc`, check-brand, check-traceability, `pnpm install --frozen-lockfile`; VM checks recorded in `11-VM-CHECKS.md` (Phase 10 format); TESTING.md updated.

### Wave 0 Gaps
- [ ] `internal/relay/preview_test.go`, `internal/daemon/preview_test.go`, `internal/daemon/clilink_test.go`, `cmd_open_test.go`, `app_cli_install_test.go`
- [ ] `frontend/src/lib/previewAssembler.test.ts`, `markdown.test.ts`, `components/__tests__/PreviewPanel.test.tsx`, `App.preview.test.tsx`
- [ ] Hostile fixture set under `frontend/src/lib/__fixtures__/preview/` and `internal/.../testdata/` (see list below)
- [ ] VM check script set in the style of `~/p10` (office/home sessions, beacon listener, Safari driver)
- [ ] Framework install: only `marked` and `dompurify` (frontend)

**Hostile fixture list (Markdown):** `<script>`, `<img src=x onerror=…>`, `[x](javascript:alert(1))` and entity/whitespace/tab/newline obfuscations, `[a]: javascript:…` reference links, `<javascript:…>` autolink, `data:text/html` links, `![](https://127.0.0.1:PORT/b.png)`, `<iframe>`, `<object>`, `<embed>`, `<svg onload>`, `<math>` mXSS strings (`<noscript><p title="</noscript><img src=x onerror=…>">`), `<style>@import url(…)`, `<form action=…>`, `<meta http-equiv=refresh>`, `<base href>`, `<link rel=stylesheet>`, very deep nesting and a very long line (DoS: assert parse time/size bounds). **SVG:** `<script>`, `onload`, `<image href=http://127.0.0.1:PORT/…>`, `<use href=…>`, `<foreignObject><iframe>`, `<style>@import`, a billion-laughs-style `<!ENTITY>` (loads as image: browser handles; assert tab survives). **Proof of "fetches nothing":** the VM runs a loopback `python3 -m http.server`-style beacon (or `nc -l`) and asserts zero requests after rendering each fixture.

**Suggested manual items (TESTING.md Category BA, M-121+):** office-to-home previews of each type; home Safari share viewer preview (incl. PDF under the real CSP); hostile fixtures + beacon in a real build; near-cap streaming with a ticking terminal and two viewers; no-owner and owner-focus behaviour; upgrade-era session without the env var; old-daemon "update needed"; PREV-05 PATH in a fresh login session and `portty list` in Terminal.app after install; cancel; foreign link; DMG path refusal.

## Security Domain

### Applicable ASVS Categories (ASVS level 1 per config)

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no new | existing share token gate unchanged |
| V3 Session Management | no new | existing capability grants unchanged |
| V4 Access Control | yes | route only on the owner-only unix socket mux; absent from `RelayHandler` and share server; no path parameter anywhere |
| V5 Input Validation | yes | CLI sniff + cap, daemon `MaxBytesReader` + MIME enum + basename; viewer length/id checks; DOMPurify allowlist + URI regexp; React text for names |
| V6 Cryptography | no | none added |
| V12 Files/Resources | yes | no file read by path on daemon/share server; only the user's own shell process opens files; blob-only rendering; no storage of remote content |
| V14 Config | yes | CSP delta minimal and reviewed; `frame-ancestors 'none'`, token gate, tailnet listener unchanged |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Hostile Markdown/SVG from a compromised or malicious host session XSSes the viewer | Tampering / Elevation | `<img>` for SVG; marked `html`+`image` overrides then DOMPurify explicit allowlist; no raw HTML; share-page CSP `script-src 'self'` as a second layer; desktop page has no CSP, so the sanitizer is the only layer there (consider an optional `<meta>` CSP on the preview container; not recommended unless the review asks) |
| Remote content fetch / tracking beacon via image, CSS, link prefetch | Information disclosure | image placeholder, `img-src` has no network origin, `style` forbidden, no `<link>`/`<meta>`/`<base>` |
| Memory/CPU exhaustion by a huge or endless preview | DoS | 20 MiB cap in CLI, daemon, and viewer (validate header size, abort on overflow), one in-flight per hub and per RelayClient, cap on open previews and bytes, paced send so no viewer is kicked |
| Another local user or web page injects previews | Spoofing / Tampering | unix socket dir `0700`; route not on the loopback TCP relay (browser-reachable); client-to-server `0x21` frames dropped |
| Viewer attacked by a hostile *host* (any share host is trusted only as far as its terminal output) | Tampering | same sanitizer/blob/enum rules; never `window.open` an SVG blob; PDF parsed by the OS viewer (accepted residual risk) |
| Admin prompt command injection (PREV-05) | Elevation | `osascript` argv + `quoted form of`, absolute binaries (`/bin/mkdir`, `/bin/ln`), `ln -s` without `-f`, link inspected before prompting, foreign link never overwritten |
| Symlink in `<cfg>/bin` replaced to point elsewhere | Tampering | dir is inside the `0700` config dir; refresh rewrites it from `os.Executable()` on every session spawn |
| CSP loosening drift | Tampering | tests pin `frame-ancestors 'none'`, absence of `blob:` in script/connect/default, no wildcards; security review recorded (`11-SECURITY.md`) naming `marked` and `dompurify` |

## PREV-05 detail (Sec. 7 evidence)

- **path_helper verified by running it** (macOS 27.2.1): `PATH="/opt/portty-test/bin:/usr/bin:/bin:/sbin" /usr/libexec/path_helper -s` printed `PATH="/usr/local/bin:/System/Cryptexes/App/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:…:/Applications/Wireshark.app/Contents/MacOS:/Applications/nmap.app/Contents/Resources/bin:…:/opt/portty-test/bin"`; the pre-existing entry survived at the **end**. A real login shell with an empty `ZDOTDIR` and `PATH="/opt/portty-test/bin:/usr/bin:/bin"` ended with `…:/opt/portty-test/bin` last [VERIFIED: ran both]. `/etc/zprofile` runs `eval \`/usr/libexec/path_helper -s\`` when executable [VERIFIED: /etc/zprofile]. So a prepended directory survives but is searched after system dirs (no system `portty` exists, so no shadowing).
- **Binary and name:** the bundle binary is `Contents/MacOS/Portty` (capital P; wails.json `"outputfilename": "Portty"` [VERIFIED: wails.json]); README currently tells users `sudo ln -sf /Applications/Portty.app/Contents/MacOS/Portty /usr/local/bin/portty` [VERIFIED: README.md:16], which the menu item replaces (update README and the website step 2 copy). Putting `…/Contents/MacOS` itself on PATH would work for `portty` only on case-insensitive APFS and would list `Portty` for completion; use the per-user symlink dir instead: `<UserConfigDir>/portty/bin/portty -> filepath.EvalSymlinks(os.Executable())`, refreshed atomically (temp symlink + `os.Rename`) on every `CreateSession` (cheap, self-healing after app moves/updates and dev builds), placed first on the session `PATH` via `CreateRequest.Env`.
- **Menu item:** `appMenu()` is in `main.go` (not `BuildMenuItems`, which is the tray menu in `tray_common.go`). Add `appSub.AddText("Install Command-Line Tool…", nil, installCLICallback)` after Settings… (main.go:170), keeping `TestAppMenu_Structure` invariants (About, separator, Settings…; six top-level items). Mirror `checkForUpdatesCallback`: run in a goroutine (native alerts block), use the existing `a.alert`/`a.dialog` seams (`messageDialogFunc`) [VERIFIED: app_update.go:209-222, app.go:77-78].
- **Logic order (all unprivileged first):** `Lstat(/usr/local/bin/portty)`; absent -> prompt; symlink resolving to the same `EvalSymlinks(exe)` -> "already installed"; symlink elsewhere -> alert with `Readlink` target, no prompt; regular file/dir -> alert, no prompt; refuse DMG/translocated exe paths; then `osascript` (see Code Examples); exit status 1 + `-128` -> silent "no change"; other failure -> alert with the error; success -> alert with the link path.
- **Testing without touching the host:** inject `linkPath` (default `/usr/local/bin/portty`) and a `runPrivileged func(exe, link string) error` seam; unit tests use a temp dir and a fake runner that does the unprivileged equivalent. The real `osascript` admin prompt runs only in a VM (never the host).

## Suggested plan split, waves and file-overlap constraints

Spike first (SC2: "proven inside the Wails v2 web view before anything is built on it") means nothing may depend on the PDF *renderer*; the Go substrate does not, so it runs in parallel with the spike.

| Wave | Plan | Scope | Files (ownership) | Depends on |
|------|------|-------|-------------------|-----------|
| 1 | 11-01 Spike (VM-only) | PDF in the real Wails build (iframe/embed/object variants, `securitypolicyviolation` log) **and** on the real share page in Safari under the real CSP; record macOS version; decision tree below | `frontend/src/spike/*` (throwaway, deleted by 11-06), `.planning/phases/11-file-preview/11-SPIKE.md` | — |
| 1 | 11-02 Relay wire + hub | `MsgPreview`, frame build/parse, `Hub.SendPreview` (paced, owner flag, skip replaying), near-cap test | `internal/relay/protocol.go`, `preview.go`, `hub.go` (small accessor), `preview_test.go` | — |
| 2 | 11-03 Daemon, session env, API, PATH dir | `PORTTY_SESSION_ID`, `<cfg>/bin/portty` symlink + PATH env, `POST /sessions/{id}/preview`, `APIVersion` 3, client method, route-absence tests | `internal/pty/native.go`, `internal/daemon/{api,preview,client,types,engine,clilink}.go` + tests | 11-02 |
| 2 | 11-05 Frontend plumbing + tab model | `relayClient.ts` frame, assembler, store, `Tab` type, `App.tsx` keep-owner-mounted, `TabBar` (web-mode closable previews), callbacks through `TerminalPanel`/`WebShareSessionView`; renders a stub `PreviewPanel` | `frontend/src/lib/{relayClient,tabs,previewAssembler,previewStore}.ts`, `App.tsx`, `TabBar.tsx`, `TerminalPanel.tsx`, `WebShareSessionView.tsx`, `style.css` (tab bits only) + tests | wire spec only (parallel with 11-03) |
| 3 | 11-04 CLI `portty open` | `cmd_open.go`, `main.go` runCLI case, `cmd_cli.go` usage + `cliNeedsDaemon`, real-launch note | `cmd_open.go`, `cmd_cli.go`, `main.go` (runCLI only), `dispatch_test.go`, `cmd_open_test.go` | 11-03 |
| 3 | 11-06 Renderers | `PreviewPanel` (images/SVG, Markdown, PDF per spike), `markdown.ts`, deps, styling in light/dark per UI-SPEC | `frontend/package.json`, `pnpm-lock.yaml`, `frontend/src/lib/markdown.ts`, `components/PreviewPanel.tsx`, `style.css` | 11-01, 11-05 (does **not** touch `App.tsx`) |
| 3 | 11-07 CSP | minimal delta + tests | `internal/webserver/csp_mw.go`, `csp_mw_test.go`, `csp_integration_test.go` | 11-01 (frame vs object) |
| 4 | 11-08 PREV-05 menu + install | `appMenu` item, `app_cli_install.go`, seams, README | `main.go` (`appMenu` only), `app_cli_install*.go`, `menu_test.go`, `README.md` | 11-04 (serialize on `main.go`) |
| 5 | 11-09 Gate + evidence | TESTING.md (counts, Section 4 rows, Category BA), full gates once, VM checks (`11-VM-CHECKS.md`), security review (`11-SECURITY.md`) | `TESTING.md`, `.planning/phases/11-file-preview/*` | all |

**Overlap rules:** `main.go` is touched by 11-04 (`runCLI`) and 11-08 (`appMenu`): serialize (different functions, but same file and `TestAppMenu_*`/dispatch tests). `internal/daemon/engine.go` only in 11-03 (do the session id and PATH env in one plan; do not split across waves). `App.tsx`, `TabBar.tsx`, `TerminalPanel.tsx`, `WebShareSessionView.tsx` only in 11-05. `style.css`: 11-05 (tab styles) before 11-06 (preview styles), never concurrently. `package.json`/`pnpm-lock.yaml` only in 11-06. Worktrees start at an old commit: fast-forward to `gsd/v1.2` first (CONTEXT).

**PDF spike decision tree (11-01):** build the real Wails app (`wails build -s -platform darwin/arm64 -tags wailsassets`, ad-hoc signed) into the office VM and test a fixture PDF fetched into a `Blob` with `type: 'application/pdf'` in three variants: `<iframe src=blob:>`, `<object data=blob: type=application/pdf>`, `<embed type=application/pdf src=blob:>`; log `securitypolicyviolation` and `iframe.onload`; screenshot. Repeat on the real share page in Safari (home VM) under the real CSP with the candidate directive (`frame-src blob:` or `object-src blob:`). Outcomes: (a) Wails passes and Safari passes: ship that variant. (b) Wails passes, Safari fails because of inherited `frame-ancestors`: browser viewers get an "Open PDF" button that opens the blob top-level (never loosen `frame-ancestors`); record in the security review. (c) Wails fails: stop and report with options (try the other variants first; pdf.js needs user sign-off, a bigger CSP delta and a worker; a Go-side "open in Preview" would write remote content to disk, which violates CONTEXT). Also record whether pdf pages scroll, text selection and zoom work, and dark-mode appearance (the PDF viewer chrome will not follow the app theme).

## Sources

### Primary (HIGH confidence)
- This repo, read this session: `internal/pty/native.go`, `internal/daemon/{engine,api,client,types,path,socket,remote_ws_proxy,handoff}.go`, `internal/relay/{protocol,hub,server,pong,scrollback}.go`, `internal/webserver/{server,csp_mw}.go` and tests, `internal/attach/{attach,loop}.go`, `cmd_cli.go`, `main.go`, `menu_test.go`, `app_update.go`, `frontend/src/{App.tsx,lib/relayClient.ts,lib/tabs.ts,lib/urlSafety.ts,lib/openLink.ts,components/TabBar.tsx,components/TerminalPanel.tsx,components/WebShareSessionView.tsx}`, `TESTING.md`, `.planning/phases/10-*/10-0*-SUMMARY.md`, `10-VM-CHECKS.md`
- Ran: `/usr/libexec/path_helper -s`, `zsh -l` with empty `ZDOTDIR`, `stat` of the daemon socket dir, `tart list`, `npm view`, `gsd-tools package-legitimacy check`, `gh api …/security-advisories` for `cure53/DOMPurify` and `markedjs/marked`
- Go stdlib `net/http/sniff.go` in the pinned toolchain (lines 89, 118-126)

### Secondary (MEDIUM confidence)
- marked docs https://marked.js.org/using_pro (renderer signatures, no built-in sanitization guarantee)
- DOMPurify README (allowlists, `ALLOWED_URI_REGEXP`, profiles, hooks) via GitHub
- WebKit bug 290935 (https://www2.webkit.org/show_bug.cgi?id=290935): `blob:` PDF `<object>` inherits the page CSP incl. `frame-ancestors` (WebKitGTK; macOS uses Apple's implementation)

### Tertiary (LOW confidence)
- WebKit bug 197748 (https://bugs.webkit.org/show_bug.cgi?id=197748): a 2019 "PDF not rendering in WKWebView on macOS" report, closed INVALID after discussion that moved to printing; says nothing about iframe/embed/object. Web search found no Wails v2 macOS PDF evidence either way, which is why the spike is mandatory.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH for choosing marked + DOMPurify and the versions (registry, advisories); the `SUS too-new` flag is procedural.
- Architecture (relay, daemon, tab model): HIGH, all anchored in files read this session; the paced sender and keep-mounted design are recommendations, not yet prototyped.
- PDF rendering (Wails and Safari-under-CSP): LOW until the spike; fallbacks defined.
- PREV-05: HIGH for PATH behaviour (ran it); MEDIUM for osascript cancel and DMG path shapes (to confirm in the VM).
- Pitfalls: HIGH for the repo-specific ones.

**Research date:** 2026-11-21
**Valid until:** 2026-12-05 for package pins (DOMPurify and marked release weekly: re-run `npm view` and the legitimacy check at install); 30 days for the repo analysis.
