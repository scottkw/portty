---
phase: 11-file-preview
verified: 2026-11-21T19:23:13Z
status: passed
score: 9/9 must-haves verified
covered_files: [".planning/phases/11-file-preview/11-01-PLAN.md",".planning/phases/11-file-preview/11-01-SUMMARY.md",".planning/phases/11-file-preview/11-02-PLAN.md",".planning/phases/11-file-preview/11-02-SUMMARY.md",".planning/phases/11-file-preview/11-03-PLAN.md",".planning/phases/11-file-preview/11-03-SUMMARY.md",".planning/phases/11-file-preview/11-04-PLAN.md",".planning/phases/11-file-preview/11-04-SUMMARY.md",".planning/phases/11-file-preview/11-05-PLAN.md",".planning/phases/11-file-preview/11-05-SUMMARY.md",".planning/phases/11-file-preview/11-06-PLAN.md",".planning/phases/11-file-preview/11-06-SUMMARY.md",".planning/phases/11-file-preview/11-07-PLAN.md",".planning/phases/11-file-preview/11-07-SUMMARY.md",".planning/phases/11-file-preview/11-08-PLAN.md",".planning/phases/11-file-preview/11-08-SUMMARY.md",".planning/phases/11-file-preview/11-09-PLAN.md",".planning/phases/11-file-preview/11-09-SUMMARY.md",".planning/phases/11-file-preview/11-10-PLAN.md",".planning/phases/11-file-preview/11-10-SUMMARY.md","README.md","TESTING.md","app.go","app_cli_install.go","app_cli_install_test.go","cmd_cli.go","cmd_cli_test.go","cmd_open.go","cmd_open_test.go","dispatch_test.go","frontend/index.html","frontend/package.json","frontend/src/App.tsx","frontend/src/components/ImagePreview.tsx","frontend/src/components/MarkdownPreview.tsx","frontend/src/components/PdfPreview.tsx","frontend/src/components/PreviewPanel.tsx","frontend/src/components/TabBar.tsx","frontend/src/components/TerminalPanel.tsx","frontend/src/components/WebShareSessionView.tsx","frontend/src/components/__tests__/App.nav.test.tsx","frontend/src/components/__tests__/App.open-remote.test.tsx","frontend/src/components/__tests__/App.preview.test.tsx","frontend/src/components/__tests__/MarkdownPreview.test.tsx","frontend/src/components/__tests__/PreviewPanel.test.tsx","frontend/src/components/__tests__/PreviewRenderers.test.tsx","frontend/src/components/__tests__/TabBar.test.tsx","frontend/src/components/__tests__/TerminalPanel.preview.test.tsx","frontend/src/components/__tests__/WebShareSessionView.test.tsx","frontend/src/lib/__tests__/fixtures/preview-frames.json","frontend/src/lib/__tests__/fixtures/preview/hostile-external.svg","frontend/src/lib/__tests__/fixtures/preview/hostile-script.svg","frontend/src/lib/__tests__/fixtures/preview/hostile.md","frontend/src/lib/__tests__/markdown.test.ts","frontend/src/lib/__tests__/parseMarkdownInWorker.test.ts","frontend/src/lib/__tests__/previewAssembler.test.ts","frontend/src/lib/__tests__/previewStore.test.ts","frontend/src/lib/markdown.ts","frontend/src/lib/markdownSanitize.ts","frontend/src/lib/markdownWorker.ts","frontend/src/lib/parseMarkdownInWorker.ts","frontend/src/lib/previewAssembler.ts","frontend/src/lib/previewStore.ts","frontend/src/lib/relayClient.test.ts","frontend/src/lib/relayClient.ts","frontend/src/style.css","internal/daemon/api.go","internal/daemon/api_test.go","internal/daemon/client.go","internal/daemon/client_test.go","internal/daemon/clilink.go","internal/daemon/clilink_test.go","internal/daemon/engine.go","internal/daemon/preview.go","internal/daemon/preview_test.go","internal/daemon/types.go","internal/pty/native.go","internal/pty/native_test.go","internal/relay/hub.go","internal/relay/preview.go","internal/relay/preview_test.go","internal/relay/protocol.go","internal/webserver/csp_integration_test.go","internal/webserver/csp_mw.go","internal/webserver/csp_mw_test.go","internal/webserver/files_removed_test.go","internal/webserver/input_queue_test.go","main.go","menu_test.go","testmain_test.go"]
covered_digest: "v4:sha256:a982d29c2bb2a1e19d1987da7deb253f27de69ae3f81cf9a132dbebc4af1ae93"
behavior_unverified: 0
overrides_applied: 0
coincidental_reliance_items:
  - truth: "A link inside an untrusted PDF can't load a page into the desktop preview iframe (11-REVIEW CR-01)"
    reason: undeclared-precondition
    harden: "RF-01 saw no navigation and 0 beacon hits at ea8bb0e2, but the click did nothing before the fix as well (no pre-fix baseline), so the outcome may rest on WKWebView's PDF plugin not following /URI links inside an iframe rather than on the frame-src blob: meta. Only the meta's text is pinned (PreviewRenderers.test.tsx). To prove the CSP itself, frame a non-PDF blob that links to http: in the VM and check that frame-src blocks the navigation."
human_verification:
  - test: "Decide on P-07: GET /app/sessions/<id>/preview on the share server answers 200 with the SPA shell (620 bytes, byte-identical to GET /app/, no file bytes). The 11-10 plan's must-have says 'never 2xx'."
    expected: "Either accept it (add an override for the 11-10 'never 2xx' wording: it's the /app/* SPA fallback, not a preview route, as 11-04 Deviation 1 and the security review's residual risks record) or ask for /app/sessions/* to answer 404."
    why_human: "ROADMAP SC3 is met: no route reads a file by path. Only the plan's literal wording is missed, and 11-10-SUMMARY flagged it for the user, but nobody has accepted it yet."
  - test: "Accept the macOS 27.2 administrator prompt copy: 'Allow administrator access for a script started by “Portty”?' plus Apple's malware notice. macOS no longer shows the app's 'with prompt' line."
    expected: "Accepted as Apple's copy. The IN-04 confirm before the prompt ('macOS will ask for your password to put the portty command in /usr/local/bin.') explains what the password is for (RF-04 PASS)."
    why_human: "This is OS copy that Portty can't change. Whether the confirm dialog is enough to explain it is a product call."
  - test: "Look at the desktop app's preview panes (image with checkerboard, PDF, Markdown, the Receiving/Too Large/Couldn't panes) in light and dark mode over Screen Sharing on a VM."
    expected: "Each pane looks macOS-native in both modes and repaints live when the appearance changes."
    why_human: "screencapture over SSH draws no web content in the VMs and AX can't read colours, so the desktop look was never seen. Dark-mode colours were checked only in Safari (P-04 computed styles). The desktop rows proved the content and the no-reload switch, not the look."
  - test: "Decide whether the WR-03 Markdown output caps are acceptable: 2,000,000 HTML characters or 50,000 elements. A legitimate file over either cap shows Couldn't Show File."
    expected: "Accept the caps, or set different values."
    why_human: "11-REVIEW-FIX marks the caps as the fixer's judgement, never measured in WebKit. RF-02 covered only a normal file and the slow-parse file, not a large legitimate one."
  - test: "The website's install step 2 (PREV-05's last sentence). /Users/ken/dev/portty-website/public/index.html:375 still shows the old `sudo mkdir -p /usr/local/bin && sudo ln -sf …` command. The app README is updated."
    expected: "The website's step 2 gets the 11-08-SUMMARY 'Website step 2' copy, timed with the v1.2 release (v1.1 users have no menu item), and is tracked in the website repo."
    why_human: "The website is a separate repo, and 11-08 deliberately handed this over through its SUMMARY. ROADMAP SC5 doesn't include it, and no later phase in this milestone covers it."
---

# Phase 11: File Preview Verification Report

**Phase Goal:** A user in any Portty session, local or remote, can run `portty open <file>` and see an image, PDF or Markdown file in a preview tab beside the terminal, in the app and in a browser share viewer. The bytes never leave the session's authenticated stream.
**Verified:** 2026-11-21T19:23:13Z
**Status:** human_needed
**Re-verification:** No. This is the first verification. The VM evidence comes from two runs: the main rows at product code `7042f5ce`, and RF-01..RF-04 at `ea8bb0e2`.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 / PREV-01: `portty open` in an office session opens a preview tab beside that session's tab in every app viewer (the office local tab and the home peer tab over the tailnet). Closing it leaves the terminal and session as they were. A missing or unsupported file gets a one-line CLI error. | ✓ VERIFIED | **Code:** `cmd_open.go` (args, then session id, then stat, cap, sniff, then `HealthInfo` API gate, then `SendPreview`). `PORTTY_SESSION_ID` is set before spawn (`internal/pty/native.go`). `POST /sessions/{id}/preview` is registered on the socket mux only (`internal/daemon/api.go:202`). `Hub.SendPreview` targets every non-replaying subscriber (`internal/relay/preview.go`). The frontend path is relayClient `MSG_PREVIEW`, then the TerminalPanel/WebShareSessionView assembler, then `App.handlePreview`, then `insertPreviewTab` (beside the owner). On close, `previewStore.close` releases the blob and focus returns to the owner's terminal. **VM:** P-01 (a–d) at 7042f5ce: both viewers got the preview, the × left shell 34184 and the grid untouched, and the missing and notes.txt cases printed the exact one-liners. P-08a, U-01, U-02 and RF-01..03 (at ea8bb0e2) also open previews from real sessions. **Tests:** `TestCmdOpen_EndToEnd`/`_Lines`, `TestPreview_EndToEnd`, `App.preview.test.tsx`. |
| 2 | Focus rule (CONTEXT, locked by the user): only the viewer that typed last switches to the new tab, and only while it shows that session or one of its previews. Every other viewer gets a background tab. | ✓ VERIFIED | `Focus: sub == owner` (the grid owner from Phase 10 typing) in `SendPreview`. `App.tsx` switches only on `ev.focus && (cur === ownerTabId \|\| curOwner === ownerTabId)`. **Tests:** `TestHubSendPreview_TwoViewersOwnerFocus`, `_NoOwnerNobodyFocuses`, App.preview focus cases (in the full vitest run, 1514/1517 with the 3 unrelated failures fixed and re-run 44/44). **VM:** P-01a/b (the typist's GUI switched, the other stayed in the background) and P-04 (Safari typist switched, desktops in the background). |
| 3 | SC2 / PREV-02: PNG, JPEG, GIF, WebP, SVG, PDF and Markdown render with the native look, following light and dark mode live. PDF was proven in the Wails v2 web view first. Hostile SVG and Markdown fixtures run nothing and fetch nothing. | ✓ VERIFIED (look: human item 3) | **Code:** `ImagePreview` renders a single `<img src=blob:>` (SVG too). `PdfPreview` uses an iframe on the desktop and an Open PDF button on the web. Markdown is marked (pinned 18.1.3) in a worker with a 3 s kill, then DOMPurify (pinned 3.4.17) on the main thread with explicit tag and attribute allowlists, `https?:\|mailto:\|#` URIs only, no `img` (placeholders instead), no `style`. It fails closed if DOMPurify can't run (WR-02). **Spike first:** 11-01 / 11-SPIKE.md verdict (b) before any renderer. **VM:** P-02 at 7042f5ce found all 7 types in both GUIs, in light and after a live dark switch, with no log line and no reconnect. P-03 (hostile.md, hostile-script.svg, hostile-external-nodtd.svg in 3 viewers) logged 0 beacon connections, 0 `on*` attributes, no new window and 0 Safari violations. The billion-laughs SVG is refused at the CLI (P-03, RF-03). At ea8bb0e2: RF-02 (desktop .md and .pdf, Safari .md and Open PDF, 0 violations) and RF-03 (Illustrator SVG renders). **Tests:** `PreviewRenderers.test.tsx` (hostile SVGs give one blob img), `markdown.test.ts` (hostile fixture inert), `MarkdownPreview.test.tsx` (never fetches; hostile pane has no fetching/scripting/styled element). |
| 4 | SC3 / PREV-03: an over-cap file shows a clear "too large" message, never a partial render. A near-cap stream keeps the terminal updating and drops no viewer. Neither the daemon nor the share server has a file-by-path route. | ✓ VERIFIED (P-07 wording: human item 1) | **Code:** `PreviewMaxBytes = 20_000_000` is enforced in the CLI (stat, then `LimitReader`+1), the daemon (`ContentLength` and `MaxBytesReader`), `validate()`, and the frontend assembler (`tooLarge`). There is a paced per-viewer sender (`PreviewWindow=16` of 256, never `sendLocked`, a stall abort that never kicks). `preview.go`/`relay/preview.go` import no `os`/`io/fs`/`path/filepath` (`TestPreview_SourceGate`). There is no relay or share route (`TestPreview_NotOnRelayHandler`, `TestPreview_NotOnShareServer`). **VM:** P-06: a 19.9 MB file reached 4 viewers (168 ms to 19.4 s), ticks were gapless in every viewer, and there were 0 leave/too slow/heartbeat lines. P-07: over.png printed the line in every viewer, no tab, no `preview accepted`. Probes on the share server gave 405/405/404, plus the SPA-shell 200; the loopback relay gave 404/404. **Tests:** `TestHubSendPreview_NearCapNoKick`, `TestPreview_HandoffDrainNotBlocked`, `previewAssembler.test.ts`. |
| 5 | SC4 / PREV-04: the home Safari share viewer gets the same previews. The share CSP changes only as far as the review approved, and the token gate, the tailnet-only listener and `frame-ancestors 'none'` are unchanged. | ✓ VERIFIED | **Code:** `csp_mw.go` adds only `img-src … blob:` and `worker-src 'self'` (Amendment 3). `frame-ancestors 'none'` stays, and blob: is never in default/script/connect (`TestCSPHeaders_PreviewDelta`, `TestCSPHeaderStrict_App`). `csp_mw.go`, `ImagePreview.tsx` and `style.css` are unchanged since 7042f5ce (`git diff 7042f5ce..HEAD`: only comments in `relay/preview.go`). **VM:** P-04: the served CSP equals the reviewed policy character for character, wrong token gets 401, PNG/PDF (Open PDF)/Markdown work in Safari, 0 violations, dark repaint shown in computed styles. At ea8bb0e2, RF-02 found Safari Markdown and Open PDF with 0 violations, and RF's `sharechk.sh` gave 302/200/401/401. 11-SECURITY-REVIEW.md: verdict approved, plus the CR-01 amendment. |
| 6 | SC5 / PREV-05: `portty open` works in a new Portty session with nothing added to the PATH by hand. In Terminal.app it isn't found until Install Command-Line Tool… is used; after the admin prompt, `/usr/local/bin/portty` points into /Applications/Portty.app and `portty list` works. Cancel changes nothing, and a foreign link is reported, not overwritten. | ✓ VERIFIED (website clause: human item 5) | **Code:** `cliPathEnv`/`refreshCLILink` (atomic rename, self-loop refusal, dedup per IN-03) is wired at `engine.go:234`. `installCommandLineTool` does an Lstat-first refusal of any existing link or file, a /Volumes/AppTranslocation refusal, the IN-04 confirm, osascript with paths passed as argv (`quoted form of`), `ln -s` without `-f`, and a silent return on cancel (-128). There is a one-run guard (IN-01). The menu item is at `main.go:171`. README step 2 is updated. **VM:** P-10 and P-08a (PATH in a zsh login session), P-08b (Terminal.app rc=1), P-08c (user-run install: link to /Applications/Portty.app, `portty list` rc 0), P-08d (Don't Allow: nothing changed), P-08e (foreign link reported and unchanged; own link reports "already installed"), P-08f (DMG copy refused). RF-04 at ea8bb0e2: confirm copy, Cancel gives no prompt, Continue gives the prompt, Don't Allow changes nothing. RF-01..03 typed plain `portty open` in a session made at ea8bb0e2, so the PATH still works after IN-03. **Tests:** `clilink_test.go` (incl. the real path_helper zsh case), `app_cli_install_test.go`, `TestAppMenu_Structure`. |
| 7 | A link inside an untrusted PDF can't load a page into the desktop preview iframe (11-REVIEW CR-01, desktop frames-only CSP). | ✓ VERIFIED (coincidental-reliance) | `frontend/index.html` has exactly one meta, `frame-src blob:; child-src blob:; worker-src 'self'`, pinned by `PreviewRenderers.test.tsx`. **VM RF-01** at ea8bb0e2: synthesized clicks, a double click and an AXPress on the PDF's `AXLink` gave 0 beacon requests, the PDF still shown, no navigation, `capability.key` unchanged, `/health` ok. Caveat: there was no pre-fix baseline (the click did nothing before the fix either), so this shows no regression and no navigation, but not that the CSP is the cause. See `coincidental_reliance_items`. |
| 8 | Markdown output past the WR-03 caps is refused in the worker before the main thread sanitizes or lays it out, and the result is Couldn't Show File. | ✓ VERIFIED (cap values: human item 4) | `markdown.ts` `MARKDOWN_MAX_HTML_LENGTH=2,000,000` / `MARKDOWN_MAX_ELEMENTS=50,000` throw in `parseMarkdown` (worker). Real-thread tests in `parseMarkdownInWorker.test.ts` cover a table over the cap and a code block over the cap (both rejected with the budget error) and a table under the cap (renders). RF-02: the slow file still stops at about 3 s, the normal file renders. |
| 9 | The review fixes (ea8bb0e2) keep every earlier behaviour: gates are green at HEAD, and the VM-only parts that changed were re-checked. | ✓ VERIFIED | `p11rf-go-test.log` (13:06:02, after the last Go commit 2a8c5975 at 13:03:38): 13/13 packages ok, no race. `p11rf-vitest.log` 1514/1517. The 3 failures were App.shellExit (2) and App.tailscale (1), caused by WR-04's deps change. They were fixed by 3b2e2a5d and re-run 44/44 (`p11rf-vitest-rerun.log`). `p11rf-tsc.log` is empty (clean). `p11rf-build.log`: build ok. Run by this verifier: `go build ./... && go vet ./...` clean and `check-traceability-paths.sh` reports 188 paths OK. The full vitest suite was not re-run after 3b2e2a5d. That commit only restores `[]` deps on a stable `useState` value, so the risk is low. |

**Score:** 9/9 truths verified (0 present but behaviour-unverified). There are 5 human decision or visual items.

### What the ea8bb0e2 VM checks cover, and what they don't

The main VM rows (P-00..P-10, P-08a-f, U-01..U-03) ran on product code `7042f5ce`. The review fixes then changed `index.html` (CR-01 meta), the SVG sniffer (WR-01), the sanitizer and the Markdown caps (WR-02/03), the install flow (IN-01/04/05), `cmd_open` (IN-02/06), PATH dedup (IN-03) and App.tsx `retryInit` (WR-04).

| Change | Re-checked at ea8bb0e2 | Not re-checked (and why the risk is low or what's left) |
|--------|------------------------|----------------------------------------------------------|
| CR-01 desktop meta CSP | RF-01 (PDF link gives no fetch or navigation). RF-02 (desktop Markdown worker loads under `worker-src 'self'` on `wails://`; desktop PDF renders; Safari Markdown and Open PDF with 0 violations). RF-03 (desktop SVG renders). | Desktop raster images and the hostile beacon set weren't re-run. The meta has no `img-src`/`default-src`, so it only narrows frames and workers. RF-01 has no pre-fix baseline (truth 7). |
| WR-01 SVG sniffer | RF-03 (Illustrator SVG accepted and rendered; billion-laughs refused, LOL-RC=1) | The wider accept set (an internal subset with no `&`/`%`) still renders only inside `<img>` (no script or fetch). This is unit-tested. |
| WR-02/WR-03 sanitizer and caps | RF-02 (normal .md renders on desktop and Safari; slow .md stops at about 3 s) | No large legitimate file near the caps was tried in WebKit (human item 4). |
| IN-04 confirm, IN-01 guard, IN-05 quoting | RF-04 (confirm copy, Cancel, Continue leading to the prompt, Don't Allow) | No successful install with the password was done at ea8bb0e2. The osascript argv is unchanged and the Continue path reached the same prompt. The success dialog's copy was never captured in any VM row (P-08c notes this). Unit tests pin it. |
| IN-02/IN-06 `cmd_open`, IN-03 PATH | RF-01..03 ran plain `portty open` in a new session at ea8bb0e2 | The FIFO race and the `--` handling are unit-tested only. |
| WR-04 `retryInit` | — | Unit test only (it fails with the fix commented out). |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd_open.go` | CLI: sniff, cap, one-line errors, API gate | ✓ VERIFIED | 241 lines. Wired through `cmd_cli.go` and excluded from `cliNeedsDaemon` (P-10: no daemon started). |
| `internal/daemon/preview.go` | Socket-only route, 202 then async | ✓ VERIFIED | Registered at `api.go:202` behind `freezeGate`. Not on `RelayHandler`. |
| `internal/relay/preview.go`, `protocol.go` | MsgPreview 0x21, paced sender, owner focus | ✓ VERIFIED | Golden fixture `preview-frames.json` is shared with the frontend parser. |
| `internal/daemon/clilink.go`, `internal/pty/native.go` | Session id env and PATH bin link | ✓ VERIFIED | `engine.go:234` `Env: e.cliPathEnv()`. |
| `app_cli_install.go`, `main.go` menu | Install Command-Line Tool… | ✓ VERIFIED | Menu callback runs in a goroutine. VM P-08a-f and RF-04. |
| `frontend/src/lib/{relayClient,previewAssembler,previewStore}.ts` | Frame parse, bounded assembly, 10 tabs / 100 MiB / revoke | ✓ VERIFIED | Fed from TerminalPanel (local and peer) and WebShareSessionView. |
| `frontend/src/components/{PreviewPanel,ImagePreview,PdfPreview,MarkdownPreview}.tsx`, `lib/markdown*.ts` | Renderers | ✓ VERIFIED | Only the visible preview renders (`App.tsx`). |
| `internal/webserver/csp_mw.go`, `frontend/index.html` | Share CSP delta and desktop frames-only meta | ✓ VERIFIED | Pinned by Go and vitest tests. |
| `.planning/phases/11-file-preview/11-SECURITY-REVIEW.md` | Reviewed and recorded | ✓ VERIFIED | Verdict approved. CR-01 amendment and residual risks. |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `portty open` (CLI) | daemon `POST /sessions/{id}/preview` | `DaemonClient.SendPreview` over the 0600 unix socket | WIRED (VM P-01..RF-03) |
| `handlePreview` | `Hub.SendPreview` | `a.engine.Manager().Get(id)` | WIRED |
| Hub | viewers (local, peer proxy, share WS) | per-subscriber `Msgs` queue; the peer proxy copies frames verbatim | WIRED (P-01 home peer, P-04 Safari, P-06 attach ignores frames) |
| relayClient `MSG_PREVIEW` | `App.handlePreview` and `PreviewStore` | `createPreviewAssembler` in TerminalPanel / WebShareSessionView `onPreview` | WIRED |
| `App` active preview | `PreviewPanel` renderer | `previews.find(e => e.tabId === activeId)` | WIRED |
| Daemon CreateSession | session PATH and `PORTTY_SESSION_ID` | `cliPathEnv()` and `mergeEnv` in `SpawnPTY` | WIRED (P-10, P-08a) |
| Portty menu | `installCommandLineTool` | `installCLICallback` goroutine | WIRED (P-08d-f, RF-04) |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| PreviewPanel / renderers | `entry.url` (blob) | bytes the CLI read, sent as hub frames, assembled into a Blob in previewAssembler | Yes (VM sha256-sized samples, `sent=` bytes match) | ✓ FLOWING |
| Tab list | preview tabs | `previewStore` snapshot via `useSyncExternalStore` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Tree builds and vets | `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./...` | clean | ✓ PASS |
| Traceability paths exist | `bash tests/check-traceability-paths.sh` | `OK: 188 traceability paths exist` | ✓ PASS |
| Go and vitest suites | not run, at the user's request (firewall alerts). Latest logs used instead (truth 9) | 13/13 ok; 1514/1517, then the fix and 44/44 | ✓ PASS (from logs) |

### Probe Execution

Not applicable. The phase declares no `scripts/*/tests/probe-*.sh`. The VM rows are the runtime evidence.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PREV-01 | 11-02/03/04/05/08/09/10 | `portty open` gives a preview tab in every app viewer | ✓ SATISFIED | Truths 1, 2 |
| PREV-02 | 11-01/06/07/09 | All 7 types, safe, native look, light and dark | ✓ SATISFIED (visual look: human item 3) | Truths 3, 8 |
| PREV-03 | 11-02/04/05/08 | Authenticated stream only, cap, no file-read API | ✓ SATISFIED | Truth 4 |
| PREV-04 | 11-01/06/09 | Browser previews with the reviewed CSP delta | ✓ SATISFIED | Truth 5 |
| PREV-05 | 11-03/08 | PATH in sessions, menu install, refusal and cancel, website step 2 | ✓ SATISFIED in this repo; website step 2 NEEDS HUMAN (item 5) | Truth 6 |

There are no orphaned requirements: REQUIREMENTS.md maps only PREV-01..05 to Phase 11, and the plans claim all of them.

### Decision Coverage

The gate (`check.decision-coverage-verify`) was skipped: "No trackable decisions in CONTEXT.md". The decisions are in tables. I checked them by hand:
- Honoured: focus rule, 20,000,000-byte cap, the CLI reads the file (the daemon never opens a path), paced sender, 10 tabs and 100 MiB, refresh in place by key, Open PDF fallback, `frame-ancestors` untouched, worker with a 3 s limit, `PORTTY_SESSION_ID`, API 3 and `PreviewAPI`, per-user bin link, exact pins.
- One recorded deviation: CONTEXT said "Desktop CSP: don't add one in this phase". The CR-01 review fix added a frames-and-workers-only meta. The deviation is recorded and argued in 11-SECURITY-REVIEW.md "Amendment (11-REVIEW CR-01)" and 11-REVIEW-FIX.md, and the user had the fix applied. Non-blocking.

### Test Quality Audit

| Test File | Linked Req | Skipped | Circular | Assertion Level | Verdict |
|-----------|-----------|---------|----------|-----------------|---------|
| `internal/relay/preview_test.go` | PREV-01/03 | 0 | No (golden fixture also read by the frontend) | Behavioral | OK |
| `internal/daemon/preview_test.go` | PREV-01/03 | 0 | No | Behavioral and source gate | OK |
| `internal/daemon/clilink_test.go` | PREV-05 | 1 conditional `t.Skipf` (only if path_helper is missing; the 11-10 gate had 0 SKIP lines) | No | Behavioral | OK |
| `cmd_open_test.go`, `app_cli_install_test.go` | PREV-01/03/05 | 0 | No | Value (exact lines) and behavioral | OK |
| `internal/webserver/{csp_mw,csp_integration,files_removed}_test.go` | PREV-03/04 | 0 | No | Value | OK |
| Frontend preview tests (`previewAssembler`, `previewStore`, `PreviewRenderers`, `markdown`, `MarkdownPreview`, `parseMarkdownInWorker`, `App.preview`, `TerminalPanel.preview`, `PreviewPanel`, `TabBar`, `relayClient`) | PREV-01..04 | 0 | No | Value and behavioral | OK |

Disabled tests on requirements: 0. Circular patterns: 0. Insufficient assertions: 0.

### Anti-Patterns Found

None. A scan of all 70 changed product files found no `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`. The `shortcut:` comments (`cmd_open.go` subset `]` early-end, `clilink.go` orphan temp link) are documented safe-direction trade-offs (Info).

### Human Verification Required

1. **P-07 SPA-shell 200.** `GET /app/sessions/<id>/preview` answers 200 with the app shell (byte-identical to `GET /app/`, no file bytes). The 11-10 plan said "never 2xx". SC3 itself is met. Accept it with an override, or ask for a 404 on `/app/sessions/*`.
2. **macOS 27.2 admin prompt copy.** The prompt text is Apple's own ("Allow administrator access for a script started by “Portty”?" plus the malware notice). The IN-04 confirm before it explains the purpose (RF-04). Accept.
3. **Desktop visual look in light and dark.** The VM rows proved content and the live switch, but colours and look were seen only in Safari. A quick Screen Sharing look at the desktop panes in both modes closes this.
4. **WR-03 Markdown caps** (2 M characters / 50 k elements). These are judgement values. Accept or tune them.
5. **Website install step 2** (PREV-05's last sentence). `portty-website/public/index.html:375` still shows the `sudo ln -sf` command. Update it in the website repo with the v1.2 release (copy in 11-08-SUMMARY "Website step 2").

Optional, low risk: no successful password install was done at ea8bb0e2 (only up to the prompt and Don't Allow, RF-04), and no VM row has captured the success dialog's copy.

### Gaps Summary

There are no blocking gaps. Every ROADMAP success criterion (1–5) and every merged plan truth has code, wiring, tests and VM evidence. The review fixes are re-covered at ea8bb0e2 by RF-01..RF-04 and green gate logs. What's left are product or visual decisions and one cross-repo copy change, so the status is `human_needed`. One advisory: RF-01 shows that a PDF link click does nothing, but not that the new desktop CSP is why. The meta is pinned by text only.

---

_Verified: 2026-11-21T19:23:13Z_
_Verifier: Claude (gsd-verifier)_

## Addendum (2026-11-21, human items resolved by the user: "Accept as recommended")

1. P-07: `GET /app/sessions/<id>/preview` returns the SPA shell (200, byte-identical to `/app/`, no file bytes). **Accepted.**
2. macOS 27.2 admin-prompt copy is Apple's. Portty's IN-04 confirm dialog explains it first. **Accepted.**
3. Light/dark look of the preview panes, never seen visually in the VMs. **Deferred to the v1.2 release check** on the user's own Mac.
4. WR-03 Markdown caps (2,000,000 characters / 50,000 elements). **Accepted.**
5. Website install step 2 (portty-website `public/index.html:375`, still the old `sudo ln -sf` command). **Deferred to the v1.2 release**, using the copy in 11-08-SUMMARY.md.

Status is now passed. No other content changed.
