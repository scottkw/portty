---
phase: 14-mobile-web-app
plan: 09
subsystem: copy
tags: [share-copy, launcher, docs, go-test, vitest]
requires:
  - phase: 14-01
    provides: the same-account gate that made the old share sentences false
  - phase: 14-07
    provides: spike decision "continue" and the iOS 26 Add to Home Screen labels
provides:
  - truthful share copy in seven places (MOB-05)
  - docs/launcher-app.md, the static copy for portty.app/app (MOB-04)
  - TestLauncherCopy pinning the launcher's required items and absences
affects: [14-12, 14-13, 14-14]
status: complete
actuals:
  tokens: 3100
  tasks: 3
  commits: 3
plan_head_before: cb139e3c6aa9034624ac243febff59c579510592
plan_head_after: 963579d0712f304e7fdedd597be0a41d1bac87e7
tech-stack:
  added: []
  patterns:
    - "static launcher copy lives in this repo and is pinned by a root-package test; nothing here trusts portty.app"
key-files:
  created:
    - docs/launcher-app.md
    - launcher_copy_test.go
  modified:
    - frontend/src/lib/shareCopy.ts
    - cmd_share.go
    - cmd_share_test.go
    - frontend/src/components/SettingsSheet.tsx
    - frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx
    - frontend/src/components/__tests__/SharePopover.test.tsx
    - README.md
    - CHANGELOG.md
key-decisions:
  - "Launcher step 3 uses the iOS 26 labels from the spike (More, Share, View More, Add to Home Screen) and says the icon may be only in the App Library's Webclips folder"
requirements-completed: []  # MOB-04 / MOB-05 need the real-phone check (14-14); requirements.mark-complete not run, same as 14-03, 14-04 and 14-08
---

# Phase 14 Plan 09: Share copy and launcher page copy Summary

Every sentence that said a share link opens for anyone on the tailnet now says it only opens on devices signed in to your Tailscale account, and docs/launcher-app.md holds the static portty.app/app copy, pinned by TestLauncherCopy.

Logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.yXIM29Z1bJ` (Task 1), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.OdwkBge65S` (Task 2), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.GKaRHqicZ2` (Task 3, including the single full Go run).

## Copy changes (old to new)

- `shareCopy.ts` `SHARE_WARNING` and `cmd_share.go` printed line: "Anyone on your tailnet with this link has full control." to "Treat this link like a password. It only opens on devices signed in to your Tailscale account." Same sentence in the GUI and the CLI.
- `SHARE_DESCRIPTION` and the Settings Sharing footnote: "a link for any browser on your tailnet" to "a link that opens in a browser on any device signed in to your account".
- README.md: "Any device on your tailnet can also open the session's link in a browser, and anyone with the link has full control." to "Any device signed in to your Tailscale account can also open the session's link in a browser, with full control. Treat the link like a password."
- CHANGELOG.md `## Unreleased` line only: "a link for any browser on your tailnet" to "a link that opens in a browser on any device signed in to your account". Released history untouched.
- Tests changed on purpose: `cmd_share_test.go`, `SharePopover.test.tsx` (description and warning), `SettingsSheet.tailscale-status.test.tsx` (footnote).
- Left alone (reachability, not the gate): Sidebar peer-unreachable line, cmd_attach.go, portforward.go.

## Launcher

`docs/launcher-app.md` is UI-SPEC's block with every item kept. One change beyond tightening: step 3 uses the iOS 26 labels recorded in 14-SIM-CHECKS.md "Spike decision" (More menu, Share, View More, Add to Home Screen) and notes that on a new iPhone the icon may land only in the App Library, in the Webclips folder. The warning sentence equals the new SHARE_WARNING. It has a top HTML comment saying it is static source copy. `launcher_copy_test.go` (`TestLauncherCopy`) requires the link format, Add to Home Screen, Open as Web App, Portty, iCloud, Messages, Universal Clipboard, AirDrop, the own-devices refusal, Link No Longer Works, Ctrl and the warning; it forbids `<input`, `<form`, `<script`, `wss://`, `cap=` and `?session=`. TDD order: the test was run first and failed (file missing), then passed once the doc was written.

## Verification

- Task 1: `go test -race -short -count=1 -run Share .` passed; `vitest run SharePopover` 29 passed; the old sentence is gone outside .planning.
- Task 2: `vitest run SharePopover SettingsSheet` 76 passed, 0 failed; `tests/check-share-removal.sh` exit 0; the old description and README clause are gone outside .planning.
- Task 3: `TestLauncherCopy` PASS; `git grep portty\.app` in internal/webserver and internal/capability (non-test) found nothing; the single full run `go test -race -short -count=1 ./...` exited 0 with 14 `ok` packages and no FAIL lines. The G-1 `internal/pty` flake did not occur, so no re-run was needed.

## Deviations from Plan

None to code. One note on the plan's own verify: vitest output carries ANSI colour codes, so the grep `Tests +[0-9]+ passed` misses it unless `NO_COLOR=1` is set. I set `NO_COLOR=1` for Tasks 2 and 3 and confirmed Task 1's result by reading the log. The same trap applies to 14-10 to 14-14 verify blocks that grep vitest output.

## Not verified

- The launcher steps (More, Share, View More, Add to Home Screen, App Library) rely on the 14-07 spike record; nothing here was checked on a real phone. 14-13 and 14-14 cover that.
- The launcher's claim that the icon also stops on "a new key" or a background-service restart is UI-SPEC's wording, not re-checked here.
- No rendering of the page: the website repo owns that.

## Known Stubs

None.

## Threat Flags

None. T-14-20 (copy pinned by tests) and T-14-21 (static launcher, no portty.app in origin, CSP or token code) are mitigated as planned.

## Self-Check: PASSED

Files found: docs/launcher-app.md, launcher_copy_test.go, 14-09-SUMMARY.md. Commits found: e3f6afa7, 52ba2c44, 963579d0.
