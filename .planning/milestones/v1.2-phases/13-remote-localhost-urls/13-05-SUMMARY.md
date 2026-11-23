---
phase: 13-remote-localhost-urls
plan: 05
subsystem: frontend-viewer
tags: [react, xterm, web-links, consent-sheet, notices, wails-binding]
requires:
  - phase: 13-02
    provides: the shared classifier fixture localhost-url.json
  - phase: 13-04
    provides: App.OpenRemotePort (Go side; this plan adds the hand-maintained TypeScript stub)
provides:
  - classifyLoopbackUrl and trimLinkTail (frontend/src/lib/localhostUrl.ts)
  - OpenRemotePort and RemotePortResult stubs in wailsjs/go/main/App.d.ts and App.js
  - the desktop peer tab branch in TerminalPanel's web-links handler, the consent sheet and the port notice
  - five SessionEndedBanner kinds (port-host-outdated, port-daemon-outdated, port-in-use, port-refused, port-unreachable)
affects: [13-06 revoke and streams, 13-07 TESTING.md rows and VM checks]
status: complete
actuals:
  tokens: 14500
  tasks: 3
  commits: 2
plan_head_before: 6c2c83f7348fdd066f0f7754180f36b9da7ad3c5
plan_head_after: ad23fca11937848642e5ed1fb9c7880e7cc9630c
tech-stack:
  added: []
  patterns:
    - "link handler built once reads the tab kind and the remote-port flow through refs, so the hot-swap effect's dependency array is unchanged"
    - "portNotice is a sibling of pathNotice: same slot, same PATH_NOTICE_MS, same yielding rules, showing one clears the other"
    - "consent sheet is RegenerateKeyModal with copy overrides, portal-rendered into document.body"
key-files:
  created:
    - frontend/src/lib/localhostUrl.ts
    - frontend/src/lib/__tests__/localhostUrl.test.ts
    - frontend/src/components/__tests__/TerminalPanel.remotePort.test.tsx
  modified:
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/SessionEndedBanner.tsx
    - frontend/src/components/__tests__/SessionEndedBanner.test.tsx
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
key-decisions:
  - "Whitespace and control refusal in the TypeScript classifier uses the Unicode properties White_Space and Cc, which are exactly what Go's unicode.IsSpace and unicode.IsControl test, instead of a hand-listed code-point class"
  - "A consent answer to the second (consent true) call ends in the unreachable notice rather than reopening the sheet, so a misbehaving binding cannot loop the user through sheets"
requirements-completed: []  # URL-01, URL-02 and URL-03 are verified end to end in 13-07; the plan forbids requirements.mark-complete
---

# Phase 13 Plan 05: Viewer click branch, consent sheet and notices Summary

A Cmd-click on a printed `localhost` / `127.0.0.1` / `[::1]` / `0.0.0.0` / `[::]` URL in a desktop peer tab now asks `OpenRemotePort` to open it, shows a macOS-style consent sheet when the binding asks, and opens in the default browser only the `http://localhost:PORT...` URL the binding returns; every failure is one of five fixed notices in the clicking pane.

## Accomplishments

- `localhostUrl.ts`: `classifyLoopbackUrl` and `trimLinkTail` implement the shared classifier rules without `new URL`; pinned by the 28-case fixture.
- Binding stub: `RemotePortResult` and `OpenRemotePort(sessionID, rawURL, consent)` in `App.d.ts` and `App.js`, next to `ConnectPeerSession`.
- `TerminalPanel.tsx`: the branch sits inside the existing `WebLinksAddon` handler after the scheme check and the modifier gate and before `getRisk`; it fires only when `remote && !wsURL` and the classifier matches. Flow: `OpenRemotePort(sessionId, trimmedUrl, false)`; `open` calls `openLink(result.url)`; `consent` shows the sheet; any other answer or a rejection shows the notice. A ref guard keeps one flow per pane. No new link provider; `remote` is not in the hot-swap dependency array.
- Consent sheet: `RegenerateKeyModal` with the CONTEXT copy (title `Allow access to {host}'s localhost:{port}?`, the fixed body, `Don't Allow` / `Allow`), in a `createPortal` to `document.body`; Cancel is focused first and Esc cancels through `useDialogFocus`. Allow calls again with consent true; Don't Allow and Esc make no further call.
- Notices: `portNotice` with its own `PATH_NOTICE_MS` timer, cleared by the same conditions as `pathNotice`, rendered in the same slot through `SessionEndedBanner` with the host label and a numeric port.
- `SessionEndedBanner.tsx`: five new kinds, optional `port` prop, Dismiss rule unchanged.

## Tests added or edited

- `frontend/src/lib/__tests__/localhostUrl.test.ts` (new, 32 tests): every fixture case (28 loaded), trim of balanced and unbalanced `)`, null for empty, `http://`, 10,000-character and whitespace/control input.
- `frontend/src/components/__tests__/TerminalPanel.remotePort.test.tsx` (new, 27 tests): routing (`http://[::]:8000/)` gives one call with the trimmed URL and opens only `http://localhost:8000/`; no modifier calls nothing; all five forms), consent (title, body, buttons, focus on Don't Allow, portal, Allow, Don't Allow, Esc, Allow then port_in_use, second consent), notices (8 codes plus a rejection under fake timers, pane-only), in-flight guard, and URL-03 regressions (local tab, browser share view, `https://example.com/`, `https://localhost:8000/`, `http://example.com:8000/`, `http://LOCALHOST:8000/`, punycode popover).
- `frontend/src/components/__tests__/SessionEndedBanner.test.tsx` (edited, +15 tests): exact copy, host fallback and Dismiss for each of the five kinds.
- No existing link test file was touched.

## Verification

- Log directory (private `mktemp -d`): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.5TPeG6iIXl` (gate), plus earlier per-step directories under the same parent.
- Gate: `pnpm --dir frontend vitest run` rc 0, 110 files and 1674 tests passed; `tsc --noEmit` rc 0; `tests/check-brand.sh` and `tests/check-share-removal.sh` rc 0.
- Existing link tests (TerminalPanel.web-links, pathClick, RegenerateKeyModal and the six link test files): green; `git diff 9574fd49` over the six link test files has no removed `expect(` line; `internal/webserver/web_links_test.go` and `frontend/index.html` unchanged.
- Mutation checks (each reverted, tree clean afterwards): dropping the `remote && !wsURL` condition fails the local-tab and browser-view regressions; making Don't Allow call the binding fails the Don't Allow and Esc tests.
- Tracer gate (Task 1): the routing test was green together with the web-links and path-click files before the sheet and notices were wired.

## Deviations from Plan

### Auto-fixed Issues

None for Rules 1-3.

### Process deviation

**1. Tasks 1 and 2 landed in one implementation commit, tests in a second.** The plan asks for a commit per task with the tracer proven before expansion. I wrote the panel branch, the sheet and the notices in one pass, then the tests, so the history is two commits (`feat` and `test`) instead of three-plus. The tracer behavior (routing test, web-links and path-click files green) was confirmed before the sheet and notice tests were added, but not as a separate commit. No RED commit exists for Tasks 2 and 3; the mutation checks above stand in as evidence the tests can fail.

## Known Stubs

None.

## Issues and not verified

- No GUI was driven (host or VM): focus, Esc and portal clipping are asserted in jsdom only. 13-07 owns the VM checks.
- `TESTING.md` was not updated for the three new test files (Section 2 manifest, Section 4 traceability rows); 13-07 is the plan that owns it, and `tests/check-traceability-paths.sh` was not run.
- The `OpenRemotePort` stub is hand-maintained; it was checked against `app.go`'s `RemotePortResult` by reading, not by a generated binding.

## Commits

- bf23671b feat(13-05): a Cmd-click on a remote localhost URL opens the host's page after asking first
- ad23fca1 test(13-05): routing, consent, notices and unchanged links for remote localhost URLs

## Self-Check: PASSED
