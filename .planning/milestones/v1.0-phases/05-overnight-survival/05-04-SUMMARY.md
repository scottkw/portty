---
phase: 05-overnight-survival
plan: 04
subsystem: frontend
status: complete
tags: [frontend, websocket, reconnect, resume, liveness, shar-07]
requires:
  - "05-01: ?resume= on the WS endpoints, Meta{replay} / Meta{offset} replay bracket, frame-byte offsets"
  - "05-02: MsgPong 0x04 answers to MsgPing"
provides:
  - "RelayClient opts.reconnect { beforeReconnect }: opt-in reconnect mode (close classification, jittered backoff, resume redial, watchdog, wake events)"
  - "RECONNECT constants, backoffDelayMs(attempt, rand), RelayState, ReconnectVerdict"
  - "RelayClientCallbacks.onStateChange / onReset"
  - "MSG_PONG (0x04), MSG_META (0x20); ServerFrame pong / meta"
  - "isTerminalClose(code, reason) in closeReasons.ts"
  - "SessionEndedBanner kind 'reconnecting' ('Connection lost — reconnecting…', no dismiss); onDismiss now optional"
  - "TerminalPanel beforeReconnect prop (for 05-05), guest stdin gating, 3 s delayed banner, term.reset on full replay"
affects: [05-05, 05-07]
tech-stack:
  added: []
  patterns:
    - "Socket generation counter: every handler checks its generation, dial and abandon bump it"
    - "Watchdog armed by the first MsgPong; silence earns a probe, only an unanswered probe kills"
    - "Offset trusted only after the current socket's offset meta; then + payload.length + 1 per output frame"
key-files:
  created:
    - frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx
    - .planning/phases/05-overnight-survival/deferred-items.md
  modified:
    - frontend/src/lib/relayClient.ts
    - frontend/src/lib/closeReasons.ts
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/SessionEndedBanner.tsx
    - frontend/src/lib/relayClient.test.ts
    - frontend/src/lib/__tests__/closeReasons.test.ts
    - frontend/src/components/__tests__/SessionEndedBanner.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.hot-swap.test.tsx
decisions:
  - "Watchdog death and a wake event dial at once (no backoff for that attempt); ordinary closes back off. Matches the must-have 'a new one dialed immediately'"
  - "Stale-socket detection uses a generation counter, not socket identity: during the async beforeReconnect after an abandonment this.ws still points at the dead socket"
  - "Output before replay meta clears awaitingReplayMeta even when nothing is rendered yet, so an older host's first output never triggers a reset on its second frame"
  - "The hot-swap 'never term.reset()' source guard keeps its intent and allows exactly the onReset line"
metrics:
  duration: 32min
  completed: 2026-11-17
  tasks: 2
  files: 11
estimate:
  tokens: 85000
  tasks: 2
actuals:
  tokens: 11740
  tasks: 2
  commits: 6
plan_head_before: 7d5a9abdd9f7fe7ac82a9710286114be71c35897
plan_head_after: 0de35ab3c68379dadbd05a14597132356b793c2d
---

# Phase 5 Plan 04: Guest Reconnect Loop Summary

GUI peer tabs and browser share views now reconnect by themselves after a dropped or half-open socket. RelayClient has an opt-in reconnect mode. It classifies each close, backs off 1 to 30 s with ±20 % jitter, runs a re-auth hook, and redials with `resume=<offset>`. It resets xterm only when the server announces a full replay. A watchdog armed by the first pong catches silent sockets after sleep. TerminalPanel turns this on for guests, blocks typing while the socket is down, and shows "Connection lost — reconnecting…" only after 3 s.

`actuals.commits` is 6 because two `docs(todo)` commits by the user (ba5c450e, b7b15928) landed on the branch during the run. This plan made 4 commits.

## What was built

- **`closeReasons.ts`:** `isTerminalClose(code, reason)` returns true only for 1000 with `share stopped` or `session ended`. 1000 alone is not final, because a client detach also sends 1000.
- **`relayClient.ts`:**
  - Frames: `MSG_PONG = 0x04` and `MSG_META = 0x20` parse to `pong` and `meta`.
  - Exports: `RECONNECT` holds every timing number, plus `RelayState`, `ReconnectVerdict` and `backoffDelayMs`, which rounds to whole ms.
  - Socket creation moved into `dial()`. Every handler checks a socket generation counter. Pings go out every 15 s (was 30 s).
  - Stream state: the client trusts its offset only after the current socket's offset meta, then adds `payload.length + 1` for each output frame. It calls `onReset` for `replay:"full"` when content already exists, and also when output arrives before any replay meta (an older host). A meta that fails to parse, or an offset that is not a safe integer, makes the next redial ask for a full replay.
  - Reconnect mode: a final close stops the client (`onClose`, state `stopped`). Any other close, including a failed first connect, goes to `reconnecting` and waits out the backoff. Then `beforeReconnect` runs, and a throw counts as `retry`. `ok {port}` rebuilds the loopback URL. `resume=` uses `&` when the URL already has `?cap=`. Backoff resets only after a socket stayed open 30 s.
  - Watchdog (1 s tick): it acts only when the socket is open, a pong has been seen, and the page is visible. After 40 s of silence it sends a probe ping. If nothing arrives within 5 s of the probe, the client abandons the socket (handlers nulled, `close()` called) and dials a new one at once.
  - `online` and `visibilitychange` skip the remaining backoff while reconnecting, and send a probe while open.
  - `close()` in reconnect mode clears every timer and listener, detaches the socket and fires nothing. Without reconnect mode, `close()` behaves as before.
- **`SessionEndedBanner.tsx`:** new kind `'reconnecting'` with fixed copy and no Dismiss button. `onDismiss` is now optional. The doc paragraph that described the old single-notice, never-reconnect rule was rewritten.
- **`TerminalPanel.tsx`:**
  - New `beforeReconnect` prop, read through a ref.
  - Guests get `reconnect`, `onReset: () => term.reset()` and `onStateChange`. `reconnecting` disables stdin and starts the 3 s banner timer. `open` hides the banner and re-enables stdin. `stopped` hides the banner.
  - Host-path panels get no reconnect option.
  - The stale "never reconnects" comments were rewritten.

## Verification

- Task 1 verify: tsc clean, `TerminalPanel.reconnect.test.tsx` 5/5. Full suite 83/83 files.
- Task 2 verify: tsc clean, the five-file run reported `Test Files  5 passed`, and relayClient.test.ts passed 60/60.
- Full suite at plan end: 1085/1086 tests. The one failure is the pre-existing SharePopover Copy flake, which also fails at the plan base (see Deferred Issues). Earlier full runs on this branch passed 83/83.
- Acceptance greps all match. `MSG_PONG *= *0x04`, `export const RECONNECT`, `resume=`, `export function isTerminalClose`, `Connection lost`, `term.reset()`, `visibilitychange`, `'online'` and `backoffDelayMs` in the test file are each ≥1. `never wired to auto-reconnect`, `No auto-reconnect` and `NOT wire any auto-reconnect` are each 0. `git diff --name-only fb268a0a -- web/ frontend/package.json frontend/pnpm-lock.yaml` is empty.
- Tracer gate (Task 1): the automated verify was re-run end-to-end, passed, and work continued (human_verify_mode end-of-phase).
- Note: the plan's verify greps (`Test Files +1 passed`) match only with `NO_COLOR=1`. vitest emits ANSI color here even when piped.

## TDD Gate Compliance

| Task | RED commit | GREEN commit | RED cause |
|------|-----------|--------------|-----------|
| 1 | ceebe07e | 6a673c1f | no second socket after a 1006 drop (3 reconnect cases failed; final-close and host-path guards passed, as expected) |
| 2 | d5355ce9 | 0de35ab3 | watchdog, probe, wake and late-event cases failed, plus the jitter bound (float 1200.0000000000002 > 1200) |

Many Task 2 cases (retry classes, verdicts, healthy reset, old-host reset, malformed meta) were green at the RED commit, because Task 1 built the loop they pin, as the plan orders.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The hot-swap source guard forbade any `term.reset()`**
- **Found during:** Task 1 full-suite run
- **Issue:** `TerminalPanel.hot-swap.test.tsx` greps the source and fails on any `term.reset()`. It exists so that addon hot-swaps never discard scrollback. The plan requires `onReset: () => term.reset()` for full replays.
- **Fix:** the guard now requires that exact onReset line and still forbids any other `term.reset()`.
- **Files modified:** frontend/src/components/__tests__/TerminalPanel.hot-swap.test.tsx (not in the plan's file list)
- **Commit:** 6a673c1f

**2. [Rule 1 - Bug] Stale-socket check by identity was racy**
- **Found during:** Task 2 design
- **Issue:** after an abandonment, `this.ws` still points at the dead socket while `beforeReconnect` is awaited. A late onclose would pass a `ws !== this.ws` check and schedule a second redial.
- **Fix:** a socket generation counter, bumped on dial, abandon and user close.
- **Commit:** 0de35ab3

**3. [Discretion] Watchdog death dials at once**
- The task text says "abandon the socket and schedule a reconnect". The must-have truth says "a new one dialed immediately". Abandonment and wake events call `redial()` directly, with no backoff wait for that attempt. This cannot cause a storm, because the watchdog fires at most once per 45 s of open-socket silence.

**4. [Discretion] "Re-enable stdin unless the session has ended" omitted as unreachable**
- In reconnect mode `onClose` fires only from `stop()`, which tears down every timer and listener first. No `open` state can follow a final close, so no guard was added.

## Deferred Issues

- SharePopover Copy test flake (pre-existing, fails at 7d5a9abd too). Logged in `deferred-items.md`.

## Known Stubs

None. `beforeReconnect` defaults to `{ kind: 'ok' }` until 05-05 wires the GUI and browser re-auth hooks, as planned.

## Notes for later plans

- 05-05: pass `beforeReconnect` to TerminalPanel. Return `{kind:'ok', port}` after `GetRelayPort`, `{kind:'stop', reason}` for not_sharing / no_such_session / forbidden, and `{kind:'retry'}` otherwise. A throw already counts as retry.
- 05-07: add a TESTING.md row for `TerminalPanel.reconnect.test.tsx`, and note the new cases in `relayClient.test.ts`, `closeReasons.test.ts`, `SessionEndedBanner.test.tsx` and `TerminalPanel.guestClose.test.tsx`.
- The watchdog's pong arming is client-wide, not per socket. If a reconnect lands on an older host after a newer one, the watchdog stays armed. Both Macs should run the same build (RESEARCH Pitfall 8).

## Self-Check: PASSED

- FOUND: frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx, .planning/phases/05-overnight-survival/deferred-items.md
- FOUND commits: ceebe07e, 6a673c1f, d5355ce9, 0de35ab3
