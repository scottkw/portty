---
phase: 11-file-preview
fixed_at: 2026-11-21T00:00:00Z
review_path: .planning/phases/11-file-preview/11-REVIEW.md
iteration: 1
findings_in_scope: 12
fixed: 12
skipped: 0
status: all_fixed
---

# Phase 11: Code Review Fix Report

**Fixed at:** 2026-11-21
**Source review:** .planning/phases/11-file-preview/11-REVIEW.md (committed as `8f451a92`)
**Iteration:** 1
**Scope:** all 12 findings (CR, WR, IN, and IN-07 as a keep-with-note), by the orchestrator's request.

**Summary:**
- Findings in scope: 12
- Fixed: 11 code fixes, plus IN-07 kept with a note (its fix is optional and the code is used by tests)
- Skipped: 0
- Needs a VM check: CR-01 (required), IN-04 (the new confirm, quick look)

## Fixed Issues

### CR-01: A link in an untrusted PDF can load an attacker page into the desktop preview iframe

**Status:** fixed, VM check required (not run here)
**Files modified:** `frontend/index.html`, `frontend/src/components/PdfPreview.tsx`, `frontend/src/components/__tests__/PreviewRenderers.test.tsx`, `.planning/phases/11-file-preview/11-SECURITY-REVIEW.md`
**Commit:** `61890db1`
**Applied fix:** `index.html` now has a single meta CSP:

```
frame-src blob:; child-src blob:; worker-src 'self'
```

- There is no `default-src` or `script-src`. Wails' injected `/wails/runtime.js` and `/wails/ipc.js`, its IPC and every other load are untouched.
- I checked Wails v2.10.2's asset server. `processIndexHTML` parses `index.html` with `x/net/html`, inserts its two scripts at the top of `<head>` and re-renders. The meta survives.
- `worker-src 'self'` is required. Without it, the Markdown worker falls back to `child-src blob:` and is blocked. On the desktop the worker URL is `new URL('markdownWorker-*.js', import.meta.url)`, which resolves to `wails://wails/assets/markdownWorker-*.js`. That is same-origin, so `'self'` matches it as long as WebKit treats `wails://wails` as a tuple origin, which it does for a custom scheme with a host. The VM check confirms this.
- A grep of `frontend/src` finds no other iframe or worker.
- On the share page the meta combines with the header CSP. Both must allow a load, so the header's `default-src 'none'` still blocks frames. The share CSP tests check only the header, so they are unchanged.
- No `sandbox` on the iframe. It would null `contentDocument`, which breaks the non-PDF load check, and 11-SPIKE never proved it works with the native PDF view. A comment in `PdfPreview.tsx` says why.
- 11-SECURITY-REVIEW.md has a new "Amendment (11-REVIEW CR-01)" section covering: the desktop CSP now exists and is frames and workers only; PDF link navigation is blocked; Wails accepts script messages from any frame, to be raised upstream (`message.frameInfo.isMainFrame`); and why `sandbox` is not used. It also gets a residual-risk line.
- Test: `PreviewRenderers.test.tsx` pins exactly one CSP meta with exactly this content.

### WR-01: The SVG sniffer refuses valid SVGs with a DOCTYPE internal subset or a header over 1 KiB

**Status:** fixed
**Files modified:** `cmd_open.go`, `cmd_open_test.go`
**Commit:** `ab6867f2`
**Applied fix:** I used the review's function with one tightening. The internal subset is refused if it contains `&` or `%` anywhere, not only the literal `<!ENTITY %`, so `<!ENTITY\t%` with any whitespace is refused too. In an XML entity value, `%` is always a parameter-entity reference. The window is now 8 KiB. A `]` inside a quoted value ends the subset early, so such a file is refused; that is safe and marked with a `shortcut:` comment.

Tests:
- Illustrator "SVG 1.1" header: accepted.
- A root after a roughly 2 KiB licence comment: accepted.
- The committed billion-laughs `hostile-external.svg`: refused.
- A tab-separated parameter entity: refused.
- An unclosed subset: refused.
- A root past 8 KiB: refused.

### WR-02: Nothing stops the sanitizer from silently passing HTML through

**Status:** fixed
**Files modified:** `frontend/src/lib/markdownSanitize.ts`, `frontend/src/lib/__tests__/markdown.test.ts`, `frontend/src/components/__tests__/MarkdownPreview.test.tsx`
**Commit:** `bbe2ae3e`
**Applied fix:** `sanitizeMarkdown` throws `sanitizer unavailable` when `!DOMPurify.isSupported` or there is no `document`. `MarkdownPreview`'s existing `.catch` then shows "Couldn't Show File".

Tests:
- The placement test now also asserts that `markdownWorker.ts` and `markdown.ts` never import `dompurify` or `markdownSanitize`.
- A new test sets `DOMPurify.isSupported = false` and expects the throw.

### WR-03: Sanitizing and inserting large Markdown output still blocks the main thread

**Status:** fixed, requires human verification of the caps (a judgement on size, not a proven threshold)
**Files modified:** `frontend/src/lib/markdown.ts`, `frontend/src/lib/markdownSanitize.ts`, `frontend/src/lib/__tests__/markdown.test.ts`, `frontend/src/lib/__tests__/parseMarkdownInWorker.test.ts`, `.planning/phases/11-file-preview/11-SECURITY-REVIEW.md`
**Commit:** `192f9309`
**Applied fix:** `parseMarkdown` runs in the worker. It now throws `Markdown too large to format` when its HTML is longer than `MARKDOWN_MAX_HTML_LENGTH` (2,000,000 characters) or has more than `MARKDOWN_MAX_ELEMENTS` (50,000) elements.
- Elements are counted by finding each `<` not followed by `/`. marked escapes `<` in text, so this counts only real tags.
- The throw becomes the worker's error event, which leads to `onFail` and "Couldn't Show File".
- An oversize string is never cloned back to the main thread, sanitized, inserted or laid out.
- I corrected the "linear, so it can't freeze the window" comment.
- Two existing DoS-bound tests used inputs whose output is now over the cap. A 2 MB single line became 1.99 MB, and the indentation-nested list went from 2,000 levels to 1,000, which is still 50 times `MAX_DEPTH`. Each test keeps its purpose, and a new test pins the refusal.

New real-thread tests (bounded, each in a `worker_threads` thread the 3 s limit kills):
- A GFM table past the element cap: rejected in under 3 s, and the thread's error is the budget error, not a timeout.
- A code block past the length cap: rejected the same way.
- A table just under the cap: still renders.

The caps are my judgement of what WebKit sanitizes and lays out in well under a second. They are not measured in WebKit. A legitimate Markdown file over about 2 MB of HTML or 50,000 elements now shows "Couldn't Show File".

### WR-04: `retryInit` replaces all tabs but leaves PreviewStore entries orphaned

**Status:** fixed
**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.preview.test.tsx`
**Commits:** `09cc3425`, `3b2e2a5d` (puts back the empty deps array, see Verification)
**Applied fix:** `retryInit` calls `previewStore.closeAll()` just before `setTabs(restoredTabs)`. Its deps stay `[]`, because `previewStore` is a stable `useState` value. The mount-time `init` path at `App.tsx:413` is left alone, because no preview can exist before the first tabs do.

Test: a peer tab opens while Retry awaits and receives a preview, then Retry restores a local session. Checks:
- Only the session tab is left.
- The preview's URL is revoked.
- Ten new previews all get tabs, so no stale entry holds a slot.

With the fix commented out, the test fails.

### IN-01: Install Command-Line Tool… can run twice at once

**Status:** fixed
**Files modified:** `app.go`, `app_cli_install.go`, `app_cli_install_test.go`
**Commit:** `660b63ac`
**Applied fix:** an `atomic.Bool` on `App` (`cliInstalling`) with `CompareAndSwap(false, true)` at the top of `installCommandLineTool` and a deferred `Store(false)`.

Test: a second call while the first run's runner is blocked returns at once with no second runner call. After the first run finishes, the guard is free again.

### IN-02: A file swapped for a FIFO between stat and open makes `portty open` hang

**Status:** fixed
**Files modified:** `cmd_open.go`, `cmd_open_test.go`
**Commit:** `ffd83c02`
**Applied fix:** the file is opened with `O_RDONLY|O_NONBLOCK`, and the descriptor's `Stat()` must be a regular file, otherwise the unsupported-type line is shown. The first `os.Stat` and its exact error lines are kept. On a regular file `O_NONBLOCK` changes nothing, and Go does not use the poller for regular files on darwin.

Test: a FIFO is refused within 5 s. The race window itself has no seam, so it isn't tested directly.

### IN-03: The per-user bin dir is prepended even when it is already on PATH

**Status:** fixed
**Files modified:** `internal/daemon/clilink.go`, `internal/daemon/clilink_test.go`
**Commit:** `365d6c0e`
**Applied fix:** `cliPathEnv` builds PATH as `binDir` followed by every inherited entry except `binDir` (`filepath.SplitList`). Empty entries keep their positions. An empty PATH now gives `binDir` alone, not `binDir:`.

Test: `binDir:/usr/bin:binDir:/bin` becomes `binDir:/usr/bin:/bin`.

### IN-04: macOS 27 doesn't show the `with prompt` text

**Status:** fixed (UI copy, VM look recommended)
**Files modified:** `app_cli_install.go`, `app_cli_install_test.go`, `.planning/phases/11-file-preview/11-UI-SPEC.md`
**Commit:** `9da471cb`
**Applied fix:** just before the admin prompt, and only when a link is needed, Portty shows a native Question dialog through the existing `a.dialog` seam:
- Title: "Install Command-Line Tool"
- Message: "macOS will ask for your password to put the portty command in /usr/local/bin."
- Buttons: **Continue** (default) and **Cancel** (`CancelButton`, so Escape cancels too).

Anything but Continue, including no dialog at all, returns with no change and no further alert. `with prompt` stays for older macOS. The UI-SPEC has an amendment paragraph, a new row in the dialog table and a new row in the copy table.

Tests: the success path checks the confirm's exact copy and buttons, and a new test answers Cancel and checks that there is no runner call, no alert and no link.

### IN-05: The manual-install hint double-quotes a path the user may paste into a shell

**Status:** fixed
**Files modified:** `app_cli_install.go`, `app_cli_install_test.go`, `.planning/phases/11-file-preview/11-UI-SPEC.md`
**Commit:** `dd35aa56`
**Applied fix:** `shellQuote` wraps the path in single quotes and writes each `'` as `'\''`. The UI-SPEC failure row is amended to match.

Test: two paths, one with `'`, `"`, `$(…)`, backticks, `$HOME` and a backslash, each go through `/bin/sh -c 'printf %s <quoted>'` and come back byte for byte.

### IN-06: `portty open` drops daemon flags anywhere in its arguments and has no `--`

**Status:** fixed
**Files modified:** `cmd_open.go`, `cmd_open_test.go`
**Commit:** `e74759d5`
**Applied fix:** `--restart-daemon` and `--keep-daemon` are dropped only while they lead the arguments. `runCLI` and `flagFirstArgs` already put them right after `open`. Next comes the `-h`/`--help` check, then a leading `--` is consumed.

Tests:
- `-- -h`, `-- --help` and `--restart-daemon -- --keep-daemon` each send the named file.
- A daemon flag after the file is now a second argument, which gives the usage line. The old test that had a trailing flag now puts both flags first.

### IN-07: The Go preview frame parsers and `PreviewSend.Done` have no production caller

**Status:** kept, with a note (Chesterton's Fence)
**Files modified:** `internal/relay/preview.go` (comments only)
**Commit:** `2a8c5975`
**Applied fix:** none to behaviour. Both the relay and daemon preview tests decode frames with `ParsePreviewHeader`/`Chunk`/`End` against the shared wire fixture. The daemon tests can't use a `_test.go` helper from another package, so the parsers must stay exported in non-test code. Tests also wait on `PreviewSend.Done` to know a send has settled. Comments at both sites now say this.

## Verification

Every gate ran once, in this worktree (not the main checkout). Logs are in the session scratchpad, `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/`, with the prefix `p11rf-`.

| Gate | Result | Log |
|------|--------|-----|
| `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./...` | clean | `p11rf-go-build-vet.log` |
| `go test -race -short -count=1 ./...` | all 13 packages `ok`, 0 FAIL, no DATA RACE or panic | `p11rf-go-test.log` |
| `pnpm vitest run` | 102 of 104 files and 1514 of 1517 tests passed; 3 failures I introduced, then fixed (below) | `p11rf-vitest.log`, `p11rf-vitest-rerun.log` |
| `pnpm tsc --noEmit` | clean | `p11rf-tsc.log` |
| `pnpm exec vite build --outDir <scratchpad>/p11rf-dist` (not `frontend/dist`) | ok. The entry `index-*.js` has no marked or DOMPurify code; `markdownWorker-*.js` and `markdownSanitize-*.js` are separate chunks; the built `index.html` carries the CR-01 meta | `p11rf-build.log` |
| `bash tests/check-brand.sh` | `OK: brand gate clean` | `p11rf-check-brand.log` |
| `bash tests/check-traceability-paths.sh` | `OK: 188 traceability paths exist` | `p11rf-check-traceability.log` |

The 3 vitest failures were mine, not flakes. The source tests `App.shellExit.test.tsx` (2) and `App.tailscale.test.tsx` (1) cut the `retryInit` body at the `}, [])` marker, and WR-04's first commit had changed the deps to `[previewStore]`. Commit `3b2e2a5d` puts back `[]`, which is correct because `previewStore` is a stable `useState` value. I reran those two files plus `App.preview.test.tsx`: 44 of 44 passed. The full suite was not rerun.

Focused runs during the fixes (root package Go tests for WR-01 and IN-01, the touched vitest files) are logged under `p11rf-wr*-*.log`, `p11rf-in01-go.log` and `p11rf-cr01-vitest.log`.

TESTING.md: no test file was added, renamed or removed. Every new test is in an existing file, so Sections 2 and 4 are unchanged, and `check-traceability-paths.sh` passes.

## VM checks needed

1. **CR-01 (required).** On the office VM's desktop app:
   - Make a PDF with a `/URI` link to the loopback beacon (`http://127.0.0.1:8765/cr01`), run `portty open` on it, and click the link in the preview.
   - Expect 0 beacon lines, the iframe still showing the PDF, and the app still running.
   - Then `portty open` a normal `.md` file. It must render, which proves `worker-src 'self'` matches the `wails://` origin.
   - Also confirm that a normal PDF still renders and that the share viewer's Markdown and PDF still work.
2. **IN-04 (look).** On the office VM, choose Install Command-Line Tool… with no `/usr/local/bin/portty` present.
   - The confirm appears with the copy above.
   - Cancel changes nothing.
   - Continue leads to the macOS admin prompt.

---

_Fixed: 2026-11-21_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
