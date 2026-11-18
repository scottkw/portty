---
phase: 05-overnight-survival
plan: 11
subsystem: build
status: complete
gap_closure: true
tags: [build, toolchain, tailscale, json, shar-07]
requires:
  - "tailscale.com v1.98.3 (go.mod), whose JSON types need Go 1.26's encoding/json"
provides:
  - "go.mod: toolchain go1.26.11"
  - "build.sh: exports GOTOOLCHAIN from go.mod's toolchain line unless the caller set one; fails if the line is missing"
  - "toolchain_guard.go: //go:build go1.27 compile guard in package main (undefined buildPorttyWithGo1_26__export_GOTOOLCHAIN_from_go_mod)"
  - "internal/webserver/tailscale_json_test.go: TestTailscaleJSON_LocalAPIShapes, TestTailscaleJSON_FormatTagCanary, fakeLocalAPI(t) *local.Client"
affects: [05-15]
tech-stack:
  added: []
  patterns:
    - "Toolchain pin in three parts: go.mod toolchain line (CI and older Go), build.sh GOTOOLCHAIN export (newer local Go), build-tagged compile guard (direct go/wails builds)"
    - "Real local.Client tested against an httptest LocalAPI through Client.Dial with OmitAuth"
key-files:
  created:
    - toolchain_guard.go
    - internal/webserver/tailscale_json_test.go
  modified:
    - go.mod
    - build.sh
    - tests/build-script.test.sh
decisions:
  - "Pin go1.26.11, not the todo's go1.26.3: go.mod's go line is already 1.26.4, and 1.26.11 is the newest 1.26 patch with the same encoding/json behavior"
  - "build.sh honors a GOTOOLCHAIN the caller already set; the compile guard still stops a Go 1.27 override"
  - "The shapes test runs Portty's own checkHealth and sameUser on the decoded responses instead of only checking struct fields"
  - "TESTING.md is left to 05-15, which lists tailscale_json_test.go in its manifest and traceability updates and documents GOTOOLCHAIN"
metrics:
  duration: 6min
  completed: 2026-11-17
  tasks: 2
  files: 5
estimate:
  tokens: 40000
  tasks: 2
actuals:
  tokens: 3300
  tasks: 2
  commits: 3
plan_head_before: 705c5f1b7eac39bc1a463b0139bc24f7997ac291
plan_head_after: 836abdcdd6eead1d248254c66a3d58fe4f247eda
---

# Phase 5 Plan 11: Pin the Go Toolchain for tailscale.com Summary

Portty now builds with Go 1.26.11, the Go line that tailscale.com v1.98.3 supports. go.mod has a `toolchain go1.26.11` line, and build.sh exports GOTOOLCHAIN from that line. A `//go:build go1.27` guard file makes any direct Go 1.27+ build of the app or CLI fail with an error that names the fix. A new test runs the real `local.Client` against a fake LocalAPI and checks every field Portty reads, and a canary catches the Go 1.27 `format:nano` decode failure.

## What was built

- **go.mod:** one line added, `toolchain go1.26.11`. After `go mod tidy`, go.mod showed only that line and go.sum did not change. CI's setup-go (`go-version-file: go.mod`) reads the line, and a machine whose default Go is older upgrades to it automatically.
- **build.sh:** after argument validation and before the first `go` or `wails` call, it reads `toolchain` from go.mod and runs `export GOTOOLCHAIN="${GOTOOLCHAIN:-$GOMOD_TOOLCHAIN}"`. If go.mod has no toolchain line it stops with an error. A two-line comment gives the reason and says to remove it with the tailscale.com upgrade. The Wails CLI starts `go build` itself and inherits the variable.
- **tests/build-script.test.sh:** three static checks. build.sh exports GOTOOLCHAIN, build.sh reads go.mod's toolchain line, and go.mod has a toolchain line. Result: 40 passed, 0 failed.
- **toolchain_guard.go:** has the `//go:build go1.27` constraint and is in `package main`. It references the undefined `buildPorttyWithGo1_26__export_GOTOOLCHAIN_from_go_mod`. Its doc comment explains the pin and says to remove the file together with the go.mod line, once the canary passes under the new Go.
- **internal/webserver/tailscale_json_test.go:**
  - `TestTailscaleJSON_LocalAPIShapes`: an httptest server answers four routes: `/localapi/v0/status`, `status?peers=false`, `whois?addr=100.64.0.2:51234` and `prefs`. The JSON uses the v1.98.3 field names from `ipnstate.Status/PeerStatus/TailnetStatus`, `apitype.WhoIsResponse`, `tailcfg.Node/UserProfile` and `ipn.Prefs`, read from the module cache. A real `local.Client{OmitAuth: true, Dial: …}` talks to that server. The test checks:
    - `checkHealth` returns Connected, HasCerts, the IP, the domain and AcceptDNS.
    - `sameUser(whois, self)` returns true.
    - Self.UserID, Tags, AltSharerUserID and KeyExpiry decode.
    - WhoIs Node.User, Sharer and Tags, and UserProfile.ID decode.
    - CurrentTailnet.MagicDNSSuffix decodes.
    - For each peer, the fields internal/tailnet reads decode: HostName, DNSName, OS, TailscaleIPs and Online. Tags are checked as null and as `["tag:server"]`, and LastSeen is checked on the offline peer. Each peer's PublicKey matches its `nodekey:` map key.
  - `TestTailscaleJSON_FormatTagCanary`: decodes `{"accept":true,"sessionDuration":3600000000000}` into `tailcfg.SSHAction` and expects one hour. If the decode fails, it names the running toolchain and the fix.

## Verification

Task 1 verify passed. A binary built with the go.mod toolchain reports `go1.26.11`. Running the build.sh export block with GOTOOLCHAIN unset printed `GOTOOLCHAIN=go1.26.11` and `go version go1.26.11 darwin/arm64`. Tracer gate (end-of-phase mode, automated verify): I re-ran it end to end and it passed.

Task 2 verify passed in full under go1.26.11: both `--- PASS: TestTailscaleJSON_` lines, `go build ./...`, `go vet ./...`, `go vet -tags vmpreview ./...`, the go1.27.4 guard check, and the full `go test -race -short -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./...` (exit 0, no flake reruns needed).

Evidence in both directions:

```
$ GOTOOLCHAIN=go1.26.11 go build -o /dev/null .      # succeeds
$ GOTOOLCHAIN=go1.27.4 go build -o /dev/null .
# github.com/scottkw/portty
./toolchain_guard.go:13:9: undefined: buildPorttyWithGo1_26__export_GOTOOLCHAIN_from_go_mod

$ GOTOOLCHAIN=go1.26.11 go test -race -count=1 ./internal/webserver -run '^TestTailscaleJSON_' -v
--- PASS: TestTailscaleJSON_LocalAPIShapes (0.02s)
--- PASS: TestTailscaleJSON_FormatTagCanary (0.00s)

$ GOTOOLCHAIN=go1.27.4 go test -count=1 ./internal/webserver -run '^TestTailscaleJSON_' -v
--- PASS: TestTailscaleJSON_LocalAPIShapes (0.02s)
    tailscale_json_test.go:223: Go toolchain go1.27.4 cannot decode tailscale.com v1.98.3 JSON (format tag); build and test with the go.mod toolchain: GOTOOLCHAIN=go1.26.11 (json: cannot unmarshal object into Go value of type tailcfg.SSHAction: Go struct field SessionDuration has unsupported `format` tag option)
--- FAIL: TestTailscaleJSON_FormatTagCanary (0.00s)
```

This matches the planner probe: Portty's current decode surface (Status, WhoIs, Prefs) works under Go 1.27, while the SSHAction format-tag class does not.

## For the user

- A plain `go build`, `go test ./...` or `wails build` on this Mac (Homebrew Go 1.27.4) now fails at the guard, by design. Run `export GOTOOLCHAIN=go1.26.11` in your shell (or prefix each command with it) to use those commands directly. `./build.sh --platform macos` sets it for you. No shell profile or `go env` setting was changed.
- Removing the pin: upgrade tailscale.com, run `TestTailscaleJSON_FormatTagCanary` under the new Go, then delete `toolchain_guard.go` and the go.mod toolchain line together.

## TDD Gate Compliance

Task 2 adds characterization tests of an external library, and Portty's code did not change. RED evidence is the canary failing under go1.27.4 (shown above), and GREEN is go1.26.11 with the pin. Commits went in this order: `test(05-11)` e3a30562, then `build(05-11)` 836abdcd (the guard).

## Deviations from Plan

None. The plan ran as written.

Note: the portty/CLAUDE.md standing rule asks for TESTING.md updates when tests are added. 05-15 already owns those updates for this phase's gap closures, including `internal/webserver/tailscale_json_test.go` and the GOTOOLCHAIN note, so this plan does not edit TESTING.md. 05-10's `wakewatch_test.go` is not counted there yet either.

## Commits

- 73562abb build(05-11): pin Go toolchain to go1.26.11 and build with it
- e3a30562 test(05-11): LocalAPI decode test through local.Client and SSHAction format-tag canary
- 836abdcd build(05-11): refuse to compile the Portty binary under Go 1.27+

## Self-Check: PASSED

All 5 files exist; commits 73562abb, e3a30562, 836abdcd are ancestors of HEAD.
