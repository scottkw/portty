---
phase: 12-file-path-click
plan: 06
subsystem: frontend
tags: [typescript, react, xterm, path-click, relay-client, link-provider]

requires:
  - phase: 12-file-path-click
    provides: "12-04 MsgPathClick 0x13, pathClick capability bit, notice metas and the shared fixture frontend/src/lib/__tests__/fixtures/path-click.json; 12-05 share-server arm so peer and browser viewers reach the same host path"
provides:
  - "frontend/src/lib/pathLinks.ts: findPathLinks and stripLineCol (pure), registerPathLinks (xterm link provider, logical-line builder, modifier tracker)"
  - "RelayClient: MSG_PATH_CLICK, sendPathClick (dropped unless open), onPathClickCapability and onPathNotice from MsgMeta"
  - "TerminalPanel: path links in the web-links arm, capability ref, pathNotice state and 5000 ms timer"
  - "SessionEndedBanner kinds path-unsupported and path-too-large"
affects: [12-07 TESTING.md consolidation and VM check of the underline mechanism]

actuals:
  tokens: 10700
  tasks: 3
  commits: 3
plan_head_before: 21837682a7569cbde09e810b3ca12facd9fcae33
plan_head_after: 79cab1e34a0fa5ab88769683e5ceef5f2e6a2aad

tech-stack:
  added: []
  patterns:
    - "One xterm link provider, in lib/pathLinks.ts only; TerminalPanel calls registerPathLinks and holds no provider code"
    - "Capability gated per socket by a ref, not by re-registering the provider on every reconnect"
    - "Decorations always written through link.decorations (xterm swaps in a tracked object after hover)"

key-files:
  created:
    - frontend/src/lib/pathLinks.ts
    - frontend/src/lib/__tests__/pathLinks.test.ts
    - frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx
  modified:
    - frontend/src/lib/relayClient.ts
    - frontend/src/lib/relayClient.test.ts
    - frontend/src/lib/openLink.ts
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/SessionEndedBanner.tsx
    - frontend/src/components/__tests__/SessionEndedBanner.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.web-links.test.tsx

key-decisions:
  - "Bare-form paths may not start with `~` (the plan's rule alone would link `~user/x.png`, which its own extra case requires to be none)"
  - "Anchored paths need a non-slash character right after the prefix, so `//` and `./` alone are not links"
  - "A visible notice is also cleared when another banner takes the slot, and hidden while the pane is inactive, not only dropped on arrival"

requirements-completed: []

coverage:
  - id: D1
    description: "A modifier-click on a printed path sends one 0x13 frame on the session's socket, only after the host's pathClick meta; plain click sends nothing; nothing after close or before open"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "frontend/src/lib/relayClient.test.ts#MSG_PATH_CLICK (PATH-02)"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx#path link gating (PATH-02)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Detection matches the shared fixture exactly; wrapped and wide rows; URLs never become path links"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/pathLinks.test.ts#findPathLinks against the shared fixture (PATH-02)"
        status: pass
      - kind: unit
        ref: "frontend/src/lib/__tests__/pathLinks.test.ts#rows to links (PATH-02)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Underline and pointer only while the modifier is held; activate re-checks the modifier"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/pathLinks.test.ts#underline follows the modifier (PATH-02)"
        status: pass
      - kind: unit
        ref: "frontend/src/lib/__tests__/pathLinks.test.ts#activate is modifier-gated (PATH-02)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Notice: exact copy, 5000 ms, replace not stack, yields to other banners, no extra announcement, never takes focus"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx#path notice (PATH-02)"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx#path click adds no announcement (PATH-02)"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/SessionEndedBanner.test.tsx#kind %s (PATH-02)"
        status: pass
    human_judgment: false
  - id: D5
    description: "WKWebView delivers window key events so the underline appears on Cmd-hover (otherwise the accepted fallback applies)"
    requirement: PATH-02
    verification: []
    human_judgment: true
    rationale: "jsdom dispatches the key events by hand; only the real desktop WebView in 12-07's VM check shows whether the underline follows Cmd"

duration: 35min
completed: 2026-11-21
status: complete
---

# Phase 12 Plan 06: Viewer half of a path click Summary

**Every TerminalPanel (local tab, peer tab, browser viewer) now underlines path-like text under a held Cmd/Ctrl and sends a modifier-click to the host as one 0x13 frame on the session's own socket, but only after the host's replay meta said path clicks are available; a refused type or size shows a fixed 5-second notice in the clicking pane.**

## Accomplishments

- `relayClient.ts`: `MSG_PATH_CLICK = 0x13`, `PATH_CLICK_MAX_BYTES = 1024`. `sendPathClick(text)` uses the same two guards as `sendClaim` (reconnect state open, socket open), drops empty text and over 1024 bytes, never queues. `handleMeta` calls `onPathClickCapability(pathClick === true)` on every replay meta and `onPathClickCapability(false)` on close and on abandon (beside both `onPreviewReset` calls); a meta whose `notice` is exactly `path-unsupported` or `path-too-large` calls `onPathNotice(kind)` (the wire string is never forwarded), and carries no replay or offset so resume is untouched.
- `pathLinks.ts`: pure detection plus `registerPathLinks(term, {enabled, modifier, onClick})`. Both `enabled` and `modifier` are functions read at event time.
- `TerminalPanel.tsx`: registers once per terminal in the existing `webLinks` arm (disposed in the else branch and on unmount), `pathClickEnabledRef` fed by the RelayClient callback, `onClick` is `clientRef.current?.sendPathClick`. The comment above the arm now says the OSC 8 provider is still absent and path links come from lib/pathLinks.ts. No `registerLinkProvider` and no hyperlink-id walking in the file.

## Detection rules (as built)

- Alphabet: Unicode letters and digits, `_ . ~ @ % + # -` and `/`; a candidate is a maximal run of it; trailing `.` trimmed.
- Link when anchored (`/`, `~/`, `./`, `../` followed by a non-slash character) or bare (contains `/`, does not start with `/` or `~`, last segment ends in `.` plus letters or digits).
- A `:digits(:digits)?` directly after the token (not followed by a letter or digit) is underlined but is not in the request (`request` is the token alone; `stripLineCol` exported for the same rule).
- Anything overlapping `scheme://\S+` or `mailto:\S+` is dropped, so URLs keep the web-links confirm popover.
- Paths with spaces are detected up to the space. No quote parsing.
- Every `detect` case of path-click.json passes as written (fixture not edited).
- Rows: wrapped rows are joined (walk up while `isWrapped`, down while the next row `isWrapped`, capped at 2048 characters), a string-offset to cell map skips width-0 spacers, a link's end x is the last cell of its last character, and only matches touching row y are returned.

## Gating

Path links exist only while `pathClickEnabledRef.current` is true: set by a replay meta with `pathClick:true`, cleared by any replay meta without it, by close and by abandon. A read-only viewer or an older host therefore sees no underline and no clickable text (the host refuses regardless: 12-04, 12-05). Follows the web-links toggle and `webLinksConfigRef.current?.modifier || 'platform'`; no new setting.

## Underline mechanism (12-07's VM check decides the fallback)

Links are provided with `decorations {underline, pointerCursor}` equal to "modifier held". `registerPathLinks` keeps `modDown`, updated from window `keydown` and `keyup` (capture phase) and `mousemove` on `term.element` via `isModifierPressed`, and reset by window `blur` and `visibilitychange`. On a change it writes both decorations through `hovered.decorations` (the hovered link tracked from `hover` and `leave`). `activate` calls `onClick` only if `isModifierPressed(event, mode)`. With modifier `none` links start underlined. If WKWebView does not deliver the key events, the accepted fallback (underline on plain hover, click still modifier-gated, today's URL behaviour) is a change for 12-07's decision; it is not made here. `isModifierPressed` now takes `Pick<MouseEvent, 'metaKey' | 'ctrlKey'>` so key events use the same rule.

## Notice rules

Two `SessionEndedBanner` kinds with the exact copy (straight apostrophe), the existing info banner, icon and Dismiss; no CSS, no new component. `TerminalPanel` keeps `pathNotice`, `PATH_NOTICE_MS = 5000`, a timer restarted on each notice (replace, never stack), cleared on Dismiss and unmount. A notice arriving while the paste-interrupted, reconnecting, unreachable, ended or share-stopped banner is showing is dropped (not queued); one already showing is removed when another banner takes the slot; it is cleared and hidden while `isActive` is false; focus never moves; sending a click adds no aria-live element. `style.css` is unchanged.

## Test names (for 12-07's TESTING.md rows)

- `frontend/src/lib/__tests__/pathLinks.test.ts` (new): "findPathLinks against the shared fixture (PATH-02)", "findPathLinks extra cases", "rows to links (PATH-02)", "underline follows the modifier (PATH-02)", "activate is modifier-gated (PATH-02)".
- `frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx` (new): "path link gating (PATH-02)", "path click adds no announcement (PATH-02)", "path notice (PATH-02)".
- `frontend/src/lib/relayClient.test.ts`: "MSG_PATH_CLICK (PATH-02)" (frame, guards, capability, close and abandon, notices).
- `frontend/src/components/__tests__/SessionEndedBanner.test.tsx`: "kind %s (PATH-02)" for both kinds.
- `frontend/src/components/__tests__/TerminalPanel.web-links.test.tsx`: the Plan B test, renamed "Plan B (Wave 0 spike outcome), updated for Phase 12: no OSC 8 provider in TerminalPanel; path links live in lib/pathLinks.ts". It still asserts no `registerLinkProvider` and no `getHyperlinkId` in TerminalPanel.tsx, and now also that it imports `registerPathLinks` and that `lib/pathLinks.ts` is the only non-test file under src that registers a link provider.
- New files needing a Section 4 traceability row (PATH-02): `frontend/src/lib/__tests__/pathLinks.test.ts`, `frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx`. TESTING.md was not edited, per the plan (12-07 consolidates).

## Gate results

- `pnpm install --frozen-lockfile`: clean (lockfile, package.json and wailsjs unchanged).
- Full `pnpm vitest run`: 106 test files, 1576 tests, all passed (log `/tmp/p12-06-gate-vitest.log`).
- `pnpm exec tsc --noEmit`: exit 0. `bash tests/check-brand.sh`: exit 0.
- Every `<verify>` grep for the three tasks passed. No Go file touched, no Go test run.

## Task Commits

1. **Task 1 (tracer): MSG_PATH_CLICK, capability, registerPathLinks, TerminalPanel wiring** - `c853d26c` (feat)
2. **Task 2: detection tests, wrapped and wide rows, underline tracker, web-links guard edit** - `d2fb2f2d` (feat)
3. **Task 3: notice banner kinds and TerminalPanel notice** - `79cab1e3` (feat)

Tracer gate: Task 1's `<verify>` re-ran end to end (relayClient and pathClick suites) before Task 2; passing, expanded.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking, instruction conflict] Commit attribution trailer**
- **Issue:** The plan says commits end with `Co-Authored-By: Claude Opus 5.5`; the session's attribution instruction specifies `Claude Sonnet 5.5`, and says it takes precedence.
- **Fix:** Used `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`, as 12-04 did.

**2. [Rule 1 - Bug in the plan's rule] `~user/x.png` would have been a link**
- **Found during:** Task 1, writing findPathLinks
- **Issue:** The plan's bare-form rule (contains `/`, does not start with `/`, extension) matches `~user/x.png`, which the plan's own extra cases require to be none (the host refuses `~user`).
- **Fix:** Bare form also may not start with `~`. Also an anchored form needs a non-slash character after its prefix so `//` is not underlined. Every fixture case still passes unchanged.
- **Files modified:** frontend/src/lib/pathLinks.ts
- **Commit:** c853d26c

**3. [Rule 3 - Blocking] `isModifierPressed` widening moved to Task 1**
- **Issue:** `registerPathLinks` passes KeyboardEvents to `isModifierPressed`, so tsc failed in Task 1 until the parameter type was widened (the plan lists it under Task 2).
- **Fix:** Widened in Task 1's commit; Task 2's openLink.test.ts run still passes.
- **Commit:** c853d26c

**4. [Rule 1 - Bug avoided] Notice kept visible when another banner arrives later**
- **Issue:** The plan says an arriving notice is dropped if another banner holds the slot; it did not cover another banner arriving while the notice shows (both would render).
- **Fix:** The notice is cleared when another banner takes the slot, and hidden while the pane is inactive (a passive effect alone left one render with the notice after `isActive` turned false).
- **Commit:** 79cab1e3

## Known Stubs

None.

## Threat Flags

None. The new surface (a client frame, two meta fields) is the one in the plan's threat model: T-12-29 (exact-code comparison, fixed copy, `relayClient` and banner tests), T-12-30 (activate re-checks the modifier), T-12-31 (URL-shaped text excluded, fixture cases), T-12-32 (never queued; capability cleared on close and abandon), T-12-33 (no links without the host's bit).

## Limits and risks

- The underline depends on WKWebView delivering window key events; only 12-07's VM check can confirm it.
- Paths with spaces are linked only up to the space; hard-wrapped TUI output is not joined (host has the same limit).
- Peer tabs and browser viewers get path links only if the host's web-links toggle is on (they use the host's plugin config, as URLs do).

## Self-Check: PASSED

Created files exist (pathLinks.ts, pathLinks.test.ts, TerminalPanel.pathClick.test.tsx); commits `c853d26c`, `d2fb2f2d`, `79cab1e3` are ancestors of HEAD.
