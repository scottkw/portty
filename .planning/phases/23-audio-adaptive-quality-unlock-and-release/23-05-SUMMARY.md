---
phase: 23-audio-adaptive-quality-unlock-and-release
plan: 05
subsystem: desktop-unlock
tags: [unlock, relock, audit, fail-secure, runtime-state, input-tap]
requires:
  - phase: 22-click-to-share-for-tailnet-people
    provides: the local audit log (22-02), PhysicalState and the input tap life cycle (22-03, 22-05), own-account identity on the viewer (22-04, 22-06)
  - phase: 23-audio-adaptive-quality-unlock-and-release
    provides: the unlock sequence, the unlockEnd hook point and the RelockArmed finding (23-04)
provides:
  - audit events unlocked, unlock_failed and relocked (own mode, the viewer's identity, fixed reasons only)
  - the re-lock rule: armed by a Portty unlock, 30 s grace after the last own-account viewer leaves, Ctrl+Cmd+Q posted through the input bridge, fail secure, kept across a restart
  - Deps.Audit seam, wired in the daemon to the existing audit appender
  - the tap stays up while re-lock is armed
affects: [23-07 unlock panel (RelockArmed is in UnlockStats), 23-10 VM rows, 23-12 TESTING.md, 23-14 real-Mac UAT]
status: complete
commits: 2
plan_head_before: b6bcc1f0542d2b4632a7adb16721f8885bfc204d
plan_head_after: b3e3ca26229edfb619efd55f5ae25e64d2b52538
actuals:
  tokens: 31000
  tasks: 3
  commits: 2
tech-stack:
  added: []
  patterns:
    - "rule driven by the lease loop's one-second poll with the clock injected, so tests step a fake clock and call one pass"
    - "own-account viewer count derived from the viewer set under rl.mu (never counted), so a racing join and leave cannot leave a stale timer"
    - "state-file fields written only while set (omitempty / omitzero), so a file that never armed is byte for byte the old format"
key-files:
  created:
    - internal/desktop/relock.go
    - internal/desktop/relock_test.go
    - internal/desktop/export_relock_test.go
  modified:
    - internal/desktop/audit.go
    - internal/desktop/audit_test.go
    - internal/desktop/unlock.go
    - internal/desktop/manager.go
    - internal/desktop/lease.go
    - internal/desktop/runtime_state.go
    - internal/desktop/unlock_test.go
    - internal/desktop/unlock_redact_test.go
    - internal/daemon/desktop_share_api.go
key-decisions:
  - "Audit constants follow the file's existing naming (AuditUnlocked, AuditUnlockFailed, AuditRelocked), not the plan's EventUnlocked spelling; the wire strings are the planned ones"
  - "unlock_failed carries one of two fixed reasons, still_locked or error, so the ordinary key stays free of anything derived from the password"
  - "The Manager reaches the audit file through Deps.Audit (event, identity, reason); the daemon wires it to API.record, which owns the lazily opened file"
  - "Both fail-secure signals are sticky for the armed window and the decision is lock unless a healthy tap saw input and no reading was ever unhealthy (the plan's wording), so an unhealthy poll overrides earlier input"
  - "A chord that could not be posted keeps the rule armed and retries each second; relocked is written only when the keys went out"
patterns-established:
  - "Re-lock verification is by construction, not by read-back: a Portty unlock needs the same HID posting path, so it only arms when posting already worked"
requirements-completed: [UNLK-01]
---

# Phase 23 Plan 05: Unlock audit, re-lock and lock gate Summary

A Portty unlock is now written to the local audit log, and the Mac locks itself again about 30 seconds after the last own-account viewer leaves unless somebody used it. A missing or unhealthy physical-input signal locks, the arm survives a daemon restart, and the lock-time input rule is proven for every viewer.

## What was built

- **audit.go**: `AuditUnlocked`, `AuditUnlockFailed`, `AuditRelocked` join the closed event list (strings `unlocked`, `unlock_failed`, `relocked`); `ReasonStillLocked` and `ReasonUnlockError` are the only reasons an unlock line carries. No field was added; the closed-list test names all fifteen events.
- **unlock.go `unlockEnd`** (the hook point 23-04 left): every attempt that typed writes one line, mode own, the submitter's `Identity`. Success arms the rule before the viewer hears `unlocked`. Refusals before typing, rate limits and pauses write nothing. `unlockState.armed` is gone; `UnlockStats.RelockArmed` now reads the one source of truth, `relock.go`.
- **relock.go**:
  - `armRelock(at)` records the unlock time, resets both signals, starts the grace at once if no own-account viewer is connected, and persists the arm.
  - `relockViewer(joined, v)` runs from `join` and `leave` for own-account viewers. The count is read from the viewer set under `rl.mu`, so a join racing a leave cannot leave a timer running over a connected viewer. A return cancels the grace, the last leave starts `relockGrace` (30 s).
  - `relockPoll(now)` runs once a second from the lease loop, after the tap's own poll: disarm if a fresh lock read says the Mac is locked (by any other means), sample the physical source, decide once the grace is over and still no own-account viewer.
  - `relockDecide`: Mac found locked, or healthy tap showed untagged input after the unlock and no reading was ever unhealthy, ends the rule; anything else locks. The no-signal case logs one fixed line. Lock = `releaseHeldAll("relock")`, then Ctrl+Cmd+Q down and up through `Input.PostKey` (the bridge tags every event as Portty's), then the optimistic `lockUntil`, the `relocked` line, disarm.
  - `postLockChord` releases every key even if one post fails and returns the first error; on error the rule stays armed and retries each second (`relock_failed` counter, one log line per streak).
- **lease.go**: `syncTap` wants the tap while `AnyController() || relockArmed()`; `leasePass` runs `relockPoll` after `pollTap`.
- **runtime_state.go**: `RelockArmed` and `RelockSince`, written only while armed. At `NewManager`, a persisted arm with the Mac not found locked re-arms with the daemon's start time as the "since" (only input after the start can disarm it); a Mac found locked clears the flag.
- **daemon/desktop_share_api.go**: `wireGuestDeps` sets `Deps.Audit` to `API.record` (own mode, empty share ID). This file is not in the plan's list; without the wire the events would go nowhere in production.
- **Lock gate (Task 3)**: tests only. The Phase 20 rule already held; no daemon code needed changing (see Deviations).

## Results the plan asked to be recorded

- **Precondition**: the 22-02, 22-03, 22-05, 22-06 and 23-04 SUMMARYs and `internal/desktop/audit.go` exist. `git log --oneline 1c55b159..HEAD -- audit.go lease.go manager.go runtime_state.go` at base: 2e5e0add (23-04), 45ee75ad (23-02), 2057d0d9 and 0467877b (23-01), 88e098af and 76bc8b51 and b570ae27 (22-05), a4fb5d09 (22-04), 588285e9 and 98b547a2 (22-02), d47bd7eb, efcc60d2, 1dc75276, a8c3ceb0.
- **Test runs**: `go test -race -count=3 -run 'TestUnlockLockGate_|TestUnlockRedact_|TestRelock_|TestAudit|TestUnlock_'` passes; the full `internal/desktop`, `internal/screencap` and `internal/daemon` packages pass with `-race`; `golangci-lint --new-from-rev=1c55b159` reports 0 issues on `internal/desktop` and `internal/daemon`; `tests/check-brand.sh` clean. 11 `TestRelock_` and 3 `TestUnlockLockGate_` tests are listed.
- **Mutation checks** (reverted before commit): `seen && !unknown` changed to `seen` made `TestRelock_UnhealthyTapLocks` fail; an unlock line with reason `len 17` made the sentinel subtest of `TestUnlockRedact_NoLeakThroughTheRealChannel` fail twice (leak grep and fixed-reason check).

## New test files for 23-12 (TESTING.md Sections 2 and 4)

- `internal/desktop/relock_test.go`: `TestRelock_Tracer`, `_ReturnCancels`, `_PhysicalInputDisarms`, `_InputBeforeTheUnlockDoesNotCount`, `_UnhealthyTapLocks`, `_TapStaysUpWhileArmed`, `_RestartResumes` (three subtests), `_LockedElsewhereDisarms`, `_NotArmedByAnUnlockAtTheMac`, `_ChordFailureStaysArmed`, `_FailedUnlockIsAuditedAndDoesNotArm`. Traceability row, UNLK-01.
- `internal/desktop/export_relock_test.go`: test-only export of one poll (not a test file in the traceability sense).
- Extended: `internal/desktop/unlock_test.go` (`TestUnlockLockGate_DropsEveryViewersPresses`, `_LockBeginReleasesHeldOnce`, `_AttemptDoesNotLoosenIt`; the rig gained `newUnlockRigWith`, `ownUnlockViewerAs`, the fake input bridge as `r.fi`), `internal/desktop/unlock_redact_test.go` (a real audit file under `t.TempDir()` after seven outcomes plus the new retry and pause ones), `internal/desktop/audit_test.go` (closed list now fifteen events and twelve reasons). All already have rows or files in the manifest.
- TESTING.md was not edited, as instructed.
- Manual items for Section 5 (none run on this host): see Not verified here.

## Not verified here (deferred, by the plan's host-safety rules)

Nothing was posted, locked, woken or typed on this host; the input bridge and unlock bridge were fakes throughout and the real tap was never started. For 23-10 (VM rows) and 23-14 (the office Mac with the user present):

1. **The chord locks a real Mac**: that `Ctrl+Cmd+Q` from `PostKey` (control and command key events with those flags, then Q) reaches the Lock Screen action on a logged-in Mac from the launchd daemon, and that the Mac is locked within the 3 s optimistic window. The Phase 20 viewer path posts the same keys but the daemon posting the whole chord itself is new.
2. **Tap health right after start**: `PhysicalReading.Healthy` is false until the tap runs and has settled. `leasePass` polls the tap before `relockPoll`, but a tap that reads unhealthy on its first second sets the sticky unknown flag and the Mac locks even if someone then types. Safe direction, but check on a VM whether a freshly started tap reads healthy by the first poll, otherwise a restart-resumed arm will always lock.
3. **Untagged input as the signal**: that typing the password at the lock screen, moving the mouse and a key at the keyboard each give `Seen` with a small `Idle` after the unlock, while the Portty wake key and typed strokes (tagged) do not.
4. **Restart handoff**: kill the daemon while armed with the Mac unlocked and confirm the new one locks 30 s after its start; and that an old build reading the new state file with `relockArmed` present drops to "state file unusable, persistence off" (its decoder rejects unknown fields). The fields are written only while armed, so a Mac that is not mid-session is unaffected.
5. **One-second poll granularity**: the lock comes at the first poll at or after 30 s, so up to about 31 s; fine for "about 30 s", noted for the UI sentence in 23-07.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Tasks 1 and 2 are one commit**
- **Issue:** `relock.go` (Task 1) calls `m.persist` with the new state-file fields and reads `m.rl.resume`, both Task 2 files, and the tracer's arm must keep the tap up (`lease.go`). The two tasks do not compile apart.
- **Fix:** one commit covers the code and the `TestRelock_` tests of both; Task 3 is its own commit.
- **Commits:** 013d72a1 (Tasks 1 and 2), b3e3ca26 (Task 3)

**2. [Rule 3 - Blocking] The daemon had to wire the audit**
- **Issue:** the audit appender lives in the daemon's `API`, not the Manager; the plan's file list has no daemon file, so the events would have been dropped in production.
- **Fix:** `Deps.Audit` seam on the Manager and a six-line wire in `wireGuestDeps`.
- **Files modified:** internal/daemon/desktop_share_api.go
- **Commit:** 013d72a1

**3. [Plan wording] Task 3 needed no daemon fix**
- The plan expected a possible fix for "held keys are released when the lock begins". The input goroutine's own tick (`inputTick`, Phase 20) already releases whatever a viewer holds when the screen locks, once. `TestUnlockLockGate_LockBeginReleasesHeldOnce` proves it. The gate was not touched.

**4. [Plan wording] One holder, not two**
- The "keys held by two viewers" case cannot occur: the lease releases the previous holder when it changes, so at most one viewer holds anything. The test holds a key and a button for one viewer with another connected.

**5. [Plan wording] Audit constant names**
- `Audit*` prefix like the rest of the file instead of `Event*`; the strings are as planned.

**6. [Rule 2 - Missing critical functionality] A failed chord must not end the rule**
- If the post fails, the rule would otherwise disarm with the Mac unlocked. It stays armed and retries; `relocked` is written only after the keys went out. Tested by `TestRelock_ChordFailureStaysArmed`.

## Findings (not fixed here)

- **The lock is not read back after the chord.** The plan says record `relocked` and disarm, which this does. If the chord were silently ignored, the Mac would stay unlocked with the rule gone. A Portty unlock only arms after typing worked through the same HID path, so the risk is low, but row 1 above is where it is proven.
- **The audit has no reason on `relocked`** and identifies the last own-account viewer seen (empty identity after a restart). Enough for "the Mac was secured again"; add a reason (for example `physical_unknown`) only if the record is wanted to say why.
- **`unlock_failed` after a Type error mid-password** (reason `error`) is audited though the Mac never saw a whole password; kept because the attempt typed.
- **Flaky under load**: none seen this run; the known list in the hard constraints is unchanged.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-23-14 mitigated (grace, fail secure on an unknown signal, persisted arm, retry on a failed post; a test for each branch), T-23-15 mitigated (three events with the own-account identity, written before the viewer is told), T-23-16 mitigated (Phase 22 fields only, fixed reasons, sentinel, length and numeric-field checks on a real file after nine outcomes), T-23-SC mitigated (no package added).

## Self-Check: PASSED

- internal/desktop/relock.go, relock_test.go, export_relock_test.go: FOUND
- Commits 013d72a1, b3e3ca26: FOUND in `git log`
