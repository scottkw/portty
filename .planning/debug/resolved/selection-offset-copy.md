---
status: resolved
trigger: "Highlighting text (either in the app or in the web app) does not highlight where the mouse pointer actually is. It highlights several rows above it. Additionally once something is highlighted, copying doesn't seem to copy that text to the clipboard."
created: 2026-11-21
updated: 2026-11-21
---

## Symptoms

- expected: dragging in the terminal highlights the cells under the pointer; Cmd-C puts that text on the clipboard.
- actual: the highlight lands several rows above the pointer; after highlighting, Cmd-C does not put the text on the clipboard.
- errors: none reported.
- timeline: seen on v1.1.0 (the user's installed build); unknown whether it ever worked or when it started.
- repro: drag-select in a terminal tab, in the desktop app (Wails/WKWebView) or the web share view (browser). Happens with only ONE viewer attached. Option-drag + Cmd-C also fails.
- environment: macOS Apple Silicon; xterm.js 6 frontend (frontend/src/components/TerminalPanel.tsx).

## Eliminated

- hypothesis: CSS `scale()` on term.element (recomputeScale, TerminalPanel.tsx:272) skews the mouse-to-row mapping on guest or grid-following panels.
  evidence: the user reproduced with a single viewer attached, where no scale is applied.
- hypothesis: a TUI's mouse mode (Claude Code) took the drag, so xterm made no selection.
  evidence: Option-drag (forces an xterm selection on macOS) + Cmd-C also fails.
- OVERTURNED 2026-11-21: both eliminations above rested on untested premises. "Single viewer => no scale" is false (a lone viewer follows the last claimed grid until it types or resizes; reproduced at s=0.81 and 0.83 with one viewer). "Option-drag forces a selection" is false (only with macOptionClickForcesSelection, default false, unset here; reproduced). Both are confirmed contributing causes; see Evidence.
- hypothesis: viewport scroll position (ydisp) out of sync with the render, or a CSS rule / font metric offsetting .xterm-screen from its rows.
  evidence: with grid == fit (scale 1) a drag on line-59 selects line-59 exactly, and with the unscale patch drags on rows 3, 25, 48 under scale 0.833 are exact. No residual offset exists once the scale is compensated.
  timestamp: 2026-11-21
- hypothesis: Cmd-C is not routed to xterm (Edit menu, custom key handler, paste guard, clipboard addon intercepting copy).
  evidence: main.go appends menu.EditMenu(); with an xterm selection ControlOrMeta+C puts exactly the selected text on the clipboard (activeElement = xterm-helper-textarea), scaled or not.
  timestamp: 2026-11-21

## Current Focus

- bug_class: Bohrbug (deterministic geometry and configuration).
- status: RESOLVED. Fix committed (7a37bfb4); chromium spec 5/5 with mutation checks; VM WKWebView pre/post check passed.
- open decision (not approved, not applied): altClickMovesCursor: false. With forced selection on, a quick Option-click (no drag) in a mouse-mode TUI can send Up/Down arrows from xterm's move-to-cell, which Claude Code reads as prompt history; turning it off loses Option-click-to-move-cursor at a plain shell prompt. Not exercised in the VM (only Option-drags, which sent zero bytes).
- next_action: none (archived).

reasoning_checkpoint:
  hypothesis: "Two causes, AND-gated per symptom. (1) Offset: recomputeScale puts transform scale(s<1) on term.element whenever the session grid (0x02) is larger than this pane's fit, and xterm 6 maps the pointer to a cell as (clientY - rect.top)/cssCellHeight with a post-transform rect and an unscaled cell, so the selection AND every mouse report sent to a TUI land at row*s, above the pointer. A lone viewer hits this: reports never move a known grid, only a claim or a keystroke does. (2) Copy: the user runs Claude Code with tui=fullscreen, which captures the mouse; the highlight is Claude Code's own, xterm has no selection, so Cmd-C copies nothing, and Option-drag cannot force an xterm selection because macOptionClickForcesSelection is false (unset). Claude Code's own auto-copy uses pbcopy on the Mac that runs it, so a remote viewer never gets it."
  confirming_evidence:
    - "Chromium, one viewer: scale 0.81 -> drag on line-59 selects line-50; scale 0.833 -> line-51; scale 1 -> line-59. Offset = (1-s)*row."
    - "Runtime unscale patch on _core._mouseService: rows 3/25/48 exact under scale 0.833; mouse report row 48 (patched) vs 40 (unpatched)."
    - "macOS platform, mouse tracking on: plain and Option drag both go to the app, no selection; with macOptionClickForcesSelection=true Option-drag selects line-59 and ControlOrMeta+C copies it."
    - "~/.claude/settings.json has \"tui\": \"fullscreen\"; Claude Code docs: fullscreen captures mouse, copies via pbcopy locally, xterm.js terminals need Option + macOptionClickForcesSelection for native selection."
  falsification_test: "If the user's offset persists at scale(1) (inspect term.element.style.transform in the app/web view while it happens), cause (1) is not the user's offset. If Option-drag + Cmd-C still fails with the option on in Claude Code fullscreen, cause (2) is incomplete."
  fix_rationale: "Fix-1 corrects the one place xterm turns a pointer into a cell (its single MouseService instance, shared by SelectionService, Linkifier and the mouse-report path), so selection, TUI mouse rows and link hits all follow the pointer at any scale, without changing INPT-06 grid ownership. Fix-2 restores the macOS escape hatch (Option-drag = native selection) that every other mac terminal and Claude Code's docs assume, so Cmd-C has an xterm selection to copy."
  blind_spots: "Not observed on the user's own machines: whether their offset happened at s<1 (likely after switching Macs, before typing; also after a sidebar toggle). Neither WebKit nor the desktop WKWebView was driven: the Playwright webkit-2272 install hung extracting (partial files removed); the VM desktop check is planned after the fix. SelectionService drag-autoscroll and the right-click textarea placement still use untransformed rects (minor). Legacy web/assets/terminal.js has the same transform but /sessions/{id} redirects to /app/."
  candidate_causes:
    - "code: xterm 6 Mouse.ts ignores CSS transforms; TerminalPanel.recomputeScale applies one (CONFIRMED)"
    - "config: Terminal option macOptionClickForcesSelection unset/false (CONFIRMED); user's Claude Code tui=fullscreen captures the mouse (CONFIRMED present)"
    - "data/state: hub grid left at another viewer's size (D-01) so a lone viewer scales (CONFIRMED mechanism); web join sends the 220x50 fallback for an unsized grid (side finding)"
    - "environment: WKWebView vs browsers (ELIMINATED as a distinct cause: same failure in Chromium; same code path)"
  and_gate: "yes. Offset needs s<1 (grid > fit) AND xterm's transform-blind mapping. Copy failure in Claude Code needs mouse capture (tui=fullscreen) AND no forced-selection modifier (macOptionClickForcesSelection false); with a plain shell, copy of an xterm selection already works (it copies the highlighted, offset text)."

## Evidence

- timestamp: 2026-11-21
  checked: node_modules/@xterm/xterm/src/browser/input/Mouse.ts (xterm 6.0.0) getCoordsRelativeToElement / getCoords
  found: row = ceil((clientY - rect.top - paddingTop) / cssCellHeight), rect from getBoundingClientRect (post-transform), cssCellHeight unscaled. No transform compensation.
  implication: any CSS scale(s) on term.element maps the pointer to row*s, i.e. above the pointer, the error growing with distance from the top.
- timestamp: 2026-11-21
  checked: internal/relay/hub.go sizeClient / NoteTyping / ResizeClient doc
  found: a plain report never moves a known grid (except the local bootstrap on a never-sized hub); only a claim (window resize, font zoom) or a typed keystroke (NoteTyping) moves the grid to the viewer's fit. Joins and reconnects never claim.
  implication: a lone viewer that joins a session whose grid was set by another (now absent) viewer, by the spawn grid, or before a sidebar toggle shows that grid scaled (s<1 when larger) until it types or resizes. "Single viewer" does not imply s=1.
- timestamp: 2026-11-21
  checked: main.go:190 menu setup
  found: menu.EditMenu() is appended, so Cmd-C reaches WKWebView as a copy event; xterm 6 handles 'copy' on term.element (CoreBrowserTerminal.ts:334) and only acts when hasSelection().
  implication: a missing Edit menu is not the copy cause in the desktop app.
- timestamp: 2026-11-21
  checked: Playwright chromium, frontend/e2e/selection-offset.spec.ts (new, untracked), /app/ web view (the share link /sessions/{id} redirects there, server.go:787), ONE viewer, 60 numbered lines injected via page.routeWebSocket, real page.mouse drag across visual row rows-2, Terminal instance hooked via an Object.prototype._core setter.
  found: |
    - fixture join grid 220x50 (hub fallback, see below) on a 178x42 pane: transform scale(0.81039); drag on visual row 48 (line-59) selected line-50 (9 rows above).
    - grid 1.2x the fit (214x50): scale(0.833111); drag on line-59 selected line-51 (8 rows above).
    - grid == fit (178x42): scale(1); drag on line-59 selected line-59; ControlOrMeta+C put "line-59 abcdefg" on the clipboard (activeElement xterm-helper-textarea).
    - scaled case + ControlOrMeta+C: clipboard == the highlighted (wrong) text "line-51 abcd". Copy itself works for an xterm selection.
  implication: H-scale CONFIRMED for the offset. The offset appears exactly when s<1 and is (1-s)*row. A lone viewer reproduces it. Copy of an xterm selection is not broken.
- timestamp: 2026-11-21
  checked: same harness, mouse tracking on (ESC[?1002h ESC[?1006h, as a TUI sets), grid == fit; plain drag, then Option(Alt)-drag.
  found: both drags made NO xterm selection (getSelection() ""); both went to the app as SGR mouse reports (e.g. ESC[<0;1;40M, 24 input frames for the two drags).
  implication: under a mouse-capturing TUI there is no way to make an xterm selection on macOS: xterm only forces one for Option when macOptionClickForcesSelection is true (SelectionService.ts:437-443), and its default is false (OptionsService.ts:41); TerminalPanel never sets it. The earlier elimination ("Option-drag + Cmd-C fails, so it is not mouse mode") rested on the wrong premise that Option-drag forces a selection. RE-OPENED.
- timestamp: 2026-11-21
  checked: ~/.claude/settings.json on this Mac; Claude Code fullscreen docs (code.claude.com/docs/en/fullscreen)
  found: "tui": "fullscreen" (Claude Code 2.1.330). Fullscreen captures the mouse; drag selects INSIDE Claude Code (its own highlight, at the row xterm reports); it copies on mouse release with pbcopy on the machine running Claude Code (OSC 52 only over SSH); Cmd+C works only on kitty-keyboard terminals; for a native selection xterm.js terminals need Option with macOptionClickForcesSelection enabled.
  implication: in Claude Code the highlight is Claude Code's, drawn at xterm's mouse-report row, which the same scale bug shifts (getMouseReportCoords uses the same untransformed math). Cmd-C finds no xterm selection, so nothing is copied; Option-drag cannot help; the auto-copy lands on the host Mac's clipboard (pbcopy), not a remote viewer's.
- timestamp: 2026-11-21
  checked: internal/webserver/server.go:901-908 vs internal/relay/server.go:123-129
  found: the web relay sends hub.Cols()/Rows() at join even when the grid was never set (the 220x50 fallback); the local relay checks GridKnown() first.
  implication: side finding. A web viewer of a never-sized session follows 220x50 (scaled). Real sessions are bootstrapped by the local tab, so this is not the user's main path; noted, not part of the fix.
- timestamp: 2026-11-21
  checked: correction to the mouse-mode run above. Playwright's 'Desktop Chrome' device reports navigator.platform "Win32", so xterm's isMac was false there. Re-ran with navigator.platform forced to MacIntel before xterm loads.
  found: macOS behaviour, default options: plain drag and Option-drag both reach the app (Option-drag as alt-modified SGR reports ESC[<8;1;40M), no xterm selection. Conclusion unchanged.
- timestamp: 2026-11-21
  checked: EXPERIMENT fix-1 (runtime only): wrap term._core._mouseService.getCoords and getMouseReportCoords so the pointer is mapped into the element's unscaled box first (s = rect.width / el.offsetWidth; x' = rect.left + (x - rect.left)/s, same for y). Scale 0.833 (grid 1.2x fit).
  found: drags on visual rows 3, 25, 48 selected line-14, line-36, line-59 (all correct). A click on visual row 48 reported ESC[<0;1;48M patched vs ESC[<0;1;40M unpatched.
  implication: compensating the scale inside xterm's single MouseService instance (shared by SelectionService, Linkifier and the mouse-report path) fixes both the xterm selection and the row a TUI like Claude Code receives.
- timestamp: 2026-11-21
  checked: EXPERIMENT fix-2 (runtime only): term.options.macOptionClickForcesSelection = true, mouse tracking on, macOS platform.
  found: plain drag still went to the app (12 frames, no selection); Option-drag made an xterm selection "line-59 abcdefg", sent no frames, and ControlOrMeta+C put "line-59 abcdefg" on the clipboard.
  implication: with the option on, Option-drag + Cmd-C works inside Claude Code fullscreen (what Claude Code's docs tell xterm.js terminals to use).
- timestamp: 2026-11-21
  checked: renderer in the chromium runs (count of canvas vs .xterm-rows divs in .xterm-screen)
  found: canvases 2, DOM rows 0: the WebGL addon was active in every chromium result above.
  implication: the offset and both fixes hold on the WebGL renderer the desktop app uses, not only the DOM renderer.
- timestamp: 2026-11-21
  checked: Playwright webkit project
  found: webkit-2272 (the build Playwright 1.59.1 wants) was not installed; `playwright install webkit` downloaded 75 MiB but hung twice extracting (stuck writing libwebrtc.dylib at 15 MB). Processes stopped, partial ~/Library/Caches/ms-playwright/webkit-2272 removed; webkit-2336 untouched.
  implication: no WebKit result. The mapping math is engine-independent (getBoundingClientRect includes transforms in every engine), but the WKWebView path stays unobserved until the post-fix VM check.

- timestamp: 2026-11-21
  checked: FIX APPLIED. frontend/src/lib/terminalScale.ts (unscalePointer, compensatePointerScale), TerminalPanel.tsx (compensatePointerScale(term) after the WR-01 paste guard; macOptionClickForcesSelection: true). Regression spec frontend/e2e/selection-offset.spec.ts rewritten (exp: tests gone; 5 hard tests), vitest terminalScale.test.ts +8.
  found: |
    - pnpm test: 106 files, 1584 tests passed; tsc --noEmit clean.
    - two source-text gates moved the edits: TerminalPanel.test.tsx reads 300 chars after `new Terminal(` for `theme` (option placed after theme), pasteGuard.test.ts requires guardPasteInsert right after term.open (wrap placed after it).
    - chromium spec: 5/5 passed, 3 repeat runs 5/5.
    - mutation, fix-1 reverted: 4 failed (1.2x drag got line-32 for line-36; mouse report ESC[<0;1;40M for row 48; Option-drag line-51 for line-59 twice); the scale-1 test still passed.
    - mutation, fix-2 reverted: 2 failed (Option-drag selection "" in both mouse-mode tests); the 3 scale tests passed.
  implication: each fix is independently load-bearing and the spec catches the loss of either.

- timestamp: 2026-11-21
  checked: Playwright webkit (one bounded attempt, `timeout 300 pnpm exec playwright install webkit`) and firefox
  found: webkit-2272 downloaded 75.4 MiB to 100% and hung extracting again (15 MB on disk for 4+ min); timeout fired (exit 124), no leftover processes, partial ~/Library/Caches/ms-playwright/webkit-2272 removed, webkit-2336 untouched. firefox-1511 (the build Playwright 1.59.1 wants) is not installed either (firefox-1538 is); not attempted. CI (e2e.yml, ubuntu-24.04) installs all three with --with-deps.
  implication: no local WebKit or Firefox result. The spec avoids host-specific behaviour on CI: the copy check dispatches a copy event (all engines) and the real Cmd-C test is skipped unless the host is macOS and the browser chromium. WKWebView coverage comes from the VM check.

- timestamp: 2026-11-21
  checked: VM check, Tart VM portty-test (macOS 27.2, desktop app in WKWebView, WebGL setting on, Courier 18). Pane fit 87x29 (grid bootstrap); a `portty attach` in a 109x36 expect pty typed once (grid 87x29 -> 109x36 why=input), so the lone desktop tab followed the larger grid at scale 0.803 (visual cell 16.86 px from the AX frames of .xterm-viewport and the cursor's helper textarea). `clear; seq -f row-%g-abcdefghij 1 35` put row-N on visual row N. Drags by CGEvents (helper seldrag: plain, or Option flags on every event plus the Option key), Cmd-C by a real Cmd+C key event, result by pbpaste. Mouse TUI stand-in: ~/psel/tui.zsh enables ESC[?1002h ESC[?1006h (the modes Claude Code fullscreen uses) and logs every input byte. Claude Code itself is not installed in the VM (no node, no login).
  found: |
    - PRE-FIX build (installed HEAD dev, binary bfddb9cf, no fix marker): drags on visual rows 3/18/34 copied row-2 / row-15 / row-27. Under the mouse TUI a plain drag on row 34 reported ESC[<0;1;27M (row 27) and Cmd-C left the pasteboard unchanged; an Option-drag reported ESC[<8;1;27M (alt-modified, row 27), no selection, pasteboard unchanged.
    - FIXED build (v1.2.0-selfix from 7a37bfb4, binary 4500411b, fix marker present): drags on rows 3/18/34 copied row-3 / row-18 / row-34. Under the mouse TUI a plain drag on row 34 reported ESC[<0;1;34M ... (row 34; Cmd-C copies nothing, as it should while the app owns the drag); Option-drags on rows 34, 3 and 18 copied row-34 / row-3 / row-18 and the TUI received zero bytes.
    - FIXED build at scale 1 (the desktop typed: grid 109x36 -> 87x29 why=input): drags on rows 3/15/27 copied row-3 / row-15 / row-27.
  implication: the user's two symptoms reproduce in the real desktop app before the fix (highlight several rows above the pointer; Cmd-C copies nothing under a mouse-capturing TUI) and are gone after it. The WKWebView blind spot is closed. Copy through the Edit menu's Cmd-C works whenever xterm has a selection.

## Specialist Review

- reviewer: typescript (typescript-expert skill not installed; reviewed directly against xterm 6.0.0 source)
- verdict: SUGGEST_CHANGE (direction correct)
- suggestions:
  - altClickMovesCursor: false alongside fix-2. With forced selection on, a quick Option-click in a mouse-mode TUI reaches SelectionService._handleMouseUp (SelectionService.ts:708) and, in the alt buffer, moveToCellSequence sends Up/Down arrows (MoveToCell.ts:24-28), which Claude Code reads as prompt-history navigation. Cost: Option-click-to-move-cursor at a plain shell prompt is lost. NOT part of the user-approved fix; left as an open decision for the user.
  - Do not skip silently: if term.element exists but _core?._mouseService is missing, console.warn once (vitest mocks have no real element; production does). Adopted as an implementation detail of fix-1.
- pitfalls (implementation guidance for fix-1):
  - Transform the event, not the rect or cell size: getCoordsRelativeToElement (Mouse.ts:6-14) subtracts unscaled padding after the rect delta, so mapping clientX/Y into the unscaled box keeps padding right; scaling cell dims breaks padding and the canvas clamp (MouseService.ts:37-38).
  - Derive s and origin from the `element` argument on every call (callers pass screenElement at SelectionService.ts:398, CoreBrowserTerminal.ts:609, Linkifier.ts:382, or term.element at SelectionService.ts:710). s = rect.width / offsetWidth; guard offsetWidth > 0 && rect.width > 0, else pass through.
  - Pass { clientX, clientY } explicitly ({...ev} is {} for MouseEvent getters); call the original with orig.call(ms, ...) since it reads this._renderService.
  - Re-open is not a concern (open() returns early, CoreBrowserTerminal.ts:403-408); TerminalPanel makes a new Terminal per effect run, so wrap once after term.open (~TerminalPanel.tsx:476).
  - Typing: narrow `as unknown as { _core?: { _mouseService?: ... } }` or the repo's existing `(term as any)._core` style.
  - Residual (minor, out of scope): drag-past-bottom autoscroll (SelectionService.ts:418) and right-click textarea placement (Clipboard.ts:66) still use transformed rects.
  - Side benefit: link hover/click goes through getCoords (Linkifier.ts:382), so path links under scale should also be fixed; worth re-checking Phase 12 U-01.
- timestamp: 2026-11-21

## Resolution

root_cause: "(1) Offset: TerminalPanel.recomputeScale sets transform scale(s<1) on term.element whenever the session grid is larger than the pane's fit (a lone viewer that has not typed or resized since joining, or after a sidebar toggle), and xterm 6's MouseService maps pointers with a post-transform rect over an unscaled cell, so selection and TUI mouse reports land at row*s, above the pointer; (2) Copy: Claude Code fullscreen (user's tui=fullscreen) captures the mouse, so the highlight is Claude Code's and xterm has no selection for Cmd-C, and Option-drag cannot force one because macOptionClickForcesSelection is unset (false); Claude Code's own auto-copy goes through pbcopy on the Mac running it, never to a remote viewer."
fix: "fix-1: frontend/src/lib/terminalScale.ts adds unscalePointer (maps a pointer into an element's untransformed box: s = rect.width / offsetWidth, x' = left + (x - left)/s, same for y; unmeasurable box passes through) and compensatePointerScale (wraps the terminal's one xterm MouseService, getCoords and getMouseReportCoords, via .call(ms, ...); console.warn once if _core._mouseService is missing). TerminalPanel.tsx calls compensatePointerScale(term) right after the WR-01 paste guard that follows term.open. fix-2: macOptionClickForcesSelection: true in the Terminal options. altClickMovesCursor left at its default (open decision)."
verification: |
  guardrail_verdict: accepted
  signals:
    - original_repro: PASS. Chromium spec (scaled 1.2x grid, one viewer) selects the pointer row on rows 3/mid/rows-2; VM desktop app (WKWebView, scale 0.803) copies row-3/row-18/row-34 for drags on rows 3/18/34 (pre-fix: row-2/row-15/row-27).
    - revert_returns_bug: PASS. Chromium with fix-1 reverted: 4 of 5 tests fail (offset rows and mouse report row 40 for 48); with fix-2 reverted: the 2 Option-drag tests fail (selection ""). VM pre-fix build shows the offset and the empty copy.
    - regression_suite: PASS. pnpm test 106 files / 1584 tests; tsc --noEmit clean; check-traceability-paths 197 paths OK; chromium spec 3 repeat runs 5/5; VM scale-1 drags exact.
    - mutation_at_fix_site: PASS. vitest row-boundary neighbours (rows 47.9/48.1 drawn at 0.8) fail without the 1/s mapping; the e2e spec fails with either fix removed.
    - diff_shape: PASS. Additive (54 lines helper, 5 lines TerminalPanel), no deletions of behaviour.
  oracle_type: specified (the row under the pointer is the row the spec's own geometry says; the copy must equal the selection)
  not_run: Playwright webkit (webkit-2272 install hangs extracting; one bounded attempt timed out) and firefox (firefox-1511 not installed); CI e2e.yml runs all three. Real Claude Code fullscreen not run in the VM (not installed there); a mouse-tracking TUI stand-in with the same modes was used.
files_changed:
  - frontend/src/lib/terminalScale.ts
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/lib/terminalScale.test.ts
  - frontend/e2e/selection-offset.spec.ts
  - TESTING.md
commit: 7a37bfb4

## Constraints

- Phase 12 gap planning runs in parallel: do not touch .planning/phases/12-file-path-click/.
- Never stage .gitignore, .planning/state.json, .ignore or agenthub/.
- Commit trailer: Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
- Update TESTING.md (Sections 2 and 4) for any new test file; run `bash tests/check-traceability-paths.sh`.
