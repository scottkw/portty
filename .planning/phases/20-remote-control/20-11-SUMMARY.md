---
phase: 20-remote-control
plan: 11
subsystem: testing
tags: [testing-md, traceability, manual-checklist, regression-suite]
requires:
  - phase: 20-01 to 20-10
    provides: "the test files, the TESTING.md rows each plan listed, and 20-VM-CHECKS.md"
  - phase: 19-16
    provides: "Category BJ and the M-179 numbering base"
provides:
  - "TESTING.md Section 2 counts and the Phase 20 note"
  - "Section 4 rows for every Phase 20 test file and all seven requirement IDs"
  - "Section 5 Category BK, M-180..M-193"
affects: [21, 22]
tech-stack:
  added: []
  patterns:
    - "one Section 4 row per (requirement, file); seam files get a row that says they hold no tests"
key-files:
  created: []
  modified:
    - TESTING.md
key-decisions:
  - "Section 2 counts are the real git ls-files counts (Go 238, vitest 133, total 383), which include Phase 21 files already merged on this branch; the note names them so Phase 21 can claim them"
  - "The golden .jsonl files are named in the Section 2 note and the protocol_test.go rows, not given a Section 4 row, because the checker only validates .go, .ts, .tsx and .sh paths"
patterns-established:
  - "Category BK header ends 'Phase 21 numbers its items after M-193.'"
requirements-completed: [DCTL-01, DCTL-02, DCTL-03, DCTL-05, DCTL-06, DESK-05, DESK-06]
duration: 40min
completed: 2026-11-26
status: complete
commits: 2
plan_head_before: fff0e330abe75bf4ce5b6e0a5b2326b83d1c24c6
plan_head_after: 943f483e4429f9beb27aef4253377bf2f2c6adf3
actuals:
  tokens: 21000
  tasks: 2
  commits: 2
---

# Phase 20 Plan 11: TESTING.md standing convention Summary

**Phase 20's 22 new test files are counted and traced (Go 238, vitest 133, total 383) and 14 owner-run checks, M-180..M-193, record what only a real Mac, display, trackpad, phone or password manager can show.**

## Performance

- **Duration:** about 40 min
- **Tasks:** 2 of 2
- **Files modified:** 1 (TESTING.md)

## Accomplishments

- Section 2: counts recomputed from `git ls-files` and a Phase 20 note listing the 18 new Go files, the 4 new vitest files, the golden protocol files `internal/desktop/testdata/input-v1.jsonl` and `signal-v1.jsonl`, every edited-in-place file, the post-VM X1 test `TestWindow_SameAppPopupAllowed`, the two load-sensitive flakes, and the VM record path.
- Section 4: about 50 rows covering DCTL-01, DCTL-02, DCTL-03, DCTL-05, DCTL-06, DESK-05 and DESK-06, test names in the Notes column. The four rows 20-01 added were kept, not duplicated.
- Section 5: Category BK with a rig paragraph and 14 items, each with steps, a PASS line, `_Last run:_ not yet run.`, `_Why not automatable:_` and `_Source:_`.

## Task Commits

1. **Task 1: Section 2 counts and note, Section 4 rows** - `ce91e3c1` (docs)
2. **Task 2: Section 5 Category BK** - `943f483e` (docs)

## Numbering (for the orchestrator)

- Highest M-NN found before writing: **M-179** (Category BJ, whose header said "Phase 20 numbers its items after M-179", so they agreed). Last category label before: **BJ**; new label: **BK**.
- Range used: **M-180..M-193** (14 items). The orchestrator's branch (`gsd/phase-17-desktop-feasibility-spike`, at the same base) still had M-179 as its highest when I re-checked just before each commit. If Phase 21 claimed numbers in 180..193 since, renumber one side; the three places that name the range are the Section 2 Phase 20 note ("M-180..M-193"), the BK header sentence ("Phase 21 numbers its items after M-193."), and the item numbers; M-183 and M-186 are cross-referenced inside BK (M-192 refers to M-183 for the lock screen, the Section 2 note refers to M-186 for the X1 recheck).

Items and their sources:

| Item | Covers | Source row or plan |
|------|--------|--------------------|
| M-180 | Retina and external pointer at any size, natural scrolling, double click | E2, S1, UI-SPEC backstop |
| M-181 | IME, emoji, dictation, dead keys, non-US layout (recorded) | UI-SPEC backstop, 20-02 and 20-06 summaries |
| M-182 | nothing held: Cmd chords, closed tab, sleeping Mac, dropped network | H1 |
| M-183 | Send Keys, Lock Screen, Ctrl and Cmd swap on a non-Mac keyboard | C1, C2 |
| M-184 | window share: raise, confinement, banner, chords, hung app | W1, W2 |
| M-185 | sheet, Open and Save panel, pop-up in the decoded frame (recorded) | W3 |
| M-186 | pop-up click after the X1 carve-out | X1 |
| M-187 | AX focus read -25204 on the real Macs (recorded) | X3 |
| M-188 | display switching, unplug, closed window, iPhone and Wails in-band resize | D1, D2, A11, A12 |
| M-189 | clipboard in Chrome and Safari, password manager item, 1 MB | P1, P2 |
| M-190 | Paste from Other Apps on the corporate Mac (recorded) | P1, P3, A10 |
| M-191 | Chrome Keyboard Lock, Safari and Firefox notice, light and dark | UI-SPEC backstops |
| M-192 | password field and Caps Lock (A5, A8; recorded) | closure, A5, A8 |
| M-193 | viewer never says Streaming ahead of a picture, ICE failure copy | X2 |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The plan's "first Phase 20 commit" lookup finds a March commit from an older milestone**
- **Found during:** Task 1
- **Issue:** `git log --reverse --grep='^[a-z]+\(20-01\)'` returns three 2026-03-23 commits from an earlier milestone's phase 20 first, so the plan's verify (`head -1`, then `git diff --diff-filter=A "$FIRST^" HEAD`) would demand Section 4 rows for every test file added since March.
- **Fix:** used the first commit of this phase (`c5ccd717`, 2026-11-26, the first `feat(20-01)` after `docs(phase-20): begin phase execution`) and cross-checked the file list against the `(20-NN)` commit messages. Every file added by a Phase 20 commit has a Section 4 row (checked by a loop), and all seven requirement IDs have rows.
- **Files modified:** none beyond the plan's
- **Commit:** n/a (verification only)

**2. [Rule 1 - Bug] Counts include Phase 21 files already on this branch**
- **Found during:** Task 1
- **Issue:** `git ls-files` on the base shows Go 238 and vitest 133, but Phase 20 alone is +18 and +4 (213 and 126 after Phase 19 and quick pn6). The other +7 Go and +3 vitest are Phase 21's, already merged: `internal/daemon/desktop_caps_test.go`, `desktop_peer_token_test.go`, `peer_desktop_test.go`, `remote_desktop_ws_proxy_test.go`, `internal/tailnet/desktop_meta_test.go`, `internal/webserver/desktop_peer_test.go`, `app_peer_desktop_test.go`, and `desktopCoords.test.ts`, `peerDesktop.test.ts`, `touchTrackpad.test.ts`.
- **Fix:** the table carries the real `git ls-files` counts, and the note says which files are Phase 21's and that Phase 21 records them. I did not add Section 4 rows for them (their requirement IDs are Phase 21's to choose). Note for the orchestrator: the plan's literal `git diff --diff-filter=A "$FIRST^" HEAD` verify loop run from `c5ccd717^` would also flag those ten files and `internal/daemon/restart_pin_test.go` (quick pn6, which already has its own UPGR-01 row); that is Phase 21's and the quick task's, not a Phase 20 gap.
- **Commit:** ce91e3c1

**3. [Rule 3 - Blocking] The plan's whole-file forbidden-text grep matches three old lines**
- **Found during:** Task 2
- **Issue:** `grep -Eq '...\.ts\.net|cap=[A-Za-z0-9]...' TESTING.md` matches existing placeholders (a bare `.ts.net` in a Phase 165 note, `?cap=TOKEN` in a Phase 159 row, `<macA>.<tailnet>.ts.net` in an old manual step). None is a real name, address or token, and none is mine; changing old history is out of scope.
- **Fix:** ran the same pattern (plus the five preflight symbols) over the added lines of my diff: 0 matches. `tests/check-brand.sh` and `tests/check-traceability-paths.sh` pass, and `TestPreflightSymbols_OnlyInPreflightFile` passes.
- **Commit:** 943f483e

**4. [Plan wording] `SettingsSheet.test.tsx` was not edited in place**
- The plan's edited-in-place list names it, but no Phase 20 commit touched it (only `SettingsSheet.desktop.test.tsx` and `DesktopSetupSheet.test.tsx`), so the note leaves it out.

## Not turned into M-NN items

- A9 (`kCGEventSourceUserData` survives to listeners) is not something the owner can observe; it is Phase 22's to prove.
- A14 (cheap window frame fetch cost) and the VM-run A15 are performance and feel notes; A15's double-click feel is a step in M-180, A14 has no owner-visible check.

## Known Stubs

None. TESTING.md only.

## Issues Encountered

- The load-sensitive flakes (`TestFanout_ThreeViewersEachGetKeyframe`, and `TestFanout_ReconnectWhileJoining` as the orchestrator reported it) are recorded in the Section 2 Phase 20 note, since TESTING.md has no separate flakes section. I could not locate a Phase 20 summary naming `TestFanout_ReconnectWhileJoining`, so that name comes from the dispatch prompt; the first one is named in 20-02-SUMMARY.md.
- An unrelated empty scratch file was written to a wrong `/private/tmp` path once and removed; nothing in the repo was affected.

## Threat Flags

None. The only boundary is the public repo docs (T-20-50); the checks above show no home path, address, ts.net name, token or preflight symbol added.

## Self-Check: PASSED

- TESTING.md exists with `Remote Control (Phase 20)` once and `Phase 21 numbers its items after M-193.` once; M-180..M-193 are unique; 14 `_Source:_` lines name 20-VM-CHECKS.md or a 20-NN plan.
- Commits `ce91e3c1` and `943f483e` are on the branch.
- `bash tests/check-traceability-paths.sh` (295 paths exist) and `bash tests/check-brand.sh` pass.
