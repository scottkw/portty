---
status: awaiting_human_verify
trigger: "U-load (23.10.3 re-check, 23-VM-CHECKS notes item 10): after a remote unlock armed the re-lock, a wedged encoder made every join fail at once (capture start failed -> join failed -> viewer left); the page kept retrying and relock_armed stayed true with viewers 0 and the Mac unlocked for more than 5 min; relocked fired only about 30 s after Safari was closed. Each join attempt, even a failed one, restarts the 30 s grace"
created: 2026-11-27T22:00:00Z
updated: 2026-11-27T23:00:00Z
---

## Current Focus

hypothesis: CONFIRMED. join() calls relockViewer(true, v) right after registering v and before startSource; a start failure then calls leave(v) -> relockViewer(false, v), which finds graceAt zero (cleared by the join) and sets it to now+30 s, so every failed join restarts the grace. Second path: ownViewers counts registered viewers whose start is still in flight, holding the poll off
test: TestRelock_FailedJoinsDoNotRestartTheGrace, TestRelock_JoinStillStartingDoesNotHoldOffTheLock (RED on 859746f7)
expecting: GREEN after the fix; all other TestRelock_/TestUnlock_ still pass (met, see Resolution.verification)
next_action: VM re-check on a build with this fix (orchestrator / user): see Resolution.vm_recheck; then archive to resolved/ and add the knowledge-base entry
bug_class: Bohrbug (deterministic, fake clock)

reasoning_checkpoint:
  hypothesis: "The re-lock is deferred because an own-account join is counted by the rule from registration, before its capture start: relockViewer(true) clears the grace and the failed start's leave restarts it at now+30 s; and ownViewers counts the not-yet-started join so the poll is held off while a start is in flight"
  confirming_evidence:
    - "U-load: relocked came ~30 s after the last failed join, not 30 s after the last real leave (03:12:25 -> 03:18:27)"
    - "manager.go: relockViewer(true, v) at join line 957 precedes startSource; join's failure path calls leave -> relockViewer(false, v) with graceAt zero -> now+30 s"
    - "RED: failed joins at +10/+20/+29 s give no chord at +30 s; a start blocked across +30 s gives no chord"
  falsification_test: "With relockViewer(true) only after a successful join and ownViewers counting only such viewers, the chord still fails to come at +30 s in either test, or TestRelock_ReturnCancels (a real rejoin cancels) fails"
  fix_rationale: "The rule's notion of an own viewer becomes 'an own viewer whose join worked' (flag set by relockViewer(true), called after join returns nil); a failed or in-flight join is invisible to the grace and the due check, so it can neither restart, extend nor disarm it; a real rejoin still clears the grace"
  blind_spots: "An own viewer that joined while another viewer's start was in flight (start not needed, join returns nil) counts as working even if that start later fails; it is connected and the state loop retries, the same as a viewer told 'no permission'. Real-page timing on the VM not re-run here"
  candidate_causes:
    - "code: relockViewer(true) before the capture start; ownViewers counts registered-but-not-joined viewers"
    - "environment: wedged VideoToolbox encoder service under load makes every start fail (the trigger, not the cause of the deferral; separate encoder-recovery work)"
    - "data/config: none (relockGrace is a compiled constant; page retry cadence only sets how often the grace restarts)"
  and_gate: "yes: the deferral needs the code ordering AND repeated failing starts (environment) AND a page that retries inside 30 s; the code fix removes it for any start-failure source"

## Symptoms

expected: the Mac re-locks about 30 s after the last own-account viewer that was actually viewing leaves (UNLK-01); a join that never becomes a working viewer does not restart or extend that grace, and does not disarm it
actual: every failed join attempt (encoder start -12902) restarted the 30 s grace; the page retried every few seconds, so relock_armed stayed true with viewers 0 and the Mac unlocked for more than 5 min after the last real leave (03:12:25); relocked came at 03:18:27, about 30 s after the joins stopped
errors: "capture start failed err=screencap: starting the encoder failed (code -12902)", "viewer left viewers=0", "join failed"; stats {"viewers":0,"unlock":{"relock_armed":true}} with locked=false
reproduction: .planning/phases/23-audio-adaptive-quality-unlock-and-release/evidence/U-load-encoder.txt (03:12:25 last real leave, failed joins from 03:12:33 every ~3-12 s; step 1 closes the page and relocked follows at 03:18:27)
started: since UNLK-01 re-lock shipped (relockViewer called from join before the capture start); surfaced by the U-load encoder wedge on build 23.10.3

## Eliminated

## Evidence

- timestamp: 2026-11-27T22:00:00Z
  checked: .planning/debug/knowledge-base.md
  found: no entry on relock, grace or failed joins
  implication: no known-pattern candidate

- timestamp: 2026-11-27T22:05:00Z
  checked: evidence/U-load-encoder.txt and 23-VM-CHECKS notes item 10
  found: last real leave 03:12:25 (audit leave viewer_closed); from 03:12:33 every join logs capture start failed -12902, viewer left viewers=0, join failed, roughly every 3-12 s; relock_armed true with viewers 0 throughout; step 1 (page closed, Safari quit, no joins) -> relocked at 03:18:27, ~30 s after the last attempt; failed joins write no join/leave audit lines (OnViewerJoin/Leave never ran)
  implication: the grace restarts on every attempt; the attempts never became viewers

- timestamp: 2026-11-27T22:10:00Z
  checked: internal/desktop/manager.go join, startSource, leave; relock.go relockViewer, ownViewers, relockPoll
  found: join registers v in m.viewers and calls relockViewer(true, v) (graceAt cleared) BEFORE startSource; a start error that is not permission/no-display calls leave(v), whose relockViewer(false, v) finds own == 0 and graceAt zero and sets graceAt = now + 30 s. ownViewers counts every registered ModeOwn viewer, so relockPoll's due check (ownViewers() == 0) is also held off while a join's capture start is in flight (up to ~35 s for a start held by a macOS dialog). leave is only called from serveAdmitted's defer (after a successful join) and from join's failure path, so a call placed after join returns nil is strictly ordered before that viewer's leave
  implication: confirms the hypothesis; a second path (in-flight start counted as a viewer) extends the grace even without the restart

- timestamp: 2026-11-27T22:40:00Z
  checked: new tests TestRelock_FailedJoinsDoNotRestartTheGrace and TestRelock_JoinStillStartingDoesNotHoldOffTheLock against the unfixed code (go test -race -run TestRelock_)
  found: both FAIL with "posted 0 key events [], want the Lock Screen chord (6)": failed joins at +10/+20/+29 s leave no chord at +30 s; a join whose start is blocked across +30 s holds the lock off. All 12 existing TestRelock_ pass
  implication: reproduced deterministically on the fake clock (RED)

- timestamp: 2026-11-27T22:55:00Z
  checked: fix applied; TestRelock_|TestUnlock|stats tests -race -count=3; five hand-seeded mutants (scratch script, each applied, TestRelock_|TestUnlock run with -race, files restored)
  found: GREEN, zero failures. M1 ownViewers counts not-yet-working joins -> killed by JoinStillStartingDoesNotHoldOffTheLock; M2 old order (count at registration) -> killed by both new tests; M3 a never-counted leave still runs the rule -> killed by FailedJoinsDoNotRestartTheGrace (the relocked line named the failed device home-phone); M4 a working join never counted -> killed by ReturnCancels; M5 counted at registration and after the start -> killed by both new tests
  implication: each part of the fix is load-bearing for a test; M2 is the revert of the ordering change and brings the RED back

## Resolution

root_cause: "internal/desktop: the re-lock rule counted an own-account viewer from the moment join() registered it, before its capture start. join called relockViewer(true, v) (grace cleared) before startSource; a start failure (the wedged encoder's -12902) called leave(v), whose relockViewer(false, v) found own == 0 with the grace cleared and set it to now + 30 s. So every failed join restarted the 30 s grace, and a page retrying faster than 30 s kept an unlocked Mac unlocked indefinitely. Second path, same cause: ownViewers counted every registered own viewer, so relockPoll's due check was also held off while a join's capture start was in flight (up to ~35 s for a start held by a macOS dialog). AND-gate: the code ordering plus repeated start failures (the encoder wedge, separate work) plus a page that retries inside 30 s"
fix: "An own-account viewer counts for the re-lock rule only once its join worked. viewer.counted (guarded by vmu) is set by relockViewer(true, v), now called in serveAdmitted after join returns nil (after the capture start; strictly before that viewer's deferred leave) instead of inside join before startSource. ownViewers counts only counted own viewers, so a join still starting or that failed neither holds off relockPoll nor appears in armRelock. relockViewer(false, v) returns early for a viewer that was never counted, so a failed join's leave neither restarts, ends nor re-attributes the grace (rl.who stays the last real viewer, the relocked audit line names it). The redundant `|| joined` in the grace switch is dropped so a join can only clear the grace when a counted own viewer is really registered (fail-secure). A real own-account rejoin within the grace still cancels it (TestRelock_ReturnCancels unchanged). Side effect: /desktop/stats own_viewers now counts the same set (own viewers whose join worked)"
verification:
  target_test: { result: pass, tests: [TestRelock_FailedJoinsDoNotRestartTheGrace, TestRelock_JoinStillStartingDoesNotHoldOffTheLock], red_on: "859746f7 (posted 0 key events, want the chord)", repeated: "TestRelock_|TestUnlock -race -count=3 zero failures; new tests -race -count=20 (see adjacent_tests)" }
  mutation_check: { result: pass, reason_if_skipped: "no Stryker for Go; five hand-seeded mutants at the fix sites", mutant_killed: "5 of 5" }
  no_op_deletion: { result: pass, deletion_justified_by_rca: "the removed relockViewer(true, v) in join is moved, not deleted (RCA: it ran before the capture start); the removed `|| joined` was redundant under the new ordering and was the fail-open branch" }
  adjacent_tests: { result: pass, suites_run: ["go test -race ./internal/desktop/ ok (185.6 s)", "go test -race ./internal/daemon/ ok (132.6 s)", "new tests -race -count=20 zero failures", "go build ./internal/... (default and -tags vmdesktop)", "go vet ./internal/desktop/ ./internal/daemon/ ./internal/screencap/ (default and -tags vmdesktop) clean", "gofmt -l internal/ clean", "bash tests/check-brand.sh clean"] }
  revert_and_reconfirm: { result: pass, bug_returned_on_revert: true (RED on the original code; M2 reverts the ordering alone and both new tests fail again), fixed_on_reapply: true }
  guardrail_verdict: accepted
  human_verify_pending: "VM re-check (vm_recheck)"
files_changed: [internal/desktop/relock.go, internal/desktop/manager.go, internal/desktop/relock_test.go]
oracle_type: specified (UNLK-01 re-lock contract from the brief: lock 30 s after the last own viewer that was actually viewing left; failed or in-flight joins neither restart, extend nor disarm; a real rejoin cancels) with boundary neighbours (no chord at +29 s right after a failed join, chord at +30 s; a start in flight across +30 s)
vm_recheck: "On a build with this fix, repeat the U-load re-lock half without needing the load: remote unlock from the home viewer (relock_armed true), close the viewer (note the leave time), then make joins fail at once (wedge the encoder under load as in U-load, or any capture start failure) while the home page keeps retrying. Expect: relocked audit line and locked=true about 30 s (first poll at or after) after the last real leave, while failed joins continue; relock_armed true until then and false after; the relocked line names the viewer that left (node home-air VM); no relocked line earlier than 30 s; a real rejoin that streams inside 30 s still cancels (U-10r style). Evidence: lockstate/stats poll every 5 s plus daemon.log and audit tail"
vm_result: "2026-11-28, build 23.10.4 (9ee3d660, carries 508c3d03), RL-01 RESULT=NOT-RUN. The failing joins were to come from the encoder wedge (EW-01). The wedge reproduced (first -12902 start failure 04:54:09) but no daemon unlock had armed the re-lock before it (relock_armed false; the lock and unlock cycles under load ended through the keyboard path because the viewer was gone at each submit), and the office VM's tailscaled exited at 04:53:56 under load (launchd did not restart it), so no join reached the daemon afterwards and the run stopped (off-tailnet rule). The fix is unverified in the VM. Evidence: phase 23 evidence/EW-01.txt; row RL-01 in 23-VM-CHECKS.md"
