---
phase: 19-unattended-desktop-viewing
plan: 03
subsystem: desktop-sharing
tags: [pion, webrtc, bind, tailnet-only, keyframe-coalescing, multi-viewer, data-channel, sctp]
status: complete

requires:
  - phase: 19-01
    provides: Manager, ICE-lite peer, screencap.Source and SyntheticSource, loopback tracer
provides:
  - "internal/desktop/bind.go: checkBind (tailnet block only, loopback only under the test hook), listenMedia with EADDRINUSE retry, ErrBindRefused, ErrNoTailnetAddr, MediaPortDefault 7444"
  - "per-viewer outbound track fed from one packetizer; a viewer receives no delta before the first keyframe written after its media path is up"
  - "requestKeyframe: join (on Connected), PLI and FIR coalesced to one forced keyframe per second, one deferred at the window end"
  - "Viewer record (ViewerID, Rights, RightsViewOnly, JoinedAt), Manager.ViewerList, Manager.Full, MaxViewers 8, CloseFull 4429"
  - "Deps.OnDataChannel hook (default CloseDataChannel), SCTP max message size 8192"
affects: [19-04, 19-08, 19-09, 19-15, 20]

actuals:
  tokens: 21000
  tasks: 3
  commits: 5
plan_head_before: 542c37c015c5a91ded4db4b3024652f9f9a2d682
plan_head_after: 1f7a3d0fab113c226a83dd2cf53256a043a23c4f

tech-stack:
  added: []
  patterns:
    - "One rtp.Packetizer in fanOut, one TrackLocalStaticRTP per viewer; SSRC and payload type are rewritten per binding by pion"
    - "Per-viewer gate: ready (PeerConnection Connected) then armed (first keyframe written after ready), both under vmu"
    - "engineSummary records what the SettingEngine was told, because pion's SettingEngine has no getters"

key-files:
  created:
    - internal/desktop/bind.go
    - internal/desktop/bind_test.go
    - internal/desktop/viewer.go
    - internal/desktop/peer_test.go
    - internal/desktop/fanout_test.go
    - internal/desktop/export_test.go
    - internal/screencap/synthetic_test.go
  modified:
    - internal/desktop/manager.go
    - internal/desktop/peer.go
    - internal/desktop/loopback_test.go
    - go.mod

key-decisions:
  - "A keyframe is requested when the PeerConnection reaches Connected, not at join: a join-time request is wasted (no media path yet) and burns the one-second window, delaying the request that matters by up to a second"
  - "Per-viewer tracks replace the shared TrackLocalStaticSample, to deliver 19-01's hand-off (no delta before a viewer's first keyframe)"
  - "The cap is checked at the top of ServeDesktopViewer and again under the lifecycle lock in join, so simultaneous handshakes cannot overshoot 8"

requirements-completed: [DESK-02, DESK-04, DESK-01]

coverage:
  - id: E1
    description: "Three viewers joined in turn each get SPS+PPS+IDR as their first access unit within 2 s; one source start; 3 keyframes forced; every viewer sees the 1280x800 size and the viewer count 3"
    requirement: "DESK-04"
    verification:
      - kind: integration
        ref: "internal/desktop/fanout_test.go#TestFanout_ThreeViewersEachGetKeyframe"
        status: pass
    human_judgment: false
  - id: E2
    description: "The media bind accepts the two ends and the middle of the tailnet block and refuses the address below it, the one above it, unspecified, LAN, IPv6, IPv4-mapped and (outside tests) loopback; the zero address gets its own error; a refusal binds no socket; a busy port is retried and a refused address is not"
    requirement: "DESK-02"
    verification:
      - kind: unit
        ref: "internal/desktop/bind_test.go#TestCheckBind_Boundaries"
        status: pass
      - kind: unit
        ref: "internal/desktop/bind_test.go#TestManager_StartRefusesWithoutTailnetAddr"
        status: pass
      - kind: unit
        ref: "internal/desktop/bind_test.go#TestListenMedia_RetriesWhileBusy"
        status: pass
    human_judgment: false
  - id: E3
    description: "The answer offers one host candidate on the media socket (pion writes it as component 1 and 2), ICE-lite; a whole session writes nothing to ports 3478, 19302 or 5353 and only to the viewer's one address; the configuration has no ICE servers and the engine is UDP4 with mDNS off"
    requirement: "DESK-02"
    verification:
      - kind: integration
        ref: "internal/desktop/peer_test.go#TestPeer_AnswerHasOneHostCandidate"
        status: pass
      - kind: integration
        ref: "internal/desktop/peer_test.go#TestPeer_NoStunTurnMdnsTraffic"
        status: pass
      - kind: unit
        ref: "internal/desktop/peer_test.go#TestPeer_APIHasNoICEServersAndNoMDNS"
        status: pass
    human_judgment: false
  - id: E4
    description: "A Rebuild makes the first later unit a keyframe at the new size; connected viewers get the size message and a keyframe, a later viewer gets the size after its answer"
    requirement: "DESK-04"
    verification:
      - kind: unit
        ref: "internal/screencap/synthetic_test.go#TestSynthetic_RebuildChangesSize"
        status: pass
      - kind: integration
        ref: "internal/desktop/fanout_test.go#TestFanout_SizeEventOnRebuild"
        status: pass
    human_judgment: false
  - id: E5
    description: "A viewer-made data channel reaches the hook with a view-only Viewer record; with no hook the server closes it within 2 s; the answer carries max-message-size 8192; unknown hello caps are ignored and only video is echoed"
    requirement: "DESK-04"
    verification:
      - kind: integration
        ref: "internal/desktop/fanout_test.go#TestFanout_DataChannelHook"
        status: pass
      - kind: integration
        ref: "internal/desktop/fanout_test.go#TestFanout_HelloCapsForwardCompatible"
        status: pass
    human_judgment: false
  - id: E6
    description: "A ninth viewer is closed 4429 \"full\" and the eight keep streaming; a reconnect during another viewer's join ends with everyone streaming, one source start and 1-2 forced keyframes; 50 PLIs in one window force at most 2 keyframes"
    requirement: "DESK-04"
    verification:
      - kind: integration
        ref: "internal/desktop/fanout_test.go#TestFanout_ViewerCap"
        status: pass
      - kind: integration
        ref: "internal/desktop/fanout_test.go#TestFanout_ReconnectWhileJoining"
        status: pass
      - kind: integration
        ref: "internal/desktop/fanout_test.go#TestFanout_PLIStormCoalesced"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-11-26
---

# Phase 19 Plan 03: Media hardening and multi-viewer Summary

**Media is bound to a tailnet address only and never touches STUN, TURN or mDNS; up to eight viewers each get their own first keyframe on join; keyframe requests are coalesced to one per second; and the Phase 20 seams (size events, data-channel hook, viewer rights record) exist and are tested.**

## Accomplishments

- **Tailnet only (T-19-10):** `checkBind` admits a plain IPv4 address inside the tailnet block and nothing else. Loopback needs the test hook, which panics outside a test binary. `NewManager` rejects the zero address, `Start` binds through `listenMedia`, so a refusal opens no socket. A busy port is retried every 250 ms for up to 30 s (the upgrade case); a refused address is never retried.
- **One host candidate, no outside traffic:** `newSettingEngine` is the single hardening point (ICE-lite, UDP4, mDNS off, IP filter equal to the bind address, SCTP limit). `peerConfig` has no ICE servers. A recording `PacketConn` on the media socket shows a full session writes only to the viewer, never to 3478, 19302 or 5353.
- **Picture on join, no delta first:** one packetizer in `fanOut` feeds a track per viewer. A viewer is `ready` when its PeerConnection is Connected and `armed` by the first keyframe written after that; it never receives a delta before. `TestFanout_ThreeViewersEachGetKeyframe` asserts that the first access unit each viewer receives is SPS+PPS+IDR (this strictness was not possible with the shared sample track).
- **Coalescing (T-19-11):** joins (on Connected), PLI and FIR go through `requestKeyframe`, driven by `Deps.Now`. 50 PLIs in a window force 1 or 2 keyframes.
- **Phase 20 seams:** `Viewer{ID, Rights, JoinedAt}` and `ViewerList`, `Deps.OnDataChannel` (default closes the channel), SCTP max message size 8192, size messages on change and after each answer, hello caps ignore unknown keys.
- **Viewer cap:** `MaxViewers` 8; a ninth socket gets close 4429 "full" before any PeerConnection; `Full()` is there for 19-04's route check.

## Task Commits

1. **Task 1 (tracer): per-viewer tracks, join keyframe gating, coalescing** - `b8aaa8be` (feat)
2. **Task 2 RED: bind, candidate and traffic tests** - `7c87f37d` (test; the test package did not build)
3. **Task 2 GREEN: tailnet-only bind, hardened engine** - `5fd0410a` (feat)
4. **Task 3 RED: size, data channel, cap, reconnect, PLI storm tests** - `9076fb1d` (test; the test package did not build)
5. **Task 3 GREEN: cap, hook, SCTP limit** - `1f7a3d0f` (feat)

## TDD Gate Compliance

Both TDD tasks have a `test(19-03)` commit before their `feat(19-03)` commit. Both RED commits fail at build time (undefined symbols), not at an assertion, because the production names did not exist yet. Notes on unexpected greens, investigated rather than trusted:

- Task 3's size-change, hello-caps and PLI behaviour already existed (19-01's size tracking and Rebuild, Task 1's coalescer), so those tests passed once the package built. I checked them by mutation, each reverted: dropping the forced keyframe in `SyntheticSource.Rebuild` fails `TestSynthetic_RebuildChangesSize`; removing the pending check in `requestKeyframe` fails `TestFanout_PLIStormCoalesced` (50 keyframes forced); making `Full()` return false fails `TestFanout_ViewerCap`.
- `synthetic.go` needed no change: 19-01 already shipped a correct `Rebuild`. Only its test is new.

## Deviations from Plan

**1. [Rule 1 - Bug] Keyframe requested on PeerConnection Connected, not at join**
- **Found during:** Task 1 design.
- **Issue:** The plan has every join call `requestKeyframe("join")` after the answer. At that moment the viewer's media path is not up, so the keyframe is dropped. It also uses the one-second window, so the request that matters (Connected) was deferred up to a second.
- **Fix:** the request is made on Connected only (a first viewer also gets the source's start keyframe). Joins are still coalesced because they go through the same function.
- **Consequence:** the plan's "KeyframesForced is 3" for three spaced joins holds exactly, as asserted.
- **Commit:** `b8aaa8be`

**2. [Rule 1 - Plan conflict] "Exactly one a=candidate line" is "exactly one candidate"**
- **Found during:** Task 2 GREEN. pion's SDP writer emits every candidate twice, as component 1 (RTP) and component 2 (RTCP), with no setting to turn it off (`addCandidatesToMediaDescriptions`). With rtcp-mux the second is ignored.
- **Fix:** the test asserts exactly one distinct candidate (`udp host 127.0.0.1 <media port>`) across the lines. The security property (one host candidate on the media socket) is unchanged.
- **Commit:** `5fd0410a`

**3. [Rule 3 - Test env] Loopback test client restricted to loopback addresses**
- The pion test client offered the machine's LAN address too, so the server answered a second viewer address. `newLoopbackClient` now filters to loopback so the viewer has one address on any machine. Test-only (`loopback_test.go`, which was not in this plan's file list; no other wave-2 plan touches it).

**4. [Plan interpretation] Reconnect test keeps a third viewer**
- With only one other viewer, A's close can take the count to zero and stop the source, so "the source never restarted" cannot be asserted. `TestFanout_ReconnectWhileJoining` therefore keeps a third viewer connected and ends at 3 viewers, not 2. It still covers A leaving and rejoining during B's join, with coalescing asserted at 1-2 forced keyframes.

**5. [Plan interpretation] Shared track became per-viewer tracks**
- The plan truth says "one shared track, one source". A `TrackLocalStaticSample` is shared by all bindings and cannot gate per viewer, which 19-01 handed to this plan. The source and packetizer are shared; each viewer has its own `TrackLocalStaticRTP`, so every viewer shows the same picture. Packets are not copied beyond pion's own per-binding header clone.

**Total deviations:** 5 (2 behaviour/spec, 1 test env, 2 plan interpretation). No impact on the contracts 19-04, 19-08 and 19-09 build on.

## Known Stubs

- `shortcut:` comments in code: the RTP timestamp step truncates to whole ticks (exact at 30 fps, drifts at an odd frame rate); `broadcast` is sequential (19-01's note).
- The pre-upgrade HTTP 429 lives in 19-04's route code: `Full()` is exported for it. Until that lands, only the in-Manager 4429 refusal is tested here.
- `Manager.Status()` still reports only idle or streaming (19-09).

No frontend or UI stubs. Nothing added to `.planning/WINDOWS.md`.

## Threat Flags

None beyond the plan's register. T-19-10 (bind and egress), T-19-11 (cap and coalescing) and T-19-12 (SCTP limit and view-only hook) are implemented and tested. T-19-13 stays accepted. T-19-SC: no new module; `go mod tidy` only moved `pion/rtcp` from indirect to direct.

## Verification

- `go vet`, `gofmt -l`, `golangci-lint run --new-from-rev=89d29266` for `internal/desktop` and `internal/screencap`: clean.
- `go test -race -count=1 ./internal/desktop/ ./internal/screencap/` pass; the `TestFanout_*` set passes `-race -count=10`, and the tracer plus first fanout test `-count=30`. `internal/desktop` statement coverage 87.3 percent.
- `go build ./...` ok; `bash tests/check-brand.sh` clean. Acceptance greps: `100.64.0.0/10` once in bind.go, no `ICEServers:` in peer.go, `SetSCTPMaxMessageSize(8192)` once, `MaxViewers` 3 times in manager.go.
- Host safety: only synthetic sources and loopback sockets were used. No VM, launchd, TCC or tailnet work was done.
- TESTING.md was not touched (19-16 owns the consolidated update). No `STATE.md` or `ROADMAP.md` changes.

## Self-Check: PASSED

- Files exist: bind.go, bind_test.go, viewer.go, peer_test.go, fanout_test.go, export_test.go, synthetic_test.go.
- Commits `b8aaa8be`, `7c87f37d`, `5fd0410a`, `9076fb1d`, `1f7a3d0f` are ancestors of HEAD.
