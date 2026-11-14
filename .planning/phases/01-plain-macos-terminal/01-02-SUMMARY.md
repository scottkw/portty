---
phase: 01-plain-macos-terminal
plan: 02
subsystem: frontend / web-share / e2e
tags: [strip-down, file-browser, web-share, share-dialog, codemirror, testing]
status: complete
requires: [01-01]
provides:
  - Frontend with no file browser/editor UI (no tab type, menu items, web-share file panel, remote-browse flow)
  - Share dialog without the browse toggle or home-dir warning; Tailnet link copy per UI-SPEC
  - Hand-maintained bindings without SetSessionBrowse, DownloadFile, homeDir, browseEnabled
  - CodeMirror-free dependency set on a removal-only pnpm v9.0 lockfile
  - Web-share e2e that asserts owner and viewer caps get no chevron and no file tab
affects: [01-03, 01-04, 01-05]
tech-stack:
  added: []
  removed: [codemirror, "@codemirror/* (16 packages)"]
  patterns: [outside-in removal (UI first, daemon API in 01-03)]
key-files:
  created: []
  modified:
    - frontend/src/App.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/Hub/SessionCard.tsx
    - frontend/src/components/Hub/SessionCardGrid.tsx
    - frontend/src/components/Hub/HubPanel.tsx
    - frontend/src/components/Hub/SessionShareModal.tsx
    - frontend/src/components/SessionShare/TailnetTab.tsx
    - frontend/src/components/RemoteJoinCodeModal.tsx
    - frontend/src/lib/remoteAdapter.ts
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/wailsjs/wailsjs/go/models.ts
    - frontend/src/style.css
    - frontend/src/content/help/faq.md
    - frontend/src/content/help/getting-started.md
    - frontend/package.json
    - frontend/pnpm-lock.yaml
    - frontend/e2e/web-share-scope.spec.ts
    - frontend/e2e/web-csp.spec.ts
    - frontend/e2e/fixture-env.ts
    - frontend/e2e/global-setup.ts
    - TESTING.md
key-decisions:
  - "Web-share guests get no session-menu chevron at all (owner or viewer cap); the webFilesEnabled prop and the /info perms probe in the web bootstrap are gone"
  - "Dropped the dead 'files' intent from RemoteJoinCodeModal and App's joinModalForSession type; only 'open-session' and 'hub-modal' remain"
  - "IssueCapabilitiesResponse.homeDir also removed from App.d.ts (the acceptance grep counts it); Go still sends it until 01-03, JSON extra fields are ignored"
  - "Deleted dead e2e helpers filesApiURL and e2e/fixtures/remote-peer-setup.ts (only the deleted files specs used them); FixtureEnv.remotePeerURL left for 01-03, which removes the fixture's remote peer"
  - "Kept the double-click-rename guard from the deleted 'files.read guest' e2e test by folding it into the owner/viewer chevron test"
requirements-completed: [STRP-02, STRP-04]
duration: 26 min
completed: 2026-11-14
actuals:
  tokens: 148000
  tasks: 3
  commits: 3
plan_head_before: abc0e0d224a59f3b46e9d8c9b919e0042cb9246c
plan_head_after: d7646de0ba8bfe913128997c53b4cd5c376cbfa1
coverage:
  - deliverable: "No file browser in desktop tabs, Hub cards or web-share guests; remote open and Hub modal still work"
    human_judgment: false
    verification:
      - {kind: command, ref: "pnpm -C frontend exec tsc --noEmit && pnpm -C frontend test (127 files / 2076 tests)", status: pass}
      - {kind: test, ref: "src/components/__tests__/App.open-remote.test.tsx + App.hub.test.tsx", status: pass}
      - {kind: test, ref: "frontend/e2e/web-share-scope.spec.ts (owner + viewer: no chevron, no file tab)", status: pass}
  - deliverable: "Share dialog, bindings, CSS, help and deps carry no file-browsing remnants"
    human_judgment: false
    verification:
      - {kind: command, ref: "pnpm -C frontend install --frozen-lockfile; head -1 pnpm-lock.yaml == lockfileVersion: '9.0'; grep gates on package.json/style.css/App.d.ts", status: pass}
  - deliverable: "Playwright chromium suite and TESTING.md match the browser surface without files"
    human_judgment: false
    verification:
      - {kind: command, ref: "pnpm -C frontend exec playwright test --project=chromium (14 passed, 9 skipped)", status: pass}
      - {kind: command, ref: "bash tests/check-traceability-paths.sh (OK: 114)", status: pass}
  - deliverable: "Native menus and live share flow show no file browsing"
    human_judgment: true
    verification:
      - {kind: manual, ref: "Task 3 human-check: wails build, open chevron + Hub card menus, Share dialog, web-share link in dev-browser (deferred to end-of-phase UAT)", status: pending}
---

# Phase 1 Plan 02: File Browser/Editor UI Removal Summary

Removed the file browser and editor from every UI surface. Gone: the `file-browser` tab type, both "Browse files" menu items, the remote-browse join flow, the DNS banner, the web-share file tab, the Share-dialog browse toggle and home-dir warning, 1,174 lines of CSS, and all 17 CodeMirror packages. The daemon files API is still served and is now unreachable from the UI. Plan 01-03 deletes it.

## Performance

- Start 2026-11-14T21:05:33Z, end 2026-11-14T21:31Z (about 26 min, of which about 8 min went on the Playwright browser download).
- Tasks: 3 of 3. 103 files changed (+114 / -13,277).

## Accomplishments

- **Task 1 (tracer):** deleted FileBrowserTab, `components/FileBrowser/` (including its 11 tests), Editor, RemoteBrowseDNSWarning, 6 file libs and 9 vitest files. App.tsx lost `handleOpenFileBrowser`, `handleBrowseFilesRemote`, the files branch of `handleModalExchange`, the file-tab render block, the DNS banner and its banner-stack trigger, and `webFilesEnabled`. The web bootstrap now only calls `openWebSessionTab`. `'open-session'` and `'hub-modal'` behave as before, held-cap reuse included. The `acceptDns` type and probe stay (F7). The tracer gate re-ran tsc, vitest and the import grep on the committed tree, and all passed.
- **Task 2:** Share dialog cleaned up. TailnetTab shows exactly "Watch the live session only — cannot send input." and "Full control of the live session (send input).". Bindings and both models.ts trees are in sync. `homeDir`/`browseEnabled` keys were removed from about 15 test mocks. CSS removed: file-browser, editor, home-write-warning and `hub-share-modal__toggle-wrap` (used only by the browse toggle). `.sr-only` stays because HubModal uses it. Help text edited.
- **Task 3:** e2e reflects the new surface. The fixture still prints `WRITE_CAP`/`SESSION_CWD`, which nothing parses now. TESTING.md updated.

## Recorded Outputs (per plan `<output>`)

- **pnpm lockfile check:** `head -1 frontend/pnpm-lock.yaml` still reads `lockfileVersion: '9.0'` after `pnpm remove` on local pnpm 12.6.0. The diff only removes lines (569 deletions, 0 additions), and `pnpm install --frozen-lockfile` passes. No pnpm 9 download was needed.
- **Suite Manifest counts (from `git ls-files`):** Go 132, vitest 127 (down 21), Playwright 7 (down 2), build-script 1. Total 267 (was 290).

## Task Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 | 465101b2 | feat(01-02): remove file browser tab, menu entries and web-share file panel |
| 2 | 04b651e2 | feat(01-02): drop file-browsing toggle, bindings, CSS, help and CodeMirror |
| 3 | d7646de0 | test(01-02): web-share e2e and TESTING.md without file browser |

All deletions were intentional: the plan's `files_deleted` list, plus `frontend/e2e/fixtures/remote-peer-setup.ts` (deviation 3).

## Plan-level Verification

- `pnpm install --frozen-lockfile && tsc --noEmit && pnpm test` passed (127 files, 2076 tests).
- `go build ./... && go vet ./... && go test -race -short ./...` passed.
- `playwright test --project=chromium`: 14 passed, 9 skipped. The same 9 tests were also skipped in the pre-install run, so the skips are environment-conditional and came before this plan.
- `check-traceability-paths.sh` printed OK: 114, and `build-script.test.sh` passed 37/37.
- `wails build -platform darwin/arm64 -tags wailsassets` succeeded. Afterwards `git checkout -- frontend/src/wailsjs/wailsjs/` was run, and the tree was clean.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Dead code] Removed the `'files'` intent from RemoteJoinCodeModal**
- Found during: Task 1. After the files branch was removed, nothing passed `'files'` any more, but the modal still had a "Join Remote Session — Files" title branch and a test for it.
- Fix: removed the union member, the title branch and that one test. The hub-modal and default title tests still pass. Commit 465101b2.

**2. [Rule 1] Removed `IssueCapabilitiesResponse.homeDir` from App.d.ts as well as the SessionInfo fields**
- Found during: Task 2. The acceptance grep (baseline 5) counts this field, and no frontend code reads it. The dead tree was mirrored. Commit 04b651e2.

**3. [Rule 1 - Dead code] Deleted the `filesApiURL` helper and `frontend/e2e/fixtures/remote-peer-setup.ts`**
- Found during: Task 3. Only the deleted files specs used them. Both build `/api/files/*` URLs, which 01-03 removes. Commit d7646de0.

**4. [Rule 3 - Blocking] Updated additional tests outside the task file lists**
- `StatusBar.shareSession.test.tsx` (tab-type gate list), `RemoteJoinCodeModal.test.tsx`, `remoteSession.test.ts`/`remoteSession.ts` (comments only), and the SM-01/N-A toggle-row tests in `SessionShareModal.test.tsx`. These asserted the removed `'file-browser'` type or the browse toggle row.

**5. [Rule 3 - Environment] Installed the Playwright headless shell by hand**
- `playwright install chromium` downloaded the pinned v1217 archive but hung during extraction twice: no CPU use, and the install stopped after writing ABOUT/LICENSE. I fetched the same archive from the same pinned CDN URL with curl (96,491,915 bytes, the same size Playwright reported), unzipped it into `~/Library/Caches/ms-playwright/chromium_headless_shell-1217`, and added the `INSTALLATION_COMPLETE` marker. No package was installed. `~/Library/Caches/ms-playwright/chromium-1217` holds only stale profile dirs from an earlier session. The headless project does not need the full chromium build.

**6. [Rule 2] Updated TESTING.md manual items M-01 and M-04**, which described the home-dir banner and the "Browse files" card item.

**Total deviations:** 6 (3 dead-code removals, 2 blocking, 1 doc). **Impact:** none on kept behavior.

## Issues Encountered

- During one full vitest run, `SessionShareModal.test.tsx > SM-02 tabpanel nested` failed once. It passed in isolation (48/48) and on every later full run, so it is load-sensitive and was not caused by this change. Noted, not fixed.
- The two Playwright installs I killed left `~/Library/Caches/ms-playwright/__dirlock`. Nothing seemed to depend on it; later runs worked.

## Known Stubs

None.

## Threat Flags

None. This plan only removes surface: the GUI no longer calls SetSessionBrowse (T-01-08). The Playwright assertions for T-01-07 cover both the owner and viewer caps. The lockfile change only removes entries (T-01-09).

## Next Phase Readiness

Ready for 01-03. Go still serves `/api/files/*`, `SetSessionBrowse`, `DownloadFile`, `homeDir`/`browseEnabled`, the `files.*` perms and the fixture's `WRITE_CAP`/`SESSION_CWD`/remote peer. None of these is reachable from the UI now. `FixtureEnv.remotePeerURL` and the `REMOTE_PEER_URL` parse in global-setup can go together with the fixture's remote peer. The TESTING.md SHARE-03 row (`internal/daemon/api_test.go` browse matrix) belongs to 01-03.

## Self-Check: PASSED

- Commits 465101b2, 04b651e2 and d7646de0 are ancestors of HEAD, and the rev-list count from plan_head_before is 3.
- Key modified files exist. The deleted paths are absent from `git ls-files`.
