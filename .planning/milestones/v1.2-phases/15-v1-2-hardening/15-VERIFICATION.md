---
phase: 15-v1-2-hardening
verified: 2026-11-23T20:15:00Z
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
  - "cmd_share.go"
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
covered_digest: "v4:sha256:69bc6879785a1852f90ddba99e1511eb3455cdcc345f2fa67b5f8a0db67dcfba"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 8/8 roadmap success criteria verified
  gaps_closed: []
  gaps_remaining: []
  regressions: []
  note: "after Phase 16 (moved cleanup.go, hub.go, cwd_test.go, input_queue_test.go, TESTING.md); IN-02, IN-03, IN-06 resolved by Phase 16"
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

## Re-verification after Phase 16

Phase 16 "v1.2 Cleanup" (`git diff cc595307..HEAD`, HEAD 3c09f851) moved five files this report covers: `TESTING.md`, `internal/pty/cleanup.go`, `internal/relay/cwd_test.go`, `internal/relay/hub.go` and `internal/webserver/input_queue_test.go`. `cmd_share.go` is now also in the fingerprint because `portty attach` resolves its target through `resolveSession`. Phase 16 resolved three of this phase's open review items (15-REVIEW-DISPOSITION.md): **IN-02** (duplicate `killSession` branches, fixed in 97611082), **IN-03** (ED 3 `History.Skip` counted main-screen scrollback in the alternate screen, fixed in 0af0cd46) and **IN-06** (`joinPasteViewer` copied `dialResume`, fixed in 97611082). IN-05 was covered by M-152 (passed).

| Moved file | Phase 16 change | Effect on Phase 15 truths |
|------------|-----------------|---------------------------|
| `internal/pty/cleanup.go` (SC3, IN-02) | The two early-return branches (`cmd == nil || cmd.Process == nil` and `reaped.Load()`) are one `||` condition with the same body (`closePTY`, `cancel`, return nil); a doc sentence records the accepted IN-01 residual | Behaviour-identical, read line by line: `exited != nil` still goes to `killAdopted` first, a reaped session is still never signalled or re-waited, a running one still gets SIGHUP to the group. `TestKillSession_ReapedSkipsGroupSignal`, `_RunningSignalsGroup`, `_NoGoroutineLeak_NormalCase`, `TestKillClean`, `_NoOrphans`, `_NotFound` and the whole `./internal/pty` package pass under `-race`. |
| `internal/relay/hub.go` (SC6, IN-03) | ED 3 calls `History.Skip(emu.ScrollbackLen())` only when `!emu.IsAltScreen()` | The F-1 fix (rows stay monotonic after `clear`) is kept for the main screen; the guard only stops over-counting while the alternate screen is active (vt's ED 3 clears the active screen's scrollback, `ScrollbackLen` is always the main screen's). `TestCwd_ClearKeepsRowsMonotonic`, `TestCwd_ScrollbackEraseKeepsRows` and `TestPathClick_RelativeAfterClear` still pass. Mutation check: with the guard replaced by `if true`, the new `TestCwd_AltScreenScrollbackEraseKeepsRows` fails (`MARK-A resolves to "/b", want "/a"`); the file was restored and `git diff` is back to the committed state. |
| `internal/relay/cwd_test.go` | Adds `TestCwd_AltScreenScrollbackEraseKeepsRows` | The pin for IN-03 above. Passes with the guard, fails without it. |
| `internal/webserver/input_queue_test.go` (SC7, IN-06) | `joinPasteViewer` calls the shared `dialResumePing` helper in `resume_test.go` instead of copying the dial code; same `OnPingReceived` hook, same Origin header, same read limit | `TestShare_PasteBacklogKeepsViewer` (the 15-02 deterministic ping-counting test) still passes; the whole `./internal/webserver` package passes. Test-only. |
| `TESTING.md` | Phase 16 note and Category BG (M-156) added; Phase 15's note and Category BF (M-152..M-155, with their PASS lines) are intact | Traceability script: OK 228. |
| `cmd_share.go` (added to the fingerprint) | `resolveSession` rejects `""`; `lookupSessionStrict` is used by kill and rename only | `portty attach` still resolves by exact name, exact ID and unique prefix; only an empty target now returns `noSessionError`, which attach already handles. `TestAttach*`, `TestInitialResizeFrame` and the attach package pass. The SC1 attach-bar truth is not affected. |

The two CLI bugs recorded as non-blocking observations in this report (`kill-by-name-fails`, `version-flag-hangs`) and the DoS-bound timeout todo are fixed in Phase 16 and sit in `.planning/todos/completed/`. The pending todos are now `dompurify-3418` and `v12-release-checks` only. The untracked `agenthub` binary in the observations is still untracked and still not phase output.

Verdict: all eight roadmap success criteria hold. The VM results for M-152..M-155 carry forward: Phase 16 changed none of the code those four checks exercised (notification delegate, attach bar, ended peer tab and sidebar, daemon handoff and socket mode).

Checks run at HEAD (`GOTOOLCHAIN=go1.26.11`, `-race -short -count=1`, named runs only, no GUI, no tailnet, no host tailscaled):

| Check | Result |
|-------|--------|
| `go build ./...`; `go vet` on root, daemon, relay, webserver, pty; `gofmt -l` | clean |
| `./internal/pty` (whole package) and the named kill tests with `-v` | ok, 6 of 6 named PASS |
| `./internal/relay -run 'PortPrinted\|HasURLPort\|PathClick\|Cwd_\|Hub_Grid'` and the four ED 3 / clear tests with `-v` | ok |
| IN-03 mutation check (guard removed, test must fail) | FAILED as required; file restored |
| `./internal/webserver` (whole package, includes `PasteBacklog`) | ok |
| `./internal/daemon -run 'Handoff_\|ListenDaemonSocket\|...'`, `./internal/attach`, `./internal/statusbar`, `./internal/tailnet`, `./internal/fdpass`, `./internal/capability` | all ok |
| root `-run 'TestInitialResizeFrame\|TestNotificationClick\|TestResolveSession\|TestCmdKill\|TestCmdRename\|TestLaunchMode'` | all PASS |
| `bash tests/check-traceability-paths.sh` | OK: 228 |
| Phase 16's own full `go test -race -short ./...` (14/14) and full vitest (1826/1826) at f1e7280d, reported by the orchestrator | not re-run here; root package re-tested clean after the later `cmd_cli.go`/`cmd_share.go` edits |

No failure, no new gap. Status stays `passed`.

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
_Re-verified: 2026-11-23 (after Phase 16, at HEAD 3c09f851)_
_Verifier: Claude (gsd-verifier)_
