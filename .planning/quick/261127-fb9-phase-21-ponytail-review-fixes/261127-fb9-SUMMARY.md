---
phase: quick-261127-fb9
plan: 01
subsystem: desktop-viewer
tags: [desktop, phone, swap, ponytail-review, tidy]
status: complete
requires: []
provides:
  - iPhone surface picker reports a refused switch through DesktopView's pickSurface
  - phone Send Keys gated by sendable()
  - no Swap in the app's Desktop tab (embedded)
key-files:
  modified:
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/DesktopPhoneLayer.tsx
    - frontend/src/components/DesktopPhoneBars.tsx
    - frontend/src/components/DesktopControlBar.tsx
    - frontend/src/components/DesktopTab.tsx
    - frontend/src/lib/peerDesktop.ts
    - frontend/src/App.tsx
    - frontend/src/components/Sidebar.tsx
    - frontend/src/style.css
    - internal/daemon/remote_desktop_ws_proxy_test.go
    - frontend/src/components/__tests__/DesktopPhoneBars.test.tsx
    - frontend/src/components/__tests__/DesktopPhoneLayer.test.tsx
    - frontend/src/components/__tests__/DesktopControlBar.test.tsx
    - frontend/src/components/__tests__/DesktopTab.test.tsx
decisions:
  - "Embedded viewer gets null storage for createDesktopInput so a remembered Swap is read as off; the bar hides the Swap button"
actuals:
  tokens: 20000
  tasks: 3
  commits: 3
plan_head_before: 12cd903a29cf394ec04a57554df507d41d4f9986
plan_head_after: 7b60fe5f
completed: 2026-11-27
---

# Quick 261127-fb9: Phase 21 ponytail-review fixes Summary

Nine Phase 21 review items applied in three atomic commits: the phone picker now reports a refused switch, the app's Desktop tab no longer turns Cmd+V into Ctrl+V, and leftovers are trimmed.

## Commits

| Task | Hash | Message |
| ---- | ---- | ------- |
| 1 | 98c47df9 | fix: phone surface picker reports a refused switch; phone Send Keys respects sendable (items 1, 3) |
| 2 | f82c4c87 | fix: no Swap in the app's Desktop tab so Cmd+V stays Cmd+V; drop unused DesktopTab controlRef (items 2, 4) |
| 3 | 7b60fe5f | chore: tidy Phase 21 leftovers; cover token-refresh failure close codes (items 5-9) |

`commits: 3` counts only this plan's commits. The raw `rev-list` range from `plan_head_before` also contains six Phase 22 commits that landed concurrently on the same branch.

## What changed

1. DesktopView's bar handler became a named `pickSurface`, passed to the control bar and to DesktopPhoneLayer, which threads it to DesktopPhoneBars. The phone picker now has the in-flight guard, the SWITCH_TIMEOUT_MS timeout and the "Couldn't switch to X. Still showing Y." notice.
2. `BarState.embedded` hides Swap in `barAvailability`, and the Swap button renders only when not hidden. In DesktopView, `storage: embedded ? null : safeStorage()` makes an embedded viewer read Swap as off, so a remembered Swap cannot turn Cmd+V into Ctrl+V.
3. Phone `sendChord` checks `sendable()`.
4. DesktopTab's unused `controlRef` prop is removed. The `controlRef={setHandle}` attribute on DesktopView stays.
5. `forceRebuild` is no longer public on `createPeerDesktopOptions`. The Local Network watcher still uses it internally.
6. App's peer-poll prune uses `desktopConnectHost(p)`. `dropPeerSession` is untouched.
7. The Sidebar comment now says a click focuses the open tab.
8. Two duplicate CSS declarations are removed (`.toolbar-chip--desktop` padding and `.desktop-view--embedded` background). The base rules still carry them.
9. New Go test `TestRemoteDesktopWS_RefreshFailureMapsToFixedCloses`: after a token 401, a cap 404 closes 4410 off and a cap 403 closes 4403 forbidden, with exactly one refresh POST.

## Tests

- New tests that failed before the change and pass after: the DesktopPhoneLayer phone `selectFailed` notice, the DesktopTab Cmd+V mask 8 with remembered Swap, and the two DesktopControlBar embedded tests.
- DesktopPhoneBars picker test now asserts `onSelectSurface` and no direct client call.
- Item 9 covers existing behavior and passed without production changes.
- Items 3-8 have no dedicated behavior test: they are guard, dead-code and cosmetic changes covered by tsc, the grep gates and the existing suites.
- `pnpm exec tsc --noEmit` is clean.
- The 12 frontend suites in the plan's verification list pass (518 tests).
- `GOTOOLCHAIN=go1.26.11 go test ./internal/daemon -run Desktop` passes.
- `bash tests/check-brand.sh` was clean before each commit.
- No TESTING.md change was needed: no test file was added.

## Deviations from Plan

None. The plan was executed as written.

## Known Stubs

None.

## Self-Check: PASSED

The three commits (98c47df9, f82c4c87, 7b60fe5f) are ancestors of HEAD, and the modified files exist.
