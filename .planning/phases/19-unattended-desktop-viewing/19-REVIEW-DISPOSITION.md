---
phase: 19
review: 19-REVIEW.md
titles: json
findings:
  - id: CR-01
    severity: critical
    disposition: fixed
    title: "Regenerating the signing key does not revoke a viewer that is mid-handshake (revocation bypass)"
  - id: WR-01
    severity: warning
    disposition: fixed
    title: "`Manager.mu` is held across blocking native capture starts (up to about 35 s). This stalls the state loop and Close, and can falsely trip the crash-loop breaker"
  - id: WR-02
    severity: warning
    disposition: fixed
    title: "RTP timestamps lag one access unit behind capture time, and the tick count can overflow after long idle gaps"
  - id: WR-03
    severity: warning
    disposition: fixed
    title: "`Manager.RefreshPermissions` is never called, so the Needs permission state stays stale for up to 5 minutes after the user grants Screen Recording"
  - id: WR-04
    severity: warning
    disposition: fixed
    title: "launchd job liveness depends on one regex. A parse miss lets `bootout` run against a live daemon that holds every terminal session"
  - id: WR-05
    severity: warning
    disposition: fixed
    title: "The offer-wait loop reads without a deadline once the offer deadline has passed"
  - id: WR-06
    severity: warning
    disposition: fixed
    title: "`fanOut` does not handle the Errors channel being closed, so it logs spurious `capture error <nil>` lines on every stop"
  - id: WR-07
    severity: warning
    disposition: fixed
    title: "The GUI's lapse-claim guard `lapseTried` is never reset when the daemon's `LapseSeq` restarts, so later lapses are not claimed by the GUI"
  - id: IN-01
    severity: info
    disposition: fixed
    title: "Doc comment promises a retry that is not implemented (viewer power holds)"
  - id: IN-02
    severity: info
    disposition: fixed
    title: "`LoadConfig` does not reject trailing data, unlike `readRuntimeState` and `decodeProbe`"
  - id: IN-03
    severity: info
    disposition: deferred
    title: "The debug hook ships in the release viewer"
  - id: IN-04
    severity: info
    disposition: deferred
    title: "Production code branches on `testing.Testing()`"
  - id: IN-05
    severity: info
    disposition: deferred
    title: "A persistently failing encoder produces one error log line per captured frame"
open: 0
total: 13
recorded: 2026-11-26T22:31:54.932Z
---

# Phase 19: Code Review Disposition

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| CR-01 | critical | fixed | 779d5924: admit re-check at join, in the same vmu hold as the registration; refused with 4401 |
| WR-01 | warning | fixed | a0279fd3: start outside mu, Close clears captureActive first. Residual: the first viewer's own answer still waits for its start |
| WR-02 | warning | fixed | 5a0ec826 |
| WR-03 | warning | fixed | 36d55f9a |
| WR-04 | warning | fixed | 202b8425: an unparsed pid counts as running (db033df0: CLI test fake prints launchd's real idle output). Residual: the print-to-bootout race window is not addressed |
| WR-05 | warning | fixed | df7164c3 (no automated test: the deadline is wall-clock) |
| WR-06 | warning | fixed | babc963f |
| WR-07 | warning | fixed | e3377af2 |
| IN-01 | info | fixed | cf9c60ab |
| IN-02 | info | fixed | ed9848f4 |
| IN-03 | info | deferred | The debug fragment and window.__desktopStats are read by TestDesktopCSP_E2E and the Phase 19 VM rig on release builds, so a DEV-only gate would break them; it holds counts only. The listener leak is one per start(), which runs once per page |
| IN-04 | info | deferred | Optional per the review; moving the testing.Testing() rails into TestMain seams is a cross-package refactor, not a trivial fix |
| IN-05 | info | deferred | Rate-limiting the fan-out log or restarting after N encode failures is new behaviour, not a trivial fix; the stall watchdog bounds the burst to seconds |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.
