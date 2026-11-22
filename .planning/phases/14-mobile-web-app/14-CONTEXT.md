# Phase 14: Mobile Web App - Context

**Gathered:** 2026-11-22
**Status:** Ready for planning

<domain>
## Phase Boundary

The user can drive a shared session from an iPhone, through the host Mac's own browser share view (`/app/`) over the tailnet.

Built first (MOB-05): browser share links open only on devices signed in to the host's Tailscale account. Every other device gets "This link only works on your own devices" before any terminal data is sent.

Then:
- MOB-01: the share view works on a phone: width fit, keyboard inset, key bar, pinch-to-zoom.
- MOB-02: the share view installs to the Home Screen and opens full screen, with every asset served by the host.
- MOB-03: the view reconnects after the phone sleeps or changes network.
- MOB-04: this repo supplies the copy and link format for the static portty.app/app launcher, which holds no token, opens no WebSocket and has no shell access.

The desktop browser view is unchanged. Invited devices (pairing codes) stay deferred.

</domain>

<decisions>
## Implementation Decisions

### Same-account gate (MOB-05)
- `requireSameUser` runs outermost, before any capability work, on: `GET /sessions/{id}`, all of `GET /app/` (SPA, chunks, manifest, icons), `/api/sessions/{id}/info`, `/api/plugin-config`, `/api/plugin-config/stream`, and `/sessions/{id}/ws` (order there: origin → WhoIs → cap). The discovery probe `GET /api/sessions` is ALSO gated (user decision 2026-11-22, after the plan checker found it returns the cap's session name, status and hostname; the earlier "carries no session data" premise was wrong). Accepted cost: if the host's own tailscaled blips, peers can't list its sessions until the next refresh (5-30 s). Left open: `/assets/`, `/` and `/dashboard` (no session data). Per-route wrapping keeps a per-session hook for invited devices later. — **Reversibility:** one-way — auth model; tests, the Playwright fixture and CI need a same-user WhoIs fake (the fixture is not a test binary, server.go:178-181).
- Refusal page: a static embedded page in the link-invalid.html style (403 + X-Portty marker; names no account). Heading "This link only works on your own devices"; body "Open it on a device signed in to the same Tailscale account as the Mac that shared it." (Claude's discretion)
- Share copy that MOB-05 makes false (shareCopy.ts, SettingsSheet.tsx, cmd_share.go, README.md and their tests) becomes: "Treat this link like a password. It only opens on devices signed in to your Tailscale account." (Claude's discretion)

### Input: lone Esc
- Bug found: `InputAbsorber.Filter` holds a bare ESC until the next byte (oscabsorb.go:52-75; pinned by test P9), so the key bar's Esc and Claude Code's "Esc to interrupt" lag in every viewer, local tabs included. Fix: pass a bare ESC through when it ends a MsgInput frame. Research must confirm xterm never splits an auto-reply across onData calls; P9 is changed deliberately. Verify the current behaviour on a VM first (vim, a TUI). — **Reversibility:** costly — an input filter shared by every viewer and both pumps.

### Home Screen (MOB-02)
- The token stays where it is today, in `/app/?session=…&cap=…`. Ship a static manifest with no `start_url` (it defaults to the document URL) and scope `./`, so the icon opens the exact link. The icon stops working when sharing stops, the key is regenerated, or the daemon restarts (share state lives in memory); the user re-adds it. SC2's "Link No Longer Works" comes from the existing 403 on `/info`. — **Reversibility:** one-way — URL shape and auth.
- CSP: add only `manifest-src 'self'` to the share CSP (csp_mw.go), with security-review sign-off. The icon is already covered by `img-src 'self'`. Serve the manifest (`.json`, or an explicit content type) and icons from frontend/public with no-cache. `display: standalone`, `viewport-fit=cover`, safe-area insets. — **Reversibility:** costly — CSP change.

### Key bar (MOB-01, Claude's discretion)
- Keys: exactly the SC1 set: Esc, Tab, Ctrl, ←, ↑, ↓, →, `|`, in one row of ~44 pt keys.
- Ctrl is one-shot: tap highlights it, the next key (from the bar or the iOS keyboard) gets Ctrl, then it releases; tap again to cancel. Map a-z and A-Z to 0x01-0x1A (iOS auto-capitalises), `@ [ \ ] ^ _` and space to C0, and Ctrl+arrow to `CSI 1;5`.
- Placement: always shown at the bottom on touch-only devices, docked above the keyboard when it is up, so Esc or Ctrl-C is one tap with no keyboard (e.g. interrupting Claude Code).

### Phone layout (MOB-01, Claude's discretion)
- Reflow: keep INPT-06 as it is. Typing, pinch or rotation claims the grid; joining and just looking never reflow the office Mac. Until then the phone shows the owner's grid scaled, with the 0.7 floor and horizontal scroll.
- Pinch-to-zoom changes the terminal font, not the page. The size is kept in memory only: a relaunch returns to the host's font, and nothing is stored on the phone. One claim when the pinch ends (debounced).
- Chrome: hide the tab strip on phones while only the session tab exists; show it when a preview arrives. Tighter padding.

### Icon and launcher (MOB-02, MOB-04, Claude's discretion)
- Icon: a full-bleed iOS version derived once from build/appicon.png (gradient square with the glyph), committed as one source PNG and exported at 180 px (apple-touch-icon), 192 and 512 px (manifest).
- Home Screen name: "Portty".
- Launcher (portty.app/app): static copy only. It covers what Portty is, how to get the link from the Mac's Share button onto the phone (AirDrop, Universal Clipboard), the Add to Home Screen steps, and the link format `https://<mac>.<tailnet>.ts.net:7444/sessions/…`. No inputs, no token, no WebSocket. It lives as a markdown file in this repo for the website repo to use. The copy warns that sending the link to yourself in Messages stores it in iCloud.

### Decided autonomously
- [Gate] Q: "Fail-closed codes and SPA reaction" → Selected: "Keep 503 for a local tailscaled fault (clients retry; page routes get a static retry page); the SPA tells a WhoIs 403 (marker) apart from a cap 403 and shows the refusal text at bootstrap and in the reconnect probe; live sockets are not re-checked (the Phase 9 handoff reconnect re-runs the gate)" — one client header branch; no wire change.
- [Key bar] Q: "How keys reach the shell and which devices get the bar" → Selected: "term.input(seq, true) so bytes take the onData → sendInput path and count as typing; arrows follow applicationCursorKeysMode; preventDefault on pointerdown keeps the keyboard up; shown only when (pointer: coarse) and (hover: none)" — frontend only.
- [Layout] Q: "Keeping the cursor line above the keyboard" → Selected: "On touch devices, size the app root to visualViewport height and offset; ResizeObserver reports, and the next keystroke claims at the reported size" — frontend only, existing report/claim paths.
- [Reconnect] Q: "Reconnect after sleep and standalone recovery" → Selected: "No new mechanism; add pageshow as a wake trigger; unit-test an iOS-like suspend; give the stopped, ended, unreachable and ErrorBoundary states a Reload button on the phone; real-phone check recorded as an M-NN item (numbering left to the planner: M-145 is the Home Screen check)" — client only.

### Claude's Discretion
Refusal and share copy, key set, Ctrl behaviour, bar placement, reflow trigger, font persistence, phone chrome, icon art, icon name and launcher scope, as recorded above. The planner may adjust details within these choices.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- internal/webserver/whois.go: `requireSameUser` (:88-116, fails closed, 3 s timeout), `refuseOtherUser` (:72-75), `refuseLocalFault` (:80-82), `SetWhoIsClientForTest`, `noLiveWhoIs`; whois_test.go:25-60 has `fakeWhoIs`.
- internal/webserver/capability_mw.go:52-73: the static refusal-page pattern (`requireCapabilityPage`, embedded page); web/link-invalid.html, web/embed.go.
- internal/webserver/server.go routes: :541-723, the `/app/` SPA (:667-708), the `/sessions/{id}` redirect (:848-856), the share pump (:1023-1027).
- internal/relay/oscabsorb.go:52-75 (`Filter`), :209-234; relay/server.go:186 (local pump).
- Frontend: webMode.ts (session/cap from URL), App.tsx bootstrap :937-958, LinkNoLongerWorks, ErrorBoundary, WebShareSessionView (:136-150 probeShare, :196-210 font), TerminalPanel (fit/measure/claim :45-80, :267-337, onData :675, attachTouchScroll :805-811, resize :1132-1187), terminalScale.ts (0.7 floor), touchScrollHandler.ts (multi-touch released to the browser today), style.css (`touch-action: pan-y` :39).
- relayClient.ts reconnect: RECONNECT :164-176, listeners :300-303 (online, portty:wake, visibilitychange), wake :472-530, resume :555-612; closeReasons.ts `shareInfoVerdict` :31-36.

### Established Patterns
- Same-user check = numeric Tailscale user IDs; tagged or shared-in nodes are refused; 403 + marker for a refusal, 503 for a local fault.
- INPT-06: typing or a user-caused resize claims; join and reconnect never claim.
- Tests never touch the tailnet; new npm deps are pinned exactly (14-day rule); GOTOOLCHAIN=go1.26.11; each full suite runs once per plan.

### Integration Points
- frontend/index.html is shared by Wails and `/app/`: manifest and meta additions also land in the desktop app (check they are inert).
- cmd/playwright-fixture/main.go: needs a same-user WhoIs fake, and its `inputCaptureR` must be drained for key-bar e2e tests. playwright.config.ts has no iPhone project yet. e2e CI runs on ubuntu with no tailscaled.
- Copy to update: shareCopy.ts:6-9, SettingsSheet.tsx:394, cmd_share.go:91 (cmd_share_test.go:108), SharePopover.test.tsx:140, README.md:22.

</code_context>

<specifics>
## Specific Ideas

- The SC5 VM check needs a second Tailscale user. The rig has one account, so use a faked WhoIs behind a build tag that can never reach release builds (follow the check-brand.sh guard pattern), or ask the user for a second login (a new ephemeral key plus the user's Tailnet Lock signature).
- Playwright's WebKit iPhone emulation has no on-screen keyboard, no visualViewport shrink and no pinch: unit-test those and check them on the real phone.
- WhoIs on every `/app/` chunk means two LocalAPI calls per request; measure the cost. A host opening its own link needs WhoIs to resolve its own IP to itself; check in the VM.
- Research list: iOS Add to Home Screen launch URL with no start_url or no manifest; Home Screen storage separation; whether Safari applies the page CSP to manifest and icon fetches, and the manifest MIME type; WebSocket behaviour across suspend and Wi-Fi/cellular roaming with the Tailscale VPN; which of visibilitychange, online and pageshow fire in standalone mode; visualViewport, interactive-widget and safe areas; blocking page pinch on iOS; xterm 6's textarea on iOS (autocorrect, autocapitalize, smart punctuation); whether xterm splits auto-replies across onData calls.
- Phase 11's browser "Open PDF" opens the blob top-level, which may strand a standalone app: check it, and give it a way back.
- Typing on the phone narrows the office grid to about 43 columns, which can bring up the stray-prompt known issue (README.md:24-29).

</specifics>

<deferred>
## Deferred Ideas

- Invited devices / pairing code for a browser share (future milestone; builds on MOB-05).
- More key-bar keys (Shift-Tab, ~, /, -, Alt).
- Saving the pinch font size per device.
- A "Fit" control or first-tap claim on phones.
- A launcher "check my connection" field, or a QR code in the Share popover.
- Pre-filled per-session Home Screen names.
- An icon that survives re-sharing (token-less start_url plus same-account token fetch).

</deferred>
