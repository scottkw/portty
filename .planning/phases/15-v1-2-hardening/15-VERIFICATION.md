---
phase: 15-v1-2-hardening
verified: 2026-11-23T18:45:00Z
status: passed
score: 8/8 roadmap success criteria verified
covered_files:
  - ".planning/phases/15-v1-2-hardening/15-01-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-01-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-02-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-02-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-03-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-03-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-04-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-04-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-05-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-05-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-06-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-06-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-07-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-07-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-08-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-08-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-09-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-09-SUMMARY.md"
  - "TESTING.md"
  - "app_update.go"
  - "app_update_test.go"
  - "cmd_attach.go"
  - "cmd_attach_resize.go"
  - "cmd_attach_test.go"
  - "frontend/src/App.tsx"
  - "frontend/src/components/TabBar.tsx"
  - "frontend/src/components/TerminalPanel.tsx"
  - "frontend/src/components/WebShareSessionView.tsx"
  - "frontend/src/components/__tests__/App.open-remote.test.tsx"
  - "frontend/src/components/__tests__/App.update.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx"
  - "frontend/src/lib/__tests__/closeReasons.test.ts"
  - "frontend/src/lib/__tests__/pathLinks.linkifier.test.ts"
  - "frontend/src/lib/closeReasons.ts"
  - "frontend/src/lib/pathLinks.ts"
  - "internal/attach/attach.go"
  - "internal/attach/attach_test.go"
  - "internal/attach/attach_unix.go"
  - "internal/daemon/engine.go"
  - "internal/daemon/handoff.go"
  - "internal/daemon/handoff_it_test.go"
  - "internal/daemon/handoff_test.go"
  - "internal/daemon/ipc.go"
  - "internal/daemon/socket.go"
  - "internal/daemon/socket_test.go"
  - "internal/fdpass/fdpass.go"
  - "internal/fdpass/fdpass_test.go"
  - "internal/pty/adopt_test.go"
  - "internal/pty/cleanup.go"
  - "internal/pty/native_test.go"
  - "internal/pty/reap_other.go"
  - "internal/pty/session.go"
  - "internal/relay/cwd_test.go"
  - "internal/relay/history.go"
  - "internal/relay/hub.go"
  - "internal/relay/hub_grid_owner_test.go"
  - "internal/relay/manager.go"
  - "internal/relay/pathclick_test.go"
  - "internal/statusbar/bar.go"
  - "internal/statusbar/bar_test.go"
  - "internal/tailnet/sessions.go"
  - "internal/tailnet/tailnet_test.go"
  - "internal/webserver/capability_helpers_test.go"
  - "internal/webserver/input_queue_test.go"
  - "tray.go"
  - "tray_objc_darwin.m"
covered_digest: "v4:sha256:b47bf98c044617c4f83a4b559432738beb2bc50ebe92fd7bad4a2dc32cbe9485"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 8/8 in code (4 human items pending)
  gaps_closed:
    - "M-152 live notification click: PASS in VM (TESTING.md, 15-UAT.md)"
    - "M-153 attach bar with real terminal apps: PASS in VM"
    - "M-154 ended peer tab and sidebar over the two-VM tailnet: first run FAILED (not_sharing after the ended close), fixed in 7ca43451, rerun PASS"
    - "M-155 TestHandoffIT and the pre-phase upgrade chain: PASS in VM (first run hit a stale API assertion, fixed in 1c8d3a09)"
  gaps_remaining: []
  regressions: []
---

# Phase 15: v1.2 Hardening Verification Report

**Phase Goal:** Close the bugs and test flakes found during v1.2 so the milestone ships clean (ROADMAP Phase 15, success criteria 1-8; SC1 amended 2026-11-23 to the user-confirmed claim/typing scope).
**Verified:** 2026-11-23
**Status:** passed
**Re-verification:** Yes. The previous report was `human_needed` and went stale because covered files changed (7ca43451, 1c8d3a09, TESTING.md and 15-UAT.md).

All eight roadmap success criteria hold in the current tree. The four live checks that kept the previous report at `human_needed` ran in the Tart VMs and passed (TESTING.md Category BF "Last run" lines, 15-UAT.md 4/4 pass). I did not run VM checks myself; I treat the recorded results as the evidence, as instructed. The VM runs found two real defects, and both are fixed in code that I read and ran tests against.

## What changed since the last report

| Commit | Change | Verified |
|--------|--------|----------|
| 7ca43451 | `peerConnectVerdict` maps `not_sharing` to `ended` after the session-ended close (`closeReasons.ts:33`). `forbidden` still stops as `share-stopped`. | Read the code and diff. Vitest on closeReasons, App.open-remote and TerminalPanel.guestClose: 3 files, 82 of 82 pass. The tests cover `no_such_session`, `not_sharing` and `forbidden` after the ended close, and `unreachable` still retries. |
| 1c8d3a09 | `TestHandoffIT` asserts `APIVersion` instead of the stale literal 2. | Read the diff. The file is env-gated, and `go vet` (default and `-tags integration`) is clean. The VM run at 1c8d3a09 passed. |
| ce95e76f, TESTING.md | The M-152..M-155 "Last run" lines record PASS on 2026-11-23, with M-154 re-run after 7ca43451. | Read the lines at TESTING.md 1855 to 1880. The M-154 line records the first FAIL honestly, then the rerun PASS (2 of 2 runs, pf drop showed the can't-reach line, tab recovered). |

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `portty attach` with its bar shown sends rows-1 initially and on every resize; the reduced height wins once the CLI claims or types (amended scope); a test pins it | VERIFIED | Unchanged since the previous report: `attach.BarRows`, `initialResizeFrame`, `bar.ReservedRows`, `watchResize`, `TestHub_GridAttachBarRows`. Re-ran `internal/attach`, `internal/statusbar` and the relay grid-owner test under `-race`: pass. M-153 in the VM: PASS (bar row intact after every CLI type or resize, `tput lines` = rows-1). The passive-watch case keeps today's behaviour, per the user's decision. |
| 2 | Clicking an already-ended peer tab shows "session ended", not "stopped sharing" | VERIFIED | The code-level mapping is above. The M-154 failure was a real case the in-process tests had not covered: the ended session was the host's last share, so the host stopped its share server and the redial returned `not_sharing`. 7ca43451 fixes it, and the rerun on v1.2.0-vm16 shows the ended tab staying `Session ended` after switch-back in 2 of 2 runs. The sidebar half (`MetaRefused` reads as reachable with no sessions) passed in the VM, with 0 can't-reach readings in 2 of 2 runs, and `internal/tailnet` tests pass under `-race`. |
| 3 | Dismissing a naturally exited session never signals a stale pgid; the other todo items are resolved | VERIFIED | Unchanged. `internal/pty` passes under `-race`, including the kill and reaped tests. |
| 4 | Phase 9 socket-hardening items (O-1, O-2, O-6) done or recorded as accepted | VERIFIED | Unchanged in code. Re-ran `-run 'TestHandoff_\|TestListenDaemonSocket'` in `internal/daemon` and the `internal/fdpass` tests under `-race`: pass. M-155 in the VM now closes the real-process clause: the pid check accepted a genuine old daemon across real processes, the socket went 755, 600, 600, and the logs hold 0 `does not match the socket peer`. |
| 5 | Clicking "Portty X is available" brings Portty forward | VERIFIED | Unchanged in code. Root package `TestNotificationClick_*` passes. M-152 in the VM (a locally Developer-ID-signed build, not notarized): the click brought the window forward with Download, no download started, and a notification left from the previous process also worked after relaunch. Residual: the run used a signed build, not a notarized one. The UNUserNotificationCenter delegate path is the same, and the user accepted the run. |
| 6 | F-1 relative path on row 0 after `clear`; F-2 first Cmd-click after leave and return | VERIFIED | Unchanged. The previous report's mutation checks show both tests fail when the fix is removed. `TestPathClick*` and `TestCwd_*` re-run under `-race`: pass. |
| 7 | Flaky tests deterministic; full Go and vitest suites pass once, clean | VERIFIED | The orchestrator supplied full vitest on current HEAD (122 of 122 files, 1826 tests, clean) and full Go `-race` 14 of 14 on the code before 1c8d3a09. Only the env-gated `handoff_it_test.go` changed since, and `go vet` on it is clean. I re-ran the touched Go packages (attach, statusbar, tailnet, fdpass, pty, daemon, relay) with `-race -short -count=1`: all `ok`. |
| 8 | Fixed todos moved to completed; TESTING.md updated per the standing rule | VERIFIED | The nine fixed todos are in `.planning/todos/completed/`. Pending holds `dompurify-3418` and `v12-release-checks` (out of scope) plus three new todos from VM findings (`dos-bound-test-timeouts`, `kill-by-name-fails`, `version-flag-hangs`; see Observations). TESTING.md Category BF has M-152..M-155 with "Last run" PASS lines. `bash tests/check-traceability-paths.sh` gives OK: 228. |

**Score:** 8/8 truths verified; 0 behavior-unverified; 0 failed.

### Backstop truths (previously abstained, now evidenced)

| Truth | Status | Evidence |
|-------|--------|----------|
| Live notification click on a build in the VM (15-06) | VERIFIED | M-152 PASS, TESTING.md and 15-UAT.md #1 |
| Pre-phase socket reads 0600 after two real-process upgrades (15-03) | VERIFIED | M-155 chain PASS (755, 600, 600) |
| Pid check accepts a genuine old daemon across real processes, including a listener created by another process (15-08) | VERIFIED | M-155 PASS (ship blocker cleared): the upgrade subtest passed, sessions kept, shells ppid 1, A exit 0, 0 mismatch lines |

### Required Artifacts, Key Links, Data Flow

Carried from the previous report with no regression: every artifact exists, is substantive and is wired. The three tool-reported key-link failures remain pattern-syntax artefacts, not broken links (each call site was confirmed by reading the code). Data flows from real sources (terminal size and bar rows, the real session-ended close, the real transport error classification). The only source files touched since are `closeReasons.ts` (a pure function, read) and one env-gated test.

### Behavioral Spot-Checks (single named runs, this session)

| Behavior | Command | Result |
|----------|---------|--------|
| Ended-peer verdicts (SC2) | `npx vitest run closeReasons.test.ts App.open-remote.test.tsx TerminalPanel.guestClose.test.tsx` | 82 of 82 PASS |
| attach, statusbar, tailnet, fdpass, pty (SC1-4) | `go test -race -short -count=1` | all ok |
| Handoff, socket mode, grid bar rows, path click, cwd (SC1, SC4, SC6) | `go test -race -short -run ...` in daemon and relay | ok |
| Notification click, kill session (SC3, SC5) | `go test -short -run 'TestNotificationClick\|TestKillSession'` | ok |
| Traceability | `bash tests/check-traceability-paths.sh` | OK: 228 paths |
| Compile of the env-gated IT test | `go vet` default and `-tags integration` on `internal/daemon` | clean |

### Probe Execution

SKIPPED (no declared probe scripts).

### Requirements Coverage

No requirement IDs are declared: all nine plans have `requirements: []`, and REQUIREMENTS.md maps nothing to Phase 15. No requirement is orphaned.

### Anti-Patterns Found

No TBD, FIXME, XXX, TODO or HACK markers were added in the changes since aba14c53 (checked by grep over the diff). No stubs.

### Observations (non-blocking, not gaps)

- Two CLI bugs found during M-153 (`kill-by-name-fails`, `version-flag-hangs`) and a DoS-bound vitest-timeout todo are filed as pending todos. They are new findings from the VM runs, outside the nine todos this phase targeted, and are correctly kept pending. They are not Phase 15 success criteria.
- The untracked 27 MB `agenthub` binary at the repo root is not phase output. Delete or ignore it before shipping (carried from the previous report).
- Open review infos IN-02, IN-03 and IN-06 are cosmetic or narrow. IN-05 is covered by the M-152 relaunch step, which passed. The incremental review (937c8115) reports 0 warnings.

### Human Verification Required

None. All four items were executed in the VMs and passed (15-UAT.md: total 4, passed 4, issues 0).

### Gaps Summary

No gaps. All eight success criteria are delivered in code, covered by tests I re-ran, and the four live checks are recorded as PASS. The M-154 failure was a real miss in the earlier in-process coverage, and it is fixed and re-verified live.

---

_Verified: 2026-11-23_
_Verifier: Claude (gsd-verifier)_
