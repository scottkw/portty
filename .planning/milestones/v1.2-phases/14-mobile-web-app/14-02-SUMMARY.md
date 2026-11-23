---
phase: 14-mobile-web-app
plan: 02
subsystem: relay
tags: [go, input-filter, esc, vm-check, tart, mobile]
requires:
  - phase: 11
    provides: InputAbsorber (OSC 10/11 and DA1 reply absorber) shared by the local relay pump and the share server pump
provides:
  - InputAbsorber.Filter forwards a bare ESC that ends a MsgInput frame at once (both pumps)
  - 14-VM-CHECKS.md with the header, before rows E-00 and E-01 and the 14-02 cleanup (14-12 appends the after rows, M-150)
  - P9 changed on purpose, the accepted split-reply cost pinned (T-14-09)
affects: [14-03, 14-04, 14-12]
status: complete
actuals:
  tokens: 4500
  tasks: 2
  commits: 3
plan_head_before: 9b3e6b2a1fc19920008d190b562be8a519b04d3b
plan_head_after: 55953d3d67403b0f284c81d9d98da45dc03dca5f
tech-stack:
  added: []
  patterns:
    - "flush the one ambiguous absorber state (stateGotEsc) at the end of a frame; stateInOSC and stateInCSI keep absorbing across frames"
key-files:
  created:
    - .planning/phases/14-mobile-web-app/14-VM-CHECKS.md
  modified:
    - internal/relay/oscabsorb.go
    - internal/relay/oscabsorb_test.go
    - internal/relay/oscabsorb_relay_test.go
    - internal/webserver/oscabsorb_relay_test.go
key-decisions:
  - "A lone ESC ending a frame is forwarded at once; the only cost is a reply split exactly between ESC and the introducer, which xterm.js never produces (RESEARCH Q10) and which a test pins and the doc comment names"
  - "IsUserInput stays unchanged: a lone trailing ESC still does not claim the grid (INPT-06)"
requirements-completed: []  # MOB-01 is only the server-side Esc fix here; the key bar and the page are later plans, so requirements.mark-complete was not run (same call as 14-01)
---

# Phase 14 Plan 02: Lone Esc reaches the shell at once Summary

A bare ESC that ends an input frame now goes straight to the PTY through both the local relay pump and the share server pump; before the change, the same lag was first reproduced on the two Tart VMs in vim and a raw key reader.

## What was done

- **Task 1 (tracer).** Before rows on the VMs' installed build (Phase 13 HEAD `dev`, binary sha256 `a01ae0e147c03409…`, `/health` `api":5`, identical on both): E-00 (local tab) and E-01 (home peer tab over the tailnet) both PASS. The ESC byte is held until the next key: key reader log `1b` and `78` 0.2 ms apart (local) and 0.55 ms apart (peer) with `x` sent 3.09 s after the ESC; vim's `InsertLeave` probe absent 3 s after the ESC and set only once `:` arrived. Committed `3d2bf009`, then the fix `659aee3a`: after the byte loop, `if a.state == stateGotEsc { out = append(out, 0x1b); a.state = stateOutside }`, doc comments rewritten (Filter, and the shortcut note above IsUserInput), plus `TestRelay_LoneEscReachesPTY` in `internal/webserver/oscabsorb_relay_test.go` (a lone-ESC frame over the real WebSocket reaches the PTY pipe within 500 ms; confirmed to FAIL on the pre-fix filter, 0.53 s, and PASS after).
- **Task 2.** `55953d3d`: P9 (`TestInputAbsorber_P9_BareEscThenColon`) now asserts call 1 returns ESC and call 2 returns `:`; new `TestInputAbsorber_LoneEscEndsFrame` (ESC, `abc` ESC, ESC ESC; next byte passes alone; IsUserInput of a lone ESC false), `TestInputAbsorber_EscBracketSplitLeaks` (the accepted cost) and `TestRelay_handleSession_LoneEscReachesPTY` (local relay pump). S1-S5, OSC 10/11, DA1 and the other IsUserInput tests are untouched and pass.

## Test results

- Logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.6h4KkgGZoz` (task 1), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.d3MO7B3SFT` (task 2 targeted and full run), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.LS8H2CJJGI` (the pre-fix failing run).
- Task 1 verify: vet clean; `TestRelay_LoneEscReachesPTY`, `TestRelay_KeystrokesStillForwarded`, `TestRelay_OSC11SplitAcrossTwoFrames` PASS; flush present; evidence grep for link/tailnet URL patterns 0.
- Task 2 verify: the four named tests PASS (22 tests in the targeted run, 0 failures); `want empty (ESC buffered)` gone.
- Full `go test -race -short -count=1 ./...` (run once, `GOTOOLCHAIN=go1.26.11`): exit 0, 14 `ok` package lines, no FAIL, DATA RACE or panic. The G-1 `internal/pty` flake did not occur.

## VM results

| Row | Result | Key numbers |
|-----|--------|-------------|
| E-00 | PASS | vim: tail sha256 `8d8116068829` at the ESC and at +3 s, `leave.txt` absent; key reader `1b` and `78` at `…225.2735` and `…225.2737`, ESC sent `…222.22` |
| E-01 | PASS | peer tab (`join … origin=web`): `1b` and `78` at `…285.2300` and `…285.2306`, ESC sent `…282.137` from the home VM |

Cleanup: share ended, session killed, link file and private directories and `~/p14` deleted, settings restored (`cmp` clean), dotfiles equal (the office `~/.zsh_history` was restored from the private copy after `e00`'s shell ended), both VMs relaunched on the installed build and still on the tailnet, 0 sessions. Details in `.planning/phases/14-mobile-web-app/14-VM-CHECKS.md`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Terminal rows are not readable over AX, so the "AX-read the bottom row" step could not run as written**
- **Found during:** Task 1, E-00
- **Issue:** the AX dump of the terminal panel has no accessibility-tree rows (only the viewport, helper textarea and decoration groups), also with WebGL off, and `screencapture` of the window is an empty frame (the web view is not captured).
- **Fix:** "`-- INSERT --` still shown" is evidenced by the session tail's sha256 being identical at the ESC and at +3 s (vim redraws its mode line on leaving Insert mode) plus a VM-only `InsertLeave` probe (`vim -S ~/p14/probe.vim`, writes `localtime()`): absent at +3 s, set only after the next key. The row's pass condition (vim stays in Insert for the window, the ESC byte reaches a raw reader only with the next key) is unchanged. Recorded in the header of 14-VM-CHECKS.md.
- **Files modified:** `.planning/phases/14-mobile-web-app/14-VM-CHECKS.md`
- **Commit:** `3d2bf009`

**2. [Plan ordering] The tree was red for one commit**
- Task 1's filter change makes the old P9 assertion fail (the plan assigns P9's rewrite to Task 2). Commit `659aee3a` therefore has `TestInputAbsorber_P9_BareEscThenColon` failing in `internal/relay`; `55953d3d` fixes it. Nothing else in the package failed after the filter change.

## Known Stubs

None.

## Threat Flags

None. T-14-09 (only `stateGotEsc` flushed; the accepted cost pinned by `TestInputAbsorber_EscBracketSplitLeaks`) and T-14-15 (no network drop, no VM stop, link only in a 0600 file in a deleted directory, evidence grep clean, settings and dotfiles restored) are mitigated as planned.

## Not verified

- Claude Code or any real TUI: neither VM has node, so the TUI stand-in is the raw key reader (and vim). The after rows are M-150 in 14-12, run on a build that contains this change; no VM was run on the new filter in this plan (the plan states the fix is proven by tests only here).
- A raw terminal behind `portty attach` that splits a reply exactly between ESC and `]`: pinned by a unit test, not run live.
- The `1b` hold timing in E-01 compares two VM clocks (they agree to a few ms in the evidence), but the pass condition only uses the office log's two lines.

## Self-Check: PASSED

- Files: `14-VM-CHECKS.md`, `internal/relay/oscabsorb.go`, `internal/relay/oscabsorb_test.go`, `internal/relay/oscabsorb_relay_test.go`, `internal/webserver/oscabsorb_relay_test.go` all present.
- Commits `3d2bf009`, `659aee3a`, `55953d3d` are ancestors of HEAD; `git rev-list --count 9b3e6b2a..HEAD` = 3 (before this SUMMARY's own commit).
