---
phase: 03-portty-brand
plan: 05
subsystem: in-app-and-web-text
status: complete
tags: [brand, web-pages, csp, basic-auth, quit-dialog, localstorage, test-fixtures]
requires:
  - "App identity plan (03-04): Portty.app, menus, tray and notification already say Portty"
provides:
  - "web/dashboard.html, web/join.html, web/terminal.html say Portty with the UI-SPEC copy (tagline, em-dash join title, Session Ended copy without agent wording)"
  - "internal/webserver/brand_pages_test.go: TestBrandPages_* checks the live /dashboard and /join routes (copy and CSP header) and the embedded terminal page"
  - "Basic auth realm Portty; self-signed cert organisation Portty Local"
  - "Quit Portty? dialog title, desktop document title Portty, LAST_DIR_KEY = 'portty:lastWorkDir'"
  - "No Go, Objective-C, TS, TSX, HTML, JS, CSS, plist or wails.json file contains the old name"
affects: [03-06]
tech-stack:
  added: []
  patterns: ["Brand text proven through the live route (testServer + client.Get), not only the embedded file", "Negative test assertions written as patterns, so the banned word is never spelled out"]
key-files:
  created:
    - internal/webserver/brand_pages_test.go
  modified:
    - web/dashboard.html
    - web/join.html
    - web/terminal.html
    - internal/webserver/auth.go
    - internal/webserver/auth_test.go
    - internal/webserver/selfcert.go
    - internal/webserver/app_bundle_test.go
    - frontend/index.html
    - frontend/src/components/QuitConfirmModal.tsx
    - frontend/src/components/__tests__/QuitConfirmModal.test.tsx
    - frontend/src/components/NewSessionModal.tsx
    - frontend/src/components/__tests__/NewSessionModal.test.tsx
    - frontend/src/components/__tests__/App.test.tsx
    - frontend/src/components/__tests__/RegenerateKeyModal.test.tsx
    - frontend/src/lib/__tests__/stripAnsi.test.ts
    - frontend/src/components/__tests__/UpdateBanner.test.tsx
    - frontend/e2e/progress.spec.ts
    - frontend/e2e/web-links-live-toggle.spec.ts
    - frontend/e2e/web-plugin-hot-swap.spec.ts
    - TESTING.md
decisions:
  - "Web pages change text only: the name Portty in the system font, with no logo image and no CSS or layout changes"
  - "The folder memory key becomes portty:lastWorkDir with no migration, so the last-used folder resets once to home (CONTEXT fresh start)"
  - "App.test.tsx bans any quoted namespaced theme storage key with one regex, so App.tsx's const terminalTheme does not match and the old name is not spelled out"
metrics:
  duration: "~15 min"
  completed: 2026-11-16
  tasks: 3
  files: 21
estimate:
  tokens: 55000
actuals:
  tokens: 5800
  tasks: 3
  commits: 3
plan_head_before: a6512d5a89c4051f101209e0be5cc156c86e70c6
plan_head_after: 03c160b543af0ba27803ea6cb80489ae556945aa
---

# Phase 3 Plan 05: Portty In-App and Web Text Summary

A browser on the tailnet now gets Portty pages. The dashboard shows the tagline "Terminal sessions shared over your tailnet.", the join page title is "Join Session — Portty" with five Portty headings, and the Session Ended copy no longer mentions an agent. A new Go test checks this through the live routes and confirms the CSP header is still sent. On the desktop, the Quit dialog says "Quit Portty?", the document title is Portty, and New Session remembers folders under `portty:lastWorkDir`. After this plan, no source file in any language contains the old name.

## What was built

**Task 1 (tracer), 9fc2ae56**
- web/dashboard.html: `<title>Portty</title>`, `<h1>Portty</h1>`, tagline `Terminal sessions shared over your tailnet.`
- web/join.html: `<title>Join Session — Portty</title>` (em dash), all five state headings `<h1>Portty</h1>`, and the State E copy `This session is no longer available. The shell may have exited, or the owner stopped sharing it.`
- web/terminal.html: `<title>Portty</title>`.
- internal/webserver/brand_pages_test.go (new): a `getPage` helper GETs a live route through `testServer`, requires a 200 and a non-empty Content-Security-Policy header, and returns the body. It backs four tests: TestBrandPages_DashboardSaysPortty, TestBrandPages_JoinSaysPortty (exactly five headings by `strings.Count`), TestBrandPages_TerminalPageSaysPortty (`webfs.WebFS`), and TestBrandPages_NoAgentWording (a case-insensitive `agent` regex over all three embedded pages).
- auth.go: realm Portty in both the doc comment and the header, and auth_test.go expects `Basic realm="Portty"`. selfcert.go: Organization `Portty Local`. app_bundle_test.go fixture title is Portty.

**Task 2, 8c084544**
- QuitConfirmModal.tsx: the title is `Quit Portty?`. The id, class, aria wiring and buttons are unchanged, and both the source and rendered-title assertions were updated.
- frontend/index.html: `<title>Portty</title>`.
- NewSessionModal.tsx: `LAST_DIR_KEY = 'portty:lastWorkDir'`, with no code that reads the old key. The test is renamed "uses the portty:lastWorkDir key", and the seed and read-back tests use the new key.

**Task 3, 03c160b5**
- App.test.tsx: the two old-name literals are replaced by `expect(raw).not.toMatch(/['"][a-z]+:(terminalTheme|uiTheme)['"]/)`.
- RegenerateKeyModal.test.tsx body prop, stripAnsi.test.ts sample prompt (`Portty $ echo hello`), and UpdateBanner.test.tsx releaseURL `https://github.com/scottkw/portty/releases/tag/v2.0.0`.
- e2e comments: "a running Portty instance" (two specs) and "See: upstream issue #112".
- TESTING.md: Go **110**, total **209**, a 03-05 note in Section 2, and four Section 4 rows (BRND-04 brand_pages_test.go, BRND-01 QuitConfirmModal, BRND-03 NewSessionModal, BRND-06 UpdateBanner).

## Gate results

| Gate | Result |
|------|--------|
| Task 1 `go build ./... && go vet ./... && go test -race -short ./internal/webserver/` | PASS |
| TestBrandPages_ / TestBasicAuthMiddleware_ | 8 PASS (needs 8): DashboardSaysPortty, JoinSaysPortty, TerminalPageSaysPortty, NoAgentWording, Unauthorized, WrongPassword, Authorized, EmptyUsername |
| Old name under `web/` and `internal/webserver/` (git grep) | exit 1 (no hits) |
| `agent` in the three HTML pages (grep -i) | exit 1 (no hits) |
| Task 2 tsc + vitest (QuitConfirmModal, NewSessionModal) | PASS, 2 files and 38 tests |
| Full `pnpm -C frontend exec tsc --noEmit && vitest run` | PASS, 91 files and 1173 tests |
| Full `go build ./... && go vet ./... && go test -race -short ./...` | PASS, every package `ok` (no flake hit) |
| Source-wide gate `git grep -l -i -I 'agent[h]ub' -- '*.go' '*.m' '*.ts' '*.tsx' '*.html' '*.js' '*.css' '*.plist' wails.json ':!.planning'` | prints nothing, exit 1 |
| `bash tests/check-traceability-paths.sh` | OK: 105 traceability paths exist |
| `git ls-files '*_test.go'` count | 110 |

Tracer feedback gate: the Task 1 `<verify>` is fully automated and passed end to end before Task 2 started.

Not run: `wails build`, because no native or bundle file changed. The skipped Playwright e2e specs were not run either, since only their comments changed.

## Deviations from Plan

**1. [Rule 1 - Bug] e2e comment grammar**
- **Found during:** Task 3
- **Issue:** the plan's literal replacement "a Portty instance" follows "requires a running" on the line above, which made "requires a running a Portty instance"
- **Fix:** the comments now read "requires a running Portty instance"
- **Files modified:** frontend/e2e/progress.spec.ts, frontend/e2e/web-links-live-toggle.spec.ts
- **Commit:** 03c160b5

No other deviations. The plan was executed as written.

## Orchestrator reminders

- BRND-05 stays open until the 03-06 zero-hit gate, and no requirements were marked complete here. Docs, CI, scripts, package metadata and older TESTING.md text still carry the old name and belong to 03-06.
- On first launch, the New Session folder resets to home once, by design (no migration).

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/webserver/brand_pages_test.go
- FOUND: 9fc2ae56, 8c084544, 03c160b5 (ancestors of HEAD)
