---
phase: 19-unattended-desktop-viewing
plan: 04
subsystem: desktop-sharing
tags: [capability-token, same-user-gate, csp, static-pages, cross-token]

requires:
  - phase: 19-01
    provides: /desktop/ page and /desktop/ws routes, desktopTokenOutcome, DesktopBackend
provides:
  - static first-load pages for a rejected desktop token and for desktop off, with fixed status and X-Portty-Desktop reason header
  - GET /desktop/check (204, or 401/410 with a JSON reason) behind the same-user gate
  - 429 before upgrade on /desktop/ws when the backend reports Full()
  - GET /desktop answering 301 with the query kept
  - TestCrossToken_Matrix (DSHR-08), desktop rows in the same-account gate table, a CSP pin, the manifest and caching contract
affects: [19-06, 19-08, 19-09, 21]

actuals:
  tokens: 14000
  tasks: 2
  commits: 2
plan_head_before: 542c37c015c5a91ded4db4b3024652f9f9a2d682
plan_head_after: 390a111e9196b1ef70571c092c8a0015b9922055

tech-stack:
  added: []
  patterns:
    - "One writeDesktopRefusal helper for every refused /desktop/ request: reason header + status + page or JSON"
    - "Cross-token matrix: one table of token kind x route, asserting the matching cell succeeds so a broken route cannot pass as refused"

key-files:
  created:
    - web/desktop-off.html
    - web/desktop-link-invalid.html
    - internal/webserver/desktop_routes_test.go
    - internal/webserver/crosstoken_test.go
  modified:
    - internal/webserver/desktop_routes.go
    - internal/webserver/whois_gate_test.go
    - web/embed.go
    - web/service-retry.html
    - internal/desktop/loopback_test.go

key-decisions:
  - "Status codes: 401 for a rejected token, 410 for desktop off; neither is 403, so the X-Portty marker stays on the user-refusal answer only"
  - "GET /desktop is an explicit 301 handler: ServeMux's own subtree redirect answers 307 on this toolchain, the contract is 301"
  - "The check endpoint has no Origin gate: it is a same-origin GET whose answer a cross-site page cannot read (no CORS answered), and it sits behind requireSameUser"

requirements-completed: [DSHR-08, DVWR-02, DSHR-02]

coverage:
  - id: D1
    description: "A first load with a rejected token gets 401 + X-Portty-Desktop: token and the invalid-link page; with desktop off it gets 410 + off and the off page; the pages differ and hold no script, link (other than the stylesheet), host or account"
    requirement: "DVWR-02"
    verification:
      - kind: integration
        ref: "internal/webserver/desktop_routes_test.go#TestDesktopRoutes_Outcomes"
        status: pass
    human_judgment: false
  - id: D2
    description: "GET /desktop/check answers 204, 401 {reason:token} and 410 {reason:off}; /desktop/ws refuses with the same codes and a full share gets 429 before any upgrade"
    requirement: "DVWR-02"
    verification:
      - kind: integration
        ref: "internal/webserver/desktop_routes_test.go#TestDesktopRoutes_Outcomes"
        status: pass
    human_judgment: false
  - id: D3
    description: "Session, port and desktop tokens are each refused on every other route family (desktop, session, API, port proxy), and each opens its own"
    requirement: "DSHR-08"
    verification:
      - kind: integration
        ref: "internal/webserver/crosstoken_test.go#TestCrossToken_Matrix"
        status: pass
    human_judgment: false
  - id: D4
    description: "Another Tailscale account gets the 403 user-refused page with the marker on /desktop/ and 403 on /desktop/check and /desktop/ws, before any token check; a tailscaled fault gets the 503 retry page"
    requirement: "DVWR-02"
    verification:
      - kind: integration
        ref: "internal/webserver/whois_gate_test.go#TestSameUserGate_DesktopRoutes"
        status: pass
    human_judgment: false
  - id: D5
    description: "The /desktop/ CSP equals /app/ and /sessions/{id} byte for byte with no media-src; csp_mw.go is unchanged since 89d29266"
    requirement: "DSHR-02"
    verification:
      - kind: integration
        ref: "internal/webserver/desktop_routes_test.go#TestDesktopRoutes_CSPPinned"
        status: pass
    human_judgment: false
  - id: D6
    description: "The manifest has scope ./ and display standalone with no start_url and no id, so a Home Screen launch opens the saved link; /desktop (no slash) redirects 301 with the query byte for byte; an assets/ directory URL serves the index, never a listing"
    requirement: "DSHR-02"
    verification:
      - kind: integration
        ref: "internal/webserver/desktop_routes_test.go#TestDesktopRoutes_ManifestAndCaching"
        status: pass
      - kind: integration
        ref: "internal/webserver/desktop_routes_test.go#TestDesktopRoutes_RedirectNoSlash"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-11-26
status: complete
---

# Phase 19 Plan 04: Desktop Route Gates Summary

**Every refused `/desktop/` request now answers with a fixed status and an `X-Portty-Desktop` reason code and a first load shows its own static page, a check endpoint lets the viewer tell the cases apart, and DSHR-08 is proven by a full token-kind-by-route matrix.**

## Accomplishments

- `writeDesktopRefusal`: 401 + `X-Portty-Desktop: token` (rejected, wrong kind or replaced grant) and 410 + `off` (desktop off, including no backend), with the matching static page on page routes and `{"reason":...}` JSON on API routes. Neither status is 403, so the `X-Portty` marker stays on the user refusal alone.
- `web/desktop-off.html` and `web/desktop-link-invalid.html` carry the UI-SPEC copy exactly; no script, no link but the stylesheet, nothing from the request. Both embedded in `web/embed.go`.
- `GET /desktop/check` behind `requireSameUser`: 204 for a good token, otherwise the same codes. `/desktop/ws` answers 429 before Accept when the backend asserts `Full() bool`.
- `web/service-retry.html` now reads "Tailscale on the Mac you're connecting to didn't answer. Wait a moment, then try again." for terminal and desktop routes.
- Tests: `TestDesktopRoutes_Outcomes`, `_NoBackendIsOff`, `_CSPPinned`, `_RedirectNoSlash`, `_ManifestAndCaching`, `TestCrossToken_Matrix` (3 kinds x 9 routes plus the port route's capability slot), `TestSameUserGate_DesktopRoutes`. `desktop_routes.go` statement coverage 93.2 percent. The new tests reuse the port-proxy env (`newPPEnv`), so one server holds the session, port and desktop routes together.
- Tests were mutation-checked: removing `requireSameUser` from the check route fails the gate table on every refusal row; making the desktop outcome accept a session capability fails the matrix on `/desktop/` and `/desktop/ws`.

## Task Commits

1. **Task 1 (tracer): static pages, reason header, check endpoint, capacity refusal** - `cda218ca` (feat)
2. **Task 2: DSHR-08 matrix, gate rows, CSP pin, manifest contract, 301 redirect** - `390a111e` (test)

## TDD Gate Compliance

Task 2 is `tdd="true"`, but its tests assert behaviour Task 1's tracer (and 19-01) already built, so they passed on first run except one. The one genuine RED was `TestDesktopRoutes_RedirectNoSlash`: it failed with "status 307, want 301" (the plan's own contingency: ServeMux's subtree redirect kept the query but answered 307). The fix, an explicit `GET /desktop` 301 handler, is in `390a111e` together with the test, not as a separate `feat` commit, so there is no separate `test(19-04)` before `feat(19-04)` (here `feat` comes first for Task 1, as in 19-01). For the "unexpected GREEN" cases I verified the tests are not vacuous by mutation (see Accomplishments), reverting each mutation by restoring a copy of the file.

## Deviations from Plan

**1. [Rule 3 - Blocking] `internal/desktop/loopback_test.go` pinned the old refusal**
- **Found during:** Task 1 verification. `TestDesktopWS_RefusesBeforeUpgrade` expected 401 for "link off" on `/desktop/ws`, which is now 410 by this plan's own contract.
- **Fix:** the one expectation changed to `http.StatusGone`. The file is not in the plan's `files_modified`; the change follows from the plan's status contract.
- **Commit:** `cda218ca`

**2. [Rule 1 - Bug] ServeMux subtree redirect answers 307, not 301**
- **Found during:** Task 2 (`TestDesktopRoutes_RedirectNoSlash`).
- **Fix:** explicit `GET /desktop` handler that redirects 301 to `/desktop/` + `r.URL.RawQuery`. Fixed same-origin Location, so not an open redirect.
- **Files modified:** `internal/webserver/desktop_routes.go`. **Commit:** `390a111e`

**3. [Rule 3 - Lint] Pre-existing errcheck at `whois_gate_test.go:172`**
- 19-01 flagged this unchecked `conn.CloseNow()` as a standing lint failure. This plan edits that file and its verify command requires a clean lint, so it is now `_ = conn.CloseNow()`. Lint is at 0 issues for `./internal/webserver/... ./web/...` against `89d29266`.

**Retry-page pin:** no Go or frontend test pinned the old "shared this session" sentence (the Go gate tests compare against the embedded page itself), so no test needed updating for the reword.

**Total deviations:** 3 auto-fixed. **Impact:** none on the contract 19-06 and Phase 21 read.

## Known Stubs

None. The `shortcut:` in `desktop_routes.go` from 19-01 is resolved. The `Full()` check depends on the backend implementing it (19-03 owns the viewer cap); until then the 429 path is exercised only through the test fake.

## Threat Flags

None beyond the plan's register. T-19-14 (matrix), T-19-15 (same-user gate rows, including bad-token-from-other-account order rows), T-19-16 (static pages hold nothing from the request; `no-store`/`no-referrer` from `cspHeaders`) and T-19-17 (CSP pin; `git diff 89d29266 -- internal/webserver/csp_mw.go` empty) are tested. T-19-SC: no package added.

## Verification

- `go vet`, `gofmt -l` clean; `go test -race -short -count=1 ./internal/webserver/ ./internal/desktop/ ./web/` pass; new tests pass at `-race -count=8`.
- `golangci-lint run --new-from-rev=89d29266 ./internal/webserver/... ./web/...`: 0 issues.
- `bash tests/check-brand.sh` clean. Acceptance greps: off page 1, invalid page 1, embed 1, retry wording 1, `GET /desktop/check` 1.
- TESTING.md was not touched (19-16 owns the consolidated update, which should list `crosstoken_test.go` and `desktop_routes_test.go` and traceability rows for DSHR-08, DVWR-02, DSHR-02). No STATE.md or ROADMAP.md changes. No VM work was needed or done.
- The browser half of the CSP check (a real page load with no violations) is 19-06 and 19-12's, as the plan says.

## Self-Check: PASSED

- Files exist: both static pages, `desktop_routes_test.go`, `crosstoken_test.go`.
- Commits `cda218ca`, `390a111e` are ancestors of HEAD.
