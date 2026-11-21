---
phase: 10-input-path-reliability
plan: 07
subsystem: phase gate, regression home, VM evidence
status: complete
tags: [gate, testing-md, vm-checks, pf, paste, grid-owner, v-02]
requires: [10-01, 10-02, 10-03, 10-04, 10-05, 10-06]
provides:
  - "10-VM-CHECKS.md: R-00, R-01a-c, R-05 (FAIL, then PASS after 9cda1853), R-02, V-02r, R-06a-f, R-03, R-04, all PASS in the second pass"
  - "TESTING.md: Phase 10 Section 2 notes and counts (Go 152, total 255), 29 INPT Section 4 rows, Category AZ (M-116..M-120), M-101 restated, M-106 last run"
affects: [ROADMAP Phase 10 SC1-SC6 evidence]
tech-stack:
  added: []
  patterns: ["silent pf block of the share port (anchor com.apple/portty-drop, enable token, nohup'd safety net) as the network drop on ephemeral tailnet nodes"]
key-files:
  created:
    - .planning/phases/10-input-path-reliability/10-VM-CHECKS.md
  modified:
    - TESTING.md
    - internal/daemon/remote_ws_proxy_test.go
    - internal/daemon/attach_viewers_test.go
    - internal/daemon/handoff_test.go
    - internal/daemon/handoff_fault_test.go
    - internal/attach/loop_test.go
decisions:
  - "SC3's drop is a pf block of TCP 7444 on utun4 instead of `tailscale down` (ephemeral nodes); the fallback was never needed"
  - "Safari pastes are DOM paste events (DataTransfer): Safari under WebDriver ignores real key events"
  - "Browser zoom was driven with Safari's View > Zoom In after ending the WebDriver session (Stop Session); WebDriver key actions cannot zoom"
  - "R-05's FAIL row is kept, with the post-fix re-run row below it (as Phase 9 kept V-05b's)"
requirements: [INPT-01, INPT-02, INPT-03, INPT-04, INPT-05, INPT-06]
metrics:
  duration: "~3 h (03:06 to 06:10 UTC), including three SC5 runs of ~31 min each and two coordinator round trips"
  completed: 2026-11-21
estimate:
  tokens: 150000
  tasks: 3
actuals:
  tokens: 31800   # chars/4 over d36ac2e8..HEAD (127,273 chars; includes the coordinator's fix commits 9cda1853 and 35aaa035)
  tasks: 3
  commits: 10     # measured: git rev-list --count d36ac2e8..HEAD at SUMMARY write (8 by this executor, 2 by the coordinator); the SUMMARY commit follows
plan_head_before: d36ac2e8303b722fd9323dc9b30b8e243a97a0ad
plan_head_after: 67ca3b2c
---

# Phase 10 Plan 07: Phase gate, TESTING.md and the two-VM checks Summary

The phase is green and has real-machine evidence for every success criterion. SC5's `go test -race -count=20 ./internal/daemon/` passed after three test-only timing fixes. The two Tart VMs then showed the following, all with byte-exact receiver files:

- a 4 MB slow paste keeps a peer tab, a browser viewer and a local tab connected;
- a frozen viewer is reaped while its backlog drains in full;
- a completed paste stays silent on a later close;
- a pf-dropped paste shows the notice in the app and in the browser's `role="status"`;
- the viewer you type on, zoom or resize sets the grid, and idle viewers never do;
- Phase 9's V-02 still holds without its settle window.

R-05 first failed on a full replay after the reconnect. The coordinator fixed that in the product (9cda1853), and the re-run passed.

## Gate (Task 1)

| Step | Result | Log (scratchpad `p10/`) |
|------|--------|-------------------------|
| build, vet | OK | p10-gate-build.log, p10-gate-vet.log |
| `go test -race -short -count=1 -v ./...` at d36ac2e8 | rc 0, 13 packages ok, 0 DATA RACE | p10-gate-go.log |
| SC5 `-race -count=20 ./internal/daemon/`, run 1 (d36ac2e8) | **FAIL**: 3 proxy tests in one 35 s host stall (loopback dials of 1.3-1.9 s) | p10-gate-daemon20.log |
| SC5 run 2 (9d7c2922) | **FAIL**: TestCLIAttach_IdleNoReconnecting (timing) and TestHandoffFault_EveryStep/old:after-hello (`descriptor count 7 after, 9 before`) | p10-gate-daemon20-rerun.log |
| SC5 run 3 (3985dda6) | **PASS**: `ok … 1853.588s`, 0 FAIL, 0 DATA RACE, 0 `device not configured` | p10-gate-daemon20-rerun2.log |
| fresh `go test -race -short -count=1 -v ./...` (3985dda6) | 12 packages ok; `internal/attach` TestRun_AbruptCloseRetries failed once (an ordering race in the test); fixed test-only (bc86de32), then `-race -count=50` ok | p10-gate-go-final.log, p10-abrupt-count50.log |
| vitest | 95 files / 1332 tests passed | p10-gate-vitest.log |
| tsc | rc 0 | p10-gate-tsc.log |
| build-script test | 40 passed, 0 failed | p10-gate-buildscript.log |
| check-brand, check-share-removal, check-traceability | OK (174 paths) | p10-gate-sharerm.log |
| Guards | csp_mw.go, capability_mw.go and tailnet/live.go unchanged since d027e86d; MaxInputPayload still 32767; `frontend/src/wailsjs` clean | |

The coordinator ran a full `-short` gate at 9cda1853, the R-05 fix commit (10-07-FIX.md).

**Skips** in the `-short` runs (gated by `-short` or an environment variable):
- TestQuitAllSubprocess (not a subprocess invocation)
- TestHandoffIT (needs PORTTY_HANDOFF_IT)
- TestRemoteSessionWS_LongLived (-short)
- TestExitDetection_ExitCode_RunningProcess and TestExitDetection_KillCachesExitCode (-short)
- TestHub_SlowClientDisconnected
- TestRealDMG (needs UPDATE_REAL_DMG)
- TestReleaseJSON (needs UPDATE_RELEASE_JSON)

**TESTING.md counts:** Go 152, vitest 95, Playwright 7, build-script 1, total 255.

## VM rows (10-VM-CHECKS.md)

| Row | Result | Key evidence |
|-----|--------|--------------|
| R-00 | PASS | share port 7444 on `utun4`. With the block: a new curl got `000` after 5 s, and a held s_client request got no reply for 10 s. After the lift: the reply arrived 3.1 s later. pf back to Disabled with the anchor empty. Both nodes listed throughout. **drop mechanism: pf** |
| R-01a | PASS | home peer tab: 4,000,000 B drained in **116.3 s**, sha256 equal, no reap or banner |
| R-01b | PASS | Safari (DOM paste event): **117.5 s**, sha256 equal |
| R-01c | PASS | office local tab: **101.8 s**, sha256 equal |
| R-05 (first pass) | FAIL | reap and drain held, but the rejoin was `mode=full bytes=376` (the paste's typing claim set geomEnd and no output followed) |
| R-05 (re-run, 35aaa035) | PASS | reap about **43 s** after the STOP, backlog 4,066,747 B; file 1.94 MB at the reap; `input backlog drained`; all 6,000,000 B, `cmp` equal; rejoin `mode=delta bytes=0`, no notice |
| R-02 | PASS | 5 MB + 5 MB into an echoing `cat`; both halves sha256-equal; only the share-stopped banner in the tab and in Safari |
| V-02r | PASS | 2 runs: 5/5 `mode=delta` each, 0 grid lines, no repeated or missing tick. Rejoin order: run 1 office attach first, run 2 office GUI tab first |
| R-06a | PASS | zoom ×2 and a window resize each gave `why=claim` (80x27, 76x26, 60x24); office typing gave `why=input` 85x29; home typing `why=input` 60x24 |
| R-06b | PASS | WebDriver window rect gave `why=claim` 80x22; View > Zoom In gave `why=claim` (69x18, 63x17); office typing took the grid back; Safari typing `why=input` |
| R-06c | PASS | home attach and office-local attach: connecting moved nothing; SIGWINCH gave `why=claim` (70x24, 72x20); typing took it back and forth |
| R-06d | PASS | 5 viewers plus vim's queries (DSR, DA2, DECRQM, OSC 10/11): 0 log lines in 62 s |
| R-06e | PASS | relaunch: no grid line; first typing gave exactly one `why=input` line |
| R-06f | PASS | 10 alternating typings gave exactly 10 grid lines |
| R-03 | PASS | **mechanism: pf**, drop window 22 s (block 06:01:02.032, lift 06:01:24.800). `Paste may have been cut short (4883 KB)`; file 65,534 B = exact prefix = `input total`; delta rejoin; takes typing |
| R-04 | PASS | **mechanism: pf**, drop window 42 s (block 06:04:03.656, lift 06:04:45.199). Notice in `[role=status]` after 39.1 s; 0 CSP violations, 0 errors; file 98,301 B = exact prefix = `input total`; delta rejoin |

**pf state.** Before every drop, `Status: Disabled` with the `com.apple/portty-drop` anchor empty. During, Enabled with 2 rules. After each lift and at the end, Disabled with the anchor empty. Every safety net fired harmlessly.

### Builds

All are ad-hoc signed (`Signature=adhoc`, `TeamIdentifier=not set`), and the sha256 matched on the host and in the VMs.

| Pass | A (v1.2.0-vmA) | B (v1.2.0-vmB) | HEAD (dev) |
|------|----------------|----------------|------------|
| First pass (bc86de32) | `16d33279…` | `8b0106c3…` | `0adf9e8b…` |
| Second pass (35aaa035) | `6a936f58…` | `20b94052…` | `8d864c0f98bb6f2bdc6c6173647b0ba3af584d0925e2db7fa234a60c70114cf7`, deployed on both at the end |

## Commits

| Commit | What |
|--------|------|
| 744480fc | docs: TESTING.md Sections 2 and 4 (29 INPT rows; VIEW-02/04/05, SHAR-06, UPGR-02 notes corrected) |
| 9d7c2922 | test: proxy test timing (PasteBackpressure 200/500 ms with a ~2 s drain and a 3x hold; DiagLog and ForwardsResume 20 s contexts) |
| bbc315dd | test: IdleNoReconnecting timing (200 ms ping / 1 s watchdog / 3 s idle) |
| 3985dda6 | test: the handoff fd check flags new fds only, with a GC-settled baseline (a mutant opening an fd after the baseline is caught) |
| bc86de32 | test: AbruptCloseRetries waits for the connected state |
| 12e411d0 | docs: VM checks first pass (R-00, R-01a-c PASS, R-05 FAIL) |
| 9cda1853, 35aaa035 | (coordinator) fix: delta resume from exactly geomEnd, plus 10-07-FIX.md |
| fb7f5505 | docs: VM checks second pass, all PASS |
| 67ca3b2c | docs: TESTING.md Section 5 (Category AZ, M-101, M-106), gate-fix note |

## Deviations from Plan

### Auto-fixed issues (test-only, Rule 3; coordinator-approved)
1. **9d7c2922**: SC5 run 1 failed in TestRemoteWSProxy_DiagLog, ForwardsResume/numeric and PasteBackpressureStaysOpen. All three failed during one host stall, with 100 ms heartbeat timeouts and 5 s budgets that covered several dials. The fix widens them, following 10-03's share-test precedent.
2. **bbc315dd**: TestCLIAttach_IdleNoReconnecting failed once in SC5 run 2. Its watchdog was 200 ms under -race; widened.
3. **3985dda6**: TestHandoffFault_EveryStep/old:after-hello saw the process-wide fd count drop from 9 to 7: an unrelated fd closed mid-subtest. The check now fails only on new fds, compared by (fd, dev, ino, rdev), after a GC barrier, and the message names the extra fds.
4. **bc86de32**: TestRun_AbruptCloseRetries asserted the state list right after the output appeared. The reader writes the output before the loop reports "connected", so the test now waits for the state (bounded).

No product code was changed by this executor. The R-05 product fix (9cda1853, `joinLocked` `resume >= geomEnd`) was the coordinator's decision (b) after the R-05 FAIL checkpoint.

### Rig and plan deviations
- **SC3 deviation (for the user to accept).** SC3 says "with `tailscale down` on the office VM during a paste". The drop was a pf `block drop` of TCP 7444, the office share listener, on `utun4`, in the anchor `com.apple/portty-drop`, with a nohup'd safety net. It gives the share connections the same silent stall as a dead network, while `tailscaled` stays up and the ephemeral node stays online. The `tailscale down` fallback was never used. The exact paragraph is in the 10-VM-CHECKS.md header.
- **Safari pastes (R-01b, R-02, R-04) were DOM paste events,** a `DataTransfer` on xterm's textarea. Safari under WebDriver ignored both a System Events Cmd+V and a WebDriver Meta+v. From the clipboard event on, the app's own paste path runs.
- **R-06b browser zoom used Safari's View > Zoom In after Stop Session** (it ended the WebDriver session; the window kept the page). A WebDriver Meta+= action and a System Events Cmd+= keystroke did not zoom.
- **R-01a ran twice.** The first perl `read()` receiver held back the final 256 bytes. The receiver became a `sysread` reader. The office VM has no `pv`.
- **R-00 was not repeated in the second pass.** It is the same office VM and the same mechanism, and R-03 and R-04 recorded pf before and after each drop.
- **The FAIL row is kept.** R-05's first-pass FAIL row stays in 10-VM-CHECKS.md, as the coordinator asked, so the plan's Task 2 verify grep "no FAIL row" counts 1 by design. The PASS re-run row follows it.
- **R-05 reap arithmetic.** File + backlog equals the bytes read plus the part of the in-flight frame already written, because the backlog counts that frame whole. Both passes show the exact frame arithmetic: 66 and 59 whole frames.
- **Logs in the scratchpad.** Gate logs went to the session scratchpad, not /tmp.
- **pnpm install.** `pnpm install --frozen-lockfile --offline` ran in the worktree: lockfile only, nothing downloaded.

### Cleanup findings
- **safclean.sh misses one cache.** After R-04, `safclean.sh` found the r4 token in Safari's tab snapshot cache (`…/Caches/com.apple.Safari/TabSnapshots/Metadata.db`), which the script does not clean. The cache was emptied, and a re-search found 0 copies of the remaining tokens.
- **Per-site zoom entry.** R-06b's zoom left a per-site zoom entry for the office host in `PerSiteZoomPreferences.plist`. It was deleted.

## End state
- Both VMs run HEAD `dev` `8d864c0f…`:
  - office: GUI 91084, daemon 91096
  - home: GUI 24540, daemon 24542
  - both: 0 sessions, `/health` dev api 2 handoff 1
- settings.json is restored (`cmp` clean, `"webgl":true`).
- pf on the office is Disabled with the anchor empty.
- No link files or staging folders are left.
- Safari, safaridriver and expect are not running.
- The helpers and payloads stay in `~/p10`; they hold no secrets.
- The VMs were never stopped or logged out.
- Host: `git status` clean, `frontend/src/wailsjs` unchanged, build outputs removed. The host's own Portty was not touched.

## Known Stubs
None.

## Threat Flags
None. No product surface was added by this plan's commits.

## Self-Check: PASSED
- FOUND: .planning/phases/10-input-path-reliability/10-VM-CHECKS.md and TESTING.md (Category AZ, M-116..M-120)
- FOUND commits: 744480fc, 9d7c2922, bbc315dd, 3985dda6, bc86de32, 12e411d0, fb7f5505, 67ca3b2c (ancestors of HEAD)
- Task 2 and Task 3 verify greps: every row PASS; R-00, R-03 and R-04 record the mechanism; `mode=delta`, `input backlog`, `why=claim`, `why=input`, `COLS=`, two notice readings, Cleanup present; 0 link, token or cap patterns; M-116..M-120 present; Category AZ present; 10 `10-VM-CHECKS.md` references in TESTING.md; the traceability gate passes. The one exception is the kept FAIL row (see Deviations).
