# Phase 11 Security Review: File Preview

Scope: what Phase 11 (PREV-01..05) changes at the security boundary. This file is written in three parts:

- **11-06 (this part):** the share page CSP before and after, the spike evidence it rests on, what stays unchanged, the desktop page's lack of a CSP, and PREV-03's route absence.
- **11-07:** the "Dependencies" section (the Markdown renderer and sanitizer).
- **11-10:** the "VM evidence and verdict" section.

Host names in policies are written as `<host>`. No share link, token or tailnet host appears in this file.

## Share page CSP (PREV-04)

Set by `cspHeaders` in `internal/webserver/csp_mw.go` on every HTML route of the share server, including `/app/`, which serves the same React app the browser viewer uses. The change is "Amendment 3 (Phase 11, PREV-04)" in that file's package comment.

### Before (Amendment 2, Phase 96)

```
default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>; img-src 'self' data:; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
```

### After (Amendment 3, Phase 11)

```
default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>; img-src 'self' data: blob:; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
```

### Directive by directive

| Directive | Before | After | Change and reason |
|-----------|--------|-------|-------------------|
| `default-src` | `'none'` | `'none'` | Unchanged. No `blob:` |
| `script-src` | `'self' 'wasm-unsafe-eval'` | same | Unchanged. No `blob:`, so a blob can never run as script |
| `style-src` | `'self' 'unsafe-inline'` | same | Unchanged |
| `connect-src` | `'self' wss://<host>` | same | Unchanged. No `blob:`. This is also why Safari can't show a framed or embedded blob PDF (S-03); loosening it was not tried and is not proposed |
| `img-src` | `'self' data:` | `'self' data: blob:` | **Changed.** Image and SVG previews are `<img src=blob:…>` built from bytes that arrived over the session's own authenticated socket. `blob:` URLs are same-origin objects the page itself created; no network origin is added, so an image can still fetch nothing from elsewhere. An SVG inside an `<img>` runs no script and loads no sub-resource (S-05) |
| `font-src` | `'self'` | same | Unchanged |
| `base-uri` | `'none'` | same | Unchanged |
| `form-action` | `'self'` | same | Unchanged |
| `frame-src` / `object-src` | absent (falls back to `default-src 'none'`) | absent | **Not added.** 11-SPIKE.md's "Share CSP delta" line: `img-src 'self' data: blob:` only. Neither directive makes Safari show a PDF on this page (S-03), so adding one would loosen the policy for nothing |
| `frame-ancestors` | `'none'` | `'none'`, still last | Unchanged. The share page can't be framed by anyone (clickjacking on a page with remote write access to a shell) |

The browser viewer's PDF path is the spike's Browser PDF mode `open-pdf`: the preview pane shows the file name, **This browser can't show the PDF inside the page.** and an **Open PDF** button, which calls `window.open(blobUrl, '_blank', 'noopener,noreferrer')`. A top-level blob document is not framed, so `frame-ancestors` plays no part, and Safari shows the PDF in its own viewer under the unchanged policy (S-04). Only a PDF is ever opened top-level; `ImagePreview` has no `window.open` (source gate in 11-06 Task 1), so an SVG is never a top-level document.

### What stays unchanged

- `frame-ancestors 'none'`, still the last directive.
- The share token gate (`internal/webserver/capability_mw.go`): every share route still needs a valid capability token; preview bytes reach a browser only over the already-authenticated session WebSocket.
- The tailnet-only listener: the share server still binds only the tailnet address (`internal/tailnet`, `internal/webserver/listener_test.go`).
- No `blob:` in `default-src`, `script-src` or `connect-src`; no wildcard; no `'unsafe-eval'`; no report endpoint.

### Tests that pin it

- `internal/webserver/csp_mw_test.go` `TestCSPHeaders_PreviewDelta`: the whole policy equals Amendment 2 plus ` blob:` in img-src; `blob:` only in img-src; no `frame-src`, `object-src`, `child-src` or `worker-src`; `frame-ancestors 'none'` last.
- `internal/webserver/csp_mw_test.go` `TestCSPHeaders_RequiredTokens`: now expects `img-src 'self' data: blob:`.
- `internal/webserver/csp_integration_test.go` `TestCSPHeaderStrict_App`: the served `/app/` page carries `img-src 'self' data: blob:`, `blob:` exactly once, no `frame-src` or `object-src`, and `frame-ancestors 'none'` last.
- `frontend/src/components/__tests__/PreviewRenderers.test.tsx`: each hostile SVG fixture (`frontend/src/lib/__tests__/fixtures/preview/hostile-script.svg`, `hostile-external.svg`) is shown only as one `<img>` from its blob URL, with no `svg`, `script`, `iframe`, `foreignObject`, `object` or `embed` element in the document; Open PDF calls `window.open` with the blob URL, `_blank` and `noopener,noreferrer`, and only on click.

## Spike evidence (11-SPIKE.md)

Run 2026-11-21 in two Tart VMs (macOS 27.2, Safari 27.2, Wails v2.10.2) with a throwaway build that was never committed. Verdict **(b)**.

| Row | What it shows for this review |
|-----|-------------------------------|
| S-01 | Desktop (Wails web view, light): iframe, object and embed all render a `blob:` PDF in WKWebView's own PDF view, with scrolling and text selection. Only the iframe fires `load`, with a readable `application/pdf` plugin document. 0 CSP violations: the desktop page has no CSP. The SVG `<img>` loaded, ran no script, and the beacon got 0 requests |
| S-02 | Desktop, dark: nothing reloads on an appearance switch; PDF pages stay white (native look, accepted) |
| S-03 | Safari on the real share page, for each candidate (none, `frame-src blob:`, `object-src blob:`, both): no variant shows a PDF page. The framed blob document inherits `frame-ancestors 'none'` and is refused (RESEARCH Pitfall 6); object and embed are blocked under `connect-src`. `frame-ancestors 'none'` stayed last in every response |
| S-04 | Safari, Open PDF under the final delta (only `blob:` in img-src): the PDF opens top-level in a new tab with three pages and no new violation |
| S-05 | SVG in `<img>` in Safari and the desktop: draws (240x120), runs no script or handler, and both loopback beacons got 0 requests |

The decision block names: **PDF element:** iframe; **Share CSP delta:** img-src 'self' data: blob: (no frame-src and no object-src); **Browser PDF mode:** open-pdf.

## Desktop page CSP (observation)

The Wails desktop page has no Content-Security-Policy and Wails injects none (S-01: 0 violations, Assumption A8). This phase does not add one (CONTEXT "Desktop CSP": a desktop CSP is a separate change with a wide blast radius). On the desktop the defences are:

- Images and SVG only ever as `<img src=blob:>`, so SVG script and sub-resources never run or load.
- PDFs in WKWebView's own PDF view inside an iframe. Parsing an untrusted PDF is the OS viewer's job; this is an accepted residual risk (T-11-26). Portty enables no plugin or script for it.
- Markdown through the sanitizer (11-07), which is the only layer there: the share page has `script-src 'self'` as a second layer, the desktop page does not.

Recorded as an observation, not a finding to fix in this phase.

## PREV-03: no file-by-path route

There is no route that reads a file by path. File bytes travel only over the session's existing authenticated stream: the user's own shell process opens the file (`portty open`), and the daemon relays it to viewers. 11-04 pins this:

- `internal/daemon/preview_test.go` `TestPreview_NotOnRelayHandler`: POST and GET of the preview route on the loopback TCP relay handler get 404 or 405; the route lives only on the owner-only unix socket mux.
- `internal/daemon/preview_test.go` `TestPreview_SourceGate`: the daemon and relay preview code import neither `os`, `io/fs` nor `path/filepath`, so it can't touch the file system.
- `internal/webserver/files_removed_test.go` `TestPreviewRoute_NotOnShareServer`: the share server, with a valid token, has no preview route under `/sessions/` or `/app/sessions/` (`GET /app/*` is the single-page-app fallback).

## Dependencies

_11-07 fills this section (the Markdown renderer and sanitizer: versions, legitimacy check, configuration)._

## VM evidence and verdict

_11-10 fills this section (hostile fixtures and beacons in a real build, Safari share viewer, verdict)._
