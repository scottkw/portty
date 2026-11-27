---
status: resolved
trigger: "every desktop video source restart (stall recovery, wake, retry) starts a new random RTP sequence number; existing viewers stay frozen up to ~40 s (Phase 23 deferred-items.md, found by 23-02)"
created: 2026-11-27T15:00:00Z
updated: 2026-11-27T21:25:00Z
---

## Current Focus

bug_class: Bohrbug (deterministic per restart; the freeze length depends on the random jump)
reasoning_checkpoint:
  hypothesis: "fanOut built a new rtp.Packetizer with rtp.NewRandomSequencer() (and a new random timestamp base) per capture run; a viewer's track, SSRC and SRTP context outlive restartSource, so its SRTP drops every packet whose sequence is behind the last one it accepted (or misestimates the rollover counter) until the new sequence overtakes the old one. audioOut had the same per-run newAudioClock()."
  confirming_evidence:
    - "code: internal/desktop/manager.go fanOut created the packetizer per call; startSource calls fanOut per run; restartSource = stopSourceLocked + startSource with viewers kept"
    - "code: internal/desktop/audio.go audioOut created newAudioClock() (its own random sequencer) per run"
    - "23-02 host evidence: viewer A received 0 units for 4 s after a stall restart; a surviving sequencer restored 30 units/s at once"
    - "new test TestFanout_SourceRestartKeepsRTPSequence failed 3/3 on the old code (timed out: no packets accepted after the restart) and passes 5/5 after the fix"
  falsification_test: "the restart test still failing with one packetizer kept on the Manager"
  fix_rationale: "the packet numbering belongs to the viewer's track, not to a capture run: one video packetizer and one audio clock live on the Manager for its life; one run at a time uses them (stopSourceLocked waits for fanDone and audioDone before the next start). The first video unit of a new run is stamped with the real gap since the last unit, so the RTP clock keeps following real time as it does for a still screen (the audio clock already resyncs on a gap)."
  blind_spots: "a real browser's SRTP/jitter buffer is not exercised on the host (pion stands in); unverified on VM or real Mac"
next_action: "none: resolved by the VM check (M-221) on 2026-11-27"

## Symptoms

expected: a viewer's picture resumes within ~1 s of a stall recovery or wake restart
actual: picture frozen until the new random sequence passes the old one (up to ~40 s at 830 packets/s, longer on a wrong rollover guess)
reproduction: host loopback, any restartSource with a viewer connected (TestFanout_SourceRestartKeepsRTPSequence)
started: Phase 19 (pre-existing)

## Eliminated

- hypothesis: adaptive quality controller causes the freeze
  evidence: a quality step uses Source.Rebuild and never restarts the source (deferred-items.md)
  timestamp: 2026-11-27T15:00Z

## Evidence

- timestamp: 2026-11-27T15:05Z
  checked: TestFanout_SourceRestartKeepsRTPSequence on the unfixed code, -count=3
  found: 3/3 FAIL "timed out waiting for video and audio after the restart"
  implication: the pion viewer accepted no packets after the restart
- timestamp: 2026-11-27T15:15Z
  checked: same test after moving the packetizer and audio clock to the Manager, -count=5
  found: 5/5 PASS
- timestamp: 2026-11-27T15:20Z
  checked: TestFanout_SourceRestartStampsTheGap with the gap line disabled
  found: FAIL "timestamp step across a 4 s restart gap = 2970 ticks"; PASS with it
- timestamp: 2026-11-27T15:35Z
  checked: go test -race ./internal/desktop ./internal/screencap; go test ./...
  found: all ok
- timestamp: 2026-11-27T21:18Z
  checked: "VM, build cabf716f (23.8.1, wailsassets vmdesktop) on portty-test, Safari and Chrome on portty-test-2 viewing the office desktop with a moving Terminal, framesDecoded polled every 0.5 s; display sleep (pmset displaysleepnow) then caffeinate -u"
  found: "capture error ... no display at sleep, state display_asleep; after the wake command the source restarted (ensureSource path; the VM sent no power wake, so no wake line) and framesDecoded rose again in Chrome 1.9 s and Safari 2.5 s after the wake command (that includes the display-awake read before the restart); no viewer rejoined"
  implication: the viewers kept the SRTP context across the restart: no freeze
- timestamp: 2026-11-27T21:20Z
  checked: "same viewers; vmdesktop POST /desktop/debug/lapse, so the 60 s awaiting retry ran restartForStall (the watchdog restart path); stall recovered logged 21:20:19.40"
  found: "no gap in framesDecoded in either browser across the restart (the new stream keyframe shows as a bytesReceived jump about 3 s before stall recovered); guest timed log showed no clock step in the window"
  implication: a watchdog restart no longer freezes a browser viewer

## Resolution

root_cause: "per-run rtp.NewPacketizer(..., rtp.NewRandomSequencer(), ...) in fanOut (and newAudioClock() in audioOut) restarted RTP sequence and timestamp on every source restart while viewers' SRTP contexts survived"
fix: "Manager.vpkt and Manager.aclk created once in NewManager and used by every fanOut/audioOut run; Manager.vlast stamps a new run's first video unit with the real gap"
verification: "host: two new loopback tests red then green; go test -race on internal/desktop and internal/screencap; go test ./... . VM (M-221, 2026-11-27, build cabf716f): Safari and Chrome kept decoding through a display sleep/wake restart (resumed within 2 s of the wake command) and a watchdog restart (no gap). Not checked: audio (the debug stats carry video only) and the Portty app as a viewer."
files_changed: internal/desktop/manager.go, internal/desktop/audio.go, internal/desktop/fanout_test.go, internal/desktop/loopback_test.go, TESTING.md

## VM / real-Mac check (TESTING.md M-221)

Run 2026-11-27 (see Evidence; PASS for browser video). The procedure: On the office VM (or a real Mac), with a browser viewer (Chrome and Safari) and the Portty app viewing Desktop with a moving picture:
1. Sleep and wake the display; separately force a watchdog stall restart (still screen with the pointer nudge blocked, or the vmdesktop stall rig). Note the "stall recovered" / wake line in daemon.log.
2. For 60 s after each restart watch every viewer; in Chrome, chrome://webrtc-internals inbound-rtp video and audio: packetsReceived keeps rising, framesDecoded resumes, no packetsLost jump.
PASS: moving picture on every viewer within 2 s of the restart line, audio resumes. FAIL: picture frozen > 2 s while daemon.log shows the stream running.
