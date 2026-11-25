---
phase: 17-desktop-feasibility-spike
plan: 01
subsystem: desktop-spike
tags: [go, cgo, objc, videotoolbox, pion, webrtc, h264, feas-01, feas-02]

requires:
  - phase: 16-v1-2-cleanup
    provides: green root suite and brand gate to baseline against
provides:
  - spike/desktop nested Go module (portty-spike-desktop) with serve and selftest
  - a loopback pipeline proven end to end, synthetic frames through VideoToolbox H.264 and pion to a Go receiver
  - token-gated viewer page that posts a getStats report with every pinned key
  - 17-SPIKE-MATRIX.md with rows R00-R23 PENDING, tracer row T0 GO, and the FEAS-02 decision rule fixed before any probe
  - SC4 baseline (root suite, brand gate, traceability, repo-walking tests) with the spike present
affects: [17-02, 17-03, 17-04, 17-05, 17-06, 17-07, 17-08, 17-09, 17-10, 17-11, 17-12]

actuals:
  tokens: 18600
  tasks: 2
  commits: 2
plan_head_before: b21e0e7c76fb688223454a0ac1fbc616cd78a1cb
plan_head_after: 40d35b80b37c3d5723570fd54efa65e9f5a698c0

tech-stack:
  added: [github.com/pion/webrtc/v4 v4.2.29, github.com/pion/ice/v4 v4.4.9, github.com/pion/interceptor v0.1.51, github.com/pion/rtcp v1.2.20, github.com/pion/rtp v1.10.6]
  patterns: [one cgo Go file declares every C prototype and the exported callback, ObjC in sibling .m files with ARC, frames stay in ObjC from source to encoder and cross into Go once per access unit]

key-files:
  created:
    - spike/desktop/go.mod
    - spike/desktop/go.sum
    - spike/desktop/.gitignore
    - spike/desktop/main.go
    - spike/desktop/bridge_darwin.go
    - spike/desktop/media_darwin.m
    - spike/desktop/server.go
    - spike/desktop/viewer.html
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md
    - .planning/phases/17-desktop-feasibility-spike/evidence/t0-loopback.txt
  modified: []

key-decisions:
  - "The FEAS-02 decision rule is copied verbatim from the plan into the matrix and committed before any probe, so the capture-host decision cannot be fitted to results"
  - "startEncoder is a separate call from startSource: the plan's startSource(name, width, height, fps) signature has no bitrate, and sources feed the one global ObjC encoder"
  - "/report treats a JSON null frameJpeg as absent, because the viewer sends null when no frame arrived"
  - "The viewer page is served with Cache-Control no-store and Referrer-Policy no-referrer because the token sits in its URL"

patterns-established:
  - "Bind guard before any socket: wildcard refused always, loopback and 100.64.0.0/10 accepted, anything else only with --allow-lan, exit 2"
  - "Report filenames are built only from values matched against ^[A-Za-z0-9._-]{1,32}$ and ice lite|full, files created with O_EXCL"

requirements-completed: [FEAS-01, FEAS-02]

coverage:
  - id: D1
    description: "Tracer: synthetic frames through VideoToolbox Constrained Baseline H.264 and pion over a loopback UDP mux and the token-gated /offer to a Go receiver; ffprobe decodes 150 frames"
    requirement: "FEAS-01"
    verification:
      - kind: integration
        ref: "spike/desktop/build/spike selftest --out spike/desktop/build/t0 --seconds 5 plus ffprobe (evidence/t0-loopback.txt)"
        status: pass
    human_judgment: false
  - id: D2
    description: "serve refuses 0.0.0.0, ::, 192.168.1.10 and 8.8.8.8 with exit 2 before binding; the token gate answers 403 on /viewer.html, /offer and /report and 200 on /viewer.html with the token"
    requirement: "FEAS-01"
    verification:
      - kind: integration
        ref: "Task 1 and Task 2 automated verify blocks in 17-01-PLAN.md"
        status: pass
    human_judgment: false
  - id: D3
    description: "17-SPIKE-MATRIX.md with R00-R23 PENDING, T0 GO, and the nine-row FEAS-02 decision rule identical to the plan's text"
    requirement: "FEAS-02"
    verification:
      - kind: other
        ref: "diff of the rule section against 17-01-PLAN.md in the Task 2 verify block (empty)"
        status: pass
    human_judgment: false
  - id: D4
    description: "viewer.html behaves correctly in a real browser (negotiation, getStats report with every pinned key, frame capture)"
    verification: []
    human_judgment: true
    rationale: "The page passed a JavaScript syntax check and is served correctly, but no browser has run it; R11 to R14 in later plans are its first real run"
  - id: D5
    description: "SC4 baseline: root suite, brand gate, traceability paths and the two repo-walking release tests pass with spike/desktop present and the root module lists no spike package"
    requirement: "FEAS-01"
    verification:
      - kind: integration
        ref: "GOTOOLCHAIN=go1.26.11 go test -race -short ./... (exit 0, 14 ok, 0 FAIL); bash tests/check-brand.sh; bash tests/check-traceability-paths.sh"
        status: pass
    human_judgment: false

duration: 13min
completed: 2026-11-24
status: complete
---

# Phase 17 Plan 01: Tracer, Matrix Skeleton and SC4 Baseline Summary

**Synthetic frames reach a pion receiver as decodable Constrained Baseline H.264 through a hardware VideoToolbox encoder on loopback, and the FEAS-02 decision rule is committed in the matrix before any probe runs.**

## Performance

- **Duration:** 13 min
- **Started:** 2026-11-25T02:16:33Z (2026-11-24 local)
- **Completed:** 2026-11-25T02:29:47Z
- **Tasks:** 2
- **Files created:** 10

## Accomplishments

- A separate module `spike/desktop/` (module `portty-spike-desktop`, go 1.26.4, pion/webrtc v4.2.29) builds a single binary with `serve` and `selftest`. The root module's `go list ./...` lists no spike package.
- Tracer result on this Mac: hardware encoder (`com.apple.videotoolbox.videoencoder.ave.avc`, `hardware: true`), 150 access units and 437 packets in a 5 s run, ICE connected in about 219 ms, data channel round trip 1.1 ms, and ffprobe reads `Constrained Baseline,150`. The SDP advertises 42e01f and pion negotiated it, so the bitstream's 42c0xx did not block negotiation.
- Bind guard, token gate and caps work as specified: wildcard, LAN and public binds exit 2 before any socket opens; the viewer, /offer and /report answer 403 without the token; /offer rejects a 70 KB body and malformed SDP with 400; /report redaction turned `100.64.x`, `100.100.x` and RFC1918 addresses into `100.x.x.<n>` and `lan.<n>` and replaced the token with `REDACTED`.
- `viewer.html` is a one-script vanilla page that always writes every pinned report key, captures a frame at most 1280 px wide, and posts to /report.
- `17-SPIKE-MATRIX.md` has the eight required headings in order, R00-R23 all PENDING, the tracer row T0 GO, the SC2 coverage with the user-accepted Wails-shell substitution, and the nine-row first-match FEAS-02 rule copied byte for byte from the plan (the verify's diff is empty).
- SC4 baseline: `go test -race -short ./...` passes (14 ok packages, 0 FAIL), check-brand and the traceability check pass, and the two repo-walking release tests pass with the spike present.

## Task Commits

1. **Task 1: tracer** - `9c923f82` (feat)
2. **Task 2: viewer, matrix, evidence, SC4 baseline** - `40d35b80` (feat)

**Plan metadata:** committed with this summary (docs).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] /report rejected a viewer report with no frame**
- **Found during:** Task 2
- **Issue:** the viewer sends `frameJpeg: null` when no frame arrived; the writer parsed null as an empty string and answered 400, which would have lost the evidence of a failed run.
- **Fix:** a JSON null `frameJpeg` is removed without writing a .jpg.
- **Files modified:** spike/desktop/server.go
- **Verification:** manual POST with `"frameJpeg":null` stored a report (204); a valid JPEG data URL wrote a sibling .jpg.
- **Commit:** 40d35b80

**2. [Rule 3 - Blocker] go.mod as produced by the plan's `go get`**
- **Issue:** `go get` leaves every pion module `// indirect`, which would have failed no check but misdescribes the module.
- **Fix:** `go mod tidy` after the code existed; go directive stays 1.26.4 with no toolchain line.
- **Files modified:** spike/desktop/go.mod
- **Commit:** 9c923f82

**Interface note (not a rule fix):** the plan lists `startSource(name, width, height, fps)` without a bitrate, so `serve` calls a separate `startEncoder(width, height, fps, bitrate)` first. The prototypes and exported callback are exactly as planned.

**Total deviations:** 2 auto-fixed (1 bug, 1 blocker). **Impact:** none on scope or files; every plan file path was touched and nothing else was staged.

## Authentication Gates

None.

## Issues Encountered

None blocking. Two observations for later plans:

- The viewer page was only syntax-checked (`node --check`) and fetched over HTTP. It has not run in a browser; a loopback-only browser run would not connect (a browser does not offer a loopback candidate), so R11 to R14 are its first real run.
- No Go test file was added: the plan's file list is closed. The bind guard is covered by the verify loop, the pipeline by `selftest`, and `redact` by a manual POST; a table test for `checkBind` and `redact` is the first thing worth adding if the spike lives past this phase.

## Known Stubs

- `--source sck` is accepted by the flag check and fails with `unknown source "sck" (known: synthetic)`; 17-02 adds the ScreenCaptureKit case. Planned, not a goal blocker for this plan.
- Data channel `key:` and `wake` messages from the viewer are ignored by the server (threat T-17-04); a later plan adds `--allow-input`.

## Threat Flags

None. The surfaces added (viewer, /offer, /report, UDP mux, data channel, evidence writer) are all in the plan's threat model and carry its mitigations (T-17-01 to T-17-05, T-17-SC).

## Next Phase Readiness

Ready for 17-02 (ScreenCaptureKit capture, vmprobe, signed Spike.app). 17-02 extends `bridge_darwin.go` (the `sck` source and the pinned `encoder` keys are already in place) and adds `sys_darwin.m`.

## Self-Check: PASSED

- Files: all ten plan paths exist (spike/desktop go.mod, go.sum, .gitignore, main.go, bridge_darwin.go, media_darwin.m, server.go, viewer.html; 17-SPIKE-MATRIX.md; evidence/t0-loopback.txt).
- Commits: 9c923f82 and 40d35b80 are ancestors of HEAD; `git rev-list --count` from the recorded base is 2.
- Task 1 and Task 2 automated verify blocks passed; all acceptance greps passed; tracer verify re-run after the Task 1 commit passed.
