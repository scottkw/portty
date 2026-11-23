---
phase: 16-v1-2-cleanup
plan: 04
subsystem: regression-home
tags: [testing-md, dispositions, full-suite-gate, todos]
status: complete

requires:
  - phase: 16-v1-2-cleanup
    provides: 16-01 CLI fixes, 16-02 review cleanup, 16-03 planning hygiene
provides:
  - "TESTING.md Phase 16 Section 2 note, corrected PATH-02 row, Category BG with M-156 (not yet run)"
  - "15-REVIEW-DISPOSITION.md IN-02, IN-03, IN-06 fixed with 16-02 commits; IN-05 still open"
  - "One clean full Go -race and vitest run on the phase's final tree, logs kept"
  - "Three fixed todos moved to todos/completed with Resolution sections"
affects: [/gsd-secure-phase 11 (orchestrator), /gsd-verify-work 16 (runs M-156 in the Tart VM), v1.2 milestone close]

actuals:
  tokens: 2400
  tasks: 3
  commits: 2
plan_head_before: 02f7675f813707b7c566f2f8c93452db629799a5
plan_head_after: 9f9c9aca4b1f138c6ed0b2e665e7d9e87ef0b11d

tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - TESTING.md
    - .planning/phases/15-v1-2-hardening/15-REVIEW-DISPOSITION.md
    - .planning/todos/completed/2026-11-23-kill-by-name-fails.md
    - .planning/todos/completed/2026-11-23-version-flag-hangs.md
    - .planning/todos/completed/2026-11-23-dos-bound-test-timeouts.md

key-decisions:
  - "IN-05 stays open: M-152's relaunch step relaunches first and then clicks the notification, which is not the cold launch-from-notification path IN-05 names; Phase 16 SC4 lists only IN-02, IN-03, IN-06"
  - "PathClickAPI removal (a Phase 12 review item) is recorded in the Section 2 note and PATH-02 row, not in the Phase 15 disposition file"
  - "M-156 schedules the real launch check in the Tart VM; it must PASS at /gsd-verify-work before ship"

requirements-completed: []

coverage:
  - id: D1
    description: "TESTING.md and the review dispositions record every Phase 16 test change; traceability passes"
    verification:
      - kind: other
        ref: "bash tests/check-traceability-paths.sh -> OK: 228 traceability paths exist; Task 1 verify block passed"
        status: pass
    human_judgment: false
  - id: D2
    description: "Full Go -race -short suite, full vitest suite, go vet, gofmt, tsc, check-brand, check-share-removal pass once, clean"
    verification:
      - kind: other
        ref: "t2.sh single run, exit 0, t2-done present; 14 of 14 ok packages; vitest 122 files / 1826 tests passed"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every launch path still opens Portty and CLI flags never start the GUI (M-156)"
    verification: []
    human_judgment: true
    rationale: "Real LaunchServices launches of a signed bundle cannot run in process or on the user's Mac; M-156 runs in the Tart VM at /gsd-verify-work"
  - id: D4
    description: "Three fixed todos moved to completed with Resolution sections; two out-of-scope todos stay pending"
    verification:
      - kind: other
        ref: "Task 3 verify block passed"
        status: pass
    human_judgment: false
---

# Phase 16 Plan 04: Close-out Summary

**TESTING.md records Phase 16 (Section 2 note, PATH-02 row fixed, Category BG with M-156), IN-02/IN-03/IN-06 are marked fixed, the full Go `-race` and vitest suites passed once and clean (14 of 14 packages, 1826 of 1826 tests), and the three fixed todos are in completed.**

## Performance

- **Duration:** about 25 min wall clock, most of it the single full Go run (relay 202 s, daemon 98 s)
- **Tasks:** 3 of 3
- **Commits:** 2 task commits (Task 2 changes no files)

## Accomplishments

- **Task 1 (tracer, 73a55889):** the Phase 16 Section 2 note sits above the Phase 15 note with counts from `git ls-files` (Go 174, vitest 122, Playwright 10, build-script 1, total 307; no new test files) and the edited-in-place list from 16-01 and 16-02. The `PATH-02 | internal/daemon/client_test.go` row no longer claims a path-click pin. Category BG holds **M-156**, with `_Last run:_ not yet run`. `15-REVIEW-DISPOSITION.md` rows IN-02 (97611082), IN-03 (0af0cd46) and IN-06 (97611082) read `fixed`; IN-05 is unchanged and open. The `updated:` date was already 2026-11-23, so it did not change.
- **Task 2 (single run, no commit):** see below.
- **Task 3 (9f9c9aca):** `git mv` of the three todos to `.planning/todos/completed/`, each with a `## Resolution` naming its plan and commit (kill-by-name-fails: 16-01, e6ec4050; version-flag-hangs: 16-01, 9321f505; dos-bound-test-timeouts: 16-02, 7aa3fbb3). `2026-11-21-dompurify-3418.md` and `2026-11-21-v12-release-checks.md` remain in pending.

## Task 2: the phase's one full run

- Run once, as one bash invocation of the plan's script; exit status 0; `t2-done` present. It was not re-run and no step was run separately beforehand.
- **Log directory:** `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.sAUD92yTnm` (pointer file `${TMPDIR}/portty-16-04-t2-logdir`; script text at the session scratchpad `t2.sh`).
- gofmt listed no changed Go file; go vet, tsc, check-brand and check-share-removal exited 0.
- **Go:** `go test -race -short -count=1 -timeout 30m ./...`: packages with test files per `go list`: 14; `ok` lines: 14; no `--- FAIL` or `FAIL` line. Package times: root 36.7 s, attach 6.5, capability 1.0, daemon 97.8, fdpass 1.0, previewfile 1.0, pty 7.8, relay 202.1, release 18.3, statusbar 5.5, tailnet 6.9, updater 2.5, wakewatch 1.0, webserver 23.7.
- **vitest:** `Test Files  122 passed (122)`, `Tests  1826 passed (1826)`. The two DoS-bound cases passed inside the full run.
- No flakes, so no targeted re-run was needed.

## Deviations from Plan

None - plan executed exactly as written. The plan's script was written to a scratchpad file and run in the background (it exceeds the 10 minute foreground cap); its text is identical to the plan's thirteen lines.

## Notes for the orchestrator and verifier

- `/gsd-secure-phase 11` (11-SECURITY.md, ROADMAP SC5) was not run and its file not written, per the plan; the orchestrator runs it before `/gsd-verify-work 16`.
- **M-156 has not run.** It must PASS in the Tart VM `portty-test` at `/gsd-verify-work` before ship (T-16-11: the remote Mac must always open).
- IN-05 stays open deliberately (see key-decisions); the remaining Phase 15 manual item is its cold-launch notification path.
- Unrelated uncommitted files (.gitignore, .planning/state.json, .ignore, agenthub, .gitkeep files) were left unstaged.

## Known Stubs

None.

## Threat Flags

None. T-16-09 mitigated (traceability script exits 0, 228 paths); T-16-10 mitigated (one run, logs kept, ok count equals the `go list` count); T-16-11 mitigated by scheduling M-156 (not yet run).

## Commits

| Task | Commit | Description |
| ---- | ------ | ----------- |
| 1 | 73a55889 | TESTING.md Phase 16 note, PATH-02 row, Category BG M-156; IN-02/03/06 fixed |
| 2 | (none) | Single full Go and vitest run, logs only |
| 3 | 9f9c9aca | Three fixed todos moved to completed with Resolution sections |

## Self-Check: PASSED

- TESTING.md, 15-REVIEW-DISPOSITION.md and the three completed todos exist and carry the edits; the three pending paths are gone.
- Commits 73a55889 and 9f9c9aca are ancestors of HEAD (2 commits since plan_head_before 02f7675f).
