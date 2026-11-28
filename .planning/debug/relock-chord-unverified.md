---
status: awaiting_human_verify
trigger: "RL-01r (23.10.5 VM re-check, 23-VM-CHECKS Notes 20-25): re-lock armed after a remote unlock; last real own viewer left 06:35:21; `relocked` audited 06:35:52 and relock_armed false afterwards, but the Mac never locked (GET /desktop locked=false at every read through 06:37:54, no loginwindow lock line) and a recovered viewer rejoined the unlocked desktop. Office load ~100, busy Terminal frontmost. Control (no load, Finder frontmost) locked at 30 s. relockDecide posts Ctrl+Cmd+Q, writes `relocked` and disarms without re-reading the lock state"
created: 2026-11-28T08:00:00Z
updated: 2026-11-28T10:30:00Z
---

## Current Focus

hypothesis: CONFIRMED. relockDecide treats a successfully posted Ctrl+Cmd+Q as a locked Mac: it audits relocked, counts relocked and disarms in the same pass, with no SessionLocked read afterwards, so a chord macOS drops (load ~100, busy Terminal frontmost) is never retried and the audit claims a lock
test: TestRelock_LostChordIsRetriedNotClaimed, TestRelock_LockReadErrorIsNotLocked, TestRelock_ViewerBackPausesTheRetries (RED on d9b71c13)
expecting: GREEN after the fix with every other TestRelock_/TestUnlock still passing (met, see Resolution.verification)
next_action: VM re-check on a build with this fix (orchestrator / user): see Resolution.vm_recheck and TESTING.md M-223; then archive to resolved/ and add the knowledge-base entry
bug_class: Bohrbug (deterministic on the fake clock once the chord is lost; the loss itself is environmental)

reasoning_checkpoint:
  hypothesis: "The Mac stayed unlocked with a relocked audit line because relockDecide equates 'six key events posted without error' with 'Mac locked': it audits relocked and disarms in the posting pass, so when macOS drops the chord (load ~100, busy Terminal frontmost) nothing reads the lock state again and nothing re-posts"
  confirming_evidence:
    - "relock.go relockDecide: postLockChord() == nil -> audit relocked, count relocked, log, disarmRelock(); no SessionLocked read after the post"
    - "EW-01r.txt: relocked 06:35:52 then locked=false at every read to 06:37:54 and relock_armed false; the control run (no load) locked 2 s after relocked with the same code"
    - "RED: with the fake chord lost, the rule disarms and claims relocked at the first post (three new tests)"
  falsification_test: "With relocked audited only after a later pass reads SessionLocked true (err nil) and the rule kept armed and re-posting otherwise, the lost-chord test still sees relocked or a disarm on an unlocked Mac, or the existing TestRelock_ suite (which now needs one confirming pass) fails"
  fix_rationale: "The claim moves from the post to the observation: relocked is written only by the top-of-pass lock read that already ends the rule, and only when this rule's chord was posted; a Mac that keeps reading unlocked stays armed, so the existing one-second loop re-runs relockDecide (same physical-input rules, held keys released again) at graceAt, which is reused as the next-try time. No new goroutine or timer"
  blind_spots: "Why macOS dropped the chord (load vs. the busy frontmost Terminal, or the 23-10 finding-6 'Portty window in front') is not separated and is not fixed here; if the chord never takes on a given Mac the rule now reports relock_failed and keeps posting every 30 s but the Mac stays unlocked. Untagged input appearing after a lost chord still disarms (the unchanged physical-input rule). The real lock-flag lag under load is unmeasured; a 5 s confirm window may re-post on a Mac that is already locking (harmless: Ctrl+Cmd+Q at the lock screen)"
  candidate_causes:
    - "code: relockDecide audits relocked and disarms on a nil post error without reading SessionLocked (root cause of the false claim and of no retry)"
    - "environment: load ~100 and/or a busy Terminal (AppleEvent timeouts) frontmost made the WindowServer/loginwindow drop or not act on the posted Ctrl+Cmd+Q (trigger; outside the daemon's control)"
    - "config/data: none (relockGrace a compiled constant; the chord's key codes and flags match the control run that locked)"
  and_gate: "yes: the open Mac with a false audit needs the environment dropping the chord AND the code not confirming; the fix makes the code side fail-secure for any chord-loss cause"

## Symptoms

expected: after the grace the Mac is locked and only then is `relocked` audited and the rule disarmed (UNLK-01, fail-secure); a chord that does not lock the Mac is re-posted, and a re-lock that never takes is reported as failed, never as relocked
actual: `relocked` audit line and "the Mac was locked again" log at 06:35:52 (31 s after the 06:35:21 leave), relock_armed false from 06:35:59, but GET /desktop locked=false at every read 06:35:59-06:37:54, no loginwindow lock-screen line; the recovered home viewer joined the unlocked desktop at 06:36:18
errors: none logged; the daemon logged success ("desktop: the Mac was locked again after the last own-account viewer left")
reproduction: .planning/phases/23-audio-adaptive-quality-unlock-and-release/evidence/EW-01r.txt lines 88-117 (leave 06:35:21, relocked 06:35:52, locked false through 06:37:54); control RL-01r.control-unlock.txt (leave 06:40:45, relocked 06:41:15, locked true 06:41:17)
started: since the UNLK-01 re-lock shipped (relockDecide never confirmed the lock); surfaced on build 23.10.5 under load with Terminal frontmost

## Eliminated

## Evidence

- timestamp: 2026-11-28T08:00:00Z
  checked: .planning/debug/knowledge-base.md
  found: closest entry unlk01-late-success-retype (a loaded Mac's authorization and lock flag lag; fix read the lock fresh instead of assuming the outcome). Same shape: an action on a loaded Mac whose outcome was assumed, not read
  implication: candidate pattern "assumed outcome under load"; also warns the lock flag can lag on a loaded Mac, so the confirm window must tolerate a late flag (re-posting on a Mac that is locking is harmless)

- timestamp: 2026-11-28T08:05:00Z
  checked: internal/desktop/relock.go relockPoll, relockDecide, postLockChord; lease.go leasePass
  found: relockDecide, after a postLockChord that returned nil, sets lockUntil, audits AuditRelocked, counts "relocked", logs the locked-again line and calls disarmRelock; there is no SessionLocked read after the post. postLockChord returning nil only means CGEventPost accepted six events. The only retry path is a post error (relock_failed counter, stays armed). relockPoll's top-of-pass SessionLocked read (err == nil && locked -> disarm) never sees the outcome because the rule is already disarmed
  implication: confirms the mechanism: a lost chord leaves an unlocked Mac, a false relocked audit line and a disarmed rule (nothing retries)

- timestamp: 2026-11-28T08:08:00Z
  checked: evidence EW-01r.txt 06:35:12-06:37:54 and the control
  found: 06:35:21 viewer left viewers=0; 06:35:52 INFO "the Mac was locked again"; stats relock_armed false from 06:36:24 read on; desktop locked false at every read to 06:37:54; recovered viewer joined 06:36:18. Control: relocked 06:41:15, locked true 06:41:17
  implication: the code path is identical in both runs; only the environment (load, frontmost app) decides whether the chord takes, and the daemon cannot tell the difference today

- timestamp: 2026-11-28T09:05:00Z
  checked: relock rig changed so its input bridge (macKeys) locks the fake Mac on a Ctrl+Cmd+Q press unless `lost` is set (replaces failingKeys, which becomes its `fail` knob); existing tests that read audit/armed state after the chord poll once more ("the next pass reads the Mac locked"); new tests TestRelock_LostChordIsRetriedNotClaimed, TestRelock_LockReadErrorIsNotLocked, TestRelock_ViewerBackPausesTheRetries; go test -race -run TestRelock_ on d9b71c13
  found: the three new tests FAIL ("the grace's end / a failed lock read / a viewer back on the unlocked Mac: the rule ended on a Mac that reads unlocked"); every existing TestRelock_ passes with the rig change (it is neutral on the old code)
  implication: RED reproduces RL-01r deterministically: one lost chord -> relocked written, rule disarmed, nothing retries; a failed lock read cannot matter because nothing reads

- timestamp: 2026-11-28T10:00:00Z
  checked: fix applied (relock.go relockEndIfLocked, posts, relockConfirm/relockTries/relockBackoff; audit.go AuditRelockFailed); TestRelock_|TestUnlock|TestAudit -race -count=3; 11 hand-seeded mutants; full desktop and daemon race suites
  found: GREEN, zero failures; 11 of 11 mutants killed by tests (see Resolution.verification); desktop suite one known load flake (TestQualityShaped_SlowViewerDoesNotStallOthers) that passes alone 3/3 and on a full re-run; daemon suite ok
  implication: relocked is now only written after a clean locked read; a lost chord is re-posted and reported as relock_failed, never as relocked

## Resolution

root_cause: "internal/desktop/relock.go relockDecide equated 'Ctrl+Cmd+Q posted without error' with 'Mac locked': on a nil postLockChord error it audited relocked, counted relocked, logged 'the Mac was locked again' and disarmed in the same pass, with no SessionLocked read afterwards. In RL-01r macOS did not act on the chord (load ~100, busy Terminal frontmost), so the Mac stayed unlocked, the audit trail claimed a lock, and the disarmed rule never retried; the recovered viewer joined the open desktop. AND-gate: the environment dropping the chord plus the code not confirming"
fix: "The claim follows the read, not the post. relockEndIfLocked (one helper, used by relockPoll's top-of-pass read and relockDecide's re-read) ends the rule only on SessionLocked true with a nil error (a read error is not locked) and writes relocked (audit, counter, log) only when the rule's own chord was posted (rl.posts > 0). After a successful post relockDecide keeps the rule armed and reuses graceAt as the next decision time: relockConfirm (5 s) after each of the first relockTries (3) posts, then relockBackoff (30 s). Each re-post runs the unchanged decision (locked -> end; healthy-tap input after the unlock -> disarm; held keys released; chord posted; optimistic input lock). On the post made with posts == relockTries (three unconfirmed) it logs WARN 'the Mac did not lock after the re-lock keys; still trying', counts relock_failed and audits the new event relock_failed, once per streak. An own-account viewer back clears graceAt (as before) and resets posts, so posting pauses while it is connected, a lock it makes is not claimed as relocked, and its leave starts a fresh 30 s grace and streak. No new goroutine or timer: the lease loop's one-second poll drives everything. Post errors keep their old path (counted, logged once per streak, retried each second). Stats: no new key. relock_armed now stays true until the Mac reads locked, which is the security-relevant bit ('armed and locked=false after the grace' = confirming or failing); the failure is in the audit file (relock_failed), daemon.log and the vmdesktop probe counters; a new /desktop/stats key would need the closed-key-set privacy review for no extra safety. Rig: p23-rig.sh's audit privacy summary includes relock_failed. TESTING.md: M-223"
verification:
  target_test: { result: pass, tests: [TestRelock_LostChordIsRetriedNotClaimed, TestRelock_LockReadErrorIsNotLocked, TestRelock_ViewerBackPausesTheRetries, TestRelock_LockWithAViewerBackIsNotClaimed], red_on: "d9b71c13 (first three: 'the rule ended on a Mac that reads unlocked'); the fourth was added after the fix to kill mutant M6", repeated: "TestRelock_|TestUnlock|TestAudit -race -count=3 zero failures" }
  mutation_check: { result: pass, reason_if_skipped: "no Stryker for Go; hand-seeded mutants at each fix site (scratch runner, relock.go restored after each)", mutant_killed: "11 of 11 by tests: M1b read error treated as locked (LockReadErrorIsNotLocked); M2 relocked claimed on the post (9 tests incl. LostChord); M3b never re-post (LostChord, LockReadError); M4 no backoff (LostChord); M5b no relock_failed audit (LostChord); M6 posts kept when a viewer is back (LockWithAViewerBack); M7b any lock claimed as relocked (LockedElsewhereDisarms, LockWithAViewerBack); M8 posts never counted (8 tests); M9 relock_failed one post early (LostChord); M10 disarm after the post, the old behaviour (8 tests); M11 confirm window 4 s (LostChord). Four first-draft variants (M1, M3, M5, M7) only failed to compile and were replaced by the compile-valid b variants" }
  no_op_deletion: { result: pass, deletion_justified_by_rca: "the removed audit/count/log/disarm after the post is moved into relockEndIfLocked behind the lock read (RCA: the claim was made on the post); failingKeys in relock_test.go is folded into the rig's macKeys.fail knob (same behaviour)" }
  adjacent_tests: { result: pass, suites_run: ["go test -race ./internal/desktop/: first run one known load flake TestQualityShaped_SlowViewerDoesNotStallOthers (quality controller, no relock path), re-run alone -count=3 ok (177.7 s); full package re-run ok (187.7 s)", "go test -race ./internal/daemon/ ok (133.6 s)", "go vet ./internal/desktop/ ./internal/daemon/ ./internal/screencap/ (default and -tags vmdesktop) clean", "go build ./... and -tags vmdesktop ./internal/... ok", "gofmt -l internal/ clean", "bash tests/check-brand.sh clean", "bash tests/check-traceability-paths.sh OK"] }
  revert_and_reconfirm: { result: pass, bug_returned_on_revert: true (RED on the original code; M10 restores the old disarm-after-post and M2 the old claim-on-post, both fail the new tests), fixed_on_reapply: true }
  guardrail_verdict: accepted
  human_verify_pending: "VM re-check (vm_recheck)"
files_changed: [internal/desktop/relock.go, internal/desktop/audit.go, internal/desktop/relock_test.go, internal/desktop/audit_test.go, .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/p23-rig.sh, TESTING.md]
oracle_type: specified (UNLK-01 fail-secure contract from the brief: relocked only after SessionLocked reads locked; read error is not locked; lost chord re-posted with spacing and reported relock_failed after bounded tries; keep trying on a backoff while armed and no own viewer is back) with boundary neighbours (no re-post at 4 s, re-post at 5 s; relock_failed not before the third unconfirmed post's window ends; no post at 29 s into the backoff, post at 30 s)
vm_recheck: "On a build with this fix, repeat RL-01 under load with Terminal frontmost (as RL-01r): remote unlock from the home viewer (relock_armed true), load ~100 with the busy Terminal motion window in front, close the home viewer and note the leave time; poll GET /desktop locked and /desktop/stats unlock.relock_armed every 2-5 s, tail daemon.log and desktop-audit.jsonl, and on the vmdesktop build read the probe's counters. PASS is one of: (a) the chord takes: locked=true, and the relocked audit line plus 'the Mac was locked again' log at or after the first locked read (now about 31-32 s after the leave, one poll after the chord, instead of exactly at the chord), relock_armed false after; or (b) the chord is lost: no relocked line while locked=false, relock_armed stays true, the chord is re-posted 5 s apart, and about 45 s after the leave one relock_failed audit line (mode own, the last viewer's node) plus WARN 'the Mac did not lock after the re-lock keys; still trying posts=3 every=30s' and counters.relock_failed 1, then a post every 30 s until one takes (locked=true, then relocked). A recovered viewer that rejoins during the retries pauses them (relock_armed stays true) and its leave restarts the 30 s. FAIL: a relocked line or relock_armed false while GET /desktop reads locked=false. Control (no load, Finder in front): relocked about 1 s after locked=true. If (b) repeats with every post lost, the chord itself is not reaching loginwindow under load and needs a separate lock path (not in this fix)"
