---
phase: 04-one-click-share
plan: 06
subsystem: sharing-ui
tags: [react, wails, web, ci, removal-gate, docs]
status: complete

requires:
  - phase: 04-one-click-share
    provides: "04-03: relay close reasons, /api/sessions/{id}/info, web/link-invalid.html; 04-04: ConnectPeerSession binding; 04-05: share popover, stale stubs left for this plan"
provides:
  - "Codeless peer tabs through ConnectPeerSession, connect-failed banner and retry"
  - "lib/closeReasons.ts (CLOSE_REASON_SHARE_STOPPED / CLOSE_REASON_SESSION_ENDED), pinned to the Go literals"
  - "Stopped-sharing banner for peer tabs and the web page; stdin disabled on any guest close; row dropped at once"
  - "LinkNoLongerWorks in /app/ for missing, invalid or revoked caps"
  - "tests/check-share-removal.sh (SC4 gate) running in build.yml"
  - "components/__tests__/appHarness.ts: mounts the real App on a faked Wails bridge"
affects: [phase verification, M-79 VM check, M-80 two-Mac UAT, M-77]

actuals:
  tokens: 36400
  tasks: 3
  commits: 3
plan_head_before: a9123812516c323c7fe425330437b70af2200456
plan_head_after: cc3ebb449264251343fc51f58704c8221086d3df

tech-stack:
  added: []
  patterns:
    - "Real-App render tests: window.go.main.App is a Proxy resolving per-method fakes, window.runtime a no-op Proxy, WebShareSessionView stubbed"
    - "Banner copy chosen by a close-reason constant, never wire text"

key-files:
  created:
    - frontend/src/lib/closeReasons.ts
    - frontend/src/lib/__tests__/closeReasons.test.ts
    - frontend/src/components/LinkNoLongerWorks.tsx
    - frontend/src/components/__tests__/LinkNoLongerWorks.test.tsx
    - frontend/src/components/__tests__/App.webLink.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx
    - frontend/src/components/__tests__/appHarness.ts
    - tests/check-share-removal.sh
  modified:
    - frontend/src/App.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/WebShareSessionView.tsx
    - frontend/src/components/SessionEndedBanner.tsx
    - frontend/src/lib/remoteSession.ts
    - frontend/src/lib/remoteAdapter.ts
    - frontend/src/style.css
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/components/__tests__/App.open-remote.test.tsx
    - frontend/src/components/__tests__/SessionEndedBanner.test.tsx
    - frontend/src/components/__tests__/WebShareSessionView.test.tsx
    - frontend/src/lib/__tests__/remoteSession.test.ts
    - frontend/src/lib/__tests__/remoteAdapter.test.ts
    - internal/webserver/server.go
    - internal/webserver/whois.go
    - .github/workflows/build.yml
    - README.md
    - CHANGELOG.md
    - .claude/CLAUDE.md
    - .planning/PROJECT.md
    - TESTING.md
  deleted:
    - frontend/src/components/RemoteJoinCodeModal.tsx
    - frontend/src/components/RemoteJoinCodeModal.test.tsx
    - frontend/src/components/__tests__/RemoteJoinCodeModal.test.tsx

key-decisions:
  - "Clicking the row of an open, working peer tab only activates it; ConnectPeerSession runs only for a new tab or a connect-failed one (no needless cap fetch)"
  - "Tab gains peerHost (sidebar group key, banner host) and connectFailed; a retry replaces the tab's fields and keeps its label"
  - "A rejected binding or malformed URL counts as a failed connect (same banner)"
  - "The removal gate's public-write and view-only patterns were narrowed after the dry run: a TS mock's `public write` field and the kept capability model's `read-only cap/viewer` comments are not removed features"
  - "tests/fixtures/settings_legacy_v4.json is out of the gate's scope: it must spell the old reminder keys so the migration test can pin their removal (same reason as *_removed_test.go)"
  - "No vmpreview WhoIs fake added: M-79's browser path (/app/, /api/sessions/{id}/info, the link WebSocket) is cap-gated, not same-user gated; only /api/sessions/meta and the peer cap route use WhoIs"

patterns-established:
  - "appHarness.mountApp(overrides) for behavior tests of App instead of source slicing"

requirements-completed: [SHAR-01, SHAR-02, SHAR-03, SHAR-04, SHAR-05, SHAR-06]

coverage:
  - id: D1
    description: "Peer row click opens <host> › <session> through ConnectPeerSession, no dialog; errorCode shows the connect-failed banner; second click retries"
    covered_by: [frontend/src/components/__tests__/App.open-remote.test.tsx]
  - id: D2
    description: "Share-stopped close: stopped-sharing banner, stdin disabled, onShareStopped drops the row; session-ended keeps its banner"
    covered_by: [frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx, frontend/src/components/__tests__/SessionEndedBanner.test.tsx, frontend/src/components/__tests__/App.open-remote.test.tsx, frontend/src/lib/__tests__/closeReasons.test.ts]
  - id: D3
    description: "Web page: 401/403/missing cap shows Link No Longer Works with no tab and no session id; 200 opens the tab with the info host"
    covered_by: [frontend/src/components/__tests__/App.webLink.test.tsx, frontend/src/components/__tests__/LinkNoLongerWorks.test.tsx, frontend/src/components/__tests__/WebShareSessionView.test.tsx]
  - id: D4
    description: "SC4 removal gate clean and in CI; go-qrcode out of go.mod"
    covered_by: [tests/check-share-removal.sh]
  - id: D5
    description: "Browser link states in the built app; two-Mac flow"
    covered_by: [TESTING.md M-79 (backstop, VM), TESTING.md M-80 (backstop, deferred UAT)]

metrics:
  duration: 11min
  completed: 2026-11-16
---

# Phase 4 Plan 06: Peer Tabs, Web Link States and the Removal Gate Summary

Clicking a peer row now opens its tab at once through ConnectPeerSession, with no code and no dialog. A failed connect leaves the tab open with the warning banner, and clicking the row again retries. When the host stops sharing, a peer tab or browser viewer keeps its scrollback, stops taking input and says `<host> stopped sharing this session.` (the peer's sidebar row disappears at once). The /app/ page checks the cap first and shows `Link No Longer Works` for dead or incomplete links. The join dialog and its last stubs are deleted. `tests/check-share-removal.sh` proves SC4 and runs in CI, and the docs now describe one Share switch.

## Removal gate output (final tree)

```
$ bash tests/check-share-removal.sh
OK: share removal gate clean
```

The first dry run had 23 findings: 1 leftover doc line (.claude/CLAUDE.md), 2 comments that still mentioned codes (server.go, whois.go), 4 TESTING.md Section 5 lines (M-53, M-57, M-67), 2 legacy-fixture keys (now out of scope, see decisions), 3 TS mock `public write = vi.fn()` fields and 11 comments about the kept read-only capability model. The last two groups meant the patterns were too broad, so they were narrowed. The plan's own Sections 4-5 acceptance awk, which still uses the broad patterns, prints nothing.

## Full phase gate (final tree, HEAD cc3ebb44 before this docs commit)

| Check | Result |
|-------|--------|
| `bash tests/check-share-removal.sh` | OK: share removal gate clean |
| gate never matches itself (`f[u]nnel\|j[o]in…\|q[r]…` grep on the script) | no match |
| `bash tests/check-brand.sh` | OK: brand gate clean |
| `grep -n 'skip2/go-q[r]code' go.mod go.sum` | no match |
| build.yml runs the gate; parses as YAML (ruby) | yes |
| `bash scripts/grep-gate.sh` | PASS: all action refs are SHA-pinned |
| `go build ./... && go vet ./...` | clean |
| `go vet -tags playwrightfixture ./cmd/playwright-fixture/`, `go vet -tags vmpreview ./...` | clean |
| `go test -race -short ./...` | all ok (no flakes this run); `pgrep -f '\.test daemon'` empty |
| `pnpm -C frontend install --frozen-lockfile` | lockfile unchanged |
| `tsc --noEmit` | clean |
| `vitest run` | 82 files, 1036 tests passed |
| `playwright test --project=chromium` | 6 passed, 9 skipped (the documented `test.skip` specs) |
| `bash tests/build-script.test.sh` | 37 passed, 0 failed |
| `bash tests/check-traceability-paths.sh` | OK: 108 traceability paths exist |
| `bash scripts/verify-icons.sh` | 27 passed, 0 failed |
| `wails build -platform darwin/arm64 -tags wailsassets` | built Portty.app; the regenerated wailsjs/wailsjs only added TerminalFont, PeerConnectResult and ShareResult (the same additive drift 04-05 saw); restored with `git checkout -- frontend/src/wailsjs/wailsjs/`, tree clean |
| removed files (`git ls-files` on the nine paths) | none tracked |

## TESTING.md

- Counts: vitest 80 → 82 (−2 join-dialog tests, +4: closeReasons, TerminalPanel.guestClose, LinkNoLongerWorks, App.webLink), total 197 → 199.
- Section 4: FIX-03 open-contract rows collapsed into one, NAV-02 rewritten, 9 SHAR rows added, including the two gate rows.
- Section 5: M-03 and M-04 rewritten for codeless opening; Category G (M-13) retired into M-04; M-53, M-57 and M-67 reworded; new Category AS with **M-79** (browser link states, VM) and **M-80** (two-Mac flow, deferred UAT).
- CI Workflow Mapping names the new gate.

**M-79 result:** not run by this executor (the orchestrator runs it in the Tart VM with the vmpreview build).

## Manual follow-ups for the user

- **M-77** and **M-80** on both Macs. Both need the new build installed on both Macs at the same time.
- **M-79** and **M-81** in the Tart VM (orchestrator).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The vitest output is coloured, so the plan's `grep -E 'Test Files +3 passed'` never matched**
- **Found during:** Task 1 verify
- **Issue:** The shell has COLORTERM=truecolor, so vitest puts ANSI codes between "Test Files" and "3 passed".
- **Fix:** The verify runs used `NO_COLOR=1`. No code change.

**2. [Rule 2 - Test placement] The guest-close tests are in a new file `TerminalPanel.guestClose.test.tsx`, not `TerminalPanel.test.tsx`**
- **Reason:** TerminalPanel.test.tsx only inspects source and never mocks xterm. The close path has to be rendered with xterm and RelayClient mocked, and that mock set would change how the existing file runs. The new file copies the TerminalPanel.contextLoss mock set.
- **Commit:** 0aacbf20

**3. [Rule 2] The App behavior tests mount the real App**
- **Issue:** Every App test so far sliced the App source as text, which cannot prove "one click calls ConnectPeerSession once and opens a tab".
- **Fix:** New non-test `components/__tests__/appHarness.ts` fakes the Wails bridge on `window`. App.open-remote and App.webLink use it.
- **Commit:** 0aacbf20, 2b66f026

**4. [Rule 1] The removal gate's patterns were narrowed after the dry run**
- **Patterns:** `public[ _-]?wr[i]te` became `public[_-]?wr[i]te|public wr[i]te gate`, and `r[e]ad-only (…|cap|view)` became `r[e]ad-only (link|share|tab)`. The scope also excludes the legacy settings fixture.
- **Reason:** The plan says the gate must not target the kept capability model. A TS class field `public write` is not the public-write gate.
- **Commit:** cc3ebb44

**5. [Rule 1] Stale Section 5 text the gate flagged, reworded**
- **Items:** M-53 (named the removed public route and join page), M-57 (named the removed entry points and the old web-server start test, now `TestShare_TailscaleOff`) and M-67 (the join page's Light/Dark check is now on the dead-link page).
- **Commit:** cc3ebb44

**6. remoteSession.ts: only `remoteBaseURLFor` was removed**
- `findRemoteSession` and `isRemoteSessionId` were left alone (Chesterton). Production code doesn't use them, but they have nothing to do with codes and the plan did not ask for their removal.

**7. No vmpreview WhoIs fake** (see key-decisions). M-79 needs none.

## Known Stubs

None.

## Threat Flags

None. T-04-24: banner copy is chosen by the CLOSE_REASON constant and the host comes from local tab state or the cap-verified /info response, rendered as React text (SessionEndedBanner test: the reason is never rendered). T-04-25: LinkNoLongerWorks takes no props, and App.webLink asserts the session id never appears. T-04-26: stdin is disabled on any guest close (guestClose test). T-04-27: the gate runs in build.yml. T-04-SC: no packages were added, and the frozen install passed.

## Self-Check: PASSED

- FOUND: frontend/src/lib/closeReasons.ts, frontend/src/components/LinkNoLongerWorks.tsx, tests/check-share-removal.sh, frontend/src/components/__tests__/appHarness.ts, App.webLink.test.tsx, TerminalPanel.guestClose.test.tsx, LinkNoLongerWorks.test.tsx, closeReasons.test.ts
- FOUND: 0aacbf20, 2b66f026, cc3ebb44 (ancestors of HEAD)
