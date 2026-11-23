---
phase: 14-mobile-web-app
plan: 06
subsystem: test-rig
tags: [go, build-tag, whois, release-guard, playwright-fixture, ios-simulator]
requires:
  - phase: 14
    plan: 01
    provides: webservertest WhoIs fake, requireSameUser gate, Playwright fixture admin listener
provides:
  - "vmwhois build tag: other-account / pass / fault WhoIs wrapper on the real tailnet share server (VM only)"
  - "TestNoFakeWhoIsInRelease: go list -deps guard over three release graphs with a positive control"
  - "check-brand fake-whois and vmwhois vm-tag checks"
  - "Fixture admin routes: grid, info-hits, share-off, share-on, end, preview-pdf, plus CA PEM output"
affects: [14-07, 14-12, 14-13]
status: complete
actuals:
  tokens: 9500
  tasks: 3
  commits: 3
plan_head_before: edd53f9a62f1879ee620b2d3668b1a7405fb1a73
plan_head_after: 19f3d657f87059ea90f9dd3d2b5aeb49d5a073a2
tech-stack:
  added: []
  patterns:
    - "tagged init() wraps the previous defaultNewShareServer and installs a WhoIs wrapper (vmpreview's replace-everything pattern, narrowed to one answer)"
    - "one goListDeps helper returns an error on any go list failure, so a broken go list can never read as 'fake absent'"
key-files:
  created:
    - internal/daemon/share_vmwhois.go
    - internal/daemon/share_vmwhois_test.go
    - release_guard_test.go
  modified:
    - tests/check-brand.sh
    - cmd/playwright-fixture/main.go
key-decisions:
  - "vmwhois keeps the real tailnet server and changes only the WhoIs answer; this Mac's own node (StableID equals real Self.ID) always passes so the host can open its own link"
  - "Mode file contract: $HOME/p14/whois-mode read per call; missing or empty means other; 'other', 'pass', 'fault'; any other content means fault (fail closed)"
requirements-completed: []  # MOB-05 and MOB-02 are check-rig only here; the VM refusal run is 14-12, the simulator runs are 14-07 and 14-13, so mark-complete was not run
---

# Phase 14 Plan 06: Release guard, vmwhois tag and simulator fixture routes Summary

A VM-only `vmwhois` build can make the real tailnet share server see every node but this Mac as another Tailscale account (or a tailscaled fault), two gates keep every WhoIs fake out of releases, and the Playwright fixture can now play the share's whole life for the iOS Simulator.

Log directory (runs of the final state): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.iUNMrAFdzX` (build and package tests); Task 1 logs in `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.X9bYOkgpJ6`; fixture smoke in `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.Cor29rsLEa`.

## Accomplishments

### Task 1: vmwhois wrapper (commit 155707d2)

- `internal/daemon/share_vmwhois.go` (`//go:build vmwhois`): `vmWhoIs{inner}`; `init()` keeps the previous `defaultNewShareServer` and calls `SetWhoIsClientForTest(vmWhoIs{&local.Client{}})` on every server it returns. Imports nothing from the webserver test package. One `slog.Debug` line per decision, numeric IDs only.
- **Mode-file contract:** `$HOME/p14/whois-mode`, read on every call.

  | Content | Behaviour |
  |---------|-----------|
  | missing file, empty, `other` | real answer, except a node whose `StableID` differs from the real `Self.ID` is returned as a copy with `User = Self.UserID + 1000`; this Mac's node passes unchanged |
  | `pass` | real answers |
  | `fault` | both `StatusWithoutPeers` and `WhoIs` return an error (share server answers 503, no marker) |
  | anything else, or a read error other than not-exist | treated as `fault` |
- `internal/daemon/share_vmwhois_test.go` `TestVMWhoIs_Modes`: missing file, empty, pass, fault, bogus, and no mutation or aliasing of the inner response.
- `tests/check-brand.sh`: vm-tag check now `-E 'vmpr[e]view|vmwh[o]is'`; header updated.

### Task 2: release guard (commit 6003e93a)

- `release_guard_test.go`: `goListDeps(tags, pkg)` (separate stdout and stderr, error carries args, exit status and stderr). `TestNoFakeWhoIsInRelease`: no tags, `wailsassets` and `vmwhois` graphs each contain `internal/webserver` and lack `webservertest`; the `playwrightfixture` fixture graph contains it (positive control). Every `go list` error is `t.Fatalf`. `TestGoListDeps_ReportsFailure` pins that a failing `go list` returns an error.
- `tests/check-brand.sh`: `fake-whois` check, `git grep` for `internal/webserver/webserver[t]est` over `*.go` excluding `*_test.go`, `cmd/playwright-fixture/` and `internal/webserver/webservertest/`. Confirmed against the tree that the pattern matches the fixture and that only the fixture remains as a non-test match before exclusion.

### Task 3: fixture routes (commit 19f3d657)

All on the loopback plain-HTTP admin listener; fixture build tag `playwrightfixture`.

| Route | Request | Response |
|-------|---------|----------|
| `GET /__test__/grid` | none | 200 `{"cols":N,"rows":N,"resizes":N}` (hub resize hook; zeros until a viewer sends a grid) |
| `GET /__test__/info-hits` | none | 200 `{"count":N}` (session resolver calls, which run only after the capability check passes) |
| `POST /__test__/share-off` | none | 204; `ClearGrants`, `DisableSession`, `RevokeWebViewers("share stopped")` |
| `POST /__test__/share-on` | none | 200 `{"cap":"<token>"}`; `EnableSession`, new grant id (`grant-playwright-fixture-N`), `AddGrant`, signs `read,write` |
| `POST /__test__/end` | none | 204; closes the PTY output pipe writer, the hub ends and live sockets close "session ended" |
| `POST /__test__/preview-pdf` | none | 200 `{"viewers":N}`; 409 if a preview is streaming; sends a 329-byte one-page PDF (`fixture.pdf`, `application/pdf`, key `0123456789abcdef0123456789abcdef`) |

Wrong method gets 405 on each. Stdout line format: `BASE_URL=`, `CAP=`, `ADMIN_URL=`, then `CA_PEM=<path>` only when `PLAYWRIGHT_FIXTURE_CA_PEM` is set, then `READY=1`. `selfSignedTLS` now also returns the CA certificate PEM, written 0644 before READY; the key is never written. The package doc lists every route and the new stdout line.

## Test results

- `go vet -tags vmwhois ./internal/daemon/` clean; `go test -tags vmwhois -race -count=1 -run VMWhoIs -v ./internal/daemon/`: `--- PASS: TestVMWhoIs_Modes`.
- `go test -count=1 -run 'NoFakeWhoIsInRelease|GoListDeps' -v .`: both pass (guard 7.3 s).
- Plan Task 2 shell checks (release `go list` contains `internal/webserver`, lacks webservertest, fixture graph contains it): pass.
- Fixture smoke (own fixture run, curl with `--cacert`): CA has a certificate and no private key; info 200; info-hits count 1; grid JSON has `resizes`; preview-pdf, share-off 2xx; old cap then 403; share-on returns a cap that gives 200; end 2xx; all six routes present in main.go. `SMOKE-OK`.
- `go build ./cmd/... ./internal/...` clean; `go test -count=1 ./internal/daemon/ ./internal/webserver/...` all ok; `bash tests/check-brand.sh` clean; `bash tests/check-traceability-paths.sh` OK (207 paths).
- The full gate was not run here (14-03 and 14-12 own it), so the internal/pty flake did not come up.

## Deviations from Plan

None. The plan was executed as written. Notes:

- The tracer gate for Task 1 was satisfied by the passing verify (no `gate="blocking-human"`).
- `.planning/WINDOWS.md` does not exist, so no ledger entries were appended (no stubs).
- CLAUDE.md's standing rule asks for TESTING.md and traceability updates for new test files; the plan explicitly assigns TESTING.md to 14-12 and 14-14, so it was left alone (the new test files are `internal/daemon/share_vmwhois_test.go` and `release_guard_test.go`).

## Auth gates

None.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-14-11 (release guard plus `fake-whois`), T-14-22 (tag only in tagged files, `vmwhois` word guarded in .github, build.sh and scripts, unknown mode fails closed) and T-14-23 (loopback admin, CA key never written) are mitigated as planned.

## Not verified

- The `vmwhois` binary was never run against a real tailscaled (host rule, and the VM run is 14-12). Only the wrapper type is tested, with a fake inner client; the `init()` install into `defaultNewShareServer` is not covered by a test, as with `vmpreview`.
- `/__test__/end` was checked for a 2xx only; a live WebSocket receiving the "session ended" close reason was not observed (reasoned from hub Run EOF to Shutdown to write pump on `hub.Done`).
- `/__test__/grid` was not seen with non-zero values (needs a viewer sending a grid) and `preview-pdf` was not seen reaching a viewer (0 viewers in the smoke).
- Whether the iOS Simulator accepts the fixture CA: 14-07 and 14-13.
- Full Go suite and TESTING.md update not done (14-12, 14-14).

## Self-Check: PASSED

Created files present (share_vmwhois.go, share_vmwhois_test.go, release_guard_test.go); commits `155707d2`, `6003e93a` and `19f3d657` are ancestors of HEAD.
