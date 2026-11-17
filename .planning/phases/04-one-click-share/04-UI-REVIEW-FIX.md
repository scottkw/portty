---
phase: 04
fixed_at: 2026-11-17T06:56:16Z
review_path: .planning/phases/04-one-click-share/04-UI-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 04: UI Review Fix Report

**Fixed at:** 2026-11-17T06:56:16Z
**Source review:** .planning/phases/04-one-click-share/04-UI-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (the 3 Top Fixes plus 3 minor items: arrow size, stale error, link-row gap)
- Fixed: 6
- Skipped: 0

Worked in the main checkout on `gsd/phase-04-one-click-share` with no worktree, as instructed. The root pin script ran before the first edit and before every commit. Verification ran in the main checkout, so you can reproduce it from this tree.

## Fixed Issues

### Fix 1: An already-shared session shows an empty popover, and a GetSessionShare failure is swallowed

**Files modified:** `frontend/src/components/SharePopover.tsx`, `frontend/src/components/__tests__/SharePopover.test.tsx`
**Commit:** 15eaa3dd
**Status:** fixed: requires human verification (copy choice, see below)
**Applied fix:** While the session is shared and the link hasn't loaded yet, the body shows the UI-SPEC's starting copy, "Getting a link…". The UI-SPEC has no separate loading line, so this reuses that string. `GetSessionShare` failures are now kept in a `loadError` state. This covers both the usual path (the Go binding returns `errorCode: "unavailable"`; it rarely rejects) and a rejected promise. In the shared branch the failure shows as an error-styled line. The off state still ignores a failed lookup, because it doesn't need the link.
**Copy note:** The UI-SPEC has no sentence for "shared, but the link couldn't be read". To keep every string word-for-word from the contract, the line reuses the general start error: "Couldn't start sharing: <detail>. Check that Tailscale is connected, then try again." The recovery advice fits, but "start" is not exactly what happened. If you want something more precise, add a dedicated sentence to the UI-SPEC Copywriting Contract.
**Tests:** a loading case (body reads "Getting a link…" and has no link field), a failure through `errorCode`, and a failure through a rejected promise.

### Fix 2: Raw "Error: …" text and a doubled period in the error detail

**Files modified:** `frontend/src/lib/shareCopy.ts`, `frontend/src/components/__tests__/SharePopover.test.tsx`
**Commit:** d3e9f77b
**Applied fix:** A private `clean()` helper trims the detail, strips a leading `Error: ` and removes any trailing periods. All four detail sentences use it: `shareStartError`, `shareStopError`, `menuShareError` and `menuStopError`. It also fixes the `App.tsx` menu path, which builds its detail with `String(err)` too. The fixed copy is unchanged.
**Tests:** a rejected `new Error('…connection refused.')` renders "…connection refused. Check that…", and direct assertions check the stop error and both menu banners.

### Fix 3: The link field hides the part that differs between links

**Files modified:** `frontend/src/components/SharePopover.tsx`, `frontend/src/components/__tests__/SharePopover.test.tsx`
**Commit:** e62f0c11
**Applied fix:** A `useLayoutEffect` sets `scrollLeft = scrollWidth` on the link input whenever the link appears or changes, so the session path and token are visible. A new `showLink` flag (not busy, Tailscale on, shared, link present) is now the single condition for rendering the link row.
**Tests:** with `scrollWidth`/`scrollLeft` stubbed, the field opens at `scrollLeft === 500`. I confirmed this test fails without the effect.

### Minor: The popover arrow is 16×9 instead of 16×8

**Files modified:** `frontend/src/components/SharePopover.tsx`, `frontend/src/style.css`
**Commit:** 18a63e8d
**Applied fix:** The SVG is now 16×8 (`viewBox 0 0 16 8`, polygon `0,8 8,0 16,8`, stroke `M0.5 8 L8 0.5 L15.5 8`) with `top: -8px`. `top` is measured from the padding edge, so the arrow's bottom row still covers the popover's 1px top border and there is no seam. The CSS comment now explains this.

### Minor: A stale start error stays on screen next to the Tailscale-off reason

**Files modified:** `frontend/src/components/SharePopover.tsx`, `frontend/src/components/__tests__/SharePopover.test.tsx`
**Commit:** acb5a628
**Applied fix:** `useEffect(() => { setError(null) }, [tailscaleOff])`.
**Tests:** a start failure, then a re-render with `tailscaleOff: true`, leaves no error line, and the body reads exactly `TAILSCALE_OFF_REASON`.

### Minor: The 12px link gap is built from 8px plus a 4px margin patch

**Files modified:** `frontend/src/components/SharePopover.tsx`, `frontend/src/style.css`, `frontend/src/components/__tests__/SharePopover.test.tsx`
**Commit:** b936508c
**Applied fix:** The state container gets a `share-popover__state--link` class (`margin-top: 12px`) when `showLink` is true. The `margin-top: 4px` patch on `.share-popover__link-row` is gone. The text states keep the 8px gap from the checker notes.
**Tests:** the shared-open test asserts the class.

### Docs

**Commit:** 6abd905a. The `TESTING.md` Section 4 SHAR-01 row now describes the new cases. No test files were added or renamed; every new case is in `SharePopover.test.tsx`.

## Verification (main checkout)

- `pnpm -C frontend exec tsc --noEmit`: clean
- `pnpm -C frontend exec vitest run`: 82 files, 1045 tests passed (`SharePopover.test.tsx`: 29, up from 21)
- `go build ./... && go vet ./...`: clean
- `bash tests/check-traceability-paths.sh`, `check-brand.sh`, `check-share-removal.sh`: all OK
- No GUI check (per instructions). These still need eyes in the Tart VM: the 16×8 arrow shows no seam in Light and Dark, the field opens scrolled to the token, and the 12px gap.

Untouched and unstaged: `.gitignore` (your edit), `.ignore`, `.planning/state.json`, the root `agenthub` binary.

---

_Fixed: 2026-11-17T06:56:16Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
