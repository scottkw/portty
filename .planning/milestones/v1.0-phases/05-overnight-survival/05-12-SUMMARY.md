---
phase: 05-overnight-survival
plan: 12
subsystem: frontend
status: complete
gap_closure: true
tags: [frontend, xterm, replay, reattach, scrollbar, shar-07, verf-01]
requires:
  - phase: 05-08
    provides: "frontend/src/lib/__tests__/fixtures/wrapped-replay.json (Go wrapped full replay plus host rows, cursor, history tail, live redraw)"
  - phase: 05-09
    provides: "Two-VM rig, axdump, 05-VM-MATRIX.md Feature checks table"
provides:
  - "frontend/src/lib/guestTerminal.ts: applyHostGrid (resize in stream order through an empty write's callback) and resetForFullReplay (RIS write instead of the synchronous reset)"
  - "frontend/src/lib/__tests__/replayFidelity.test.ts: real xterm.js cross-check against the Go fixture plus ordered reset and ordered resize cases with synchronous negative controls"
  - "TerminalPanel guest onResize/onReset through the helpers"
  - "Guest element stretched to max(content width / scale, grid width), and the guest scale now fits the container's content box"
  - "V-12 PASS row in 05-VM-MATRIX.md"
affects: [05-13, 05-14, 05-15]
actuals:
  tokens: 9842
  tasks: 3
  commits: 11
plan_head_before: 93de854e81b172895f16b9730b4460529595fc41
plan_head_after: c19565b13ef6ce90d97c8185a7cef68f293273e8
tech-stack:
  added: []
  patterns:
    - "Guest-side xterm state changes that must follow the byte stream go through the write queue (empty write callback, in-band RIS), never the synchronous API"
    - "VM row evidence via an uncommitted screenReaderMode build: xterm mirrors its viewport rows into the AX tree, so axdump can diff host and guest rows exactly"
key-files:
  created:
    - frontend/src/lib/guestTerminal.ts
    - frontend/src/lib/__tests__/replayFidelity.test.ts
  modified:
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.hot-swap.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.scale.test.tsx
    - .planning/phases/05-overnight-survival/05-VM-MATRIX.md
    - TESTING.md
key-decisions:
  - "Fixture rows are compared right-trimmed of spaces, the same rule as the Go fixture's plainRows: a styled ESC[K line is replayed as background-coloured spaces, which xterm counts as content but which look the same"
  - "The guest scale is computed from the container's content box (padding excluded), as fitTerminal does for hosts; with the old clientWidth input a width-limited grid ran into the padding and the stretched element ended inside the grid"
  - "Guest element width = max(content width / scale, grid width) in every branch: it reaches the pane edge whenever the grid is narrower, and equals the grid when the grid overflows and the pane scrolls sideways"
  - "xterm 6 scrollbar location: .xterm-scrollable-element (a position:relative block child of term.element) holds the vertical scrollbar at right:0; the element fills term.element, so the inline width on term.element is enough and no style.css rule was needed"
requirements-completed: [SHAR-07, VERF-01]
coverage:
  - id: D1
    description: "Real xterm.js fed the Go wrapped replay shows the host's rows, cursor and scrollback tail, before and after the live redraw"
    requirement: VERF-01
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/replayFidelity.test.ts#rows, cursor and scrollback tail match after the replay, and again after the live redraw"
        status: pass
    human_judgment: false
  - id: D2
    description: "A full-replay reset and a host grid change are applied in stream order"
    requirement: SHAR-07
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/replayFidelity.test.ts#resetForFullReplay drops the queued burst: only the replay remains, no scrollback"
        status: pass
      - kind: unit
        ref: "frontend/src/lib/__tests__/replayFidelity.test.ts#applyHostGrid lands between old-width and new-width bytes"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx#a full replay on the new socket writes RIS once, in stream order before the next output"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/TerminalPanel.scale.test.tsx#guest: onResize waits for the queued write callback, then resizes, then rescales"
        status: pass
    human_judgment: false
  - id: D3
    description: "Guest scrollbar at the pane edge with the host grid unchanged and no guest resize sent"
    requirement: SHAR-07
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/TerminalPanel.scale.test.tsx#05-12 TerminalPanel — guest element spans the pane (scrollbar at the edge)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Two-VM reattach after >300 KiB of Claude-like output matches the office rows on first open and reopen, typing round-trips, scrollbar at the pane edge (V-12)"
    requirement: SHAR-07
    verification:
      - kind: automated_ui
        ref: ".planning/phases/05-overnight-survival/05-VM-MATRIX.md#V-12"
        status: pass
    human_judgment: false
duration: 40min
completed: 2026-11-18
---

# Phase 5 Plan 12: Guest Reattach Rendering and Scrollbar Summary

**Real xterm.js renders the Go wrapped replay exactly like the host. Guest resets and host grid changes now go through xterm's write queue (RIS, empty-write callback), so they apply in stream order. The scaled guest terminal is stretched so its scrollbar sits at the pane edge. Two VMs showed identical rows (29/29) after 390 KB of Claude-like output.**

## Performance

- **Duration:** about 40 min (04:39Z to 05:20Z)
- **Tasks:** 3
- **Files modified:** 8 (2 created)

## What was built

- **`guestTerminal.ts`** (24 lines):
  - `applyHostGrid(term, cols, rows, after?)` runs `term.resize` and then `after` inside the callback of `term.write('')`. Values are clamped to at least 1 (T-157-03).
  - `resetForFullReplay(term)` writes `ESC c`. In xterm 6, RIS fires `onRequestReset` and calls the same `Terminal.reset()` as the API, but in stream order.
- **TerminalPanel:**
  - `onResize: (cols, rows) => applyHostGrid(term, cols, rows, recomputeScale)`.
  - `onReset: () => resetForFullReplay(term)`.
  - Pitfall 5 still holds: the resize happens before recomputeScale.
- **recomputeScale (guest):**
  - The scale is computed from the content box, not clientWidth/clientHeight.
  - `term.element.style.width = max(contentW / s, cols * cellW)px`.
  - Host panels are untouched, and the guest still never calls sendResize.
- **Tests:**
  - `replayFidelity.test.ts` (5 cases): the fixture cross-check, plus ordered reset and ordered resize, each with a synchronous negative control.
  - The reconnect test asserts the RIS write comes before the next output.
  - The hot-swap guard allows exactly `onReset: () => resetForFullReplay(term)` and no `.reset()`.
  - The scale test: onResize waits for the queued callback, and there are 7 stretch/scale cases.

## Cross-check result (Task 1 RED record)

- **The fixture cross-check passed at RED** once rows were right-trimmed of spaces. Rows, cursor (13,26), the 20-line history tail, `screenAfter` and `cursorAfter` all match. Before the trim, the only difference was trailing spaces on the `+ added line N` rows: the Go side renders a styled `ESC[K` as background-coloured spaces, and `translateToString(true)` keeps printed spaces. The Go fixture strips them the same way (`plainRows`). No glyph-width disagreement: 完成, 测试通过, ✅, 🚀, ⏺, ⎿, ✻ and ✳ all line up under Unicode 11.
- **Ordered reset RED:** `expected 401 to be 10`. The burst still queued was parsed after the synchronous reset.
- **Ordered resize RED:** `'left               r | ight-status | …'` vs `'left | status | after resize  end'`. The CHA status line was parsed at the new width.

## Task Commits

1. **Task 1 (tracer):** `4818524b` test (RED), `cf66661e` fix (GREEN). Tracer gate: verify re-run after commit, passed (`TRACER-VERIFIED`).
2. **Task 2:** `67c30ed9` test (RED), `36a2cf2e` fix (GREEN).
3. **Task 3 and VM-found fixes:**
   - `5b40adac` test (RED: content-box scale)
   - `447d2ddd` fix
   - `c1b74e84` test (RED: floor reached through the height)
   - `b2e03b22` fix
   - `c19565b1` test (V-12 row and TESTING.md)

`commits: 11` is measured from the ledger. It includes two commits that are not from this plan: `ef434432` and `fdf54e25` (sketch docs by the user, made on the branch during the run).

## VM evidence (Task 3)

WebKit screenshots are blank in these VMs, so all evidence comes from the AX tree.

**How rows were read.** xterm's row container is `aria-hidden`, so for row text I built an evidence-only variant: commit 447d2ddd plus a local `screenReaderMode: true`. It was never committed, and the file was restored right after the build. With that option xterm mirrors its viewport rows into `.xterm-accessibility-tree`, which `axdump2` (values up to 400 chars) and `rows.sh` turn into numbered rows. WebGL was switched off in both VMs' settings.json for that run, because the office VM's WebKit GPU process crash-looped once a host terminal opened and the office AX tree went empty. Both settings files were restored afterwards.

- **Setup:** office session `vm12`, host grid 48x29 (office window 800 wide). It ran `claude.zsh 200`, a printf-only zsh port of the 05-08 generator: 390 KB, more than the 256 KiB ring.
- **First open** (home window 1200x634, tab `portty-vm-office › vm12`): office `daemon.log` shows `replay ... mode=full bytes=69307`. `diff` of the office and home rows is empty, 29 rows.
- **Typing:** `echo VM-REPLAY-OK` typed in the home tab. Both show the command, `VM-REPLAY-OK` and a new prompt; the diff is empty. The guest's `input total ... resizes=0`.
- **Reopen** (Cmd+W, click the row): `mode=full bytes=69367`, diff empty.
- **Live host grid change:** the office window was widened (grid 48x29 -> 85x29) with the home tab open, then 80 more turns ran live. Diff empty.
- **Scrollbar geometry:** the scrollbar is aria-hidden. Its proxy is the `.xterm-viewport` frame, which is inset 0 inside the scaled element and as wide as `.xterm-scrollable-element`, the element that holds the scrollbar at right:0. The pane content box is x 304..1224.

  | Case | Viewport width | Notes |
  |------|----------------|-------|
  | height-limited, scale 0.916 | 920 | grid 477 wide |
  | width-limited (window 1100) | 820 | = content width |
  | true overflow (window 800, grid 85 cols) | 645 | = grid x 0.7; the pane scrolls sideways |
  | pre-fix build 447d2ddd, host grid 114x38 (floor reached through the height) | 644 of 920 | inside the grid; drove the `b2e03b22` fix |
  | final clean build b2e03b22 (WebGL on, default settings), grid 48x29, windows 1200/1000/800 | 920 / 720 / 520 | = content width each time |

  The height-only floor case could not be re-created on the final build: the office grid changed late, and a larger home font did not reach the guest tab. It is covered by a unit test.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Guest scale used the padded box**
- **Found during:** Task 3 (VM geometry reasoning for the width-limited case)
- **Issue:** `computeGuestViewport(container.clientWidth, container.clientHeight, …)` includes the 12/16 px padding. A width-limited grid ran 32 px past the content box, and the stretched element would end inside the grid, putting the scrollbar over its last columns.
- **Fix:** scale and stretch both use the content box, as `fitTerminal` does for hosts.
- **Files modified:** TerminalPanel.tsx, TerminalPanel.scale.test.tsx
- **Commits:** 5b40adac, 447d2ddd

**2. [Rule 1 - Bug] The floor branch skipped the stretch**
- **Found during:** Task 3 (VM: viewport 644 of 920 px with a 114x38 host grid)
- **Issue:** the plan's "no stretch in the overflow-x branch" left the scrollbar mid-pane when the floor was reached through the height while the grid was still narrower than the pane.
- **Fix:** `max(contentW / s, cols * cellW)` in every branch. A really overflowing grid gets an element exactly as wide as the grid.
- **Commits:** c1b74e84, b2e03b22

**3. [Comparison rule] Fixture rows right-trimmed of spaces**
- **Issue:** see "Cross-check result". The same rule the Go side applies, not a weaker comparison.

**4. [Process] RED evidence**
- `RED_EVIDENCE_OK` was obtained for each target by running it alone with `-t`.
- vitest's tap-flat output for a multi-line array diff is rejected by the classifier, so the resize case leads with a one-line assertion on the affected rows before the full array comparison.

**5. [Standing rule] TESTING.md updated**
- vitest 85, total 212.
- New Section 4 rows: replayFidelity.test.ts (SHAR-07 and VERF-01) and the scale test (SHAR-07). The reconnect row now notes RIS.
- The Go row was left for 05-15: 05-10/05-11 added two files and `git ls-files` shows 121. M-89 stays with 05-15.
- `check-traceability-paths.sh`: OK (121 paths).

**6. [Environment] VM evidence build and settings**
- See "VM evidence".
- The plan's VM IPs and tailnet names were superseded by the orchestrator's (192.168.64.108/109, portty-vm-office/home).
- I added a scratchpad `click` with `type <text>` and an `axsize` (AX window move/resize) helper. Both were compiled on the host and copied to `~/ptest` as `click2` and `axsize`.

**Total deviations:** 2 bugs auto-fixed (both found by the VM check), 4 process or environment notes. **Impact:** the scrollbar fix is more correct than planned. The guest scale drops by about 3 % at s < 1 so the grid stays inside the padding, matching host panels.

## Issues Encountered

- `wails build` regenerated the nested `frontend/src/wailsjs/wailsjs/` files four times. They were restored each time and never committed.
- Shift+= sent into the home guest tab during a font experiment did not change the font. If it was forwarded, it typed `+` into the office test session `vm12`, which is harmless.
- The office VM's GPU process restarts every ~25 s while a host terminal tab is open with WebGL on. This is a VM graphics limit, the same family as the blank screenshots.

## Known Stubs

None.

## Threat Flags

None. T-05-35 holds: the RIS is a client constant written only from RelayClient's `onReset`, which fires on the server's `replay:"full"` meta or on unannounced output onto existing content. The grid values are clamped to at least 1, and the guest never sends a resize (`resizes=0` in daemon.log, unit assertions kept). No package installs.

## Next Phase Readiness

- Both VMs run the clean build b2e03b22 with their original settings. Office session `vm12` (id 15ef71b4…) is shared.
- 05-15:
  - M-89, "guest reattach rendering and scrollbar at the pane edge", can cite V-12.
  - The Go count in TESTING.md still needs reconciling.
- The two todos (`2026-11-17-jumbled-screen-on-reattach` items 1-2, `2026-11-17-guest-scrollbar-at-pane-edge`) are resolved by this plan. They stay in `todos/pending` for the verifier to close. Item 4 of the jumbled-screen todo (two real Macs) remains with M-84.

## Self-Check: PASSED

- FOUND: frontend/src/lib/guestTerminal.ts, frontend/src/lib/__tests__/replayFidelity.test.ts, frontend/src/components/TerminalPanel.tsx, 05-VM-MATRIX.md (V-12 PASS row), TESTING.md
- FOUND commits: 4818524b, cf66661e, 67c30ed9, 36a2cf2e, 5b40adac, 447d2ddd, c1b74e84, b2e03b22, c19565b1
