---
phase: 12-file-path-click
verified: 2026-11-23T17:17:50Z
status: passed
score: 21/21 must-haves verified (3 by recorded user override)
covered_files: [".planning/phases/12-file-path-click/12-01-PLAN.md", ".planning/phases/12-file-path-click/12-01-SUMMARY.md", ".planning/phases/12-file-path-click/12-02-PLAN.md", ".planning/phases/12-file-path-click/12-02-SUMMARY.md", ".planning/phases/12-file-path-click/12-03-PLAN.md", ".planning/phases/12-file-path-click/12-03-SUMMARY.md", ".planning/phases/12-file-path-click/12-04-PLAN.md", ".planning/phases/12-file-path-click/12-04-SUMMARY.md", ".planning/phases/12-file-path-click/12-05-PLAN.md", ".planning/phases/12-file-path-click/12-05-SUMMARY.md", ".planning/phases/12-file-path-click/12-06-PLAN.md", ".planning/phases/12-file-path-click/12-06-SUMMARY.md", ".planning/phases/12-file-path-click/12-07-PLAN.md", ".planning/phases/12-file-path-click/12-07-SUMMARY.md", ".planning/phases/12-file-path-click/12-08-PLAN.md", ".planning/phases/12-file-path-click/12-08-SUMMARY.md", ".planning/phases/12-file-path-click/12-09-PLAN.md", ".planning/phases/12-file-path-click/12-09-SUMMARY.md", ".planning/phases/12-file-path-click/12-REVIEW-FIX.md", ".planning/phases/12-file-path-click/12-REVIEW.md", ".planning/phases/12-file-path-click/12-SECURITY-REVIEW.md", ".planning/phases/12-file-path-click/12-SECURITY.md", ".planning/phases/12-file-path-click/12-VM-CHECKS.md", "TESTING.md", "frontend/src/App.tsx", "frontend/src/components/TerminalPanel.tsx", "frontend/src/components/WebShareSessionView.tsx", "frontend/src/components/__tests__/App.open-remote.test.tsx", "frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx", "frontend/src/components/__tests__/WebShareSessionView.peerLinks.test.tsx", "frontend/src/lib/__tests__/pathLinks.linkifier.test.ts", "frontend/src/lib/pathLinks.ts", "frontend/src/lib/relayClient.ts", "internal/daemon/engine.go", "internal/daemon/types.go", "internal/daemon/zshshim.go", "internal/daemon/zshshim.zsh", "internal/previewfile/previewfile.go", "internal/relay/cwd.go", "internal/relay/cwd_test.go", "internal/relay/history.go", "internal/relay/hub.go", "internal/relay/pathclick.go", "internal/relay/pathclick_test.go", "internal/relay/server.go", "internal/webserver/plugin_config.go", "internal/webserver/server.go"]
covered_digest: "v4:sha256:9ef88be3ee68283ff38553cd737188c369628ec642e0c0fc19e33b96cdc260b1"
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
  reason: "stale fingerprint: Phase 15 (v1.2 Hardening) edited covered files after the 2026-11-23T04:07:19Z verification (HEAD 52d960a5)"
  gaps_closed: []
  gaps_remaining: []
  regressions: []
  resolved_by_later_phases:
    - "F-1 (row-0 relative path after `clear` resolved against the start directory): fixed in Phase 15-04 (dc81e940). The ED 3 handler in internal/relay/hub.go now calls History.Skip(emu.ScrollbackLen()) so absolute rows stay monotonic."
    - "F-2 (first Cmd-click after leaving and returning to the same cell sent the old path): fixed in Phase 15-04 (a1143a00). pathLinks.ts mouseleave now calls invalidate()."
    - "Linkifier reconnect test flake: fixed in Phase 15-04 (21099b07). The test file now waits with vi.waitFor on events."
---

# Phase 12: File Path Click Verification Report

**Phase Goal:** A user can Cmd-click a file path that a program printed in the terminal and see it previewed, in local and remote sessions alike, because Portty tracks each shell's working directory.
**Verified:** 2026-11-23T17:17:50Z (HEAD c818e1e1)
**Status:** passed
**Re-verification:** Yes. The previous report (passed, 21/21, 2026-11-23T04:07:19Z at 52d960a5) went stale because Phase 15 edited files it covers. I diffed every covered file between 52d960a5 and HEAD, re-read the changed code, and re-ran targeted tests only.

Phase 15 touched Phase 12's area in three planned ways (15-04) and two incidental ways. All three planned changes hold and close the two accepted findings from 12-09 (F-1, F-2) plus the test flake. Nothing else in Phase 12 regressed.

## What Phase 15 changed in covered files since the last verification

| File | Change (commit) | Effect on Phase 12 |
|------|-----------------|--------------------|
| `internal/relay/history.go` | `History.Skip(n)` counts lines dropped before reaching history; `Clear` documented as leaving `pushed` alone (dc81e940, 15-04) | New method at `:79`. Row numbering stays monotonic. `Pushed()`/`FirstRow()` unchanged |
| `internal/relay/hub.go` | ED 3 (`CSI 3 J`) handler: `h.history.Clear()` then `h.history.Skip(emu.ScrollbackLen())` (`:1305`) (dc81e940) | F-1 fix. Without it, output after `clear` reused rows that older cwd records covered. `Hub.PathClick`, `cwdLog.at` and `pathclick.go` untouched, as 15-04 intended |
| `frontend/src/lib/pathLinks.ts` | `onLeave` now `pointerInside = false; invalidate()`, registered after `invalidate` is defined (`:152-153`) (a1143a00) | F-2 fix. A mouseleave drops xterm's same-cell memory, so returning to the same cell asks the provider again. Dispose still removes the listener (`:191`) |
| `frontend/src/lib/__tests__/pathLinks.linkifier.test.ts` | fixed sleeps replaced by `vi.waitFor` (`:96,164,167,199`); new F-2 test at `:229`; one 20 ms sleep kept at `:265` for the no-input window (21099b07) | The 12-09 flake is gone. The remaining sleep asserts that nothing happens, which no event can mark |
| `internal/relay/cwd_test.go`, `pathclick_test.go` | `TestCwd_ClearKeepsRowsMonotonic`, `TestCwd_ScrollbackEraseKeepsRows`, `TestPathClick_RelativeAfterClear` | New regression tests for F-1 |
| `frontend/src/components/TerminalPanel.tsx` | `onSessionEnded` prop and ref; close reason `session-ended` calls it (15-05) | Peer-tab reconnect verdict only. `onPathClickCapability` still calls `pathLinksRef.current?.invalidate()` (`:646-649`); `registerPathLinks` at `:1082` unchanged |
| `frontend/src/App.tsx` | `update:show` listener (15-06); `sawEnded` flag on peer tab reconnect (15-05) | Unrelated to path click. `peerPluginConfig` (`:153`) still feeds remote tabs (`:1352`) |
| `frontend/src/components/WebShareSessionView.tsx` | passes `onSessionEnded` through (15-05) | `isWebGuest = pluginConfig === undefined && remote !== true` (`:194`) unchanged |
| `internal/daemon/engine.go` | doc comment only on `closeSessionFiles` | none |
| `TESTING.md` | Phase 15 Note and rows | Section 4 rows intact; traceability gate passes (228 paths) |

No change since the last verification in `internal/relay/pathclick.go`, `cwd.go`, `internal/relay/server.go`, `internal/webserver/server.go`, `plugin_config.go`, `daemon/types.go`, `zshshim.go`, `zshshim.zsh`, `previewfile.go`, `relayClient.ts`, or the `peerLinks` and `pathClick` panel test files.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: a new zsh session reports cwd via OSC 7 after every `cd` with no setup; other shells that emit OSC 7 are tracked; without OSC 7 an absolute path works and a relative one does nothing (PATH-01) | ✓ VERIFIED | Shim, engine and `cwd.go` logic unchanged. `go test -count=1 -run 'Zsh\|Shim' ./internal/daemon/` ok; relay `Cwd\|OSC7` ok. C-08 and U-01/U-01R carried |
| 2 | SC2: a Cmd-click on an absolute or relative path opens a preview in a local tab AND in a home-VM peer tab over the real tailnet (PATH-02) | ✓ VERIFIED | `TerminalPanel.tsx:1082`, `App.tsx:153,1352`, `WebShareSessionView.tsx:194`. `peerLinks` and `App.open-remote` pass now. VM C-01R, C-02R, C-03R carried: the Phase 15 changes do not alter the path those runs exercised |
| 3 | SC3: a missing path does nothing; a plain click selects; URLs keep their behaviour | ✓ VERIFIED | `TerminalPanel.pathClick`, `TerminalPanel.web-links`, `pathLinks` suites pass now. C-01R, C-04R carried |
| 4 | SC4: the request rides the session's stream, resolves against the session's own cwd, only printed paths; the security review records it | ✓ VERIFIED | `pathclick.go` unchanged. Read pumps `relay/server.go:207-212` and `webserver/server.go:1097-1103` still drop read-only. `TestPathClick_*` pass. `12-SECURITY-REVIEW.md` verdict APPROVED; `12-SECURITY.md` `threats_open: 0`. F-1's fix only changes row numbering, so the printed-token check and cwd-at-print resolution use the same inputs |
| 5 | The full gate passes once on the host | ✓ VERIFIED (carried, refreshed) | 12-VM-CHECKS.md re-run gate. Orchestrator evidence at HEAD: Go `-race` 14/14 packages ok; vitest 122 files / 1826 tests. Not re-run here |
| 6 | `ssh localhost` works inside a shimmed session (C-08 clause) | PASSED (override) | User decision UI-1 at the 12-09 checkpoint; M-133 |
| 7 | After a window resize a printed path is still clickable (C-09) | ✓ VERIFIED | `pathLinks.ts:151` `term.onResize(invalidate)`; the real-xterm resize tests pass now; C-09R. The new mouseleave hook shares the same `invalidate` and adds no second reset path |
| 8 | After an upgrade, a pre-upgrade relative path is clickable at once in every surviving viewer; all reconnect with `mode=delta` (U-01) | ✓ VERIFIED | `TerminalPanel.tsx:646-649`; real-xterm reconnect tests pass now; U-01R. `TestHandoffState_CwdRoundTrip`, `_CwdOldShape`, `_CwdValidates` pass now: `History.Skip` changes `pushed`, which the handoff already carries |
| 9 | TESTING.md Category BB M-128..M-135 and the security review verdict are written | ✓ VERIFIED | Category BB still in TESTING.md; verdict in `12-SECURITY-REVIEW.md` |
| 10 | Cmd-held underline and pointer in the desktop app (C-06, UI-SPEC) | PASSED (override) | User decision UI-2; Safari half PASS (C-06R); M-131 |
| 11 | A live light/dark switch repaints the notice with no reload (backstop) | PASSED (override) | User decision UI-2; Safari PASS (C-06R); M-131 |
| 12 | A Cmd-click in a mouse-reporting TUI previews (backstop) | ✓ VERIFIED | C-07R; the real-xterm "sends no input to a program with mouse tracking on" test passes now, and the `onLeave` `invalidate()` dispatches the same non-bubbling event the earlier path did |
| 13 | Every FAIL reaches the checkpoint; nothing is concluded before the user answers | ✓ VERIFIED | 12-VM-CHECKS.md "Task 3 decisions", 12-09-SUMMARY |
| 14 | Click, missing-file silence, wrong-type notice and read-only refusal across local tab, peer tab and browser link (backstop) | ✓ VERIFIED | C-04R, C-05R; `TestPathClick_*` relay and webserver tests pass now |
| 15 | Real xterm 6 + real RelayClient: Cmd-clickable at once after a 1012 close and delta replay (12-08) | ✓ VERIFIED | `pathLinks.linkifier.test.ts` passes now, with event-based waits |
| 16 | Real xterm 6: after a grid resize a same-cell Cmd-click sends the path click and opens a URL (12-08) | ✓ VERIFIED | resize tests in the same file pass now |
| 17 | Each reproduction failed before its fix (12-08) | ✓ VERIFIED (carried) | RED commits 523fb5d5, 1823a698; history unchanged. 15-VERIFICATION records mutation checks showing the F-1 and F-2 tests fail with their fixes removed |
| 18 | The link refresh never sends input; a read-only viewer gets no link (12-08) | ✓ VERIFIED | `pathLinks.ts` refresh event keeps `bubbles: false`; tests pass now; C-05R |
| 19 | A desktop peer tab registers path and URL links from this Mac's settings, makes no cross-origin request, turns on no other addon (12-08) | ✓ VERIFIED | `App.tsx:153`; `WebShareSessionView.peerLinks`, `App.open-remote` pass now (App.open-remote gained a Phase 15 case, still passing) |
| 20 | The browser viewer still self-fetches; no CORS header; CSPs unchanged by Phase 12 (12-08) | ✓ VERIFIED | `isWebGuest` self-fetch path unchanged; no Phase 15 change to `plugin_config.go` or the CSP |
| 21 | TESTING.md lists the two new test files in Sections 2 and 4; the traceability gate passes (12-08) | ✓ VERIFIED | `bash tests/check-traceability-paths.sh` printed `OK: 228 traceability paths exist` |

**Score:** 21/21 truths verified (18 verified, 3 passed by override; 0 present but behavior-unverified)

Phase 15 confirmations the caller asked for:

| Finding | Status | Evidence |
|---------|--------|----------|
| F-1: row-0 relative path after `clear` | ✓ RESOLVED | `hub.go:1298-1307` calls `History.Clear()` then `History.Skip(emu.ScrollbackLen())` on ED 3, before `h.wraps.clear()`. `history.go:79` adds the counter to `pushed`. `TestPathClick_RelativeAfterClear`, `TestCwd_ClearKeepsRowsMonotonic`, `TestCwd_ScrollbackEraseKeepsRows` PASS (`-v`, `-count=1`). A behaviour test (a real hub fed `cd proj; clear; ls`), so presence alone is not the evidence |
| F-2: first click after leave and return to the same cell | ✓ RESOLVED | `pathLinks.ts:152` `onLeave` calls `invalidate()`; test "after leaving and returning to the same cell with changed text, the first Cmd-click sends the new path" PASS in a real xterm Linkifier (single named test run, 334 ms) |
| Linkifier test uses `vi.waitFor` | ✓ RESOLVED | four `vi.waitFor` calls with 5 s timeouts at `:96,164,167,199`; one deliberate 20 ms sleep at `:265` for an assertion that nothing happened |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/previewfile/previewfile.go` | shared file checks, leaf package | ✓ VERIFIED | `go test -cover` ok, 90.1% |
| `internal/daemon/zshshim.go`, `zshshim.zsh` | ZDOTDIR shim emitting OSC 7 | ✓ VERIFIED | unchanged; shim tests pass |
| `internal/relay/cwd.go`, `history.go`, `hub.go` | OSC 7 parse, row-keyed cwd log, monotonic rows | ✓ VERIFIED | cwd and handoff tests pass; `Skip` wired in the ED 3 handler |
| `internal/relay/pathclick.go` | printed-token check, resolution, notices | ✓ VERIFIED | unchanged |
| `internal/relay/server.go`, `internal/webserver/server.go` | read pump arms, read-only dropped | ✓ VERIFIED | `:207-212`, `:1097-1103` |
| `frontend/src/lib/pathLinks.ts` | link provider, invalidate, mouseleave reset | ✓ VERIFIED | `:134-153` |
| `frontend/src/components/TerminalPanel.tsx` | registers path links, capability calls `invalidate()` | ✓ VERIFIED | `:646-649`, `:1082`; no `registerLinkProvider` in the panel |
| `TESTING.md` | Sections 2, 4, Category BB | ✓ VERIFIED | traceability gate passes |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| TerminalPanel | pathLinks | `onPathClickCapability` calls `invalidate()` | ✓ WIRED | `TerminalPanel.tsx:646-649` |
| TerminalPanel | relay stream | `onClick: clientRef.current?.sendPathClick` | ✓ WIRED | `TerminalPanel.tsx:1085` |
| pathLinks | xterm Linkifier | `mouseleave` and `onResize` both call `invalidate` | ✓ WIRED | `pathLinks.ts:151-153` |
| App | WebShareSessionView | `pluginConfig={isRemoteWebTab ? peerPluginConfig : …}` | ✓ WIRED | `App.tsx:1352` |
| Peer/browser viewer | Hub.PathClick | share server read pump, write cap only | ✓ WIRED | `webserver/server.go:1097-1103` |
| Local viewer | Hub.PathClick | relay read pump | ✓ WIRED | `relay/server.go:207-212` |
| ED 3 handler | row numbering | `History.Clear` then `History.Skip(ScrollbackLen)` | ✓ WIRED | `hub.go:1303-1305`; row used at `hub.go:1382` |
| 12-SECURITY-REVIEW.md | 12-VM-CHECKS.md | verdict cites C-04R, C-05R, C-07R, C-08 | ✓ WIRED | unchanged |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| cwd lookup in `Hub.PathClick` | absolute row of the printed text | `History.Pushed() + ScrollbackLen() + cursor.Y` (`hub.go:1382`), now monotonic across ED 3 | Yes (test: `TestPathClick_RelativeAfterClear`) | ✓ FLOWING |
| path link text | `logicalLine(term, y)` over the live xterm buffer | real xterm buffer | Yes (real-xterm linkifier tests) | ✓ FLOWING |

### Behavioral Spot-Checks (run now, targeted only, `-count=1`)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Relay and share-server path click, cwd log, printed check, ED 3 rows, history, WR-02 | `GOTOOLCHAIN=go1.26.11 go test -count=1 -run 'PathClick\|TestCwd\|OSC7\|FindPrinted\|DepartedOrRevoked\|ClearKeepsRows\|ScrollbackErase\|History' ./internal/relay/ ./internal/webserver/` | ok, ok | ✓ PASS |
| F-1 named tests and handoff cwd | `go test -count=1 -v -run 'TestPathClick_RelativeAfterClear\|TestCwd_ClearKeepsRowsMonotonic\|TestCwd_ScrollbackEraseKeepsRows\|TestHandoffState_Cwd' ./internal/relay/` | 6 PASS, 0 FAIL | ✓ PASS |
| zsh shim | `go test -count=1 -run 'Zsh\|Shim' ./internal/daemon/` | ok | ✓ PASS |
| previewfile | `go test -count=1 -cover ./internal/previewfile/` | ok, 90.1% | ✓ PASS |
| Viewer suites | `vitest run` pathLinks.linkifier, pathLinks, WebShareSessionView.peerLinks, App.open-remote, TerminalPanel.pathClick, TerminalPanel.web-links (NO_COLOR=1) | 6 files, 133 tests passed | ✓ PASS |
| F-2 single test | `vitest run pathLinks.linkifier.test.ts -t "leaving and returning"` | 1 passed, 9 skipped | ✓ PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | OK: 228 paths | ✓ PASS |
| Two-Mac behaviour | not re-run (no tailnet or VM use in this pass); 12-VM-CHECKS.md re-run rows carried | 10/10 PASS | carried |

### Probe Execution

SKIPPED: the phase declares no `probe-*.sh` scripts; its runnable checks are the named Go and vitest tests above.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PATH-01 | 12-02, 12-03, 12-07, 12-09 | The terminal knows each session's working directory through OSC 7; zsh setup emits it; other shells work | ✓ SATISFIED | Truth 1; cwd log now also monotonic across `clear` (F-1). REQUIREMENTS.md: `[x]`, traceability "Phase 12 / Complete" |
| PATH-02 | 12-01, 12-04..12-09 | Cmd-click a printed path previews it, local and remote; host previews only paths the session printed; missing paths do nothing | ✓ SATISFIED | Truths 2-4, 7, 8, 12, 14-21. REQUIREMENTS.md: `[x]`, "Phase 12 / Complete". No orphaned requirement: the ROADMAP lists only PATH-01 and PATH-02 for Phase 12 |

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `frontend/src/lib/pathLinks.ts` | private xterm fields `_lastBufferCell`/`_activeLine` | ℹ️ Info | guarded and pinned by a test (T-12-44); unchanged in kind |
| `frontend/src/lib/__tests__/pathLinks.linkifier.test.ts` | one `sleep(20)` at `:265` | ℹ️ Info | deliberate no-input window; nothing marks "nothing happened" |

No `TBD`, `FIXME`, `XXX` or `TODO` markers were added by Phase 15 in the four Phase 12 files it edited.

### Accepted items (not gaps)

- **F-1, F-2 and the linkifier flake**: were accepted todos (AR-12-07); now resolved by Phase 15-04. No longer outstanding.
- **WR-01** (a relative click resolves against the newest print of that text): accepted by the user on 2026-11-22 (AR-12-09 in `12-SECURITY.md`). Unchanged.
- **G-1** (`internal/pty` flake): accepted as unrelated; Phase 15-02 later fixed it.
- **Manual checks** on the user's Macs: M-131 (desktop underline, dark repaint), M-132 (Claude Code), M-133 (`ssh localhost`) (AR-12-08).
- **Human-verification history**: the 12-09 VM runs (C-01R..C-09R, U-01R) stay verified. Phase 15 changed row numbering after `clear` and the mouseleave reset; neither alters the flows those runs checked (local, peer, Safari clicks, resize, upgrade), and both are covered by the new tests above.

### Human Verification Required

None outstanding for this verdict. The three rig-limited items were decided by the user at the 12-09 checkpoint and are carried as overrides.

### Gaps Summary

No gaps. Every Phase 12 must-have still holds at HEAD c818e1e1. Phase 15-04 fixed the row-0 relative-path-after-`clear` problem in the row numbering (`History.Skip` in the ED 3 handler), made the first Cmd-click after leaving and returning to a cell send the new path (`onLeave` calls `invalidate()`), and made the linkifier tests wait on events. The targeted Go and vitest suites for path clicks, cwd tracking, handoff, and the zsh shim pass.

---

_Verified: 2026-11-23T17:17:50Z_
_Verifier: Claude (gsd-verifier)_
