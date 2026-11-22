---
phase: 12-file-path-click
verified: 2026-11-22T02:38:41Z
status: gaps_found
score: 6/14 must-haves verified
covered_files: [".planning/phases/12-file-path-click/12-01-PLAN.md", ".planning/phases/12-file-path-click/12-01-SUMMARY.md", ".planning/phases/12-file-path-click/12-02-PLAN.md", ".planning/phases/12-file-path-click/12-02-SUMMARY.md", ".planning/phases/12-file-path-click/12-03-PLAN.md", ".planning/phases/12-file-path-click/12-03-SUMMARY.md", ".planning/phases/12-file-path-click/12-04-PLAN.md", ".planning/phases/12-file-path-click/12-04-SUMMARY.md", ".planning/phases/12-file-path-click/12-05-PLAN.md", ".planning/phases/12-file-path-click/12-05-SUMMARY.md", ".planning/phases/12-file-path-click/12-06-PLAN.md", ".planning/phases/12-file-path-click/12-06-SUMMARY.md", ".planning/phases/12-file-path-click/12-07-PLAN.md", ".planning/phases/12-file-path-click/12-07-SUMMARY.md", ".planning/phases/12-file-path-click/12-SECURITY-REVIEW.md", ".planning/phases/12-file-path-click/12-VM-CHECKS.md", "TESTING.md", "frontend/src/App.tsx", "frontend/src/components/TerminalPanel.tsx", "frontend/src/components/WebShareSessionView.tsx", "frontend/src/lib/pathLinks.ts", "frontend/src/lib/relayClient.ts", "internal/daemon/engine.go", "internal/daemon/zshshim.go", "internal/daemon/zshshim.zsh", "internal/previewfile/previewfile.go", "internal/relay/cwd.go", "internal/relay/hub.go", "internal/relay/pathclick.go", "internal/relay/server.go", "internal/webserver/plugin_config.go", "internal/webserver/server.go"]
covered_digest: "v4:sha256:3c3127d1251b3c8ddd4053433fc1971d2cc64a882f1ec6a0ec1623c6d99aaa03"
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "Cmd-clicking a printed path opens it in a preview tab in a home-VM peer tab on an office-VM session over the real tailnet (SC2), and the click, missing-file silence and wrong-type notice work in a desktop peer tab (UI-SPEC error backstop, C-04 peer clause)"
    status: failed
    reason: "VM rows C-02 and C-04: the desktop peer tab never sends a path click (no daemon log line at all) and has no URL links either; same in pre-phase build A. The code confirms the suspected cause: a peer tab gets pluginConfig undefined, its cross-origin self-fetch of /api/plugin-config cannot be read (the endpoint sends no Access-Control-* header), so webLinks stays falsy and registerPathLinks is never called."
    artifacts:
      - path: "frontend/src/App.tsx"
        issue: "line 1311 passes pluginConfig={isRemoteWebTab ? undefined : ...}; line 980 sets the tab's baseURL to the peer share origin (parsed.origin), so the self-fetch is cross-origin from the wails:// web view"
      - path: "frontend/src/components/WebShareSessionView.tsx"
        issue: "line 159 isWebGuest = pluginConfig === undefined; line 165 fetches `${apiBaseURL}/api/plugin-config` and line 177 opens an EventSource on the same peer origin; lines 172-175 swallow the failure; line 194 effectivePluginConfig falls back to the undefined prop"
      - path: "internal/webserver/plugin_config.go"
        issue: "lines 35-37 set only Content-Type and Cache-Control; no Access-Control-Allow-Origin, so the wails:// origin cannot read the response (VM curl with Origin: wails://wails confirmed no Access-Control-* header)"
      - path: "frontend/src/components/TerminalPanel.tsx"
        issue: "line 860 `if (pluginConfig?.webLinks)` gates both the WebLinksAddon and registerPathLinks (lines 931-937), so a peer tab with no config gets neither"
    missing:
      - "A way for a desktop peer tab to get the host's plugin config (or at least webLinks/webLinksConfig) that does not depend on a cross-origin fetch the share server refuses: e.g. fetch it through the Go side (as the WS already goes through the daemon proxy), or a deliberate, narrowly scoped CORS allowance for the app origin reviewed against the token-gated, tailnet-only share constraint"
      - "A test that mounts WebShareSessionView as a remote peer tab (baseURL set, pluginConfig undefined, self-fetch failing) and asserts path links and URL links are still registered; current TerminalPanel.pathClick tests always pass pluginConfig {webLinks:true}"
      - "Re-run VM rows C-02 and C-04 (peer clause)"
  - truth: "A viewer that lives through a reconnect, an upgrade handoff or a grid change keeps clickable path links: after a B to B2 upgrade a pre-upgrade relative path is clickable at once in the surviving viewer (U-01), and after a window resize a printed path is still clickable (C-09)"
    status: failed
    reason: "VM U-01: the long-lived Safari page reconnected by delta sent no 0x13 and showed no link cursor on the pre-upgrade path while a fresh page clicked it fine (host cwd log moved, host is right). C-09: after widening the window, Cmd-clicks at the same cell sent nothing until the pointer clicked elsewhere. Same symptom once in C-03 (Safari conn=3). The suspected cause 'the capability is not re-enabled after a delta reconnect' is NOT confirmed by the code: the host puts pathClick on every replay meta including delta (internal/relay/hub.go:1812, 1829-1833; written by both internal/relay/server.go:139 and internal/webserver/server.go:916), the client re-enables on every replay meta (frontend/src/lib/relayClient.ts:549-551) and the provider reads enabled() lazily (frontend/src/lib/pathLinks.ts:110). The code instead points at xterm 6's Linkifier line cache, which nothing in Portty invalidates when the capability flips: a provideLinks answered while enabled() was false (relayClient.ts:398 onclose, :525 abandon) is cached for the hovered row and never re-asked while the pointer stays on that row (node_modules/@xterm/xterm/src/browser/Linkifier.ts:93-95, 131, re-query on re-render at 301-317); and after a resize Linkifier clears the current link (line 49) but only re-hovers when the pointer moves to a different cell (line 83), which matches C-09 exactly (same-cell clicks failed; a click on another row recovered it). Unproven: no test drives the real Linkifier."
    artifacts:
      - path: "frontend/src/lib/pathLinks.ts"
        issue: "provideLinks returns undefined while enabled() is false (line 110) and there is no hook to make xterm re-ask the provider when the capability turns true again"
      - path: "frontend/src/components/TerminalPanel.tsx"
        issue: "line 534 onPathClickCapability only writes pathClickEnabledRef; nothing refreshes xterm's link state on the false-to-true transition or after applyHostGrid resizes the terminal"
      - path: "frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx"
        issue: "the gating test (line 179) calls the captured provider directly, bypassing xterm's Linkifier, so caching and resize behaviour are never exercised"
    missing:
      - "Reproduce in a test that uses a real xterm Terminal and its Linkifier (or a browser test): hover a printed path, drop and restore the capability (close, then a delta replay meta), hover the same row again and expect a link; same for a resize followed by a click at the same cell"
      - "A fix that invalidates xterm's cached link replies when the capability changes and after a host grid resize (for example forcing a fresh provideLinks on the next hover), covering the URL links too since they share the Linkifier"
      - "Re-run VM rows U-01 (B to B2 surviving viewer) and C-09 (resize clause)"
  - truth: "TESTING.md has Category BB with M-128..M-135 whose last-run lines point at 12-VM-CHECKS.md (Section 5), and 12-SECURITY-REVIEW.md has its VM evidence and a verdict citing C-04, C-05, C-07 and C-08"
    status: failed
    reason: "12-07 Task 4 did not run because the user chose 'fix' at the Task 3 checkpoint. `grep M-128 TESTING.md` finds nothing outside the Section 2 note; 12-SECURITY-REVIEW.md lines 7-8 say the 12-07 'VM evidence and verdict' section is still to be appended and T-12-27 (line 111) depends on it."
    artifacts:
      - path: "TESTING.md"
        issue: "Section 5 has no Category BB / M-128..M-135 (the project CLAUDE.md standing rule requires manual items for VM-only behaviour)"
      - path: ".planning/phases/12-file-path-click/12-SECURITY-REVIEW.md"
        issue: "no 'VM evidence and verdict' section"
    missing:
      - "Re-run 12-07 after the gap plans land; its Task 4 writes Section 5 and the verdict"
human_verification:
  - test: "C-08 rig item: in a Portty session on a Mac where `ssh localhost` can authenticate (both a clean HOME and a ZDOTDIR-in-.zshenv setup), run `ssh localhost` and confirm the remote shell starts normally with the zsh shim in the outer session"
    expected: "ssh reaches a remote prompt with no startup errors; a relative click after a remote `cd` resolves against the outer directory (documented limit)"
    why_human: "Rig limit, not a code gap: the VM user has no key for itself, copying the rig key was refused and password use is forbidden by Phase 11's rule. Every other C-08 clause passed in both setups"
  - test: "C-06 real-Mac item: in the desktop app's local tab, hold Cmd over a printed path without moving the mouse, then move it, then release Cmd without moving"
    expected: "Underline and pointer appear while Cmd is held (ideally on the keydown itself) and disappear on release; if they appear only after a mouse move, decide whether to take UI-SPEC's fallback (underline on plain hover, click still Cmd-gated)"
    why_human: "WKWebView content is blank in VM screenshots and the AX tree does not expose terminal rows, so the app underline is unobservable in the rig; Safari shows the mechanism working"
  - test: "C-06 real-Mac item: show a path notice in the desktop app (Cmd-click a .txt), then switch macOS appearance light to dark"
    expected: "The notice repaints in dark colours with no reload"
    why_human: "Verified in Safari only; the app notice's colours are not readable via AX in the VM (backstop truth)"
  - test: "C-07 note: Cmd-click a printed path inside Claude Code on a real Mac"
    expected: "outcome=preview, nothing typed into the TUI"
    why_human: "Claude Code is not installed in the VMs; vim mouse=a passed"
---

# Phase 12: File Path Click Verification Report

**Phase Goal:** A user can Cmd-click a file path that a program printed in the terminal and see it previewed, in local and remote sessions alike, because Portty tracks each shell's working directory.
**Verified:** 2026-11-22T02:38:41Z
**Status:** gaps_found
**Re-verification:** No, initial verification

The host side (shim, cwd log, printed-token check, read-only refusal, file checks, preview delivery) is in place and VM-proven. Clicks work in a local tab and in the browser share view. The remote half of the goal fails: a desktop peer tab gets no path links at all, and a viewer that survives a reconnect or resize can lose them.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: new zsh session reports cwd via OSC 7 after every `cd` with no setup; other shells emitting OSC 7 tracked; without OSC 7 absolute works, relative does nothing (PATH-01) | ✓ VERIFIED | `internal/daemon/zshshim.go`, `zshshim.zsh` wired from `engine.go`; OSC 7 parsed in `internal/relay/cwd.go` from the PTY stream, independent of shell. C-08: both dotfile setups keep prompt, alias, ZDOTDIR, HISTFILE, PATH, `portty open`, clean `$(cd …)`, nested zsh, `chpwd_functions` healed; 0 `zsh shim:` log lines. U-01: A-spawned u1 (no shim): absolute `outcome=preview`, relative `outcome=no-cwd` |
| 2 | SC2: Cmd-click on absolute or relative path opens preview in a local tab AND in a home-VM peer tab over the real tailnet (PATH-02) | ✗ FAILED | Local PASS (C-01), Safari PASS (C-03). Peer tab FAIL (C-02): no click ever reaches the host. Cause confirmed in code, see Gap 1 |
| 3 | SC3: missing path does nothing (no tab, dialog, typing); plain click selects; URLs keep their behaviour | ✓ VERIFIED | C-01 (plain click selects, URL confirm popover), C-04 (missing and folder `outcome=missing`, tail sha256 unchanged, no tab/banner anywhere; notices only in clicker, gone in about 5 s) in local tab and Safari. Peer-tab clause unreachable, rides Gap 1 |
| 4 | SC4: request rides the session's stream, resolved against the session's own cwd, only printed paths; security review records it | ✓ VERIFIED | `internal/relay/pathclick.go` (printed-token check, cwd-at-print, read-only refusal), `internal/relay/server.go:207-212`, `internal/webserver/server.go:1007-1012`; C-05: read-only Safari sees no link, hand-sent 0x13 produces no log line and no preview; C-09 hard row `not-printed`. 12-SECURITY-REVIEW.md records the PREV-03 amendment and argument (lines 12-28). Verdict section missing, tracked as Gap 3 |
| 5 | Full gate passes once on the host | ✓ VERIFIED | 12-VM-CHECKS.md gate: 17 Go packages ok under -race, vitest 106/1576, tsc, brand/share-removal/traceability checks, build-script 40/0. My spot-checks: pathLinks + TerminalPanel.pathClick vitest 44/44; `go test -run PathClick` relay and webserver ok |
| 6 | `ssh localhost` works inside a shimmed session (12-07 truth, C-08 clause) | ? UNCERTAIN | Rig limit: publickey denied, key copy refused. Human item |
| 7 | After a window resize a printed path is still clickable (C-09) | ✗ FAILED | C-09: after widening, same-cell Cmd-clicks sent nothing; recovered after a click on another row. Gap 2. Wrap, history, identical-row and hard-break clauses PASS |
| 8 | After B to B2, a pre-upgrade relative path is clickable at once; every viewer reconnects with `mode=delta` (U-01) | ✗ FAILED | Host side PASS (3/3 delta, 0 grid lines, cwd log carried, fresh page previews). Surviving viewer sends no 0x13 and shows no link. Gap 2 |
| 9 | TESTING.md Section 5 Category BB M-128..M-135 and the security review verdict written | ✗ FAILED | 12-07 Task 4 not run (user chose fix). Gap 3 |
| 10 | Cmd-held underline and pointer in the desktop app (C-06, UI-SPEC) | ? UNCERTAIN | Unobservable in VM; Safari shows it working. Human item |
| 11 | Live light/dark switch repaints the notice with no reload (backstop) | ? UNCERTAIN | Safari PASS (C-06 computed colours change in 1.2 s, same conn). App unverified; `insufficient_spec` for the app, human item |
| 12 | Cmd-click in a mouse-reporting TUI previews (backstop) | ✓ VERIFIED | C-07: vim `mouse=a`, `outcome=preview`, 0 SGR mouse reports in the tail, clean quit |
| 13 | Every FAIL reaches the Task 3 checkpoint; a fix answer ends the plan with VMs cleaned | ✓ VERIFIED | 12-07-SUMMARY: user chose "fix"; Task 4 withheld; 12-VM-CHECKS.md Cleanup section (sessions killed, dotfiles and settings restored with sha256, HEAD deployed) |
| 14 | Click, missing-file silence, wrong-type notice and read-only refusal checked across local tab, peer tab and browser link (backstop) | ✗ FAILED | Local and browser PASS; peer tab cannot click (C-02/C-04). Same root cause as #2, Gap 1 |

**Score:** 6/14 truths verified (0 present but behavior-unverified; 3 uncertain, routed to human/rig)

### Suspected causes checked against the code

**C-02/C-04 (peer tab never gets path links): CONFIRMED.**
- `frontend/src/App.tsx:980`: a peer tab's `baseURL` is the peer share origin (`parsed.origin`).
- `frontend/src/App.tsx:1311`: `pluginConfig={isRemoteWebTab ? undefined : (pluginConfig ?? undefined)}`.
- `frontend/src/components/WebShareSessionView.tsx:159`: `isWebGuest = pluginConfig === undefined`. `:165` fetches `${apiBaseURL}/api/plugin-config` and `:177` opens an EventSource, both cross-origin from the `wails://` web view. `:172-175` swallow the failure. `:194` `effectivePluginConfig = livePluginConfig ?? pluginConfig` resolves to undefined.
- `internal/webserver/plugin_config.go:35-37`: only `Content-Type` and `Cache-Control` are set. There is no `Access-Control-Allow-Origin` anywhere in `internal/webserver` (the only "Access-Control" hit is a comment at `server.go:516` saying the peer-cap route deliberately sends none).
- `frontend/src/components/TerminalPanel.tsx:860`: `if (pluginConfig?.webLinks)` gates both the WebLinksAddon and `registerPathLinks` (`:931-937`). Nothing defaults it on.
- `frontend/index.html:8`: the desktop CSP sets no `connect-src`, so CORS, not the CSP, blocks the read.
- This matches the VM probe: URL links were also missing in peer tabs, in build A too.

**U-01 (surviving viewer loses path links): the code does NOT confirm the "capability not re-enabled" hypothesis.**
- The host sets `r.PathClick = !sub.ReadOnly` on every `Join`, delta included (`internal/relay/hub.go:1812`), and `Frames()` puts it on the replay meta (`:1829-1833`). Both servers use `Frames()` (`internal/relay/server.go:139`, `internal/webserver/server.go:916`).
- The client calls `onPathClickCapability(pathClick === true)` on every replay meta (`frontend/src/lib/relayClient.ts:549-551`). The provider checks `enabled()` each time it is asked (`frontend/src/lib/pathLinks.ts:110`), so registering it once per Terminal is fine.
- The code points at a different mechanism. Nothing invalidates xterm 6's Linkifier cache when the capability changes or after a host grid resize:
  - Replies are cached per hovered row and not asked again while the pointer stays on that row (`node_modules/@xterm/xterm/src/browser/Linkifier.ts:93-95`, `:131`). A re-render of the hovered link's row asks again right away (`:301-317`). That can store an "undefined" answer given while `enabled()` was false (set false at `relayClient.ts:398` and `:525`).
  - After a resize, `:49` clears the current link, but `:83` only hovers again when the pointer reaches a different cell. That fits C-09 exactly: clicks on the same cell failed, and a click on another row brought links back.
- This is unproven. The tests call the captured provider directly (`TerminalPanel.pathClick.test.tsx:179`), never the real Linkifier.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/daemon/zshshim.go` / `zshshim.zsh` | ZDOTDIR shim | ✓ VERIFIED | wired from `engine.go`; C-08 |
| `internal/relay/cwd.go` | OSC 7 cwd log | ✓ VERIFIED | handoff carry proven in U-01 |
| `internal/relay/pathclick.go` | Hub.PathClick | ✓ VERIFIED | C-01/C-04/C-05/C-09 outcomes |
| `internal/previewfile/previewfile.go` | shared file checks | ✓ VERIFIED | `outcome=unsupported`, `too-large` in C-04 |
| `frontend/src/lib/pathLinks.ts` | link provider | ⚠️ PARTIAL | works locally/Safari; never registered in peer tabs; goes stale after reconnect/resize |
| `frontend/src/components/TerminalPanel.tsx` | provider lifecycle + capability | ⚠️ PARTIAL | gated on `pluginConfig?.webLinks`, which peer tabs never get |
| `.planning/phases/12-file-path-click/12-VM-CHECKS.md` | rows C-00..C-09, U-01 | ✓ VERIFIED | all rows present with evidence |
| `TESTING.md` | Sections 2, 4, 5 for Phase 12 | ✗ PARTIAL | Sections 2 and 4 present; Section 5 M-128..M-135 missing |
| `.planning/phases/12-file-path-click/12-SECURITY-REVIEW.md` | verdict section | ✗ MISSING section | 12-05 part present; verdict not written |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| Local TerminalPanel | Hub.PathClick | MsgPathClick 0x13 on the relay stream | ✓ WIRED | C-01 log lines |
| Browser /app/ viewer | Hub.PathClick | share server WS (`webserver/server.go:1007`) | ✓ WIRED | C-03 |
| Desktop peer tab | registerPathLinks | `pluginConfig.webLinks` from `/api/plugin-config` | ✗ NOT_WIRED | CORS-blocked self-fetch, Gap 1 |
| relayClient capability | xterm link state | `onPathClickCapability` → ref only | ⚠️ PARTIAL | no invalidation of Linkifier cache, Gap 2 |
| 12-SECURITY-REVIEW.md | 12-VM-CHECKS.md | verdict cites C-04, C-05, C-07, C-08 | ✗ NOT_WIRED | verdict not written |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| TerminalPanel (local tab) | `pluginConfig.webLinks` | App `pluginConfig` state from the Wails binding | yes | ✓ FLOWING |
| TerminalPanel (browser) | `pluginConfig.webLinks` | same-origin `/api/plugin-config` self-fetch | yes | ✓ FLOWING |
| TerminalPanel (desktop peer tab) | `pluginConfig.webLinks` | cross-origin self-fetch, no ACAO header | no, stays undefined | ✗ DISCONNECTED |
| pathLinks `enabled()` | `pathClickEnabledRef` | replay meta `pathClick` on every join | yes | ✓ FLOWING (stale xterm cache downstream, Gap 2) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Path detection and gating unit tests | `pnpm vitest run src/lib/__tests__/pathLinks.test.ts src/components/__tests__/TerminalPanel.pathClick.test.tsx` | 2 files, 44 tests passed | ✓ PASS |
| Host path-click tests | `go test -count=1 -run PathClick ./internal/webserver/ ./internal/relay/` | ok, ok | ✓ PASS |
| Two-Mac behaviour | not re-run (instructed); 12-VM-CHECKS.md used as evidence | 6 FAIL rows | see truths |

### Probe Execution

Not applicable. The phase declares no `scripts/*/tests/probe-*.sh`. Its runnable evidence is the VM rows, which were not re-run, as instructed.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PATH-01 | 12-02, 12-03, 12-07 | cwd via OSC 7; zsh emits it; other shells that emit it work | ✓ SATISFIED (ssh clause human) | Truth 1, C-08 |
| PATH-02 | 12-01, 12-04, 12-05, 12-06, 12-07 | Cmd-click previews printed paths in local and remote sessions alike; only printed paths; missing does nothing | ✗ BLOCKED | Desktop peer tabs (Gap 1) and surviving viewers (Gap 2) |

No orphaned requirements: REQUIREMENTS.md maps only PATH-01 and PATH-02 to Phase 12.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (phase-modified source files) | - | TBD/FIXME/XXX/TODO/HACK | none found | - |
| `frontend/src/components/WebShareSessionView.tsx` | 172-175 | silent `.catch(() => {})` on the config fetch | ⚠️ Warning (pre-existing) | turns the CORS failure into a silent feature loss in peer tabs, which is why C-02 went unnoticed since before this phase |
| `frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx` | 179 | provider called directly, xterm Linkifier mocked out | ℹ️ Info | unit tests cannot see Gap 2 |

### Human Verification Required

These are rig limits and real-Mac checks, not code gaps.

1. **ssh localhost with the shim (C-08).** Run `ssh localhost` in a shimmed session on a Mac where it can authenticate, in both dotfile setups. Expected: a normal remote prompt. Why human: the VM user has no key for itself, and copying keys or using a password is forbidden.
2. **Cmd-held underline in the desktop app (C-06).** Hold Cmd over a printed path in a local tab, with and without moving the mouse, then release. Expected: underline and pointer follow Cmd. If they follow only mouse moves, decide on UI-SPEC's fallback. Why human: WKWebView content is blank in VM captures.
3. **Notice dark-mode repaint in the app (C-06).** Show a notice, then switch the appearance. Expected: it repaints with no reload. Why human: verified in Safari only.
4. **Claude Code TUI (C-07 note).** Cmd-click a path inside Claude Code. Expected: `outcome=preview`. Why human: Claude Code is not installed in the VMs.

### Gaps Summary

There are three gaps. Two are product defects; the third is the deferred end of 12-07.

1. **Desktop peer tabs get no path links (C-02, C-04 peer clause; SC2 and the error backstop).** Confirmed root cause. A peer tab receives no plugin config, and its cross-origin self-fetch of `/api/plugin-config` is unreadable because the share server sends no CORS header, so `webLinks` is false and neither URL links nor path links are registered. This predates Phase 12 (build A is the same), but Phase 12's SC2 makes it a phase blocker. The fix must respect the token-gated, tailnet-only share constraint: routing the fetch through the Go side, as the WebSocket already goes through the daemon proxy, is the lower-risk shape.
2. **Surviving viewers lose path links (U-01; C-09 resize clause; the C-03 conn=3 oddity).** The host is correct. The "capability not re-enabled" hypothesis is refuted by the code. The likely cause is xterm's Linkifier cache, which Portty never invalidates on a capability change or after a host resize. The gap plan should first reproduce this against a real xterm Linkifier, then fix it.
3. **12-07 Task 4 not run.** TESTING.md Section 5 (M-128..M-135) and the security review's verdict were withheld by the "fix" decision. They close by re-running 12-07 after Gaps 1 and 2.

No gap is deferred: Phase 13 (remote localhost URLs) and Phase 14 (mobile web) do not cover path links in peer tabs or reconnect behaviour.

---

_Verified: 2026-11-22T02:38:41Z_
_Verifier: Claude (gsd-verifier)_
