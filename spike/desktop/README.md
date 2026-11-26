# Desktop feasibility spike (Phase 17)

Status: ARCHIVED (Phase 17 complete)

This is throwaway code that answered FEAS-01 and FEAS-02 for Portty v2.0: whether a Mac can capture its screen with ScreenCaptureKit, encode H.264 with VideoToolbox, and show it live over the tailnet through pion WebRTC, and which process should hold the Screen Recording and Accessibility grants. It is kept so the Phase 17 follow-ups (TESTING.md Section 5, Category BH, M-157..M-164) can rebuild the same probes on a real Mac.

## Never part of the app

`spike/desktop` is a nested Go module (its own `go.mod`; `wailsshell/` has another). The app never imports or builds it: the root `go list ./...` lists no spike package, and the root `go test ./...` does not run `hostrun_test.go`. Nothing in `build/`, `build.sh` or the release workflow refers to it.

## Layout

| Path | What it is |
|------|------------|
| `main.go` | verbs: `probe`, `probe-loop`, `status`, `register`, `unregister`, `agent`, `spawn`, `swap`, `serve`, `selftest`, `host-run`, `vmprobe`, `opusprobe` |
| `probe.go` | TCC preflight, session keys, capture and PNG records (JSON lines) |
| `server.go`, `viewer.html` | pion WebRTC server bound to one tailnet address, token-gated viewer page (ICE-lite or full) |
| `hostrun.go`, `hostrun_darwin.m`, `hostrun_test.go` | the host run: test chart, timeline, scheduled sleep, wake and allow-listed typing |
| `bridge_darwin.go`, `media_darwin.m`, `sys_darwin.m` | cgo bridges to ScreenCaptureKit, VideoToolbox, Opus, CGEvent, IOKit power and SMAppService |
| `bundle/` | `Info.plist`, entitlements, the helper and daemon LaunchAgent plists, `build.sh` |
| `wailsshell/` | the throwaway Wails v2 viewer shell (the Portty-app stand-in for R13) |

## Rebuild

```sh
bash spike/desktop/bundle/build.sh          # build/Spike.app: registrar app, daemon and Contents/Helpers/spike-helper
bash spike/desktop/bundle/build.sh --shell  # the Wails viewer shell
```

`build.sh` signs with a Developer ID identity, one binary at a time, inside out, and refuses an ad-hoc signature (TCC rows must match a release signature).

## Safety rules

- Never open Spike.app, or load its LaunchAgent, on a Mac without its owner's go.
- On a personal Mac, capture only the spike's own test chart (the default `host-run`), never the real desktop.
- Revoke afterwards: `tccutil reset ScreenCapture app.portty.spike`, `tccutil reset Accessibility app.portty.spike`, `tccutil reset PostEvent app.portty.spike`, `launchctl bootout gui/$(id -u)/app.portty.spike.daemon`, remove its plist, and `open -n Spike.app --args unregister` for the helper.
- Never commit tokens, tailnet addresses or names, account names, or frames of a real desktop. The server redacts viewer reports itself; check the rest by hand.

## Results

The go/no-go matrix and the FEAS-02 decision (capture host: in-daemon) are in `.planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md`, with raw evidence under `evidence/` next to it. The real-Mac follow-ups that reuse this code are TESTING.md M-157..M-164.
