---
phase: 11-file-preview
verified: 2026-11-23T11:20:00Z
status: passed
score: 9/9 must-haves verified
covered_files: [".planning/phases/11-file-preview/11-01-PLAN.md",".planning/phases/11-file-preview/11-01-SUMMARY.md",".planning/phases/11-file-preview/11-02-PLAN.md",".planning/phases/11-file-preview/11-02-SUMMARY.md",".planning/phases/11-file-preview/11-03-PLAN.md",".planning/phases/11-file-preview/11-03-SUMMARY.md",".planning/phases/11-file-preview/11-04-PLAN.md",".planning/phases/11-file-preview/11-04-SUMMARY.md",".planning/phases/11-file-preview/11-05-PLAN.md",".planning/phases/11-file-preview/11-05-SUMMARY.md",".planning/phases/11-file-preview/11-06-PLAN.md",".planning/phases/11-file-preview/11-06-SUMMARY.md",".planning/phases/11-file-preview/11-07-PLAN.md",".planning/phases/11-file-preview/11-07-SUMMARY.md",".planning/phases/11-file-preview/11-08-PLAN.md",".planning/phases/11-file-preview/11-08-SUMMARY.md",".planning/phases/11-file-preview/11-09-PLAN.md",".planning/phases/11-file-preview/11-09-SUMMARY.md",".planning/phases/11-file-preview/11-10-PLAN.md",".planning/phases/11-file-preview/11-10-SUMMARY.md","README.md","TESTING.md","app.go","app_cli_install.go","app_cli_install_test.go","cmd_cli.go","cmd_cli_test.go","cmd_open.go","cmd_open_test.go","dispatch_test.go","frontend/index.html","frontend/package.json","frontend/src/App.tsx","frontend/src/components/ImagePreview.tsx","frontend/src/components/MarkdownPreview.tsx","frontend/src/components/PdfPreview.tsx","frontend/src/components/PreviewPanel.tsx","frontend/src/components/TabBar.tsx","frontend/src/components/TerminalPanel.tsx","frontend/src/components/WebShareSessionView.tsx","frontend/src/components/__tests__/App.nav.test.tsx","frontend/src/components/__tests__/App.open-remote.test.tsx","frontend/src/components/__tests__/App.preview.test.tsx","frontend/src/components/__tests__/MarkdownPreview.test.tsx","frontend/src/components/__tests__/PreviewPanel.test.tsx","frontend/src/components/__tests__/PreviewRenderers.test.tsx","frontend/src/components/__tests__/TabBar.test.tsx","frontend/src/components/__tests__/TerminalPanel.preview.test.tsx","frontend/src/components/__tests__/WebShareSessionView.test.tsx","frontend/src/lib/__tests__/fixtures/preview-frames.json","frontend/src/lib/__tests__/fixtures/preview/hostile-external.svg","frontend/src/lib/__tests__/fixtures/preview/hostile-script.svg","frontend/src/lib/__tests__/fixtures/preview/hostile.md","frontend/src/lib/__tests__/markdown.test.ts","frontend/src/lib/__tests__/parseMarkdownInWorker.test.ts","frontend/src/lib/__tests__/previewAssembler.test.ts","frontend/src/lib/__tests__/previewStore.test.ts","frontend/src/lib/markdown.ts","frontend/src/lib/markdownSanitize.ts","frontend/src/lib/markdownWorker.ts","frontend/src/lib/parseMarkdownInWorker.ts","frontend/src/lib/previewAssembler.ts","frontend/src/lib/previewStore.ts","frontend/src/lib/relayClient.test.ts","frontend/src/lib/relayClient.ts","frontend/src/style.css","internal/daemon/api.go","internal/daemon/api_test.go","internal/daemon/client.go","internal/daemon/client_test.go","internal/daemon/clilink.go","internal/daemon/clilink_test.go","internal/daemon/engine.go","internal/daemon/preview.go","internal/daemon/preview_test.go","internal/daemon/types.go","internal/previewfile/previewfile.go","internal/previewfile/previewfile_test.go","internal/pty/native.go","internal/pty/native_test.go","internal/relay/hub.go","internal/relay/pathclick.go","internal/relay/preview.go","internal/relay/preview_test.go","internal/relay/protocol.go","internal/webserver/csp_integration_test.go","internal/webserver/csp_mw.go","internal/webserver/csp_mw_test.go","internal/webserver/files_removed_test.go","internal/webserver/input_queue_test.go","main.go","menu_test.go","testmain_test.go"]
covered_digest: "v4:sha256:feab6684d5732a46b83a72454bb1f2f81e093630e18bebb840a8e88ef077f0a4"
behavior_unverified: 0
overrides_applied: 1
overrides:
  - must_have: "11-10: probes of a preview route on the share server never answer 2xx"
    reason: "P-07: GET /app/sessions/<id>/preview is the /app/* SPA fallback (200, byte-identical to GET /app/, no file bytes), not a preview route. Accepted by the user on 2026-11-21 ('Accept as recommended')."
    accepted_by: user
    accepted_at: 2026-11-21
re_verification:
  previous_status: passed
  previous_verified: 2026-11-23T04:07:52Z
  previous_digest: "v4:sha256:d804bd98ff52c68abbbe7be1305c75fe6a8f51e48b032f56714994413291e912"
  reason: "stale fingerprint: Phase 15 (v1.2 Hardening) changed covered files"
  gaps_closed: []
  gaps_remaining: []
  regressions: []
accepted_limits:
  - "macOS 27.2 administrator prompt copy is Apple's; the IN-04 confirm explains it first (accepted 2026-11-21)."
  - "WR-03 Markdown caps 2,000,000 HTML characters / 50,000 elements (accepted 2026-11-21)."
  - "Desktop light/dark look of the preview panes: deferred by the user to the v1.2 release check on the user's own Mac."
  - "Website install step 2 (portty-website public/index.html): deferred by the user to the v1.2 release, copy in 11-08-SUMMARY.md."
coincidental_reliance_items:
  - truth: "A link inside an untrusted PDF can't load a page into the desktop preview iframe (11-REVIEW CR-01)"
    reason: undeclared-precondition
    harden: "Unchanged from the first verification. RF-01 saw no navigation and 0 beacon hits at ea8bb0e2, with no pre-fix baseline; the frame-src blob: meta is pinned by text only (PreviewRenderers.test.tsx). The meta is still byte-identical at HEAD (Phase 14 added only viewport-fit, manifest, apple-touch-icon and theme-color tags beside it)."
---

# Phase 11: File Preview Verification Report (re-verification)

**Phase Goal:** A user in any Portty session, local or remote, can run `portty open <file>` and see an image, PDF or Markdown file in a preview tab beside the terminal, in the app and in a browser share viewer. The bytes never leave the session's authenticated stream.
**Verified:** 2026-11-23T11:20:00Z at HEAD `c818e1e1` (branch gsd/v1.2)
**Status:** passed
**Re-verification:** Yes (second). Pass 2 (this one) re-checked only what Phase 15 changed, see the section below. The first report (2026-11-21, `20da067a`) passed after the user resolved its five human items. The GSD tool marked it stale because Phases 12, 13 and 14 changed covered files. This pass re-checks the ROADMAP goal, SC1-SC5 and the plans' must_haves against the current code. The VM evidence (11-VM-CHECKS.md, rows at `7042f5ce` and RF-01..RF-04 at `ea8bb0e2`) is not re-run; it is still valid where the code it exercised is unchanged or moved verbatim, as recorded per change below.

## What changed in covered files since the first verification

`git diff 20da067a..HEAD` over the covered files, judged one change at a time:

| Change (commit) | Effect on Phase 11 | Verdict |
|-----------------|--------------------|---------|
| `34bc8162` refactor(12-01): `portty open` file checks moved from `cmd_open.go` into `internal/previewfile` | `previewfile.Open` is the old `cmd_open` sequence verbatim: stat (missing, folder, not regular, over cap), `O_NONBLOCK` open, regular re-check on the descriptor (IN-02), `LimitReader(cap+1)`, grew-after-stat check, `MIME` (content sniff for PNG/JPEG/GIF/WebP/PDF; SVG root with the WR-01 subset rule; UTF-8 no-NUL Markdown), SHA-256 path key. `cmd_open.go` maps the sentinels to the same one-line errors. | Behaviour unchanged. `TestCmdOpen_*` (7, incl. `_FIFORefusedPromptly`, `_Lines`, `_TypeDetection`) and `internal/previewfile` tests pass. |
| `806f611b` refactor(12-01): `cleanPreviewName` moved to `relay.CleanPreviewName` | Same body; the daemon route calls it. | Unchanged. `TestPreview_NameIsPercentDecodedAndCleaned` passes. |
| `a01b36a4` feat(12-04): path clicks preview printed absolute paths | `Preview.FocusSub` lets a path click focus the clicker; nil keeps the owner rule. `relay/pathclick.go` reads a file through `previewfile` only. | Intended supersession (Phase 12 SC4 extends PREV-03: the host previews only a path the session itself printed, recorded in Phase 12's security review). The Phase 11 rule still holds for every *route*: the daemon preview route takes bytes, never a path; `TestPreview_SourceGate` (daemon/relay preview source imports no `os`/`io/fs`/`path/filepath`), `TestPreview_NotOnRelayHandler`, `TestPreviewRoute_NotOnShareServer` pass, and the new `TestPathClick_SourceGate` pins `pathclick.go` as the only relay file that reaches `previewfile`. The `portty open` focus rule is unchanged (`TestHubSendPreview_TwoViewersOwnerFocus`, `_NoOwnerNobodyFocuses` pass; `App.tsx:667` still switches only on `ev.focus` while the session or its preview is active). |
| `f26647f8` feat(14-05): share CSP Amendment 4 `manifest-src 'self'` | Adds one directive for the Home Screen manifest. `img-src 'self' data: blob:`, `worker-src 'self'`, `default-src 'none'`, `frame-ancestors 'none'` unchanged; no blob: in default/script/connect. | Intended supersession, reviewed in 14-SECURITY-REVIEW.md. `TestCSPHeaders_PreviewDelta`, `TestCSPHeaderStrict_App` and the other `TestCSP*` (19 in webserver) pass. |
| `ccd0e1b4` feat(14-05): Apple head tags in `frontend/index.html` | The CR-01 frames-only meta (`frame-src blob:; child-src blob:; worker-src 'self'`) is byte-identical; only viewport-fit, manifest, touch-icon and theme-color tags were added. | Unchanged for Phase 11. |
| `ed20db33` feat(14-08): Open PDF in a Home Screen web app | In standalone mode only, Open PDF shows an in-app layer (blob `<img>` under the existing `img-src blob:`, "Open full PDF" = the old `window.open`, Done/Esc back). In a normal browser tab Open PDF is the 11-SPIKE S-04 top-level open as before; the desktop iframe path is untouched. | Intended supersession for MOB-03; the Phase 11 browser behaviour is the non-standalone branch. `PreviewRenderers.test.tsx` passes. |
| Phase 12-14 edits to `TerminalPanel.tsx`, `WebShareSessionView.tsx`, `relayClient.ts`, `App.tsx`, `style.css`, `TabBar.tsx`, `protocol.go`, `hub.go`, `engine.go`, `types.go`, `client.go` | New message types (OSC 7 cwd, path click, port forward), key bar, phone layout, reconnect. `APIVersion` is now 5; `PreviewAPI` stays 3 and `MinDaemonAPI` stays 1, so the `portty open` gate is unchanged. `engine.go:228` still sets `cliPathEnv()`; `native.go:69` still sets `PORTTY_SESSION_ID`. | Preview wiring intact; the preview frontend tests below pass. |

The Phase 14 discovery-probe gating and the lone-Esc filter change do not touch Phase 11 code paths.

## Phase 15 delta (HEAD `c818e1e1` vs the previous pass at `52d960a5`)

`git diff 52d960a5..HEAD` over the covered files touches five of them. None is on a preview path:

| Change | Effect on Phase 11 | Verdict |
|--------|--------------------|---------|
| `internal/relay/hub.go` + `history.go`: `History.Skip(emu.ScrollbackLen())` in the ED 3 handler (15-04, F-1) | Counts dropped scrollback so absolute row numbers stay monotonic for cwd records and path clicks. `SendPreview`, the paced sender, the focus rule and `preview.go`/`protocol.go` are untouched. | No effect on `portty open`. `TestPreview*`, `TestHubSendPreview_*`, `TestCwd_ClearKeepsRowsMonotonic`, `TestPathClick_RelativeAfterClear`, `TestPathClick_SourceGate` pass. |
| `App.tsx`, `TerminalPanel.tsx`, `WebShareSessionView.tsx`, `TabBar.tsx` (15-05, 15-06): `Tab.sessionEnded`, `onSessionEnded`, `peerConnectVerdict(..., sawEnded)`, `update:show` listener | An optional callback fired only on the session-ended close, one flag on peer tabs, and an update-banner listener. Preview handling (`handlePreview`, the `ev.focus` switch, `isActive={!activePreview}`, `PreviewPanel` mounting) is not in the diff. | Preview wiring intact. App.preview, TerminalPanel.preview, WebShareSessionView, App.open-remote, App.nav, TabBar tests pass. |
| `TESTING.md` (+38 lines) | Phase 15 manifest and traceability rows. | `tests/check-traceability-paths.sh`: OK, 228 paths. |

Not changed since the previous pass: `csp_mw.go`, `frontend/index.html` (the CR-01 `frame-src blob:` meta is still byte-identical), `frontend/package.json` (marked 18.1.3 and dompurify 3.4.17 still exact-pinned), `internal/previewfile`, `internal/relay/preview.go`, `internal/relay/protocol.go`, `cmd_open.go`, `internal/daemon/{preview,api,client,types,clilink}.go`, every preview component, store, assembler and Markdown file. Phase 15's `engine.go` edit is a comment only. The other Phase 15 files (attach, handoff, socket hardening, fdpass, tray, statusbar, updater) are outside this phase's truths.

The earlier human-verification history (first report's five items, the VM rows at `7042f5ce` and RF-01..RF-04 at `ea8bb0e2`, the user's accepted decisions) is carried as already verified: no Phase 15 change alters the behaviour those runs checked.

Targeted checks run at HEAD (`-count=1`, `GOTOOLCHAIN=go1.26.11`; full suites not run): `go build ./...` clean; `go vet` on previewfile, relay, daemon, webserver clean; Go tests for previewfile, relay and daemon preview/CLI-link/path-click, pty env and health, `TestCSP*` and `TestPreviewRoute*` in webserver, and `TestCmdOpen|TestInstallCLI|TestAppMenu|TestCLINeedsDaemon` in the root package all `ok`; vitest on the same 14 preview files: 14 files, 453 tests passed (the previous pass counted 450; the difference sits in App.open-remote.test.tsx, which Phase 15 extended by 27 lines; not attributed case by case); traceability OK (228). Nothing touched a host tailscaled, a 100.x address or the VMs.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence at HEAD |
|---|-------|--------|------------------|
| 1 | SC1 / PREV-01: `portty open` opens a preview tab beside the session's tab in every app viewer; closing it leaves the session; missing or unsupported files get one CLI line. | ✓ VERIFIED | `cmd_open.go` (args, session id, `previewfile.Open`, `HealthInfo` gate on `PreviewAPI` 3, `SendPreview`). Socket-only route `POST /sessions/{id}/preview`. `Hub.SendPreview` to every non-replaying subscriber. Frontend relayClient `MSG_PREVIEW` to assembler to `App.handlePreview`. Tests: `TestCmdOpen_EndToEnd`, `TestPreview_EndToEnd`, `App.preview.test.tsx`, `TerminalPanel.preview.test.tsx`. VM P-01 (code path unchanged except the verbatim move). |
| 2 | Focus rule: only the last typist switches, and only while it shows that session or its preview. | ✓ VERIFIED | `Focus: sub == focus`, `focus = h.owner` unless a Phase 12 path click sets `FocusSub` (not used by `portty open`). `App.tsx:667`. `TestHubSendPreview_TwoViewersOwnerFocus`, `_NoOwnerNobodyFocuses`, App.preview focus cases. |
| 3 | SC2 / PREV-02: all seven types render natively in light and dark; PDF proven in Wails first; hostile SVG and Markdown run and fetch nothing. | ✓ VERIFIED (desktop look deferred by the user, accepted limit) | `ImagePreview` single `<img src=blob:>`; `PdfPreview` iframe on desktop, Open PDF on the web; marked 18.1.3 and dompurify 3.4.17 still exact-pinned in `frontend/package.json`; sanitizer and worker files unchanged. `markdown.test.ts`, `MarkdownPreview.test.tsx`, `PreviewRenderers.test.tsx`, `parseMarkdownInWorker.test.ts` pass. 11-SPIKE.md verdict stands. |
| 4 | SC3 / PREV-03: over-cap shows "too large", never partial; near-cap stream drops no viewer; no route reads a file by path. | ✓ VERIFIED (P-07 SPA-shell 200 accepted by the user) | `PreviewMaxBytes` enforced in `previewfile.Open`, the daemon (`ContentLength`, `MaxBytesReader`), `validate()` and `previewAssembler`. Paced sender unchanged apart from `FocusSub`. `TestHubSendPreview_NearCapNoKick`, `_StallAborts`, `TestPreview_HandoffDrainNotBlocked`, `TestPreview_SourceGate`, `TestPreview_NotOnRelayHandler`, `TestPreviewRoute_NotOnShareServer`, `previewAssembler.test.ts` pass. Phase 12's printed-path click is the recorded supersession above. |
| 5 | SC4 / PREV-04: Safari share viewer gets the same previews; CSP changes only as reviewed; token gate, tailnet-only listener and `frame-ancestors 'none'` unchanged. | ✓ VERIFIED | `csp_mw.go` keeps `img-src … blob:` and `worker-src 'self'` from 11-SECURITY-REVIEW; the only later delta is Phase 14's reviewed `manifest-src 'self'`. `TestCSP*` pass. `WebShareSessionView.test.tsx` passes. |
| 6 | SC5 / PREV-05: `portty open` works in a new session with no PATH setup; Install Command-Line Tool… links `/usr/local/bin/portty`; cancel changes nothing; a foreign link is reported. | ✓ VERIFIED (website step 2 deferred by the user to the v1.2 release) | `cliPathEnv` at `engine.go:228`, `PORTTY_SESSION_ID` at `native.go:69`, menu item at `main.go:171`. `TestCLILink_*` (8, incl. both real zsh path_helper cases), `TestInstallCLI_*` (12), `TestAppMenu_Structure`, `TestCLINeedsDaemon`, `TestSpawnPTY_EnvSessionID` pass. |
| 7 | CR-01: a link inside an untrusted PDF can't load a page into the desktop iframe. | ✓ VERIFIED (coincidental-reliance, unchanged) | Meta byte-identical at HEAD; pinned by `PreviewRenderers.test.tsx`. RF-01 caveat carried forward. |
| 8 | WR-03 Markdown caps refuse oversize output in the worker. | ✓ VERIFIED (cap values accepted) | `markdown.ts` unchanged; `parseMarkdownInWorker.test.ts` passes. |
| 9 | Later phases keep every Phase 11 behaviour: targeted Phase 11 tests are green at HEAD. | ✓ VERIFIED | See spot-checks. |

**Score:** 9/9 truths verified. Plan must_haves (11-01..11-10) checked against HEAD: 11-01 (spike, doc-only) and 11-10 (gate, VM rows, TESTING.md) are evidence records that stand; 11-02..11-09 truths map to the code and tests above. The one literal miss (11-10 "never 2xx") carries the user's override.

### Behavioral Spot-Checks (run by this verifier, targeted only)

| Behavior | Command | Result |
|----------|---------|--------|
| Build and vet | `GOTOOLCHAIN=go1.26.11 go build ./... && go vet` (previewfile, relay, daemon, webserver, main) | clean |
| previewfile | `go test -short ./internal/previewfile` | ok |
| Relay and daemon preview, CLI link, path-click source gate | `go test -short -run 'TestPreview\|TestHubSendPreview\|TestPathClick_SourceGate\|TestCLILink' ./internal/relay ./internal/daemon` | 32 PASS, 0 FAIL, 0 SKIP |
| PTY env, client, health | `go test -short -run 'TestSpawnPTY_EnvSessionID\|TestSendPreview\|TestHealthInfo\|TestAPIHealth' ./internal/pty ./internal/daemon` | 6 PASS |
| Share CSP and route absence | `go test -short -run 'TestCSP\|TestPreviewRoute' ./internal/webserver` | 19 PASS |
| CLI, install, menu | `go test -short -run 'TestCmdOpen\|TestInstallCLI\|TestAppMenu\|TestCLINeedsDaemon' .` | 26 PASS |
| Frontend preview path | `vitest run` on 14 files (previewAssembler, previewStore, markdown, parseMarkdownInWorker, relayClient, PreviewRenderers, MarkdownPreview, PreviewPanel, App.preview, TerminalPanel.preview, TabBar, WebShareSessionView, App.open-remote, App.nav) | 14 files, 453 tests passed (re-run at `c818e1e1`) |
| Traceability | `bash tests/check-traceability-paths.sh` | OK: 228 paths |

Full suites were not run (five verifiers in parallel). Nothing touched the host's tailscaled, a 100.x address, the host GUI or the Tart VMs.

### Requirements Coverage

| Requirement | Status | Evidence |
|-------------|--------|----------|
| PREV-01 | ✓ SATISFIED | Truths 1, 2 |
| PREV-02 | ✓ SATISFIED (desktop look: deferred, accepted) | Truths 3, 8 |
| PREV-03 | ✓ SATISFIED (Phase 12 printed-path click is a recorded extension) | Truth 4 |
| PREV-04 | ✓ SATISFIED (Phase 14 manifest-src is a reviewed amendment) | Truth 5 |
| PREV-05 | ✓ SATISFIED in this repo (website step 2: deferred to release, accepted) | Truth 6 |

### Accepted decisions carried forward (not gaps)

From the first report's addendum (user: "Accept as recommended", 2026-11-21):
1. P-07 SPA-shell 200 on `/app/sessions/<id>/preview`: accepted (override above).
2. macOS 27.2 administrator prompt copy: accepted.
3. Desktop preview panes' light/dark look: deferred to the v1.2 release check on the user's Mac.
4. WR-03 Markdown caps: accepted.
5. Website install step 2: deferred to the v1.2 release (copy in 11-08-SUMMARY.md).

Plus the recorded CONTEXT deviation (the CR-01 desktop frames-only meta), argued in 11-SECURITY-REVIEW.md "Amendment (11-REVIEW CR-01)".

### Anti-Patterns Found

None new. `previewfile.go` keeps the documented `shortcut:` on the DOCTYPE subset `]` early end (safe direction).

### Gaps Summary

No gaps. Every Phase 11 success criterion still holds at HEAD; the changes since the first verification are a verbatim refactor into `internal/previewfile` and intended, reviewed extensions from Phases 12 and 14, each covered by passing targeted tests.

---

_Verified: 2026-11-23T11:20:00Z_
_Verifier: Claude (gsd-verifier), re-verification_
