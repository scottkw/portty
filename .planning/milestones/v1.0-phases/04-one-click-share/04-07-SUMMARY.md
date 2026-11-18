---
phase: 04-one-click-share
plan: 07
subsystem: relay
tags: [go, websocket, relay, attach, share, uat-gap, terminal-queries]

requires:
  - phase: 04-one-click-share
    provides: share server, viewing-daemon proxy (/api/relay/remote/{id}/ws), portty attach through the local daemon (04-02, 04-04)
provides:
  - In-process harness of the real CLI attach path beside a GUI tab and a browser (internal/daemon/attach_viewers_test.go)
  - One harness variant per ranked UAT gap 1 hypothesis, with verdicts recorded in the file header
  - Hub.ReconnectPreamble strips reply-eliciting terminal queries from the raw catch-up replay
affects: [04-08 diagnostic log, TESTING.md M-83, phase 05 reconnect/idle]

actuals:
  tokens: 5800
  tasks: 3
  commits: 5
plan_head_before: 3d0c1d5cf43fd6f549b3841b5193c897dbc8e9de
plan_head_after: 95a3a95a03a409ce1a3f26cd48413b9a1fcc93a5

tech-stack:
  added: []
  patterns:
    - "attachScene harness: two in-process daemons (host plus viewing), the share server, a viewerTap per connection and the real attach.AttachSession"
    - "Catch-up replay is query-free: anything a joining terminal would answer is stripped before it is sent"

key-files:
  created:
    - internal/daemon/attach_viewers_test.go
    - .planning/phases/04-one-click-share/deferred-items.md
  modified:
    - internal/relay/hub.go
    - internal/relay/hub_test.go
    - internal/daemon/engine.go
    - TESTING.md
    - .planning/STATE.md

key-decisions:
  - "UAT gap 1 root cause is not pinned: no in-process variant reproduces its signature; the live repro relies on the 04-08 diagnostic log and TESTING.md M-83"
  - "H-replies was real and is fixed in shared host code: Hub.ReconnectPreamble applies liveEmuQueryStripPattern to both raw replay paths"
  - "liveEmuQueryStripPattern and engine.go queryStripPattern gain XTVERSION (CSI > Ps q) and the kitty keyboard query (CSI ? u); kitty push/pop/set and DECSCUSR are not matched"
  - "A query split across two PTY reads is not stripped (accepted, documented in the ReconnectPreamble comment)"

patterns-established:
  - "Gap diagnosis by harness: build the real path in-process first, then one variant per hypothesis, fix only what goes red"

requirements-completed: [SHAR-06, SHAR-01, SHAR-03]

coverage:
  - id: D1
    description: "A CLI attach and detach leaves the office panel, a GUI tab through the viewing daemon's proxy and a browser on the link with their output and the 120x40 host grid"
    requirement: SHAR-06
    verification:
      - kind: integration
        ref: "internal/daemon/attach_viewers_test.go#TestCLIAttach_OtherViewersKeepOutput"
        status: pass
    human_judgment: false
  - id: D2
    description: "A tiny CLI size (0x0, 1x1, 2x1) never changes the host grid or sends guests another grid; a CLI that never reads does not delay the others"
    requirement: SHAR-06
    verification:
      - kind: integration
        ref: "internal/daemon/attach_viewers_test.go#TestCLIAttach_Hypotheses/tiny_resize_0x0"
        status: pass
      - kind: integration
        ref: "internal/daemon/attach_viewers_test.go#TestCLIAttach_Hypotheses/tiny_resize_1x1"
        status: pass
      - kind: integration
        ref: "internal/daemon/attach_viewers_test.go#TestCLIAttach_Hypotheses/tiny_resize_2x1"
        status: pass
      - kind: integration
        ref: "internal/daemon/attach_viewers_test.go#TestCLIAttach_Hypotheses/stuck_cli_never_reads"
        status: pass
    human_judgment: false
  - id: D3
    description: "A terminal answering the historic queries in the catch-up replay types nothing into the shared shell (replay keeps text and mode sets, drops queries)"
    requirement: SHAR-06
    verification:
      - kind: integration
        ref: "internal/daemon/attach_viewers_test.go#TestCLIAttach_Hypotheses/replayed_query_replies"
        status: pass
      - kind: unit
        ref: "internal/relay/hub_test.go#TestReconnectPreamble_StripsTerminalQueries"
        status: pass
    human_judgment: false
  - id: D4
    description: "With a real tailnet path, a CLI attach from the home Mac leaves the home GUI tab and a browser on the same session working (the UAT gap 1 event itself)"
    requirement: SHAR-06
    verification: []
    human_judgment: true
    rationale: "Root cause not pinned in-process (H-grid and H-net have no in-process trigger); needs the two-Mac live repro TESTING.md M-83 with the 04-08 diagnostic log"

duration: 11min
completed: 2026-11-17
status: complete
---

# Phase 4 Plan 07: CLI Attach Beside Other Viewers (UAT gap 1) Summary

**In-process harness of the real CLI attach path, one variant per hypothesis, and a fix for the one that went red: the catch-up replay no longer carries terminal queries, whose answers a joining terminal used to type into the shared shell. UAT gap 1's root cause is still not pinned.**

## Performance

- **Duration:** 11 min
- **Started:** 2026-11-17T17:45:52Z
- **Completed:** 2026-11-17T17:56:40Z
- **Tasks:** 3
- **Files modified:** 7 (4 Go, TESTING.md, STATE.md, deferred-items.md)

## Accomplishments

- `TestCLIAttach_OtherViewersKeepOutput` runs everything in one process: the host share server, the relay hub, the viewing daemon's proxy and the real `attach.AttachSession`. The local panel, the GUI tab and the browser keep output and the 120x40 grid through a CLI attach and detach. It passed on unmodified code.
- `TestCLIAttach_Hypotheses` has five variants. Four passed on unmodified code. `replayed_query_replies` failed, which reproduced H-replies.
- Fixed H-replies in shared relay code, with a failing test committed first. The extended query pattern is mirrored in engine.go.

## Diagnosis

| Hypothesis | Evidence (unmodified code) | Verdict | Live evidence |
|---|---|---|---|
| H-grid: host-local tiny grid push | No in-process trigger. Only `Hub.ResizeClient` broadcasts a 0x02 grid frame, and only for an Origin "local" subscriber. The remote CLI is Origin "web". | Not reproducible in-process | 04-08 viewer-diag grid lines, M-83 |
| H-net: first-contact tailnet path stall | No in-process trigger | Not reproducible in-process | 04-08 proxy dial/close lines, M-83 `tailscale ping` |
| H-replies: replayed query replies typed into the shared shell | `replayed_query_replies` **FAIL**. local, guiTab and browser each got 54 echoed reply bytes (`^[[24;1R^[[>1;10;0c^[[?2026;2$y^[P>|fake-term^[\^[[?0u`). | Reproduced and fixed | n/a |
| H-resize: the CLI reports a tiny size | `tiny_resize_0x0`, `tiny_resize_1x1`, `tiny_resize_2x1` **PASS** | Refuted | n/a |
| H-stuck: a CLI that stops reading blocks fan-out | `stuck_cli_never_reads` **PASS** | Refuted | n/a |

Every result matched the expectation from code reading, so there were no surprises. The OSC 11 query typed on the local conn never reaches the scrollback ring, because the host's InputAbsorber drops input that starts with OSC 11. So that variant's fake terminal answered five queries, not six.

**Fix.** `Hub.ReconnectPreamble` now runs `liveEmuQueryStripPattern` over the raw scrollback on both raw paths: ring intact, and the fallback raw tail. The pattern now also matches XTVERSION (`CSI > Ps q`) and the kitty keyboard query (`CSI ? u`), and `queryStripPattern` in engine.go mirrors both. Mode sets and colours stay in the replay (alternate screen, bracketed paste, SGR). No existing preamble fixture embedded a query, so none needed changing.

Root cause: not pinned — no in-process variant reproduces the UAT signature (guests lose output or get a non-host grid while the office panel is fine), so the live repro relies on the 04-08 diagnostic log and TESTING.md M-83.

H-replies hits every viewer, including the office's own panel. That makes it a real CLI-attach disturbance, but not the UAT event, where the office window kept working.

**Side findings.**
- `4 viewers` is normal. It is `SubscriberCount`: the office's local panel, the GUI tab, the browser and the CLI.
- `[reconnecting]` appears on any idle remote attach after 5 s. The host sends no heartbeat, and the cmd_attach.go watcher keys on received frames. This is recorded in STATE.md as a Phase 5 concern.
- **New, out of scope:** the 32 KiB WebSocket read limit drops anyone joining through the proxy once a session has more history than that. No Go reader calls `SetReadLimit`, so coder/websocket's 32768-byte default applies to the proxy's upstream conn and to the CLI conn. The preamble is the whole ring (up to 256 KiB) in one message. A scratch run with about 52 KiB of history showed the CLI get the grid frame and then EOF. It kills the joiner, not the other viewers, so it does not match gap 1. It would stop a GUI tab or CLI from attaching to a long session, though. Logged in deferred-items.md with a fix sketch.

## Task Commits

1. **Task 1: in-process harness (tracer)** - `702e73b0` (test). The tracer gate re-ran the automated verify, which passed, then expansion continued.
2. **Task 2: hypothesis variants, run on unmodified code** - `e4f6dbc4` (test, red: `replayed_query_replies`)
3. **Task 3: fix, verdicts, docs**
   - `2e586bd7` (test, RED: `TestReconnectPreamble_StripsTerminalQueries`)
   - `bac4078d` (feat, GREEN: query strip in `ReconnectPreamble` plus the pattern mirror)
   - `95a3a95a` (docs: verdict header, TESTING.md, STATE.md, deferred-items.md)

## TDD Gate Compliance

- **RED:** `e4f6dbc4` and `2e586bd7`. For `TestCLIAttach_Hypotheses/replayed_query_replies`, `gsd_run check tdd-red-evidence` returned `RED_EVIDENCE_OK` (go-test format, target failed, exit 1). Semantic check: the target ran and failed on the planned assertion, idle viewers receiving the echoed reply bytes. It was not a setup, build or fixture fault. `TestReconnectPreamble_StripsTerminalQueries` failed on its preamble-equality assertion.
- **GREEN:** `bac4078d`. All seven `TestCLIAttach_` results pass (two tests plus five subtests), and the relay suite is green.
- **REFACTOR:** none needed.

## Files Created/Modified

- `internal/daemon/attach_viewers_test.go` - harness, five variants, verdict header and Root cause line
- `internal/relay/hub.go` - `ReconnectPreamble` strips queries on the raw paths; the pattern gains XTVERSION and the kitty query; doc comments explain why
- `internal/relay/hub_test.go` - `TestReconnectPreamble_StripsTerminalQueries`
- `internal/daemon/engine.go` - `queryStripPattern` mirror kept byte-identical
- `TESTING.md` - Go 111 / total 201, a 04-07 note, two SHAR-06 rows
- `.planning/STATE.md` - Phase 5 `[reconnecting]` / heartbeat concern
- `.planning/phases/04-one-click-share/deferred-items.md` - read-limit finding, revoke test flake, pre-existing gofmt drift

## Decisions Made

See key-decisions in the frontmatter. In short:
- The root cause is not pinned.
- The query strip lives in `ReconnectPreamble`, so guests, the remote CLI and local reconnects are all covered by one change.
- Split queries are accepted as a documented residual.

## Deviations from Plan

- **Fake terminal responder runs synchronously.** It answers right after the catch-up arrives instead of from a background goroutine. The whole replay, queries included, has arrived by the time `MARKER-1` is seen, so the replies reach the shell the same way. This kept the test simpler.
- **Viewing-side GUI tab dials with `Origin: wails://wails`.** That is the real GUI's origin. The plan did not specify one.

Otherwise the plan was executed as written. No Rule 1-4 auto-fixes.

## Issues Encountered

- `TestRevoke_InputAfterRevokeNeverReachesPTY` (internal/webserver) failed once during the full `go test -race -short ./...` run. It passed 30/30 in isolation and on a full rerun. This plan changed nothing under internal/webserver. Logged in deferred-items.md. The likely cause is that `RevokeWebViewers` kicks viewers asynchronously.
- `gofmt -l internal/relay` lists `scrollback_altscreen_test.go`. That drift was already on the base commit. Logged, not touched.

## User Setup Required

None. No external service configuration required.

## Next Phase Readiness

- 04-08 adds the daemon diagnostic log and M-83. The two top-fit hypotheses, H-grid and H-net, can only be settled live.
- Consider fixing the 32 KiB read limit (deferred-items.md) before the next two-Mac UAT. Otherwise a long session cannot be attached through the proxy, which could look like a new failure.
- The CSP, capability gate and Origin allowlist are untouched. No frontend files changed, and go.mod and go.sum are unchanged.

## Self-Check: PASSED

- All six key files exist on disk.
- All five task commits are ancestors of HEAD.
- `check evaluation-scope --plan 04-07` resolved the plan's commits.
- The plan's `<verification>` passed:
  - `go build`, `go vet`, and the tagged vets (`playwrightfixture`, `vmpreview`) pass.
  - `go test -race -short ./...` is green on the rerun.
  - All seven `TestCLIAttach_` results pass.
  - The traceability, share-removal and brand gates pass.
  - Nothing changed under frontend/, go.mod or go.sum.

---
*Phase: 04-one-click-share*
*Completed: 2026-11-17*
