---
phase: 17-desktop-feasibility-spike
plan: 07
subsystem: desktop-spike
tags: [webrtc, safari, tailnet, ice-lite, h264, derp, tart, feas-01, r01, r11, r22, r23]

requires:
  - phase: 17-desktop-feasibility-spike
    provides: "17-06 and 17-09 guest state (portty-test with the signed ~/ptest/Spike.app and its grant), Pipeline shape: daemon, the spike's pion serve with the sck source"
provides:
  - "R11 GO: Safari 27.2 decodes the Constrained Baseline stream under the 42e01f SDP, ICE-lite and full ICE alike, 2560 x 1440, no SPS patch needed"
  - "R22 GO: first decoded frame 765 ms after joining a still screen (spread 0.77 to 1.20 s over three joins)"
  - "R23 GO: the running serve holds one UDP and one TCP socket on the single tailnet address, and serve --bind 0.0.0.0 and :: exit 2"
  - "R01-vm PARTIAL: the desktop is decoded live over the tailnet with 0 spike lines on ports 3478, 5353 and 19302, but both tailscale ping runs went via a DERP relay, so the SC1 clause 'no STUN, TURN or cloud server involved' is not shown"
  - "viewer.html fix: a canvas that cannot read back (GPU-less guests) now sends frameJpeg null instead of 'data:,', which the server refused with 400"
affects: [17-08, 17-11, 17-12, DESK-02, DESK-04]

actuals:
  tokens: 9974
  tasks: 3
  commits: 3
plan_head_before: 36fe0fd37fb34b8c663b58dd4e9139726721b530
plan_head_after: 3754d5bba78312f57cd4f7d69ba1480d3168312e

tech-stack:
  added: []
  patterns: ["a capture on the tailnet interface (utun4) of the same WebRTC peer port stands in for the candidate-pair address check when Safari reports candidate addresses as null", "a screenshot crop of the video element stands in for the report frame when canvas readback is unavailable in a GPU-less guest"]

key-files:
  created:
    - .planning/phases/17-desktop-feasibility-spike/evidence/rig-tailnet.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r01-network.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r11-report-safari-lite.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r11-report-safari-full.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r11-frame-safari-lite.jpg
    - .planning/phases/17-desktop-feasibility-spike/evidence/r22-join.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r23-bind.txt
  modified:
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md
    - spike/desktop/viewer.html

key-decisions:
  - "R01-vm is PARTIAL, not GO: both before and after tailscale ping runs went via DERP(mia), and a relay in the path is what the plan and ROADMAP SC1 exclude; Safari also reports null candidate addresses"
  - "Task 2 (user re-key) was skipped under the plan's own rule: both guests were already Running, Online and signed under Tailnet Lock, so no user action was needed"
  - "R22 is GO on its own run (765 ms) with the two slower R11 joins (1184 and 1198 ms) written into the row, because all three are about 1 s on a DERP-relayed path"

patterns-established:
  - "Read the page's own state through the accessibility tree (axdump) when WebKit gives no console: it showed 'report not stored: /report answered 400' and led to the canvas finding"

requirements-completed: []  # FEAS-01 stays open: only 17-12 closes it

coverage:
  - id: D1
    description: "Both VMs on the tailnet and signed; VM home macOS recorded (rig-tailnet.txt, Rigs table)"
    requirement: "FEAS-01"
    verification:
      - kind: other
        ref: "Task 1 verify block (ProductVersion, BackendState, no keys, domain or tailnet addresses) and the Task 2 guest check: tailscale status --self --json | jq -e '.BackendState == \"Running\" and .Self.Online == true' in both guests"
        status: pass
    human_judgment: false
  - id: D2
    description: "R11 lite and full GO, R22 GO, R23 GO and R01-vm PARTIAL recorded with evidence"
    requirement: "FEAS-01"
    verification:
      - kind: other
        ref: "Task 3 verify block: report keys, frame present, lsof and tailscale ping sections, path-before and path-after lines, refusal exit codes, results for R01-vm R11-lite R11-full R11 R22 R23, SC1 clause named, shape recorded in five rows, no tailnet address, domain or token in committed evidence"
        status: pass
    human_judgment: false
  - id: D3
    description: "The frame evidence is a screenshot crop of the video element rather than the viewer's own canvas frame"
    verification: []
    human_judgment: true
    rationale: "A person should confirm the cropped JPEG shows the VM office desktop live in Safari and nothing else; the executor read it and found the System Settings Login Items pane, a Finder window and the install-tools dialog"

duration: 16min
completed: 2026-11-24
status: complete
---

# Phase 17 Plan 07: Tailnet pipeline run in the VMs Summary

**Safari in VM home decodes the VM office desktop live over the tailnet from the daemon-shape pion server, with ICE-lite and full ICE giving the same result; the path was a Tailscale DERP relay both times, so R01-vm is PARTIAL on the SC1 clause.**

## Performance

- **Duration:** about 16 min (start taken from the first commit minus its prep, not recorded at launch)
- **Started:** 2026-11-25T05:11:00Z (approximate)
- **Completed:** 2026-11-25T05:27:00Z
- **Tasks:** 3 of 3 (Task 2 skipped by the plan's own rule)
- **Files modified:** 9 (7 evidence files, the matrix, viewer.html)

## Accomplishments

- **Tailnet rig.** `portty-test-2` was booted; both guests report BackendState Running, Self.Online true and a direct Tailnet Lock node signature, so no re-key was needed. VM home is macOS 27.2 (26B123), recorded in the Rigs table.
- **R11 GO (lite and full).** Safari 27.2 connected and decoded in both modes: connect 564 ms against 523 ms, first frame 1184 ms against 1198 ms, 7 frames each at 2560 x 1440, 0 dropped. The codec entry is `video/H264` with `profile-level-id=42e01f` for both. The VideoToolbox stream is 42c0xx and decoded under that SDP, so the SPS patch was not needed. Lite and full do not differ here. Plain http worked, so no TLS fallback ran.
- **R22 GO.** After 30 s untouched (two screenshots byte-identical) a new page joined with `row=R22` and decoded its first frame at 765 ms (connect 388 ms). One run for this row; the R11 joins read about 1.2 s, so the spread is 0.77 to 1.20 s.
- **R23 GO.** `lsof` on the serve process shows UDP 47801 and TCP 47800 on the tailnet address and nothing else. `serve --bind 0.0.0.0` and `--bind ::` both print "bind address refused" and exit 2.
- **R01-vm PARTIAL.** The desktop was decoded live over the tailnet. `lsof` shows only the tailnet address, tcpdump on ports 3478, 5353 and 19302 shows 0 spike lines (24 packets, all `tailscaled`'s own endpoint discovery or its replies), and a capture on `utun4` shows STUN checks, DTLS and 1210-byte RTP between the two tailnet addresses. Against that, `tailscale ping` from VM home read `via DERP(mia)` with "direct connection not established" both before and after, so `path-before: derp` and `path-after: derp`. The matrix row names the SC1 clause as not shown. Safari also reports `selectedPair` addresses as null, so the report cannot show the tailnet pair itself.
- **Cleanup.** The agent was booted out, the LaunchAgent plist and daemon log (which held the token in argv) deleted, `agent-B.json` rewritten without the token, no spike or tcpdump process left, and a scan of the guest evidence directory for the token found nothing.

## Task Commits

1. **Task 1: tailnet rig state, VM home macOS** - `a4cba71c`
2. **Task 2: user re-key** - skipped, no commit (see Deviations)
3. **Task 3 (fix found on the way): viewer sends null when canvas readback fails** - `64dc02be`
4. **Task 3: evidence and matrix rows** - `3754d5bb`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] viewer.html sent `data:,` as the frame and the server refused the report**
- **Found during:** Task 3, first R11-lite run
- **Issue:** Safari connected and decoded, but no report file appeared. The page's own state (read with axdump) said `report not stored: Error: /report answered 400`. A local test page showed `canvas.toDataURL('image/jpeg')` returns `data:,` at 64x40, 640x400 and 1280x800 in this GPU-less guest. The page then sent `"frameJpeg": "data:,"`, which `writeFrame` rejects ("not a JPEG data URL").
- **Fix:** two lines in `spike/desktop/viewer.html`: if the data URL does not start with `data:image/jpeg`, send `null`, which the server already accepts ("the page sends null when no frame arrived").
- **Rebuild:** `bundle/build.sh` (same Developer ID identity, new CFBundleVersion), the daemon booted out, the bundle swapped into `~/ptest/Spike.app` in VM office with `mv`, `codesign --verify --strict` clean, and the daemon bootstrapped again. The grant survived with no prompt: a fourth R10 data point.
- **Files modified:** `spike/desktop/viewer.html`
- **Commit:** `64dc02be`
- **Consequence:** the viewer report has no frame in these guests, so `r11-frame-safari-lite.jpg` is a crop of the video element taken from a VM home screenshot (`screencapture`, cropped with `sips`, 759 x 427). The crop excludes the Safari address bar, which showed a tailnet address.

### Plan versus dispatch

**2. [Plan path taken, differs from dispatch note] Task 2 checkpoint not raised**
- The dispatch said to stop at Task 2 with a checkpoint. Task 1's own action says: "If both guests already report Online true and unlocked by Tailnet Lock, write that in rig-tailnet.txt and skip Task 2 with the reason." Both did (the ephemeral nodes survived the one-day stop of `portty-test-2`). Asking the user to re-key nodes that were already signed would have been wrong, and Task 3's precondition (both guests Running and Online) held, so I ran Task 3. Task 2's guest check passes for both guests (`jq -e` true, true). No auth key was ever needed, asked for or seen.

### Other notes

- **Banners cleared in VM office.** Four sticky "App Background Activity" banners from the daemon start (naming the developer certificate holder) sat on the capture. `killall NotificationCenter` brought them back, so I clicked them away, which opened System Settings on Login Items in the guest. Nothing in a committed frame names anyone.
- **Supplementary capture.** The plan's tcpdump covers ports 3478, 5353 and 19302 only. Because Safari reports null candidate addresses, I added a short `utun4` capture of UDP 47801 (14 packets). It caught the lite session's traffic still running while the full session was starting (its peer port 61885 equals the lite report's `selectedPair.localPort`); it is labelled that way in `r01-network.txt`.
- **`spk_sck_stats` not read.** R22's "mostly Idle" check needs a counter the running `serve` does not expose. Stillness rests on the two byte-identical screenshots instead, and the row says so.
- **R23.** The non-100.64/10 and LAN refusals were covered in 17-01 and not re-run; the row says so.

**Total deviations:** 1 auto-fixed bug (Rule 1), 1 plan-path decision, 4 notes. **Impact:** none on the plan's answers except that R01-vm cannot reach GO on a DERP path regardless.

## Authentication Gates

None. The re-key and Tailnet Lock signing gate was not reached because both guests were already signed.

## Issues Encountered

- Both VMs reach each other only through DERP(mia) (RTT 52 to 82 ms). Tart's NAT between two guests on one host gives Tailscale no direct path. 17-11 (host to iPhone) or a rig with a direct path is the only way to show the SC1 clause; 17-12 needs an M-NN for the R01-vm follow-up.
- The R11 joins were slower (1.18 to 1.20 s) than the R22 join (0.77 s); one R22 sample is thin. The path is relayed with about 55 ms RTT, which adds to each handshake round trip.

## Known Stubs

None.

## Threat Flags

None. The viewer change adds no endpoint or file access.

## Guest end state

Both VMs are running and on the tailnet (do not stop them: ephemeral nodes drop out after hours offline and need a re-key plus a lock signature). VM office: no spike or tcpdump process, no LaunchAgent plist, `~/ptest/Spike.app` is this plan's rebuilt bundle with the grant intact, `agent-B.json` holds a placeholder instead of a token, System Settings is open on Login Items. VM home: Safari closed. The token lives only in the session scratchpad, never in a committed file.

## Next Phase Readiness

Ready for 17-08. Rows R01-vm (PARTIAL, direct-path follow-up), R11 (GO), R22 (GO), R23 (GO) are filled; base R01 stays PENDING until 17-11 adds R01-host. FEAS-01 and FEAS-02 stay open: only 17-12 closes them. 17-12 should still repair the misplaced R15 to R17 rows (deferred-items.md); this plan edited only `## Rows` entries by matching after that heading.

## Self-Check: PASSED

- Files created exist: rig-tailnet.txt, r01-network.txt, r11-report-safari-lite.json, r11-report-safari-full.json, r11-frame-safari-lite.jpg, r22-join.json, r23-bind.txt, 17-SPIKE-MATRIX.md rows.
- Commits `a4cba71c`, `64dc02be` and `3754d5bb` are on the branch.
