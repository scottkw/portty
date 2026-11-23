// Package webserver CSP header middleware (Phase 89, SEC-08).
//
// cspHeaders implements the Content-Security-Policy response header setter
// for all HTML-serving routes (terminal.html, dashboard.html).
// It is wired by Plan 04 — this file only defines the method.
//
// Policy specification (D-09, amended 2026-04-22 after Phase 89 e2e finding):
//
//	default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline';
//	connect-src 'self' wss://<host>; img-src 'self' data:;
//	font-src 'self'; base-uri 'none'; form-action 'self';
//	frame-ancestors 'none'
//
// Amendment rationale: xterm.js injects <style> elements at runtime (cursor,
// selection, theme hooks) via document.createElement('style'). CSP3 classifies
// these as style-src-elem, which style-src 'self' blocks. Chromium e2e test
// TestBrowserCSP_TerminalNoViolations surfaced 12 violations on /sessions/{id}.
// User disposition: allow 'unsafe-inline' for style-src only. script-src
// remains strict ('self') — the Finding 4 CDN class of attack is unchanged.
//
// Amendment 2 (Phase 96 IMG-03, 2026-05-07 after pre-phase CSP audit):
//
//	default-src 'none'; script-src 'self' 'wasm-unsafe-eval';
//	style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>;
//	img-src 'self' data:; font-src 'self'; base-uri 'none';
//	form-action 'self'; frame-ancestors 'none'
//
// Amendment rationale: @xterm/addon-image embeds a SIXEL decoder and
// an iTerm2 IIP base64 decoder as base64-encoded WebAssembly bytecode
// (~10 KB raw), instantiated synchronously on the main thread via
// WebAssembly.instantiate / new WebAssembly.Module / new WebAssembly.
// Instance. The pre-phase audit confirmed (see RESEARCH §"Mandatory Pre-Phase CSP Audit Finding 2",
// documented in .planning/phases/96-image-addon-csp-audit/96-RESEARCH.md):
// 0 occurrences of `new Worker(`, 0 occurrences of `eval(`, and
// 6 cumulative occurrences of WASM bootstrap APIs across the addon's
// .js + .mjs bundles. CSP3 §6.3 requires 'wasm-unsafe-eval' to permit
// these instantiations under any non-default script-src directive.
//
// Defense-in-depth distinction: 'wasm-unsafe-eval' is NARROW — it
// permits only WebAssembly compilation/instantiation. It does NOT
// enable JS `eval()`, `new Function()`, or `setTimeout(string)` —
// those still require the broader 'unsafe-eval' source expression,
// which this codebase explicitly forbids via the token-aware
// TestCSPHeaders_NoUnsafeEvalToken_TokenAware regression guard
// (csp_mw_test.go). The two source expressions share a substring but
// are distinct CSP3 tokens; the regression guard tokenizes on
// whitespace within the script-src clause to distinguish them.
//
// Browser support (CSP3 'wasm-unsafe-eval' source expression):
//
//	Chrome (Chromium):  102+   (May 2022)
//	Firefox:            102+   (June 2022)
//	Safari (WebKit):    16.0+  (September 2022)
//	iPad Safari:        16.0+  (September 2022)
//
// All four browsers in Phase 99's supported matrix support
// 'wasm-unsafe-eval' with multi-year headroom; no fallback strategy
// required.
//
// Amendment 3 (Phase 11, PREV-04, 2026-11-21):
//
//	default-src 'none'; script-src 'self' 'wasm-unsafe-eval';
//	style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>;
//	img-src 'self' data: blob:; worker-src 'self'; font-src 'self';
//	base-uri 'none'; form-action 'self'; frame-ancestors 'none'
//
// Amendment rationale: file previews (`portty open`) render images and SVG
// as <img src=blob:...> built from bytes that arrived over the session's
// own socket, so img-src gains blob: and nothing else. An SVG inside an
// <img> runs no script and fetches nothing, and img-src still names no
// network origin. blob: is never added to default-src, script-src or
// connect-src (TestCSPHeaders_PreviewDelta). The review is
// .planning/phases/11-file-preview/11-SECURITY-REVIEW.md.
//
// No frame-src and no object-src: 11-SPIKE.md's "Share CSP delta" line is
// img-src 'self' data: blob: only. Its Safari rows found that no candidate
// directive shows a framed blob: PDF on this page while frame-ancestors
// 'none' stays (the blob document inherits it, and Safari's PDF plugin loads
// under connect-src), so browser viewers get an Open PDF button that opens
// the PDF blob top-level instead, which works under this policy. The desktop
// app frames the PDF in an iframe; its page has no CSP.
//
// Amendment 3 addition (Phase 11, 11-07 fix, 2026-11-21): worker-src 'self',
// inserted after img-src. The Markdown preview parses in a dedicated Web
// Worker that is terminated after 3 s, because marked can take minutes on
// hostile emphasis and the parse would otherwise freeze the viewer's window.
// Vite emits the worker as a hashed same-origin file under /app/assets/, so
// 'self' is enough; blob: and data: workers stay blocked. Without the
// directive, workers fall back to script-src 'self' (the same sources); naming
// it pins the intent. No child-src, frame-src or object-src.
//
// Amendment 4 (Phase 14, MOB-02, 2026-11-22):
//
//	default-src 'none'; script-src 'self' 'wasm-unsafe-eval';
//	style-src 'self' 'unsafe-inline'; connect-src 'self' wss://<host>;
//	img-src 'self' data: blob:; worker-src 'self'; manifest-src 'self';
//	font-src 'self'; base-uri 'none'; form-action 'self';
//	frame-ancestors 'none'
//
// Amendment rationale: Add to Home Screen reads /app/manifest.json, and
// Chromium blocks the manifest fetch under default-src 'none' (reproduced in
// 14-RESEARCH.md Q2); WebKit also checks manifests against the page CSP. The
// delta is manifest-src 'self' and nothing else: the manifest is a static
// same-origin file with no start_url and no id (WebKit rejects a cross-origin
// start_url or scope), so the launch URL stays the exact share link. The
// manifest icons load under the existing img-src 'self'. default-src stays
// 'none' and 'unsafe-eval' stays absent (TestCSPHeaders_ManifestDelta). The
// sign-off is carried by .planning/phases/14-mobile-web-app/14-SECURITY-REVIEW.md.
//
// Per-request composition (D-10): the wss:// origin is derived from
// ws.BaseURL() on every request so it tracks the listener address
// automatically — including the random-port fallback that Phase 87/88
// already use for the same reason.
//
// No CSP violation reporting (D-11): report-uri and report-to directives are
// intentionally absent. The policy is enforced silently; violations surface
// in browser DevTools only. No report-uri endpoint is exposed.
//
// Middleware shape (D-13): func(http.HandlerFunc) http.HandlerFunc, matching
// requireAllowedOrigin in origin_mw.go. Unlike the gate-style middlewares,
// cspHeaders ALWAYS delegates to next when BaseURL is non-empty — it sets
// response headers and passes control through.
//
// Fail-closed on empty BaseURL: if ws.BaseURL() returns "" (listener not
// yet ready — theoretically unreachable because handlers don't run until
// Start() succeeds), the middleware responds HTTP 500 rather than silently
// passing without a CSP header (CLAUDE.md "Silent Fallbacks Forbidden").
//
// Cache-Control: no-store is set alongside the CSP header (Research
// Pitfall 3 guidance) so HTML pages are refetched on every request. This
// ensures CSP changes propagate immediately and aligns HTML caching with
// the /assets/ no-store policy added in Plan 04.
package webserver

import (
	"net/http"
	"strings"
)

// cspHeaders sets the Content-Security-Policy, Cache-Control: no-store and
// Referrer-Policy: no-referrer response headers, then delegates to next. When ws.BaseURL() returns ""
// (listener not ready), it responds HTTP 500 and does NOT delegate.
func (ws *WebServer) cspHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := ws.BaseURL()
		if base == "" {
			// Fail-closed: listener not ready. Theoretically unreachable
			// because handlers don't run until Start() succeeds, but
			// documented defensively (CLAUDE.md Silent Fallbacks Forbidden).
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		wssOrigin := "wss://" + strings.TrimPrefix(base, "https://")
		var b strings.Builder
		b.Grow(256)
		b.WriteString("default-src 'none'; ")
		// Phase 96 IMG-03 Amendment 2: 'wasm-unsafe-eval' permits the
		// @xterm/addon-image SIXEL/IIP WASM decoder to instantiate
		// (CSP3 §6.3 governs WebAssembly.compile/instantiate/Module/Instance).
		// 'wasm-unsafe-eval' is NARROW — it does NOT enable JS eval() or
		// new Function() (those still require the broader 'unsafe-eval', which
		// remains forbidden by csp_mw_test.go). See package comment Amendment 2.
		b.WriteString("script-src 'self' 'wasm-unsafe-eval'; ")
		b.WriteString("style-src 'self' 'unsafe-inline'; ")
		b.WriteString("connect-src 'self' ")
		b.WriteString(wssOrigin)
		b.WriteString("; ")
		// Phase 11 Amendment 3: blob: for the image and SVG previews.
		b.WriteString("img-src 'self' data: blob:; ")
		// Phase 11 Amendment 3 (11-07 fix): the Markdown parse worker, same origin only.
		b.WriteString("worker-src 'self'; ")
		// Phase 14 Amendment 4 (MOB-02): the static Home Screen manifest, same origin only.
		b.WriteString("manifest-src 'self'; ")
		b.WriteString("font-src 'self'; ")
		b.WriteString("base-uri 'none'; ")
		b.WriteString("form-action 'self'; ")
		b.WriteString("frame-ancestors 'none'")
		w.Header().Set("Content-Security-Policy", b.String())
		w.Header().Set("Cache-Control", "no-store")
		// The /app/ URL carries a full-control token; never leak it via Referer.
		w.Header().Set("Referrer-Policy", "no-referrer")
		next(w, r)
	}
}
