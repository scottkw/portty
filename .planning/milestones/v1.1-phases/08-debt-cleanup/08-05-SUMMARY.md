---
phase: 08-debt-cleanup
plan: 05
subsystem: regression-suite, vm-verification
status: complete
tags: [DEBT-01, DEBT-02, DEBT-03, DEBT-04, DEBT-05, DEBT-06, DEBT-07, testing-md, tart-vm, gate]
requires: [08-01, 08-02, 08-03, 08-04]
provides:
  - "TESTING.md reconciled for Phase 8: Go 137, total 235, the Phase 8 note, Section 4 DEBT-01..DEBT-06 rows, M-97 restated for the fixed 32 MiB bound, Category AW (M-98..M-102)"
  - "Full gate run once with nothing skipped, all green"
  - "08-VM-CHECKS.md V-01..V-06: five PASS, V-05 FAIL recorded for the verifier"
affects: [TESTING.md, .planning/phases/08-debt-cleanup/08-VM-CHECKS.md]
requirements-completed: [DEBT-01, DEBT-02, DEBT-03, DEBT-04, DEBT-06, DEBT-07]
tech-stack:
  added: []
  patterns:
    - "Office-side watcher that drops the tailnet the moment the receiver file grows, so the cut lands mid-paste"
    - "AX hit test (AXUIElementCopyElementAtPosition) to find what swallows a rig click"
key-files:
  created:
    - .planning/phases/08-debt-cleanup/08-05-SUMMARY.md
  modified:
    - TESTING.md
    - .planning/phases/08-debt-cleanup/08-VM-CHECKS.md
decisions:
  - "V-05 graded FAIL as written: the N-of-M paste notice never appeared on the real GUI path; recorded with evidence, not fixed (plan rule), and V-checks stopped (executor rule)"
  - "V-02 wraps the ring before the wrap cases: the plan's order would push them out of the 10,000-line history"
  - "V-05 main run held the tailnet down until the reconnect banner instead of a fixed 15 s: a 15 s outage is shorter than the client's 45 s dead-socket detection"
metrics:
  duration: "~85 min"
  completed: 2026-11-19
estimate:
  tokens: 130000
  tasks: 3
actuals:
  tokens: 8200 # chars/4 over the added lines of TESTING.md and 08-VM-CHECKS.md
  tasks: 3
  commits: 3
plan_head_before: 390f832f88f6d2857357a445e3c81bc3bdbbdac4
plan_head_after: 028eccf8511a5b84bfe4dd47120d8910a7d344d2
---

# Phase 8 Plan 05: TESTING.md reconciliation, full gate and final VM checks Summary

TESTING.md now covers every Phase 8 test. The full gate passed once with nothing skipped. In the Tart VMs, five of the six checks passed: the alt-screen heap bound at 100 and 220 columns, soft wrap and rendered replay in WebKit, the peer-tab snapshot lifecycle, and quiet launches. **V-05 (DEBT-05, partial paste) FAILED as written.** Both drops mid-paste showed "Paste may have been cut short (M KB)", never "Paste interrupted: N of M KB sent". The office still got an exact prefix with nothing repeated. A 20 MB paste also hung the GUI's WebContent process for over 15 minutes. Per the executor rules I stopped there: the V-05 100 KB sub-run was not run.

## Task results

| Task | Result | Commit |
|------|--------|--------|
| 1 | TESTING.md reconciled; full gate green once | 7dcb66b2 |
| 2 | V-01, V-02, V-03 PASS; M-97..M-99 last-run lines | 440d18e0 |
| 3 | V-04 PASS, V-05 FAIL, V-06 PASS; M-96, M-100..M-102 last-run lines; both VMs cleaned up | 028eccf8 |

## Gate (Task 1, run once, `GOTOOLCHAIN=go1.26.11`)

| Step | Result | Wall time |
|------|--------|-----------|
| `go build ./...` | ok | 7 s |
| `go vet ./...` | ok | 3 s |
| `go test -race -short -count=1 ./...` | 12 packages ok, 0 FAIL (relay 126.2 s, daemon 28.7 s, webserver 18.1 s) | 136 s |
| `pnpm vitest run` | 90 files, 1276 tests passed | 28 s |
| `pnpm exec tsc --noEmit` | ok | 4 s |
| check-brand | `OK: brand gate clean` | 0 s |
| check-share-removal | `OK: share removal gate clean` | 2 s |
| check-traceability-paths | `OK: 149 traceability paths exist` | 0 s |
| verify-icons (CI step) | 27 passed, 0 failed | 0 s |
| build-script.test.sh | 40 passed, 0 failed | 1 s |

- No flake reruns were needed.
- `frontend/src/wailsjs` was clean afterwards.
- No gate or doc carries `-skip`: the grep over TESTING.md, the workflows, tests/, scripts/ and build.sh returned nothing.

## Final test-file counts (git ls-files)

- Go: 137 (+2 for Phase 8: internal/relay/vtalt_test.go and internal/relay/softwrap_test.go).
- Vitest: 90. Playwright: 7. build-script: 1.
- Total: 235.
- Section 4 now has 14 DEBT rows: DEBT-01 ×2, DEBT-02 ×2, DEBT-03 ×4, DEBT-04 ×3, DEBT-05 ×2 and DEBT-06 ×1 (internal/daemon/process_test.go, because 08-04 changed code).
- DEBT-07 is docs only and has no row.

## VM checks (build 7dcb66b2 = the code of 390f832f, ad-hoc signed, sha256 83323746…8ac3ad5, deployed to both VMs)

| Row | Result | Key numbers |
|-----|--------|-------------|
| V-01 | PASS | **C = 100:** baseline 5 MB, in-alt peak 6 MB (10 MB at the alt-exit GC), so +5 MiB at most; post-exit 6 MB; RSS 29,136 → 38,528 KiB. **C = 220:** baseline 8 MB, peak 8 MB (+0 MiB); post-exit 8 MB; RSS 43,680 → 44,048 KiB. Both are within the 32 MiB bound. Before 08-01 the same run was +123 MiB at C = 100 |
| V-02 | PASS | Rendered replay (`mode=full bytes=70902`). The 182-character echo command copies back as one line with the whole token. The 100-character `=` line and `AFTER-EQ` stay separate. seq 50023..60000 is consecutive |
| V-03 | PASS | After `reset` and a genuine resize (100x30 to 110x30, rendered replay of 804 bytes): post 1..60 consecutive with no duplicate. The cursor is on the live prompt row, and `echo CURSOR-OK-V03` landed after it |
| V-04 | PASS | Three returns gave `mode=delta`, all `resume=true`. The quick switch back after 1.05 s gave `mode=delta bytes=0 resume=true`. Ticks 1..181, 0 duplicates. After closing and reopening the tab: `mode=full resume=false` |
| V-05 | **FAIL** | **5 MB run:** the notice was `Paste may have been cut short (4883 KB)`, so M is right but there is no N. S = 98,301 bytes, an exact prefix that stayed the same for 85 s, and `echo V5-OK` ran once. **20 MB run:** `Paste may have been cut short (19532 KB)`; S = 262,136 bytes, an exact prefix that stayed the same. Sub-run: not run |
| V-06 | PASS | Three GUI relaunches kept daemon 1625 with 0 capture lines. Cold launch 45 s after a reboot: one daemon, capture only the AppKit line. macOS reopen at login: fd 2 offset 163, 0 `already running` |

## For the verifier: V-05

- **The FAIL is the main run's first rule.** The written PASS needs `Paste interrupted: N of M KB sent` with N < M, and S < (N+1) × 1024. Two runs (5 MB and 20 MB, the plan's retry limit) both showed the "nothing buffered, no ack" copy instead.
  - The other rules hold: the paste-interrupted banner kind appears, M = ceil(P/1024), the office file is an exact prefix that never grows after the drop, and the tab takes typing again.
  - The must-have truth ("shows the paste-interrupted notice while the office receives a prefix of the payload with nothing repeated") holds in substance. The detailed rule does not.
- **Likely cause (my reading, not measured):**
  - The GUI's socket goes over loopback to the home daemon's proxy, so the browser hands the whole paste off at once and `bufferedAmount` is 0 when the tailnet path dies.
  - The N-of-M branch needs bytes still queued in the browser. It may be unreachable on this path.
  - The unit tests set `bufferedAmount` directly, so they cannot show this.
- **A GUI hang that is not DEBT-05 itself:**
  - After Cmd+V, xterm's `xterm-helper-textarea` holds the pasted text as its AX value.
  - `sample` of the WebContent main thread puts 1964/1964 samples in `OpacityCaretAnimator::updateAnimationProperties` → `FrameSelection::recomputeCaretRect` → `WidthIterator::advance` → `CTFontShapeGlyphs`.
  - 20 MB held about 100% CPU for over 15 minutes (1.8 GB RSS). That delayed the watchdog and every UI update. 5 MB cost about 19 s.
  - 08-03 did not touch the paste input path, so this is probably older than Phase 8. I did not check.
- **Gap-plan options:**
  - Either accept the cut-short copy as the honest result on this path and narrow M-101 to it, or change how unacknowledged paste bytes are counted.
  - Separately, clear the helper textarea after a paste, or stop the browser inserting the paste into it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - TESTING.md mistake this plan made] M-98 order**
- **Found during:** Task 2.
- **Issue:** Run as M-98 first had it (`seq 1 60000` after the wrap cases), the seq pushes the wrap cases out of the hub's 10,000-line history.
- **Fix:** V-02 ran the seq first, which wraps the ring the same way, and M-98's step now says so.
- **Commit:** 440d18e0

**2. [Rule 3 - Blocking] gctrace has no "(forced)" marker**
- **Found during:** Task 2, V-01.
- **Issue:** gctrace in Go 1.26.11 does not mark periodic GCs "(forced)", so a wait on that marker never ended.
- **Fix:** the baseline is the newest idle GC line before each burst. V-01 records both baselines at 220 columns: 8 MB, and 6 MB from the last fully idle GC.

**3. [Rule 3 - Blocking] clicktab.sh hit a Notification Center banner**
- **Found during:** Task 3, V-04.
- **Issue:** `clicktab.sh`'s 60 px offset landed on a macOS Notification Center banner over the rightmost tab (found with an AX hit test), so the first V-04 pass got only one return.
- **Fix:** the recorded pass clicks 16 px into the label. Both passes are in the row.
- **Side effect:** an earlier click on the banner had opened System Settings in the home VM, which added it to loginwindow's relaunch list. I quit it and removed the entry.

**4. [Test procedure] V-05 held the tailnet down until the reconnect banner instead of 15 s**
- **Issue:** the client marks a socket dead only after 40 s of silence plus a 5 s probe (RECONNECT.deadMs + probeGraceMs). A 15 s outage could leave the socket alive and let the paste finish.
- **What happened:** in the 5 MB run the office ended the viewer at 22 s (`heartbeat timeout`), and the notice showed then. Tailnet down for 39 s in that run.

### Process notes

- Startup: fast-forwarded the worktree from c2f4255d to 390f832f with `git merge --ff-only gsd/phase-07-hardening`, as instructed.
- `pnpm install --frozen-lockfile --offline` in the worktree's frontend/ (no node_modules). It used the lockfile only and added no package.
- VM mishaps, all fixed at once and none affecting a check:
  - running the bare binary to read CLI usage started a second GUI in the home VM, which I killed;
  - `swiftc` and `strings` are Command Line Tools shims in the VM and each opened an install dialog, which I closed. The AX hit-test tool was built on the host instead and removed from the VM afterwards.
- The home relaunch-list backup was deleted before a last `cmp`. The list was restored by content (Portty, Finder), not byte-compared.
- Not done, per the caller's rules: ROADMAP, STATE and requirements.mark-complete. DEBT-05 is left out of `requirements-completed` until the verifier disposes of V-05.

## Known Stubs

None.

## Threat Flags

None.
- No code changed.
- T-08-18: every kill, reboot, `tailscale down/up` and click ran inside a VM.
- T-08-19: 08-VM-CHECKS.md and TESTING.md hold no `cap=` value, share URL or key, and the payloads appear only by sha256. The grep before each commit found only the word `cap=` in prose.
- T-08-22: both VMs list each other in `tailscale status` after cleanup.

## Self-Check: PASSED

- FOUND: TESTING.md, .planning/phases/08-debt-cleanup/08-VM-CHECKS.md (V-01..V-06 rows), this SUMMARY.
- FOUND commits: 7dcb66b2, 440d18e0 and 028eccf8 are ancestors of HEAD.
- `git status --porcelain -- frontend/src/wailsjs` is empty.
- `bash tests/check-traceability-paths.sh`: OK, 149 paths.
