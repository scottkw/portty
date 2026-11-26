---
status: resolved
trigger: "Terminal sessions stopped producing visible output for ~17 minutes in Phase 18 VM row RC-WR04-stall (keep-daemon rollback: 1.8.0 GUI on a kept 1.8.1 daemon; both sessions, same pids, gapless ticks, resumed when a CLI viewer joined)"
created: 2026-11-26T15:30:00Z
updated: 2026-11-26T16:00:00Z
---

## Resolution

root_cause: "No stall happened. The portty-test guest's wall clock stepped forward 17 min 03 s during the row: at guest time 07:10:02 timed got 'Received time Thu Nov 26 07:27:05 2026 from NTP' and called settimeofday ('=== system wallclock time adjusted' at 07:27:05). The guest clock had fallen behind its RTC some time after the previous in-step sync at 06:47:07 (the VM was not running for ~17 min while the RTC kept going). The row's '17 min stall' came from tick arithmetic that assumed the wall clock was continuous."
fix: "None in code (no Portty defect). Recorded the evidence (evidence/RC-WR04-clockstep.log) and annotated 18-VERIFICATION.md observation 1. 18-VM-CHECKS.md was left as recorded so the verification report's covered digest stays valid."
verification: "Guest unified log: 46k-52k lines/min at 07:05-07:08, 423 at 07:10, zero for 07:11-07:26, 15.6k at 07:27. Tick arithmetic with the step counted: tickA ran at 0.985 ticks/s (TICK 281 at 07:29:18, TICK 950 at 07:40:37), so TICK 0 = 07:24:33 post-step = 07:07:30 pre-step (row says about 07:07:35); the 07:28:19 replay is predicted to end near TICK 223 and ended at TICK 224. The ticker therefore ran without any pause in guest time."
files_changed: []
regression_test: "None: there is no product behaviour to pin. Rig guard recommended: any VM row that infers elapsed time from guest wall-clock timestamps should first check `/usr/bin/log show --predicate 'process == \"timed\" AND eventMessage CONTAINS \"Received time\"'` for a step inside the row window."

## Symptoms

expected: tickers (1 tick/s) keep producing output with only the 1.8.0 GUI attached to the kept 1.8.1 daemon
actual: first CLI viewers at 07:28:19/07:28:25 replayed only up to TICK 228 / VTICK 234, about 1240 s after the tickers started at about 07:07:35
reproduction: not reproducible as a product behaviour; it was a one-off guest clock step
started: RC-WR04 row, 2026-11-26

## Eliminated

- hypothesis: a GUI viewer that stops reading backpressures the PTY reader (relay flow control)
  evidence: Hub.broadcast/sendLocked never block (full channel or replay-pending overflow -> kickSlow in a goroutine); and the joining viewer at 07:28:19 got its replay at once, so emuMu/hub.mu were free and Run was not wedged on a lock. More decisively, the ticker never stopped in guest time (tick arithmetic above).
- hypothesis: PTY kernel buffer full because the hub stopped reading (Run exited on a freeze or blocked in Read)
  evidence: a not-read PTY lets the writer run on until ~2 KB (hiwat) are queued, which would have shown up as a burst of ~200 ticks at the join; the replay ended at TICK 224 and live ticks continued at 1/s. Also moot given no stall in guest time.
- hypothesis: guest system sleep, display sleep or App Nap froze the processes
  evidence: pmset 'sleep 0 (sleep prevented by powerd)', no Sleep/Wake events in pmset -g log; the gap is a clock step, not a process freeze.
- hypothesis: keep-daemon mode (older GUI, newer daemon from an unlinked bundle) causes a session hold
  evidence: nothing to explain; the session ran continuously.

## Evidence

- timestamp: 2026-11-26T15:40Z
  checked: guest unified log 07:05-07:32 line counts per minute (portty-test)
  found: zero lines for 07:11-07:26; last line before the gap 07:10:02.978, next 07:27:05.265 '=== system wallclock time adjusted'
  implication: the guest clock jumped; the minutes 07:11-07:26 never existed in guest time
- timestamp: 2026-11-26T15:40Z
  checked: timed 'Received time' lines 05:00-07:10
  found: in step at 05:14, 05:34, 05:58, 06:22, 06:47; at guest 07:10:02 NTP said 07:27:05
  implication: the guest lost ~17 min between 06:47:07 and 07:10:02 and was stepped forward during the row
- timestamp: 2026-11-26T15:45Z
  checked: RC-WR04.log tick counts against the step
  found: steady 0.985 ticks/s run fits every data point once 17:03 is removed
  implication: no process (shell, vim, daemon) was ever held

## Soak test

Not run. The symptom it was meant to reproduce did not occur, so a GUI-only soak on the same-version and keep-daemon pairs would test for a hold that the evidence rules out. It can still be scheduled as general reliability coverage; it is not needed to close this item.
