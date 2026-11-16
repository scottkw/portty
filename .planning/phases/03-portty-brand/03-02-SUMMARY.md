---
phase: 03-portty-brand
plan: 02
subsystem: tailnet-webserver-updater
status: complete
tags: [brand, port, peer-discovery, update-feed, go-selfupdate]
requires:
  - "Go module github.com/scottkw/portty (03-01)"
provides:
  - "tailnet.DefaultProbePort = 7444, tailnet.MarkerHeader/MarkerValue (X-Portty: 1)"
  - "isPorttyProbeResponse: only marked 200/401 replies count as Portty peers"
  - "porttyMarker wrapper on GET /api/sessions only"
  - "updater.detectLatestFrom(ctx, src, slug) with dmgAssetFilter; app.go updateSlug = scottkw/portty"
affects: [03-04, 03-06]
tech-stack:
  added: []
  patterns: ["Identity header on one route, outside the capability check", "Offline fake selfupdate.Source for detection tests"]
key-files:
  created:
    - internal/webserver/probe_marker_test.go
  modified:
    - internal/tailnet/tailnet.go
    - internal/tailnet/tailnet_test.go
    - internal/webserver/server.go
    - internal/daemon/process.go
    - internal/daemon/client.go
    - internal/updater/updater.go
    - internal/updater/updater_test.go
    - app.go
    - frontend/src/components/SettingsSheet.tsx
    - frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx
    - frontend/src/lib/__tests__/remoteSession.test.ts
    - internal/daemon/api_test.go
    - internal/daemon/funnel_test.go
    - TESTING.md
key-decisions:
  - "Portty web/peer port is 7444 (tailnet.DefaultProbePort); the daemon binds the same constant the probe dials"
  - "Peer discovery requires X-Portty: 1 with 200 or 401; the probe no longer reads the body, so upstream-app peers are not shown"
  - "The X-Portty header is set only on GET /api/sessions, outside requireCapability; it is an identity hint, not authentication"
  - "Update check reads scottkw/portty through go-selfupdate with Filters ^portty-v?\\d+\\.\\d+\\.\\d+.*-darwin-universal\\.dmg$; every no-release or error path stays silent"
requirements-completed: [BRND-03, BRND-05, BRND-06, NOTF-01]
duration: 28 min
completed: 2026-11-16
commits: 3
plan_head_before: 9dc66c47d7f4306abf8fd125b73433fe19c0c0d6
plan_head_after: e983943dd62407473e3d68594f6ba970b7255845
actuals:
  tokens: 12500
  tasks: 3
  commits: 3
---

# Phase 3 Plan 02: Port 7444, X-Portty Marker and Portty Update Feed Summary

Portty now listens and probes on port 7444, finds peers only by an `X-Portty: 1` header that GET /api/sessions sets on both the 401 and the 200, and checks GitHub releases of scottkw/portty with a filter that matches the `portty-v<x.y.z>-darwin-universal.dmg` asset. When nothing is found, the check says nothing.

## Performance

- Duration: about 28 min (2026-11-16T16:00:13Z to 16:28Z)
- Tasks: 3 of 3
- Files: 1 created, 14 modified

## Task Commits

| Task | Name | Commit |
|------|------|--------|
| 1 (tracer) | Peer discovery on 7444 requires the X-Portty marker; daemon binds the same constant | 599509c9 |
| 2 | Update check reads scottkw/portty and matches the release DMG | 7032ac6c |
| 3 | Settings defaults to 7444; tests and TESTING.md follow | e983943d |

## What Changed

- `internal/tailnet/tailnet.go`: `DefaultProbePort = 7444`; `MarkerHeader = "X-Portty"`, `MarkerValue = "1"`; `isPorttyProbeResponse` replaces the body-marker predicate (no body read, `io` import dropped). Package and function comments say Portty.
- `internal/webserver/server.go`: `porttyMarker(next)` wraps GET /api/sessions outside `requireCapability`. No other route gets the header. Old-app comments reworded; example URLs use 7444.
- `internal/daemon/process.go`: all three web-server starts bind `tailnet.DefaultProbePort`.
- `internal/updater/updater.go`: `dmgAssetFilter`, `detectLatestFrom(ctx, src, slug)` (a nil source falls back to GitHub); `DefaultDetect` is one line. `Check` unchanged.
- `app.go`: `const updateSlug = "scottkw/portty"` used at both `updater.Check` call sites.
- Frontend: Settings web-server port defaults to 7444.
- TESTING.md: Go 109 / total 208 note, four new Section 4 rows (BRND-03 x2, NOTF-01, BRND-06), STRP-01 row names `TestFetchPeerSessionsMeta_IgnoresLegacyFields`, M-70 two-Mac check in Category B.

## Gate Results

| Gate | Result |
|------|--------|
| Task 1 verify (build, vet, race tests for tailnet/webserver/daemon) | pass |
| TestProbePeer_* + TestProbeMarker_* PASS count | 15 (needs 15 or more) |
| Task 2 verify (build, vet, race tests for updater and root package) | pass |
| TestDetectLatestFrom_* + TestCheck_* PASS count | 13 (needs 13 or more) |
| `pnpm -C frontend exec tsc --noEmit` | pass |
| Targeted vitest (SettingsSheet.tailscale-status, remoteSession) | 21 of 21 pass |
| Full vitest | 91 files, 1173 tests pass (see Issues: first run under load flaked) |
| `go build ./... && go vet ./... && go test -race -short ./...` | pass, no FAIL lines |
| Old-port sweep (`git grep 7443` outside .planning, TESTING.md, CHANGELOG.md, testdata) | exits 1, nothing found |
| `bash tests/check-traceability-paths.sh` | OK: 98 traceability paths exist |
| Acceptance greps (all three tasks) | all match the expected counts |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Task 2 old-name grep scoped away from app.go lines owned by plan 03-04**
- **Found during:** Task 2
- **Issue:** Task 2's verify runs `git grep -i 'agent[h]ub' -- 'internal/updater/*' app.go` and expects no match. app.go lines 344-348 (QuitGUIOnly notification body, identifier and title) still name the old app. Plan 03-04 Task 2 owns exactly those lines and replaces them with `quitToMenuBarBody(count)` and new constants, with its own tests.
- **Fix:** Left those lines alone so 03-04 can apply its tested copy. Ran the grep over `internal/updater/*` only (exit 1, clean) and confirmed every update-check line in app.go uses `updateSlug` (4 occurrences: const, its comment, two call sites). The updater and the update-check code contain no old name or slug.
- **Files modified:** none beyond the plan
- **Commit:** 7032ac6c

**2. [Rule 1 - Bug] `tailnet.MarkerHeader` appeared twice in server.go**
- **Found during:** Task 1 acceptance check
- **Issue:** The porttyMarker doc comment also named `tailnet.MarkerHeader`, so the "one route only" grep printed 2.
- **Fix:** The comment now says `X-Portty: 1`; the constant appears once, in code.
- **Commit:** 599509c9

Otherwise the plan ran as written. `TestProbePeer_Generic401_NotFound` was kept (a bare 401 with no marker is still rejected); only its comment changed.

## Issues Encountered

- The first full vitest run in Task 3 reported 1 failed test and 6 errors out of 1097 tests run. Two reruns with no code change passed fully: 91 files and 1173 tests. The targeted files for this plan passed 21 of 21 every time. This looks like a load-related flake, not something this plan caused. The failing test name was not captured from the first run.
- `gofmt -l` lists `internal/daemon/client_test.go` and `internal/relay/scrollback_altscreen_test.go`. This plan did not touch either file, and they were already unformatted before it ran. Out of scope.

## Threat Model

- T-03-04: `porttyMarker` only sets a header and then always calls `requireCapability`. `TestProbeMarker_UnauthenticatedSessionsCarriesMarker` checks for 401 with `capability required`, and the existing capability tests pass in the full race run.
- T-03-06: `TestProbeMarker_OtherRoutesHaveNoMarker` checks that /api/sessions/meta and /dashboard do not carry the header.
- T-03-07: the probe reads only the header and no longer reads the body.
- T-03-08: the filter is anchored with `^…$`, the slug is a single constant, and nothing is downloaded.
- T-03-SC: no packages were added.

## Human Check Pending

M-70 (TESTING.md Category B): two Macs on one tailnet. Run the upstream app and Portty side by side on Mac A. On Mac B, Other Macs should list only Portty hosts. A curl to Mac A's 7444 should show 401 and `x-portty: 1`. This needs physical hardware, so it is left for end-of-phase verification.

## Next Phase Readiness

03-04 can now replace the QuitGUIOnly copy in app.go. After that, app.go no longer contains the old name. The DMG filter expects 03-06 to rename the release asset to `portty-v<x.y.z>-darwin-universal.dmg`. Until that rename lands, a release built with the old asset name will not be detected.

## Self-Check: PASSED
