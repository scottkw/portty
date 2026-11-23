---
phase: 15-v1-2-hardening
plan: 08
subsystem: daemon handoff / fdpass
tags: [go, handoff, fdpass, LOCAL_PEERPID, SCM_RIGHTS, hardening]
status: complete

requires:
  - phase: 15-03
    provides: socket mode hardening in the same handoff.go and handoff_test.go
provides:
  - "takeoverPeerPID seam and a HELLO-OK pid check against the kernel's LOCAL_PEERPID (takeoverRefused on mismatch or lookup error)"
  - "fdpass.rightsFromOOB: Recv frees descriptors from messages before a malformed control message"
affects: [15-09 TESTING.md traceability and M-155]

actuals:
  tokens: 1600
  tasks: 2
  commits: 2
plan_head_before: 205a452c8155fce10fce95d82cdc2485da149dce
plan_head_after: d610b7c86e15d9c6c91c7fda80cf4be85c1e2dc5
commits: 2

tech-stack:
  added: []
  patterns:
    - "Trust the kernel's peer identity over a pid written in a protocol frame; a failed lookup refuses, never falls back"
    - "Walk SCM_RIGHTS control messages with ParseOneSocketControlMessage so a parse error still returns what was collected"

key-files:
  created: []
  modified:
    - internal/daemon/handoff.go
    - internal/daemon/handoff_test.go
    - internal/fdpass/fdpass.go
    - internal/fdpass/fdpass_test.go

key-decisions:
  - "The takeover's pid check uses its own seam takeoverPeerPID, never peerPID (the old side's lookup, replaced by the scene), so in-process tests see the real lookup equal os.Getpid()."
  - "A LOCAL_PEERPID lookup error refuses the takeover (ABORT, old daemon keeps serving); it never trusts HELLO-OK."
  - "t.oldPID is set from the kernel's value after the check, not from HELLO-OK."
  - "rightsFromOOB never closes descriptors; Recv's existing close loop frees each exactly once."

requirements-completed: []
---

# Phase 15 Plan 08: Kernel-confirmed handoff pid and fdpass descriptor cleanup Summary

The takeover now refuses unless the old daemon's pid in HELLO-OK equals the kernel's LOCAL_PEERPID for the handoff connection, and `fdpass.Recv` closes descriptors that arrived before a malformed control message.

Duration: about 15 min. Tasks: 2. Files: 4.

## What changed

- `internal/daemon/handoff.go`: new seam `takeoverPeerPID = fdpass.PeerPID`; in `runTakeover`, right after HELLO-OK is read and before `after-hello`, a lookup error or a pid differing from `ok.PID` returns `t.fail(takeoverRefused, ...)` (ABORT sent, everything closed, old daemon rolls back). `t.oldPID` now comes from the kernel value. `takeoverRefused` comment extended.
- `internal/daemon/handoff_test.go`: `TestHandoff_RefusesPIDMismatch` (rows `mismatch` and `lookup error`): outcome `takeoverRefused`, no FD leak (`checkNewHandoffFDs`), and `checkOldServes` proves every session lists as running and the viewer keeps ticking with no gap. `installSeams` now saves and restores `takeoverPeerPID` with the other seams.
- `internal/fdpass/fdpass.go`: `rightsFromOOB` walks the control buffer with `unix.ParseOneSocketControlMessage` while at least `unix.CmsgLen(0)` bytes remain; `Recv` calls it in place of `ParseSocketControlMessage` and keeps its MSG_CTRUNC, parse-error and count-mismatch switch and close loop untouched.
- `internal/fdpass/fdpass_test.go`: `TestRightsFromOOB` over one message with two fds, two messages, a malformed block after a good message (error plus both fds), and an empty buffer.

## Verification

Logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.R2OpyyAnPJ` (task 1) and `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.YlsO2cVP6I` (task 2; also holds the full-package runs). Task 1 RED evidence is in `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.IFgtDyktmf/t1-red.log`.

- Task 1: `go vet ./internal/daemon/` clean; `go test -race -count=1 -run TestHandoff_ -v ./internal/daemon/` passes, including `TestHandoff_RefusesPIDMismatch` (both rows) and `TestHandoff_HappyPath`. With the check disabled (mutation) both rows fail, so the test is load-bearing.
- Task 2: `go vet ./internal/fdpass/ ./internal/daemon/` clean; `TestRightsFromOOB`, `TestRecv_TruncatedClosesFds`, `TestRecv_CountMismatch` pass under `-race`. Before the implementation `TestRightsFromOOB` failed to build (`undefined: rightsFromOOB`).
- Full packages once, `-race -short -count=1`: `internal/fdpass/...` ok, `internal/daemon/...` ok (96.5s), with 15-03's O-2, this plan's O-6 and O-1 all in.
- Acceptance greps: `takeoverPeerPID` 2 occurrences, `t.oldPID = ok.PID` 0, `func TestHandoff_RefusesPIDMismatch` 1, `func rightsFromOOB(` 1, `ParseOneSocketControlMessage` 1, `ParseSocketControlMessage(` 0, `func TestRightsFromOOB` 1.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] First draft of the refusal test asserted `old:watch-peer` never ran**
- **Found during:** Task 1
- **Issue:** the old side starts watching the takeover's exit right after HELLO-OK, before the takeover's ABORT, so the event legitimately appears on a refusal after HELLO-OK. The assertion (and an unlocked read of `s.events` in its failure message) failed under `-race`.
- **Fix:** replaced it with a check that the takeover returned the refusal; the old-daemon-serves proof stays in `checkOldServes`.
- **Files modified:** internal/daemon/handoff_test.go
- **Commit:** e08d1b90

**2. [Plan wording] Seam restore through `installSeams` instead of a per-row `t.Cleanup`**
- The plan said to restore `takeoverPeerPID` in `t.Cleanup`; adding it to `installSeams`' save/restore tuple does the same for every test that replaces it and matches how `peerUID` and `peerPID` are restored. No behavior difference.

`gofmt` realigned the seam `var` block in handoff.go (whitespace only) because of the new commented entry.

**Total deviations:** 1 auto-fixed (bug in my own first test draft), 1 wording-level. **Impact:** none on scope or behavior.

TDD note: the plan commits once per task, so RED and GREEN are not separate commits. Task 1's RED is the mutation run above (check disabled, both rows fail); Task 2's RED is the build failure before `rightsFromOOB` existed.

## Pending real-process check

TESTING.md M-155 (TestHandoffIT `upgrade` and `fault`, plus a pre-phase daemon upgraded twice) runs in the Tart VM `portty-test`; 15-09 records it and `/gsd-verify-work` runs it before ship. Nothing here started a real daemon, and nothing contacted the host's tailscaled or a 100.x address.

## Known Stubs

None.

## Threat Flags

None. T-15-07 (fd leak on a malformed control message) and T-15-08 (spoofed HELLO-OK pid) are mitigated as planned.

## Self-Check: PASSED

- internal/daemon/handoff.go, internal/daemon/handoff_test.go, internal/fdpass/fdpass.go, internal/fdpass/fdpass_test.go present.
- Commits e08d1b90 and d610b7c8 are ancestors of HEAD; measured `git rev-list --count 205a452c..HEAD` = 2.
- No unrelated files staged; `.gitignore`, `.planning/state.json`, `.ignore`, `agenthub/`, `.planning/phases/15-v1-2-hardening/.gitkeep` untouched.
