---
phase: 11-file-preview
reviewed: 2026-11-21T00:00:00Z
depth: standard
files_reviewed: 60
files_reviewed_list:
  - app_cli_install.go
  - app_cli_install_test.go
  - cmd_cli.go
  - cmd_cli_test.go
  - cmd_open.go
  - cmd_open_test.go
  - dispatch_test.go
  - frontend/src/App.tsx
  - frontend/src/components/ImagePreview.tsx
  - frontend/src/components/MarkdownPreview.tsx
  - frontend/src/components/PdfPreview.tsx
  - frontend/src/components/PreviewPanel.tsx
  - frontend/src/components/TabBar.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/WebShareSessionView.tsx
  - frontend/src/components/__tests__/App.nav.test.tsx
  - frontend/src/components/__tests__/App.open-remote.test.tsx
  - frontend/src/components/__tests__/App.preview.test.tsx
  - frontend/src/components/__tests__/MarkdownPreview.test.tsx
  - frontend/src/components/__tests__/PreviewPanel.test.tsx
  - frontend/src/components/__tests__/PreviewRenderers.test.tsx
  - frontend/src/components/__tests__/TabBar.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.preview.test.tsx
  - frontend/src/components/__tests__/WebShareSessionView.test.tsx
  - frontend/src/lib/__tests__/markdown.test.ts
  - frontend/src/lib/__tests__/parseMarkdownInWorker.test.ts
  - frontend/src/lib/__tests__/previewAssembler.test.ts
  - frontend/src/lib/__tests__/previewStore.test.ts
  - frontend/src/lib/markdown.ts
  - frontend/src/lib/markdownSanitize.ts
  - frontend/src/lib/markdownWorker.ts
  - frontend/src/lib/parseMarkdownInWorker.ts
  - frontend/src/lib/previewAssembler.ts
  - frontend/src/lib/previewStore.ts
  - frontend/src/lib/relayClient.test.ts
  - frontend/src/lib/relayClient.ts
  - internal/daemon/api.go
  - internal/daemon/api_test.go
  - internal/daemon/client.go
  - internal/daemon/client_test.go
  - internal/daemon/clilink.go
  - internal/daemon/clilink_test.go
  - internal/daemon/engine.go
  - internal/daemon/preview.go
  - internal/daemon/preview_test.go
  - internal/daemon/types.go
  - internal/pty/native.go
  - internal/pty/native_test.go
  - internal/relay/hub.go
  - internal/relay/preview.go
  - internal/relay/preview_test.go
  - internal/relay/protocol.go
  - internal/webserver/csp_integration_test.go
  - internal/webserver/csp_mw.go
  - internal/webserver/csp_mw_test.go
  - internal/webserver/files_removed_test.go
  - internal/webserver/input_queue_test.go
  - main.go
  - menu_test.go
  - testmain_test.go
findings:
  critical: 1
  warning: 4
  info: 7
  total: 12
  necessity: 1
status: issues_found
---

# Phase 11: Code Review Report

**Reviewed:** 2026-11-21
**Depth:** standard (plus targeted cross-file tracing into hub.go, TerminalPanel.tsx, App.tsx tab paths, and the vendored Wails v2.10.2 darwin web view)
**Files Reviewed:** 60 (`git diff 23d2eb54..c23c5889 -- '*.go' '*.ts' '*.tsx' '*.js'`)
**Status:** issues_found

## Summary

The Go side holds up under tracing. Nothing in the preview path reads a file by path: `TestPreview_SourceGate` checks this, and so does a grep of every writer to `sub.Msgs`. The route exists only on the unix-socket mux. Every producer queues under `h.mu`, so the paced sender's window check cannot race. A full queue never goes to `sendLocked`, so the slow-viewer kick can't fire. Every way a target can end (left, revoked, hub ended, stalled, ok) ends its goroutine and clears `previewBusy`. Hidden web-session views report no bogus size, because `measureFit` returns null for a `display:none` parent. The CLI never starts the daemon. The CSP delta is exactly `img-src … blob:` plus `worker-src 'self'`, and `frame-ancestors 'none'` is unchanged. The osascript call passes both paths as argv items through `quoted form of`. `go vet ./...` is clean.

There is one serious gap: the **desktop PDF iframe**. Wails v2.10.2 sets a navigation delegate that has no navigation policy. It also exposes the `external` script-message handler to every frame and never checks which frame sent a message. So if a link inside an untrusted PDF navigates the preview iframe to an attacker page, that page can blindly call bound Go methods with no arguments, such as `main.App.QuitAll`, which ends every session. The security review's residual-risk entry (T-11-26) covers PDF *parsing* only, not this.

Other issues:
- The SVG sniffer refuses common real-world files, such as Adobe Illustrator exports. I confirmed this with `go run` on a copy of `hasSVGRoot`.
- The sanitizer has no hard guard against DOMPurify's silent no-op mode.
- Sanitizing and inserting large Markdown output still runs unbounded on the main thread.
- The `retryInit` path orphans preview-store entries.

**The three items flagged for the user:**
- **P-07 SPA 200:** not a finding. `GET /app/*` returns the SPA shell byte-for-byte. It carries no preview or file bytes, and `TestPreviewRoute_NotOnShareServer` pins the fallback.
- **SVG with a DOCTYPE internal subset:** a real correctness defect, and wider than the billion-laughs fixture. See WR-01, which includes a fix that still keeps entity-expansion files out.
- **macOS prompt copy:** see IN-04.

The necessity rubric (`necessity-review.md`) was read and applied. The duplicate validation layers (daemon, relay, frontend) sit on trust boundaries, so they are not necessity findings.

## Critical Issues

### CR-01: A link in an untrusted PDF can load an attacker page into the desktop preview iframe, and that page can call Wails-bound Go methods

**File:** `frontend/src/components/PdfPreview.tsx:35`, `frontend/index.html:3-6` (no CSP on the desktop page); upstream: `~/go/pkg/mod/github.com/wailsapp/wails/v2@v2.10.2/internal/frontend/desktop/darwin/WailsContext.m:249,281,479-494`

**Issue:** On the desktop, the PDF is `<iframe src=blob:…>` with no restrictions. The following points are proven from the source:
- Wails sets itself as the `WKNavigationDelegate` (`WailsContext.m:281`) but implements no `decidePolicyForNavigationAction`, so WebKit allows every navigation in every frame.
- Wails registers the `external` script-message handler (`:249`). Its `didReceiveScriptMessage` (`:479`) passes any message body to `processMessage` without checking `message.frameInfo`, so a cross-origin subframe can post binding calls.
- The desktop page has no CSP, so nothing restricts what the iframe may load.

One link in this chain is WebKit behaviour I could not verify without the app: that clicking a URI link in WKWebView's PDF view navigates the hosting frame (the iframe). Historically, PDFPlugin's `navigateToURL` loads into its own frame.

Failure scenario:
1. Someone runs `portty open` on a hostile PDF, for example a downloaded invoice, or a file a write-capable share viewer dropped on the host. Every viewer gets it, including the unattended office window as a background tab.
2. A user clicks an ordinary-looking link in it.
3. The iframe loads `https://attacker/…`, whose script runs `window.webkit.messageHandlers.external.postMessage('C'+JSON.stringify({name:'main.App.QuitAll',args:[],callbackID:'x'}))`.
4. `QuitAll` shuts down the daemon and terminates every session, on a Mac that nobody can reach to fix. Other no-argument bindings are also reachable blind, such as `RegenerateSigningKey`, which kills every share link, and `DownloadUpdate`. The page cannot read results, because callbacks run in the main frame, but it doesn't need to.

Tests and VM rows P-02/P-03 used hostile Markdown and SVG only. No row clicked a link in a PDF.

**Fix:** Pin what the iframe may load from the embedding page. WebKit enforces the parent's `frame-src` on every load into a child frame, including navigations the frame starts itself. A single directive has no wide blast radius: it sets no `default-src` and touches only frames.

```html
<!-- frontend/index.html, in <head>: only blob: documents (the PDF preview) may load in frames -->
<meta http-equiv="Content-Security-Policy" content="frame-src blob:; child-src blob:">
```

On the share page this meta policy combines with the header policy, which already blocks frames, so nothing changes there. Verify it in the VM:
1. Make a PDF with a URI link to the loopback beacon. `cupsfilter` output with a link added, or a tiny hand-written PDF with a `/URI` action, both work.
2. Open it in the desktop app and click the link.
3. Expect 0 beacon hits and the iframe still showing the PDF.

Record this in 11-SECURITY-REVIEW.md as an amendment to the "desktop has no CSP" observation. Separately, raise the frame-agnostic `external` handler upstream. Wails v2 could check `message.frameInfo.isMainFrame` and deny subframe navigations to non-`wails:`/`blob:` URLs.

## Warnings

### WR-01: The SVG sniffer refuses valid SVGs with a DOCTYPE internal subset (Illustrator exports) or a header over 1 KiB

**File:** `cmd_open.go:170-194`

**Issue:** For `<!DOCTYPE`, `hasSVGRoot` skips to the first `>`. That `>` closes the first `<!ENTITY …>` inside the internal subset, not the DOCTYPE. The loop then sees `<!ENTITY` and returns false. Separately, the 1 KiB window rejects any SVG whose root starts after a long licence or generator comment.

I checked both with `go run` on a verbatim copy of the function:
- An Adobe Illustrator "SVG 1.1" header (`<!DOCTYPE svg PUBLIC … [ <!ENTITY ns_extend "…"> <!ENTITY ns_ai "…"> ]>`) returns false.
- A root after a 1,100-byte comment returns false.
- A plain external DOCTYPE returns true.

So `portty open logo.svg` prints "can't preview this file type" for a very common class of real SVGs, not only the billion-laughs fixture. The refusal is safe, but it is wrong for the user.

**Fix:** Parse the internal subset. Refuse only what makes entity expansion dangerous: entity values that reference other entities, and parameter entities. Then widen the window. WebKit's libxml2 also has its own amplification limits, so this is defence in depth, not the only barrier.

```go
func hasSVGRoot(data []byte) bool {
	b := data[:min(len(data), 8<<10)] // licence/generator headers often exceed 1 KiB
	b = bytes.TrimPrefix(b, []byte("\xEF\xBB\xBF"))
	for {
		b = bytes.TrimLeft(b, " \t\r\n")
		var end string
		switch {
		case bytes.HasPrefix(b, []byte("<?")):
			end = "?>"
		case bytes.HasPrefix(b, []byte("<!--")):
			end = "-->"
		case bytes.HasPrefix(b, []byte("<!DOCTYPE")):
			gt, lb := bytes.IndexByte(b, '>'), bytes.IndexByte(b, '[')
			if lb >= 0 && (gt < 0 || lb < gt) { // internal subset
				rb := bytes.IndexByte(b[lb:], ']')
				if rb < 0 {
					return false
				}
				subset := b[lb+1 : lb+rb]
				// Nested or parameter entities are the billion-laughs shape: refuse those only.
				if bytes.IndexByte(subset, '&') >= 0 || bytes.Contains(subset, []byte("<!ENTITY %")) {
					return false
				}
				rest := bytes.TrimLeft(b[lb+rb+1:], " \t\r\n")
				if !bytes.HasPrefix(rest, []byte(">")) {
					return false
				}
				b = rest[1:]
				continue
			}
			end = ">"
		case bytes.HasPrefix(b, []byte("<svg")):
			return len(b) > 4 && bytes.IndexByte([]byte(" \t\r\n>/"), b[4]) >= 0
		default:
			return false
		}
		i := bytes.Index(b, []byte(end))
		if i < 0 {
			return false
		}
		b = b[i+len(end):]
	}
}
```

Add `cmd_open_test.go` cases for an Illustrator header (accepted), the committed `hostile-external.svg` (still refused, because its entity values contain `&lol…;`), and a 2 KiB leading comment (accepted).

### WR-02: Nothing in the code stops the sanitizer from silently passing HTML through, and the placement test doesn't guard the worker modules

**File:** `frontend/src/lib/markdownSanitize.ts:28-30`; `frontend/src/components/__tests__/MarkdownPreview.test.tsx:93-101`

**Issue:** `sanitizeMarkdown` calls `DOMPurify.sanitize` without checking `DOMPurify.isSupported`. In any context without a usable DOM, DOMPurify returns its input unchanged. Today it runs on the main thread, which is correct.

The invariant rests on a comment and on one source-text test, and that test only checks `MarkdownPreview.tsx` and `parseMarkdownInWorker.ts`. It does not check `markdownWorker.ts` or `markdown.ts`, the modules where moving the sanitizer "for performance" is the obvious refactor.

On the desktop page, which has no CSP, the sanitizer is the only barrier. An XSS there reaches every Wails binding, including `QuitAll`, `KillSession` and `SetSessionShared`.

**Fix:** Make a silent pass-through impossible, and pin the worker side in the test:

```ts
export function sanitizeMarkdown(html: string): string {
  // DOMPurify returns its input unchanged where it can't run (e.g. a worker): fail closed.
  if (!DOMPurify.isSupported || typeof document === 'undefined') throw new Error('sanitizer unavailable')
  return DOMPurify.sanitize(html, SANITIZE)
}
```

Then add test assertions that `markdownWorker.ts` and `markdown.ts` never import `dompurify` or `markdownSanitize`. A throw is safe here: the existing `.catch` in `MarkdownPreview` shows "Couldn't Show File".

### WR-03: Only the Markdown parse has a time limit; sanitizing and inserting up to 20 MB of output still blocks the main thread

**File:** `frontend/src/components/MarkdownPreview.tsx:83-85,168`; `frontend/src/lib/markdownSanitize.ts:5-6`

**Issue:** The 11-07 fix moved `marked` into a worker because "a frozen main thread freezes the whole window, terminal tabs included". The worker's output is then sanitized on the main thread with `DOMPurify.sanitize` (DOMParser plus a full tree walk) and assigned through `innerHTML`, which triggers style and layout. The comment "Sanitizing is linear in the input, so it can't freeze the window" mixes up linear with fast.

Failure scenario: a 15 MB GFM table (`|a|a|…` rows) parses within 3 s in the worker, because the table tokenizer is cheap. It produces millions of `<td>` nodes, which DOMPurify walks and WebKit then lays out, all on the main thread. The typing viewer (focused arrival) or anyone who clicks the tab freezes, terminal tabs included, for seconds or longer. The 3 s limit does not cover any of this.

**Fix:** Bound the work that runs on the main thread. For example, refuse formatted rendering past a size or node budget, and show the existing "Couldn't Show File" pane, or a plain-text `<pre>` fallback built with `textContent`:

```ts
const MARKDOWN_RENDER_MAX = 2_000_000 // bytes of source; beyond this, no formatted render
…
blob.text().then((src) => {
  if (src.length > MARKDOWN_RENDER_MAX) throw new Error('too large to format')
  return parseMarkdownInWorker(src, stop.signal)
})
```

Alternatively, have the worker return the HTML length or a tag count, and refuse past a cap before sanitizing. Then correct the comment in `markdownSanitize.ts`.

### WR-04: `retryInit` replaces all tabs but leaves PreviewStore entries orphaned (hidden blobs and object URLs that still count toward the caps)

**File:** `frontend/src/App.tsx:1075` (also `:413`); prune effect `App.tsx:664-670`

**Issue:** `retryInit` calls `setTabs(restoredTabs)`, which drops every preview tab. The reconcile effect at `:664` only removes *tabs* that lack a store entry, not *entries* that lack a tab. After a daemon error and Retry:
- up to 10 entries with up to 100 MiB of blobs and live `blob:` URLs stay in `PreviewStore` with no UI to close them;
- they keep counting toward `PREVIEW_MAX_TABS` and `PREVIEW_MAX_RETAINED`, so new previews evict each other early;
- in-flight entries keep their `inflight` record.

This breaks the "bounded memory, URLs revoked on close" guarantee.

**Fix:** Reconcile in both directions. Either call `previewStore.closeAll()` next to the `setTabs(restoredTabs)` in `retryInit` (previews are ephemeral by design), or add a store method that drops entries whose `tabId` is not in the current tab list, and call it from an effect on `tabs`.

## Info

### IN-01: Install Command-Line Tool… can run twice at once

**File:** `main.go:215-218`, `app_cli_install.go:77`

**Issue:** Each menu click starts a new goroutine. Choosing the item again while the first admin prompt is up gives two prompts. The second `ln -s` then fails with "File exists" and shows "Couldn't install the portty command" right after the success dialog.

**Fix:** Guard with an `atomic.Bool` (`if !a.cliInstalling.CompareAndSwap(false, true) { return }; defer a.cliInstalling.Store(false)`).

### IN-02: Stat-then-open race: a file swapped for a FIFO makes `portty open` hang

**File:** `cmd_open.go:49-66`

**Issue:** The regular-file check runs on `os.Stat(arg)`, but the file is then opened by path. If the path becomes a FIFO in between, `open(2)` blocks until a writer appears, and the CLI hangs. Same-user only, so not a security issue, but the code says "a FIFO … would block or never end".

**Fix:** Open first with `O_RDONLY|O_NONBLOCK`, then `f.Stat()` and check `Mode().IsRegular()` on the descriptor.

### IN-03: The per-user bin dir is prepended even when it is already on PATH

**File:** `internal/daemon/clilink.go:38`

**Issue:** A daemon started from inside a Portty session (CLI ensure or a Phase 9 handoff via `portty list`) inherits that session's PATH, which already starts with `<configDir>/bin`. New sessions then get the directory twice, and each daemon generation started this way adds one more copy. It is harmless but grows over time.

**Fix:** Drop existing occurrences of `binDir` from `os.Getenv("PATH")` before prepending.

### IN-04: macOS 27 doesn't show the `with prompt` text (user item)

**File:** `app_cli_install.go:31,55`

**Issue:** P-08c/P-08d recorded that macOS 27.2 shows only its own copy ("Allow administrator access for a script started by “Portty”? … Apple could not verify this script is free of malware"). The user gets no explanation from Portty of what the password is for, and Apple's malware wording may make a careful user refuse. `cliInstallPrompt` still helps on older macOS.

**Fix:** Optional: before `cliInstallRun`, show a short native confirm such as "Portty will ask for your password to link /usr/local/bin/portty to this app." with Continue and Cancel. Keep `with prompt` for older systems. Otherwise, accept it and note it in the UI-SPEC.

### IN-05: The manual-install hint double-quotes a path the user may paste into a shell

**File:** `app_cli_install.go:126`

**Issue:** `sudo ln -s "<exe>" /usr/local/bin/portty` is built by string concatenation. If the app path contains `"`, `$` or a backtick (the user can rename the folder), the suggested command is wrong. It could even expand something when pasted into a root shell.

**Fix:** Single-quote it, escaping embedded single quotes (`'` → `'\''`), or show the path on its own line without a ready-made command.

### IN-06: `portty open` drops daemon flags anywhere in its arguments and has no `--`

**File:** `cmd_open.go:33-41`

**Issue:** `slices.DeleteFunc` removes `--restart-daemon` and `--keep-daemon` in any position, so a file with one of those names can't be opened. A file named `-h` or `--help` shows usage instead, and `portty open -- -h` is a usage error.

**Fix:** Strip the flags only before the first non-flag argument, and accept `--` as the end of options.

### IN-07: Generality without instantiation: the Go preview frame parsers and `PreviewSend.Done` have no production caller

**File:** `internal/relay/preview.go:85-90,113-139`; `internal/daemon/preview.go:66-79`
**Category:** necessity

**Issue:** `ParsePreviewHeader`, `ParsePreviewChunk` and `ParsePreviewEnd` are exported, but only tests call them. No Go code reads preview frames: the peer proxy copies them verbatim and the frontend parses them. `handlePreview` also ignores `PreviewSend.Done`, which only tests wait on. This is judgment, not proven harm: they pin the wire format against the fixture, which has some value.

**Fix:** Optional: move the parsers into `preview_test.go` (test helpers), or leave them and note that they exist for the fixture test.

---

_Reviewed: 2026-11-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
