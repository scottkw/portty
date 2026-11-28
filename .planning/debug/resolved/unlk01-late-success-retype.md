---
status: resolved
trigger: "UNLK-01 F-late-success: a slow macOS authorization (2.3 s) was read as still locked after the 3 s check; the daemon retyped the password (a real wrong try; in U-10 the retype hit the live desktop), reported unlock_failed and never armed the re-lock, leaving the Mac unlocked with no viewer"
created: 2026-11-27T19:50:00Z
updated: 2026-11-28T00:00:00Z
---

## Current Focus

hypothesis: three code facts in internal/desktop/unlock.go combine with a slow Mac: (1) unlockCheck is 3 s, shorter than authorization plus lock-flag clear under load; (2) nothing re-reads the lock right before a key, so the retype runs on a read up to 250 ms plus a progress broadcast old and keeps typing after the Mac unlocks; (3) the verdict is final, so a Mac that unlocks after unlock_failed is never treated as a Portty unlock and armRelock (only called for ulUnlocked in unlockEnd) never runs
test: fake-bridge unit tests in unlock_test.go / relock_test.go: unlock at 2.3 s after the first Return, flip to unlocked between the poll verdict and the retype, genuine wrong password, unlock after the failure verdict
expecting: the 2.3 s case currently retypes and reports failed with relock_armed false (RED); after the fix one typing, unlocked, relock armed
next_action: VM re-run on a build with this fix (orchestrator / user): loaded-VM unlock cycles, a wrong-password submit, U-10 re-lock, U-12 sentinel grep; then archive to resolved/ and add the knowledge-base entry
bug_class: Bohrbug (deterministic given the timing; reproduced exactly with a fake clock)

reasoning_checkpoint:
  hypothesis: "The Mac is left unlocked with no re-lock because (a) the 3 s poll ends before a loaded Mac clears CGSSessionScreenIsLocked after a slow authorization, (b) the retype posts keys with no fresh lock read so it types after the Mac unlocked (or as a wrong try into the dismissing lock UI), and (c) the failure verdict is final so an unlock that lands after it never reaches armRelock"
  confirming_evidence:
    - "F-late-success: auth succeeded 2.3 s after loginPressed, the daemon read locked through its 3 s window, retyped, reported unlock_failed, failures 1, relock_armed false, Mac later locked=false"
    - "U-10: stats show locked=false with state retrying in one sample; loginwindow logged nothing for the retype (it went to the live desktop)"
    - "Fake-clock tests: flag clearing at 4.5 s / 9.75 s after Return produces a retype; a flip after the verdict still gets 17 strokes and Return typed; an unlock after failed is never reported"
  falsification_test: "With unlockCheck 10 s and a lock read before every key, a fake Mac that unlocks at 4.5 s or 9.75 s still sees a second typing, or a flip after the verdict still sees a retyped stroke, or an unlock after failed leaves relock_armed false"
  fix_rationale: "The longer bound covers the observed authorization plus dismissal delay; the per-key read makes 'never type into an unlocked desktop' structural instead of timing-dependent; the late watch closes the remaining tail so any unlock following a typed submit arms the re-lock exactly like a normal success"
  blind_spots: "The real lock-flag clear time under heavy load is not measured beyond F-late-success (at least 3.1 s after auth there); one key can still race the flag if macOS dismisses the lock window before clearing the flag; attribution of an unlock at the Mac within 30 s of a failed submit to Portty (fail-secure: arms the re-lock)"
  candidate_causes:
    - "code: unlockCheck 3 s and no pre-key read in typeStrokes; verdict-only armRelock in unlockEnd"
    - "environment: loaded VM (load average 15) slows loginwindow authorization and dismissal"
    - "config: none (constants are compiled in)"
  and_gate: "yes: the bad outcome needs the slow Mac (environment) AND the short window AND the unguarded retype AND the final verdict; fixing the code parts removes it for any environment speed"

## Symptoms

expected: a submit whose first typing unlocks the Mac is reported unlocked (audit `unlocked`, state unlocked, failures 0) and arms the 30 s re-lock; no key is ever posted unless the Mac reads locked right before it
actual: in a loaded VM authorization took 2.3 s; the daemon's 3 s poll read locked, it retyped (macOS: return in secure textfield, -60005 INCORRECT password; U-10: retype started the same second the Mac read unlocked, i.e. into the live desktop), reported unlock_failed / still_locked, failures 1, relock_armed false; the Mac stayed unlocked with no viewer
errors: audit `unlock_failed own still_locked`; lockstate locked=false {"unlock":{"state":"idle","failures":1,"relock_armed":false}}
reproduction: .planning/phases/23-audio-adaptive-quality-unlock-and-release/evidence/F-late-success.txt, U-10.unlock.txt; in unit tests: fake bridge that unlocks the fake Mac 2.3 s (fake clock) after the first Return
started: since UNLK-01 shipped the retry-once sequence (23-04); found in 23-10 P-01 (Chrome half)

## Eliminated

## Evidence

- timestamp: 2026-11-27T00:00:00Z
  checked: .planning/debug/knowledge-base.md
  found: no entry on unlock, lock poll or retype
  implication: no known-pattern candidate

- timestamp: 2026-11-27T00:00:00Z
  checked: evidence/F-late-success.txt
  found: loginPressed 01:24:02.166, "Screensaver authorization succeeded" 01:24:04.470 (2.3 s); the retype reached loginwindow at 07.59 with the lock UI still up ("return in secure textfield"), auth failed -60005 at 11.519; daemon audit unlock_failed at 01:24:11Z; later locked=false, failures 1, relock_armed false
  implication: the first typing unlocked the Mac; the lock flag stayed set more than 3 s after Return (authorization 2.3 s plus the dismissal, delayed further by the retype's own check)

- timestamp: 2026-11-27T00:00:00Z
  checked: evidence/U-10.unlock.txt and U-02.txt
  found: U-02 (unloaded) auth 0.36 s after loginPressed, unlocked read about 2 s after Return; U-10 auth 1.2 s, the stats read locked=false with state retrying at the same sample, and loginwindow logged nothing for the retype
  implication: the lock flag clears about 1.5 to 2 s after authorization succeeds even unloaded; under load the window passes 3 s, and a retype started on a stale "locked" read lands on the live desktop

- timestamp: 2026-11-27T00:00:00Z
  checked: internal/desktop/unlock.go attemptUnlock, typeStrokes, lockedAfterReturn, unlockEnd
  found: unlockCheck = 3 s; typeStrokes posts every stroke and Return with no lock read; the retype follows lockedAfterReturn's last read after an unlockProgress broadcast (sequential websocket writes); unlockEnd arms the re-lock only for ulUnlocked, ulFailed counts a failure; nothing looks at the lock after the verdict
  implication: matches the three-part hypothesis; no loginwindow failure signal exists in the bridge (UnlockBridge has only SessionLocked), so a longer bound is the available fix for (1)

- timestamp: 2026-11-27T00:00:00Z
  checked: internal/desktop/relock.go armRelock / relockPoll, lease.go leasePass
  found: armRelock(at) arms and starts the grace at once when no own-account viewer is connected; relockPoll runs each second from the lease loop for the Manager's life and disarms when the Mac reads locked
  implication: a late success can be armed from the same one-second lease poll; arming on a Mac someone else unlocked is fail-secure (their input after `since` disarms it)

- timestamp: 2026-11-27T20:05:00Z
  checked: new tests against the unfixed code (temporary no-op UnlockLatePollForTest)
  found: LateSuccessIsOneTyping 4.5s and 9.75s FAIL (a retrying state appears: the retype ran; at 9.75 s the old code ends failed with the Mac unlocking after, the F-late-success outcome); UnlockedBeforeTheRetypeTypesNothing FAIL (bridge saw 34 strokes and 2 returns: the retype typed after the verdict although the Mac read unlocked right after it); the mid-retype case FAIL; SuccessAfterTheVerdictCountsAsSuccess FAIL (no unlocked after failed); RetriesOnceThenFails FAIL (gap 3 s, want 10 s). 2.3s case and WrongPasswordStaysFailed PASS on the old code (expected: 2.3 s flag-clear is inside 3 s; the genuine-failure path is the regression guard)
  implication: hypothesis confirmed on all three parts, reproduced deterministically on the fake clock

- timestamp: 2026-11-27T20:40:00Z
  checked: nine hand-seeded mutants of the fix (scratch script, each applied, suite TestUnlock|TestRelock run with -race, file restored)
  found: all nine killed. M1 no pre-wake read -> UnlockedBeforeAnyKeyIsNotPorttys; M2 late success keeps the pause -> SuccessAfterTheVerdict/after_the_third_failure; M3 no pre-Return read -> UnlockedBeforeTheRetype/before_the_retype's_Return; M4 no pre-stroke read -> UnlockedBeforeTheRetype between/middle; M5 3 s check -> LateSuccessIsOneTyping 4.5s/9.75s, RetriesOnceThenFails, SuccessAfterTheVerdict; M6 late poll not wired into the lease loop -> SuccessAfterTheVerdict/after_the_first_failure; M7 late window never expires -> WrongPasswordStaysFailed; M8 no late watch armed -> SuccessAfterTheVerdict both; M9 unlocked-before-any-stroke claimed as Portty's -> UnlockedBeforeAnyKeyIsNotPorttys
  implication: every part of the fix is load-bearing for a test; reverting any part brings its failure back (revert-and-reconfirm per part)

## Resolution

root_cause: "AND-gate of three code facts in internal/desktop/unlock.go meeting a slow Mac: unlockCheck (3 s) ended before a loaded Mac cleared CGSSessionScreenIsLocked after a slow authorization; typeStrokes posted every stroke and Return with no fresh lock read, so the retype ran on a stale 'locked' read (into the live desktop in U-10, as a real wrong try into the dismissing lock UI in F-late-success); and unlockEnd armed the re-lock only for a verdict of ulUnlocked, so an unlock that landed after unlock_failed was never treated as Portty's and the Mac stayed open with no re-lock"
fix: "unlockCheck 3 s -> 10 s; lockedForKey (a fresh SessionLocked read) right before the wake key, every stroke and Return: a failed read ends in error and posts nothing, unlocked after a stroke went in ends as this submit's success (retype stopped, nothing more typed), unlocked before any stroke posts nothing and tells every unlock viewer idle; ulRun.typed now means a password stroke was posted; unlockEnd starts a 30 s late watch (lateBy/lateUntil) for a typed submit that did not end unlocked, and unlockLatePoll (each lease-loop second) turns an unlock inside it into the normal success path via unlockEnd (unlocked message by_portty, audit unlocked, armRelock, failures 0) and clears any pause the failure started"
verification:
  target_test: { result: pass, tests: [TestUnlock_LateSuccessIsOneTyping, TestUnlock_UnlockedBeforeTheRetypeTypesNothing, TestUnlock_UnlockedBeforeAnyKeyIsNotPorttys, TestUnlock_WrongPasswordStaysFailed, TestUnlock_SuccessAfterTheVerdictCountsAsSuccess, TestUnlock_RetriesOnceThenFails], repeated: "-race -count=5, zero failures" }
  mutation_check: { result: pass, reason_if_skipped: "no Stryker for Go; nine hand-seeded mutants at the fix sites instead", mutant_killed: "9 of 9" }
  no_op_deletion: { result: pass, deletion_justified_by_rca: n/a (the diff adds reads and a watch; the only removed lines are the old early r.typed = true and bool returns, replaced) }
  adjacent_tests: { result: pass, suites_run: ["go test -race ./internal/desktop/ (190 s)", "go test -race ./internal/daemon/ (142 s)", "go vet ./internal/desktop/ ./internal/daemon/ ./internal/screencap/", "go vet -tags vmdesktop (same)", "go build ./... and -tags vmdesktop ./internal/...", "gofmt -l internal/ clean", "bash tests/check-brand.sh clean"] }
  revert_and_reconfirm: { result: pass, bug_returned_on_revert: true (the RED run on the original code; M4, M5, M8 revert each part individually), fixed_on_reapply: true }
  guardrail_verdict: accepted
  human_verify_pending: "VM re-run (see Current Focus)"
files_changed: [internal/desktop/unlock.go, internal/desktop/lease.go, internal/desktop/unlock_test.go, internal/desktop/export_unlock_test.go]
oracle_type: specified (the brief's required behaviour: exact typing counts, states, audit lines, relock_armed, failure count)
vm_result: "2026-11-28, build 23.10.3 (5ea587fe), office under load. U-03r PASS: 10 typed cycles, each with exactly one typing and a success, authorization 0.40-2.08 s after loginPressed, failures 0, relock_armed true, no retype; 3 further cycles typed nothing because the viewer was reconnecting or gone. U-04r PASS: one wrong sentinel gave exactly two typed tries, the failed state 28.13 s after the click, failures 1, relock_armed false, no late success. U-10r PASS: relocked 31 s after the leave (under load macOS read the session locked at 41 s). U-12r PASS: 0 sentinel hits. New and outside this fix: U-load FAIL, because failed joins restart the 30 s re-lock timer (an unlocked Mac stayed unlocked for more than 5 min while a page retried against a wedged encoder). Evidence (phase 23 evidence folder): U-03r.*, U-04r.txt, U-10r.txt, U-10r.unlock.txt, U-12r.txt, U-load-encoder.txt"
resolved_note: "Resolved 2026-11-28 by the orchestrator on U-03r, U-04r, U-10r and U-12r (build 23.10.3, office under load): one typing per submit, no retype into an unlocked desktop, a wrong password still fails after exactly two typed tries, the re-lock fires on time. The re-lock deferral by failed joins seen in U-load is a separate session (relock-deferred-by-failed-joins)."
