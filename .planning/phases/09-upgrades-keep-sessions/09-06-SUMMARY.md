---
phase: 09-upgrades-keep-sessions
plan: 06
subsystem: upgrade-handoff-verification
status: complete
tags: [upgrade, handoff, integration-test, tart-vm, reconnect, attach, testing-md, gate]
requirements-completed: [UPGR-01, UPGR-02, UPGR-03, UPGR-04, UPGR-05]
requirements-note: "Code and automated evidence are complete for all five. UPGR-01..03 have real-binary evidence in the VM (I-01..I-03). The two-build GUI/CLI VM rows for every success criterion (M-105..M-113, M-115) are 09-07."
dependency_graph:
  requires:
    - "09-01 copy strings, /health api and handoff, EnsureOptions, the update-needed check"
    - "09-02/09-03 adopt, fdpass, hub state export/import, RestartViewers 1012"
    - "09-04 runTakeover, outcome codes, the vmpreview fault syntax and step names"
    - "09-05 portty daemon --takeover, the status pipe, the EnsureDaemon handoff branch"
  provides:
    - "TestHandoffIT (env-gated, office VM): upgrade, fault and from-v1.1 against prebuilt binaries"
    - "hostReconnectVerdict(sessionId, deps?) and host-tab reconnect in TerminalPanel"
    - "attach.VerdictEnded, attach.ErrSessionEnded; local attach through attach.Run (newLocalTarget)"
    - "TESTING.md Phase 9 note and counts, 24 UPGR rows, Category AY M-105..M-115, M-104 restated"
    - "09-VM-CHECKS.md header, builds table and rows I-01..I-03"
  affects: [09-07 VM rows, TESTING.md Section 5 Last run fields]
tech-stack:
  added: []
  patterns:
    - "an env-gated integration test that drives prebuilt binaries in temp HOMEs and never builds or runs daemons on the host"
    - "re-auth verdict from the daemon's session list: listed and not stopped redials, missing or stopped ends (GUI host tab and CLI local attach alike)"
key-files:
  created:
    - internal/daemon/handoff_it_test.go
    - frontend/src/lib/hostReconnect.ts
    - frontend/src/lib/hostReconnect.test.ts
    - .planning/phases/09-upgrades-keep-sessions/09-VM-CHECKS.md
  modified:
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx
    - cmd_attach.go
    - cmd_attach_test.go
    - internal/attach/loop.go
    - internal/attach/loop_test.go
    - TESTING.md
decisions:
  - "The v1.1 build comes from `git archive v1.1` into the scratchpad, not `git worktree add`: same source, and nothing is added to the repository's worktree list"
  - "VerdictEnded ends attach.Run with a new sentinel ErrSessionEnded ('portty: the session is no longer running'); a local end still prints 'Detached.' first, and remoteAttachEnd stays remote-only"
  - "remoteTarget is now attachTarget{url, reauth, remote}; runAttach always uses attach.Run, and attach.AttachSession is kept (its own tests and the daemon viewer tests use it)"
  - "Host tabs show the existing reconnecting banner (the banner condition no longer requires a guest); their final close still shows nothing"
  - "The fd/env check uses the shell's own builtin `[ -e /dev/fd/3 ]`: `ls /dev/fd` lists its own directory fd, so it is evidence, not the test"
metrics:
  duration: ~25m
  completed: 2026-11-19
estimate:
  tokens: 115000
  tasks: 2
actuals:
  tokens: 20000
  tasks: 2
  commits: 6
plan_head_before: a13f49281ab0c25a67450cc6b52a89d241867884
plan_head_after: 9d7cfc62e59a2cb9da264b9e8586ea18caeb11b9
---

# Phase 9 Plan 06: Real-binary handoff evidence, the last viewer paths, and the Phase 9 gate Summary

A real build A handed three live sessions to a real build B in the office VM: same ids, same shell pids, `/health` B, and a CLI viewer that reconnected once with a contiguous tick sequence. A fault-injected takeover left A serving, and a v1.1 daemon got the plain DEBT-08 refusal. Local GUI tabs and local `portty attach` now reconnect by themselves after a handoff and stop when the session is gone. TESTING.md documents Phase 9, and the full gate passed (one known flake, which passed on its single rerun).

## Integration test rows (09-VM-CHECKS.md)

| Row | Result | Key evidence |
|-----|--------|--------------|
| I-01 upgrade | PASS | `B list --local` rc 0 with empty stderr. `/health` = `{v1.2.0-itB, api 2, handoff 1}`. Ids before = after (3). Shells `[27640 27649 27656]` alive with ppid 1. A exited 0. Viewer: reconnects=1, TICK-1 ran 1..15 with no gap and no repeat. A fresh viewer's replay holds HIST-1-1 and HIST-1-300. fd/env check: `env=0 fd3=closed` for A's shell before, the same adopted shell after, and a shell started by B. |
| I-02 fault | PASS | `new:before-ready:exit`: rc 1. stderr has `couldn't move the running sessions to version v1.2.0-itB (the new version exited)…` and the DEBT-08 refusal. A logged `rolled back at ready after 5ms`. `/health` still itA. A and both shells alive (ppid A). Viewer: reconnects=0, ticks contiguous. |
| I-03 from-v1.1 | PASS | rc 1 with the plain refusal and no `couldn't move`. `/health` = `{"status":"ok","version":"v1.1.0-itOld"}` (no `api`). Daemon and shells alive. Viewer: reconnects=0. |

09-05's open item is closed: shells never see `PORTTY_TAKEOVER_STATUS_FD` or an inherited fd 3. This holds for adopted shells and for shells the takeover daemon starts after the commit.

### IT binaries (sha256, identical on the host and in the VM; all `Signature=adhoc`, `TeamIdentifier=not set`)

| Binary | Version | sha256 |
|--------|---------|--------|
| portty-itA | v1.2.0-itA | 3b749ecc24e054d2e23456d546215d835f21953ee3dbd7b1a6f436f763aa9795 |
| portty-itB | v1.2.0-itB | e0686938e37798b382473117d16f0f0ee54da6b7c683cdde62c5ee62c90e8211 |
| portty-itBfault (`-tags vmpreview`) | v1.2.0-itB | f8453f7d9374ad82add15170d972c18a9a4422728718a45f59a8732dd06410e4 |
| portty-itOld (tag v1.1, bbbd8060) | v1.1.0-itOld | 03e79f90bc6aceeb7977c04256eb5db11f66d4ac1c64d7d1b4c5c726050a9c29 |
| daemon-it.test (23509107) | n/a | 53a4cc021261274ff45f06b8ab4e8e99d0b2479d2584297f3f9326b86e66f90c |

## Reconnect for local tabs and local attach

- `hostReconnectVerdict(sessionId, deps = App)` (frontend/src/lib/hostReconnect.ts): if the port or list read fails, it returns `{kind:'retry'}`. If the session is missing or `stopped`, it returns `{kind:'stop', reason:'ended'}`. Otherwise it returns `{kind:'ok', port}`. The bindings are read only at call time.
- TerminalPanel: host tabs pass `reconnect: { beforeReconnect: () => hostReconnectVerdict(sessionId) }`. The comment now explains why host tabs reconnect and why a crash still ends them. The reconnecting banner shows for host tabs too.
- `attach.VerdictEnded` ends `Run` with `attach.ErrSessionEnded`.
- `attachLocal` builds `newLocalTarget(client.ListSessions, client.GetRelayPort, id, clientName, port)` and runs `attach.Run`. Its re-auth reads the session list:
  - Daemon unreachable: it calls `ensureLocalDaemon` once and reads the list again. Still unreachable means retry.
  - Session missing or stopped: VerdictEnded.
  - Otherwise: it re-reads the relay port and returns VerdictOK.

## Tests added (for the record)

| File | Tests |
|------|-------|
| internal/daemon/handoff_it_test.go (new) | TestHandoffIT/upgrade, /fault, /from-v1.1 (env-gated) |
| frontend/src/lib/hostReconnect.test.ts (new) | 5 cases: port fails, list fails, missing, stopped, running |
| frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx | "a host-path tab reconnects after 1012 with a delta from the re-read port while its session is listed", "a host-path tab stops quietly when the daemon no longer lists its session"; the old "host-path tab does not reconnect after a 1006 close" case is replaced (see deviation 1) |
| internal/attach/loop_test.go | TestRun_VerdictEndedStops |
| cmd_attach_test.go | TestAttachLocal_Reauth (7 subtests + the client-name URL) |

## Gate (run once, full output saved, no -skip)

Saved to the session scratchpad: `gate-go.log`, `gate-rest.log` and `rerun-exitevent.log` under `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/gate/`.

- `go build ./... && go vet ./...`: rc 0.
- `go test -race -short -count=1 ./...`: one `--- FAIL` line, `--- FAIL: TestExitEvent_ListSessions_ExitCodePopulatedForStopped (0.00s)` (`engine_exit_test.go:136: ExitCode for running session: want nil`). It is on the known-flake list and uses a spyBackend, not Phase 9 code. Its one rerun (`-run '^TestExitEvent_ListSessions_ExitCodePopulatedForStopped$'`) gave `--- PASS`, `ok internal/daemon 1.024s`. Every other package was ok: root, attach, capability, fdpass, pty, relay, release, statusbar, tailnet, updater, wakewatch, webserver.
- vitest: 95 files, 1300 tests passed. `tsc --noEmit`: rc 0.
- check-brand `OK: brand gate clean`. check-share-removal `OK: share removal gate clean`. check-traceability `OK: 163 traceability paths exist`. build-script.test.sh `Results: 40 passed, 0 failed`.
- `git status --porcelain -- frontend/src/wailsjs` is empty. The build-script test generated nothing there, so no restore was needed.
- csp_mw.go, capability_mw.go and tailnet/live.go are unchanged against b4d63644. `const MaxInputPayload = 32767` is unchanged.

**Skip list:** TestHandoffIT (env gate, documented). The existing conditional skips are unchanged: `-short` integration tests (pty session_exit, the long-lived remote WS proxy, the relay timing test), Chromium-unavailable chromedp tests, the updater real-DMG tests, the live-tailnet tests, and missing-binary guards.

## TESTING.md final counts

Go **145**, vitest **95**, Playwright **7**, build-script **1**, total **248**. These match the planned expectation. Section 4 has 24 UPGR rows, and every test name in them exists. Section 5 has Category AY (M-105..M-115). M-114's Last run points at I-01..I-03, and the others say "pending (09-07)". M-104 is restated for the handoff era with the same number.

## Deviations from Plan

1. **[Requirement change, not weakening] The host-path "never reconnects" test was replaced.** TerminalPanel.reconnect.test.tsx pinned "a host-path tab does not reconnect after a 1006 close". This plan reverses that behaviour (CONTEXT "Local GUI tabs and local `portty attach` gain reconnect"). The case became two: reconnect on 1012 while the session is listed, and a quiet stop when it is not. The stop case keeps the old guarantee for a real crash.
2. **[Rule 1, own test] The IT's first VM run left a `/tmp/pit-*` dir.** All three subtests passed, but an adopted zsh saved `.zsh_history` into the temp HOME after cleanup had removed it. Fix (23509107): the test's `.zshrc` unsets HISTFILE, and cleanup waits for the recorded shells to exit (SIGKILL for any left). The from-v1.1 subtest also records its shells and checks that they stay under the v1.1 daemon with a viewer that never reconnects. The rows come from the second run, on the final test binary. The leftover dir was removed by hand.
3. **[Constraint] The v1.1 build used `git archive v1.1`** instead of `git worktree add` (worktree isolation, and nothing is left to remove).
4. **[Scope] The IT binaries were built before Task 2's reconnect change.** The IT's viewer uses `attach.Run`, which only gained the VerdictEnded case. The new local-attach and host-tab paths are covered by unit tests; their real-build check is M-106 in 09-07.
5. **[Environment]** `pnpm install --frozen-lockfile --offline` in the worktree (no package changed).

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-09-31: only builds, one skipping `-run` and the gate ran on the host; daemons and the fault ran in the VM. T-09-32: the IT never shares a session, and the cap/token grep on 09-VM-CHECKS.md finds nothing. T-09-33: hostReconnect.test.ts and TestAttachLocal_Reauth. T-09-34: one ensure per re-auth, paced by the backoff.

## Self-Check: PASSED

- FOUND: internal/daemon/handoff_it_test.go, frontend/src/lib/hostReconnect.ts, frontend/src/lib/hostReconnect.test.ts, .planning/phases/09-upgrades-keep-sessions/09-VM-CHECKS.md
- FOUND (ancestors of HEAD): 0e9730af, 23509107, 6e9ae88e, 020b882f, b26693cd, 9d7cfc62
