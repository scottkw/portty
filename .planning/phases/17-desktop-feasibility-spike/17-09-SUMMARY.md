---
phase: 17-desktop-feasibility-spike
plan: 09
subsystem: desktop-spike
tags: [opus, audioconverter, screencapturekit, system-audio, replayd, screen-recording-approval, tart, feas-01, r19, r21]

requires:
  - phase: 17-desktop-feasibility-spike
    provides: "17-06 guest state (portty-test with ~/ptest/Spike.app and its grant), the matrix with R19 and R21 PENDING, the spike's probe-loop --stream"
provides:
  - "spike opusprobe: AudioConverter Opus encode of a generated sine (host-safe) or of ScreenCaptureKit system audio, spk_opus_sine and spk_audio_stats, SCK audio output in spk_sck_start"
  - "R21 GO: Opus encodes at 960 frames per packet on the host and in the guest, and SCK delivers system-audio buffers in the guest (501 buffers in 10 s, peak 0.297, silent control peak 0)"
  - "R19 PARTIAL: the approval store layout, that the monthly prompt is the picker-bypass dialog raised on a fresh capture start with a 31-day-old entry, that frames keep flowing while it is pending, that a replayd restart silently freezes a running stream, and what a zero-frame check does and does not catch"
affects: [17-10, 17-11, 17-12, DPRM-03, DCTL-07]

actuals:
  tokens: 25700
  tasks: 3
  commits: 4
plan_head_before: 55ba99c3db1f14481c5e41c0ad7e764beaf4d997
plan_head_after: 818b702e1ff2238fc54ac3d564e28840a595d2ea

tech-stack:
  added: []
  patterns: ["AudioToolbox AudioConverter to kAudioFormatOpus, one 960-frame packet per FillComplexBuffer call, framesPerPacket read back from the converter", "SCStreamOutputTypeAudio on its own serial queue feeding the same encoder, with a peak-sample reading to tell silence from sound", "a tart clone driven through Virtualization.framework VNC (one-shot RFB client with the DesktopSize pseudo-encoding) when the clone has no TCC grants for the ssh chain"]

key-files:
  created:
    - .planning/phases/17-desktop-feasibility-spike/evidence/r21-opus.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r19-approvals.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r19-stream.jsonl
    - .planning/phases/17-desktop-feasibility-spike/deferred-items.md
  modified:
    - spike/desktop/media_darwin.m
    - spike/desktop/bridge_darwin.go
    - spike/desktop/probe.go
    - spike/desktop/main.go
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md

key-decisions:
  - "R21 is GO for both parts: AudioConverter makes Opus (C7 holds, libopus not needed) and SCK delivers real system audio in the guest, so DCTL-07 stays"
  - "R19 is PARTIAL, not GO: the store and the backdated behaviour are documented, but the real 30-day prompt was not observed and a mid-stream expiry is unknown"
  - "The zero-frame heuristic is not enough for DPRM-03: it misses the expired approval (frames flow under the prompt) and, without a pointer nudge, cannot tell a dead stream from a still screen"

patterns-established:
  - "A control run next to every attribution claim: the replayd-restart-only control showed the freeze is not caused by the aged entry"

requirements-completed: []

coverage:
  - id: D1
    description: "AudioConverter Opus encoder, SCK system-audio output and the opusprobe subcommand"
    requirement: "FEAS-01"
    verification:
      - kind: integration
        ref: "Task 1 verify block: vet, gofmt, golangci-lint, build.sh, opusprobe --source sine --seconds 5 (framesPerPacket 960, 250 packets), the two release tests, check-brand"
        status: pass
    human_judgment: false
  - id: D2
    description: "R21 in the guest: Opus encode and system-audio capture"
    requirement: "FEAS-01"
    verification:
      - kind: integration
        ref: "Task 2 verify block (framesPerPacket and audio buffers in r21-opus.txt, matrix row R21 with a result)"
        status: pass
    human_judgment: true
    rationale: "The guest has only a virtual output device; whether capture is the same with the office Mac's real device is a follow-up in the host run"
  - id: D3
    description: "R19 in the disposable off-tailnet clone: approval store, backdated approval, replayd restart, zero-frame question"
    requirement: "FEAS-01"
    verification:
      - kind: integration
        ref: "Task 3 verify block (store named, zero-frame answered, rig and clone-tailnet lines, ten write[spike-r19] lines all on the clone, stream records, matrix row R19 PARTIAL with follow-up, no user path, clone gone from tart list)"
        status: pass
    human_judgment: true
    rationale: "A backdated entry in a clone is the strongest available evidence short of waiting a month; whether the DPRM-03 design should rely on a nudge-based liveness check is a design judgment"

duration: 33min
completed: 2026-11-25
status: complete
---

# Phase 17 Plan 09: Opus audio and the monthly re-approval rows Summary

**AudioConverter encodes Opus at 960 frames per packet on the host and in the guest and ScreenCaptureKit delivers real system audio there (R21 GO); in a deleted off-tailnet clone the monthly prompt turned out to be the picker-bypass dialog raised on a fresh capture start with a 31-day-old approval, frames keep flowing while it waits, and a replayd restart freezes a running stream silently, so a zero-frame check cannot see the expiry (R19 PARTIAL).**

## Performance

- **Duration:** 33 min
- **Started:** 2026-11-25T04:35:00Z (approximate)
- **Completed:** 2026-11-25T05:08:15Z
- **Tasks:** 3
- **Files:** 4 spike sources modified, 4 evidence or planning files created, the matrix modified

## Accomplishments

- **Opus encoder and `opusprobe` (Task 1).** `spk_opus_sine` feeds a 440 Hz tone through AudioConverter (48 kHz float32 stereo to `kAudioFormatOpus`, 960 frames per packet, 96 kbps) and returns JSON with `packets` and `framesPerPacket` always present; `framesPerPacket` is read back from the converter, not assumed. `spk_sck_start` with audio adds a `SCStreamOutputTypeAudio` output on its own queue (own process excluded), converts the planar float buffers to interleaved stereo and feeds the same encoder; `spk_audio_stats` reports buffers, frames, packets, bytes, kbps, first-buffer ms, errors, the peak sample and the buffer format. `opusprobe --source sine|sck` writes its record before it fails on a zero-packet run. Host run (macOS 27.2.1): 250 packets for 5 s, 960 frames per packet, no TCC call.
- **R21 GO (Task 2).** In the guest: sine encode 500 packets in 10 s, same numbers as the host (55 kbps for a sine at the 96 kbps target, 312 prime frames, 442-byte first packet with TOC 0xfc). With a sound looping, SCK delivered 501 audio buffers (480960 frames, 48 kHz float32, non-interleaved) in 10 s, first buffer after 524 ms, peak 0.297, 501 Opus packets, 0 errors; a control with no sound gave 252 buffers, peak 0 and 3-byte silence packets. The guest's only device is the Apple Virtual Sound Device.
- **R19 PARTIAL (Task 3), measured only in the clone `spike-r19`.**
  - Store: `~/Library/Group Containers/group.com.apple.replayd/ScreenCaptureApprovals.plist`, key is the app bundle as a file URL, five items per entry (`UsageCount`, `LastAlerted`, `LastUsed`, `PrivacyHintDate` = `LastAlerted` + 30 days, `PrivacyHintPolicy` 2592000). Readable over SSH only because `sshd-keygen-wrapper` has Full Disk Access in the clone; Terminal, without it, was refused with no prompt.
  - Refresh: each capture start moves `LastUsed` and `UsageCount`, never `LastAlerted`, so use does not push the prompt away.
  - Backdate (31 days) then a fresh start: the "bypass the system private window picker" prompt appeared at once, frames kept flowing under it (1037 Complete in 65 s, 0 Blank, no error), the dialog stays after the process exits, and replayd reset the entry to now.
  - Backdate on a running stream: nothing for 76 s. replayd restart on a running stream: the stream freezes (running true, no error, 0 Complete and 0 Idle frames for 100 s or more), and the replayd-restart-only control behaves the same with a valid entry, so the freeze is the restart, not the expiry.
  - Zero-frame check: misses the expired approval, catches a replayd restart but cannot tell it from a still screen (still screens also stop after about 6 s) without a nudge.
- **Cleanup.** Clone stopped and deleted, `portty-test` still running with its clock within a second of the host, `portty-test-2` still stopped, no Screen Sharing window left on the host.

## Task Commits

1. **Task 1: AudioConverter Opus encoder, SCK system-audio output and opusprobe** - `c55ef707` (feat)
2. **Task 2: R21 GO** - `a666d68d` (feat)
3. **Task 3: R19 PARTIAL** - `1dc3f379` (feat)
4. **Deferred item note** - `818b702e` (docs)

**Plan metadata:** committed with this summary (docs).

## Deviations from Plan

### Plan adjustments

**1. [Rule 3 - Blocking] The clone could not be driven with the click tool, axdump or screencapture.** In a vanilla clone no Screen Recording, Accessibility or PostEvent grant exists for the ssh chain (only Full Disk Access for `sshd-keygen-wrapper`), so `screencapture` over ssh fails and the click tool would be inert, and the first grant needs the GUI. Fix: run the clone with `tart run --vnc-experimental` and drive it with a one-shot RFB client kept in the scratchpad (not in the repo). The first client crashed the VNC server (it needs the DesktopSize pseudo-encoding), which hard-stopped the clone once; it was restarted and nothing in it was lost beyond `/tmp`. `tart` opens a Screen Sharing window on the host when it starts a VNC server; that window was closed within seconds both times (a brief host window, no TCC prompt). Whether it tried to connect to the clone's port is not known; it was never used. All typed credentials were the clone's local admin/admin, inside the clone.

**2. [Judgment] Step order for the running-stream test.** The plan backdates first and then starts the stream. A stream that starts after the backdate is a fresh start, so I started the stream under a valid entry and backdated mid-run, and kept the plan's order for the fresh-start test (R19-b).

**3. [Rule 2 - Missing critical measurement] Controls and an animated screen.** The first running-stream run used a still screen and showed zero frames before any write, which is the R22 still-screen effect and says nothing about approvals. Added: an animated screen (a Terminal window printing the date every 50 ms) for the later runs, a replayd-restart-only control (R19-c1), a backdate-only run (R19-d) and a non-FDA read of the store. The still-screen run stays in `r19-stream.jsonl` as label `R19`, marked as unusable for the approval question in `r19-approvals.txt`.

**4. [Judgment] Clock jump not used.** The plan allows it only if the store cannot be written; plutil wrote it, so the clone's clock was never changed.

**5. [Judgment] Stream records slimmed.** `r19-stream.jsonl` keeps label, time, `sr_preflight`, `streamRunning`, the per-record status delta and any stream error (274 records over five labelled runs). The full probe records carry the session user name and UUID, which the evidence rules say to redact; the slim form leaves them out.

**6. [Rule 2 - Extra measurement] `peak` in `spk_audio_stats`.** Not in the plan's list of keys; without it a silent buffer stream could not be told from real system audio.

### Pre-existing issue (not fixed, logged)

17-06's R15, R16 and R17 rows (and sub-rows) were written into the `## Routing` table instead of `## Rows`; the real Rows entries still read PENDING. See `deferred-items.md`. 17-09's R19 and R21 edits are in the Rows table.

**Total deviations:** 6 plan adjustments (1 blocking fix, 2 added measurements, 3 judgment calls), 1 pre-existing issue logged. **Impact:** none on the R19 and R21 answers; the clone work took longer.

## Authentication Gates

None.

## Issues Encountered

- **Mid-stream expiry is unknown.** Backdating a running stream did nothing in 76 s and a replayd restart froze it regardless of the entry, so whether replayd raises the prompt in the middle of a stream when the 30 days pass while it runs was not observed. Follow-up on the real Mac.
- **The store is not an ordinary app's detection route.** It needs Full Disk Access; a non-FDA app is refused silently. DPRM-03 cannot assume a store read.
- **The detection signal for the prompt itself is open.** The dialog is a window; whether the capture host can see it (window list, menu bar capture indicator) was not measured. The menu bar capture indicator was seen to disappear in the frozen runs, by eye only.
- **Audio capture in the guest uses a virtual device.** The follow-up in the R21 row asks the host run (17-11) to confirm with the Mac's real output device.

## Known Stubs

None.

## Threat Flags

None new. T-17-20 held: ten writes, all logged as `write[spike-r19]:`, all in the clone, which had no Tailscale install (three checks passed before the first write) and was deleted; `portty-test`'s clock was within a second of the host's at the end. T-17-21 held: the approvals store was never read on the host; the one non-FDA read ran in the clone. T-17-37 held: only the spike's entry exists in the store and was recorded, user paths are `/Users/USER/`, no session name or UUID in the evidence.

## Next Phase Readiness

Ready for 17-07 (wave 8): `portty-test` is running with the rebuilt `~/ptest/Spike.app` (this plan's build, grant intact after the swap, a third R10 data point with no prompt), `portty-test-2` is stopped and was not started, the clone is gone. The `opusprobe` verb and the audio path are in the spike. The R19 and R21 follow-ups (real 30-day prompt on the office Mac; real output device in the host run) need M-NN numbers from 17-12. FEAS-01 and FEAS-02 stay open: only 17-12 closes them. 17-12 should also read `deferred-items.md` before it consolidates the matrix.

## Self-Check: PASSED

- Files: `media_darwin.m`, `bridge_darwin.go`, `probe.go`, `main.go`, the matrix, `r21-opus.txt`, `r19-approvals.txt`, `r19-stream.jsonl`, `deferred-items.md` and this summary exist.
- Commits: `c55ef707`, `a666d68d`, `1dc3f379`, `818b702e` are ancestors of HEAD; `git rev-list --count` from the recorded base is 4.
- All three task verify blocks passed and the acceptance criteria hold: `kAudioFormatOpus` and `SCStreamOutputTypeAudio` in `media_darwin.m`, `"opusprobe"` in `main.go`, `framesPerPacket == 960` in `opus-sine.json`, R21 and R19 rows with results, `clone tailnet: none`, ten `write[spike-r19]` lines of ten `write[` lines, `tart list` shows `portty-test` running, `portty-test-2` stopped and no `spike-r19`.
