---
phase: 21-portty-app-and-iphone-viewers
plan: 07
subsystem: gui-bindings
tags: [wails, peer-desktop, local-network, plist]
requires:
  - phase: 21-06
    provides: DaemonClient.ConnectPeerDesktop and PeerDesktopLink
  - phase: 21-01
    provides: tailnet PeerSessionMetaGroup.Desktop
provides:
  - ConnectPeerDesktop, OpenPeerDesktopInBrowser, OpenLocalNetworkSettings Wails bindings
  - desktop flag on RemotePeerSessions
  - NSLocalNetworkUsageDescription in both plists
affects: [21-08, 21-11, 21-13]
tech-stack:
  added: []
  patterns: [never-throw binding results, Go-side opening of token-bearing links]
key-files:
  created: [app_peer_desktop.go, app_peer_desktop_test.go]
  modified: [app.go, frontend/src/wailsjs/go/main/App.d.ts, frontend/src/wailsjs/go/main/App.js, build/darwin/Info.plist, build/darwin/Info.dev.plist]
key-decisions:
  - "OpenPeerDesktopInBrowser treats any link-route error as 'not held': one connect, one retry (the client does not type the 404)"
  - "paneLocalNetwork anchor is an assumption confirmed by the 21-11 VM row"
status: complete
actuals:
  tokens: 9000
  tasks: 3
  commits: 3
plan_head_before: e7a74d9d8a9e068c00d529ac7c963a1ae3b1fc80
plan_head_after: 13d792db98dc2bd81dd5c23b5fed0c785053b4ae
---

# Phase 21 Plan 07: Peer desktop bindings Summary

Wails bindings that turn a sidebar click into a daemon-held token and a token-free loopback URL, with the discovery flag, browser fallback and Local Network pane, plus the Local Network usage string in both plists.

## What was built

- `ConnectPeerDesktop(host)` returns `{host, url, errorCode}`; url is `ws://127.0.0.1:<relay port>/api/relay/remote-desktop/<lower-cased, dot-trimmed, path-escaped host>/ws`, filled whenever the relay port is known (also with an error code). Nil client or dead daemon gives `unreachable` and an empty url. No token field exists in the result.
- `remotePeersFromGroups` (extracted from `GetRemoteSessionsWithMeta`) carries `Desktop` as `desktop`; sessions stay a non-nil slice.
- `OpenPeerDesktopInBrowser(host)` opens the held link through the `openURLFunc` seam (default `runtime.BrowserOpenURL`), returns `""` or a code, never returns or logs the link.
- `OpenLocalNetworkSettings()` calls `openSystemURL(paneLocalNetwork)`.
- App.d.ts and App.js declare all three bindings, `PeerDesktopResult` and `desktop: boolean`, beside `ConnectPeerSession`, re-derived from the Go signatures at execution time (no generator run, Phase 20 hunks untouched).
- Both plists carry `NSLocalNetworkUsageDescription` with the exact string; nothing else changed.

## Tests (new files listed for 21-13)

- `app_peer_desktop_test.go` (new): TestConnectPeerDesktop_*, TestRemotePeersFromGroups, TestOpenPeerDesktopInBrowser, TestOpenLocalNetworkSettings. Fake daemon on a unix socket in a temp dir; seams for browser and System Settings; no host side effects.

## Verification

- `go vet .`, gofmt, `golangci-lint run --new-from-rev=060281bd .` (0 issues), `tsc --noEmit`, `tests/check-brand.sh` all clean.
- `go test -race -count=3 -run TestConnectPeerDesktop .` and the Task 2 tests pass.

## Deviations from Plan

**1. [Rule 1 - Plan inaccuracy] `plutil -lint` cannot lint the raw plists.**
- Both plists are Wails Go templates (`{{end}}` etc.); `plutil -lint` fails on them at the pre-change HEAD too (line 30 of Info.plist). Verified instead by stripping `{{...}}` and linting the result: both OK. Key counts match (25 and 24) and the ATS key count is unchanged.

**2. Commit trailer** names the actual model (Claude Sonnet 5.5) per the dispatch constraints, not the plan's Opus string.

### Deferred / out of scope

- `TestNoFakeWhoIsInRelease` fails in this worktree because `frontend/dist` is not built (`pattern all:frontend/dist: no matching files found`). Pre-existing and environmental; every other root-package test passes with `-race`.

## Known Stubs

None.

## Threat Flags

None. No new network surface; T-21-21 (no token in any binding result, tests assert no `cap` in the url), T-21-22 (host normalised and path-escaped), T-21-23 (only the Local Network string added), T-21-24 (browser open only from the bound click path) are met.

## Self-Check: PASSED
