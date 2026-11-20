---
phase: 09-upgrades-keep-sessions
plan: 07
subsystem: vm-evidence
status: complete
tags: [upgrade, handoff, vm, tart, evidence, quit]
requirements-completed: [UPGR-01, UPGR-02, UPGR-03, UPGR-04, UPGR-05]
dependency_graph:
  requires: ["09-01..09-06 (handoff, takeover, reconnect, copy strings, fault hooks, I-rows)"]
  provides:
    - "09-VM-CHECKS.md rows V-00..V-05a PASS, V-05b FAIL then PASS on the re-run after the fix (09-07-FIX.md), V-05c and V-05d PASS, plus SOH note and Cleanup"
    - "TESTING.md M-105..M-111 and M-115 last-run lines; M-112/M-113 marked not run"
  affects: [TESTING.md, .planning/phases/09-upgrades-keep-sessions/09-VM-CHECKS.md]
tech-stack:
  added: []
  patterns:
    - "Fresh check viewers use the running daemon's own build, so a check never triggers an upgrade itself"
    - "Strip the ring's SOH frame headers (tr -d '\\001') before grepping raw attach output"
key-files:
  created:
    - .planning/phases/09-upgrades-keep-sessions/09-07-SUMMARY.md
  modified:
    - .planning/phases/09-upgrades-keep-sessions/09-VM-CHECKS.md
    - TESTING.md
decisions:
  - "OLD is a plain go build of git archive v1.1 (CLI/daemon only), as 09-06 I-03: every OLD row needs only a v1.1 daemon; the GUI under test is always B"
  - "B, B-fault, A-fault and HEAD reused A's frontend/dist via wails build -s (same commit)"
  - "V-05b is FAIL: the plan's PASS bar (exit within 5 s of Quit All) was missed by 54.76 s; the plan stops there (W6), so V-05c and V-05d were not run"
metrics:
  duration: ~45m
  completed: 2026-11-20
estimate:
  tokens: 130000
actuals:
  tokens: 9900
  tasks: 3
  commits: 4
plan_head_before: 8d5bcbb54ebede80e1313ecc0f054bb6edb38b7b
plan_head_after: adb946353c13bac759d9edde0b60f3ad561c4e3e
---

# Phase 9 Plan 07: Two-build Tart VM rows Summary

Real app builds in the two Tart VMs passed SC1 to SC4:
- an upgrade under live sessions asks nothing and keeps every shell, tick, history line and share link
- every viewer kind comes back in 1 to 1.5 s with a delta
- killed takeovers and a killed old daemon cost no session
- a v1.1 daemon gets the plain DEBT-08 choice
- the API version and Update needed work

**SC5 is met after a fix (09-07-FIX.md, commits 4f9fdb82 and 72214c58).** The re-run of V-05b PASSED: the Quit dialog showed 2.3 to 2.4 s after Cmd-Q with an unknown count, and the GUI exited 3.07 to 3.09 s after Quit Everything. V-05c (one dialog under Cmd-Q) and V-05d (a macOS restart with the GUI open; the boot time was 12 s after the trigger) PASSED. That makes UPGR-05 complete.

The first pass did not meet SC5: SIGTERM passed, but **V-05b FAILED**. With the v1.1 daemon SIGSTOPped under an approved restart, the Quit dialog took 30.2 s to appear, and the GUI exited 54.76 s after Quit Everything. The bar was 5 s. The plan stopped at that row.

## CHECKPOINT: V-05b FAIL (plan check W6)

- **Row:** V-05b (M-111, SC5, 08-06 IN-07).
- **What happened:**
  - The v1.1 daemon was SIGSTOPped while its DEBT-08 dialog was open in GUI B.
  - Restart Now was clicked. A confirmation run shows the button then reads `Restarting…`.
  - Cmd-Q showed the Quit dialog after 30.2 s.
  - Quit Everything was clicked, and the GUI exited 54.76 s later.
  - After `kill -CONT`, the old daemon processed the queued shutdown and exited. No fresh daemon was left.
- **Likely cause (code reading only, no product code changed):** each step waits on an HTTP call to the stopped daemon, which ends only at the client's 30 s timeout.
  - The Quit dialog waits for `ListSessions()` (App.tsx `app:quit-requested`).
  - `QuitAll` (app.go) cancels the ensure ctx. It then waits for `daemonMu`, which `AnswerDaemonRestart`'s ensure holds until its in-flight call times out. After that it calls `ShutdownDaemon` on the same stopped daemon.
  - 09-01-SUMMARY's notes already flagged this: the client calls are not ctx-aware.
- **Not run:** V-05c (M-112, one dialog under Cmd-Q) and V-05d (M-113, a macOS restart with the GUI open). The 09-01 logout/restart hook therefore still has no real-AppKit proof.
- **Decision needed:** fix the quit path against a hung daemon (bounded or ctx-aware ListSessions/ShutdownDaemon in the quit flow), or restate the V-05b bar. Then re-run V-05b, V-05c and V-05d. The helper scripts are still in `~/p907` in both VMs.

## Row results

| Row | Result | Key evidence |
|-----|--------|--------------|
| V-00 | PASS | B with no arguments, directly and through `open`: `/health` vmB api 2 handoff 1, stderr 0 bytes, window with No Open Sessions |
| V-01 | PASS | GUI A SIGTERM gone in 0.07 s. Install and launch B: 0 dialogs at +5 s and +20 s; `/health` vmB api 2 handoff 1; shells 32437/32502 same pids, now ppid 1; viewer TICK-s1 0..103 and vim file 1..97 with no gap; fresh replay has HIST-s1-1..300 and `ESC[?1049h`; share link 302/200 before and after, changed token 401/401. **Cert warm 17 ms; freeze 9 ms** (accepted 05:07:19.218 → committed 05:07:19.227) |
| V-02 | PASS | Trigger: B's CLI `list`. All 5 s1 viewers rejoined with `mode=delta resume=true` 1.03 to 1.53 s after COMMITTED: office GUI tab, office attach, home peer tab, home Safari share, home attach. Each viewer's tick sequence across the upgrade had 0 duplicates and no jump. Cert warm 19 ms. Safari cleanup done |
| V-03a | PASS | `new:before-ready:pause30`; `kill -9` 46 ms after the pause line; `rolled back at ready after 182ms: EOF`. Dialog body starts `Portty couldn't move your sessions to the new version.`, with Not Now and Restart Now. Not Now: A serving for 30 s, shells still A's children, home peer tab TICK-s1 0..123 never left. First attempt discarded: its check viewer was B-fault and upgraded A by itself |
| V-03b | PASS | `new:after-serve-before-committed:pause3`. Kill 49 ms after the pause, and in a second run 1.55 s after it: `no COMMITTED … waiting for it to exit before resuming`, then `exited after COMMIT; serving 2 sessions again` only after the kill. CLI rc 1 with the couldn't-move line and DEBT-08 refusal. **Post-COMMIT gap: 0 ticks for a 49 ms window; 2 ticks (18, 19) for a 1.55 s window.** One attempt discarded (the watcher lacked +x, so the pause ran out and the takeover committed) |
| V-03c | PASS | v1.1 daemon: plain dialog with `2 sessions are running. Restarting ends them.` and 0 `couldn` in the AX dumps. Not Now: GUI B tab ticking on v1.1. B `list` rc 1 with the DEBT-08 text; `list --keep-daemon` rc 0 |
| V-03d | PASS | `old:after-state-sent:pause30`; old daemon `kill -9` 40 ms after its pause line; the takeover logged `old daemon pid … is gone (EOF); proceeding as committed with 2 sessions`. CLI rc 0 with empty stderr; `/health` B; same shell pids; ticks with no gap; history (300) and alt screen in fresh viewers |
| V-04 | PASS | B `/health` api 2; OLD has no `api`. GUI B-fault with `PORTTY_VMPREVIEW_MIN_DAEMON_API=3` shows the `Update needed` banner (no button, empty stderr). CLI rc 1 with the one-line update-needed message |
| V-05a | PASS | `kill -TERM` GUI B: gone in 0.11 s; daemon 54443 and shells unchanged; ticks advanced |
| V-05b | **FAIL** | See the checkpoint above: Quit dialog after 30.2 s; GUI exit 54.76 s after Quit Everything (bar: 5 s) |
| V-05b (re-run after fix) | PASS | Fixed B `c9115f7a…`: Quit dialog 2.4 s and 2.3 s after Cmd-Q, with `Portty couldn't check whether any sessions are running.`; GUI gone 3.09 s and 3.07 s after Quit Everything; old daemon stopped and never killed; after `kill -CONT` it processed the queued /shutdown; no fresh daemon left |
| V-05c | PASS | Cmd-Q under the open restart dialog: exactly 1 dialog, `Quit Portty?` with `You have 2 active sessions running`; v1.1 daemon still serving, shells its children, ticks advancing; no restart question came back |
| V-05d | PASS | GUI B open on s1/s2; `osascript … restart`: SSH dropped, boot time `1795154365`, 12 s after the trigger (`1795154353`); auto-login back; no cancellation |

## Builds (all `Signature=adhoc`, `TeamIdentifier=not set`; sha256 equal on the host and in the VM)

| App | Version | sha256 |
|-----|---------|--------|
| A | v1.2.0-vmA | 095d751b7a1bce6001bbbfb2482ea978839705f8603b7ea6b7f38e1256c41282 |
| B | v1.2.0-vmB | 58678320a0efcdb44aa14b66346d6132117ba9331d239ab6a2c2b0077044be18 |
| B-fault (vmpreview) | v1.2.0-vmB | 86a5b15f5c75107f588228b747e66dd5d76659cd5a48622282020b80dc988f4d |
| A-fault (vmpreview) | v1.2.0-vmA | ddf8f73c5b2b08d125faf17e2475f530199801c464225733973e093a278c3f45 |
| HEAD (deployed at the end) | dev | 79b11d05090615eedec2ca56c8739fd9283bc2e39653fed64c98e8c6920c11f7 |
| OLD (v1.1 CLI/daemon) | v1.1.0-vmOld | 894168b7769e448db5f0bdf71ae9cd4773d8e5f33256bd51a384caea40f80821 |

## Deviations from Plan

1. **[Rig] OLD is a CLI/daemon build** from `git archive v1.1`, made with plain `go build` as in 09-06. It is not a Wails app built in a `git worktree add` checkout. Every OLD row only needs a v1.1 daemon.
2. **[Rig] Home VM upgraded to HEAD at the start**, so the home viewers in V-02 and V-03a run this commit.
3. **[Rig] The vim ticker comes from a sourced file** (`vim -u NONE -N -S tick.vim`) instead of a one-line `-c`. It is the same 1 s `timer_start`; the change avoids quoting trouble through the FIFO.
4. **[Rule 1, own check] Fresh viewers must use the running daemon's build.** In the first V-03a attempt, the check opened a viewer with the installed B-fault binary, and that `portty attach` upgraded the A daemon. The attempt was discarded and re-run. From then on, `view.sh` takes the binary explicitly.
5. **[Rule 1, own check] SOH bytes in raw replays.** By design, the ring's `0x01` frame headers ride along in raw replays. A plain grep under-counted V-03d's history (297 of 300). The checks now strip SOH first. This is explained in the VM file; it is not a product defect.
6. **[Plan check W6] Stopped at V-05b FAIL, then resumed after the fix.** The first pass stopped there. The quit path was then fixed (09-07-FIX.md), V-05b was re-run, and V-05c and V-05d were run, all PASS. The FAIL row stays in 09-VM-CHECKS.md for the record, next to its re-run row. Because of that, the plan's verify check "no FAIL row remains" still matches it; the user asked for the row to be kept.
7. **[Re-run rig] One more V-05b run with AX captures** (`v05b2.sh`) recorded the `Restarting…` button and the Quit dialog's unknown-count line. V-05c and V-05d use new helpers `v05c.sh` and `v05d.sh` in `~/p907` on office. Rebuilt OLD and B have new sha256 values (the builds are not reproducible byte for byte); both are in 09-VM-CHECKS.md.

## Known Stubs

None.

## Threat Flags

None. T-09-35: kills, faults and SIGSTOP ran only in the office VM. T-09-36: share checks record codes only, and the cap/link grep on 09-VM-CHECKS.md finds 0. T-09-37: the Safari cleanup is recorded. T-09-38: both VMs are back on HEAD with settings restored.

## Self-Check: PASSED

- FOUND: .planning/phases/09-upgrades-keep-sessions/09-VM-CHECKS.md (rows V-00..V-05b, Cleanup), TESTING.md last-run lines
- FOUND (ancestors of HEAD): 3784d0a6, a2a7a55b, 6f34b180, adb94635
- Secret grep on 09-VM-CHECKS.md: 0. `git status --porcelain -- frontend/src/wailsjs`: empty. Traceability gate: OK (163 paths)
