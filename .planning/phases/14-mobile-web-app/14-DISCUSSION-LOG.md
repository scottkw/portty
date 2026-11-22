# Phase 14: Mobile Web App - Discussion Log

> Audit trail only. Decisions live in 14-CONTEXT.md.

**Date:** 2026-11-22
**Mode:** smart discuss (autonomous)

## Area 1: Gate and input (hard to reverse)

| Question | Options | Choice |
|---|---|---|
| Which routes get the same-account check | ✅ Outermost on page, /app/, info, plugin-config (+stream), WS; probe/assets/dashboard open; mux-wide default-deny; data routes only | Accept all |
| Lone Esc held by the input filter | ✅ Pass a bare ESC that ends a frame; flush after ~50 ms; leave it | Accept all |

## Area 2: Home Screen (hard to reverse)

| Question | Options | Choice |
|---|---|---|
| How the token survives Add to Home Screen | ✅ Token stays in URL, static manifest without start_url, re-add after re-share; per-share manifest route; token-less icon via same-account fetch | Accept all |
| Manifest and CSP | ✅ manifest-src 'self' only; no manifest | Accept all |

## Preference questions

| Question | Options | Choice |
|---|---|---|
| Refusal page body | Heading only; requirement line; troubleshooting line | You decide → requirement line |
| Share copy | Password framing; shortest; plain statement | You decide → password framing |
| Keys | Plus shell staples; exactly SC1; plus Alt | You decide → exactly SC1 |
| Ctrl | Toggle; one-shot + lock; one-shot | You decide → one-shot |
| Bar placement | Always at bottom; with keyboard; always + keyboard key | You decide → always at bottom, docked above keyboard |
| Reflow | Fit button; first tap; as today | You decide → as today (INPT-06) |
| Font size | Memory only; per device; per device host wins | You decide → memory only |
| Chrome | No tab strip; same as web; hide when alone | You decide → hide when alone |
| Icon | Reuse Mac icon; iOS full-bleed | You decide → iOS full-bleed |
| Icon name | Session on host; Portty | You decide → Portty |
| Launcher | Copy + check field; static copy; copy + QR | You decide → static copy only |

## Decided autonomously

[auto] [Gate] — Q: "Fail-closed codes and SPA reaction" → Selected: "503 retry, marker-aware 403 handling, no live re-check" (reversible technical — decided autonomously: one client branch)
[auto] [Key bar] — Q: "How keys reach the shell and which devices" → Selected: "term.input path, cursor-key mode, coarse-pointer only" (reversible technical — decided autonomously: frontend only)
[auto] [Layout] — Q: "Cursor line above the keyboard" → Selected: "visualViewport-sized root, report then claim on keystroke" (reversible technical — decided autonomously: frontend only)
[auto] [Reconnect] — Q: "Reconnect after sleep" → Selected: "existing logic + pageshow + Reload buttons + M-145" (reversible technical — decided autonomously: client only)
