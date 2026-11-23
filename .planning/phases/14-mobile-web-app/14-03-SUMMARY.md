---
phase: 14-mobile-web-app
plan: 03
subsystem: webserver
tags: [go, tailscale, whois, same-account-gate, static-pages, cache-control, playwright]
requires:
  - phase: 14-01
    provides: webservertest WhoIs fake, requireSameUser on the API routes, Playwright POST /__test__/whois
provides:
  - requireSameUserPage and sameUserStatus (internal/webserver/whois.go)
  - web/user-refused.html (403 + X-Portty) and web/service-retry.html (503, Try again link), embedded
  - GET /app/ and GET /sessions/{id} behind cspHeaders(requireSameUserPage(...))
  - a.button rule in web/assets/dashboard.css
  - /app/ files outside assets/ served Cache-Control: no-cache
affects: [14-04, 14-05, 14-12]
status: complete
actuals:
  tokens: 5500
  tasks: 3
  commits: 3
plan_head_before: 81bb0d646f7f92379fe00a883282e576fe84881a
plan_head_after: bd06729f74f0e6ae1cc1ce8d7f18610d89f51c79
tech-stack:
  added: []
  patterns:
    - "one same-user decision (sameUserStatus) with two writers: plain-text for the API, fixed static pages for the browser routes"
key-files:
  created:
    - web/user-refused.html
    - web/service-retry.html
    - internal/webserver/app_static_test.go
    - frontend/e2e/web-same-user.spec.ts
  modified:
    - internal/webserver/whois.go
    - internal/webserver/server.go
    - internal/webserver/whois_gate_test.go
    - web/embed.go
    - web/assets/dashboard.css
key-decisions:
  - "requireSameUser's body moved unchanged into sameUserStatus (0 / 403 / 503); requireSameUser and requireSameUserPage are thin writers over it"
requirements-completed: []  # MOB-05 still needs 14-04 (the SPA's reaction); MOB-02 needs the manifest and icons from later plans. requirements.mark-complete was not run.
---

# Phase 14 Plan 03: Same-account gate on the page routes Summary

A browser on another Tailscale account (or a tagged, shared-in or unknown node) that opens `/app/` or `/sessions/{id}` now gets 403 with `X-Portty: 1` and a fixed "This link only works on your own devices" page, before the SPA, any `/info` request or any WebSocket; a tailscaled fault gets 503 with no marker and a "Can't check this device right now" page with a Try again link. Files under `/app/` outside `assets/` are no-cache.

## Accomplishments

- **Gate.** `sameUserStatus(r)` holds the old `requireSameUser` logic verbatim (3 s timeout, fail-closed branches, numeric-ID debug log). `requireSameUser` behaves as before (existing `SameUserGate_*`, `WhoIs`, `SameUser` tests pass unchanged). `requireSameUserPage` writes 403 + marker + `userRefusedPage`, or 503 + `serviceRetryPage` with no marker, with `Content-Type: text/html; charset=utf-8` and `nosniff`. Page bytes are read once from `webfs.WebFS` at init (panic if missing), via a small `mustWebFile` helper.
- **Routes.** `GET /app/` = `cspHeaders(requireSameUserPage(handler))`; `GET /sessions/{id}` = `cspHeaders(requireSameUserPage(requireCapabilityPage(handleTerminalPage)))`. `/assets/` stays open so the pages load `dashboard.css`. Route comments updated.
- **Pages.** Both copy `link-invalid.html`'s structure; refusal has no button, link or script; retry has `<a class="button" href="">Try again</a>`. Nothing from the request is rendered.
- **CSS.** `a.button` (padding 12px 20px, min-height 44px, same tokens as `button`, display block, centered, no underline).
- **Cache.** In the `/app/` handler, `Del("Cache-Control")` is now only for `assets/`; every other existing file gets `no-cache`. Both comments say which files get which policy and why.

## Final Cache-Control per /app/ file class

| File class | Cache-Control |
|------------|---------------|
| `/app/`, `/app/?session=...`, SPA fallback (index.html) | `no-store` |
| `/app/assets/*` (hashed bundles) | none (cached by content hash) |
| `/app/manifest.json`, `/app/icon-192.png`, `/app/apple-touch-icon.png` and any other file | `no-cache` |

`/app/manifest.json` serves `Content-Type: application/json`.

## Results

Log directory (from `mktemp -d`): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.eBa6AbcwgR` (t1-vet, t1-gate, t1-e2e, t2-gate, t2-e2e, t3-static, t3-full).

- `go vet ./internal/webserver/ ./web/`: clean. `gofmt -l`: clean.
- Task 1 gate run (`SameUserGate|WhoIs|SameUser`): pass, no FAIL. `TestSameUserGate_AppPage` pass.
- Task 2 gate run (`SameUserGate|LinkPage|CapabilityPage`): pass. `TestSameUserGate_LinkPage`, `_AppPage`, `_PagesNameNoAccount` all pass.
- Task 3 (`AppStatic|AppBundle|CSPHeaderStrict`): `TestAppStatic_CacheHeaders` and the existing `TestAppBundle_HashedAsset_CacheableNotNoStore` pass.
- Full Go suite `go test -race -short -count=1 ./...`, run once: exit 0, 14 `ok` packages, no FAIL. The G-1 `internal/pty` flake did not occur.
- Playwright `web-same-user` (3 tests, run after Task 1 with 2 tests, and after Task 2 with 3): chromium 3 passed, firefox 3 passed, webkit 3 passed (9 passed, 0 failed). Covers the refusal page (403, `x-portty: 1`, heading, no `.xterm`, zero WebSockets, no `/api/sessions/` request), the retry page (503, no marker, empty `href`, link height at least 44 px, click after switching to `same` loads `.xterm`), and the same-account open.

## Deviations from Plan

**1. [Ordering] `service-retry.html` was created and embedded in the Task 1 commit.** `whois.go` reads both page files at init (panic on a missing file), so the package cannot build with only the refusal page. Task 2 added the CSS, the `/sessions/{id}` gate, the tests and the e2e case.

**2. [Process] `server.go` hunks split per task with `git update-index --cacheinfo`** (interactive `git add -p` is unavailable), so each commit contains only its own server.go changes. The working tree and final HEAD match the full edit.

**3. [Test coverage] The Go page tests assert the Content-Security-Policy contains `default-src 'none'`** rather than the exact string (14-05 changes it), as the plan says.

No Rule 1-4 deviations beyond the above.

## Known Stubs

None.

## Threat Flags

None. No new surface beyond the planned gated routes and two static pages.

## Not verified

- Another Tailscale account against a real tailscaled: only the `webservertest` fake was used (host tailscaled and 100.x addresses were never touched). The two-Mac check belongs to 14-12 / 14-14.
- The refusal and retry pages were checked by DOM assertions in three desktop browsers, not on a 375 pt phone viewport: the "nothing truncated, wraps inside 480 px / 1rem padding" claim rests on reusing the existing container CSS and was not visually inspected. No mobile Playwright project exists yet.
- The e2e 44 px check measures the link's bounding box height, not a touch target on a device.
- Dark-mode rendering of `a.button` was not inspected (it uses the same tokens as `button`).
- TESTING.md and the traceability row for `whois_gate_test.go`, `app_static_test.go` and `web-same-user.spec.ts` are not added (the plan assigns TESTING.md to 14-12 and 14-14; `tests/check-traceability-paths.sh` was not run).

## Self-Check: PASSED

- Files present: web/user-refused.html, web/service-retry.html, internal/webserver/app_static_test.go, frontend/e2e/web-same-user.spec.ts, and the modified whois.go, server.go, whois_gate_test.go, embed.go, dashboard.css.
- Commits in HEAD: e25e13d4, b019e2a8, bd06729f (`git rev-list 81bb0d64..HEAD` = 3).
- Self-check covers files and commits; the verify results above are from this run's logs.
