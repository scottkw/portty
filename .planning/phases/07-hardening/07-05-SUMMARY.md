---
phase: 07-hardening
plan: 05
subsystem: testing, vm-verification
status: complete
tags: [HARD-01, HARD-02, HARD-03, HARD-04, HARD-05, HARD-06, HARD-07, HARD-08, testing-md, gate, tart-vm]
requires: [07-01, 07-02, 07-03, 07-04]
provides:
  - "TESTING.md reconciled for Phase 7: Go 134, vitest 90, total 232; HARD-01..HARD-07 Section 4 rows; Category AV (M-94..M-97)"
  - "Full gate green twice on the developer Mac with no -skip"
  - "07-VM-CHECKS.md: H-01..H-04 PASS, H-05 FAIL (in-alt peak 0.19 MiB over the plan's ceiling)"
affects: [TESTING.md, .planning/phases/07-hardening/07-VM-CHECKS.md]
tech-stack:
  added: []
  patterns: ["gctrace live-heap readings mapped to script markers through a 5 s epoch monitor"]
key-files:
  created:
    - .planning/phases/07-hardening/07-VM-CHECKS.md
  modified:
    - TESTING.md
decisions:
  - "H-05 recorded as FAIL on the letter of the plan (128 - 5 = 123 MiB > 122.81 MiB) although plateau and release pass; left for the verifier / a gap plan, no product change"
metrics:
  duration: "~75 min"
  completed: 2026-11-18
estimate:
  tokens: 120000
  tasks: 2
actuals:
  tokens: 5500
  tasks: 2
  commits: 2
plan_head_before: 2dfde6a6227a0e6722d426f7e7537725542a3982
plan_head_after: 816cf46f
---

# Phase 7 Plan 05: TESTING.md reconciliation, no-skip gate, Tart VM checks Summary

TESTING.md now lists every Phase 7 test (Go 134, vitest 90, total 232, HARD-01..HARD-07 rows, M-94..M-97). The full gate passed twice on this Mac with nothing skipped. In the Tart VMs, large pastes (GUI peer tab and `portty attach`), the delta tab-switch rejoin and the no-respawn launch all pass. The alt-screen soak (H-05) is bounded and fully released on exit, but its in-alt peak reads 0.19 MiB over the plan's ceiling, so it is recorded as FAIL.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Full regression gate with nothing skipped, TESTING.md reconciled | fb72d7b9 | TESTING.md |
| 2 | Tart VM spot checks H-01..H-05 | 816cf46f | 07-VM-CHECKS.md, TESTING.md |

## Final test-file counts (git ls-files)

- Go `*_test.go`: 134 (129 + querystrip, altscreen_heap, testmain, relay input_limit, webserver input_limit)
- vitest: 90 (88 + peerSnapshots.test.ts, TerminalPanel.tabSwitch.test.tsx)
- Playwright 7, build-script 1, total 232. All match the plan's expectations.

## Gate (run once, Go suite twice, GOTOOLCHAIN=go1.26.11)

| Step | Result | Wall time |
|------|--------|-----------|
| go build ./... | ok | 5 s |
| go vet ./... | ok | 1 s |
| go test -race -short -count=1 ./... (run 1) | all ok: root 22.868s, attach 5.896s, daemon 18.286s, relay 43.378s, webserver 12.662s | 48 s |
| go test -race -short -count=1 ./... (run 2) | all ok: root 23.867s, attach 5.984s, daemon 18.926s, relay 44.954s, webserver 13.128s | 46 s |
| pnpm vitest run | `Test Files 90 passed (90)`, `Tests 1218 passed (1218)` | 12 s |
| pnpm exec tsc --noEmit | clean | 2 s |
| tests/check-brand.sh | `OK: brand gate clean` | 0 s |
| tests/check-share-removal.sh | `OK: share removal gate clean` | 1 s |
| tests/check-traceability-paths.sh | `OK: 146 traceability paths exist` | 0 s |
| tests/build-script.test.sh | `Results: 40 passed, 0 failed` (static checks only; no Wails build) | 0 s |

- No flake reruns were needed.
- `git grep -e '-skip ' -e '-skip='` over TESTING.md, .github, tests, scripts and build.sh: no match (exit 1).
- frontend/src/wailsjs: clean after the gate and after the Wails app build.
- Tracer gate: the cheap Task 1 verify checks (HARD rows, Category AV, Phase 7 note, Go count, traceability) were re-run after the commit. The full suite was not re-run a third time (user rule: no extra full-suite loops).

## VM checks (07-VM-CHECKS.md)

| Row | Result | Key evidence |
|-----|--------|--------------|
| H-01 | PASS | Cmd+V of 100,000 base64 bytes into a home GUI tab on office h1: office file 100000 bytes, sha256 equal to the payload's; office `input total ... frames=74 bytes=100070`; no 1009 close |
| H-02 | PASS | Same payload through `portty attach` (expect FIFO, one syswrite): office file 100000 bytes, sha256 equal; `frames=103 bytes=100070`; only close is the detach (code 1000) |
| H-03 | PASS | Three returns to the ticker tab: `mode=delta bytes=260/200/200 resume=true`; copied tab has ticks 1..291, 0 dups, no jumps |
| H-04 | PASS | Three GUI-only relaunches (kill -9 of the GUI, which ignores SIGTERM): 0 respawn lines, daemon pid 14757 throughout. Positive control: killing the daemon produced `respawn attempt 1: ok` in the same capture |
| H-05 | FAIL | C = 100, ceiling 122.81 MiB. Baseline 5 MiB. In-alt peak 128 MiB in both runs (123 MiB over baseline, 0.19 MiB over), idle in-alt 127. Plateau passes (run 2 = run 1). Post-exit 6 MiB on both runs (passes) |

**HARD-04 launch diagnostic:** no `not answering right after launch` line appeared in any of the three launches (or the first launch after deploy), so there is still no root-cause error text. The respawn line format with the health error was confirmed by the positive control.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Wails build in a fresh worktree needed frontend/dist to exist**
- **Found during:** Task 2 build
- **Issue:** Binding generation compiles with `-tags wailsassets`, whose `//go:embed all:frontend/dist` fails when the (gitignored) directory is absent: `pattern all:frontend/dist: no matching files found`.
- **Fix:** created a placeholder `frontend/dist/index.html`; the Wails frontend build replaced it with the real bundle. Nothing committed (gitignored).

**2. [Rule 3 - Blocking] The GUI ignores SIGTERM**
- **Found during:** H-04
- **Issue:** the first H-04 attempt sent SIGTERM; the GUI kept running, `open` reattached to it and no capture file was written. That attempt was discarded.
- **Fix:** kill the GUI pid with `kill -9` (never the daemon). M-96's step 1 in TESTING.md now says so.

**3. Process notes**
- SSH needed `-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null` (as the `vm.sh` wrapper uses); the VMs' host keys are not in known_hosts.
- python3 in the VMs is the Xcode stub, so the WebGL toggle was done with `sed` on settings.json (backed up and restored).
- A positive control was added to H-04 (kill the home daemon, expect a respawn line) to prove the stderr capture works, because all three launch captures were empty.

## Issues for the verifier

- **H-05 FAIL.** As read, the in-alt peak is 123 MiB over baseline against a 122.81 MiB ceiling. gctrace truncates to whole MiB, so the true margin is between -0.8 and +1.2 MiB. The growth is bounded at vt's cap (same peak in both runs) and freed on exit. My reading: the plan's fixed 16 MiB headroom does not scale with width (vt's per-line overhead was 8.3 MiB at 80 columns in 07-01 and is about 10.4 MiB at 100). A gap plan could recalibrate the ceiling or take a precise heap reading. No product code was changed.
- Task 2's automated verify requires all five rows PASS, so it fails on H-05 by design of this outcome.

## Known Stubs

None.

## Threat Flags

None. No product code changed. 07-VM-CHECKS.md was checked: no `cap=` value, no share URL, no auth key.

## Self-Check: PASSED

- FOUND: TESTING.md, .planning/phases/07-hardening/07-VM-CHECKS.md
- FOUND commits: fb72d7b9, 816cf46f (ancestors of HEAD)
