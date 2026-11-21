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
default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>; img-src 'self' data: blob:; worker-src 'self'; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
```

`worker-src 'self'` was added by the 11-07 fix (11-07-FIX.md); 11-06 added only `blob:` in img-src.

### Directive by directive

| Directive | Before | After | Change and reason |
|-----------|--------|-------|-------------------|
| `default-src` | `'none'` | `'none'` | Unchanged. No `blob:` |
| `script-src` | `'self' 'wasm-unsafe-eval'` | same | Unchanged. No `blob:`, so a blob can never run as script |
| `style-src` | `'self' 'unsafe-inline'` | same | Unchanged |
| `connect-src` | `'self' wss://<host>` | same | Unchanged. No `blob:`. This is also why Safari can't show a framed or embedded blob PDF (S-03); loosening it was not tried and is not proposed |
| `img-src` | `'self' data:` | `'self' data: blob:` | **Changed.** Image and SVG previews are `<img src=blob:…>` built from bytes that arrived over the session's own authenticated socket. `blob:` URLs are same-origin objects the page itself created; no network origin is added, so an image can still fetch nothing from elsewhere. An SVG inside an `<img>` runs no script and loads no sub-resource (S-05) |
| `worker-src` | absent (falls back to `script-src 'self'`) | `'self'` | **Added (11-07 fix).** The Markdown preview parses in a dedicated module Web Worker that is terminated after 3 s (see "Residual risk" below, now mitigated). Vite emits it as a hashed same-origin file, `assets/markdownWorker-*.js`, loaded with `new Worker(new URL(…, import.meta.url), {type: 'module'})`; the production build was inspected and the URL is never `blob:` or `data:`, which this directive keeps blocked. Without the directive a worker would fall back to `script-src 'self'`, the same sources, so this loosens nothing; naming it pins the intent and stops a later `script-src` change from widening workers. The worker script is served by the same `/app/` handler with this same CSP and fetches nothing |
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

- `internal/webserver/csp_mw_test.go` `TestCSPHeaders_PreviewDelta`: the whole policy equals Amendment 2 plus ` blob:` in img-src and `worker-src 'self'` after it; `blob:` only in img-src; `worker-src` exactly `'self'`; no `frame-src`, `object-src` or `child-src`; `frame-ancestors 'none'` last.
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

### Amendment (11-REVIEW CR-01): a frames-only desktop CSP

The observation above missed one path. Wails v2.10.2 is the web view's `WKNavigationDelegate` but implements no navigation policy (`WailsContext.m:281`), and its `external` script-message handler accepts messages from any frame without checking `message.frameInfo` (`:249`, `:479-494`). So if a link in an untrusted PDF navigated the preview iframe to an attacker page, that page could blindly call no-argument bindings such as `main.App.QuitAll` (ends every session on an unreachable Mac), `RegenerateSigningKey` or `DownloadUpdate`.

The desktop page now has a CSP, set by a `<meta http-equiv>` in `frontend/index.html`, and it restricts frames and workers only:

```
frame-src blob:; child-src blob:; worker-src 'self'
```

- `frame-src blob:` allows the PDF preview's `blob:` document and nothing else. WebKit checks the embedding document's `frame-src` on every load into a child frame, including a navigation the frame starts itself, so a PDF link to `https:`, `http:`, `data:` or `wails:` is blocked and the iframe keeps the PDF.
- `child-src blob:` is the CSP2 fallback for the same frames.
- `worker-src 'self'` keeps the Markdown worker loading. Without it, workers would fall back to `child-src blob:` and the worker (a hashed same-origin file, `wails://wails/assets/markdownWorker-*.js` on the desktop) would be blocked.
- No `default-src`, `script-src`, `connect-src` or `style-src`, so Wails' injected `/wails/runtime.js` and `/wails/ipc.js`, its IPC and every existing load are unaffected. Wails' asset server parses and re-renders `index.html` (`processIndexHTML` in `pkg/assetserver/assetserver.go`): it only inserts its scripts at the top of `<head>` and keeps the meta.
- The app has no other frame or worker: a grep of `frontend/src` finds only the PDF iframe and the Markdown worker.
- On the share page the same meta combines with the header policy. Both must allow a load, so the header's `default-src 'none'` still blocks every frame and its `worker-src 'self'` is unchanged.
- `PreviewRenderers.test.tsx` pins the meta: exactly one CSP meta in `index.html`, with exactly this content.

**Not done: `sandbox` on the PDF iframe.** A sandboxed frame without `allow-same-origin` gets an opaque origin. Then `contentDocument` is null, so the load check that detects a non-PDF document stops working, and WebKit may refuse the PDF plugin in a sandboxed frame. 11-SPIKE.md proved the plain iframe only, so this is left out until a spike proves `sandbox` keeps the native PDF view.

**Upstream:** the frame-agnostic `external` handler is a Wails v2 issue. Wails could check `message.frameInfo.isMainFrame` in `didReceiveScriptMessage` and deny subframe navigations to anything but `wails:` and `blob:`. Raise it with the Wails project. The CSP above closes the path in Portty regardless.

**VM check needed (pending):** make a PDF with a `/URI` link to a loopback beacon, open it on the desktop and click the link. Expect 0 beacon requests and the iframe still showing the PDF. Also open a normal `.md` on the desktop and confirm it still renders, which proves `worker-src 'self'` matches the `wails://` origin.

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

### Configuration (`frontend/src/lib/markdown.ts`, `frontend/src/lib/markdownSanitize.ts`)

The configuration has two layers. Unsanitized HTML never reaches a live or detached DOM node: DOMPurify returns a string, which `MarkdownPreview` assigns once.

Since the 11-07 fix the two layers run in different places. marked (`markdown.ts`, `parseMarkdown`) runs in a Web Worker (`markdownWorker.ts`) started by `parseMarkdownInWorker.ts` and terminated when the parse settles or after `MARKDOWN_PARSE_TIMEOUT_MS` (3000). Its output, still unsanitized, comes back as a string. DOMPurify (`markdownSanitize.ts`, `sanitizeMarkdown`) runs on the main thread: in a worker there is no `document`, DOMPurify sets `isSupported = false`, and `sanitize()` returns its input unchanged (purify.es.mjs: `if (!DOMPurify.isSupported) return dirty`). Sanitizing in the worker would therefore have been a silent bypass. The sanitizer configuration below is byte for byte the 11-07 one.

1. **marked** (`new Marked({gfm: true, async: false})`, in the worker):
   - The `html` renderer returns `''`, so raw HTML, block and inline, is dropped.
   - `image` returns `<span class="md-image">` holding the escaped alt text (or `Image`). No `<img>` is ever produced, so nothing is fetched or decoded.
   - `checkbox` returns a `md-task` span with ☑ or ☐, so no `<input>` is produced.
   - Blockquote and list nesting is capped at 20 levels (`MAX_DEPTH`). Past it, a container line stays literal paragraph text (markdown-it's `maxNesting` idea). Without the cap, marked 18.1.3 takes time that grows with the square of the depth (about 1 s at 1,000 levels) and overflows the stack near 2,000.
2. **DOMPurify** in plain string mode, on the main thread:
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

Since the 11-07 fix, `MarkdownPreview` reaches marked only through the worker and DOMPurify only through `import('../lib/markdownSanitize')`; a source test in `MarkdownPreview.test.tsx` rejects a static import of `marked`, `dompurify`, `lib/markdown` or `lib/markdownSanitize`, and checks that `parseMarkdownInWorker.ts` imports nothing and builds the worker with `new Worker(new URL('./markdownWorker.ts', import.meta.url), { type: 'module' })`.

11-07 fix build check (2026-11-21, output not committed): the standard `pnpm build` still has a single entry chunk with no marked or DOMPurify code (MarkdownPreview is not imported until 11-09). A throwaway build with a second entry importing MarkdownPreview gave:

| Chunk | Contents |
|-------|----------|
| `index-*.js` (entry) | no marked, no `ALLOWED_URI_REGEXP` |
| the MarkdownPreview chunk | the `new Worker(new URL("markdownWorker-*.js", import.meta.url), {type: "module"})` call and `import("./markdownSanitize-*.js")` |
| `markdownWorker-*.js` (49.5 KB) | marked only, a self-contained IIFE with no imports; a separate same-origin file, not a `blob:` or `data:` URL |
| `markdownSanitize-*.js` (29 KB) | DOMPurify and the configuration (`ALLOWED_URI_REGEXP`) |

The 11-07 build check that follows predates the split.

- **Standard production build** (`vite build --outDir /tmp/p11-07-dist`): the entry chunk `assets/index-*.js` contains no `ALLOWED_URI_REGEXP`. It is the build's only JS chunk, because nothing imports `MarkdownPreview` until 11-09 composes it into PreviewPanel, so Rollup leaves the Markdown code out entirely.
- **Build with MarkdownPreview reachable** (a throwaway second entry that imports it, the way 11-09 will; nothing committed; `/tmp/p11-07-dist-probe`):

  | Chunk | Contains `ALLOWED_URI_REGEXP` |
  |-------|-------------------------------|
  | `index-*.js` (entry) | no |
  | the MarkdownPreview chunk | no; it reaches `markdown-*.js` only through `import("./markdown-…")` |
  | `markdown-*.js` (79 KB, marked and DOMPurify) | yes |

11-09's gate should re-run the standard-build check once PreviewPanel imports MarkdownPreview: `ALLOWED_URI_REGEXP` absent from the entry chunk and present in exactly one lazy chunk.

### Tests that pin it

- `frontend/src/lib/__tests__/markdown.test.ts` (since the 11-07 fix, through `sanitizeMarkdown(parseMarkdown(src))`, the two steps MarkdownPreview runs):
  - The allowed tags render, raw HTML is dropped, images become placeholders and tasks become glyphs.
  - Every section of `frontend/src/lib/__tests__/fixtures/preview/hostile.md` renders with no `img`, `script`, `style`, `iframe`, `object`, `embed`, `form`, `input`, `meta`, `base`, `link`, `svg` or `math` markup, no `on*` attribute, no `style=`, `src=`, `srcset`, `url(` or `@import`, and only `href`, `class`, `title` and `align` attributes.
  - `javascript:` links (every obfuscation), `data:`, `vbscript:`, `file:` and relative links keep no href. `https:`, `mailto:` and `#intro` keep theirs. Tables keep `align`.
  - 10,000-level blockquote and list nesting renders, capped at 20. A 2 MB line renders.
- `frontend/src/components/__tests__/MarkdownPreview.test.tsx`:
  - Links: clicks are prevented, and safe links go to `openLink` while a mismatch shows `LinkConfirmPopover`. `auxclick` and Enter are prevented. Fragments scroll with no `id`. A stripped anchor isn't focusable.
  - Decoration: placeholders get `role="img"` and an `aria-label`, links get title = href, and tables sit in `.md-table-wrap`. The hostile fixture leaves no fetching, scripting or styled element in the pane.
  - 11-07 fix: jsdom has no Worker, so an in-thread stub runs the real `parseMarkdown`. A parse that never finishes calls `onFail` within 3.5 s, the worker is terminated, and a main-thread timer keeps firing. Unmounting terminates a running parse.
- `frontend/src/lib/__tests__/parseMarkdownInWorker.test.ts` (11-07 fix): the timeout, error, abort and termination logic with a fake worker, and a real `worker_threads` thread running `parseMarkdown`, where 81 KB of `*a **b _c` is rejected within 3.5 s, the thread is terminated, and a 50 ms main-thread timer keeps ticking.

### Residual risk: marked's emphasis parsing time (mitigated by the 11-07 fix)

**Status: mitigated (2026-11-21, 11-07-FIX.md; decision in 11-CONTEXT.md "Decided during execution").** The parse now runs in a dedicated Web Worker, never on the main thread. Each render gets a fresh worker; it is terminated when the parse settles, when the pane unmounts, or after `MARKDOWN_PARSE_TIMEOUT_MS` (3000 ms), and on timeout the pane shows "Couldn't Show File". A hostile file therefore costs a viewer at most 3 s of one background thread, and the window, terminal tabs included, stays responsive throughout. The limit bounds any parser pathology, not just this one. The share CSP gained `worker-src 'self'` for it (above). marked stays at 18.1.3; the slowdown itself is unchanged and still unreported upstream. Per the same decision, 11-09 renders a preview pane only while its tab is visible. What remains: a legitimate Markdown file that takes marked more than 3 s (about 40 KB of dense unclosed emphasis) shows "Couldn't Show File" instead of rendering, and the worker has to load on the desktop page's `wails://` origin too (11-10 VM check).

The original finding, as recorded by 11-07:

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

Recorded in `.planning/phases/11-file-preview/deferred-items.md` (now closed there).

Reproduction note from the fix: the input is `'*a **b _c'` repeated with no separator. With a trailing space (`'*a **b _c '`) marked is linear (80 KB in 18 ms). In a node worker thread the no-separator string took 0.3 s at 10 KB and 1.2 s at 20 KB, and was killed at 3 s at 40 KB.

## VM evidence and verdict

Run 2026-11-21 by 11-10 in the two Tart VMs with the real ad-hoc build B (`v1.2.0-vmB`, product code `7042f5ce`, sha256 `248f596870e4…`); rows in `.planning/phases/11-file-preview/11-VM-CHECKS.md`. Host names are written `<host>`; no link, token or capability is recorded.

### Served share CSP (P-04)

From the home VM, `curl` of a live share link (302 to `/app/`) and of `/app/` (200): both responses carry

```
default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>; img-src 'self' data: blob:; worker-src 'self'; font-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
```

character for character the "After (Amendment 3)" policy above, `frame-ancestors 'none'` last. The token gate held: a one-character-wrong token got 401 on the link and on the info API (the valid token got 302 and 200).

### Safari under that policy (P-03, P-04)

- A `securitypolicyviolation` listener, error and `unhandledrejection` hooks and a `window.open` counter were installed through WebDriver before each Safari row. Over P-03, P-04, P-06 and U-02 Safari logged **0 violations and 0 errors**.
- Image and SVG previews were `<img src=blob:…>` (PNG 1600x900, SVG 240x120); the PDF pane showed the Open PDF button, and Open PDF opened a top-level `blob:` window of `application/pdf` (the only `window.open`).
- The Markdown parse worker loaded as `/app/assets/markdownWorker-*.js` (same origin) and the sanitizer as `/app/assets/markdownSanitize-*.js`, with no violation, so `worker-src 'self'` is enough and nothing needed `blob:` or `data:` for a worker. An 81,000-byte `*a **b _c` file hit the 3 s limit and showed Couldn't Show File while a 50 ms page timer never gapped more than 59 ms. The same worker loaded on the desktop page's `wails://` origin (P-02), so the residual-risk check above is closed.

### Hostile fixtures against loopback beacons (P-03)

The office local tab (WKWebView), a home peer tab (WKWebView, through the home daemon's proxy) and home Safari each showed hostile.md, hostile-script.svg and an external-fetch SVG, with every network reference pointed at a loopback beacon on 127.0.0.1:8765 in each VM. Both beacons logged **0 connections** during the run and for more than 30 s after (only their self-tests). Window and page titles were unchanged, no window or tab opened, the Markdown had no `on*` attribute and no fetching element (the only `svg` elements were the app's own image-placeholder icons), and typing still worked.

One finding about the fixture, not the renderer: `hostile-external.svg` as committed is **refused by `portty open`** as an unsupported type, because its billion-laughs `<!DOCTYPE svg [ … ]>` internal subset contains `>`, which 11-08's SVG sniffer treats as not-an-SVG (the documented safe-direction shortcut). So a billion-laughs SVG never reaches a viewer through the CLI. Its fetch vectors (`@import`, `<image href>`, `<image xlink:href>`, `<use href>`, a `<foreignObject>` iframe) were tested with the same file minus the DOCTYPE block, with the result above. Side effect for users: a real SVG with an internal DTD subset (some editors write entity declarations there) is refused too.

### Route probes (P-07)

- Share server, with p1's valid token: `POST /sessions/<id>/preview` 405, `POST /app/sessions/<id>/preview` 405, `GET /sessions/<id>/preview` 404, and `GET /app/sessions/<id>/preview` 200 with a body byte-identical to `GET /app/` (620 bytes, the app's HTML shell): the `/app/*` single-page-app fallback that 11-04 recorded, not a preview route, and never file bytes.
- Office loopback relay (`127.0.0.1:<port>`, the daemon's only loopback listener): `POST` and `GET /sessions/<id>/preview` both 404.
- Previews still reached viewers only as MsgPreview frames over each viewer's existing session socket (`preview end … conn=N … status=ok` in the office log for every viewer).

### Desktop page (observation, P-02 and P-03)

As the spike found, the desktop page has no CSP. In the real build its defences held on their own: SVG only as `<img>` (0 beacon requests), PDF in WKWebView's own viewer, Markdown sanitized (no script, no fetch). A damaged PDF (a PDF header followed by random bytes) loaded WKWebView's PDF plugin with 0 pages and a blank view instead of Couldn't Show File (11-06's predicted limit); parsing it is the OS viewer's job (T-11-26).

### Install path (P-08, partial)

The menu item's administrator prompt appeared through `osascript` only when no link existed; a foreign link, the app's own link and a disk-image copy each got their warning or notice with no prompt and no change (P-08d..f). Don't Allow changed nothing and logged nothing. macOS 27.2 shows its own prompt copy ("Allow administrator access for a script started by “Portty”?" with Apple's malware-verification warning) and does **not** display the app's `with prompt` text. The successful install (P-08c) needs the VM password typed into that prompt and is pending the user; it is not one of the rows this verdict rests on.

### Residual risks

- PDFs are parsed by the OS's PDF viewer inside WKWebView (desktop) and Safari's viewer (browser, top-level `blob:`); accepted (T-11-26).
- Wails v2.10.2 accepts IPC messages from any frame. The desktop CSP's `frame-src blob:` stops a PDF link from loading a foreign page into the preview iframe (11-REVIEW CR-01, amendment above); the handler itself is an upstream issue.
- DOMPurify is the desktop page's only XSS layer and needs periodic bumps; the move to 3.4.18 or later is tracked in `.planning/todos/pending/2026-11-21-dompurify-3418.md` (due 2026-11-28). Re-check its advisories at each milestone.
- A legitimate Markdown file that takes marked more than 3 s shows Couldn't Show File.
- `PORTTY_SESSION_ID` is inherited: inside `tmux` or `screen` started in another session, `portty open` targets the session that started the server (11-03 documented limit).
- The share server's `/app/*` fallback answers 200 with the app shell for any path, including `/app/sessions/<id>/preview`; it serves no file bytes.

**Verdict:** approved. Every row this review cites (P-03, P-04, P-07) is PASS: the served share CSP is exactly the reviewed Amendment 3 policy with `frame-ancestors 'none'` last, the token gate refuses a wrong token, Safari logged no violation, hostile content fetched and ran nothing in either engine, and neither the share server nor the loopback relay has a preview route.
