---
phase: 02-tabs-sidebar-navigation
plan: 05
subsystem: frontend
status: complete
tags: [nav-03, hub-removal, share-dialog, testing]
requires: ["02-04"]
provides:
  - "frontend/src/components/Share/ (SessionShareModal, FunnelRiskPanel, ShareSegmentedControl)"
  - "frontend/src/components/InlineSessionName.tsx (for 02-08 sidebar/tab rename)"
  - "A frontend with no Hub grid, Welcome or in-app Help code"
affects: ["02-06", "02-07", "02-08", "02-12"]
tech-stack:
  added: []
  patterns:
    - "vi.waitFor around assertions that depend on an async mount-time seed (no single tick)"
key-files:
  created:
    - frontend/src/components/Share/SessionShareModal.tsx (moved)
    - frontend/src/components/Share/FunnelRiskPanel.tsx (moved)
    - frontend/src/components/Share/ShareSegmentedControl.tsx (moved)
    - frontend/src/components/InlineSessionName.tsx (moved)
    - frontend/src/components/__tests__/InlineSessionName.test.tsx (moved)
  modified:
    - frontend/src/App.tsx
    - frontend/src/style.css
    - frontend/src/test-setup.ts
    - frontend/src/components/__tests__/SessionShareModal.test.tsx
    - frontend/src/components/__tests__/App.test.tsx
    - frontend/src/components/__tests__/style.hub.test.ts
    - frontend/src/components/__tests__/style.hub.modal.test.ts
    - TESTING.md
decisions:
  - "Share dialog files moved byte-identical (git mv, 0-line rename diff in Task 1). The consent gate and FunnelRiskPanel logic are unchanged."
  - "InlineSessionName's class renamed .hub-card__name to .inline-session-name so no hub-card rule survives. The style is unchanged."
  - "style.css pruned by class ownership: a rule went only if every class in it was referenced by deleted sources and by no kept TSX/TS."
  - "M-21 retired rather than retargeted: M-30 step 3 already checks the terminal-tab overlay for no-resize."
metrics:
  duration: "12 min"
  completed: 2026-11-15
  tasks: 2
  files: 58
estimate:
  tokens: 70000
actuals:
  tokens: 140722
  tasks: 2
  commits: 2
plan_head_before: 364760741f81f3b9fae4eea9fa85104a0fdd5537
plan_head_after: 87d3e8a4bb790ff1ef6e18837889c8b0421ce73c
---

# Phase 2 Plan 05: Hub/Welcome/Help removal, Share dialog relocation Summary

The Share dialog, Funnel risk panel and segmented control moved unchanged to `components/Share/`, and inline rename moved to `components/InlineSessionName.tsx`. The orphaned Hub grid, Welcome tab and in-app Help were deleted with their 19 test files, 4 help docs, 3 helper libraries and about 1,500 lines of CSS. The SM-02 flake is fixed with `vi.waitFor`.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 4e587aae | Moved the Share dialog to components/Share and InlineSessionName out of Hub; fixed the SM-02 flake |
| 2 | 87d3e8a4 | Deleted the Hub grid, Welcome and Help code with its tests, styles and helpers; reconciled TESTING.md |

(`actuals.tokens` is chars/4 over the whole diff. The diff is mostly deleted lines, which is why it is twice the estimate.)

## Moved (git mv, history follows)

- `components/Hub/SessionShareModal.tsx` → `components/Share/SessionShareModal.tsx`
- `components/Hub/FunnelRiskPanel.tsx` → `components/Share/FunnelRiskPanel.tsx`
- `components/Hub/ShareSegmentedControl.tsx` → `components/Share/ShareSegmentedControl.tsx`
- `components/Hub/InlineSessionName.tsx` → `components/InlineSessionName.tsx`
- `components/Hub/InlineSessionName.test.tsx` → `components/__tests__/InlineSessionName.test.tsx`

Share is at the same depth as Hub, so no relative import changed. The three Share files are byte-identical at the Task 1 commit. Task 2 changed one comment in SessionShareModal (it named HubPanel's poll). Importers re-pointed: App.tsx, FunnelRiskPanel.test, SessionShareModal.test, SessionShareModal.disconnect.test and ShareSegmentedControl.test. App.test now checks the `./components/Share/SessionShareModal` import and that App renders exactly one `<SessionShareModal`.

## Deleted (39 files)

- Hub: HubPanel, HubModal, HubInteractiveModal, HubEmptyState, HubFilterBar, SessionCard, SessionCardGrid and MiniPreview, each `.tsx` plus its co-located `.test.tsx` (16 files)
- `components/__tests__/SessionCard.share.test.tsx`
- Welcome: `WelcomeTab.tsx`, `__tests__/WelcomeTab.test.tsx`, `__tests__/WelcomeTab.install.test.tsx`
- Help: `HelpTab.tsx`, `HelpSearch.tsx`, `HelpSectionNav.tsx` and `HelpContent.tsx`, plus five tests: `HelpTab`, `HelpTab.integration`, `HelpSearch`, `HelpSectionNav` and `HelpContent`
- `content/help/getting-started.md`, `sharing-guide.md`, `faq.md`, `chat.md`
- `lib/hubGroups.ts`, `lib/hubGroupCounts.ts` and `lib/vtColor.ts`, each with its test

`components/Hub/` now holds only the chat components and WebShareSessionView, for 02-07 to handle.

## Other edits

- **test-setup.ts:** removed the IntersectionObserver polyfill. Its only consumer was HelpSectionNav, and git grep finds no other user.
- **style.css:** 5,650 → 4,157 lines. The Help, Welcome, Hub grid/card/filter/empty/error, group sidebar, mini preview, card menu, `.hub-modal` panel and header, and `.hub-internet-badge`/`.hub-fullaccess-badge` rules were removed, along with the now-unused `hub-modal-grow`/`hub-modal-shrink` keyframes. Kept: `.hub-modal-overlay*`, `.hub-modal__close`, `.hub-modal__body*` (WebShareSessionView, App), `.hub-modal__chat-toggle*` (TerminalChatHost, WebShareSessionView), `.hub-share-modal*`, the Funnel/share rules and all `--hub-*` tokens. A patience diff shows only 4 added lines.
- **Tests that referenced removed code:** Sidebar.test lost its POL-03 HubFilterBar/HubEmptyState source gates. style.hub.test and style.hub.modal.test lost their pins on the deleted rules; the reduced-motion pin now targets `.hub-modal-overlay` and the focus-ring token pin targets `.hub-modal__close:focus-visible`. App.open-remote.test's header comment no longer cites SessionCard.share.test. TabBar.test's no-chevron fixture is a Settings tab instead of Welcome.
- **Comments:** App.tsx comments that named HubPanel or WelcomeTab were reworded. The single-SessionShareModal warning is kept.

## TESTING.md

- **Section 2:** vitest 118 → **99**, total 235 → **216**, with a 02-05 note listing what was deleted, moved and edited.
- **Section 4:** 25 rows removed. 23 pointed at deleted files: TERM-05 ×3, NET-01, SHARE-06, CARD-02, CARD-03, CARD-05, TEST-03, Hub group persistence, FIX-03, UX-02, HELP-01 ×5, NOTIF-01, INSTALL-02, FUI-03, HLP-01, HLP-02 and FNL-09. The other 2 were the POL-03 and CARRY-02 `style.hub.test.ts` rows, whose CSS pins went with the rules. The TEST-03 style.hub row was trimmed. Two NAV-03 rows were added, for SessionShareModal.test.tsx (Share is reachable without the Hub, and SM-02 is fixed) and InlineSessionName.test.tsx. `bash tests/check-traceability-paths.sh` reports 97 paths, all present.
- **Section 5 items removed:** **M-02** (Share button on a Hub remote peer card) and **M-21** (chat overlay no-resize in the Hub interactive modal; M-30 step 3 still covers the terminal tab).
- **Section 5 items reworded** to drop the Hub card or Hub modal entry points: M-13, M-16, M-28, M-29, M-30, M-31, M-32, M-39, M-40, M-42, M-44, M-47 and M-57. M-06 was already retired in 02-04.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] SessionCard import broke in Task 1**
- **Issue:** Moving InlineSessionName broke `Hub/SessionCard.tsx`'s `./InlineSessionName` import, which failed tsc. SessionCard is deleted in Task 2.
- **Fix:** Re-pointed the import to `../InlineSessionName` so Task 1 commits green. The file went in Task 2.
- **Commit:** 4e587aae

**2. [Rule 1 - Acceptance] `.hub-card__name` survived the CSS prune**
- **Issue:** InlineSessionName, which is kept, renders `className="hub-card__name"`. That kept the rule alive and broke the acceptance grep `^\.(help-tab|welcome|hub__grid|hub-card)` = 0.
- **Fix:** Renamed the class to `.inline-session-name` in the component, its test and the CSS rule. The style is unchanged.
- **Commit:** 87d3e8a4

**3. [Rule 3 - Blocking] Two CSS pin test files were not in the plan's file list**
- **Issue:** `style.hub.test.ts` and `style.hub.modal.test.ts` pinned 40 of the deleted rules.
- **Fix:** Dropped the pins on deleted rules and retargeted two pins (reduced-motion, focus-ring token) to rules that survive.
- **Commit:** 87d3e8a4

**4. [Rule 1 - Acceptance] Comments failed the no-reference grep**
- **Issue:** App.tsx comments named HubPanel and WelcomeTab, and a SessionShareModal comment named HubPanel's poll. That broke the `WelcomeTab|HelpTab|HubPanel|hubGroups` = no-match acceptance check.
- **Fix:** Reworded the comments only.
- **Commit:** 87d3e8a4

**Total deviations:** 4 auto-fixed. **Impact:** none on scope or behaviour.

## Deferred Issues

Logged in `deferred-items.md`:
- `remoteAdapter.adaptAllRemoteSessions` and `remoteSession.findRemoteSession` were orphaned by 02-04 but are not on this plan's delete list, so they are untouched.
- `frontend/src/assets/agenthub-title-logo.png` has no frontend importer but `scripts/verify-icons.sh` (SC5) checks that it exists, so it was kept per the plan's "only if no importer" rule.
- The Share dialog's classes still carry `hub-*` names.

## Verification

- `pnpm exec tsc --noEmit`: clean
- `pnpm exec vitest run`: 99 files, 1408 tests passed, twice in a row after the final commit (plus one run before it)
- SM-02 flake: `SessionShareModal.test.tsx` passed 5/5 in a loop after Task 1, 10/10 in a loop after Task 2, and 3/3 inside the plan's own verify loop
- `go build ./... && go vet ./...`: clean. `go test -race -short ./...`: exit 0 (no Go change)
- `pnpm exec playwright test --project=chromium`: 7 passed, 9 skipped (the skips are the existing documented `test.skip`s, e.g. progress.spec.ts)
- `bash tests/check-traceability-paths.sh`: OK, 97 paths
- All acceptance checks for both tasks pass. Share/Inline files exist. The old-path grep finds nothing. SM-02 contains `vi.waitFor`. `git ls-files` finds no Hub grid, card, preview, help, Welcome, hubGroups or vtColor files. test-setup has 0 IntersectionObserver lines. style.css has 0 lines matching `^\.(help-tab|welcome|hub__grid|hub-card)`. Non-test frontend source has no WelcomeTab/HelpTab/HubPanel/hubGroups references.
- `./build.sh --platform macos` built `build/bin/agenthub.app`. The generated wailsjs files (`models.ts`, `runtime.d.ts`, `runtime.js`) were restored with `git checkout`.

## Needs VM check

Run against `build/bin/agenthub.app`, with any installed AgentHub quit and its daemon stopped first:
1. Open a session tab and click the footer "Share Session" button. The Share dialog opens centred with its scrim, at about 520px wide, with the header "Share — <name>" and a working close (×) button. Escape closes it.
2. Toggle "Share the session" ON. The Tailnet / Internet·Read-only / Internet·Full-access segmented control shows and the link panel scrolls on its own (SM-02).
3. In tailscale mode, toggle "Enable internet sharing". The Funnel risk panel replaces the segmented control with the risk text, the expiry select, "Enable internet share" and "Keep local only". "Keep local only" collapses it with no change. Nothing goes public without the CTA.
4. No unstyled leftovers: no Hub, Welcome or Help surface appears anywhere, and the sidebar and tabs look as they did after 02-04.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: frontend/src/components/Share/SessionShareModal.tsx, FunnelRiskPanel.tsx, ShareSegmentedControl.tsx
- FOUND: frontend/src/components/InlineSessionName.tsx, frontend/src/components/__tests__/InlineSessionName.test.tsx
- FOUND: commits 4e587aae and 87d3e8a4 are ancestors of HEAD
