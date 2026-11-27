---
phase: 23-audio-adaptive-quality-unlock-and-release
plan: 04
subsystem: desktop-unlock
tags: [unlock, password, daemon, security, cgo, UCKeyTranslate, throttle]
requires:
  - phase: 20-remote-control
    provides: the input data channel, releaseHeldAll, the lock gate (20-02)
  - phase: 22-click-to-share-for-tailnet-people
    provides: ModeOwn and ModeGuest viewers, the guest hello, the rights bits kept apart from the new capability (22-04, 22-05)
  - phase: 23-audio-adaptive-quality-unlock-and-release
    provides: additive hello caps and golden-line pattern (23-01), a settled quality controller on the same Manager (23-02)
provides:
  - binary 0x01 unlock request on the input channel, copied once into a redacting byte buffer and zeroed
  - viewer.unlockCap, set only on the own-account serve path, and caps unlock 1 on that hello only
  - the unlock sequence (fresh lock read, wake key, settle, typed strokes, Return, 3 s poll, one retype) behind an UnlockBridge seam
  - throttle (one attempt at a time, 5 s spacing, pause 15 minutes after 3 failed submits) and the {type: unlock} state message
  - screencap.UnlockBridge (UCKeyTranslate reverse map, Caps Lock read, wake, typed strokes)
  - Manager.UnlockStats for 23-09 and RelockArmed for 23-05
affects: [23-05 re-lock and audit, 23-07 unlock panel, 23-09 stats, 23-10 VM row U-01, 23-12 TESTING.md]
status: complete
actuals:
  tokens: 17100
  tasks: 3
  commits: 3
plan_head_before: f23eba380a8502889ace464f6fe587f4c44d8f65
plan_head_after: c74063b0608422d1ec80762ca1d643cb6a1436ec
tech-stack:
  added: []
  patterns:
    - "secret held as a struct whose every formatter (String, GoString, Format, MarshalJSON, LogValue) returns one marker"
    - "attempt goroutine with ordered defers: end-of-attempt report, strokes wipe, buffer zero, recoverLog"
    - "native bridge with the same shape as InputBridge: reads run in tests, every posting call refuses under testing.Testing()"
key-files:
  created:
    - internal/desktop/unlock.go
    - internal/desktop/unlock_test.go
    - internal/desktop/unlock_redact_test.go
    - internal/desktop/unlock_golden_test.go
    - internal/desktop/export_unlock_test.go
    - internal/screencap/unlock_darwin.go
    - internal/screencap/unlock_darwin.m
    - internal/screencap/unlock_darwin_test.go
  modified:
    - internal/desktop/input.go
    - internal/desktop/manager.go
    - internal/desktop/protocol_test.go
    - internal/desktop/fanout_test.go
    - internal/desktop/testdata/signal-v1.jsonl
key-decisions:
  - "The refused-before-typing and not-locked answers go to the submitter alone; attempt progress, results and the pause go to every unlock-cap viewer"
  - "A viewer that joins a locked Mac is always sent one unlock message (idle, unlocking, retrying or paused), so a reconnecting panel cannot keep a stale status"
  - "The spacing window starts only when typing began; a refusal never starts it, so a user who turns Caps Lock off can resubmit at once"
  - "When a pause ends the failure count restarts at 0"
  - "The retry reuses the strokes mapped for the first typing (identical by construction); the strokes slice is zeroed beside the buffer"
requirements-completed: [UNLK-01]
---

# Phase 23 Plan 04: Remote unlock, daemon side Summary

An own-account viewer can now unlock a locked Mac through the daemon: it sends the password as one binary message on the encrypted input channel, and the daemon maps it through the Mac's current keyboard layout, wakes the screen with a non-printing key, types it, checks a fresh lock read, retypes once if still locked, and reports. A guest is never heard, whatever it is granted. The password exists only in one daemon-owned byte buffer that is zeroed in a panic-safe defer, and a sentinel password sent through the real channel path appears in no log, counter, message or recovery line in any outcome. The viewer panel is 23-07.

## What was built

- **screencap/unlock_darwin.m and .go** (`-framework Carbon`):
  - `pc_unlock_map` decodes strict UTF-8 into a scratch buffer, builds the reverse table from `TISCopyCurrentKeyboardLayoutInputSource` plus `UCKeyTranslate` over keys 0 to 127 with no modifier, Shift, Option and Shift-Option (plainest stroke first, lowest key of equals so the main block beats the keypad), and returns one `{vk, flags}` per character. A character with no key, invalid UTF-8, a control character, Delete, a private-use character or an empty buffer returns `ErrUnmappable` with every stroke zeroed. The scratch buffer is zeroed (`memset_s`) before return.
  - `pc_unlock_caps_lock` reads the alpha-shift flag of `CGEventSourceFlagsState(kCGEventSourceStateHIDSystemState)`.
  - `pc_unlock_wake` declares remote user activity, then presses the wake key (a Shift goes down with the Shift flag set). `pc_unlock_key` is key down, 60 ms, key up, with the stroke's flags. Both post through `pc_input_key`, so they carry the Portty event marker and go to the HID tap. Return is `kVK_Return` through the same call.
  - Go `UnlockBridge` (six methods): `Wake`, `Type`, `Return` refuse with `ErrRefusedInTest` under `testing.Testing()`; `MapRunes`, `CapsLockOn`, `SessionLocked` only read. No error carries a buffer, stroke or character.
- **desktop/unlock.go**:
  - `secretBytes` with String, GoString, Format (every verb), MarshalJSON, LogValue all returning `[redacted]`; it offers only `bytes()` and `zero()`.
  - `handleUnlockBytes`: a guest (no `unlockCap`) is counted `unlock_forbidden` and the slice zeroed; a message shorter than 2, longer than 1025 or without byte 0 equal to 0x01 is counted `malformed`; otherwise the bytes after the type byte are cloned into a `secretBytes`, the inbound slice is zeroed, and `runUnlock` starts. No string is ever built (the acceptance grep for `string(data|buf|b` finds none).
  - The sequence in `attemptUnlock`, with the refusals in the order not locked, paused, rate limited, Accessibility, Caps Lock, unmappable. Then `releaseHeldAll("unlock")`, `Wake(unlockWakeKey)`, 1 s settle, strokes 60 ms apart, Return, poll a fresh `SessionLocked` for 3 s (every 250 ms); still locked gives state `retrying` and one retype of the same strokes, a second Return and another 3 s poll; still locked gives a failed submit.
  - Waits run through `Deps.UnlockSleep` and `Deps.Now`, so the tests run on a fake clock. `Deps.Unlock` defaults to `screencap.NewUnlockBridge()`.
  - Throttle under one leaf mutex: one attempt flag, `lastEnd` (set only when typing began), consecutive failures, `pauseUntil`. A failed submit counts once although its retry means the Mac checked twice (the pause arrives after at most 6 macOS checks); the third failed submit sends `paused` (wait 900000 ms) to every unlock-cap viewer; a success resets the count and sets `RelockArmed`.
  - `UnlockStats{State, Failures, RelockArmed}`; state is idle, unlocking, retrying or paused.
- **manager.go**: `viewer.unlockCap` (set true only in `ServeDesktopViewerAs`, never from rights bits, guest path untouched), `caps unlock 1` on that hello only, `Deps.Unlock` and `Deps.UnlockSleep`, the join message sent after the answer and the first state broadcast.
- **input.go**: the non-string branch hands a message whose byte 0 is 0x01 to `handleUnlockBytes`; any other binary message is zeroed and counted `malformed`.
- **Wire** (matches 23-01's contract): `{type: unlock, state, failures, wait_ms?, reason?, by_portty?}`; states idle, unlocking, retrying, unlocked, failed, rate_limited, paused, refused (reason caps_lock or unmappable), unavailable (reason accessibility), error. `failures` is always present. Not-locked answers `idle` to the submitter. Eleven golden lines: `hello-server-unlock` and the ten `unlock-*` lines (`unlock-idle` is one beyond the plan's list), plus an `unlock` row in the signal-shape table.

## Results the plan asked to be recorded

- **Precondition:** 20-02, 22-04, 22-05 and 23-02 SUMMARYs exist; `grep -c IsString internal/desktop/input.go` is 1. `git log --oneline 1c55b159..HEAD -- internal/desktop/input.go manager.go viewer.go gates.go` at base: 45ee75ad (23-02), 2057d0d9, 0467877b (23-01), 88e098af, 76bc8b51, b570ae27 (22-05), a4fb5d09 (22-04), d47bd7eb (quick-261127-bb7), efcc60d2, 1dc75276, a8c3ceb0 (Phase 20 fixes).
- **Starting `unlockWakeKey`:** `0x38`, `kVK_Shift`. VM row U-01 (23-10) decides the final key; if it changes, `unlock.go` and the test that names it (`TestUnlock_Tracer` checks `0x38`) change together.
- **Layout map on the host:** all 95 printable ASCII characters map and round-trip on the host layout (`TestUnlockBridge_MapRoundTrips`). The native calls ran on a background thread under `go test` with no main run loop and did not crash. The daemon's own thread situation is still to be seen in a VM row (see below).
- **Mutation check of the leak test:** a temporary `slog.Info("...", "pw", string(secret.bytes()))` made five of the seven outcomes of `TestUnlockRedact_NoLeakThroughTheRealChannel` fail; reverted before commit.
- **Test runs:** `go test -race -count=3 -run 'TestUnlock_|TestUnlockRedact_'` passes (12 tests listed); the full `internal/desktop` and `internal/screencap` packages pass with `-race`; `golangci-lint --new-from-rev=1c55b159` reports 0 issues; `tests/check-brand.sh` clean.

## New test files for 23-12 (TESTING.md Sections 2 and 4)

- `internal/desktop/unlock_test.go` (loopback harness with a fake bridge and a fake clock: `TestUnlock_Tracer`, `_GuestNeverUnlocks`, `_RetriesOnceThenFails`, `_RetrySucceeds`, `_RefusalsTypeNothingAndCostNothing`, `_RateLimited`, `_ThreeFailuresPause`, `_SuccessResetsFailures`, `_JoinerHearsCurrentState`)
- `internal/desktop/unlock_redact_test.go` (`TestUnlockRedact_NoLeakThroughTheRealChannel`, `_SecretFormatters`)
- `internal/desktop/unlock_golden_test.go` (internal package: `TestUnlock_GoldenLines`; not in the plan's file list, needed because `readGolden` and `msgUnlock` are internal)
- `internal/desktop/export_unlock_test.go` (test-only exports of the secret type; not a test file in the traceability sense)
- `internal/screencap/unlock_darwin_test.go` (`TestUnlockBridge_MapRoundTrips`, `_RefusesUnmappable`, `_ReadsAreHarmless`, `_RefusesUnderTest`; darwin only, reads the host layout and posts nothing)
- Existing files touched: `internal/desktop/protocol_test.go` (one signal-shape row), `fanout_test.go` (two hello-caps assertions now expect `unlock:1` on the own-account hello).
- Traceability rows (UNLK-01): the four test files above plus `unlock_golden_test.go`, all repo-relative `.go` paths. TESTING.md was not edited, as instructed.
- Manual items for Section 5 (cannot be automated here; none was run on this host): see the next section.

## Not verified here (deferred, by the plan's host-safety rules)

Nothing was posted, typed, locked or woken on this host; every posting call refuses under `go test`. For 23-09 and 23-10 (VM rows, public password only) and 23-14 (the office Mac with the user present):

1. **U-01, the wake key:** that `kVK_Shift` (or the candidate that replaces it) reveals the password field and inserts nothing (loginwindow's "length = 0" log), and that a dark display lights from the activity declaration plus the key.
2. **Modifier flags on typed events at the lock screen:** strokes for upper-case letters and symbols carry Shift or Option in the event flags rather than a separate modifier press; whether loginwindow's password field honors that is unproven. If it does not, `pc_unlock_key` must press the modifier key around the character.
3. **Layout the lock screen uses:** the map reads the session's current input source; the lock screen's source can differ (the R15-B frame shows "ABC"). A different layout there would type a wrong password and count as a failed submit.
4. **Caps Lock read** (`CGEventSourceFlagsState` on the HID state) while the Mac is locked and from the launchd daemon.
5. **TIS from the daemon:** `TISCopyCurrentKeyboardLayoutInputSource` is documented as main-thread-only on some macOS builds; it worked from a background thread under `go test` on this host, but the daemon's thread has not been exercised. If a VM row crashes or returns no layout, dispatch the layout read to the main queue.
6. **Retry timing:** that 3 s after Return is long enough for `CGSSessionScreenIsLocked` to clear on a correct password, and that a retype after a wrong one lands in an emptied, revealed field.
7. **Real throttle behavior:** the pause after 3 failed submits against macOS's own failed-attempt policy (with the retry that is up to 6 checks).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Task 2 was committed before Task 1**
- **Issue:** Task 1's default `Deps.Unlock` is `screencap.NewUnlockBridge()` and `desktop.KeyStroke` is an alias of `screencap.KeyStroke` (a single type is what lets the screencap bridge satisfy the `desktop.UnlockBridge` interface), so Task 1 cannot compile without Task 2's types.
- **Fix:** the native bridge (Task 2) landed first, then Task 1, then Task 3. Each commit builds and passes on its own.
- **Commits:** 1f09ad11 (Task 2), 2e5e0add (Task 1), c74063b0 (Task 3)

**2. [Rule 1 - Bug] Two existing hello tests asserted the old exact own-account caps**
- **Found during:** the full-package run after Task 1 (it was cut from view by `tail`; seen in the Task 3 run)
- **Issue:** `TestFanout_HelloCapsForwardCompatible` and `TestHello_RightsAndInputCap` required exactly `{video, input, audio}`.
- **Fix:** both now require `unlock:1` as a fourth cap (own-account hello only; the guest hello test still passes unchanged).
- **Files modified:** internal/desktop/fanout_test.go
- **Commit:** c74063b0

**3. [Plan wording] The viewer joining a locked Mac is always sent a message**
- The plan says "once to a joining one while locked"; this sends `idle` too (with the failure count) rather than nothing, so a reconnecting panel cannot keep a stale "Unlocking" line. Tests that count unlock messages ignore that first idle one.

**4. [Plan wording] Extra files**
- `unlock_golden_test.go` and `export_unlock_test.go` were added (see above); `unlock-idle` is a golden line the plan did not list.

**5. [Task order] Tests and code were not strictly red then green**
- Task 2's tests were written before the bridge but never run red; Tasks 1 and 3 had the code written first and the tests after. The leak test was proven able to fail with a deliberate leak (see Results).

## Findings (not fixed here)

- **Flaky under load:** `TestQualityShaped_MediumLinkPullsTheTargetDown` (23-02) failed once in a full run while another executor's tests ran in parallel on the host ("lowest encoder target 8000000 ... want it pulled under 4 Mbps") and passed alone and in the next two full runs. It does not touch unlock code. Left alone.
- **A Type error mid-password leaves typed characters in the lock screen's field.** Return is never pressed, so the Mac does not check a password and the submit is not counted, but the next attempt would append to what is there. The field normally clears when the screen idles or sleeps; a keystroke to clear it was not added (no evidence yet it is needed; VM row territory).
- **A failed read of the lock after Return counts as still locked** (fail closed, like `Manager.Locked()`); a Mac whose session cannot be read at all would therefore end every submit as failed and start the pause. Unlikely, and the first read of the sequence (before anything is typed) already refuses with `error`.
- **No audit event is written.** CONTEXT adds an unlock event to Phase 22's audit log; this plan's file list has no audit change, and 23-05 is the plan that reads `audit.go`. The hook point is `unlockEnd` (who: the submitter's identity on `v`, when, success or failure).
- **`RelockArmed` is set on a successful Portty unlock and never cleared here.** 23-05 owns the disarm.
- **`sendUnlock` writes sequentially** (like `broadcast`), so a stalled viewer socket delays the other viewers' messages, and the attempt, by up to 5 s once. Noted as a `shortcut:` in the code.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-23-08 mitigated (`secretBytes` on every formatter, binary intake with no string conversion, defers that run on panic, fixed reason strings, sentinel test over every outcome and a mutation check of it), T-23-09 mitigated (`unlockCap` set only in `ServeDesktopViewerAs`, separate from rights, guest test with control and clipboard granted), T-23-10 mitigated (spacing, pause, shared by every viewer), T-23-11 mitigated (single attempt flag, `TestUnlock_RateLimited/while one runs`), T-23-12 mitigated (Caps Lock, unmappable and Accessibility refusals never counted, tested), T-23-13 mitigated (only a binary 0x01 message on the DTLS channel, 1024-byte cap; the signalling socket never carries a password).

## Self-Check: PASSED

- internal/desktop/unlock.go, unlock_test.go, unlock_redact_test.go, unlock_golden_test.go, export_unlock_test.go: FOUND
- internal/screencap/unlock_darwin.go, unlock_darwin.m, unlock_darwin_test.go: FOUND
- Commits 1f09ad11, 2e5e0add, c74063b0: FOUND in `git log`
