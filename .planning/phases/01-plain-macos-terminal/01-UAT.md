---
phase: 01-plain-macos-terminal
status: passed
date: 2026-11-15
build: wails build -platform darwin/arm64 -tags wailsassets
---

# Phase 1 UAT

| # | Check | Result |
|---|-------|--------|
| 1 | New Session spawns `/bin/zsh -l` (zsh 5.9, LOGIN), no badges (TERM-01) | pass |
| 2 | Tab closes immediately when its shell exits (TERM-05 / SC6) | pass |
| 3 | GUI force-killed (`kill -9`); daemon kept `shell 5`; relaunched GUI reattached with `persist-check-123` in scrollback (TERM-02 / M-56) | pass |
| 4 | Tailscale off → banner with exact NET-01 sentence, Share Session disabled, Settings shows Not connected; reconnect → banner clears, Share enabled, no restart (NET-01 / M-57 / M-40) | pass |
| 5 | Theme switch applies live; WebGL renderer off/on keeps rendering (TERM-04) | pass (theme library to be replaced by system light/dark, THEM-01 → Phase 2) |
| M-16 | Web-share warning re-armed → fires once with focus on "Keep sharing off", Esc keeps sharing off, enable works, next share does not warn | pass |
| M-24 | Web share (rw link) opened in dev-browser: scrollback intact; `@session echo hi` showed as plain chat, nothing typed into the PTY; desktop received it (unread badge 1). Desktop→browser leg not run | superseded — user decided 2026-11-15 to remove chat entirely (CHAT-01 → Phase 2) |

## Notes (not gaps)

- "Quit GUI Only" hides the window to the tray; the GUI process stays alive (by design, `app.go` QuitGUIOnly).
- The GUI process ignores SIGTERM; `kill -9` was used to simulate a GUI crash. Not a requirement.
- A web server that is already running keeps running when Tailscale drops; only Start is gated (Amendment 3 scope).
- First HTTPS request after the Tailscale reconnect timed out once (cold cert/handshake); the retry answered in 50 ms.
- M-41 (native notification) needs a signed build; deferred to Phase 6.
