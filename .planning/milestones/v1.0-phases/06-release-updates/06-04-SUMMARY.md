---
phase: 06-release-updates
plan: 04
subsystem: updater
status: complete
tags: [updater, vm-checks, vmpreview, fakegh, testing-md]
requires:
  - "06-01: internal/updater check, download, verify, reveal; release.yml DMG notarization"
  - "06-02: app_update.go seam vars, poller, notify-once, Help alerts, tray item"
  - "06-03: banner states driven by update:status"
provides:
  - "updates_vmpreview.go: VM-only fake update feed (127.0.0.1:8788, loopback allowlist, 15 s tick, 2 min cadence)"
  - "cmd/fakegh: loopback-only fake GitHub Releases API for M-92"
  - "06-VM-CHECKS.md: U-01..U-06 PASS on real macOS"
  - "TESTING.md: Phase 6 counts, UPD rows, Category AU with M-92 and M-93"
affects: [phase-6 verification, first tagged release (M-93)]
tech-stack:
  added: []
  patterns: ["VM-only behaviour behind the existing vmpreview tag, guarded by tests/check-brand.sh"]
key-files:
  created:
    - updates_vmpreview.go
    - cmd/fakegh/main.go
    - app_update_release_test.go
    - .planning/phases/06-release-updates/06-VM-CHECKS.md
  modified:
    - app_update_test.go
    - tests/check-brand.sh
    - TESTING.md
decisions:
  - "TestUpdateDefaults_ReleaseBuild moved to its own !vmpreview file (app_update_release_test.go), so go vet/test with the vmpreview tag compiles; this adds one counted Go test file"
  - "No verification bypass: the reveal path used GIFsmith v1.0.1's notarized Team S2K7P43927 DMG through the real codesign and spctl"
  - "Notification delivery is not observable on the ad-hoc wails build; U-01/U-02 PASS rests on one dispatch and one notified line per version, and M-93 checks delivery on the notarized release"
metrics:
  duration: "about 95 min wall clock (includes a host sleep of about 40 min during U-02)"
  completed: 2026-11-18
actuals:
  tokens: 10100
  tasks: 3
  commits: 3
plan_head_before: 5cfa6b4d4b5a6bb610e7901b5f08703cb20d825e
plan_head_after: f575240b13f55e32f95b48c0ee1a58e954b75c31
---

# Phase 6 Plan 04: VM Update Checks and TESTING.md Summary

A vmpreview build pointed at a fake GitHub inside the Tart VMs met all four Phase 6 success criteria on real macOS. It found updates on its own and showed one notice per version. The four Check for Updates… alerts appeared. Unsigned, ad-hoc and bad-digest DMGs were rejected and deleted. A notarized DMG was revealed in Finder with mode 0600 and the quarantine flag. Every failure stayed quiet. TESTING.md now covers the whole phase.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 (tracer) | vmpreview seam, cmd/fakegh, U-01 | f562372e |
| 2 | VM checks U-02..U-06 | 18df7f92 |
| 3 | TESTING.md for Phase 6 | f575240b |

Tracer gate: Task 1's verify passed (vet with and without vmpreview, fakegh vet, normal build, brand gate, wailsjs clean, U-01 PASS row) before Task 2 started.

## VM results

| Check | Result | Summary |
|-------|--------|---------|
| U-01 | PASS | First request 12 s after `open` (10 s after Portty's startup); banner "Portty 9.9.9 is available." with Download; one dispatch, one `notified 9.9.9`; tray menu `Open Portty, Update Available…, Quit Portty` |
| U-02 | PASS | current 0.9.0 is quiet; newer 9.9.9 found at the 2 min periodic check with one notification; the next request is `inm=yes 304` with no repeat; 9.9.10 notified once |
| U-03 | PASS | "Portty is up to date."; "Version 9.9.10 is available." Download/Later (Later downloads nothing); "Couldn't check for updates." with the network text and with the rate-limit text; no notified lines from manual checks |
| U-04 | PASS | unsigned (codesign) and ad-hoc (team) give the signature text; bad digest gives the checksum text; Try Again; updates folder empty; no Finder window on a file |
| U-05 | PASS | from the banner and from the alert: Verifying, then "verified and shown in Finder"; Finder front window `updates`, DMG shown, "1 of 1 selected"; mode 600; quarantine `0081;…;Portty;` |
| U-06 | PASS | 404, draft, prerelease, noasset (two automatic checks each, 2 min apart), ratelimit and unreachable (three 15 s retries each): no banner, notification or alert; manual check says up to date for the first four |

Full evidence is in `.planning/phases/06-release-updates/06-VM-CHECKS.md`.

## Gate results

- `go vet -tags vmpreview .`, `go vet .`, `go vet ./cmd/fakegh`, `go build ./...`, gofmt: clean.
- `go test -race -short -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./...`: all packages ok.
- `bash tests/check-brand.sh`: clean. `bash tests/check-traceability-paths.sh`: 138 paths exist.
- frontend/src/wailsjs restored after both Wails builds; `git status` was clean before each commit.
- Task 3 verify: Go 129, vitest 88, total 225 match `git ls-files`; 16 UPD rows; no updater_test.go row; Category AU, M-92 and M-93 present.
- fakegh smoke test on the host loopback: 200 with ETag, 304 on a matching If-None-Match, the asset served, 404 for other names, and a non-loopback `-addr` refused.

## Deviations from Plan

1. **[Rule 3 - Blocking] TestUpdateDefaults_ReleaseBuild moved to app_update_release_test.go (`//go:build !vmpreview`).** This is what the 06-02 handoff asked for. It adds one counted Go test file, so the Go count is 129 and not 128. TESTING.md gives it a BRND-06 row.
2. **Not fixed, only noted: other root tests assume the release cadence.** `go test -tags vmpreview .` fails `TestUpdatePoller_DailyCadenceAndNoRepeat`, because the vmpreview cadence is 2 min and not 24 h. No gate or CI job runs the tests with that tag; only `go vet -tags vmpreview` runs. The test was left as it is.
3. **Notification delivery not observable (planned fallback).** The ad-hoc wails build gets `UNErrorDomain Code=1 "Notifications are not allowed for this application"`. U-01 and U-02 pass on one dispatch and one notified line per version, as the plan allows. M-93 checks delivery.
4. **The banner's downloading state was not caught in U-05.** A 6 MB download over loopback finishes inside one 1 s AX poll. The `/download/` request, Verifying and "verified and shown in Finder" were all seen. The downloading state itself is covered by UpdateBanner.test.tsx and App.update.test.tsx. I recorded U-05 as PASS and said this in its Evidence cell.
5. **The host slept in the middle of U-02.** The VM clocks fell about 67 minutes behind, then NTP corrected them from 17:45 to 18:52. The 9.9.10 check ran on the first 15 s tick after the jump, which is what the wall-clock design intends. The evidence states it.
6. **VM side effects, both dismissed.** `strings` in my deploy script made the VMs offer to install the Command Line Tools; I killed that prompt. A Finder osascript query raised an Automation prompt for sshd-keygen-wrapper; I answered Don't Allow. I used the AX status bar ("1 of 1 selected") as the selection evidence instead.
7. **Plan commit ledger.** The protocol 0c ledger file was not written before the first commit, and the worktree guard blocks writes to the git dir. The base is 5cfa6b4d, the reset point, and `git rev-list --count 5cfa6b4d..HEAD` gives 3.

No product code changed, and no U-row failed.

## VM cleanup

Both VMs got a clean HEAD build from `./build.sh --platform macos` (no vmpreview, ad-hoc, `grep -c 127.0.0.1:8788` gives 0) and are running it. settings.json was restored from the backup; its sha1 d26950059cbd… matches the original. fakegh, the DMGs, axtray, the logs, update_notified and the updates cache were removed. Nothing listens on 8788.

## Known Stubs

None.

## Threat Flags

None. fakegh (loopback only, VM only) is T-06-18. The vmpreview feed is T-06-17. Nothing was signed with the host Developer ID (T-06-20).

## Self-Check: PASSED

- FOUND: updates_vmpreview.go, cmd/fakegh/main.go, app_update_release_test.go, .planning/phases/06-release-updates/06-VM-CHECKS.md
- FOUND commits (ancestors of HEAD): f562372e, 18df7f92, f575240b
