---
status: complete
phase: 04-one-click-share
source: [04-VERIFICATION.md]
started: 2026-11-17T07:00:00Z
updated: 2026-11-17T20:40:03Z
---

## Current Test

[testing complete]

## Tests

### 1. Two-Mac share flow (M-80)
expected: Share on the office Mac shows the session in the home Mac's sidebar; one click opens it with no code; the browser link works from another tailnet device; from Warp, `portty attach <host>:<session>` shows the status bar, and bare `portty attach <host>` attaches (one share) or lists (several); Share off shows the stop banner in the GUI tab and browser, removes the sidebar row, and the CLI prints "Portty: <host> stopped sharing <session>." and exits 0; re-share then clicking the row reconnects to the right Mac (WR-05/WR-06); a Mac on another account sees nothing. Check `tailscale status` shows no tags on either Mac.
result: pass (step 6, different-account Mac, skipped: no second account available)
notes: Office = kens-macbook-air-1574, home = kens-personal-macbook-air, build 9af051d3 (ad-hoc signed, arm64). First Share-on took up to ~20 s (first HTTPS cert issue, warmCert); re-share was instant. Bidirectional typing confirmed across home GUI, office GUI, browser and CLI. No tags on either Mac. See Gaps for 3 issues found during this test.

### 2. No lingering public serve config (M-77)
expected: `tailscale serve status` prints "No serve config" on both Macs (run `tailscale serve reset` if anything is listed). Closes T-04-01.
result: pass

### 3. Share controls follow real Tailscale (M-82)
expected: With real Tailscale disconnected, Share is dimmed with its reason and Stop Sharing still works; reconnecting re-enables it.
result: pass
notes: Toolbar Share button is disabled with Tailscale off, so the popover switch is unreachable; Stop Sharing worked from the sidebar right-click menu. Share state (sidebar icon) is kept while disconnected, per 04-UI-SPEC.md:270.

### 4. Live repro of gap 1 with diagnostic log (M-83)
expected: On a shared session with more than 32 KiB of history, after `tailscale ping` from the home Mac, attach and detach `portty attach <office>:<session>` three times beside the home GUI tab and a browser. No viewer blanks. If one does, collect the `viewer-diag:` lines from `~/Library/Application Support/portty/daemon.log` on both Macs.
result: [pending]

### 5. daemon.log after a real daemon restart (WR-01 fix)
expected: After restarting the daemon, `daemon.log` is mode 0600 and starts at the `daemon: listening on` line; the previous run is in `daemon.log.1`.
result: [pending]

### 6. Browser banner uses the tailnet name (M-80/M-79, gap 2)
expected: Turning Share off shows the browser banner with the tailnet short name (same as GUI and CLI), never `.local`.
result: [pending]

### 7. Long-session attach (M-80, gap 4)
expected: After `seq 1 20000` in the shared session, both the home GUI tab and `portty attach` show history ending in 20000 and keep receiving live output.
result: [pending]

### 8. Tailscale-off switch-off from the popover (M-82, gap 3)
expected: With real Tailscale off, a shared session's Shared button opens the popover and its switch turns Share off; the button then closes the popover once and goes dim. An unshared session's button stays dim.
result: [pending]

### 9. Different-account Mac sees nothing (M-80 step 6)
expected: A Mac on another Tailscale account shows no group for the office Mac, and `portty attach` there prints the access-refused line.
result: [pending]

## Summary

total: 9
passed: 3
issues: 4
pending: 6
skipped: 0
blocked: 0

## Gaps

- truth: "Attaching from the CLI does not disturb other viewers of the same shared session"
  status: failed
  reason: "User reported: first `portty attach kens-macbook-air-1574:\"shell 1\"` blanked the home GUI tab and the browser (lone `-` top-left) and the CLI status bar showed `4 viewers` / `[reconnecting]`. Office Mac's own window kept working. Detaching the CLI (Ctrl-\\) restored the GUI tab and browser within ~10 s. Second attach and a 3x attach/detach loop did not reproduce."
  severity: major
  test: 1
  artifacts: []
  missing: []

- truth: "Browser stop banner names the host the same way as the GUI and CLI (tailnet name)"
  status: failed
  reason: "User reported: browser banner read `Kens-MacBook-Air.local stopped sharing this session.` while GUI and CLI used `kens-macbook-air-1574`. Browser gets the OS hostname via the webserver session resolver."
  severity: cosmetic
  test: 1
  artifacts: [frontend/src/components/SessionEndedBanner.tsx, internal/daemon/share.go]
  missing: []

- truth: "With Tailscale off, turning Share off works from the popover switch (04-UI-SPEC.md:346)"
  status: failed
  reason: "User reported: toolbar Share button is disabled with Tailscale off, so the popover cannot be opened; only the sidebar right-click Stop Sharing works. Matches 04-UI-SPEC.md:139 but contradicts :270/:346. Spec needs one answer."
  severity: minor
  test: 3
  artifacts: [.planning/phases/04-one-click-share/04-UI-SPEC.md]
  missing: []

Observation (not a gap): browser tab label reads "Session" rather than the session name.

- truth: "A viewer attaching through another Mac's daemon proxy (GUI tab or `portty attach`) joins a session whose catch-up history is larger than 32 KiB"
  status: failed
  reason: "Found by 04-07's harness (not seen live): no Go WebSocket reader in the attach path calls SetReadLimit, so coder/websocket's 32768-byte default applies; the host sends the scrollback ring (up to 256 KiB) as one preamble message, and the viewing daemon's proxy (remote_ws_proxy.go copyWS upstream.Read) and the CLI (attach.WsOutputPump) drop the joiner after the grid frame. User decision 2026-11-17: fix in gap plan 04-10 before the next two-Mac UAT. See deferred-items.md."
  severity: major
  test: 1
  artifacts: [internal/daemon/remote_ws_proxy.go, internal/attach]
  missing: []
