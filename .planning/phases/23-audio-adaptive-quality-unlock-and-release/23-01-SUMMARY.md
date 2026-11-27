---
phase: 23-audio-adaptive-quality-unlock-and-release
plan: 01
subsystem: desktop-audio
tags: [opus, audiotoolbox, screencapturekit, pion, webrtc, audio]
requires:
  - phase: 20-remote-control
    provides: media_darwin stream switch (20-03), input path (20-04)
  - phase: 22-click-to-share-for-tailnet-people
    provides: guest serve path (22-04), rights (22-05)
provides:
  - screencap.AudioSource optional interface, AudioPacket, ErrAudioUnavailable, Opus constants
  - system audio on the one SCStream, encoded to Opus by AudioConverter (64000 bps read back)
  - stereo Opus track per viewer in stream portty-desktop, host-clock RTP timestamps
  - state field audio=failed, hello caps audio 1, three golden lines
affects: [23-06 viewer unmute and disclosure copy, 23-09 VM rig audio rows, 23-12 TESTING.md, 23-14 M-162]
status: complete
actuals:
  tokens: 16500
  tasks: 3
  commits: 4
plan_head_before: 4004dae81c7840efaa29f5b599bf83ac50afd87f
plan_head_after: 2057d0d940373007432bb0bee05092226f86fe88
tech-stack:
  added: []
  patterns:
    - "optional interface (AudioSource) so every existing fake Source keeps compiling"
    - "audioOut on its own goroutine, no lock shared with the video fan-out while writing"
key-files:
  created:
    - internal/screencap/audio.go
    - internal/screencap/audio_darwin.go
    - internal/screencap/audio_darwin.m
    - internal/screencap/audio_darwin_test.go
    - internal/desktop/audio.go
    - internal/desktop/audio_test.go
  modified:
    - internal/screencap/media_darwin.go
    - internal/screencap/media_darwin.m
    - internal/desktop/peer.go
    - internal/desktop/manager.go
    - internal/desktop/state.go
    - internal/desktop/guest_viewer.go
    - internal/desktop/testdata/signal-v1.jsonl
    - internal/desktop/protocol_test.go
    - internal/desktop/export_test.go
    - internal/desktop/loopback_test.go
    - internal/desktop/fanout_test.go
    - internal/desktop/guest_session_test.go
key-decisions:
  - "Audio RTP timestamps follow the host wall clock at packet arrival, not SCK presentation timestamps (recorded deviation, see below)"
  - "The answer SDP's Opus fmtp is completed to stereo=1;sprop-stereo=1 by text rewrite, because pion echoes the offer's fmtp"
  - "AudioConverter accepted 64000 bps (read back 64000); the 96000 fallback exists but was not needed"
requirements-completed: [DCTL-07]
---

# Phase 23 Plan 01: Opus audio capture and encode Summary

The Mac's system audio now leaves the daemon as stereo Opus (AudioConverter, 64 kbps, 20 ms packets) on the same ScreenCaptureKit stream as the picture, and reaches every viewer that offers an audio m-line as a second track in the same MediaStream. An audio fault is a state field and never touches the picture. The viewer half (muted start, Unmute) is 23-06.

## What was built

- **screencap/audio.go** (cross-platform): `AudioPacket{Data, Samples}`, optional `AudioSource{AudioPackets, AudioErrors}`, `ErrAudioUnavailable`, `OpusFrames=960`, `OpusRate=48000`, `OpusBitrate=64000`.
- **screencap/audio_darwin.m and .go**: `pc_opus_new` (AudioConverter to `kAudioFormatOpus`, bit rate set and read back, 96000 fallback), `pc_opus_encode` (960-frame accumulation, a partial block is held), the SCK audio sample handler `pc_audio_handle` (planar or interleaved float32 stereo 48 kHz, errors reported once until a good buffer follows), `goScreencapAudioPacket` and `goScreencapAudioError` exports, and the host-safe test hook `encodeOpusForTest` / `encodeOpusChunksForTest` (refuses outside a test binary).
- **screencap/media_darwin.m and .go**: `pc_stream_start` sets `capturesAudio`, `sampleRate 48000`, `channelCount 2`, `excludesCurrentProcessAudio YES` and adds an `SCStreamOutputTypeAudio` output on its own serial queue. If the encoder or the audio output cannot be made, the error goes to `AudioErrors` and the stream starts without audio. `pc_stream_stop` drains the audio queue before freeing the encoder. `DisplaySource` implements `AudioSource` with buffered channels (16 packets, 4 errors); a full channel drops audio and never blocks capture.
- **desktop/audio.go**: `opusCapability` (opus/48000/2, `minptime=10;useinbandfec=1;stereo=1;sprop-stereo=1`), `audioClock`, `audioOut`, `writeAudio`, `noteAudioFailed` / `noteAudioOK`, `ensureOpusStereo`, `offersAudio`.
- **desktop/peer.go**: Opus registered at payload type 111 (`desktop: register opus`); `answer` adds an audio `TrackLocalStaticRTP` (id `audio`, stream `portty-desktop`) plus its RTCP read loop before `SetRemoteDescription`, only when the viewer's hello caps had audio 1 and the offer has a live audio m-line.
- **desktop/manager.go, state.go, guest_viewer.go**: `viewer.audio` and `viewer.audioCap`; `startSource` starts `audioOut` beside `fanOut` when the source is an `AudioSource`, `stopSourceLocked` joins it; server hello caps gain `audio: 1` for own-account viewers and guests; `msgState.Audio` (`json:"audio,omitempty"`) via one `stateMsg` helper shared by `broadcastState` and `pushIfChanged`.
- **Golden lines**: `hello-viewer-audio`, `hello-server-audio`, `state-audio-failed` in `signal-v1.jsonl`; `ProtocolVersion` stays 1.

## Results the plan asked to be recorded

- **Read-back Opus bit rate: 64000.** AudioConverter accepted 64000 (RESEARCH assumption A1 held on this Mac); the 96000 fallback was not exercised. 1 s of 440 Hz stereo sine gives exactly 50 packets, 1 s of silence 50 packets with a median under 16 bytes. No priming packet is swallowed.
- **A/V timestamp deviation (reversible, wall clock instead of SCK PTS).** CONTEXT [Audio] says both tracks are stamped from SCK presentation timestamps. This plan stamps audio from the host wall clock at packet arrival, because the video track already stamps capture time from wall-clock arrival (`packetize`, WR-02, pinned by `TestFanout_RTPTimestampFollowsCaptureTime`) and sender-report lip sync needs both tracks on the same clock. Audio uses a 48 kHz sample counter advanced 960 per packet and resynced to wall time when it drifts past 60 ms: forward drift is corrected with `SkipSamples`, backward drift (a burst arriving faster than real time) moves the base instead because RTP timestamps never go back. Arrival trails the PTS by the capture and encode delay only. Route back to the letter of the decision: carry the `CMSampleBuffer` PTS on `AudioPacket` and `AccessUnit` if 23-09 or M-162 shows an audible offset. Known edge: after a late-arrival burst the counter stays ahead of arrival time by the burst size for the rest of that run (bounded by the 60 ms trigger); only a later gap, not time, re-anchors it.
- **Precondition git log** (`git log --oneline 1c55b159..HEAD -- peer.go manager.go media_darwin.go media_darwin.m`): 88e098af (22-05 lease and press gate), 76bc8b51 (22-05 revoke and Stop), b570ae27 (22-05 control and clipboard switches), a4fb5d09 (22-04 guest Admit), f93c44dc (quick-261127-bb7 encoder rebuild keeps old session), d47bd7eb (quick-261127-bb7 input channel close), 47317b21 (M-07 autorelease pools), efcc60d2 (M-04 clamped moves), 1dc75276 (M-01 input mapping on the real surface), a8c3ceb0 (H-01 release held input on quit). All four hard-gate SUMMARYs (20-03, 20-04, 22-04, 22-05) existed and `RegisterDefaultInterceptors` was present once in peer.go.
- **Assumption rows that did not hold exactly.**
  - "answer adds one track before SetRemoteDescription" held, and audio is added the same way.
  - "Server hello caps are built in the serve path": there are two construction sites now (`ServeDesktopViewerAs` and the 22-04 guest hello in `guest_viewer.go`); both gained audio 1. The guest hello still has no unlock cap (23-04 owns that).
  - pion's answer does not use the registered codec's fmtp. In v4.2.29 `updateFromRemoteDescription` pushes the *remote* codec on a partial match, so the answer mirrors the offer's `minptime=10;useinbandfec=1;stereo=1` and never has `sprop-stereo=1`. The plan truth requires both, so `ensureOpusStereo` rewrites the answer text returned to the viewer (pion's own state is untouched). This is the only SDP text edit in the daemon.

## New test files for 23-12 (TESTING.md Sections 2 and 4)

- `internal/screencap/audio_darwin_test.go` (`TestOpus_SineMakesFiftyPackets`, `TestOpus_SilenceIsSmall`, `TestOpus_PartialBlockIsHeld`; AudioConverter on synthetic PCM, host-safe, darwin only)
- `internal/desktop/audio_test.go` (`TestAudio_TracerLoopback`, `TestAudio_NoAudioOffer`, `TestAudio_ClockStepsAndResync`, `TestAudio_ClockDriftLimit`, `TestAudio_FailureIsStateNotOutage`, `TestAudio_GuestHearsAudio`, `TestAudio_AnswerFmtp`)
- `TestAudio_GoldenLines` lives in the existing `internal/desktop/protocol_test.go`.
- Traceability rows (DCTL-07): `internal/screencap/audio_darwin_test.go`, `internal/desktop/audio_test.go`, `internal/desktop/protocol_test.go`.
- Manual item for Section 5 (cannot be automated here, this is M-162's neighbour): on a real Mac with a real output device and a real browser viewer, play a left-only test tone and confirm the viewer plays stereo (not downmixed to mono), that audio and picture are within about 100 ms, and that silence on the Mac sends only a few bytes per packet. Live SCK audio capture was never run on this host by design.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Task 1 carries the audio state and clock code that Task 3 describes**
- **Found during:** Task 1
- **Issue:** `audioOut` needs `noteAudioFailed`, `noteAudioOK` and the 960-per-packet clock to compile and to stamp RTP correctly, so those landed in the Task 1 commit with `msgState.Audio`. Task 3's tests (clock, drift limit, failure-as-state, golden lines) were therefore green on first run, not red-then-green. Task 2's RED commit (`c0636c0a`, tests that did not compile) preceded its GREEN commit as normal.
- **Fix:** none needed; recorded for the TDD gate reader.
- **Commit:** 0467877b

**2. [Rule 3 - Blocking] `ensureOpusStereo` added (answer fmtp)**
- **Found during:** Task 1 first test run
- **Issue:** the plan truth requires the answer to carry `stereo=1` and `sprop-stereo=1`; pion mirrors the offer's fmtp (see Assumption rows).
- **Fix:** text rewrite of the Opus fmtp line in the returned answer, unit-tested by `TestAudio_AnswerFmtp`.
- **Files modified:** internal/desktop/audio.go, internal/desktop/peer.go
- **Commit:** 0467877b

**3. [Rule 2 - Missing critical] Guest hello and guest viewer record also gain audio**
- **Found during:** Task 3
- **Issue:** the plan names `manager.go` for the hello, but guests have their own hello in `guest_viewer.go` (22-04). Without it a guest would never be offered audio, contradicting "guests hear audio like the picture".
- **Fix:** audio 1 in the guest hello and `audioCap` on the guest viewer; `TestAudio_GuestHearsAudio`.
- **Files modified:** internal/desktop/guest_viewer.go
- **Commit:** 2057d0d9

**4. [Rule 3 - Blocking] `audioOut` takes the source context**
- **Issue:** the plan signature is `audioOut(src, done)`. A source whose audio channels were nil would block `stopSourceLocked` (held under `m.mu`) forever, since `fanOut`-style "channel closes on Stop" is the only exit.
- **Fix:** `audioOut(ctx, src, done)` also returns on the source context that `stopSourceLocked` cancels first.
- **Commit:** 0467877b

**5. Existing test expectations and helpers touched**
- `TestHello_RightsAndInputCap` and the hello-caps test in `fanout_test.go` pinned caps to exactly `{video, input}`; they now pin `{video, input, audio}`.
- `loopbackClient` gained an audio branch in `OnTrack`, an audio packet channel, stream-id fields and an offer `munge` hook; `joinGuestAs` became a thin wrapper over a new `joinGuestCaps`; `fanViewer` records the state message's audio field. No existing assertion was weakened.

## Known Stubs

None.

## Threat Flags

None. The only new surface is the audio RTP track inside the existing DTLS-SRTP PeerConnection and an additive state field. T-23-01 (audio to muted viewers and guests) is accepted per CONTEXT and disclosed in 23-06 Task 3; T-23-02 mitigated (recover in every export, non-blocking sends, `audioOut` shares no write-time lock with video, failure is a state); T-23-03 mitigated (only `SCStreamConfiguration.capturesAudio` on the existing stream, no `AVCaptureDevice`).

## Not verified here (deferred, by the plan's host-safety rules)

- Live ScreenCaptureKit audio capture, the real output device, and `excludesCurrentProcessAudio` behaviour: needs a real Mac (M-162) or the VM rig in 23-09. Nothing here launched Portty, captured the screen or audio, or touched the tailnet.
- Stereo actually decoding as stereo in Chrome/Safari (Pitfall 8) and the A/V offset: browser-side, 23-06 and 23-09.
- The audio-queue drain order in `pc_stream_stop` and the failure paths of `pc_stream_start` (encoder fails, audio output refused) are reviewed but only reachable with live capture; they compile clean under `-Wall -Wextra`.

## Verification run

- `GOTOOLCHAIN=go1.26.11 go vet ./internal/desktop/ ./internal/screencap/`: clean; `gofmt -l`: empty.
- `golangci-lint run --new-from-rev=1c55b159 ./internal/desktop/... ./internal/screencap/...`: 0 issues.
- `go test -race -count=3 -run 'TestAudio_|TestProtocol' ./internal/desktop/` and `-run TestOpus_ ./internal/screencap/`: pass.
- `go test -race -count=1 ./internal/desktop/ ./internal/screencap/`: pass (desktop 90 s, screencap 2 s). `go build ./...`: ok.
- `bash tests/check-brand.sh`: clean before every commit.

## Self-Check: PASSED

All created files exist (audio.go in both packages, audio_darwin.go/.m/_test.go, audio_test.go, this SUMMARY) and commits 0467877b, c0636c0a, 760e4701, 2057d0d9 are ancestors of HEAD.
