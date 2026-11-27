---
phase: 22-click-to-share-for-tailnet-people
plan: 10
subsystem: frontend
tags: [react, wails, click-to-share, share-sheet, settings, security]
requires:
  - phase: 22-07
    provides: lib/shareDesktopCopy.ts strings
  - phase: 22-09
    provides: StartDesktopShare, GetDesktopShare, StopDesktopShare, SetDesktopViewerRights, DisconnectDesktopViewer bindings and desktopshare:* events
provides:
  - useDesktopShare store (state, lastEvent, sheetOpen, openSheet, closeSheet, stoppedNotice)
  - ShareDesktopSheet (Not sharing, Starting, Sharing), ViewerList, SettingsRow (Row moved out of SettingsSheet)
  - Settings Row 6 (Share Desktop / Manage Sharing) and the new Session keys sentence
  - .share-desktop*, .viewer-row*, .viewer-badge and .admit* CSS (the .admit classes are for 22-11)
  - AR-21-01 own-account peer filter in tailnet discovery
affects: [22-11, 22-12, 22-13]
tech-stack:
  added: []
  patterns: [link kept in a ref and never rendered, requested grant held only while the daemon still reports the pre-call value]
key-files:
  created:
    - frontend/src/lib/desktopShare.ts
    - frontend/src/components/ShareDesktopSheet.tsx
    - frontend/src/components/ViewerList.tsx
    - frontend/src/components/SettingsRow.tsx
    - frontend/src/components/__tests__/ShareDesktopSheet.test.tsx
    - frontend/src/components/__tests__/ViewerList.test.tsx
    - frontend/src/components/__tests__/SettingsSheet.share.test.tsx
  modified:
    - internal/tailnet/tailnet.go
    - internal/tailnet/tailnet_test.go
    - internal/daemon/peer_desktop_test.go
    - internal/daemon/peer_connect_test.go
    - frontend/src/App.tsx
    - frontend/src/components/SettingsSheet.tsx
    - frontend/src/components/Switch.tsx
    - frontend/src/components/__tests__/SettingsSheet.test.tsx
    - frontend/src/lib/shareDesktopCopy.ts
    - frontend/src/lib/tailscale.ts
    - frontend/src/style.css
key-decisions:
  - "App owns useDesktopShare and hands Settings a small share prop (active, viewers, onOpen); the sheet gets state and stoppedNotice as props, so the store is not a module singleton and tests inject the subscribe function"
  - "The sheet reads GetDesktopShare on open first: a running share shows Sharing with its link with no new start (works with Tailscale off); otherwise HasCTDisclosure plus a connected Tailscale starts at once"
  - "A requested grant is held only while the daemon still reports the value it had at call time, so the switch never flickers back and a missed state line cannot leave a wrong switch"
  - "The Switch gained aria-labelledby so a grant switch is named by its label plus the person"
  - "USERSPACE_SENTENCE moved to lib/tailscale.ts so the sheet and Settings share one copy"
status: complete
commits: 4
plan_head_before: 12cd903a29cf394ec04a57554df507d41d4f9986
plan_head_after: c8fefe80b8c47c3a0f551880cc0f00566a362006
actuals:
  tokens: 60000
  tasks: 3
  commits: 4
---

# Phase 22 Plan 10: Share Desktop sheet, Watching list and Settings row Summary

One click from the tray, a notification or Settings now starts a share and gives the sharer a link to copy; the Watching list names every viewer by login name with per-person Allow control, Allow clipboard and Disconnect; and, carried in, Portty now only discovers and connects to peers on the viewer's own Tailscale account (AR-21-01).

## Precondition record

Gate passed: 20-10, 21-08, 21-10, 22-07 and 22-09 SUMMARY files all exist. `git log --oneline 07ad7d21..HEAD -- frontend/src/App.tsx frontend/src/style.css frontend/src/components/SettingsSheet.tsx` listed these, all read before editing (the plan's assumptions on SettingsSheet's Desktop group, App mounting and style.css held):

- 307277be fix(21): WR-05 clear a Desktop tab's sidebar line and label on close
- bb55eae6 feat(21-10): phone bottom bar, key bar, soft keyboard, armed modifiers and viewport inset
- 2e9f88c4 feat(21-08): toolbar label, control seams for control mode, and the Desktop tab styles
- 3c22c9f7 feat(21-08): app states, Local Network and Refused cards, hidden tab and window, wake, sidebar feed
- 0b4978a1 feat(21-08): a Desktop row click opens a tab that streams through the loopback options
- 0a95e689 feat(20-10): Settings Showing row, Row 4 control state, consent copy that says control
- db356d0b feat(20-08): state chips wrap at 480 px and the notice clears a wrapped state chip
- 9d0bff43 feat(20-08): Paste to Mac and the Copy fallback and Paste popovers
- e31f3825 feat(20-07): bar reveal and density, availability table, Swap, View Only/Control, Full Screen, Lock Screen confirm

## AR-21-01

**What changed.** `discoverPeers` in `internal/tailnet/tailnet.go` keeps only online peers whose `UserID` equals `status.Self.UserID`, and returns an empty (non-nil) list when `Self` is nil or its `UserID` is zero (fail closed). `DiscoverPeers` and `DiscoverAndProbe` both go through it, so the terminal peer list and the Desktop rows are covered by the one filter, which is the small-change case CONTEXT allowed. I found no caller that needs other-account peers: `status.Peer` is read in exactly one place (`tailnet.go`); the other users of `tailnet.Peer` (`cmd_cli.go`, `daemon/peer_client.go`, `client.go`, `tailnet_cache.go`) only consume discovery output.

**Connect paths.** `connectPeerDesktop` (`peer_desktop.go`) and `handlePeerConnect` (`peer_connect.go`) already call `a.discoverPeers` (the filtered `tailnet.DiscoverPeers`) and match the requested host with `matchPeer` against that result; a name from the UI that is not listed gets `404 unknown_host` before any network call or cap fetch, and the probed-host list they return also comes from filtered discovery. No production change was needed there; I added tests pinning it.

**Commit.** 130f64ea `fix(22-10): AR-21-01 offer only own-account Tailscale peers`. Tests were written first and seen failing (`TestDiscoverPeers_OwnAccountOnly` and `_FailsClosedWithoutSelfIdentity`, nil Self case) before the filter was added.

**Tests** (fake `statusFunc`, `go test ./internal/tailnet ./internal/daemon ./internal/webserver -short` green):
- `internal/tailnet/tailnet_test.go`: `TestDiscoverPeers_OwnAccountOnly` (other-account online peer excluded, own-account included), `TestDiscoverPeers_FailsClosedWithoutSelfIdentity` (nil Self and zero Self.UserID both return an empty non-nil list); the existing discovery tests now carry a `Self` and own `UserID`.
- `internal/daemon/peer_desktop_test.go`: `TestPeerDesktop_RefusesHostMissingFromDiscovery` (a host discovery does not list, by FQDN and short name, gets 404 `unknown_host`, the peer's cap route is hit zero times, no cap stored).
- `internal/daemon/peer_connect_test.go`: `TestPeerConnect_RefusesHostMissingFromDiscovery` (404 `unknown_host`, no cap requested).

**Deferred real-tailnet UAT item.** On a real tailnet with a node shared in from another Tailscale account online: it must not appear as a Desktop row in the Portty app sidebar nor as a terminal peer, `portty list` must not show it, and a hand-typed `connect` to its host must be refused as unknown host. Confirm an own-account Mac still appears. Not run here: no host tailscaled or 100.x contact, by constraint.

## What was built

**Task 1 (tracer, b40367f3).** `lib/desktopShare.ts` subscribes to `desktopshare:state`, `:event`, `:stopped` and `:open-sheet` (injectable `subscribe`, no-op in web mode) and exports the viewer types plus `sanitizeName` and `cleanDetail`. `ShareDesktopSheet` is `.dialog.dialog--setup.dialog--share`: on open it reads the CT acknowledgement and any running share; a running share shows Sharing with the same link, otherwise an acknowledged, connected Tailscale starts at once (Getting a link…). First run waits in Not sharing with the CT line and Start Sharing records the acknowledgement first. Sharing shows the Link row with Copy Link (`ClipboardSetText`, Copied! for 2 s, the link lives in a ref and is never rendered), the footnote, Stop Sharing (left, acts at once, drops to Not sharing without restarting) and Done (primary). App mounts the sheet after SettingsSheet. `Row` moved to `SettingsRow.tsx` in this commit (the Link row needs it), earlier than the plan's Task 2.

**Task 2 (f10975a4).** `ViewerList`: oldest connection first (since, then id), names sanitized again and rendered in `bdi` (login wraps, device may ellipsize with `title`), the display-name-equals-login rule, `View only` / `Controlling` (follows the grant) / `Own account` badges with `aria-hidden` icons, independent Allow control and Allow clipboard switches that call `SetDesktopViewerRights(id, control, clipboard)` at once with a saving state, Disconnect with `aria-label="Disconnect <name>"`, the grant caption (name in `bdi`), a per-row `role="alert"` error, a polite live group and a visually hidden `role="status"` line that announces `<name> joined` and `<name> left`. `Switch` gained `aria-labelledby`.

**Task 3 (c8fefe80).** Settings Row 6 (last row of the Desktop group, always visible): Share Desktop… with the CT line until acknowledged, `aria-disabled` and described by the Tailscale reason when not connected; Shared with nobody yet / 1 person / N people plus Manage Sharing… while sharing (opening never blocked then). The Security pane Session keys sentence is the new one. The sheet gained the start-error table (tailscale_off and userspace block Start Sharing with their sentence, https_off and other codes are an inline `role="alert"`), the Status row for Needs permission (with Set Up Permissions…, opening the Phase 19 setup sheet after this one), Waiting for approval, Display asleep, Paused and Waiting for Tailscale, the `Sharing stopped…` notice (`role="status"`, no auto-start), and a quiet drop to Not sharing when the share ends elsewhere (the tray).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Row moved to SettingsRow.tsx in Task 1, not Task 2**
- **Found during:** Task 1
- **Issue:** the sheet's Link row needs `Row`, which was private to SettingsSheet.tsx.
- **Fix:** moved it (markup unchanged) with the tracer commit; Task 2's acceptance greps hold at HEAD.
- **Commit:** b40367f3

**2. [Rule 3 - Blocking] USERSPACE_SENTENCE moved to lib/tailscale.ts**
- **Found during:** Task 1
- **Issue:** the sheet and Settings both need the sentence; it was a private const in SettingsSheet.tsx.
- **Files modified:** frontend/src/lib/tailscale.ts, frontend/src/components/SettingsSheet.tsx
- **Commit:** b40367f3

**3. [Rule 1 - Spec change] Existing Security-pane assertion updated**
- **Found during:** Task 3
- **Issue:** `SettingsSheet.test.tsx` asserted the old Session keys sentence the plan replaces.
- **Fix:** updated the expected string.
- **Commit:** c8fefe80

**4. Small additions to the copy file.** `UNNAMED_VIEWER` ("Unknown viewer", for a row whose names are all empty after sanitizing) and `SHARE_STATUS_LABEL` ("Status") were added to `lib/shareDesktopCopy.ts` so the components still hold no literal UI string. `SECURITY_KEYS_DESCRIPTION` is imported into SettingsSheet under the alias `SESSION_KEYS_DESCRIPTION` (the plan's grep name).

**5. The plan's store artifact lists `events`; the hook returns `lastEvent`** (the latest event line). The join/leave announcement is computed in `ViewerList` from the viewers list instead, so it works without the event stream.

**6. TDD note.** The ShareDesktopSheet Task 3 cases, ViewerList and the AR-21-01 tests were seen failing first. `SettingsSheet.share.test.tsx` was written first but its component code was added before I ran it, so I did not observe it red.

## Verification

- `pnpm exec tsc --noEmit -p tsconfig.json` clean; full `vitest run` 145 files, 2647 tests pass (ShareDesktopSheet 26, ViewerList 16, SettingsSheet.share 11 new).
- `GOTOOLCHAIN=go1.26.11 go test -short ./internal/tailnet ./internal/daemon ./internal/webserver` pass; `go vet` on tailnet and daemon clean. `bash tests/check-brand.sh` clean before each commit.
- Not run, by constraint: Portty itself, any real screen capture, tray, tailscaled or tailnet. Frontend `node_modules` came from `pnpm install --frozen-lockfile` (no new packages).

## For 22-13 (TESTING.md, not edited here)

New test files, all with Vitest unless noted:
- `frontend/src/components/__tests__/ShareDesktopSheet.test.tsx` (DSHR-04, DSHR-05, DSHR-06)
- `frontend/src/components/__tests__/ViewerList.test.tsx` (DSHR-03, DSHR-04, DSHR-05)
- `frontend/src/components/__tests__/SettingsSheet.share.test.tsx` (DSHR-06, entry points)
- Go: additions to existing files `internal/tailnet/tailnet_test.go`, `internal/daemon/peer_desktop_test.go`, `internal/daemon/peer_connect_test.go` (AR-21-01; no new Go test file)

Manual-row candidates (VM only, never the host):
- Share Desktop… from the tray, a notification click and Settings opens one sheet, the share starts with no extra click after the CT line was acknowledged, Copy Link pastes a working guest link in another app.
- First ever open (no CT acknowledgement): the CT line, Start Sharing records it and starts.
- The sheet over Settings: Done returns to Settings; Esc closes only the sheet.
- Stop Sharing from the sheet and from the tray: the open sheet drops to Not sharing and does not restart; the guest page shows the ended card.
- A real guest in the list: badge, switches, caption wrap at 560 px in light and dark; Allow control on and off changes the guest's page; a long or right-to-left display name and a look-alike name wrap and keep the login name visible.
- Status row with Screen Recording revoked (Needs permission, Set Up Permissions… opens on top of the sheet) and with the display asleep.
- VoiceOver reads `<name> joined` / `left` and the grant switches by label plus name.
- AR-21-01 real-tailnet check (see above).

## Known Stubs

None. (The `.admit*` CSS is intentionally unused until 22-11.)

## Threat Flags

None. The AR-21-01 filter narrows an existing surface; the GUI adds no endpoint, file access or schema. T-22-34 (login always visible, names as text in `bdi`, second sanitize pass) and T-22-35 (grant switches off by default with the consequence caption beside them) are implemented and tested.

## Self-Check: PASSED

Files found: frontend/src/lib/desktopShare.ts, ShareDesktopSheet.tsx, ViewerList.tsx, SettingsRow.tsx, the three new test files. Commits 130f64ea, b40367f3, f10975a4 and c8fefe80 are ancestors of HEAD.

Commit hashes updated 2026-11-27: a later rebase rewrote the plan's commits with identical content (0df42da6→130f64ea, 127e7804→b40367f3, 4c7d4c17→f10975a4, 90fa81ec→c8fefe80, e0116e79→218b8f17).
