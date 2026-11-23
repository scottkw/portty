---
phase: 11-file-preview
plan: 01
subsystem: preview-spike
status: complete
tags: [spike, pdf, svg, wkwebview, wails, safari, csp, vm-check]
requires: []
provides:
  - "11-SPIKE.md decision block (Verdict, PDF element, Share CSP delta, Browser PDF mode, Versions) for 11-06"
affects:
  - 11-06 (PdfPreview element and share CSP delta)
  - 11-10 (security review records the CSP delta)
tech-stack:
  added: []
  patterns:
    - "Throwaway spike patch in a detached worktree outside the checkout; only the evidence file is committed"
    - "Safari web-content AX in the VM needs AXEnhancedUserInterface=true; WebDriver screenshots are blank in these VMs"
key-files:
  created:
    - .planning/phases/11-file-preview/11-SPIKE.md
  modified: []
decisions:
  - "Spike verdict (b): desktop PDF via <iframe src=blob:>, browsers get Open PDF (top-level blob); share CSP delta is blob: in img-src only"
  - "In WKWebView only the iframe fires load (with a readable application/pdf plugin document); object and embed fire neither load nor error"
  - "Safari's PDF plugin loads <object>/<embed> blob data under connect-src; object-src blob: does not help; blob: in connect-src untested (outside the decision tree)"
metrics:
  duration: 35m
  completed: 2026-11-21
  tasks: 2
  files: 1
actuals:
  tokens: 5500
  tasks: 2
  commits: 2
plan_head_before: a4a81f1e
plan_head_after: 70a7e1fe743d8bae8f0da72618deeecfb0765ee2
---

# Phase 11 Plan 01: PDF and SVG spike (Wails and Safari) Summary

A `blob:` PDF renders in the real Wails v2 web view through iframe, object and embed (scroll and text selection work), while Safari on the share page renders none of them under any candidate CSP that keeps `frame-ancestors 'none'`. The verdict is (b): the desktop uses an iframe, browser viewers get an Open PDF button (proven to work top-level), and the share CSP gains only `blob:` in img-src.

## Decision (from 11-SPIKE.md)

- **Verdict:** (b)
- **PDF element:** iframe
- **Share CSP delta:** img-src 'self' data: blob: (no frame-src and no object-src)
- **Browser PDF mode:** open-pdf
- **Versions:** macOS 27.2 (26B123) arm64; Safari 27.2; Wails library v2.10.2 (CLI v2.16.0)

## Results per surface

| Surface / candidate | iframe | object | embed | Violations |
|---|---|---|---|---|
| Wails desktop (no CSP), light and dark | renders 3 pages, `load` fires, plugin doc `application/pdf` readable | renders 3 pages, no `load`/`error` | renders 3 pages, no `load`/`error` | 0 |
| Safari, none | blank (blocked) | plugin, 0 pages | plugin, 0 pages | `frame-src` x1, `connect-src` (object x3, embed x5) |
| Safari, `frame-src blob:` | `load`, `contentDocument` null, blank (matches inherited `frame-ancestors 'none'`) | 0 pages | 0 pages | `connect-src` only |
| Safari, `object-src blob:` | blank (blocked) | 0 pages | 0 pages | `frame-src` x1, `connect-src` |
| Safari, both | as `frame-src blob:` | 0 pages | 0 pages | `connect-src` only |
| Safari, Open PDF (top-level, delta only) | new tab: PDF plugin, 3 pages, `application/pdf` | | | none |

- Desktop scrolling: wheel over each PDF scrolled only that PDF. Selection: drag + Cmd+C copied PDF text in each variant. Dark mode: no reload; the native PDF view keeps white pages (UI-SPEC accepts this).
- SVG in `<img>` (both surfaces): `naturalWidth` 240, no script or `onload` effect, no `img-src` violation; both loopback beacons logged 0 requests beyond their self-tests.
- Violations seen in Safari overall: only `connect-src blocked=blob` and `frame-src blocked=blob`.

## Builds

All `v1.2.0-spike`, ad-hoc signed (`Signature=adhoc`, `TeamIdentifier=not set`), built in the throwaway worktree `/tmp/p11-spike`:

- spike-1 `6dd2518b2bc08d9fd030c630e9c127bada9e89146c96ce1d8bc8d163ff43be98`
- spike-2 `ba2bd12120e3fb36e7ad554201b97700e175c525d160981cbd66337b5f2dc9b2`
- spike-3 (final evidence) `e1bec105352f6c4f98a85b0f2919387c589a8cc8ee8e2078681df54583e67028`
- Restored on both VMs: `dev` `90541495583f9261635c5d84c0af6797f27e555e18c1eecb2ba7a08e01990244` (the same build as before)

## Commits

- `f546cb97` docs(11-01): spike desktop rows
- `70a7e1fe` docs(11-01): PDF and SVG spike (Wails and Safari) with verdict

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Spike page did not hide index.html's static splash**
- **Found during:** Task 2 (WebDriver click reported "click intercepted"; `elementFromPoint` hit `DIV#splash-static`)
- **Issue:** App hides `#splash-static` on mount; the spike page replaced App and did not, so a full-page splash covered spike-1 and spike-2. AX still read the PDFs beneath it, but it swallowed every mouse event and covered screenshots. The first commit (`f546cb97`) therefore wrote scrolling and selection as "not driven" and screenshots as a rig limit.
- **Fix:** spike-3 hides the splash; S-01 and S-02 were re-run on spike-3 (scrolling, selection, clicks and partial screenshots all work), and the Builds table explains the correction. S-03 to S-05 ran on spike-3 too.
- **Files modified:** only the throwaway spike worktree; 11-SPIKE.md rewritten in `70a7e1fe`.

**2. [Rule 3 - Blocking] Rendering evidence by AX tree instead of screenshots**
- **Issue:** WebDriver screenshots are blank in these VMs, and `screencapture` over SSH draws native PDF page frames but not glyphs or HTML.
- **Fix:** rendering is judged from the AX tree (`AXPDFPluginSubrole`, `AXPage` count, page text) plus DOM reads; Safari's web-content AX needed `AXEnhancedUserInterface=true`. Safari screenshots were dropped because the window chrome shows the share host.

**3. Minor:** screenshots were kept in `/tmp/p11-spike-shots` (not inside the worktree as `/tmp/p11-spike/shots`) and deleted at the end; a small CGEvent helper (`~/p11/m`: click with clickState, drag, wheel) was compiled on the host with `swiftc` and removed with `~/p11`.

## Known Stubs

None (no product code changed).

## Threat Flags

None. No repo code changed; the configurable-CSP patch never left the throwaway worktree (T-11-01), and no link, token or capability appears in 11-SPIKE.md (T-11-02, grep-checked).

## Notes for 11-06

- Use `<iframe src=blob:>` on the desktop; treat its `load` plus a `contentDocument.contentType` of `application/pdf` as success. Object and embed give no signal in WKWebView.
- `window.open(url, '_blank', 'noopener')` returns `null` even when it works; Open PDF was tested from a user gesture only.
- If inline PDF in browsers is wanted later, the untested direction is `blob:` in `connect-src` with `<object>`/`<embed>` (needs a security review; outside this plan's decision tree).

## Self-Check: PASSED

- FOUND: .planning/phases/11-file-preview/11-SPIKE.md
- FOUND: f546cb97, 70a7e1fe (ancestors of HEAD)
- Only 11-SPIKE.md changed between `a4a81f1e` and `70a7e1fe`; no `p11-spike` worktree; `frontend/src/wailsjs`, `internal` and `frontend/src` clean.
