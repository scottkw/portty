---
phase: 12-file-path-click
verified: 2026-11-22T13:54:43Z
status: passed
score: 21/21 must-haves verified (3 by recorded user override)
covered_files: [".planning/phases/12-file-path-click/12-01-PLAN.md", ".planning/phases/12-file-path-click/12-01-SUMMARY.md", ".planning/phases/12-file-path-click/12-02-PLAN.md", ".planning/phases/12-file-path-click/12-02-SUMMARY.md", ".planning/phases/12-file-path-click/12-03-PLAN.md", ".planning/phases/12-file-path-click/12-03-SUMMARY.md", ".planning/phases/12-file-path-click/12-04-PLAN.md", ".planning/phases/12-file-path-click/12-04-SUMMARY.md", ".planning/phases/12-file-path-click/12-05-PLAN.md", ".planning/phases/12-file-path-click/12-05-SUMMARY.md", ".planning/phases/12-file-path-click/12-06-PLAN.md", ".planning/phases/12-file-path-click/12-06-SUMMARY.md", ".planning/phases/12-file-path-click/12-07-PLAN.md", ".planning/phases/12-file-path-click/12-07-SUMMARY.md", ".planning/phases/12-file-path-click/12-08-PLAN.md", ".planning/phases/12-file-path-click/12-08-SUMMARY.md", ".planning/phases/12-file-path-click/12-09-PLAN.md", ".planning/phases/12-file-path-click/12-09-SUMMARY.md", ".planning/phases/12-file-path-click/12-REVIEW-FIX.md", ".planning/phases/12-file-path-click/12-REVIEW.md", ".planning/phases/12-file-path-click/12-SECURITY-REVIEW.md", ".planning/phases/12-file-path-click/12-SECURITY.md", ".planning/phases/12-file-path-click/12-VM-CHECKS.md", "TESTING.md", "frontend/src/App.tsx", "frontend/src/components/TerminalPanel.tsx", "frontend/src/components/WebShareSessionView.tsx", "frontend/src/components/__tests__/App.open-remote.test.tsx", "frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx", "frontend/src/components/__tests__/WebShareSessionView.peerLinks.test.tsx", "frontend/src/lib/__tests__/pathLinks.linkifier.test.ts", "frontend/src/lib/pathLinks.ts", "frontend/src/lib/relayClient.ts", "internal/daemon/engine.go", "internal/daemon/zshshim.go", "internal/daemon/zshshim.zsh", "internal/previewfile/previewfile.go", "internal/relay/cwd.go", "internal/relay/hub.go", "internal/relay/pathclick.go", "internal/relay/server.go", "internal/webserver/plugin_config.go", "internal/webserver/server.go"]
covered_digest: "v4:sha256:2353be2a6352822a342cedab732ba9b41d39c912b0a21439e3b370932e304de7"
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
  previous_status: gaps_found
  previous_score: 6/14
  gaps_closed:
    - "Cmd-clicking a printed path opens it in a preview tab in a home-VM peer tab on an office-VM session over the real tailnet (SC2), and the click, missing-file silence and wrong-type notice work in a desktop peer tab (C-02, C-04 peer clause)"
    - "A viewer that lives through a reconnect, an upgrade handoff or a grid change keeps clickable path links (U-01, C-09)"
    - "TESTING.md has Category BB with M-128..M-135 whose last-run lines point at 12-VM-CHECKS.md, and 12-SECURITY-REVIEW.md has its VM evidence and a verdict citing C-04, C-05, C-07 and C-08"
  gaps_remaining: []
  regressions: []
---

# Phase 12: File Path Click Verification Report

**Phase Goal:** A user can Cmd-click a file path that a program printed in the terminal and see it previewed, in local and remote sessions alike, because Portty tracks each shell's working directory.
**Verified:** 2026-11-22T13:54:43Z
**Status:** passed
**Re-verification:** Yes, after gap closure (12-08 code fixes, 12-09 VM re-run and user checkpoint)

All three gaps from the first verification are closed in code, in tests I ran myself, and on real machines (VM rows C-02R, C-04R, C-09R and U-01R). Three must-haves pass only by the user's recorded decision to check them on their own Macs, because the VM rig cannot observe them (M-131, M-133). Claude Code inside a session is the same kind of manual check (M-132), but it was never a must-have.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: a new zsh session reports cwd via OSC 7 after every `cd` with no setup; other shells that emit OSC 7 are tracked; without OSC 7 an absolute path works and a relative one does nothing (PATH-01) | ✓ VERIFIED | Regression check: `zshshim.go`/`.zsh`, `cwd.go` and `engine.go` have no change since the first verification (no Go commit since 81ada526 except 30ac9abb in `pathclick.go`). C-08 shell matrix carried. U-01 no-shim session: absolute `preview`, relative `no-cwd`. U-01R: the cwd log survives C to C2 |
| 2 | SC2: a Cmd-click on an absolute or relative path opens a preview in a local tab AND in a home-VM peer tab over the real tailnet (PATH-02) | ✓ VERIFIED | Code: `App.tsx:143-149` `peerPluginConfig` is passed at `:1323` for remote tabs; `WebShareSessionView.tsx:159` `isWebGuest = pluginConfig === undefined && remote !== true`; `TerminalPanel.tsx:867,938` registers path links. VM: C-02R `path click … conn=2 outcome=preview` from the home peer tab, home switched, office got a background tab; C-01R local; C-03R Safari |
| 3 | SC3: a missing path does nothing; a plain click selects; URLs keep their behaviour | ✓ VERIFIED | C-01R (plain click inert, URL popover); C-04R missing and folder `outcome=missing` with the tail sha256 unchanged, now including the peer tab as clicker; C-02R a peer-tab URL opens the confirm popover |
| 4 | SC4: the request rides the session's stream, resolves against the session's own cwd, only printed paths; the security review records it | ✓ VERIFIED | `pathclick.go:57-69` refuses nil, read-only, bad payload, departed or revoked subscribers (WR-02 fix 30ac9abb) and a busy hub. C-05R: read-only viewer gets no link and its hand-sent 0x13 leaves no log line. `12-SECURITY-REVIEW.md:172` **Verdict:** APPROVED. `12-SECURITY.md` threats_open 0 |
| 5 | The full gate passes once on the host | ✓ VERIFIED | 12-VM-CHECKS.md re-run gate: vitest 108/1600, tsc, brand, share-removal, traceability, build-script 40/0, 13 of 14 Go packages ok. One `internal/pty` `TestKillClean_NoOrphans` flake under load: that package has no diff since 81ada526, it passed alone 5/5, and the user accepted it as G-1. My checks: 6 gap vitest files 129/129, `go test -run 'PathClick\|DepartedOrRevoked'` relay and webserver ok, traceability OK (199 paths) |
| 6 | `ssh localhost` works inside a shimmed session (C-08 clause) | PASSED (override) | Override: rig limit, user chose "Check on my Macs" (UI-1), accepted on 2026-11-22. Manual check M-133 |
| 7 | After a window resize a printed path is still clickable (C-09) | ✓ VERIFIED | `pathLinks.ts` `term.onResize(invalidate)`. Real-xterm tests "after a resize…" (3) pass. C-09R: narrow, widen (the click that failed in the first run), peer grid claim and restore each `outcome=preview` on the first same-point click |
| 8 | After an upgrade, a pre-upgrade relative path is clickable at once in every surviving viewer; all reconnect with `mode=delta` (U-01) | ✓ VERIFIED | `TerminalPanel.tsx:538-541` calls `invalidate()` on every capability report (`relayClient.ts:398,525,550`). Real-xterm "after a reconnect…" tests pass. U-01R: 3/3 `mode=delta` within 1.62 s, 0 grid lines, first clicks in office, peer tab and the same Safari page all `outcome=preview` |
| 9 | TESTING.md Section 5 Category BB M-128..M-135 and the security review verdict are written | ✓ VERIFIED | `TESTING.md:1611` `### Category BB`, M-128..M-135 at 1615-1662, 10 references to 12-VM-CHECKS.md; `12-SECURITY-REVIEW.md` has `## Gap closure (12-08)` (T-12-40..45) and `## VM evidence and verdict` citing C-04R, C-05R, C-07R and C-08 |
| 10 | Cmd-held underline and pointer in the desktop app (C-06, UI-SPEC) | PASSED (override) | Override: Wails web view unobservable in the VMs; Safari half PASS (C-06R); user chose "Check on my Macs" (UI-2), accepted on 2026-11-22. M-131 |
| 11 | A live light/dark switch repaints the notice with no reload (backstop) | PASSED (override) | Override: Safari PASS (C-06R, 0.9 s, same conn); app colours not readable in the VM; user UI-2, accepted on 2026-11-22. M-131 |
| 12 | A Cmd-click in a mouse-reporting TUI previews (backstop) | ✓ VERIFIED | C-07R vim `mouse=a` `outcome=preview`, 0 SGR reports, clean quit; real-xterm test "the link refresh sends no input to a program with mouse tracking on" passes |
| 13 | Every FAIL reaches the checkpoint; nothing is concluded before the user answers | ✓ VERIFIED | 12-09 Task 3 answer recorded in 12-VM-CHECKS.md "Task 3 decisions" and 12-09-SUMMARY; first-run FAIL rows carry `Superseded by …R` markers |
| 14 | Click, missing-file silence, wrong-type notice and read-only refusal checked across local tab, peer tab and browser link (backstop) | ✓ VERIFIED | C-04R (office, Safari and peer tab as clickers; notices only in the clicker; silence everywhere), C-05R read-only |
| 15 | In a real xterm 6 Terminal fed by a real RelayClient, a path under a resting pointer is Cmd-clickable at once after a 1012 close and a delta replay meta (12-08) | ✓ VERIFIED | `pathLinks.linkifier.test.ts` imports the real `Terminal`, `WebLinksAddon` and `RelayClient` (0 `vi.mock`); both reconnect tests pass in my run |
| 16 | In a real xterm 6 Terminal, after a host grid resize a same-cell Cmd-click sends the path click and opens a URL, also after the pointer left and came back (12-08) | ✓ VERIFIED | Three "after a resize" tests, URL case included, pass in my run |
| 17 | Each reproduction failed before its fix (12-08) | ✓ VERIFIED | RED commits 523fb5d5 and 1823a698 each change only `pathLinks.linkifier.test.ts` (git show --stat). The fixes landed in separate commits (8fd350e6, 290e5860). The first VM run is independent evidence of the bug before the fix (C-09, U-01 FAIL) |
| 18 | The link refresh never sends input; a read-only viewer gets no link (12-08) | ✓ VERIFIED | `pathLinks.ts` dispatches with `bubbles: false`; `provideLinks` returns nothing while `enabled()` is false. Tests "…sends no input…" and "while the capability is false…" pass; C-05R |
| 19 | A desktop peer tab registers path and URL links from this Mac's toggle and modifier, sends a modifier-click's path, makes no cross-origin request, and turns on no other addon (12-08) | ✓ VERIFIED | `WebShareSessionView.peerLinks.test.tsx` and `App.open-remote.test.tsx` pass ("fetches nothing cross-origin", "turns on no other addon"); `createFrom` passes only `webLinks`/`webLinksConfig` |
| 20 | The browser viewer still self-fetches; no CORS header; CSPs unchanged; no Go change in 12-08 (12-08) | ✓ VERIFIED | `grep Access-Control-Allow internal/webserver` (non-test): none; `git diff 81ada526 -- frontend/index.html internal/webserver/csp_mw.go` empty; the only Go commit since 81ada526 is the 12-REVIEW WR-02 fix (30ac9abb), not 12-08; test "the browser viewer still reads the host config from its own origin" passes |
| 21 | TESTING.md lists the two new test files in Sections 2 and 4; the traceability gate passes (12-08) | ✓ VERIFIED | `TESTING.md:36` gap-closure note, `:811-812` PATH-02 rows; `check-traceability-paths.sh` exit 0 |

**Score:** 21/21 truths verified (18 verified, 3 passed by override; 0 present but behavior-unverified)

### Gap closure (previous gaps)

| Gap | Previous status | Now | Evidence |
|-----|-----------------|-----|----------|
| 1. Desktop peer tabs get no path links (C-02, C-04 peer) | failed | closed | The peer tab gets this Mac's link settings and never makes a cross-origin fetch (code + mounted peer-tab test). C-02R and C-04R peer clauses PASS on the tailnet |
| 2. Surviving viewers lose links after reconnect or resize (U-01, C-09) | failed | closed | `invalidate()` runs on every capability report and every resize; the real-xterm Linkifier reproductions failed at RED and pass now. U-01R and C-09R PASS |
| 3. TESTING.md Category BB and security verdict missing | failed | closed | Category BB M-128..M-135 and the APPROVED verdict are written |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `frontend/src/lib/pathLinks.ts` | `PathLinksHandle.invalidate()`, onResize, non-bubbling re-hover | ✓ VERIFIED | Substantive (guarded private-field reset, microtask, mouseleave tracking); wired from TerminalPanel |
| `frontend/src/components/TerminalPanel.tsx` | the capability callback calls `invalidate()` | ✓ VERIFIED | `:540` `pathLinksRef.current?.invalidate()` |
| `frontend/src/App.tsx` | peer tabs get this Mac's link settings only | ✓ VERIFIED | `:143-149`, `:1323` |
| `frontend/src/components/WebShareSessionView.tsx` | no self-fetch for remote tabs | ✓ VERIFIED | `:159` |
| `frontend/src/lib/__tests__/pathLinks.linkifier.test.ts` | real-xterm reproductions | ✓ VERIFIED | 9 tests, unmocked xterm, all pass |
| `frontend/src/components/__tests__/WebShareSessionView.peerLinks.test.tsx` | mounted peer-tab test | ✓ VERIFIED | passes |
| `internal/relay/pathclick.go` | Hub.PathClick with the writer, joined and not-revoked gate | ✓ VERIFIED | `:57-69`; `TestPathClick_DepartedOrRevokedRefused` passes |
| `.planning/phases/12-file-path-click/12-VM-CHECKS.md` | re-run rows C-00R..C-07R, C-09R, U-01R | ✓ VERIFIED | all PASS with log lines; cleanup; Task 3 decisions |
| `TESTING.md` | Sections 2, 4, 5 for Phase 12 | ✓ VERIFIED | Category BB present; the traceability gate passes |
| `.planning/phases/12-file-path-click/12-SECURITY-REVIEW.md` | gap-closure section and verdict | ✓ VERIFIED | `**Verdict:** APPROVED` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| TerminalPanel | pathLinks | `onPathClickCapability` calls `invalidate()` | ✓ WIRED | `TerminalPanel.tsx:538-541`; `relayClient.ts:398,525,550` call it on close, abandon and every replay meta |
| pathLinks | xterm Linkifier | `_lastBufferCell` and `_activeLine` reset plus a non-bubbling mousemove | ✓ WIRED | pinned by "xterm 6 still has the Linkifier fields the refresh resets" |
| App | WebShareSessionView | `pluginConfig={isRemoteWebTab ? peerPluginConfig : …}` | ✓ WIRED | `App.tsx:1323` |
| Desktop peer tab | registerPathLinks | `pluginConfig.webLinks` from this Mac | ✓ WIRED | Previously NOT_WIRED; C-02R |
| Peer/browser/local viewer | Hub.PathClick | 0x13 on the session stream | ✓ WIRED | C-01R conn=1, C-02R conn=2, C-03R conn=4 |
| 12-SECURITY-REVIEW.md | 12-VM-CHECKS.md | the verdict cites C-04R, C-05R, C-07R and C-08 | ✓ WIRED | line 172 |
| TESTING.md M-128..M-135 | 12-VM-CHECKS.md | last-run lines | ✓ WIRED | 10 references |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| TerminalPanel (local tab) | `pluginConfig.webLinks` | App state from the Wails binding | yes | ✓ FLOWING |
| TerminalPanel (browser) | `pluginConfig.webLinks` | same-origin `/api/plugin-config` | yes | ✓ FLOWING |
| TerminalPanel (desktop peer tab) | `pluginConfig.webLinks` | `peerPluginConfig` from this Mac's `pluginConfig` state | yes | ✓ FLOWING (was DISCONNECTED) |
| pathLinks `enabled()` | `pathClickEnabledRef` | replay meta `pathClick` on every join | yes | ✓ FLOWING; xterm cache now refreshed |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Gap tests and regressions | `pnpm vitest run` pathLinks.linkifier, WebShareSessionView.peerLinks, App.open-remote, TerminalPanel.pathClick, pathLinks, TerminalPanel.web-links | 6 files, 129 tests passed | ✓ PASS |
| Host path-click, including WR-02 | `go test -count=1 -run 'PathClick\|DepartedOrRevoked' ./internal/relay/ ./internal/webserver/` | ok, ok | ✓ PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | OK: 199 paths | ✓ PASS |
| Two-Mac behaviour | not re-run by me; 12-VM-CHECKS.md re-run rows used as evidence | 10/10 PASS | see truths |

### Probe Execution

Not applicable. The phase declares no `scripts/*/tests/probe-*.sh`. Its runnable evidence is the VM rows and the named tests above.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PATH-01 | 12-02, 12-03, 12-07, 12-09 | cwd via OSC 7; zsh emits it; other shells that emit it work | ✓ SATISFIED | Truth 1; C-08 (ssh clause by override); U-01/U-01R cwd log carried |
| PATH-02 | 12-01, 12-04, 12-05, 12-06, 12-07, 12-08, 12-09 | Cmd-click previews printed paths in local and remote sessions alike; only printed paths; missing does nothing | ✓ SATISFIED | Truths 2-4, 7, 8, 14-21; C-01R..C-05R, C-09R, U-01R |

No orphaned requirements: REQUIREMENTS.md maps only PATH-01 and PATH-02 to Phase 12, and both are claimed by plans. REQUIREMENTS.md still marks both "Pending"; marking them complete is left to the orchestrator.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| phase-modified source files | - | TBD/FIXME/XXX/TODO/HACK | none found | - |
| `frontend/src/lib/pathLinks.ts` | invalidate | private xterm fields `_core.linkifier._lastBufferCell/_activeLine` | ℹ️ Info | Guarded (no throw) and pinned by a test that fails if an xterm upgrade renames them (T-12-44) |
| `internal/relay/pathclick.go` | 57-69 | WR-02 fix landed after VM build C (3a8972a6) | ℹ️ Info | A tightening (more refusals), covered by `TestPathClick_DepartedOrRevokedRefused`, which I ran. Not exercised in the VMs |

### Accepted and pending items (not gaps)

- **F-1** (a relative path printed on screen row 0 of a never-scrolled session resolves against the start directory) and **F-2** (the first Cmd-click after the pointer returns to the same cell whose text changed sends nothing): the user accepted both as todos (AR-12-07). Todo files exist in `.planning/todos/pending/`.
- **G-1** (`internal/pty` flake): accepted as unrelated by the user.
- **WR-01** (a relative click resolves against the newest print of that text): a recorded 12-CONTEXT design choice ("last occurrence"). It does not loosen the printed check, and the user's decision is pending in 12-SECURITY.md. Not a gap.
- **Manual checks** on the user's Macs: M-131 (desktop underline, dark repaint), M-132 (Claude Code), M-133 (`ssh localhost`).

### Human Verification Required

None outstanding for this verdict. The three rig-limited items were decided by the user at the 12-09 checkpoint and are carried as overrides above. They remain manual regression checks M-131..M-133 in TESTING.md Section 5.

### Gaps Summary

No gaps remain. Gap 1 (peer tabs) and Gap 2 (surviving viewers) are fixed in the frontend with real-xterm reproductions that failed before and pass after, and both are confirmed on the two-VM tailnet rig. Gap 3 (TESTING.md Category BB and the security verdict) is written. The phase goal holds: local tabs, desktop peer tabs and browser viewers preview printed paths, and they keep doing so across reconnects, upgrades and resizes. The host keeps the PREV-03 contract.

---

_Verified: 2026-11-22T13:54:43Z_
_Verifier: Claude (gsd-verifier)_
