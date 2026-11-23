---
phase: 11-file-preview
plan: 07
subsystem: frontend
status: complete
tags: [frontend, preview, markdown, sanitizer, dompurify, marked, security-review, supply-chain]
requires: [11-06]
provides:
  - "MarkdownPreview({blob, name, webMode, onFail, autoFocus?})"
  - "renderMarkdown(src): sanitized HTML string (frontend/src/lib/markdown.ts, lazy chunk)"
  - "hostile.md fixture with BEACON placeholders for 11-10"
  - "Markdown document-scale styles (.preview-markdown-pane, .preview-markdown, .md-*)"
  - "11-SECURITY-REVIEW.md Dependencies section"
affects: [11-09, 11-10]
tech-stack:
  added:
    - "marked 18.1.3 (exact pin)"
    - "dompurify 3.4.17 (exact pin)"
  patterns:
    - "Sanitized string assigned once through a memoized {__html} object; trusted DOM decoration afterwards"
    - "Delegated anchor handler: every click/auxclick/Enter prevented, links through isAllowedScheme -> getRisk -> LinkConfirmPopover/openLink"
    - "marked blockquote/list tokenizers wrapped with a nesting cap (MAX_DEPTH 20)"
key-files:
  created:
    - frontend/src/lib/markdown.ts
    - frontend/src/lib/__tests__/markdown.test.ts
    - frontend/src/lib/__tests__/fixtures/preview/hostile.md
    - frontend/src/components/MarkdownPreview.tsx
    - frontend/src/components/__tests__/MarkdownPreview.test.tsx
    - .planning/phases/11-file-preview/deferred-items.md
  modified:
    - frontend/package.json
    - frontend/pnpm-lock.yaml
    - frontend/src/style.css
    - .planning/phases/11-file-preview/11-SECURITY-REVIEW.md
decisions:
  - "Blockquote and list nesting capped at 20 levels in markdown.ts; deeper container lines stay literal paragraph text (marked 18.1.3 overflows the stack near 2,000 levels and its time grows with the square of the depth)"
  - "ADD_URI_SAFE_ATTR ['align'] added to the DOMPurify config: every allowed non-URI-safe attribute is checked against ALLOWED_URI_REGEXP, so align was stripped without it"
  - "The {__html} object is memoized: React 19 reassigns innerHTML whenever its identity changes, which wiped the decoration and detached anchors on any re-render"
  - "auxclick on a Markdown link is prevented and opens nothing; click and Enter open through the gate"
metrics:
  duration: "~75 min"
  completed: 2026-11-21
actuals:
  tokens: 12700    # chars/4 over the realized diff (50,880 chars incl. lockfile and review)
  tasks: 3
  commits: 3
plan_head_before: 09107846e9c9af4490a571c5746c8f31aa384627
plan_head_after: 8224ea92a9126350b32e35c8f45e729ba518a0e4
---

# Phase 11 Plan 07: Markdown preview Summary

Markdown previews render through marked 18.1.3 and DOMPurify 3.4.17 (exact pins, lazy chunk). Raw HTML is dropped, images become placeholders, and an explicit tag, attribute and URI allowlist applies. The pane never navigates and never fetches: every link goes through the terminal's scheme and risk gate. The review records the supply-chain evidence, and one open residual DoS in marked is flagged for a decision.

## Component contract (for 11-09)

`MarkdownPreview({ blob: Blob, name: string, webMode: boolean, onFail: () => void, autoFocus?: boolean })` in `frontend/src/components/MarkdownPreview.tsx`.

- It reads `blob.text()` and never fetches the blob URL. It imports `../lib/markdown` lazily (marked and DOMPurify land in their own chunk). If reading, importing or rendering throws, it calls `onFail` once.
- Markup: `div.preview-markdown-pane` (height 100%, scrolls vertically, padding 32px 24px, `--color-window`) > `article.preview-markdown[aria-label=<name>][tabindex=0]` (720px column). It needs a definite height from its parent, as `.preview-image` does.
- It renders only its pane content, with nothing before the text arrives (an empty article). PreviewPanel owns the receiving and error panes and the ErrorBoundary.
- `autoFocus` focuses the article once the content is in.
- `webMode` is accepted for the contract but unused: `openLink` already chooses `BrowserOpenURL` or `window.open(…, 'noopener,noreferrer')`.
- `LinkConfirmPopover` is rendered by MarkdownPreview itself (portal to body) for risky links.
- Class names inside the article:
  - `.md-image` placeholder span, decorated with `role="img"`, `aria-label="Image: <alt>"` and `svg.md-image__icon` (PhotoIcon, 14px).
  - `.md-task` (☑/☐ glyph).
  - `.md-table-wrap` (each table wrapped for horizontal scroll).
- Link titles are set to their href.

## Pinned versions and dates

| Package | Version | Published | Age at install | Integrity |
|---------|---------|-----------|----------------|-----------|
| marked | 18.1.3 | 2026-11-02T18:00:32Z | 19 days | sha512-9ChMH8EV…Gapiw== |
| dompurify | 3.4.17 | 2026-10-12T12:41:28Z | 40 days | sha512-B/pSlYVH…Roz0w== |

- Legitimacy check: both `SUS`, reason `too-new` only. The checker rates the newest releases (marked 18.2.1, dompurify 3.4.18), and the pinned versions are older.
- There are no install scripts, and the publish dates match CONTEXT. `pnpm install --frozen-lockfile` is clean. The evidence is in `/tmp/p11-07-deps-{marked,dompurify}.txt` and `/tmp/p11-07-deps-legitimacy.json`.
- **DOMPurify todo:** `.planning/todos/pending/2026-11-21-dompurify-3418.md`, due 2026-11-28. Move to 3.4.18 or later once it is 14 days old. Its only fix, GHSA-qm7q-q8h9-jg8q, is in `SAFE_FOR_TEMPLATES` mode, which this config never enables. DOMPurify needs periodic bumps: it has frequent advisories and is the desktop page's only XSS layer. This plan didn't act on the todo.

## Tests (for 11-10's TESTING.md rows)

- `frontend/src/lib/__tests__/markdown.test.ts`:
  - "renderMarkdown (PREV-02)": allowed tags, raw HTML dropped, md-image with alt or Image, alt text escaped, md-task glyphs and no input.
  - "renderMarkdown hostile fixture (PREV-02)": the plan's sections are present; each section, and then the whole document, renders inert (no banned markup, no `on*`, attributes only href/class/title/align, hrefs only http/https/mailto/#); javascript:, data:, vbscript:, file: and relative links keep no href; https, mailto and #intro keep theirs; images become placeholders (also inside a link) and the beacon address never appears; tables keep align.
  - "renderMarkdown DoS bounds (PREV-02, T-11-25)": a 10,000-level blockquote and a 10,000-level list are capped at MAX_DEPTH; an indentation-nested list is capped; ordinary nesting is kept; a 2 MB line renders.
- `frontend/src/components/__tests__/MarkdownPreview.test.tsx`:
  - "MarkdownPreview (PREV-02)": the labelled article; `blob.text()` with no fetch; a source check rejects static imports of marked, DOMPurify and lib/markdown; onFail on a read failure.
  - "MarkdownPreview links (PREV-02, Pitfall 7)": a safe link goes to openLink with the click prevented; a mismatch shows the popover, where Cancel does nothing and Continue opens, and the same anchor survives the re-render; auxclick and Enter are prevented; #fragment scrolls the slug-matched heading with no id added, and no match means no scroll; a stripped javascript: anchor isn't focusable.
  - "MarkdownPreview decoration (PREV-02)": placeholder role and label plus the icon, link title = href, `.md-table-wrap`; the hostile fixture leaves no fetching, scripting or styled element in the pane and fetch is never called; autoFocus.
- Fixture `frontend/src/lib/__tests__/fixtures/preview/hostile.md` has one `## <label>` section per case. Every network reference is `http://BEACON/md-<label>`, which 11-10 replaces with the VM's loopback beacon.

## Gate results

- Task 1 verify: PASS (`/tmp/p11-07-t1-vitest.log`, `/tmp/p11-07-t1-frozen.log`).
- Task 2 verify: PASS, 65 tests across markdown, MarkdownPreview and style.tokens (`/tmp/p11-07-t2-vitest.log`).
- Task 3 gate, each step run once (`/tmp/p11-07-gate.log`):
  - Production build: the entry chunk has no `ALLOWED_URI_REGEXP`. For the lazy chunk, see Deviation 4.
  - Full vitest: 101 files, 1459 tests passed (`/tmp/p11-07-gate-vitest.log`).
  - `tsc --noEmit`: clean.
  - `go build ./...` and `go vet ./...`: clean.
  - `go test -race -short -count=1 ./...`: 13 `ok` packages, 0 FAIL lines (`/tmp/p11-07-gate-go.log`). No flake reruns.
  - check-brand: clean. check-traceability-paths: 174 paths exist.
  - Go files, App.tsx, TabBar.tsx, ImagePreview, PdfPreview, previewStore and previewAssembler are unchanged since the base. wailsjs is untouched.

## Deviations from Plan

**1. [Rule 2 - DoS] Nesting cap in markdown.ts**
- **Found during:** Task 2 (DoS bounds test).
- **Issue:** marked 18.1.3 threw `RangeError: Maximum call stack size exceeded` on 10,000-level blockquotes and lists. Measured: blockquotes are fine at 2,000 levels and overflow by 3,000; lists overflow by 1,500. Time grows with the square of the depth (a 1,000-level list takes 1 s, 2,000 takes 4 s).
- **Fix:** marked's own `blockquote` and `list` tokenizers are wrapped with a depth counter. Past `MAX_DEPTH = 20` (markdown-it's strict-preset `maxNesting`), a container line becomes a literal paragraph. This covers indentation nesting too. Ordinary nesting is unchanged (tested).
- **Files:** frontend/src/lib/markdown.ts and its test. **Commit:** 954ca500.

**2. [Rule 1 - Bug] React 19 reassigned the Markdown HTML on every re-render**
- **Found during:** Task 2 (popover test).
- **Issue:** React 19 compares `dangerouslySetInnerHTML` by object identity. An inline `{__html}` therefore reset innerHTML on any re-render, such as opening `LinkConfirmPopover`. That dropped the decoration (icons, link titles, table wrappers) and detached the anchors.
- **Fix:** the object is memoized on the html string, and the decoration runs once per string (StrictMode re-runs effects). A test checks that the same anchor is still connected after the popover closes.
- **Files:** MarkdownPreview.tsx and its test. **Commit:** 954ca500.

**3. [Rule 1 - Bug] `align` needs `ADD_URI_SAFE_ATTR`**
- DOMPurify checks every allowed attribute that isn't URI-safe against `ALLOWED_URI_REGEXP`, so the strict regexp stripped `align="center"`, which a probe confirmed. `ADD_URI_SAFE_ATTR: ['align']` exempts only align from the URI check. It is not a profile, an ADD_TAGS, a hook or a risky mode. Commit ac390542.

**4. Lazy-chunk half of the Task 3 gate**
- Nothing imports MarkdownPreview until 11-09, so the standard production build tree-shakes the Markdown code out entirely. It has a single JS chunk without `ALLOWED_URI_REGEXP`, so the plan's `grep -l … /tmp/p11-07-dist/assets/*.js` can't find a lazy chunk.
- The entry-chunk check ran as written. The lazy-chunk check ran against a throwaway build with a second entry importing MarkdownPreview (`/tmp/p11-07-dist-probe`, nothing committed). There, `ALLOWED_URI_REGEXP` is only in `markdown-*.js` (79 KB), not in the entry or the MarkdownPreview chunk.
- **11-09 should re-run the standard-build check** once PreviewPanel imports MarkdownPreview.

**5. Hostile-test details (no product change)**
- The plan's "no `src=` / `url(` …" string checks run on the markup with text nodes removed. A dropped `<script>`'s body (for example `new Image().src='…'`) remains as escaped, inert paragraph text.
- GFM autolinks the `http://BEACON/...` URLs inside the hostile `javascript:` link texts. Those anchors are plain http links whose href equals their visible text, and the test allows exactly that.

**6. A render error and onFail**
- The onFail test covers a `blob.text()` rejection. Read, import and render share one `.catch`, so a sanitizer or parser throw takes the same path. There is no separate module-mock test.

## Known limits (not stubs)

- **Open residual DoS in marked (needs a decision).** marked 18.1.3's emphasis parsing time grows with the square of the number of unclosed delimiters in one paragraph: 10 KB takes 0.3 s, 20 KB 1.3 s, 40 KB 5 s and 80 KB 23 s, and a 2 MB paragraph didn't finish in minutes. The timings are the same with plain marked, and there is no upstream advisory. The parse is on the main thread, so the viewer window freezes. The options are listed in `deferred-items.md` and in the review's "Residual risk" section: render only the visible tab (11-09), a Worker with a timeout, a lower Markdown size cap, or markdown-it. This is availability only; no script runs and nothing is fetched.
- A placeholder for an image without alt text reads "Image: Image" to VoiceOver, per the UI-SPEC pattern `Image: <alt>`, where the alt text falls back to **Image**.

## Known Stubs

None.

## Self-Check: PASSED

- Files: markdown.ts, markdown.test.ts, hostile.md, MarkdownPreview.tsx, MarkdownPreview.test.tsx, deferred-items.md and the review's Dependencies section exist.
- Commits ac390542, 954ca500 and 8224ea92 are ancestors of HEAD.
