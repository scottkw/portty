---
phase: 19-unattended-desktop-viewing
plan: 11
subsystem: desktop-permissions
tags: [perm-probe, tcc, onboarding, test-capture, wails-bindings, react-sheet, daemon-api]
status: complete

requires:
  - phase: 19-05
    provides: screencap.Preflight (non-prompting reads), screencap.CaptureStill, ErrPermissionDenied
  - phase: 19-08
    provides: daemon /desktop routes, SetDesktopNotifier, daemon client style
  - phase: 19-09
    provides: desktop.DefaultProber (fresh-process reader, 1 s cache), NoteTestCapture, Manager.ResetBreaker
  - phase: 19-17
    provides: openSystemURL (the /usr/bin/open seam), the hand-maintained App.d.ts / App.js bindings
provides:
  - "`portty daemon perm-probe`: one JSON line {sr,ax,pe} from the non-prompting reads, internal (not in usage), starts nothing"
  - "wireDesktopNotifier: the daemon's native lapse sender is wired in the no-arg, run and --takeover cases only"
  - "GET /desktop/permissions (200 {sr,ax} or 503 probe_failed) and POST /desktop/test-capture (200 image/jpeg, 409 needs_permission, 504 timeout, 500 capture_failed) on the daemon socket"
  - "DaemonClient.DesktopPermissions and DesktopTestCapture"
  - "GetDesktopPermissions, RunDesktopTestCapture, OpenDesktopPermissionPane bindings (never throw; two fixed panes)"
  - "DesktopSetupSheet: live rows, Open System Settings, Test Capture, the two notes, accessible modal shell"
affects: [19-12, 19-13, 19-14, 19-16]

actuals:
  tokens: 13000
  tasks: 3
  commits: 4
plan_head_before: d792ca0730cc1d367476952accbf8438deab4d55
plan_head_after: 50f5b13f675436de8c99c382d0074d1ca6874728

tech-stack:
  added: []
  patterns:
    - "Daemon-owned grant reads: the GUI never preflights; it asks the daemon, which asks a fresh child (research C1)"
    - "A failed read is an explicit code or null, never a default false (probe_failed, 'unavailable', 'Checking…')"
    - "Package-level seams with restore-returning setters (SetDesktopPermsForTest, SetDesktopCaptureForTest) instead of widening SetDesktopSeamsForTest"

key-files:
  created:
    - app_desktop.go
    - app_desktop_test.go
    - internal/daemon/desktop_permissions_test.go
    - frontend/src/components/DesktopSetupSheet.tsx
    - frontend/src/components/__tests__/DesktopSetupSheet.test.tsx
  modified:
    - cmd_daemon.go
    - cmd_daemon_test.go
    - internal/daemon/desktop_api.go
    - internal/daemon/client.go
    - internal/desktop/permissions.go
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js

key-decisions:
  - "Test Capture is blocked only when Screen Recording is KNOWN off (sr === false). An unknown read (probe failing) leaves it enabled, so a broken probe cannot lock the user out of the one check that proves capture works"
  - "Before the first read, and after a failed read, a row shows 'Checking…' (no how-to) rather than 'Not granted'; a failed read keeps whatever the row last showed"
  - "Latest-answer-wins sequencing in the sheet instead of an in-flight guard, so a focus refresh is never dropped behind a slow poll"
  - "A failed NoteTestCapture write is logged and the JPEG is still returned: the user's capture worked, the approval record is a best-effort signal"

patterns-established:
  - "Daemon error bodies for onboarding endpoints are {\"code\": ...}; the client returns coded refusals as a code with a nil error (test capture) or an error (permissions)"

requirements-completed: [DPRM-01]

coverage:
  - id: G1
    description: "perm-probe prints exactly one JSON line with sr, ax, pe and starts nothing; a failed read is an error with no output; the daemon notifier is wired once, before the daemon, in the no-arg, run and --takeover cases and in no other"
    requirement: "DPRM-01"
    verification:
      - kind: unit
        ref: "cmd_daemon_test.go#TestCmdDaemon_PermProbe"
        status: pass
      - kind: unit
        ref: "cmd_daemon_test.go#TestCmdDaemon_WiresDesktopNotifier"
        status: pass
    human_judgment: false
  - id: G2
    description: "GET /desktop/permissions reads through the Prober (two asks 1.1 s apart see sr false then true), shares one spawn across five overlapping asks, and answers 503 probe_failed (never sr false) for a failing child, malformed, missing-key, unknown-key, empty and trailing-line output"
    requirement: "DPRM-01"
    verification:
      - kind: integration
        ref: "internal/daemon/desktop_permissions_test.go#TestDesktopPermissions_EndToEnd"
        status: pass
      - kind: integration
        ref: "internal/daemon/desktop_permissions_test.go#TestDesktopPermissions_SharedSpawn"
        status: pass
      - kind: integration
        ref: "internal/daemon/desktop_permissions_test.go#TestDesktopPermissions_ProbeFailure"
        status: pass
    human_judgment: false
  - id: G3
    description: "POST /desktop/test-capture returns a JPEG and records the approval; needs_permission is 409, the ceiling is 504 timeout, anything else 500 capture_failed, and no failure records an approval; a good capture clears a tripped breaker on a running Manager"
    requirement: "DPRM-01"
    verification:
      - kind: integration
        ref: "internal/daemon/desktop_permissions_test.go#TestDesktopTestCapture_Codes"
        status: pass
      - kind: integration
        ref: "internal/daemon/desktop_permissions_test.go#TestDesktopTestCapture_ClearsBreaker"
        status: pass
    human_judgment: false
  - id: G4
    description: "The bindings never throw (nil client, dead daemon, failing daemon all give an error code and no guessed grant, a success gives the data URL); OpenDesktopPermissionPane opens only the two fixed panes, refuses files, empty, wrong case and raw URLs, and the real opener refuses under a test binary"
    requirement: "DPRM-01"
    verification:
      - kind: integration
        ref: "app_desktop_test.go#TestDesktopBindings_NeverThrow"
        status: pass
      - kind: unit
        ref: "app_desktop_test.go#TestOpenDesktopPermissionPane"
        status: pass
    human_judgment: false
  - id: G5
    description: "The sheet polls every 2 s while open and at once on window focus, with no timeout; rows flip between Not granted (warn dot, how-to) and Granted on the poll that sees the change; Test Capture is aria-disabled with its reason until Screen Recording reads granted, shows Capturing with a placeholder, then the thumbnail or the alert, then Test Again; the dialog is modal, labelled, traps Tab, closes on Esc and Done and returns focus; the copy matches the contract"
    requirement: "DPRM-01"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/DesktopSetupSheet.test.tsx"
        status: pass
    human_judgment: false
  - id: G6
    description: "NOT run: the real perm-probe child under a launchd daemon reading the bundle's live grant (V6), both System Settings pane URLs on macOS 27.2 (V9), the real Test Capture JPEG and the 27.2 picker-bypass dialog, the real headless-daemon notification (V1), and whether note 1 matches the real dialog wording"
    requirement: "DPRM-01"
    verification: []
    human_judgment: true
    rationale: "Host safety forbids the real preflight, capture, System Settings and notification calls on the host; rows V1, V6 and V9 in 19-12 and M-160 on the office Mac decide them. The note 1 wording is the plan's backstop truth"

duration: 75min
completed: 2026-11-26
---

# Phase 19 Plan 11: Guided Permission Setup Summary

**A fresh-process `portty daemon perm-probe`, two daemon endpoints that read the live Screen Recording and Accessibility grants and run the Test Capture in the grant-holding daemon, three never-throwing GUI bindings, and the Set Up Desktop Sharing sheet that polls every 2 s with no timeout, flips each row as the user grants, and shows a thumbnail of what viewers will see.**

## Accomplishments

- **perm-probe (DPRM-01):** `cmdDaemon perm-probe` calls `screencap.Preflight` and writes `{"sr":..,"ax":..,"pe":..}` plus a newline, nothing else; a failed read is an error and no output. It is absent from the usage text (grep of `main.go` is 0). It is the child 19-09's Prober already spawns.
- **Daemon notifier wired:** `wireDesktopNotifier` (`daemon.SetDesktopNotifier(sendNotification)`) runs first in the no-arg, `run` and `--takeover` cases and nowhere else, so the daemon can post the lapse notification when no GUI claims it. A `runDaemonFunc` seam was needed to test the order.
- **Endpoints:** `GET /desktop/permissions` returns `{sr, ax}` from `desktop.DefaultProber()`; a failed probe is 503 `{"code":"probe_failed"}`. `POST /desktop/test-capture` runs `screencap.CaptureStill` (512 px, 15 s ceiling) in the daemon, maps errors to 409/504/500 codes, and on success calls `NoteTestCapture` and `ResetBreaker` on a running Manager. The JPEG is only written to the response (`Cache-Control: no-store`), never to disk or the log.
- **Bindings:** `GetDesktopPermissions`, `RunDesktopTestCapture` (data URL) and `OpenDesktopPermissionPane("screen"|"accessibility")`. Results carry an `errorCode`; a failed read never becomes `sr:false`.
- **Sheet:** the dialog shell as `RegenerateKeyModal` with `useDialogFocus` (initial focus Done), the two rows with live status, Test Capture group and the two notes with the contract's exact copy. 14 vitest cases.

## Task Commits

1. **Task 1 (tracer): perm-probe, endpoints, client, bindings, sheet** - `f6c24b27` (feat)
2. **Task 2: bindings and daemon edge tests** - `65c0c501` (test)
3. **Task 3: sheet behaviour and accessibility tests** - `9420e8ed` (test)
4. **Whitespace fix to a binding line I realigned by mistake** - `50f5b13f` (style)

## TDD Gate Compliance

Tasks 2 and 3 are `tdd="true"` but have no RED commit before an implementing commit: the tracer (Task 1) carried the implementation they test (codes, fixed panes, never-throw, the whole component), as in 19-08. Their tests passed at first run, the "unexpected GREEN" case, so I did not trust them and shot each against a mutation (all restored with `git checkout -- <file>`):

- `mgr.ResetBreaker()` removed: `TestDesktopTestCapture_ClearsBreaker` times out.
- Probe failure answered `200 {}` instead of 503: all 6 `TestDesktopPermissions_ProbeFailure` subtests fail.
- `context.DeadlineExceeded` case removed: `TestDesktopTestCapture_Codes/ceiling` fails.
- `OpenDesktopPermissionPane` passing any other string to the opener: `TestOpenDesktopPermissionPane` fails on the AllFiles and https cases.
- Poll interval 2000 to 2500, the focus listener removed, and the blocked rule changed from `sr === false` to `sr !== true`: 4 of the 14 sheet tests fail (poll cadence, no-timeout, failed-read, row flips).

The git-log order (`test` before `feat`) is therefore not met for Tasks 2 and 3, flagged rather than hidden. The `tdd-red-evidence` classifier was not run (task-level flag in an `execute` plan).

## Deviations from Plan

**1. [Rule 3 - Blocking] Exported a Prober constructor from 19-09's file**
- **Found during:** Task 1 (tests).
- **Issue:** the plan's tests drive "the Prober's runner seam" from the daemon package, but `Prober.run` is unexported and `export_test.go` helpers are invisible outside `internal/desktop`.
- **Fix:** `desktop.NewProberWithRunner(run)` in `internal/desktop/permissions.go` (7 lines, not in `files_modified`). It is what makes `TestDesktopPermissions_SharedSpawn` exercise the real cache and shared spawn instead of a fake that trivially shares.
- **Commit:** `f6c24b27`

**2. [Plan interpretation] Seams are package-level vars, not SetDesktopSeamsForTest parameters**
- The plan says to add the capture func to `SetDesktopSeamsForTest`. Widening that signature would touch every 19-08 test; instead `SetDesktopPermsForTest(*Prober)` and `SetDesktopCaptureForTest(fn, ceiling)` are separate, return a restore func, and `defaultCaptureStill` / `desktopProber` are package vars. `api.go` (not in `files_modified`) is untouched.

**3. [Plan interpretation] `openSystemURL` reused instead of a new `openURLFunc`**
- 19-17 already added `openSystemURL` (runs `/usr/bin/open`, refuses under `testing.Testing()`). A second identical var would be duplicate code, so `OpenDesktopPermissionPane` uses it. The plan's acceptance criteria do not grep for the name.

**4. [Rule 2 - Missing critical behaviour] A failed or unknown read never reads as "Not granted" or blocks Test Capture**
- Plan says "a probe failure answers an error code, never a default false". Carrying that to the sheet: the rows show "Checking…" until the first successful read and keep their last value on a failed read; Test Capture is blocked only on a known `sr === false`. Otherwise a persistent probe failure would show a false "Not granted" and lock the user out of the capture test.

**5. [Plan interpretation] Row markup without the SettingsSheet `Row` helper**
- UI-SPEC says to move `Row` to its own file if the sheet needs it. The sheet's rows need a status line and how-to inside the text block, which `Row` (label plus description) does not take, and moving `Row` would edit `SettingsSheet.tsx` (19-13's file). The sheet uses the same `.settings-group` / `.settings-row*` class names directly.

**6. [Plan note] Binding method trailing `Cache-Control: no-store`, a code-only error body for coded failures, `DesktopTestCapture` returns a coded failure as `(nil, code, nil)`**
- As the plan's signature `(jpeg, code, err)` implies. Not a deviation of behaviour, recorded for 19-13 and 19-14 callers.

**7. [Instruction conflict] Commit trailer**
- The shared instructions say `Claude Opus 5.5`; the dispatch said to use exactly `Claude Sonnet 5.5`. I used the dispatch's (the accurate model), as 19-02, 19-09 and 19-17 did.

**Total deviations:** 7 (1 blocking, 1 missing behaviour, 3 interpretation, 2 notes). None changes a contract 19-12, 19-13 or 19-14 build on.

## Notes for later plans

- **19-13 (CSS, owns style.css):** classes this sheet uses without styling: `.dialog--setup` (width), `.desktop-setup__thumb` (the `<img>`), `.desktop-setup__placeholder` (the 256x160 box, `--color-field`), `.desktop-setup__result` (a `.settings-row` holding the thumbnail and caption), `.ts-status__dot--warn`, and `.ts-status__text--granted` (text in `--color-success-text`; the UI-SPEC needs a modifier this plan invented). A row with no status class (`ts-status__dot` alone) is the "Checking…" state.
- **19-13 (wiring):** `<DesktopSetupSheet open onClose returnFocusTo />`; `returnFocusTo` is the Row 4 button, or the switch when the sheet opens itself. Mount it after the settings sheet in the DOM so it is the topmost modal for Esc and Tab.
- **19-12 (VM):** V6 (the probe child of a launchd daemon reads the bundle's live grant) and V9 (both pane URLs on 27.2) are the rows this plan could not run. A contradicted V6 stops the phase there. V1 measures the daemon's notification sender wired here.
- **19-14:** adds the GUI-side notifier and the claim endpoint; the daemon side is wired.
- **19-16 (TESTING.md):** add `app_desktop_test.go`, `internal/daemon/desktop_permissions_test.go`, `frontend/src/components/__tests__/DesktopSetupSheet.test.tsx` (and the two new `cmd_daemon_test.go` tests) to the suite groups and a traceability row for DPRM-01. TESTING.md was not touched.

## Known Stubs

None. No placeholder data reaches the UI; the "Checking…" state is a real unknown. Nothing added to `.planning/WINDOWS.md`.

## Threat Flags

None beyond the register. T-19-38 (fixed panes, `TestOpenDesktopPermissionPane`), T-19-39 (thumbnail only over the 0600 socket, no-store, never written or logged), T-19-40 (strict decode in 19-09's Prober, an explicit `probe_failed`, six malformed cases tested) and T-19-41 (`TestDesktopPermissions_SharedSpawn`) are implemented. The new `POST /desktop/test-capture` is a screenshot on the daemon socket, same user and 0600 like every other daemon route; the relay loopback port does not serve it (it is built from `RelayHandler`, not the API mux).

## Verification

- `go build ./...`, `go vet ./...` clean; `gofmt -l` on the touched files empty; `golangci-lint run --new-from-rev=d792ca07 . ./internal/daemon/... ./internal/desktop/...` 0 issues.
- `go test -race -count=1`: `TestDesktopPermissions_*`, `TestDesktopTestCapture_*`, `TestDesktopAPI_*`, `TestCmdDaemon*`, `TestDesktopBindings_*`, `TestOpenDesktopPermissionPane` pass. `go test -race -short -count=1 ./internal/daemon/ ./internal/desktop/ ./internal/screencap/` pass (daemon 112 s). `go test -race -count=1 .` passes except `TestNoFakeWhoIsInRelease`, pre-existing in this worktree (`frontend/dist` is not built; same failure recorded in 19-02 and 19-17).
- `TestPreflightSymbols_OnlyInPreflightFile` passes: none of the three non-prompting read symbols or any prompting variant was named outside `preflight_darwin.m`.
- `pnpm install --frozen-lockfile`, `pnpm exec tsc --noEmit` clean; `vitest run` whole suite 125 files, 1915 tests pass (14 new).
- Acceptance greps: `"perm-probe"` in cmd_daemon.go 1; `SetDesktopNotifier(sendNotification)` 1; `perm-probe` in main.go 0; `/desktop/test-capture` in desktop_api.go 1; `Privacy_ScreenCapture` 1, `Privacy_Accessibility` 1, `AllFiles|FullDisk` 0 in app_desktop.go; `GetDesktopPermissions` in App.d.ts 2; `Set Up Desktop Sharing` in the sheet 2; `PERMISSION_POLL_MS.*2000` 1; the alt string 1; 14 test cases.
- `bash tests/check-brand.sh` clean.
- **Host safety:** no real perm-probe, preflight, capture, System Settings pane, notification, launchd, tailnet or daemon ran on the host. Every probe is a fake runner, every capture a fake still, every pane open a recording seam; the real bridges refuse under a test binary.

## Self-Check: PASSED

- Files exist: `app_desktop.go`, `app_desktop_test.go`, `internal/daemon/desktop_permissions_test.go`, `frontend/src/components/DesktopSetupSheet.tsx`, `frontend/src/components/__tests__/DesktopSetupSheet.test.tsx`.
- Commits `f6c24b27`, `65c0c501`, `9420e8ed`, `50f5b13f` are ancestors of HEAD.
