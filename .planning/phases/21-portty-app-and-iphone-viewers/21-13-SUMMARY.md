---
phase: 21-portty-app-and-iphone-viewers
plan: 13
subsystem: docs, regression-suite
tags: [testing-md, traceability, readme, uat, deferred]
requires:
  - phase: 21-portty-app-and-iphone-viewers
    provides: the test files of 21-01 to 21-10 and the VM rows of 21-02, 21-11 and 21-12
  - phase: 20-remote-control
    provides: Category BK and M-193 (Phase 21 numbers after it)
provides:
  - TESTING.md Sections 2, 3 and 4 for Phase 21 and Section 5 Category BL (M-194..M-205)
  - README Desktop (other Mac) and Desktop (iPhone) paragraphs
  - 21-VM-CHECKS.md device table with DEVICE_ROWS and one status line per requirement
affects: [22-13, verify-work for Phase 21]
tech-stack:
  added: []
  patterns:
    - "Requirement status computed from M-NN items plus VM closure lines: verified, partial (deferred items named) or failed"
key-files:
  created: []
  modified:
    - TESTING.md
    - README.md
    - .planning/phases/21-portty-app-and-iphone-viewers/21-VM-CHECKS.md
key-decisions:
  - "All 12 Phase 21 M-NN items are DEFERRED (the owner was not available); DVWR-01, DVWR-03 and DVWR-04 read partial, never verified."
  - "The README paragraph and every M-NN step use the shipped release chord Ctrl+Option+Cmd+Shift+Return, not the UI-SPEC's Ctrl+Option+Cmd+Return."
  - "Section 2 counts are the tree counts (Go 259, vitest 139, total 410) and name the 19 Phase 22 Go files that were already on the branch."
requirements-completed: []
requirements-partial: [DVWR-01, DVWR-03, DVWR-04]
status: complete
actuals:
  tokens: 45000
  tasks: 3
  commits: 2
plan_head_before: a0b538578378149cdc272671cbb04c9135463abe
plan_head_after: 1c046bce732fdcfb443b43068336b52e70514da5
duration: one session
completed: 2026-11-27
---

# Phase 21 Plan 13: TESTING.md closure, README help text and the device record Summary

**Phase 21's tests and manual items are now in TESTING.md (Sections 2, 3, 4 and Category BL with M-194 to M-205), the README explains both Desktop viewers, and every real-device item is recorded DEFERRED, so DVWR-01, DVWR-03 and DVWR-04 read `partial`.**

## What was done

- **Task 1 (tracer), commit `4caa9fd2`.** Precondition held (20-11 and 20-02 SUMMARYs exist). Section 2: counts recounted from `git ls-files` (Go 259, vitest 139, Playwright 10, build-script 2, total 410), Guards cells extended, and a Phase 21 note listing every Phase 21 test file and the in-place edits. Section 3: one sentence that `build.yml` is unchanged (`git diff 6662f196 HEAD -- .github` is empty). Section 4: 28 rows (16 DVWR-01, 5 DVWR-04, 7 DVWR-03) with repo-relative paths only. Section 5: Category BL, M-194..M-205 (continuing after M-193; no higher M-NN existed, so Phase 22 had not taken numbers). README: the two Surface 5 paragraphs appended, 4 added lines and 0 deleted.
- **Task 2 (checkpoint:human-action), not returned.** The user is not available and must not be waited on, so the reply was treated as "defer" for every Phase 21 M-NN item.
- **Task 3, commit `1c046bce`.** Each M-NN carries `_Last run:_ 2026-11-27, DEFERRED (deferred to user UAT, 21-UAT.md ...)`; the status line sits beside the Category BL heading; 21-VM-CHECKS.md gained the device table, `DEVICE_ROWS: 0 pass, 0 fail, 12 deferred` and one status line per requirement.

## Phase 21 M-NN items (Category BL)

| Item | Req | Title | Result |
|------|-----|-------|--------|
| M-194 | DVWR-03 | One-finger trackpad, tap, double-tap timing, press-and-hold right click with no iOS callout | DEFERRED |
| M-195 | DVWR-03 | Tap then drag a window; two-finger scroll direction | DEFERRED |
| M-196 | DVWR-03 | Pinch to 5x, pan-follow, resets, no Safari tab overview | DEFERRED |
| M-197 | DVWR-03 | Soft keyboard: type, Backspace, Return, armed modifiers, form-assist gap | DEFERRED |
| M-198 | DVWR-03 | Send Keys, Lock Screen confirm, surface picker on the phone | DEFERRED |
| M-199 | DVWR-03 | Sleep and resume; Tap to start card only when iOS needs it | DEFERRED |
| M-200 | DVWR-03 | Home Screen relaunch keeps the link; clean console under the strict CSP | DEFERRED |
| M-201 | DVWR-01 | Desktop row on the home Mac: Local Network prompt, sharp picture, direct tailnet path | DEFERRED |
| M-202 | DVWR-04 | Control mode: Cmd+W, Cmd+Q, Cmd+C, release chord Ctrl+Option+Cmd+Shift+Return, plain click after it | DEFERRED |
| M-203 | DVWR-04 | Cmd+V including a password-manager item | DEFERRED |
| M-204 | DVWR-01 | Hidden window lets the office display sleep after 2 minutes, battery and power | DEFERRED |
| M-205 | DVWR-01, DVWR-04 | Label, chips and cards legible in Light and Dark over the translucent window | DEFERRED |

The two items 21-12 handed to real-Mac UAT are covered: Light/Dark legibility is M-205, and "after the release chord a plain click is a plain click" is folded into M-202 (step 4 and the PASS text).

## Status lines (as written to 21-VM-CHECKS.md)

```
DEVICE_ROWS: 0 pass, 0 fail, 12 deferred
DVWR-01: partial (deferred M-201, M-204, M-205; DVWR01_VM pass)
DVWR-03: partial (deferred M-194, M-195, M-196, M-197, M-198, M-199, M-200; no VM row covers it)
DVWR-04: partial (deferred M-202, M-203, M-205; DVWR04_VM pass)
```

## Gaps for the verifier

- **DVWR-01, DVWR-03 and DVWR-04 are partial, not verified.** Every real-device item is DEFERRED to user UAT (`21-UAT.md`, not yet created). DVWR-03 has no VM evidence at all (phone-only behaviour), so all of its truths rest on M-194..M-200. The four backstop truths of this plan (iPhone gestures, pinch and tab overview, soft keyboard, sleep/Home Screen/CSP) are unverified.
- The VM evidence from 21-11 and 21-12 stands: `DVWR01_VM: pass`, `DVWR04_VM: pass`, `PASTE_ALERT: none`.
- No item failed.

## Deviations from Plan

**1. [Rule 1 - Outdated plan text] Release chord.** The plan (Task 1 verify, the README artifact `contains`, Task 2 text) names Ctrl+Option+Cmd+Return; 21-02 showed the system takes that chord and 21-09 shipped Ctrl+Option+Cmd+Shift+Return, which the label shows. The README paragraph and every M-NN step use the shipped chord, so the plan's automated line `grep -c 'Ctrl+Option+Cmd+Return' README.md -ge 1` would read 0; I checked `Ctrl+Option+Cmd+Shift+Return` instead (count 1). The UI-SPEC still carries the old chord in its Surface 5 text, Copywriting Contract and label rows; not edited here (outside this plan's files).
- **Files:** README.md, TESTING.md. **Commit:** `4caa9fd2`

**2. [Rule 3 - Decision] Task 2 not returned as a checkpoint.** Treated as all-deferred per the orchestrator's decision; Task 3 then ran as written.

**3. [Process] Commit trailer.** The harness prescribes `Co-Authored-By: Claude Sonnet 5.5`; used it instead of the plan's Claude Opus 5.5 (same call 21-10 to 21-12 made).

**4. [Judgment] Section 2 counts include Phase 22 files.** The tree already holds 19 Phase 22 Go test files that Phase 22 had not yet recorded. The counts follow the file's own method (`git ls-files`) and the note names those files, as Phase 20's note did for Phase 21. Phase 22's plan 22-13 recounts from the tree and adds its own rows and category (it follows this plan). If its recount runs against a tree where those 19 files are already in this line, it should replace the figures, not add to them.

**5. [Judgment] M-NN count and mapping.** The plan's truths name the phone and Mac rows as groups; they are split into 12 items (M-194..M-205), each named in the table above. The items for the phone and the Macs are in one Category BL (next letter after BK).

No Go or frontend test was run (nothing in them changed); this plan edits documents only.

## Verification

- `bash tests/check-traceability-paths.sh`: OK, 313 paths exist. `bash tests/check-brand.sh`: clean.
- Section 4 floors: DVWR-01 16 (>=6), DVWR-03 7 (>=4), DVWR-04 5 (>=2). README `Pinch zooms up to 5x` count 1; README diff 4 added, 0 deleted.
- Category BL first M-NN is M-194 = previous highest M-193 plus one.
- Task 3 automated checks: `DEVICE_ROWS` line exactly once; the three status lines present; with a deferred count above zero, DVWR-0[134] reads partial.

## Known Stubs

None. (The `21-UAT.md` named in the deferral text does not exist yet; `/gsd-verify-work` creates it.)

## Threat Flags

None. No hostname, address, token or home path was written (`check-brand.sh` clean). T-21-42 to T-21-44 mitigated: brand gate and traceability script run before each commit; every item recorded verbatim as DEFERRED, none dropped.

## Self-Check: PASSED

TESTING.md, README.md and 21-VM-CHECKS.md exist and carry the changes; commits `4caa9fd2` and `1c046bce` are ancestors of HEAD.
