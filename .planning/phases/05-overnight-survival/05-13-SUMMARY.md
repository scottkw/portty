---
phase: 05-overnight-survival
plan: 13
subsystem: frontend
status: complete
gap_closure: true
tags: [wake, network-change, reconnect, relayclient, wails-event, peer-poll, shar-07, nav-02]
requires:
  - phase: 05-09
    provides: "Two-VM rig, axdump, 05-VM-MATRIX.md Feature checks table"
  - phase: 05-10
    provides: "internal/wakewatch: Watch(ctx) <-chan Reason, Woke \"wake\" / NetworkChanged \"network\""
  - phase: 05-12
    provides: "Both VMs on b2e03b22, shared office session vm12, click helper with `type`"
provides:
  - "app.go: App.wakeWatchFunc seam and startWakeWatcher (wakewatch events -> Wails event system:wake, payload wake|network), started in startup"
  - "App.tsx: system:wake re-dispatched as window CustomEvent 'portty:wake' (detail = reason)"
  - "relayClient.ts: RECONNECT.wakeGapMs and wakeMinGapMs (5 s each); reconnect-mode 'portty:wake' listener; timer-gap wake on the 1 s watchdog tick; WebSocket constructor throw handled; ws nullable"
  - "App.tsx peer poll: 5 s while visible, 30 s while hidden, immediate on focus/visible/portty:wake, one fan-out at a time"
  - "appHarness.ts: wailsEvents map of EventsOn callbacks so App tests can fire Go events"
  - "V-13 PASS row in 05-VM-MATRIX.md"
affects: [05-14, 05-15]
tech-stack:
  added: []
  patterns:
    - "Go -> page events that a browser build must also survive go through a window CustomEvent bridge in App.tsx; lib code never imports the Wails runtime"
    - "One debounce (lastWakeAt) shared by every wake source: the Wails event and the watchdog tick gap"
key-files:
  created:
    - app_wake_test.go
    - frontend/src/components/__tests__/App.remote-poll.test.tsx
  modified:
    - app.go
    - frontend/src/App.tsx
    - frontend/src/lib/relayClient.ts
    - frontend/src/lib/relayClient.test.ts
    - frontend/src/components/__tests__/appHarness.ts
    - .planning/phases/05-overnight-survival/05-VM-MATRIX.md
    - .planning/phases/05-overnight-survival/deferred-items.md
    - TESTING.md
key-decisions:
  - "A system wake restarts the backoff by setting attempt = 0 right after calling redial(), whose attempt++ runs synchronously before its first await; the next failure waits the first step"
  - "A system wake redials even in a hidden window (it is ready when shown); the probe kill keeps 05-04's visibility rule"
  - "Peer poll cadence uses document.visibilityState only (not window focus): a visible but unfocused window still polls every 5 s"
  - "Peer poll is a fixed 5 s interval; while hidden a tick polls only when 30 s have passed since the last poll"
requirements-completed: []
coverage:
  - id: D1
    description: "A wake or network change reaches every reconnect-mode RelayClient as portty:wake, which redials at once (reconnecting) or probes (open), with the backoff restarted and one action per 5 s"
    requirement: SHAR-07
    verification:
      - kind: unit
        ref: "app_wake_test.go#TestStartWakeWatcher_EmitsSystemWake"
        status: pass
      - kind: unit
        ref: "frontend/src/lib/relayClient.test.ts#system wake ('portty:wake')"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/App.remote-poll.test.tsx#the Go system:wake event is re-dispatched on window as portty:wake with its reason"
        status: pass
      - kind: automated_ui
        ref: ".planning/phases/05-overnight-survival/05-VM-MATRIX.md#V-13"
        status: pass
    human_judgment: false
  - id: D2
    description: "Browser views treat a watchdog tick more than 5 s late as a wake, debounced across sources"
    requirement: SHAR-07
    verification:
      - kind: unit
        ref: "frontend/src/lib/relayClient.test.ts#timer-gap wake (browser views)"
        status: pass
    human_judgment: false
  - id: D3
    description: "A throwing WebSocket constructor never kills the loop or the page, and the log carries no URL"
    requirement: SHAR-07
    verification:
      - kind: unit
        ref: "frontend/src/lib/relayClient.test.ts#WebSocket constructor throws (malformed URL)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Peer poll: 5 s visible, 30 s hidden, immediate on focus/visible/wake, no overlapping fan-outs (folded-in todo)"
    requirement: NAV-02
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/App.remote-poll.test.tsx"
        status: pass
    human_judgment: false
duration: 39min
completed: 2026-11-18
estimate:
  tokens: 70000
  tasks: 3
actuals:
  tokens: 12400
  tasks: 4
  commits: 7
plan_head_before: 680571c8d7258c57c8e3cb599f7d4d702172bc20
plan_head_after: f1b95d7d19ab76cd371c3ac42a5187149044963d
---

# Phase 5 Plan 13: Wake and Network-Change Reconnect for GUI and Browser Views Summary

**The 05-10 wake/network detector now runs inside the GUI app and reaches every guest view as a Wails `system:wake` event, which App.tsx re-dispatches as a window `portty:wake` event. RelayClient handles it: if a backoff wait is pending, it redials at once and restarts the schedule; if the socket is open, it sends a probe. Browser views get the same handling when a watchdog tick arrives more than 5 s late. A WebSocket constructor that throws no longer kills the loop. Folded in: the sidebar's peer list now refreshes every 5 s while the window is visible and at once on focus or wake, instead of every 30 s.**

## Performance

- **Duration:** about 39 min (05:23Z to 06:02Z)
- **Tasks:** 4 (3 planned plus 1 folded in)
- **Files modified:** 10 (2 created)

## What was built

- **app.go:**
  - The `wakeWatchFunc` seam (nil means `wakewatch.Watch`).
  - `startWakeWatcher(ctx)` calls the detector, then a goroutine ranges over the channel and calls `emitEvent("system:wake", string(reason))`.
  - Started at the end of `startup`. The detector closes its channel when the app context ends, which ends the goroutine.
- **App.tsx bridge:** `EventsOn('system:wake', reason => window.dispatchEvent(new CustomEvent('portty:wake', { detail: reason })))`, with `offWake()` in the mount effect's cleanup.
- **relayClient.ts:**
  - `RECONNECT.wakeGapMs` and `wakeMinGapMs` (5 s each).
  - In reconnect mode, a window `portty:wake` listener is added next to `online` and removed in `teardown`.
  - `onSystemWake` runs at most once per `wakeMinGapMs`:
    - reconnecting with a pending timer: clear the timer, redial, attempt = 0
    - reconnecting with a redial in flight: attempt = 0
    - open: attempt = 0 and `sendProbe()`. The existing `checkLiveness` abandons the socket after 5 s if a pong was seen and the page is visible.
  - `checkLiveness` now first measures the time since the previous tick. A gap over `wakeGapMs` calls `onSystemWake` (same debounce). The tick runs in every reconnect-mode state.
  - `dial` wraps `new WebSocket(url)`. On a throw it logs the fixed `[RelayClient] could not open a WebSocket` (no URL, no error object). In reconnect mode it then calls `scheduleRetry`; otherwise it fires `onClose(1006, '')` from a 0 ms timer.
  - `ws` is nullable, so `sendInput`, `sendResize`, `close` and the ping interval are all safe with no socket.
  - The existing `onWake` for `online`/`visibilitychange` is unchanged.
- **Peer poll (folded in):** see "Folded-in scope".

## Task Commits

1. **Task 1 (tracer): system:wake end to end**
   - `67149bc5` test (RED; app.go got the seam field and an empty `startWakeWatcher` so the Go test compiles)
   - `17021753` feat (GREEN)
   - Tracer gate (end-of-phase mode, automated verify): verify re-run after the commit passed (`TRACER-VERIFIED`).
2. **Task 2: timer gap, debounce across sources, constructor throw:** `9fd0a2fd` test (RED), `8af5c8b0` fix (GREEN)
3. **Task 4 (folded in): peer poll cadence:** `22abc0c4` test (RED), `2af365b6` feat (GREEN)
4. **Task 3: VM check:** `f1b95d7d` test (V-13 row, TESTING.md, deferred-items)

Task 4 ran before Task 3, so that one VM build covered both the wake handling and the poll change. Its commits are still separate from the plan's tasks.

## Folded-in scope

**Todo `2026-11-18-faster-remote-session-discovery`** (the user asked for it to be folded in). The todo is now `.planning/todos/completed/` with a `## Resolution` section.

- **Before:** App.tsx polled `GetRemoteSessionsWithMeta` every 30 s, so a new share took up to 30 s to show up on another Mac.
- **Now:**
  - A 5 s interval. While `document.visibilityState === 'visible'` every tick polls. While hidden, a tick polls only when 30 s have passed since the last poll.
  - `focus`, `visibilitychange` to visible and `portty:wake` refresh at once.
  - An `inFlight` flag makes any trigger skip while a fan-out is running. No queueing and no overlap.
  - All listeners are removed in cleanup. The `tailscaleOff` and web-mode early returns are unchanged.
- **Tests:** `App.remote-poll.test.tsx` (fake timers) covers the 5 s visible cadence, the 30 s hidden cadence, an immediate refresh on each trigger, no overlap while a poll is pending, and the system:wake bridge. The App.tailscale source pin still finds `30_000`, so it was not changed.
- **VM:** a session created and shared in the office VM appeared in the home sidebar 3 s after `portty share` returned. That time includes the axdump round trip. The test session was then killed.

## VM evidence (Task 3, V-13)

The rig: build `2af365b6`, deployed to the home VM only. The office VM kept `b2e03b22`, since the change is viewer-side and keeping it also kept the shared session `vm12` alive. The home GUI tab was on `vm12`. Input was typed with `click2 type` and checked as lines appended to `/tmp/v13.txt` in the office VM. The two daemon logs give the timings.

1. **tart suspend did not work for this VM.** `tart suspend portty-test-2` saved its state. Both restores then failed with `VZErrorDomain Code=12 ... "invalid argument"`, with and without `--suspendable`, because the VM was not started suspendable. I moved the 1.7 GB saved state aside, which is reversible: `scratchpad/p12/portty-test-2.state.vzvmsave.unrestorable`. The VM then cold-booted. Its disk was untouched, it auto-logged in, and tailscaled came back. Portty was relaunched.
2. **Sleep (the plan's fallback):**
   - SIGSTOP at 05:48:16 of the home Portty GUI, its daemon, the three WebKit processes (WebContent, Networking, GPU) and tailscaled. This models the whole Mac asleep.
   - The office dropped the viewer at its heartbeat (05:48:43).
   - SIGCONT at 05:51:28. The home daemon logged `proxy heartbeat timeout` at 05:51:28.13. The office logged `join conn=8` and `replay mode=delta bytes=0 resume=true` at 05:51:29.54, then `first input` 57 ms later.
   - **Recovery took about 1.5 s, and nothing was duplicated** (zero-byte delta).
   - The first line typed during the reconnect lost its leading characters, which is the expected drop-not-queue behavior. Only its `>> /tmp/v13.txt` survived, so zsh ran `cat` (NULLCMD), and the next 19 typed lines went into the file as text, once each. After Ctrl-D, `echo AFTER-WAKE` ran once.
   - **Attribution:** the GUI tab's socket is loopback to the home daemon. The daemon's proxy and its wall-clock heartbeat closed the guest socket right at resume. So this run shows the recovery time, but it does not single out `system:wake`.
3. **Network change during a long backoff. This run proves the wake path end to end.**
   - pf in the home VM blocked outbound traffic to the office tailnet IP (`block return-icmp(host-unr)`).
   - The proxy dropped the peer after its 45 s watchdog. Redials then failed (`peers/connect: portty-vm-office unreachable`). The 5th failure came at 05:59:39.71, so the next wait was the 30 s cap (24 to 36 s).
   - At 05:59:40.87, pf was turned off and an en0 alias was added (a routable address change, so the detector reports NetworkChanged).
   - The office logged `join conn=10` at **05:59:42.13, 1.3 s later**, with `mode=delta bytes=0 resume=true`. `echo AFTER-NET` ran once.
   - Without the wake handling, that redial would have come at least 24 s later.
   - pf rules were flushed and the alias removed afterwards.
4. **A first attempt with `block return` (a locally made RST) misfired.** The tab ended as "portty-vm-office stopped sharing this session": the daemon maps ECONNREFUSED to `not_sharing`, which is final. That was an artifact of the test method, since a real tailnet refusal comes from the host itself. It is logged in deferred-items. The tab was closed and reopened before run 3.

Screenshots: WebKit still captures blank in these VMs, so the evidence is the AX tree (tab, panel and banner text) plus the two daemon logs. No share link or cap was printed or stored. The host Mac's network and power were not touched, and neither was the user's `shell 1` session.

## Verification

- Task 1 verify:
  - `go vet .` passed.
  - `--- PASS: TestStartWakeWatcher_EmitsSystemWake` (with -race).
  - tsc passed.
  - relayClient.test.ts 66/66 at that point.
  - `portty:wake` appears in App.tsx.
- Task 2 verify:
  - tsc passed.
  - The three-file run gave `Test Files 3 passed`.
  - Full vitest `--maxWorkers=4`: 85 files, 1154 tests passed.
  - Full Go `-race -short` suite (with the two skips): all packages ok.
- After Task 4: full vitest 86 files, 1159 tests passed.
- Task 3 verify: relayClient.test.ts passed, exactly one `| V-13 | PASS |` row, no `cap=` token in 05-VM-MATRIX.md.
- `tests/check-traceability-paths.sh`: OK (123 paths).
- Mutation checks (each applied alone, then reverted):

  | Mutation | Test that failed |
  |----------|------------------|
  | No `attempt = 0` after a wake redial | the backoff-restart case and the debounce case |
  | No debounce | "two events 100 ms apart cause one redial" |
  | `portty:wake` listener not removed | "close() removes the listener" |
  | No `inFlight` guard in the poll | "never starts a second fan-out" |
  | Poll does not listen for `portty:wake` | the immediate-refresh case |

## TDD Gate Compliance

| Task | RED commit | GREEN commit | RED cause (RED_EVIDENCE_OK, semantic check done) |
|------|-----------|--------------|-----------------------------------------------|
| 1 | 67149bc5 | 17021753 | Go: `TestStartWakeWatcher_EmitsSystemWake`: `system:wake payloads = []; want [wake network]` (it compiled against the empty scaffold). vitest target `...system wake ('portty:wake') > while reconnecting with a pending backoff...`: `expected 2 to be 3`. The hidden, probe and debounce cases also failed on assertions. The host-path and close() guards passed at RED, as expected with no listener, and became meaningful after GREEN (the mutation check above) |
| 2 | 9fd0a2fd | 8af5c8b0 | Target `...WebSocket constructor throws > on a redial...`: `redial at 2000 ms: expected 1 to be 2`, plus an unhandled rejection from the throwing redial (the bug itself). The gap target failed with `expected 2 to be 3`. The first-dial and no-reconnect cases threw out of the constructor, and the log case threw before logging |
| 4 (folded) | 22abc0c4 | 2af365b6 | Target `...polls every 5 s while the window is visible`: called 1 time, expected 2 (30 s interval). The immediate-refresh case failed the same way. The no-overlap case failed only on its final cadence check, and its overlap assertion was pinned by the mutation check. The bridge case passed at RED because Task 1 had already shipped the bridge (it is coverage for Task 1) |

RED records were classified with `gsd-tools check tdd-red-evidence`. vitest records used `--reporter=tap-flat` with stdout only, and the target was run alone with `-t`, as in 05-12.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] tart suspend could not resume the home VM**
- **Found during:** Task 3
- **Issue:** the plan's first method failed. The suspend succeeded, but both restores failed (VZ Code=12) because the VM was not started with `--suspendable`. The VM was stuck in `suspended`.
- **Fix:** I moved the saved state into the scratchpad (reversible) and cold-booted. Then I used the plan's fallback (SIGSTOP/SIGCONT), widened to the WebKit processes and tailscaled: WebKit runs JS and the socket outside the app process, so stopping only the app would leave the socket alive.
- **Files modified:** none in the repo

**2. [Discretion] An extra network-change run**
- **Why:** the sleep run's fast recovery came from the daemon proxy's heartbeat, so it could not show that `system:wake` works. The pf + alias run does show it: a 1.3 s redial in the middle of a 30 s backoff.

**3. [Standing rule] TESTING.md updated**
- vitest 86, total 213. The Go row is left for 05-15, as in 05-12; `git ls-files` now shows 122.
- Section 4 has new SHAR-07 rows (app_wake_test.go, App.remote-poll.test.tsx) and a NAV-02 row, and the relayClient row is extended.
- The Category AT caveat "no system wake hook yet" was replaced with what 05-10/05-13 now do.

**4. [Test harness] appHarness keeps EventsOn callbacks**
- The test runtime stub (`src/wailsjs/runtime/runtime.js`) calls `window.runtime.EventsOn`. The harness now records each callback in an exported `wailsEvents` map, so App tests can fire Go events. Other App suites are unaffected (9 files, 199 tests passed).

**5. [Typing] App.remote-poll.test.tsx mock typed for tsc in the GREEN commit**
- `vi.fn` needed an explicit signature to fit the harness's `Impl` type. This is a test-only change, included in `2af365b6`.

**Total deviations:** 1 blocking fix outside the repo, plus 4 minor items. **Impact:** none on the planned interfaces.

## Known Stubs

None.

## Threat Flags

None. No new endpoint, auth path or file access.
- T-05-36: the forged-event surface is as accepted.
- T-05-37: pinned by the debounce tests across both sources.
- T-05-38: pinned by "logs a fixed message without the URL" (no `ws://`, `wss://` or `cap=` in any console argument).

## Notes for later plans

- **05-15 (TESTING.md):**
  - New Go file: `app_wake_test.go` (TestStartWakeWatcher_EmitsSystemWake). `git ls-files '*_test.go'` = 122.
  - New vitest file: `frontend/src/components/__tests__/App.remote-poll.test.tsx` (5 tests).
  - 11 new cases in relayClient.test.ts.
- **Daemon proxy watchdog:** a GUI tab's link to the peer is the daemon proxy, and system:wake does not reach it. A black-holed peer with no wake or address change still takes about 45 s, the proxy's watchdog, to notice. Recorded in deferred-items as a possible follow-up: feed wakewatch to the proxy.
- **VM state left:**
  - The home VM was cold-booted and runs build 2af365b6. The office VM still runs b2e03b22, and `vm12` is still shared.
  - The unrestorable saved state is at `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/p12/portty-test-2.state.vzvmsave.unrestorable` (1.7 GB). It is safe to delete.
  - The home VM was started with `nohup tart run --no-graphics portty-test-2`.

## Self-Check: PASSED

- FOUND: app.go, app_wake_test.go, frontend/src/App.tsx, frontend/src/lib/relayClient.ts, frontend/src/lib/relayClient.test.ts, frontend/src/components/__tests__/App.remote-poll.test.tsx, frontend/src/components/__tests__/appHarness.ts, .planning/phases/05-overnight-survival/05-VM-MATRIX.md, TESTING.md
- FOUND commits: 67149bc5, 17021753, 9fd0a2fd, 8af5c8b0, 22abc0c4, 2af365b6, f1b95d7d
- FOUND: `| V-13 | PASS |` once in 05-VM-MATRIX.md; todo in .planning/todos/completed/
