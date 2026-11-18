---
phase: 05-overnight-survival
plan: 15
subsystem: testing
status: complete
gap_closure: true
tags: [testing-md, traceability, todos, regression-gate, toolchain, shar-07, verf-01]
requires:
  - phase: 05-14
    provides: "05-VM-MATRIX.md resilience rows R1-R9"
  - phase: 05-16
    provides: "R6 PASS re-run, app_respawn_test.go, cmd_attach_test.go cases"
provides:
  - "TESTING.md Section 2: Go 123, vitest 86, Playwright 7, build-script 1, total 217; Go run command under GOTOOLCHAIN=go1.26.11; a Toolchain note; one Phase 5 gap-closure note (05-08..05-16)"
  - "TESTING.md Section 4: SHAR-07 rows for replay_fidelity_test.go, wakewatch_test.go, tailscale_json_test.go, TerminalPanel.hot-swap.test.tsx, tests/build-script.test.sh; VERF-01 rows for replay_fidelity_test.go and TestCLIAttach_LongOutputDuringGap; resume_test.go and loop_test.go rows extended"
  - "TESTING.md Section 5: wake caveat rewritten (about 10 s recovery), viewing-daemon respawn caveat, long-session reattach step in M-84 (step 5) and M-85 (step 4), M-88, M-89, M-90 (with the 2026-11-18 matrix results)"
  - "Five gap-source todos in .planning/todos/completed/ with Resolution sections"
affects: [phase-05-verification, gsd-ship]
tech-stack:
  added: []
  patterns:
    - "Newest gap-closure note goes on top of Section 2; the Toolchain note sits above it"
key-files:
  created:
    - .planning/phases/05-overnight-survival/05-15-SUMMARY.md
  modified:
    - TESTING.md
    - .planning/todos/completed/2026-11-17-jumbled-screen-on-reattach.md
    - .planning/todos/completed/2026-11-17-duplicate-remote-tab-blank-window.md
    - .planning/todos/completed/2026-11-17-network-resilience-matrix.md
    - .planning/todos/completed/2026-11-17-pin-go-toolchain-for-tailscale.md
    - .planning/todos/completed/2026-11-17-guest-scrollbar-at-pane-edge.md
key-decisions:
  - "TerminalPanel.hot-swap.test.tsx and tests/build-script.test.sh had no traceability row to extend, so each got a new SHAR-07 row"
  - "The gap-closure note covers 05-08..05-16 (05-16 landed before this plan), not only 05-08..05-14"
  - "M-90 records R3 as covering an interface flap and address changes only; a real Wi-Fi access-point switch stays with M-85"
requirements-completed: []
duration: 20min
completed: 2026-11-18
estimate:
  tokens: 45000
  tasks: 2
actuals:
  tokens: 5400
  tasks: 2
  commits: 2
plan_head_before: 443f33ff9c22d741e165786ca0ac9d9ed64131f0
plan_head_after: 8dd047e059986785cd3dff577b7fe144a9538615
---

# Phase 5 Plan 15: Gap-closure close-out Summary

TESTING.md now matches the suite on disk after 05-08..05-16 (123 Go, 86 vitest, total 217), documents the go1.26.11 toolchain pin, traces every new gap-run test file, and carries M-88..M-90. The five gap-source todos are filed as completed, and the full regression gate passed on the pinned toolchain with no flakes.

## What was done

**Task 1: TESTING.md** (`6df3eb5f`)
- Section 2: recounted with `git ls-files`. Go 123 is 118 plus replay_fidelity_test.go, wakewatch_test.go, tailscale_json_test.go, app_wake_test.go and app_respawn_test.go. Vitest 86 is 83 plus ErrorBoundary, replayFidelity and App.remote-poll. The Go run command is now `GOTOOLCHAIN=go1.26.11 go test -race -short ./...`.
- Added a Toolchain note: the go.mod pin, why `GOTOOLCHAIN=auto` does not downgrade, why the root package refuses Go 1.27 (tailscale.com v1.98.3 and tailcfg's `format:nano` tag), and how to remove the pin (canary first, then the go.mod line and toolchain_guard.go together).
- Added one gap-closure note listing the new and edited test files with test names, plus one line per production change.
- Section 4: added 5 SHAR-07 rows and 2 VERF-01 rows, and extended 2 existing rows. The earlier plans had already added rows for app_wake, app_respawn, cmd_attach (05-16), App.nav, App.remote-poll, ErrorBoundary, replayFidelity, tabs, TerminalPanel.scale and the relayClient/reconnect extensions. Traceability now checks 129 paths.
- Section 5: rewrote the wake caveat. A wake or network change now probes or redials at once, and recovery takes about 10 s (R4). The caveat says where the 45 s watchdog still applies. Also:
  - a new caveat for the viewing-daemon respawn (05-16);
  - a long-session reattach step in M-84 and M-85;
  - M-88 (duplicate tab and close, V-09) and M-89 (reattach rendering and scrollbar, V-12);
  - M-90 (the R1-R9 matrix with its 2026-11-18 per-row results: 9 PASS / 0 FAIL).

**Task 2: todos and gate** (`8dd047e0`)
- Moved the five todos with `gsd-tools todo complete`. Each got a Resolution section that names its plans, commits, pinning tests and remaining manual items: M-84/M-85 for the jumbled screen, M-85 for a real AP switch and lid sleep, and the tailscale.com upgrade for the toolchain pin.
- The other six pending todos are untouched.

## Full gate (GOTOOLCHAIN=go1.26.11, go1.26.11 darwin/arm64)

| Check | Result |
|-------|--------|
| `go build ./...` | ok |
| `go vet ./...`, `-tags playwrightfixture ./cmd/playwright-fixture/`, `-tags vmpreview ./...` | ok |
| `go test -race -short -count=1 -skip '^(TestAPIAppearance_GetPatch\|TestAPITerminalFont_GetPatch)$' ./...` | exit 0, 12 packages ok; 736 top-level PASS (950 including subtests), 5 SKIP, 0 FAIL |
| `bash tests/build-script.test.sh` | 40 passed, 0 failed |
| `pnpm -C frontend exec tsc --noEmit` | ok |
| `vitest run --maxWorkers=4` | 86 files passed, 1160 tests passed |
| `check-traceability-paths.sh` / `check-brand.sh` / `check-share-removal.sh` | OK (129 paths) / clean / clean |

No known flake (SharePopover Copy, initialGrid jsdom, TestServer_LoopbackOrigin127Accepted, TestCLIAttach_LongHistoryJoiners) fired on this run.

## Deviations from Plan

- **Scope 05-08..05-16 instead of 05-08..05-14:** 05-16 landed before this plan and added app_respawn_test.go, so the counts and the note include it. Its rows were already in Section 4.
- **New rows instead of extensions** for TerminalPanel.hot-swap.test.tsx and tests/build-script.test.sh: neither file had a traceability row to extend.
- **Stale caveat already gone:** the "no system wake hook yet" wording had already been replaced in 05-13. The caveat was still rewritten to the post-gap behavior (about 10 s recovery, where the 45 s watchdog still applies).
- **M-85 step renumbering:** the new step 4 moved the viewer-diag step to 5, which now points at M-84 step 6.

Otherwise the plan ran as written. REQUIREMENTS.md and the ROADMAP success criteria were not touched, and requirements.mark-complete was not run.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: TESTING.md (GOTOOLCHAIN=go1.26.11, M-88/M-89/M-90, 6+ SHAR-07 rows for the new files)
- FOUND: all five files in .planning/todos/completed/, none in pending/
- FOUND: 6df3eb5f, 8dd047e0 (ancestors of HEAD)
