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

Phase 11 adds two runtime npm packages to the frontend, both for the Markdown preview (PREV-02). The Go side adds none. Both reach the desktop app and the share page's `/app/` bundle, and both load only lazily (proof below).

### The two packages

Checked on 2026-11-21, before install. The evidence was saved under `/tmp/p11-07-deps-*` (`npm view <pkg>@<version> time maintainers scripts dist.integrity` and `gsd-tools package-legitimacy check --ecosystem npm marked dompurify`).

| | `marked` | `dompurify` |
|---|---|---|
| Version (exact pin, `--save-exact`) | **18.1.3** | **3.4.17** |
| Published | 2026-11-02T18:00:32Z | 2026-10-12T12:41:28Z |
| Age at install (2026-11-21) | 19 days | 40 days |
| Maintainers (npm) | chjj, styfle, amidknight, tonybrix | cure53 |
| Repository | github.com/markedjs/marked | github.com/cure53/DOMPurify |
| Integrity (npm and lockfile, identical) | `sha512-9ChMH8EVVgjgID49nsL6IWxWk5c6GtAsq/xw7U1ggkjkU5AGuv68oLTVlFRtvcUYZtpbNyjyvGo3YDu0fGapiw==` | `sha512-B/pSlYVHQ58kqISD/yDjuJ1yfzI34m1gF4lAod2bwSRhiN4K5Cl3H8Af7hqB2PmTL911tdEudnnXn5FnhRoz0w==` |
| Install scripts | none (no `preinstall`, `install` or `postinstall`) | none (its `prepare: husky` runs only for git or local installs, never for a registry tarball) |
| Runtime dependencies | none | `@types/trusted-types` 2.0.7 (optional, types only) |
| License | MIT | MPL-2.0 OR Apache-2.0 |
| Legitimacy check | `SUS`, reason `too-new` only | `SUS`, reason `too-new` only |

**Legitimacy result.** The checker rates each package's newest release (marked 18.2.1 of 2026-11-16, dompurify 3.4.18 of 2026-11-14), and both of those are under 14 days old, hence `too-new`. Both packages exist, are years old, have 137.9M and 107.6M weekly downloads, have their source repositories, aren't deprecated and have no postinstall. CONTEXT "Decided at planning" replaces the human checkpoint for `too-new` with the 14-day rule: the pinned versions are older, and their publish dates match the planned ones. There was no `SLOP` verdict and no other `SUS` reason, so the install went ahead. After `pnpm add --save-exact`, `pnpm install --frozen-lockfile` is clean.

**marked 18.1.3** is above 18.0.2, which fixed the OOM denial of service GHSA-6v9c-7cg6-27q7 (infinite recursion in the 18.0.0 tokenizer). It is the newest marked that is at least 14 days old.

**Why DOMPurify 3.4.17 is acceptable.** The only fix in 3.4.18 is GHSA-qm7q-q8h9-jg8q, in `SAFE_FOR_TEMPLATES` mode. This configuration never enables `SAFE_FOR_TEMPLATES`, or any of the other modes where the 2026 DOMPurify advisories cluster (`IN_PLACE`, `RETURN_DOM`, custom element handling, hooks, `PARSER_MEDIA_TYPE`). 3.4.17 contains every earlier fix, including the `high` "selectedcontent re-clone" XSS fixed in 3.4.5. The move to 3.4.18 or later is the todo `.planning/todos/pending/2026-11-21-dompurify-3418.md`, due 2026-11-28, once 3.4.18 is 14 days old. DOMPurify is the desktop page's only XSS layer and has frequent advisories, so it needs periodic bumps: re-check its advisory list at each milestone.

### Configuration (`frontend/src/lib/markdown.ts`)

The configuration has two layers. Unsanitized HTML never reaches a live or detached DOM node: DOMPurify returns a string, which `MarkdownPreview` assigns once.

1. **marked** (`new Marked({gfm: true, async: false})`):
   - The `html` renderer returns `''`, so raw HTML, block and inline, is dropped.
   - `image` returns `<span class="md-image">` holding the escaped alt text (or `Image`). No `<img>` is ever produced, so nothing is fetched or decoded.
   - `checkbox` returns a `md-task` span with ☑ or ☐, so no `<input>` is produced.
   - Blockquote and list nesting is capped at 20 levels (`MAX_DEPTH`). Past it, a container line stays literal paragraph text (markdown-it's `maxNesting` idea). Without the cap, marked 18.1.3 takes time that grows with the square of the depth (about 1 s at 1,000 levels) and overflows the stack near 2,000.
2. **DOMPurify** in plain string mode:
   - `ALLOWED_TAGS`: h1-h6, p, br, hr, ul, ol, li, blockquote, pre, code, em, strong, del, a, table, thead, tbody, tr, th, td, span.
   - `ALLOWED_ATTR`: href, class, title, align.
   - `ALLOWED_URI_REGEXP: /^(?:https?:|mailto:|#)/i`.
   - `ALLOW_DATA_ATTR: false`, `ALLOW_ARIA_ATTR: false`, `FORBID_ATTR: ['style']`.
   - `ADD_URI_SAFE_ATTR: ['align']`. DOMPurify checks every allowed attribute that isn't URI-safe against `ALLOWED_URI_REGEXP`, so without this, `align="center"` would be stripped. It exempts only `align` from the URI check and adds no tag or attribute.
   - No `USE_PROFILES` (a profile replaces the allowlists and turns SVG and MathML on), no `ADD_TAGS` or `ADD_ATTR`, no hooks, and none of the `IN_PLACE`, `SAFE_FOR_TEMPLATES` or `RETURN_DOM*` modes.

Links never navigate the pane (`frontend/src/components/MarkdownPreview.tsx`):

- One delegated handler prevents the default of every anchor `click`, `auxclick` and Enter.
- `#fragment` links scroll to the first heading whose slug matches, and no `id` is ever added.
- Any other link goes through `isAllowedScheme`, then `getRisk(text, href)`, then `LinkConfirmPopover` (risky) or `openLink` (desktop `BrowserOpenURL`; browser `window.open` with `noopener,noreferrer`).
- An anchor whose href the sanitizer stripped has no href, so it is neither focusable nor styled as a link.

### Lazy-load proof

`MarkdownPreview` reaches the renderer only through `import('../lib/markdown')`; a source test in `MarkdownPreview.test.tsx` rejects a static import of `marked`, `dompurify` or `lib/markdown`.

- **Standard production build** (`vite build --outDir /tmp/p11-07-dist`): the entry chunk `assets/index-*.js` contains no `ALLOWED_URI_REGEXP`. It is the build's only JS chunk, because nothing imports `MarkdownPreview` until 11-09 composes it into PreviewPanel, so Rollup leaves the Markdown code out entirely.
- **Build with MarkdownPreview reachable** (a throwaway second entry that imports it, the way 11-09 will; nothing committed; `/tmp/p11-07-dist-probe`):

  | Chunk | Contains `ALLOWED_URI_REGEXP` |
  |-------|-------------------------------|
  | `index-*.js` (entry) | no |
  | the MarkdownPreview chunk | no; it reaches `markdown-*.js` only through `import("./markdown-…")` |
  | `markdown-*.js` (79 KB, marked and DOMPurify) | yes |

11-09's gate should re-run the standard-build check once PreviewPanel imports MarkdownPreview: `ALLOWED_URI_REGEXP` absent from the entry chunk and present in exactly one lazy chunk.

### Tests that pin it

- `frontend/src/lib/__tests__/markdown.test.ts`:
  - The allowed tags render, raw HTML is dropped, images become placeholders and tasks become glyphs.
  - Every section of `frontend/src/lib/__tests__/fixtures/preview/hostile.md` renders with no `img`, `script`, `style`, `iframe`, `object`, `embed`, `form`, `input`, `meta`, `base`, `link`, `svg` or `math` markup, no `on*` attribute, no `style=`, `src=`, `srcset`, `url(` or `@import`, and only `href`, `class`, `title` and `align` attributes.
  - `javascript:` links (every obfuscation), `data:`, `vbscript:`, `file:` and relative links keep no href. `https:`, `mailto:` and `#intro` keep theirs. Tables keep `align`.
  - 10,000-level blockquote and list nesting renders, capped at 20. A 2 MB line renders.
- `frontend/src/components/__tests__/MarkdownPreview.test.tsx`:
  - Links: clicks are prevented, and safe links go to `openLink` while a mismatch shows `LinkConfirmPopover`. `auxclick` and Enter are prevented. Fragments scroll with no `id`. A stripped anchor isn't focusable.
  - Decoration: placeholders get `role="img"` and an `aria-label`, links get title = href, and tables sit in `.md-table-wrap`. The hostile fixture leaves no fetching, scripting or styled element in the pane.

### Residual risk: marked's emphasis parsing time (open)

Found while measuring the DoS bounds. marked 18.1.3 takes time that grows with the square of the number of unclosed emphasis delimiters in one paragraph. The same timings show with and without this configuration, so it is marked's own inline tokenizer. Measured in jsdom (`*a **b _c` repeated):

| Size | Time |
|------|------|
| 10 KB | 0.3 s |
| 20 KB | 1.3 s |
| 40 KB | 5 s |
| 80 KB | 23 s |

A 2 MB paragraph of this did not finish in several minutes. No marked advisory covers it (`gh api repos/markedjs/marked/security-advisories`, 2026-11-21).

The parse runs on the page's main thread, so such a file freezes that viewer's window until it finishes. That includes an unattended viewer, if 11-09 renders a background preview eagerly. The source is a file someone in the session explicitly ran `portty open` on, and it runs no script and fetches nothing, so this is availability only. The mitigations are a decision for the planner and the user, not something to improvise here:

- render only the visible preview tab (11-09);
- parse in a Web Worker that is terminated after a time limit (the share CSP would need `worker-src 'self'`);
- a much lower size cap for Markdown than the 20 MB transfer cap;
- markdown-it, research's alternative, if it proves linear here.

Recorded in `.planning/phases/11-file-preview/deferred-items.md`.

## VM evidence and verdict

_11-10 fills this section (hostile fixtures and beacons in a real build, Safari share viewer, verdict)._
