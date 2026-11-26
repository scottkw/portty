---
phase: 19
review: 19-REVIEW.md
titles: json
findings:
  - id: CR-01
    severity: critical
    disposition: open
    title: "Regenerating the signing key does not revoke a viewer that is mid-handshake (revocation bypass)"
  - id: WR-01
    severity: warning
    disposition: open
    title: "`Manager.mu` is held across blocking native capture starts (up to about 35 s). This stalls the state loop and Close, and can falsely trip the crash-loop breaker"
  - id: WR-02
    severity: warning
    disposition: open
    title: "RTP timestamps lag one access unit behind capture time, and the tick count can overflow after long idle gaps"
  - id: WR-03
    severity: warning
    disposition: open
    title: "`Manager.RefreshPermissions` is never called, so the Needs permission state stays stale for up to 5 minutes after the user grants Screen Recording"
  - id: WR-04
    severity: warning
    disposition: open
    title: "launchd job liveness depends on one regex. A parse miss lets `bootout` run against a live daemon that holds every terminal session"
  - id: WR-05
    severity: warning
    disposition: open
    title: "The offer-wait loop reads without a deadline once the offer deadline has passed"
  - id: WR-06
    severity: warning
    disposition: open
    title: "`fanOut` does not handle the Errors channel being closed, so it logs spurious `capture error <nil>` lines on every stop"
  - id: WR-07
    severity: warning
    disposition: open
    title: "The GUI's lapse-claim guard `lapseTried` is never reset when the daemon's `LapseSeq` restarts, so later lapses are not claimed by the GUI"
  - id: IN-01
    severity: info
    disposition: open
    title: "Doc comment promises a retry that is not implemented (viewer power holds)"
  - id: IN-02
    severity: info
    disposition: open
    title: "`LoadConfig` does not reject trailing data, unlike `readRuntimeState` and `decodeProbe`"
  - id: IN-03
    severity: info
    disposition: open
    title: "The debug hook ships in the release viewer"
  - id: IN-04
    severity: info
    disposition: open
    title: "Production code branches on `testing.Testing()`"
  - id: IN-05
    severity: info
    disposition: open
    title: "A persistently failing encoder produces one error log line per captured frame"
open: 13
total: 13
recorded: 2026-11-26T22:31:54.932Z
---

# Phase 19: Code Review Disposition

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| CR-01 | critical | open | - |
| WR-01 | warning | open | - |
| WR-02 | warning | open | - |
| WR-03 | warning | open | - |
| WR-04 | warning | open | - |
| WR-05 | warning | open | - |
| WR-06 | warning | open | - |
| WR-07 | warning | open | - |
| IN-01 | info | open | - |
| IN-02 | info | open | - |
| IN-03 | info | open | - |
| IN-04 | info | open | - |
| IN-05 | info | open | - |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.
