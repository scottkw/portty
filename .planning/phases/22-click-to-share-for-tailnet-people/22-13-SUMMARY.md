---
phase: 22-click-to-share-for-tailnet-people
plan: 13
subsystem: docs-and-security-review
tags: [testing-md, traceability, manual-checklist, readme, security-review, phase-gate]
requires:
  - phase: 22-12
    provides: 22-VM-CHECKS.md rows, evidence files and the owner-run candidate list
provides:
  - "TESTING.md Sections 2, 3, 4 and 5: Phase 22 counts and note, 40 traceability rows, Category BM with M-206..M-220"
  - "README Share Desktop (guests) paragraph with the tcp/7444 and udp/7444 access-rule note"
  - "22-SECURITY.md: register T-22-01..T-22-40 and T-22-SC, the ten review cases, accepted risks AR-22-01..08, flagged probe assumptions, threats_open 0"
affects: []
key-files:
  created:
    - .planning/phases/22-click-to-share-for-tailnet-people/22-SECURITY.md
  modified:
    - TESTING.md
    - README.md
key-decisions:
  - "threats_open is 0 at test depth: the identity threats (T-22-02, T-22-13, T-22-16) rest on the fake WhoIs; the live two-account proof (SC1, M-206) and live own-account filter (AR-21-01, M-207) stay UNVERIFIED and owner-run"
  - "T-22-19 (Stop delayed by silent guests, medium) is recorded open below the high threshold, because VM G-04 saw one 6.9 s straggler under heavy host load; M-215 is the quiet-machine re-measure"
  - "Phase 23 test files already merged on this branch are counted in Section 2 but not given Section 4 rows or Section 5 items; Phase 23 records them"
requirements-completed: []
status: complete
commits: 3
plan_head_before: b6bcc1f0542d2b4632a7adb16721f8885bfc204d
plan_head_after: aeff234576f66536dcd6b099009c3472b1eeee1e
duration: about 1 h 40 min
completed: 2026-11-27
actuals:
  tokens: 52000
  tasks: 3
  commits: 3
---

# Phase 22 Plan 13: TESTING.md, README and the recorded security review Summary

**Phase 22 is recorded in the regression suite (40 traceability rows, 15 owner-run items M-206..M-220 with SC1 and AR-21-01 marked UNVERIFIED), the README names the tcp/7444 and udp/7444 access rules a guest link needs, and 22-SECURITY.md records the review of the widening with threats_open 0 and the ten cases each tied to a passing test or VM row.**

## Preconditions

Hard gate passed: 20-11, 21-13 and 22-12 SUMMARY files all exist. `git log --oneline 07ad7d21..HEAD -- TESTING.md README.md .github/workflows/build.yml` listed, and was read before editing: `596b413d` (README iPhone Desktop copy), `9ef402e6` (WR-01 retryable refused peer port), `1c046bce` (21-13 device results, all 12 deferred), `4caa9fd2` (21-13 TESTING.md Sections 2, 4, 5 and the README Desktop paragraphs), `943f483e` (20-11 Category BK, M-180..M-193) and `ce91e3c1` (20-11 Section 2 and 4). `build.yml` has no change since the start of Phase 22, so branch protection needs no re-apply.

## What was done

- **Task 1 (tracer, TESTING.md):** Section 2 counts now Go 273, vitest 149, Playwright 10, build-script 2, total 434, with a Phase 22 note listing every new file by plan, the files edited in place, and the Phase 23 files already merged. Section 3 notes build.yml is unchanged. Section 4 has 40 rows (DSHR-03: 10, DSHR-04: 11, DSHR-05: 5, DSHR-06: 8, DSHR-07: 2, DCTL-04: 4), one or more per new Phase 22 test file and per edited file, repo-relative paths only. Section 5 has Category BM. `bash tests/check-traceability-paths.sh` reports 340 paths exist; `bash tests/check-brand.sh` clean. Commit `0c703850`.
- **Task 2 (README):** one "Share Desktop (guests)" paragraph in the existing style. Commit `e9bacd88`.
- **Task 3 (22-SECURITY.md and the phase gate):** the register, the ten review cases, accepted risks, flagged probe assumptions, observations and audit trail. Commit `aeff2345`.

## M-NN numbers assigned (Category BM, M-206..M-220)

| M | Item | Source row | State |
|---|------|-----------|-------|
| M-206 | SC1: second Tailscale user, wait at Admit, View only, own-devices refusal | SC1 | UNVERIFIED, never passed |
| M-207 | AR-21-01: shared-in node not offered, hand-typed connect refused | AR-21-01 | UNVERIFIED |
| M-208 | Restrictive tailnet access rule: Can't reach this Mac with the hint | owner item 11 | NOT RUN |
| M-209 | Tray sharing icon legible in light and dark menu bars | G-08 | VM light only |
| M-210 | Admit sheet while typing in a real terminal | owner item 4 | deferred |
| M-211 | Right-to-left or look-alike display name on the real Admit sheet | owner item 5 | deferred |
| M-212 | "Someone is using this Mac" with real hardware input | G-05 | deferred (VM used a software stand-in) |
| M-213 | Warp check and real Secure Keyboard Entry (completes Appendix B) | G-06, Appendix B | PARTIAL, evidence/appendix-b-real-mac.txt |
| M-214 | M-05 with a real non-activating panel, `window_key_focus` | G-11 | PARTIAL |
| M-215 | Stop with two real guests ends both quickly | G-04 | PARTIAL (6.9 s straggler under load) |
| M-216 | Admit sheet above other sheets, window forward on a knock, banner click routing | 22-09, 22-11 | not driven |
| M-217 | Join, leave and admit banners visible while the display is captured | G-07 | delivery passed, banners not seen |
| M-218 | Share stopped banner after the daemon dies; upgrade handoff with a share | 22-06, 22-11 | not driven |
| M-219 | Quit GUI Only while sharing; GUI crash ends the share | G-09, 22-09 | VM GUI-death passed, QuitGUIOnly not driven |
| M-220 | VoiceOver reads the Admit sheet, announcements and grant switches | 22-10 | not driven |

All fifteen read "Last run: not yet run". None is marked passed.

## Phase gate (run in this worktree, 2026-11-27)

| Check | Result |
|-------|--------|
| `go vet ./...` | clean |
| `gofmt -l` over tracked Go files | no file listed (one stderr parse error for the stray tracked `.claire/` placeholder, pre-existing) |
| `golangci-lint run --new-from-rev=07ad7d21 ./...` | 0 issues |
| `go test -race -count=1 ./...` | 14 packages ok in the first run; four timing-sensitive tests failed under host load (load average 16 to 31, vitest running beside it) |
| the failed packages, re-run alone | `./internal/daemon ./internal/desktop` ok; the two daemon tests also ok on their own |
| `tsc --noEmit` | clean |
| `vitest run` | second full run 149 files, 2719 tests pass (the first run had one 5 s timeout in `pathLinks.linkifier.test.ts` under the same load; it passed alone) |
| `check-brand.sh`, `check-traceability-paths.sh` | clean; 340 paths |
| `threats_open: 0` in 22-SECURITY.md | yes |

The four first-run failures were: `TestDesktopShareAPI_StopCutsGuests` (Phase 22, timed out at 5 s waiting for the inactive state line), `TestGetSessionStyledTailLines_UnstrippedQueryNeverHangs`, and `TestQualityShaped_SlowViewerDoesNotStallOthers` and `TestQualityShaped_StillScreenDoesNotCollapseEstimate` (Phase 23). They are load flakes by the evidence (each passed on re-run) and were not changed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Frontend gate run without pnpm.** The worktree has no `frontend/node_modules` and `pnpm` refuses a symlinked one ("workspace hoist directory is not a real directory"), so tsc, vite build and vitest ran through `frontend/node_modules/.bin` with the main checkout's `node_modules` symlinked in. `vite build` produced the gitignored `frontend/dist` the root package embeds. The symlink was removed before the SUMMARY; nothing from either is committed.

### Process

**2. [Process] Counts include Phase 23's files.** The branch already holds Phase 23 plans 23-01..23-06, so the Section 2 counts (Go 273, vitest 149) include 12 Go and 2 vitest Phase 23 files, named in the note for Phase 23 to record in its own Sections 2, 4 and 5. This follows the Phase 21 note's precedent. The Phase 21 note's Go count of 259 was one high (258 at that commit); the note says so.

**3. [Process] Commit trailer.** The harness prescribes `Claude Sonnet 5.5`, which wrote these commits; the dispatch's `<your actual model name>` is the same.

## Auth gates

None.

## Known Stubs

None. Documentation only.

## Threat Flags

None new. The documents name no real tailnet, host, address or token (`check-brand.sh` clean); T-22-40 is closed by 22-SECURITY.md.

## Items for the orchestrator

- **threats_open 0 rests on test depth.** Anyone who wants a live proof of the identity gate before sharing with other people should run M-206 and M-207 first. If you want that treated as an open high threat instead, change the `status` of T-22-02, T-22-13 and T-22-16 in 22-SECURITY.md and `threats_open` to 3.
- **T-22-19 / G-04 (DSHR-05):** the 6.9 s straggler is open below the high threshold; the quiet-machine re-measure is M-215.
- **Branch protection:** unchanged; `build.yml` was not touched by Phase 22.
- The tracked file `.claire/worktrees/agent-aac25194/internal/webserver/csp_integration_test.go` is a placeholder from an old commit; it breaks `gofmt -l` with a stderr parse error. Not touched.

## Self-Check: PASSED

Files verified present: TESTING.md (Category BM, M-206..M-220), README.md (`tcp/7444`, `udp/7444`, `access rules`), 22-SECURITY.md (`threats_open: 0`, 40 `T-22-NN` rows). Commits `0c703850`, `e9bacd88` and `aeff2345` are ancestors of HEAD. STATE.md and ROADMAP.md untouched. `bash tests/check-traceability-paths.sh` and `bash tests/check-brand.sh` clean.
