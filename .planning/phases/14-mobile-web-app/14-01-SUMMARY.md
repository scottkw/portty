---
phase: 14-mobile-web-app
plan: 01
subsystem: webserver
tags: [go, tailscale, whois, same-account-gate, playwright, test-fixture]
requires:
  - phase: 13
    provides: requireSameUser, whoIsClient seam, SetWhoIsClientForTest
provides:
  - internal/webserver/webservertest, the shared WhoIs fake (SameUser, New, Set, ParseIdentity; seven identities)
  - requireSameUser outermost on GET /api/sessions (discovery probe), /api/sessions/{id}/info, /api/plugin-config, /api/plugin-config/stream
  - WebSocket order Origin, then WhoIs, then capability
  - tailnet.IsPorttyProbeResponse exported
  - Playwright fixture: same-user fake, POST /__test__/whois, GET /__test__/input, POST /__test__/input/reset
affects: [14-03, 14-04, 14-06, 14-10, 14-12]
status: complete
actuals:
  tokens: 8600
  tasks: 3
  commits: 3
plan_head_before: 2153501a36be2cd43bcf39e39c502a899f115be5
plan_head_after: 1d07120a076ff46d0b934d90d87d3df75a4c7227
tech-stack:
  added: []
  patterns:
    - "one mutex-guarded WhoIs fake answers every caller as a chosen identity; helpers install it after NewWebServer and before Start"
    - "per-route wrapping (no global middleware) so a per-session hook for invited devices stays possible"
key-files:
  created:
    - internal/webserver/webservertest/whois.go
    - internal/webserver/whois_gate_test.go
  modified:
    - internal/webserver/server.go
    - internal/webserver/capability_helpers_test.go (git mv from capability_test_helpers.go)
    - internal/webserver/server_test.go
    - internal/webserver/inject_test.go
    - internal/webserver/origin_mw_test.go
    - internal/webserver/session_ended_test.go
    - internal/daemon/share_test.go
    - cmd_share_test.go
    - internal/tailnet/tailnet.go
    - cmd/playwright-fixture/main.go
key-decisions:
  - "No verdict cache: every gated request does its own StatusWithoutPeers and WhoIs; the WhoIs cost measurement is M-149 in 14-12 and the decision is the user's at 14-14"
  - "whois_test.go's fakeWhoIs, whoIsAs and SameUserWhoIsForTest left as they were (they back TestSameUser and portproxy_test.go)"
requirements-completed: []  # MOB-05 is only the server half for the API and WebSocket here; the page routes are 14-03 and the SPA's reaction is 14-04, so requirements.mark-complete was not run
---

# Phase 14 Plan 01: Same-account gate on the session API and WebSocket Summary

Another Tailscale account (or a tagged, shared-in or unknown node) now gets 403 with `X-Portty: 1` on the discovery probe, the info route, both plugin-config routes and the WebSocket before any capability work; a tailscaled fault gets 503 with no marker; the same account sees no change.

## Accomplishments

- **Shared fake** `internal/webserver/webservertest/whois.go`: `Same`, `Other` (user 7), `Tagged`, `SharedIn`, `NotFound`, `StatusFault`, `WhoIsFault`; `ParseIdentity` errors on an unknown name. Imports only tailscale.com types.
- **Gate** (`internal/webserver/server.go`): `ws.requireSameUser(ws.requireCapability(...))` on info and both plugin-config routes; `ws.requireSameUser(porttyMarker(ws.requireCapability(ws.handleListSessions)))` on the probe; `ws.requireAllowedOrigin(ws.requireSameUser(ws.requireCapability(ws.handleWSSRelay)))` on the WebSocket. Route doc comment and registration comments updated, including the accepted probe cost during a host tailscaled blip (peers drop the host group until the next refresh, 5-30 s; T-14-43).
- **Probe predicate** `tailnet.IsPorttyProbeResponse` exported (rename, two callers) so the gate test applies the probe's own rule to the real server's answers.
- **Helpers moved into the test build** (`git mv capability_test_helpers.go capability_helpers_test.go`): they were compiled into the release package; nothing outside `_test.go` referenced them. Header says why.
- **Tests** `internal/webserver/whois_gate_test.go`: `TestSameUserGate_SessionInfo`, `_APIRoutes`, `_WebSocket` (identities, order rows, no subscriber joins after a refusal via `Hub.SubscriberCount`), `_DiscoveryProbe` (same user no cap: 401 + marker, predicate true; other/tagged/shared-in/not-found: 403 + marker, predicate false; faults: 503, no marker, predicate false; same with cap: 200), `_OpenRoutes` (`/` 302, `/dashboard` 200, `/assets/dashboard.css` 200 as another user).
- **Fixture** `cmd/playwright-fixture/main.go`: same-user fake installed before `ws.Start()`; admin routes below; the PTY input reader is now drained.

## Fixture admin routes (loopback, build tag playwrightfixture)

| Route | Request | Response |
|-------|---------|----------|
| `POST /__test__/whois` | JSON `{"as":"same\|other\|tagged\|shared-in\|not-found\|status-fault\|whois-fault"}` | 200 empty; 400 for an unknown name or bad JSON |
| `GET /__test__/input` | none | 200 `{"hex":"<lowercase hex of every byte the hub wrote to the PTY stub since the last reset>"}` |
| `POST /__test__/input/reset` | none | 204 |

## Test files given a fake

| File | Why it served a gated route |
|------|-----------------------------|
| internal/webserver/capability_helpers_test.go (`testServer`, `testServerWithHub`) | capability tests hit the info, plugin-config, probe and WebSocket routes |
| internal/webserver/server_test.go (external `testServer`; `TestWebServerWSS`; `newWebTestHub`) | WebSocket dials (TestWebServerWSS, TestWebJoin_PushesResizeBeforeScrollback, TestWebReadPump_DropsGuestResize were 503 until fixed; the file-level guard did not catch the two sites because the file already named a fake elsewhere) |
| internal/webserver/inject_test.go | WebSocket dial |
| internal/webserver/session_ended_test.go | WebSocket dial |
| internal/webserver/origin_mw_test.go (two never-started servers) | serves no route; named only so the Task 2 guard holds and no test server defaults to a live lookup |
| internal/daemon/share_test.go | local `sameUserWhoIs` type deleted, replaced by `webservertest.SameUser()` |
| cmd_share_test.go (`loopbackShareServerForTest`) | share server serves the gated routes |

Left alone: `internal/webserver/tailscale_test.go` (pins the noLiveWhoIs default), `fuzz_test.go` (error fake), `portproxy_test.go`, `listener_test.go`, `whois_test.go`.

## Results

Log directories (from `mktemp -d`): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.cuo2xQ4vM2` (Task 1 and 2) and `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.njToVxCXCz` (Task 3).

- `go vet ./...` and `go vet -tags playwrightfixture ./cmd/playwright-fixture/`: clean.
- Gate tests: five `TestSameUserGate_*` pass. Mutation check: removing `requireSameUser` from the probe and the WebSocket makes the WebSocket refusal rows and the probe `other` row fail; restored.
- Full Go suite `go test -race -short -count=1 ./...`: the first run failed (the three server_test.go dials above, 503 from the noLiveWhoIs default); fixed; second run exit 0, 14 `ok` packages, no failures. The G-1 `internal/pty` `TestKillClean_NoOrphans` flake did not occur, so no re-run was needed.
- Chromium `web-share-scope` (Task 1 proof): 2 passed.
- Fixture smoke on loopback: whois other 200, nobody 400, same 200, input `{"hex":""}`, reset 204.
- Full Playwright suite: **not clean**. Run 1 (before Firefox was installed): 20 passed, 28 skipped, 12 failed (11 Firefox "Executable doesn't exist" for `firefox-1511`; 1 WebKit selection-offset flake). Run 2 (after installing Firefox): 30 passed, 29 skipped, 1 failed: Firefox `selection-offset.spec.ts:179` clipboard readback (`""` vs `"line-59 abcdefg"`), 3 of 3 on a repeat. Gated-route specs (web-share-scope, web-csp, web-vendor-parity, web-color-scheme) pass on chromium, Firefox and WebKit. Details and the unproven "pre-existing" status are in `deferred-items.md`.

## Deviations from Plan

**1. [Rule 3 - Blocking] `playwright install webkit` hung under Homebrew node 26.13.1**
- **Found during:** Task 1. The download reached 100 percent, then extraction stalled after the first file (0 percent CPU), reproduced twice.
- **Fix:** killed it, removed the partial `webkit-2272` directory, re-ran with nvm node v24.22.1 first on PATH: installed, exit 0. No package added; the browser build is the one pinned by `@playwright/test` 1.59.1.

**2. [Rule 3 - Blocking] Firefox build for 1.59.1 not installed**
- **Found during:** Task 3, first full Playwright run (11 launch failures).
- **Fix:** `playwright install firefox` (same node 24 workaround), `firefox-1511`, then the full suite ran a second time. The plan said full suites run at most once; this is the second run, forced by the missing browser.

**3. [Rule 1 - Bug] Two more test-built share servers in server_test.go needed the fake**
- **Found during:** Task 2 full Go run (`TestWebServerWSS`, `newWebTestHub` users got 503). The plan's guard counts files, and server_test.go already named a fake, so the guard passed vacuously for these sites. Fixed at both `NewWebServer` call sites; the full run was repeated (second run clean).

**4. [Process] Task 1 commit amended once.** The first `git add` failed on the already-renamed old path, so the commit captured only the rename; amended immediately (local, before any other commit) to include the six files.

**5. [Plan text] Fixture package doc.** The first attempt to add the admin-route entries silently did not match; the whois, input and reset entries were added in Task 3 (the Task 1 commit has the whois route but not its package-doc line).

**Not done on purpose:** `requirements.mark-complete MOB-05` (server half only; 14-03 and 14-04 finish it). TESTING.md untouched (plan: 14-12 and 14-14 own it); the repo's standing rule (traceability row for `whois_gate_test.go`, `bash tests/check-traceability-paths.sh`) is therefore pending for those plans.

## Known Stubs

None.

## Threat Flags

None. New surface is test-only (the fixture admin routes, behind the `playwrightfixture` build tag and loopback).

## Not verified

- Fixture input drain with real bytes: only the empty buffer, the reset and the status codes were exercised; no WebSocket client wrote input through the hub (14-10's key-bar tests are the first consumer).
- Fixture and tests never reached a live tailscaled or a 100.x address; no VM, no GUI was driven.

## Self-Check: PASSED

- Files present: webservertest/whois.go, whois_gate_test.go, capability_helpers_test.go, cmd/playwright-fixture/main.go; capability_test_helpers.go is no longer tracked.
- Commits found in HEAD: 3df54b69, 770bfedc, 1d07120a (`git rev-list 2153501a..HEAD` = 3).
- Self-check covers the files and commits only; the full Playwright suite is not clean (see Results and deferred-items.md).
