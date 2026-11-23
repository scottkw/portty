---
phase: 12-file-path-click
verified: 2026-11-23T04:07:19Z
status: passed
score: 21/21 must-haves verified (3 by recorded user override)
covered_files: [".planning/phases/12-file-path-click/12-01-PLAN.md", ".planning/phases/12-file-path-click/12-01-SUMMARY.md", ".planning/phases/12-file-path-click/12-02-PLAN.md", ".planning/phases/12-file-path-click/12-02-SUMMARY.md", ".planning/phases/12-file-path-click/12-03-PLAN.md", ".planning/phases/12-file-path-click/12-03-SUMMARY.md", ".planning/phases/12-file-path-click/12-04-PLAN.md", ".planning/phases/12-file-path-click/12-04-SUMMARY.md", ".planning/phases/12-file-path-click/12-05-PLAN.md", ".planning/phases/12-file-path-click/12-05-SUMMARY.md", ".planning/phases/12-file-path-click/12-06-PLAN.md", ".planning/phases/12-file-path-click/12-06-SUMMARY.md", ".planning/phases/12-file-path-click/12-07-PLAN.md", ".planning/phases/12-file-path-click/12-07-SUMMARY.md", ".planning/phases/12-file-path-click/12-08-PLAN.md", ".planning/phases/12-file-path-click/12-08-SUMMARY.md", ".planning/phases/12-file-path-click/12-09-PLAN.md", ".planning/phases/12-file-path-click/12-09-SUMMARY.md", ".planning/phases/12-file-path-click/12-REVIEW-FIX.md", ".planning/phases/12-file-path-click/12-REVIEW.md", ".planning/phases/12-file-path-click/12-SECURITY-REVIEW.md", ".planning/phases/12-file-path-click/12-SECURITY.md", ".planning/phases/12-file-path-click/12-VM-CHECKS.md", "TESTING.md", "frontend/src/App.tsx", "frontend/src/components/TerminalPanel.tsx", "frontend/src/components/WebShareSessionView.tsx", "frontend/src/components/__tests__/App.open-remote.test.tsx", "frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx", "frontend/src/components/__tests__/WebShareSessionView.peerLinks.test.tsx", "frontend/src/lib/__tests__/pathLinks.linkifier.test.ts", "frontend/src/lib/pathLinks.ts", "frontend/src/lib/relayClient.ts", "internal/daemon/engine.go", "internal/daemon/types.go", "internal/daemon/zshshim.go", "internal/daemon/zshshim.zsh", "internal/previewfile/previewfile.go", "internal/relay/cwd.go", "internal/relay/hub.go", "internal/relay/pathclick.go", "internal/relay/server.go", "internal/webserver/plugin_config.go", "internal/webserver/server.go"]
covered_digest: "v4:sha256:f131e3076c018dc64765683b778c467cae5b165a8d62958c0e69dccec7a0f135"
behavior_unverified: 0
overrides_applied: 3
overrides:
  - must_have: "ssh localhost works inside a shimmed session (12-07 truth, C-08 clause)"
    reason: "Rig limit, not a code gap: the VM user has no key for itself, copying the rig key was refused and the VM password must not be used. Every other C-08 clause passed in both dotfile setups. The user chose to check it on their own Macs (manual check M-133); first-run C-08 rewritten PASS with the user's words in 12-VM-CHECKS.md."
    accepted_by: "user (12-09 Task 3 checkpoint, UI-1: \"Check on my Macs\")"
    accepted_at: "2026-11-22T13:31:05Z"
  - must_have: "Cmd-held underline and pointer in the desktop app (C-06, UI-SPEC)"
    reason: "The Wails web view is unobservable in the Tart VMs (blank WebKit captures, no terminal rows in the AX tree). Safari, with the same WebKit and bundle, shows the underline following Meta (C-06R). The user kept the Cmd-held underline (no hover fallback) and made the app check manual (M-131)."
    accepted_by: "user (12-09 Task 3 checkpoint, UI-2: \"Check on my Macs\")"
    accepted_at: "2026-11-22T13:31:05Z"
  - must_have: "Live light/dark switch repaints the notice with no reload in the desktop app (backstop)"
    reason: "App notice colours cannot be read through AX in the VMs. Safari repainted the notice dark in 0.9 s on the same connection (C-06R). The user made the app check manual (M-131)."
    accepted_by: "user (12-09 Task 3 checkpoint, UI-2: \"Check on my Macs\")"
    accepted_at: "2026-11-22T13:31:05Z"
re_verification:
  previous_status: passed
  previous_score: 21/21
  reason: "stale fingerprint: Phases 13 and 14 and the selection fix (7a37bfb4) changed covered files after the 2026-11-22T13:54:43Z verification"
  gaps_closed: []
  gaps_remaining: []
  regressions: []
  superseded_by_later_phases:
    - "Phase 14 (770bfedc, 14-01): the share server's WebSocket and plugin-config routes now also require the same Tailscale account. A browser or peer viewer of the user's own account keeps path clicks unchanged; another account is refused before any stream exists. A tightening of who reaches Hub.PathClick, not a loss of the phase goal."
    - "Phase 13 (5cad573e): pathclick.go's printed check was generalized to findMatch(predicate) for Hub.PortPrinted; findPrinted keeps the same two passes and bounds for path tokens."
    - "Phase 13: APIVersion moved 4 to 5; PathClickAPI stays 4 and MinDaemonAPI 1."
    - "Phase 14 lone-Esc change (659aee3a) is in the input absorber (oscabsorb.go), not the OSC 7 output path; selection fix 7a37bfb4 (terminalScale) corrects pointer mapping on a scaled pane and does not touch pathLinks.ts."
---

# Phase 12: File Path Click Verification Report

**Phase Goal:** A user can Cmd-click a file path that a program printed in the terminal and see it previewed, in local and remote sessions alike, because Portty tracks each shell's working directory.
**Verified:** 2026-11-23T04:07:19Z (HEAD 52d960a5)
**Status:** passed
**Re-verification:** Yes. The previous report (passed, 21/21, 2026-11-22T13:54:43Z) went stale because Phases 13 and 14 and the selection fix changed covered files. I re-checked every must-have against the current code and re-ran the targeted tests.

Nothing in the later changes breaks Phase 12. The changes to covered files are: Phase 13's refactor of the printed check into a predicate form (behaviour kept for paths), Phase 14's same-account gate on the share server (a tightening), the relay client's stuck-connect and pageshow additions, the TerminalPanel port notice and key bar, and the selection scale fix. None of them removes or bypasses path-click wiring. The three rig-limited items stay accepted by the user's 12-09 decision (M-131, M-133).

## What changed in covered files since the last verification

| File | Change (commit) | Effect on Phase 12 |
|------|-----------------|--------------------|
| `internal/relay/pathclick.go` | `findPrinted` now calls `findMatch(hasToken…, len(text))`; `firstPass`/`fallbackPass` are thin wrappers (5cad573e, Phase 13) | Same algorithm, same bounds (`len(row)+needleLen+2`). Gate at `:57-69` unchanged. PathClick tests pass |
| `internal/relay/hub.go` | adds `portPrintedBusy` (Phase 13) | separate flag; `pathClickBusy` untouched |
| `internal/webserver/server.go` | `requireSameUser` on the plugin-config, session and WebSocket routes (770bfedc, Phase 14); port grant/proxy routes (Phase 13) | MsgPathClick arm at `:1097-1103` unchanged (read-only still dropped). Only same-account viewers reach it now: superseded, tighter |
| `internal/daemon/types.go` | `APIVersion = 5` (Phase 13) | `PathClickAPI = 4` kept (`:74`); viewers gate on `>= 4` |
| `frontend/src/lib/relayClient.ts` | stuck-connect report, pageshow wake, other-account stop reason (e377fbf9, Phase 14) | capability callbacks on close, abandon and replay meta unchanged |
| `frontend/src/components/TerminalPanel.tsx` | Phase 13 port notice beside the path notice; Phase 14 key bar, pinch, Ctrl; 7a37bfb4 scale-aware selection | `onPathClickCapability` still calls `invalidate()` (`:641-643`); `registerPathLinks` at `:1076`; still 0 `registerLinkProvider` calls (Phase 95 guard) |
| `frontend/src/App.tsx` | Phase 13 remote-port handling, Phase 14 viewport inset | `peerPluginConfig` (`:153-159`) still passes only `webLinks`/`webLinksConfig`; used for remote tabs at `:1341` |
| `frontend/src/components/WebShareSessionView.tsx` | Phase 14 live plugin config, key bar, own-devices screen | `isWebGuest = pluginConfig === undefined && remote !== true` (`:191`): peer tabs still never self-fetch; `effectivePluginConfig = livePluginConfig ?? pluginConfig` keeps the peer config |
| `TESTING.md` | Phase 13/14 rows | Category BB now at `:1673`; traceability gate passes (228 paths) |
| `12-SECURITY.md` | WR-01 accepted by the user as AR-12-09 | previous "pending" item is now an accepted risk |

No change since the last verification in `zshshim.go`, `zshshim.zsh`, `engine.go`, `cwd.go`, `previewfile.go`, `relay/server.go`, `plugin_config.go`, `pathLinks.ts` or any of the Phase 12 test files.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: a new zsh session reports cwd via OSC 7 after every `cd` with no setup; other shells that emit OSC 7 are tracked; without OSC 7 an absolute path works and a relative one does nothing (PATH-01) | ✓ VERIFIED | Shim, engine and `cwd.go` unchanged since last verification. `go test -run 'Zsh\|Shim' ./internal/daemon/` 18 PASS (real-zsh matrix); `-run 'Cwd\|OSC7…' ./internal/relay/` PASS. C-08 shell matrix and U-01/U-01R (no-shim: absolute `preview`, relative `no-cwd`) carried |
| 2 | SC2: a Cmd-click on an absolute or relative path opens a preview in a local tab AND in a home-VM peer tab over the real tailnet (PATH-02) | ✓ VERIFIED | `App.tsx:153-159,1341`, `WebShareSessionView.tsx:191`, `TerminalPanel.tsx:1076`. `WebShareSessionView.peerLinks` and `App.open-remote` pass now. VM: C-01R local, C-02R peer tab, C-03R Safari. Phase 14's same-account gate leaves the user's own peer/browser viewers able to click |
| 3 | SC3: a missing path does nothing; a plain click selects; URLs keep their behaviour | ✓ VERIFIED | C-01R, C-04R, C-02R. `TerminalPanel.pathClick` and `TerminalPanel.web-links` pass now. Selection fix 7a37bfb4 improves plain-click selection on a scaled pane; it does not change path or URL clicks |
| 4 | SC4: the request rides the session's stream, resolves against the session's own cwd, only printed paths; the security review records it | ✓ VERIFIED | `pathclick.go:57-69` gate unchanged; printed check same algorithm via `findMatch`; imports still exclude os, os/exec, path/filepath, io/fs, syscall. Both read pumps (`relay/server.go:207-212`, `webserver/server.go:1097-1103`) drop read-only. `12-SECURITY-REVIEW.md:172` **Verdict:** APPROVED; `12-SECURITY.md` `threats_open: 0`. Phase 14 adds a same-account check ahead of the stream (superseded, tighter) |
| 5 | The full gate passes once on the host | ✓ VERIFIED (carried) | 12-VM-CHECKS.md re-run gate (G-1 `internal/pty` flake accepted by the user). Not re-run here: five verifiers share the host, so I ran only targeted tests. Later phases ran their own full gates (Phase 14 verification: vitest 122 files / 1813 tests) |
| 6 | `ssh localhost` works inside a shimmed session (C-08 clause) | PASSED (override) | User decision UI-1 at the 12-09 checkpoint; M-133 |
| 7 | After a window resize a printed path is still clickable (C-09) | ✓ VERIFIED | `pathLinks.ts:151` `term.onResize(invalidate)`; the three real-xterm "after a resize" tests pass now; C-09R |
| 8 | After an upgrade, a pre-upgrade relative path is clickable at once in every surviving viewer; all reconnect with `mode=delta` (U-01) | ✓ VERIFIED | `TerminalPanel.tsx:641-643`; real-xterm "after a reconnect" tests pass now; U-01R. The known full-suite flake of one reconnect test is a todo (`2026-11-22-linkifier-reconnect-flake.md`, fixed sleep under load); it passed in this targeted run |
| 9 | TESTING.md Category BB M-128..M-135 and the security review verdict are written | ✓ VERIFIED | `TESTING.md:1673` `### Category BB`, 10 references to 12-VM-CHECKS.md; verdict at `12-SECURITY-REVIEW.md:172` |
| 10 | Cmd-held underline and pointer in the desktop app (C-06, UI-SPEC) | PASSED (override) | User decision UI-2; Safari half PASS (C-06R); M-131 |
| 11 | A live light/dark switch repaints the notice with no reload (backstop) | PASSED (override) | User decision UI-2; Safari PASS (C-06R); M-131 |
| 12 | A Cmd-click in a mouse-reporting TUI previews (backstop) | ✓ VERIFIED | C-07R; real-xterm "sends no input to a program with mouse tracking on" passes now |
| 13 | Every FAIL reaches the checkpoint; nothing is concluded before the user answers | ✓ VERIFIED | 12-VM-CHECKS.md "Task 3 decisions", 12-09-SUMMARY |
| 14 | Click, missing-file silence, wrong-type notice and read-only refusal across local tab, peer tab and browser link (backstop) | ✓ VERIFIED | C-04R, C-05R; `TestPathClick_*` relay and webserver tests pass now |
| 15 | Real xterm 6 + real RelayClient: Cmd-clickable at once after a 1012 close and delta replay (12-08) | ✓ VERIFIED | `pathLinks.linkifier.test.ts` passes now |
| 16 | Real xterm 6: after a grid resize a same-cell Cmd-click sends the path click and opens a URL (12-08) | ✓ VERIFIED | three resize tests pass now |
| 17 | Each reproduction failed before its fix (12-08) | ✓ VERIFIED (carried) | RED commits 523fb5d5, 1823a698; history unchanged |
| 18 | The link refresh never sends input; a read-only viewer gets no link (12-08) | ✓ VERIFIED | `pathLinks.ts:147` `bubbles: false`; tests pass now; C-05R |
| 19 | A desktop peer tab registers path and URL links from this Mac's settings, makes no cross-origin request, turns on no other addon (12-08) | ✓ VERIFIED | `App.tsx:153-159`; `WebShareSessionView.peerLinks`, `App.open-remote` pass now |
| 20 | The browser viewer still self-fetches; no CORS header; CSPs unchanged by Phase 12 (12-08) | ✓ VERIFIED | no non-test `Access-Control-Allow` in `internal/webserver`; `isWebGuest` self-fetch path at `WebShareSessionView.tsx:191-222`. Phase 14 later added `manifest-src 'self'` to the share CSP with user approval (14-SECURITY-REVIEW Amendment 4): a later intentional change, not a Phase 12 regression |
| 21 | TESTING.md lists the two new test files in Sections 2 and 4; the traceability gate passes (12-08) | ✓ VERIFIED | `bash tests/check-traceability-paths.sh` → `OK: 228 traceability paths exist` |

Plan-level must-haves of 12-01..12-06 re-checked against current code: `internal/previewfile` imports no module package and has 90.1% statement coverage now (`go test -cover`); `relay/pathclick.go` imports no os/os-exec/filepath/fs/syscall; `PathClickAPI = 4` and `MinDaemonAPI` unchanged; TerminalPanel has no `registerLinkProvider` (provider only in `lib/pathLinks.ts`); the replay meta `pathClick` gating and read-only refusals are covered by the relay and webserver `PathClick` tests, which pass.

**Score:** 21/21 truths verified (18 verified, 3 passed by override; 0 present but behavior-unverified)

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| TerminalPanel | pathLinks | `onPathClickCapability` calls `invalidate()` | ✓ WIRED | `TerminalPanel.tsx:641-643` |
| TerminalPanel | relay stream | `onClick: clientRef.current?.sendPathClick` | ✓ WIRED | `TerminalPanel.tsx:1079` |
| App | WebShareSessionView | `pluginConfig={isRemoteWebTab ? peerPluginConfig : …}` | ✓ WIRED | `App.tsx:1341` |
| Peer/browser viewer | Hub.PathClick | share server read pump, write cap only | ✓ WIRED | `webserver/server.go:1097-1103`, behind `requireSameUser` (Phase 14) |
| Local viewer | Hub.PathClick | relay read pump | ✓ WIRED | `relay/server.go:207-212` |
| Hub.PathClick | printed check | `findPrinted` → `findMatch(hasToken)` | ✓ WIRED | refactor shared with Phase 13 `PortPrinted` |
| 12-SECURITY-REVIEW.md | 12-VM-CHECKS.md | verdict cites C-04R, C-05R, C-07R, C-08 | ✓ WIRED | line 172 |

### Behavioral Spot-Checks (run now, targeted only)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Host path click, cwd log, printed check, WR-02 | `GOTOOLCHAIN=go1.26.11 go test -count=1 -run 'PathClick\|DepartedOrRevoked\|Cwd\|OSC7\|FindPrinted' ./internal/relay/ ./internal/webserver/` | ok, ok; 33 PASS, 0 FAIL | ✓ PASS |
| zsh shim | `go test -count=1 -run 'Zsh\|Shim' ./internal/daemon/` | ok; 18 PASS | ✓ PASS |
| previewfile | `go test -count=1 -cover ./internal/previewfile/` | ok, 90.1% | ✓ PASS |
| Viewer | `vitest run` pathLinks.linkifier, pathLinks, WebShareSessionView.peerLinks, App.open-remote, TerminalPanel.pathClick, TerminalPanel.web-links (node v24.22.1, NO_COLOR=1) | 6 files, 129 tests passed | ✓ PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | OK: 228 paths | ✓ PASS |
| Two-Mac behaviour | not re-run (no tailnet or VM use allowed in this pass); 12-VM-CHECKS.md re-run rows carried | 10/10 PASS | carried |

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|-------------|--------|----------|
| PATH-01 | 12-02, 12-03, 12-07, 12-09 | ✓ SATISFIED | Truth 1; code unchanged; shim and cwd tests pass |
| PATH-02 | 12-01, 12-04..12-09 | ✓ SATISFIED | Truths 2-4, 7, 8, 14-21 |

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `frontend/src/lib/pathLinks.ts` | private xterm fields `_lastBufferCell`/`_activeLine` | ℹ️ Info | guarded and pinned by a test (T-12-44); unchanged |
| `frontend/src/lib/__tests__/pathLinks.linkifier.test.ts` | fixed `sleep(100)` render wait | ℹ️ Info | flakes under full-suite load; todo `2026-11-22-linkifier-reconnect-flake.md`; test-only |

### Accepted items (not gaps)

- **F-1** (row-0 relative path resolves against the start directory) and **F-2** (first click after returning to the same cell): accepted as todos (AR-12-07); todo files in `.planning/todos/pending/`.
- **WR-01** (a relative click resolves against the newest print of that text): accepted by the user on 2026-11-22 (AR-12-09 in 12-SECURITY.md). It was pending in the previous report.
- **G-1** (`internal/pty` flake): accepted as unrelated.
- **Manual checks** on the user's Macs: M-131 (desktop underline, dark repaint), M-132 (Claude Code), M-133 (`ssh localhost`) (AR-12-08).
- **Linkifier reconnect test flake** under full-suite load: tracked todo, test timing only; passed in this targeted run.

### Human Verification Required

None outstanding for this verdict. The three rig-limited items were decided by the user at the 12-09 checkpoint and are carried as overrides.

### Gaps Summary

No gaps. Every Phase 12 must-have still holds in the current code. Later phases tightened who can reach the path-click handler (same Tailscale account, Phase 14) and refactored the printed check for reuse (Phase 13) without changing how it treats paths. The targeted Go and vitest suites for path clicks, cwd tracking and the zsh shim pass at HEAD 52d960a5.

---

_Verified: 2026-11-23T04:07:19Z_
_Verifier: Claude (gsd-verifier)_
