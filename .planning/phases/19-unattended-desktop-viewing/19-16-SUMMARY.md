---
phase: 19-unattended-desktop-viewing
plan: 16
subsystem: regression-suite-docs
tags: [testing-md, traceability, manual-checklist, standing-convention]
status: complete

requires:
  - phase: 19-01..19-15, 19-17
    provides: the Phase 19 test files, the VM record (19-VM-CHECKS.md) and the owner-run closure list
provides:
  - "TESTING.md Section 2: Go 212, vitest 126, total 350 and a Phase 19 note listing every new and edited test file, with the e2e and vmdesktop build tags recorded"
  - "TESTING.md Section 4: a traceability row per new Phase 19 test file, all 12 phase requirement IDs covered, every DSHR-01 row carrying the 2026-11-25 amendment sentence"
  - "TESTING.md Section 5 Category BJ, M-173..M-179 (seven owner-run items); M-163 reworded; From Phase 19 lines under M-157, M-159, M-160, M-161, M-164"
affects: [phase-20]

actuals:
  tokens: 12000
  tasks: 2
  commits: 2
plan_head_before: d31aa20ad255db42059c39fe01ac744f223cf457
plan_head_after: d4ad310e6e79c795419578af523dbfc79a75c7cf

key-files:
  created: []
  modified:
    - TESTING.md

key-decisions:
  - "Category BJ has seven items, not six: the plan's six plus M-179 for the real-window look (Desktop group, setup sheet wording, 256 x 160 box, tray line, Background service row scroll, viewer chip and cards), which the 19-13 (G8) and 19-14 summaries handed to this plan"
  - "M-176 (daemon-posted lapse notification) is not written as an unproven gap: V1 failed in the VM but was fixed in 19-14 and V1b and F13 passed, so the item asks only for the real banner on a notarized build, stays human_needed and is named in the BJ header; a missing banner is called a defect to fix, not a gap to accept"
  - "Numbering starts at M-173 because Phase 18.1 already wrote M-172 (highest M-NN at run time); the header ends 'Phase 20 numbers its items after M-179.'"
  - "The two helper files (internal/desktop/export_test.go, internal/desktop/fakes_test.go) get Section 4 rows that say they hold no tests, so the plan's every-added-test-file verify holds without pretending they test anything"

requirements-completed: [DPRM-01, DPRM-02, DPRM-03, DPRM-04, DESK-01, DESK-02, DESK-03, DESK-04, DSHR-01, DSHR-02, DSHR-08, DVWR-02]
---

# Phase 19 Plan 16: Record Phase 19 in TESTING.md Summary

TESTING.md now records Phase 19 under the standing convention: recounted Section 2 totals with a Phase 19 note, a Section 4 row for every new test file (all 12 requirement IDs traceable), and Section 5 Category BJ with seven owner-run items numbered M-173..M-179.

## What was built

- **Task 1 (tracer, `0eec059b`):** Section 2 counts from `git ls-files` (Go 182 to 212, +30, spike/ excluded; vitest 122 to 126, +4; Playwright 10 and build-script 2 unchanged; total 316 to 350) and the Phase 19 note: 30 new Go files and 4 new vitest files by plan, ten edited-in-place files, and the build-tag deviations (`internal/webserver/desktop_csp_e2e_test.go` is `//go:build e2e` like `browser_csp_e2e_test.go`, run with `-tags=e2e` after `pnpm --dir frontend build` with Chrome installed; `internal/desktop/lapse_vmdesktop_test.go` is `vmdesktop`, VM rig only; `screencap_darwin_test.go` is darwin only). Section 4: 54 rows after the REL-01 block, test names in Notes, nothing but a path in the path column.
- **Task 2 (`d4ad310e`):** Category BJ after BI.1 with the rig paragraph, the DSHR-01 amendment sentence, the human_needed M-176 named in the header and the Phase 20 sentence. Items: M-173 end to end with the GUI quit (Chrome and Safari from the home Mac, Retina sharpness, no Background service row), M-174 battery and display sleep, M-175 iPhone Home Screen, M-176 lapse notification from the daemon, M-177 upgrade with a viewer and live sessions (reads the tick gap), M-178 Safari consoles, M-179 real-window look. M-163 now says a fresh-process probe (`portty daemon perm-probe`) or Test Capture, not preflight polling in a long-running process. M-157, M-159, M-160, M-161 and M-164 each gained a `_From Phase 19:_` line.

## Pre-edit record (plan step 0)

The highest M-NN in TESTING.md before any BJ line was written: **M-172** (Category BI.1, Phase 18.1). First BJ item: M-173. Last: M-179.

## Verification

- `bash tests/check-traceability-paths.sh`: OK, 272 paths exist. `bash tests/check-brand.sh`: clean.
- Every one of the 12 requirement IDs has a `| REQ | path.(go|ts|tsx) |` row; every file in `git diff --diff-filter=A 89d29266 HEAD` for `*_test.go`, `*.test.ts`, `*.test.tsx` has a row (the Phase 18 files app_install_test.go, app_mover_test.go and internal/updater/* sit in that diff too but belong to Phase 18 and already have rows); no DSHR-01 row lacks "Start it anyway".
- Symbol confinement: `go test -run TestPreflightSymbols_OnlyInPreflightFile ./internal/screencap/` passes after the edits (TESTING.md names none of the preflight, accessibility or post-event symbols). `TestLauncherCopy` passes.
- Go test file count 212 is above the plan's 175 floor. BJ has 7 items; the BJ range check and `Phase 20 numbers its items after M-179.` pass; the M-163 block no longer contains "confirmed by preflight polling".
- No home path, token, `*.ts.net` name or 100.x address was added (the only matches in the file are older lines).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The plan's first-commit lookup finds the wrong commit**
- **Found during:** Task 1 verify
- **Issue:** `git log --grep='^[a-z]+\(19-01\)'` returns `ebe68acf`, a commit from an older, unrelated phase numbered 19, so the "added since the phase's first commit" diff would pull in the whole old history.
- **Fix:** used `89d29266` (the UI contract commit named in the plan's Artifacts section) as the base for the added-file list; Phase 18 and 18.1 files in that range were excluded by hand because they are not Phase 19 tests.
- **Files modified:** none
- **Commit:** n/a

**2. [Scope] Duplicate M-41 is not unique**
- **Found during:** Task 2 verify
- **Issue:** the plan's "no M-NN appears twice" check reports `**M-41**` twice. It was already duplicated at the plan's base commit (Phase 167 era), not by this plan, and no Phase 19 item reuses a number.
- **Fix:** none, out of scope. Logged here; the BJ numbers M-173..M-179 are unique.

### Other notes

- Seven BJ items instead of the plan's minimum six (see key-decisions).
- The plan's must_have for item (4) says to state "unproven" if V1 failed. V1 failed but was superseded by V1b/F13 passing, so M-176 records that history and asks for the notarized-build banner instead, per the closure in 19-VM-CHECKS.md.
- `internal/desktop/export_test.go` and `fakes_test.go` hold no tests; their rows say so.
- The DSHR-01 Section 4 rows number seven (one per file with a Phase 19 DSHR-01 test); `internal/daemon/process_test.go` was edited in 19-02 by only three lines and got no row, since it gained no Phase 19 test.

## Known Stubs

None. TESTING.md only.

## Threat Flags

None. No code, endpoint or file access changed; T-19-50 mitigated (no addresses, names, tokens or home paths in the new text; check-brand passes).

## Self-Check: PASSED

- TESTING.md contains `Category BJ`, `Portty Phase 19 (unattended desktop viewing` and the 54 Phase 19 rows: FOUND.
- Commits `0eec059b` and `d4ad310e` are ancestors of HEAD: FOUND.
