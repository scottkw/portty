---
status: resolved
trigger: "Fix a Portty desktop-share audio bug, failing test first. After sudo killall coreaudiod while a viewer listened, capture delivered no more audio and raised no error; audio packets went flat while video kept rising and /desktop/stats kept reading audio: ok (23-09 finding 3, VM row A-03)."
created: 2026-11-27T00:00:00Z
updated: 2026-11-28T03:00:00Z
---

## Current Focus

hypothesis: H1 the Manager's audio state has only two inputs (an AudioErrors error -> failed, an AudioPackets packet -> ok); an SCK audio tap that dies silently (coreaudiod restart) produces neither, so audioFail stays false for the whole capture run and nothing re-establishes the tap
test: code read of audioOut/noteAudioFailed/noteAudioOK/startSource + ObjC pc_audio_handle/didStopWithError; then a failing Go test: audio source goes quiet with no error, fake clock advanced past a stall bound, expect stats audio=failed
expecting: test fails today (audio stays ok); passes once a packet-absence watchdog exists
bug_class: Bohrbug (deterministic: no code path observes absence of packets)
known_pattern_candidate: none (KB has no audio entry)
design: audioWatchStep in the state-loop tick (tickMu): run identity = m.audioDone; progress = atomic packet count bumped in audioOut; no progress for audioStallAfter (3 s) on a display share -> noteAudioFailed(errAudioStalled) + src.(screencap.AudioRestarter).RestartAudio() with backoff 5 s doubling to a 60 s cap; progress resets the episode and audioOut's noteAudioOK clears the state; DisplaySource.RestartAudio -> new pc_audio_restart (capturesAudio off/on via updateConfiguration, 5 s bound each like the surface/rebuild updates, gConfig.capturesAudio left YES); window shares are not watched (shortcut, per-app audio unmeasured)
reasoning_checkpoint:
  hypothesis: "Stats and the state message read audio ok during A-03 because Manager.audioFail is written only by an AudioErrors error or an AudioPackets packet, and a dead SCK audio tap produces neither; nothing ever re-creates the tap inside a run because the only re-creation is a new capture run"
  confirming_evidence:
    - "every audioFail writer read (manager.go startSource reset, audio.go noteAudioFailed/noteAudioOK); none is time-based"
    - "TestAudio_SilentStallIsFailed RED on unfixed code: 3.5 s of fake time with no packet and no error leaves Stats().Audio == ok (timed out waiting for failed)"
    - "A-03: packets flat 4584 at +8 s and +18 s, stats ok on 30 samples; a fresh capture run restored audio"
  falsification_test: "if the A-03 VM re-run still reads audio ok more than ~4 s after killall coreaudiod with the fix, packets must still be arriving (the stall is downstream of audioOut) and the hypothesis is wrong"
  fix_rationale: "an absence detector is the only signal available for a fault that raises nothing; it writes the same audioFail the stats route and the viewer state already read, so both surfaces change; recovery re-creates only the audio capture, honouring the CONTEXT no-stream-restart decision"
  blind_spots: "real output device silence delivery (M-162) and per-app window-share audio are unmeasured; whether toggling capturesAudio via updateConfiguration re-creates a tap killed by a coreaudiod restart is unverified (needs the VM)"
  candidate_causes:
    - "code: no stall detector on the audio path (confirmed)"
    - "environment: coreaudiod restart kills SCK's audio tap without an SCK error (the trigger; outside our control)"
    - "config: stream config could lack a live audio-reconnect option (SCK has none documented; recovery must be explicit)"
  and_gate: "yes: the visible bug needs the environment trigger (tap dies silently) AND the code gap (no detector); only the code gap is ours to fix"
next_action: orchestrator re-runs VM row A-03 on a build with this fix (see Resolution.vm_recheck); a real-Mac check that stats stays audio ok through a long silent display share belongs with M-162

## Symptoms

expected: an audio fault shows as audio failed in GET /desktop/stats (and the viewer UI's audio-failure surface) while video continues; ideally audio recovers on its own and stats reads ok again (DCTL-07, VM row A-03 conditions 1 and 2)
actual: after `sudo killall coreaudiod` in portty-test (pid 326 -> 32231) with a Safari viewer streaming, viewer audio packetsReceived went 4554 -> 4584 (+8 s) -> 4584 (+18 s) while framesDecoded kept rising 534 -> 609 -> 673; stats read audio ok on all 30 samples; audio returned only after the viewer closed and a new viewer started a fresh capture run
errors: none (SCStream raised no error)
reproduction: VM only (not runnable by this agent): stream to a viewer, `sudo killall coreaudiod`, poll /desktop/stats
started: always (no stall detector ever existed; noteAudioFailed fires only from the capture error callback)

## Eliminated

- hypothesis: recover by restartSource (a fresh capture run, which A-03 showed restores audio)
  evidence: 23-CONTEXT [Audio] locks "no stream restart" for an audio failure and TestAudio_FailureIsStateNotOutage pins one source start; a run restart also drops the picture to a keyframe wait for every viewer
  timestamp: 2026-11-27T00:22:00Z

- hypothesis: a per-packet timestamp (lastAudioAt) compared on each tick is enough
  evidence: the tick compares wall clock (Deps.Now().Round(0) drops the monotonic reading) and Tart guests step their clock (memory: VM clock steps); a forward step would read as a stall while packets flow. Tracking packet-count progress per tick avoids it
  timestamp: 2026-11-27T00:25:00Z

## Evidence

- timestamp: 2026-11-27T00:00:00Z
  checked: evidence/A-03.txt, A-03.stats, 23-VM-CHECKS.md row A-03, 23-09-SUMMARY finding 3
  found: audio recv flat at 4584 from +8 s to +18 s while frames rose; stats audio ok all 30 samples; new viewer (new capture run) got audio again (60 -> 118 in 4 s, ~52 pkt/s ~ 20 ms Opus frames)
  implication: capture stream keeps running for video; audio sample buffers stop with no error; nothing on the Go side notices absence of audio

- timestamp: 2026-11-27T00:10:00Z
  checked: internal/desktop/audio.go audioOut, noteAudioFailed, noteAudioOK; manager.go startSource/stopSourceLocked; stats.go; state.go stateMsg
  found: audioFail has exactly two writers inside a run: an error on AudioErrors sets it, a packet on AudioPackets clears it (startSource resets it per run). Stats().Audio and the state message's audio field both read only audioFail. No code measures time since the last audio packet.
  implication: a silent end of audio buffers (no error) can never set failed; confirmed by reading every writer (grep audioFail: manager.go:347, audio.go:174/183, state.go:251, stats.go:27)

- timestamp: 2026-11-27T00:12:00Z
  checked: internal/screencap/media_darwin.m PCStreamHandler, pc_stream_start; audio_darwin.m pc_audio_handle
  found: audio errors reach Go only from pc_audio_begin failure, addStreamOutput(audio) failure, or a malformed/failed buffer inside pc_audio_handle; didStopWithError is stream-wide (video too). A tap that stops calling didOutputSampleBuffer(type audio) calls nothing.
  implication: the ObjC side cannot report this fault either; detection must be absence-based on the Go side

- timestamp: 2026-11-27T00:15:00Z
  checked: 17-SPIKE-MATRIX R21; 23-CONTEXT [Audio]; evidence/A-01.txt, A-03.txt
  found: R21 control run (no sound, 5 s) delivered 252 buffers (960 frames each, ~50/s) with peak 0 and 3-byte Opus silence packets; A-03 viewer had 4554 packets (~90 s at ~50/s) with no tone playing before the fault, and ~52 pkt/s on the fresh run after; CONTEXT: "silence costs a few bytes per packet". Only the VM virtual sound device was measured (real device = M-162)
  implication: in silence SCK keeps delivering zero buffers, so total absence of packets for a few seconds is a stall, not silence (display capture)

- timestamp: 2026-11-27T00:20:00Z
  checked: web search (Apple forums, dev.to) on SCK audio delivery in silence and per-window audio
  found: no source says SCK stops audio buffers in silence; a real-hardware report saw 3400+ consecutive all-zero buffers over ~70 s on schedule. Window filters (initWithDesktopIndependentWindow) capture audio per app (WWDC22); no data on delivery for a silent app
  implication: display shares can use an absence detector; window shares are unmeasured, so they are left unwatched (shortcut, recorded)

- timestamp: 2026-11-27T00:22:00Z
  checked: 23-CONTEXT line 33; audio_test.go TestAudio_FailureIsStateNotOutage
  found: locked decision "Audio failure becomes a state and the picture keeps streaming; no crash and no stream restart"; the test pins "source started 1 time" after an audio failure
  implication: recovery may not use restartSource (which A-03 showed would restore audio); it must re-create only the audio capture on the live SCStream, which needs one new ObjC entry point (updateConfiguration capturesAudio off, then on; Apple's CaptureSample toggles capturesAudio live the same way)

- timestamp: 2026-11-27T00:25:00Z
  checked: state.go tick/stateLoop, watchdog.go, power.go noteWake/processWake
  found: the state loop ticks every 1 s with viewers (10 ms in tests, fake clock); the video stall watchdog is a tickMu-guarded state machine stepped by the tick and gated by paused/permDenied/asleep/noDisplay/wakeAt; a wake sets wakeAt before the tick and processWake restarts the run
  implication: an audio watchdog stepped by the tick adds no goroutine or timer (nothing to leak) and inherits the same sleep/wake gating; tracking packet-count progress per tick (not timestamps per packet) makes a forward wall-clock step harmless while packets flow

- timestamp: 2026-11-27T18:45:00Z
  checked: go test -race -run 'TestAudio_SilentStallIsFailed|TestAudio_StallRestartBacksOff|TestAudio_SilenceIsNotAStall' on unfixed code
  found: SilentStallIsFailed FAIL (timed out waiting for stats audio failed); StallRestartBacksOff FAIL (no first restart); SilenceIsNotAStall PASS (guard)
  implication: the A-03 symptom is reproduced deterministically with a fake clock and a hand-driven audio source

- timestamp: 2026-11-27T19:00:00Z
  checked: mutants seeded by hand at each fix site (no Stryker for Go), each run then restored
  found: no tick call -> SilentStallIsFailed + StallRestartBacksOff FAIL; no progress reset (n != w.seen) -> SilenceIsNotAStall (3 restarts while packets flow) + BackOff FAIL; no 60 s cap -> BackOff FAIL; no window gate -> WindowShareNotWatched FAIL; bound 2 s instead of 3 s -> SilentStallIsFailed FAIL at the 2.5 s boundary check
  implication: every branch of the new logic is pinned by a test

- timestamp: 2026-11-27T19:20:00Z
  checked: go test -race ./internal/desktop/ ./internal/daemon/ (full); -count=10 of the four new tests plus TestAudio_FailureIsStateNotOutage; -count=5 -cpu=1 of the four new tests; go vet desktop/daemon/screencap; gofmt; go test -run '^$' ./internal/screencap/ (compile only: its input/pasteboard tests are not run by this agent)
  found: all ok, no data race, no known flake hit; vet and gofmt clean; screencap test binary compiles with the new cgo entry point
  implication: no regression in the import graph of the changed files

## Resolution

root_cause: "Manager.audioFail (the only thing GET /desktop/stats and the viewers' state message read for audio) had exactly two writers inside a capture run: an error on AudioErrors and a packet on AudioPackets. When coreaudiod restarts, ScreenCaptureKit's audio tap stops calling didOutputSampleBuffer(audio) and raises nothing (no audio error, no didStopWithError, the picture continues), so neither writer ever fired; and nothing re-created the tap except a new capture run. AND-gate: environment trigger (SCK tap dies silently) + code gap (no absence detector, no audio-only restart)."
fix: "audioWatchStep, stepped by the existing state-loop tick (no new goroutine or timer): it tracks the per-run audio packet count (Manager.audioPkts, bumped in audioOut) against the run identity m.audioDone; on a display share with no new packet for audioStallAfter (3 s) and no sleep/wake/pause/permission/no-display block, it calls noteAudioFailed (stats audio failed, state message audio failed, viewer greys Sound) and, through the new optional screencap.AudioRestarter, DisplaySource.RestartAudio -> pc_audio_restart (capturesAudio off then on via updateConfiguration on the live SCStream, 5 s bound each, gConfig left with audio on), retried after 5, 10, 20, 40 s then every 60 s. The first packet after it clears the failure through the existing noteAudioOK. The stream, the picture and the viewers are never restarted (CONTEXT [Audio])."
oracle_type: specified (DCTL-07 / A-03 condition 1 and 2: stats audio failed while the picture continues, ok again on recovery; CONTEXT: no stream restart)
verification:
  target_test: { result: pass, tests: [TestAudio_SilentStallIsFailed, TestAudio_StallRestartBacksOff] }
  mutation_check: { result: pass, reason_if_skipped: "no Stryker for Go; 5 hand-seeded mutants at the fix sites, all killed", mutant_killed: true }
  no_op_deletion: { result: pass, deletion_justified_by_rca: n/a (additive diff: 2 lines changed, both comment or alignment) }
  adjacent_tests: { result: pass, suites_run: [./internal/desktop (full, -race), ./internal/daemon (full, -race), screencap compile-only] }
  revert_and_reconfirm: { result: pass, bug_returned_on_revert: true, fixed_on_reapply: true }
  guardrail_verdict: accepted
  not_verified_here: "the ObjC pc_audio_restart never ran (no capture allowed for this agent; DisplaySource.Start refuses under tests). Whether toggling capturesAudio re-creates a tap killed by a coreaudiod restart is for the VM re-run"
vm_recheck: "A-03 re-run on a build with this fix: before the fault stats audio ok; within about 3 to 4 s of `sudo killall coreaudiod` (3 s bound + up to a 1 s tick) stats reads audio failed while framesDecoded keeps rising, viewers 1, no new capture run (no 'viewer joined', no keyframe wait); daemon log: 'desktop: audio failed' with err 'no audio from the capture for 3s', then 'desktop: restarting audio capture' retry_in=5s. Recovery: if the capturesAudio toggle works, 'desktop: audio recovered' and stats audio ok within about 1 s of the first restart (packetsReceived climbing about 50/s again); if it does not, further restarts log at about +5, +15, +35, +75, +135 s after the first, stats stays failed and video is unaffected (record the toggle as ineffective: next step would be re-adding the audio stream output or accepting failed-until-next-run)"
files_changed: [internal/desktop/audio.go, internal/desktop/manager.go, internal/desktop/state.go, internal/desktop/audio_stall_test.go, internal/screencap/audio.go, internal/screencap/media_darwin.go, internal/screencap/media_darwin.m]
vm_verified: "2026-11-28, build 23.10.2 (d11e1811), VM row A-03 re-run (23-VM-CHECKS.md, evidence/A-03-rerun.txt): sudo killall coreaudiod in portty-test with an unmuted home viewer; daemon.log audio failed (no audio from the capture for 3s) 4.2 s after the command, restarting audio capture retry_in=5s, audio recovered 0.57 s later (one restart); stats failed for one sample then ok; viewers 1, state streaming at all 200 samples, no new viewer joined; audio packets back to about 50/s, framesDecoded rising through the fault. The audio-only restart is effective after a coreaudiod restart. A-03 RESULT=PASS."
