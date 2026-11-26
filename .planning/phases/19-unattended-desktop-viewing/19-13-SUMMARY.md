---
phase: 19-unattended-desktop-viewing
plan: 13
subsystem: settings-ui
tags: [settings-sheet, desktop-group, wording-table, wails-bindings, react, css-tokens]
status: complete

requires:
  - phase: 19-08
    provides: daemon GET/PUT /desktop and GET /desktop/link, client methods DesktopStatus, SetDesktopAlwaysAvailable, DesktopLink
  - phase: 19-11
    provides: DesktopSetupSheet, the CSS class names it uses unstyled
  - phase: 19-17
    provides: GetDaemonRepair and OpenLoginItems bindings
provides:
  - "GetDesktopStatus, SetDesktopAlwaysAvailable, GetDesktopLink bindings (never throw) with DesktopStatusResult, DesktopResult, DesktopLinkResult"
  - "Settings > Sharing Desktop group: switch, Status, Link (Copy Link), Permissions, Background service rows, footnote, one inline-error slot"
  - "desktopStatusText: the one wording table (Status text, dot, tray line) that 19-14's Go mirror reads"
  - "Regenerate Signing Key copy (Session keys row and dialog body) names saved desktop links"
  - "styles for the warn dot, Granted text, setup sheet width, the 256x160 capture box, Copy Link width"
affects: [19-14, 19-15, 19-16]

actuals:
  tokens: 16700
  tasks: 3
  commits: 5
plan_head_before: c8515fa59bd7ba5521a15e30abf218385b3cff42
plan_head_after: 95d6306ec35834945e5cd98d79c5d6b88d6e4240

tech-stack:
  added: []
  patterns:
    - "Poll the daemon every 2 s only while the Sharing pane is showing; a failed read keeps the last state but marks it unanswered (Checking…)"
    - "The setup sheet's open state lives in SettingsSheetContent so the sheet renders after the settings dialog in the DOM (topmost modal)"
    - "The link is fetched on Copy Link, written to the clipboard, and held only in a local variable"

key-files:
  created:
    - frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx
  modified:
    - app_desktop.go
    - app_desktop_test.go
    - frontend/src/components/SettingsSheet.tsx
    - frontend/src/components/RegenerateKeyModal.tsx
    - frontend/src/components/__tests__/SettingsSheet.test.tsx
    - frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx
    - frontend/src/style.css
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js

key-decisions:
  - "Waiting for Tailscale is GUI-derived (the Tailscale health the sheet already has) plus the daemon's userspace verdict; the daemon's own 'off' verdict is not consulted, so a stale daemon value cannot show a wrong state"
  - "A tailscale_userspace refusal shows no error line: it records userspace locally and the switch shows blocked with the userspace sentence at once (the daemon's next status says the same)"
  - "A turn-on with a known-missing permission opens the setup sheet every time, not only the first (Interaction Contract row 1 says 'a permission is missing', not 'first')"
  - "An unknown permission read stays Checking… with no dot (never 'Needs permission'), as 19-11 decided for the sheet"
  - "capture_paused tray text is 'Desktop: Paused after repeated errors', the same string 19-14's plan lists"

patterns-established:
  - "An unanswered or unknown value renders as words with no dot (StatusDot returns null)"

requirements-completed: [DSHR-01, DPRM-01, DPRM-02, DPRM-03]

coverage:
  - id: G1
    description: "The three bindings carry the daemon's status, turn-on/off result and link; a refusal keeps its code and detail; a nil client or dead daemon is 'unavailable' and never throws"
    requirement: "DSHR-01"
    verification:
      - kind: integration
        ref: "app_desktop_test.go#TestDesktopSettingsBindings"
        status: pass
    human_judgment: false
  - id: G2
    description: "Turning the switch on shows Getting a link…, then the live state (Ready, Streaming to 2 viewers); Copy Link reads the link only on click, writes it to the clipboard, reads Copied! for 2 s, and the link text never reaches the DOM"
    requirement: "DSHR-01"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx"
        status: pass
    human_judgment: false
  - id: G3
    description: "The wording table maps every daemon state (plus capture_paused and Waiting for Tailscale) to the exact Status text, dot and tray line, with 1 viewer singular; the three description lines (awaiting approval, needs permission, approaching lapse) appear only for their states"
    requirement: "DPRM-02"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx"
        status: pass
    human_judgment: false
  - id: G4
    description: "Refusals and errors: Tailscale off and userspace block the switch (aria-disabled, describedby the reason) and never block turning off; https_off, start and stop failures return the switch to its prior state with the exact inline error (role=alert); clipboard failure says so; CT line shows until acknowledged; no confirm dialog either way"
    requirement: "DSHR-01"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx"
        status: pass
    human_judgment: false
  - id: G5
    description: "Permissions row reads Granted/Review Permissions…, Needs permission/Set Up Permissions…, or Checking…; its button and a turn-on with a missing permission open the setup sheet (topmost modal), closing re-reads the grants and returns focus to the switch with the desktop still on"
    requirement: "DPRM-01"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx"
        status: pass
    human_judgment: false
  - id: G6
    description: "Background service row: shown only while on and launchd is false, with the five Row 5 descriptions by repair kind and Open Login Items for login_items only; gone when launchd is true or the desktop is off"
    requirement: "DSHR-01"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx"
        status: pass
    human_judgment: false
  - id: G7
    description: "Security copy and styles: both Regenerate strings name saved desktop links (pinned tests updated on purpose); warn dot, Granted text, setup sheet 560 px, capture box 256x160 with object-fit contain, Copy Link width, all on tokens; the no-literal-colour gate and the full frontend suite pass"
    requirement: "DSHR-01"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/SettingsSheet.test.tsx"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/style.settings.test.ts"
        status: pass
    human_judgment: false
  - id: G8
    description: "NOT run: how the group, the setup sheet and the 256x160 capture box actually look in the real WebKit window, the real Wails clipboard write, and the macOS-native feel"
    requirement: "DSHR-01"
    verification: []
    human_judgment: true
    rationale: "Host safety forbids launching the app on the host, and WebKit screenshots in the VMs are blank (axdump reads structure only). The look is a human check on the office Mac; 19-15 or 19-16 should add a manual row"

duration: 55min
completed: 2026-11-26
---

# Phase 19 Plan 13: Settings Desktop Group Summary

**Settings > Sharing now has the Desktop group: an Always-available desktop switch with live Status from a 2 s poll, Copy Link that reads the link only on click, a Permissions row that opens the setup sheet, a Background service repair row, the footnote and the refusal and error copy, all from one wording table, plus the Regenerate copy that names saved desktop links.**

## What was built

- **Task 1 (tracer, `34e91f2c`):** `GetDesktopStatus`, `SetDesktopAlwaysAvailable` and `GetDesktopLink` in `app_desktop.go` (nil client or transport error is `unavailable`, a daemon `ShareError` keeps its code and detail), the Wails d.ts/js entries, and the first path in `SettingsSheet.tsx`: switch, poll, Status row, Copy Link. `TestDesktopSettingsBindings` runs the bindings against a real HTTP server on the daemon socket path (a fake, no daemon, no tailnet).
- **Task 2 (`ce2178b5` RED, `e7c9e493` GREEN):** `desktopStatusText` for every state, Tailscale and userspace refusals, the inline errors, the CT line (this Mac's tailnet name from the Tailscale health, re-read after a turn-on), the Permissions row, the setup sheet hand-off with focus return to the switch, the Background service row and the footnote.
- **Task 3 (`743ecbfc` RED, `95d6306e` GREEN):** the two Regenerate strings, and CSS for `.ts-status__dot--warn`, `.ts-status__text--granted`, `.settings-row__note`, `.settings-row .button.button--copy` (76 px), `.dialog--setup` (560 px), `.desktop-setup__thumb`, `__placeholder` and `__result`. No token was added: `--color-status-warn` already existed in both blocks.

## TDD Gate Compliance

Task 1 is a tracer (no RED by design). Tasks 2 and 3 each have a `test(19-13)` commit before the `feat(19-13)` commit. Task 2's RED failed on the planned assertions (31 of 40 failing: the table returned Checking… for every non-idle state, rows were missing, text mismatched); the 9 passing at RED were the tracer and absence checks. Task 3's RED failed 6 of 26 on the old strings and the missing rules. The `gsd_run check tdd-red-evidence` classifier was not run (these are task-level flags in an `execute` plan); the semantic check above is self-assessed.

## Deviations from Plan

**1. [Rule 3 - Blocking] Updated a Phase 4 pin outside files_modified**
- `SettingsSheet.tailscale-status.test.tsx` asserted the whole Sharing pane held one row, no button, and no text containing "Port". The Desktop group is now in that pane on purpose (and says "Portty"). The test now scopes the same three assertions to the Tailscale group (the first `.settings-group`). One file, nine changed lines.

**2. [Plan interpretation] A first turn-on is every turn-on with a known-missing permission**
- The plan text says "first turn-on"; 19-UI-SPEC Interaction Contract row 1 says the sheet opens when a permission is missing. Opening only the first time would leave a user who skipped it with no prompt on the next turn-on. The sheet opens whenever the read after a successful turn-on says a permission is missing, and never when the read failed.

**3. [Plan interpretation] A userspace refusal is not an inline error**
- The plan lists https_off and start failures as inline errors and says userspace blocks the switch. A `tailscale_userspace` answer to a click records userspace locally so the switch blocks with the userspace sentence at once; showing the same sentence again as an error would duplicate it.

**4. [Plan interpretation] Waiting for Tailscale uses the GUI's Tailscale health plus the daemon's userspace verdict**
- The daemon's own `tailscale` value is sticky (see findings), so only `userspace` is read from it, and `off` comes from the Tailscale health the sheet already holds. This matches what 19-14's tray plan says (off or userspace).

**5. [Plan note] Commit trailer**
- The shared instructions say `Claude Opus 5.5`; the dispatch said to use exactly `Claude Sonnet 5.5`. I used the dispatch's (the accurate model), as 19-02, 19-09, 19-11 and 19-17 did.

**6. [Plan note] No plan-head ledger file**
- The per-plan ledger under the worktree's git dir could not be written from this sandbox, so `plan_head_before` is the dispatch base (`c8515fa5`) and `commits: 5` was measured with `git rev-list --count c8515fa5..HEAD` before this SUMMARY commit.

**Total deviations:** 6 (1 blocking, 3 interpretation, 2 notes). None changes a contract 19-14 or 19-16 build on.

## Findings for the orchestrator and later plans

1. **The userspace block can stick (needs a decision).** The daemon records the Tailscale verdict in `desktopTS` only when it checks (on a turn-on attempt, and every 10 s while the desktop is on). While the desktop is off nothing rechecks it, and `GET /desktop` keeps reporting the old value. So after one `tailscale_userspace` refusal, the switch stays blocked (as 19-UI-SPEC specifies for userspace) even if the user then switches Tailscale to its standard mode, until the daemon restarts or a turn-on recheck runs, which the blocked switch prevents. Options: (a) the daemon reports `tailscale: ""` while the desktop is off, and the GUI then blocks userspace only for the moment after a refusal (the refusal path already sets it locally; a clearing poll would unblock it within 2 s, so the user can retry), (b) the daemon rechecks the interface address on each `GET /desktop` while off (cheap, local), or (c) keep as is. I recommend (b), a few lines in `internal/daemon/desktop_api.go` (19-08's file; I did not touch it). The same stickiness does not affect the other refusals, which only produce errors on a click.
2. **Accessibility pane name on macOS 27.2.** 19-12 found the pane is now titled "Device Control and Data Access". No copy in this plan names a pane (the Permissions row says "Screen Recording and Accessibility.", the permission's name, per the contract). The how-to in `DesktopSetupSheet.tsx` (19-11) says "then Accessibility" and may need a 27.2 wording, for 19-16 or the owner.
3. **19-14 mirror.** `desktopStatusText(...).tray` is the column to mirror; the strings are tested in `SettingsSheet.desktop.test.tsx` ("the wording table"). `capture_paused` tray is `Desktop: Paused after repeated errors`; `locked` with 1 viewer is `Desktop: Locked, streaming to 1 viewer` (the contract says N; singular follows its own rule).
4. **TESTING.md (19-16).** Add `frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx` (new) and the `TestDesktopSettingsBindings` addition to `app_desktop_test.go` (file already listed by 19-11), with traceability rows for DSHR-01 and DPRM-01/02/03, and a manual row for the real look of the group and setup sheet (G8). TESTING.md was not touched.

## Known Stubs

None. (`NO_STATUS` in `SettingsSheet.tsx` is the explicit "no answer yet" value, not placeholder data reaching the UI as a fact: it renders Checking… or an off switch.)

## Threat Flags

None beyond the plan's register. T-19-44: the link is read inside `handleCopy`, passed to `ClipboardSetText` and not stored; the tracer test asserts neither the token nor the host reaches the DOM, and a second test asserts no field, picture or canvas exists in the group. T-19-45: one switch with a description of the consequence, plus Status, which shows the desktop is on.

## Verification

- `go vet .`, `gofmt -l app_desktop.go app_desktop_test.go` clean; `go test -race -count=1 -run TestDesktop .` passes.
- `pnpm --dir frontend exec tsc --noEmit -p tsconfig.json` clean; `pnpm --dir frontend test` 126 files, 1960 tests pass (the no-literal-colour gate included); `SettingsSheet.desktop.test.tsx` has 40 cases.
- `bash tests/check-share-removal.sh` and `bash tests/check-brand.sh` clean (a first run flagged the words "QR code" in a test title; reworded).
- Acceptance greps: `Always-available desktop` 1, `SetDesktopAlwaysAvailable` in App.d.ts 2, `func (a *App) GetDesktopLink(` 1, `Waiting for approval at the Mac` 1, `userspace-networking mode` 1, `DesktopSetupSheet` 2, `Background service` 2, `dialog--setup` 1, `ts-status__dot--warn` 1.
- No app, daemon, capture, launchd or tailnet call ran on the host; no VM work was part of this plan.

## Self-Check: PASSED

- Files exist: app_desktop.go, app_desktop_test.go, SettingsSheet.tsx, RegenerateKeyModal.tsx, SettingsSheet.desktop.test.tsx, style.css, App.d.ts, App.js.
- Commits `34e91f2c`, `ce2178b5`, `e7c9e493`, `743ecbfc`, `95d6306e` are ancestors of HEAD.
