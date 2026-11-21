---
phase: 11-file-preview
plan: 06
subsystem: frontend, webserver
status: complete
tags: [frontend, preview, csp, security-review, pdf, svg]
requires: [11-01]
provides:
  - "ImagePreview({url, name, kind: 'image' | 'svg', onFail, autoFocus?})"
  - "PdfPreview({url, name, webMode, onFail, autoFocus?}) per 11-SPIKE.md (iframe; open-pdf in browsers)"
  - "Base preview CSS and the --color-checker-a/-b tokens"
  - "Share CSP Amendment 3: img-src 'self' data: blob: only"
  - "11-SECURITY-REVIEW.md (CSP part; Dependencies and VM headings for 11-07 and 11-10)"
  - "Hostile SVG fixtures with BEACON placeholders for 11-10"
affects: [11-07, 11-09, 11-10]
tech-stack:
  added: []
  patterns:
    - "Renderers render only their pane content; PreviewPanel (11-09) owns receiving/error panes and the ErrorBoundary"
    - "Open PDF pane reuses the .empty-state classes"
key-files:
  created:
    - frontend/src/components/ImagePreview.tsx
    - frontend/src/components/PdfPreview.tsx
    - frontend/src/components/__tests__/PreviewRenderers.test.tsx
    - frontend/src/lib/__tests__/fixtures/preview/hostile-script.svg
    - frontend/src/lib/__tests__/fixtures/preview/hostile-external.svg
    - .planning/phases/11-file-preview/11-SECURITY-REVIEW.md
  modified:
    - frontend/src/style.css
    - internal/webserver/csp_mw.go
    - internal/webserver/csp_mw_test.go
    - internal/webserver/csp_integration_test.go
decisions:
  - "Share CSP delta is blob: in img-src only, no frame-src or object-src (11-SPIKE.md verdict b)"
  - "PDF failure signal is the iframe load: a readable non-application/pdf document calls onFail (React attaches no error listener to iframes, and iframes report none)"
  - "The Actual Size toggle is a native <button type=button>; Enter and Space come from the browser, no keydown handler"
  - "At Fit the toggle button is absolutely inset 16px in the pane so the img's max-width/max-height 100% resolve; the whole padded pane is the click target while the toggle is offered"
metrics:
  duration: "~45 min"
  completed: 2026-11-21
actuals:
  tokens: 10000    # chars/4 over the realized diff (~40,000 chars incl. the review)
  tasks: 3
  commits: 3
plan_head_before: a6274e3d2fcb1da20b8a08297e28d20ffa92c36a
plan_head_after: adde0898
---

# Phase 11 Plan 06: Image and PDF renderers, share CSP delta Summary

Images and SVG preview only as `<img src=blob:>` on a light checkerboard, with fit and an Actual Size toggle. PDFs use the spike's iframe on the desktop and an Open PDF button in browsers. The share page CSP gains only `blob:` in img-src, which the tests pin and the security review records.

## Component contracts (for 11-09)

### `ImagePreview` (`frontend/src/components/ImagePreview.tsx`)

`ImagePreview({ url, name, kind: 'image' | 'svg', onFail: () => void, autoFocus?: boolean })`

- Root `div.preview-image`, plus `.preview-image--actual` at Actual Size. It scrolls (`overflow: auto`) and needs a definite height from its parent (`height: 100%`).
- `img.preview-image__img` (`.preview-image__img--svg` for SVG), `src={url}`, `alt={name}`. `onError` calls `onFail`.
- For raster images larger than the pane minus 32px (measured on load and through a ResizeObserver on the pane), the img sits inside `button.preview-image__toggle[type=button][aria-label="Actual Size"][aria-pressed]`. A click toggles it and scrolls the pane to the top-left. If a resize makes the image fit, the toggle goes away and Fit comes back. SVG never gets the toggle.
- `autoFocus` focuses the toggle once it is offered. With no toggle there is nothing to focus, so 11-09 focuses the pane if needed.
- On a refresh (new `url`) the size from the new load replaces the old one. Keying the component by url is optional.

### `PdfPreview` (`frontend/src/components/PdfPreview.tsx`)

`PdfPreview({ url, name, webMode: boolean, onFail: () => void, autoFocus?: boolean })`

- Desktop (`webMode` false): `iframe.preview-pdf`, `src={url}`, `title={name}`, full-bleed. `onFail` fires when the frame loads a readable document whose `contentType` is not `application/pdf`. An unreadable (null) document is not treated as a failure. `autoFocus` focuses the iframe.
- Browser (`webMode` true, 11-SPIKE.md Browser PDF mode `open-pdf`): `div.empty-state.preview-pdf-open` with `DocumentIcon`, `h2` = name, body **This browser can't show the PDF inside the page.** and a button **Open PDF** (`.empty-state__button`). The button calls `window.open(url, '_blank', 'noopener,noreferrer')` on click only, and its `null` return is not a failure. `autoFocus` focuses the button.

### CSS (`frontend/src/style.css`)

- Tokens `--color-checker-a: #FFFFFF` and `--color-checker-b: #E0E0E4`, declared in both appearance blocks.
- `.preview-image`, `.preview-image__img` (conic-gradient checkerboard, 16px tile), `.preview-image__img--svg` (100%/100%, `object-fit: contain`), `.preview-image__toggle` (zoom-in), `.preview-image--actual …` (zoom-out, no max size), `.preview-pdf`, `.preview-pdf-open .empty-state__heading` (`overflow-wrap: anywhere`).
- 11-07 adds the Markdown styles and 11-09 adds the message panes.

## Final share CSP

```
default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>; img-src 'self' data: blob:; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
```

## Copy used beyond UI-SPEC

The Open PDF pane copy is the wording accepted in CONTEXT "Decided at planning": heading = file name, body **This browser can't show the PDF inside the page.**, button **Open PDF**. It uses a straight apostrophe, as the rest of the GUI copy does.

## Tests (for 11-10's TESTING.md rows)

- `frontend/src/components/__tests__/PreviewRenderers.test.tsx`
  - "ImagePreview (PREV-02)": one img with blob src and alt. Toggle offered when larger, and click flips aria-pressed. Toggle is a native button. Resize re-measures and resets Actual Size. A small image has no toggle. SVG has no toggle. autoFocus focuses the toggle. img error calls onFail. Each hostile SVG fixture shows only as one img (no svg/script/iframe/foreignObject/object/embed, no BEACON in the DOM, title unchanged).
  - "PdfPreview (PREV-02, PREV-04)": desktop iframe with src, title and class, and no object/embed/button. Load with an application/pdf, text/html or null document calls onFail 0, 1 or 0 times. autoFocus focuses the frame. Web mode shows the copy and Open PDF, which calls window.open with (url, `_blank`, `noopener,noreferrer`) only on click.
- `internal/webserver/csp_mw_test.go`: `TestCSPHeaders_PreviewDelta` (new: blob: only in img-src, the whole policy equals Amendment 2 plus blob:, no frame-src/object-src/child-src/worker-src, frame-ancestors last). `TestCSPHeaders_RequiredTokens` now expects `img-src 'self' data: blob:`.
- `internal/webserver/csp_integration_test.go`: `TestCSPHeaderStrict_App` now also checks the served /app/ page: img-src blob:, blob: exactly once, no frame-src/object-src, frame-ancestors last.
- Fixtures: `frontend/src/lib/__tests__/fixtures/preview/hostile-script.svg` (shapes, `<script>` and `onload` that set the title and fetch `http://BEACON/svg-script`, `/svg-script-img` and `/svg-onload`) and `hostile-external.svg` (`@import` `/svg-style-import.css`, `<image href>` `/svg-image`, `xlink:href` `/svg-image-xlink`, `<use>` `/svg-use.svg`, a `<foreignObject>` iframe `/svg-foreignobject-iframe`, and a billion-laughs entity block). Before its VM beacon rows, 11-10 replaces `BEACON` with the VM's loopback beacon address.

## Gate results

- Task 1 verify: PASS. Renderer and token tests passed (26), and the CSP tests passed.
- Task 2 verify: PASS. Element `iframe` from 11-SPIKE.md is in PdfPreview. Renderer and token tests passed (32), and the CSP tests passed.
- Task 3 gate (each run once):
  - Full vitest: 99 files, 1410 tests passed (`/tmp/p11-06-gate-vitest.log`).
  - `tsc --noEmit`: clean (`/tmp/p11-06-gate-tsc.log`).
  - `go build ./...` and `go vet ./...`: clean.
  - `go test -race -short -count=1 ./...`: exit 0, 0 FAIL lines, 13 `ok` packages including `internal/webserver` (`/tmp/p11-06-gate-go.log`). No flake reruns were needed.
  - check-brand: clean. check-traceability-paths: 174 paths exist.
  - App.tsx, TabBar.tsx, TerminalPanel.tsx, package.json, the lockfile, capability_mw.go and internal/tailnet are unchanged since the base. wailsjs is untouched.

## Deviations from Plan

**1. [Rule 1 - Bug] PDF onFail comes from the iframe's load, not its error event**
- **Found during:** Task 2.
- **Issue:** React attaches no `error` listener to `<iframe>` (only `load`), and iframes report no error event anyway, so `onError={onFail}` could never fire.
- **Fix:** The iframe `onLoad` calls `onFail` when the frame's document is readable and not `application/pdf`. This is the success and failure signal 11-SPIKE.md S-01 names: the iframe is the only variant that fires `load`, with a readable `application/pdf` plugin document.
- **Files modified:** frontend/src/components/PdfPreview.tsx and the test.
- **Commit:** 04b9f62a.

**2. Enter and Space on the toggle are covered by the native button, not by a jsdom keypress**
- jsdom does not synthesise keyboard activation, so the test asserts a native `<button type="button">` instead of dispatching Enter. A keydown handler would toggle twice in real browsers. 11-10's VM check covers real keyboard activation.

## Known limits (not stubs)

- A damaged PDF in WKWebView most likely still loads an `application/pdf` plugin document, so it shows the native viewer's blank view rather than **Couldn't Show File**. This is unverified. The spike didn't test a damaged PDF, so 11-10 could add one VM row.
- The SVG img fills the pane (`object-fit: contain`, as the plan says), so the checkerboard covers the whole padded pane, not just the drawing's box.
- While the toggle is offered at Fit, the whole padded pane is the click target and the focus ring surrounds the padded pane.

## Known Stubs

None.

## Self-Check: PASSED

- Files: ImagePreview.tsx, PdfPreview.tsx, PreviewRenderers.test.tsx, both SVG fixtures and 11-SECURITY-REVIEW.md exist.
- Commits: 82d5b07b, 04b9f62a and adde0898 are ancestors of HEAD.
