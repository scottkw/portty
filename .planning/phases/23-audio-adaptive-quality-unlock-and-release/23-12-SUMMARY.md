---
phase: 23-audio-adaptive-quality-unlock-and-release
plan: 12
subsystem: testing
tags: [testing-md, traceability, owner-run-checklist, readme, docs]
requires:
  - phase: 23-audio-adaptive-quality-unlock-and-release
    provides: 23-01 to 23-11 test files, VM rows in 23-VM-CHECKS.md, debug sessions
provides:
  - TESTING.md Sections 2 and 4 for every Phase 23 test file
  - Section 5 Category BN, M-224..M-234, owner-run, none run yet
  - From Phase 23 notes on M-161, M-162, M-172 and M-177
  - README review for v2.0 with the Sound and unlock paragraph
affects: [23-14, 23-15, 23-16]
requirements-completed: [DCTL-07, DESK-07, UNLK-01]
actuals:
  tokens: 13200
  tasks: 2
  commits: 3
plan_head_before: 472268ef492797a629927dbd85fc3ecef2b1c067
plan_head_after: 6e7b1660ef421f063dd527136ee3f970fdc80359
key-files:
  created: []
  modified: [TESTING.md, README.md]
key-decisions:
  - "Category BN holds eleven owner items (the plan listed five); the orchestrator additions and every PARTIAL, FAIL or NOT-RUN VM row each map to one of them or to an existing item (M-222, M-223, Phase 22's M-215)"
  - "The two goroutine tests map to DSHR-01 (always-available desktop stays healthy), the encoder tests already had DSHR-01 rows and were not duplicated"
  - "No 23-UAT.md was created: the plan does not ask for one and none exists; the real-Mac items live in TESTING.md"
status: complete
---

# Phase 23 Plan 12: TESTING.md and README for Phase 23 Summary

TESTING.md now records Phase 23's tests and owner-run checks (Sections 2, 4 and 5), and the README is reviewed against everything v2.0 shipped, with the Sound and unlock paragraph added.

Note on `commits:` above: 3 is measured from the ledger (`git rev-list --count 472268ef..6e7b1660`) and counts the three task commits; this SUMMARY is committed after them.

## Commits

| Task | Commit | What |
|------|--------|------|
| 1 | `bed515ba` | TESTING.md Section 2 counts and Phase 23 note, Section 4 rows |
| 2a | `b15d58e3` | TESTING.md Section 5 Category BN and From Phase 23 notes |
| 2b | `6e7b1660` | README review and the Sound and unlock paragraph |

## Task 1: Sections 2 and 4

- Counts recomputed from `git ls-files`, leaving out spike/: Go **282** (was 273), vitest **154** (was 149), Playwright 10, build-script 2, total **448** (was 434). The Phase 22 note already counted 12 Go and 2 vitest Phase 23 files merged at that time, so the new counts are +9 Go and +5 vitest. Phase 23 added 21 Go and 7 vitest files in all.
- A Phase 23 manifest note names every file, the support files (`export_unlock_test.go`, `export_relock_test.go`), the in-place edits and commit `0b4515f1`.
- **Commit `0b4515f1` (read from git):** `TestEncoderRefine_KeyframeIsSharperThanMoving` failed three times on PR #15's pull_request run (image macos-26-arm64 20261123, no AppleM2ScalerParavirtDriver; refine 35.96 dB against moving 33.11 dB). The floor (refine at least 6 dB above moving and at least 38 dB) now applies only when the encoder readback says hardware (`syntheticHardware`); otherwise the test logs both PSNRs and skips. Real Macs still enforce it. Noted in the manifest note, the Section 4 row, and M-161's From Phase 23 note.
- Section 4: 29 new rows. DCTL-07 (9), DESK-07 (8), UNLK-01 (10), DSHR-01 (2: `internal/tailnet/localapi_test.go`, `internal/desktop/goroutine_leak_test.go`). Includes in-place edited files that carry Phase 23 cases (`desktopClient.test.ts`, `TabBar.test.tsx`, `DesktopTab.test.tsx`, `audit_test.go`, `DesktopSendKeysMenu.test.tsx`). Test names are in Notes, paths are repo-relative only.
- Not duplicated: `encoder_recovery_test.go`, `encoder_wedge_test.go`, `encoder_service_darwin_test.go` already had DSHR-01 rows; M-222 and M-223 already existed.
- `bash tests/check-traceability-paths.sh`: OK, 366 paths exist. `bash tests/check-brand.sh`: clean.

## Task 2: Section 5 and README

### Category BN (Phase 23: DCTL-07, DESK-07, UNLK-01), M-224 to M-234

M-223 was the highest existing, so the new items are **M-224..M-234**, all _Last run:_ not yet run.

| M-NN | Item |
|------|------|
| M-224 | Office-Mac remote unlock, 30 s re-lock, RL-01 with a busy frontmost app (re-lock confirmed by a locked read, or `relock_failed` plus retries, never a false `relocked`), optional wrong try; the ~10 s session-flag lag is stated as expected |
| M-225 | Caps Lock refusal at a real lock screen (VM U-07 PARTIAL) |
| M-226 | Two-Mac text sharpness when still, smooth scrolling, adaptation on a weak link, the lone clean viewer returning to step 0 (finding 2; Q-03-lone2 and Q-03-lone3) |
| M-227 | Two-viewer Slow connection chip on a direct link with the hardware encoder; Q-03r5's known limit (slow viewer told at 15 s against 3 to 10 s, one drop-out from the slow set, light content under 200 kbit/s may never read slow) |
| M-228 | iPhone: unmute tap, Unlock panel above the keyboard in portrait and landscape, no save-password prompt (the iPhone Safari half of P-01), transport feedback |
| M-229 | Chrome on a real Mac: offer carries transport-cc, Unmute, packets rising; the Save password? bubble as the accepted limit (click Never) |
| M-230 | Audio output route change during a share |
| M-231 | Sound item, Unlock panel and chips in light and dark in real windows |
| M-232 | One-hour office-Mac soak with `scripts/desktop-soak.sh` and its pass rule |
| M-233 | Office Mac under heavy load: tailscaled not left dead and not restarted by launchd (23.10.4), GET /desktop reading tailscale off while tailscaled was Running (23.10.5); references M-222 for encoder-wedge recovery on real hardware |
| M-234 | The U-01 wake-key finding feeding 23-15 (no wake key reveals the field, a letter key does) and the ten-cycle retry count |

The category intro maps every PARTIAL, FAIL or NOT-RUN VM row to an item: A-02, Q-01 to M-229; U-07 to M-225; P-01 to M-228 and M-229; Q-03-lone2 to Q-03-lone4 to M-226; Q-03r4, Q-03r5, Q-03r5-light to M-227; EW-01 and the wedge to M-222; RL-01, RL-01r, U-10r to M-223 and M-224; U-01 to M-234; Z-01 to Phase 22's M-215; the AC and AD load findings to M-233; H-01 to H-03 to M-177 and M-232.

From Phase 23 notes: M-161 (hardware readback, `0b4515f1`), M-162 (through the viewer's Unmute with a left-only tone, a 10 minute silent share that keeps audio `ok`, libopus only if it fails), M-172 (rc.1 draft in 23-13, rc.2 repeat, step 4 at the real tag in 23-16), M-177 (rc.1 to rc.2 install in 23-14 with the viewer unmuted, before the publish decision; sound back without a new Unmute; re-lock armed across the upgrade).

### README, section by section

- **Intro:** one added sentence that Portty also shows and controls another Mac's screen.
- **Install:** added "Apple silicon and macOS 14 or later" (matches the release notes line "Apple Silicon and macOS 14 or later only.").
- **Use It From Another Mac:** added the own-account filter (Other Macs lists only your own account's Macs; a Mac shared in from another account is not listed and a by-name connect is refused; tests `TestDiscoverPeers_OwnAccountOnly`, `TestPeerConnect_RefusesHostMissingFromDiscovery`).
- **Desktop (other Mac):** Settings location ("under Sharing", from the Settings strings), and the Desktop tab's speaker mark while sound is on (TabBar `, playing sound`). The Portty-app release chord Ctrl+Option+Cmd+Shift+Return was already right (`APP_RELEASE_CHORD`).
- **New paragraph, Desktop (always available, any browser):** Always-available desktop with Portty quit, the launchd background service and the Needs repair row, Copy Link in Chrome or Safari from your own account, the browser's release chord Ctrl+Option+Cmd+Return, the bar items (View Only, Send Keys, Clipboard, Full Screen), permission setup, the monthly Screen Recording re-approval and the sleep behaviour (from the Settings footnote).
- **Share Desktop (guests):** Admit prompt that names the person, Allow control and Allow clipboard in the Watching list, Disconnect, Stop Sharing and quitting Portty both end the share, join and leave notices, the menu bar icon, the local audit log. The tcp/7444 and udp/7444 sentence is unchanged.
- **Sound and unlock (new, UI-SPEC Surface 5 verbatim):** with the plan's counting sentence inserted directly after the sentence it explains (the pause), not at the paragraph's end, so it reads in order: "Each try can check the password at the Mac twice, because Portty types it once more if the Mac is still locked, so the pause comes after at most six checks at the Mac."
- **Desktop (iPhone):** sound starts muted, tap Unmute in the bottom bar; the Unlock This Mac box works there too.
- **Updating:** Update in the banner downloads, verifies, replaces and restarts; the refusal fallback (says why or shows the verified DMG in Finder); the Apple silicon-only release, the `portty-vX.Y.Z-darwin-arm64.dmg` name and the same file under `-darwin-universal.dmg` kept so pre-v2.0 copies find the update.
- **Known Issues:** Chrome's Save password? bubble (click Never; nothing saved unless Save), what Unlock covers (not the login screen after a restart or with FileVault; emoji and Caps Lock refusals; wrong tries count toward the Mac's own limit), and the slow-link behaviour including the light-content limit.
- **License:** the MIT line and the LICENSE reference are unchanged. No path, address, ts.net name or account name was added; `tests/check-brand.sh` is clean.

Every label was checked against the frontend or Go source (Always-available desktop, Copy Link, Needs repair, Watching, Allow control, Allow clipboard, Admit, Stop Sharing, Quit GUI Only, View Only, Send Keys, Clipboard, Full Screen, Unmute, Slow connection, Unlock This Mac, Tap to Start, Show Keyboard, Update, DMG asset names).

## Deviations from Plan

None to the plan's outcomes. Additions beyond the plan came from the orchestrator additions (eleven M-NN items instead of five, the README review, in-place rows). No 23-UAT.md existed and the plan does not ask for one, so none was created.

## Not Done

- The plan's `read_first` listed `.planning/phases/.../23-VM-CHECKS.md` rows; it was read in full and not edited (another agent may append to it).
- No iPhone or real-Mac claim was made in the README beyond what the code and VM rows support (for example it does not say the Unlock box sits above the keyboard; that is M-228).
- No tests were run (docs only).

## Self-Check

- `bash tests/check-traceability-paths.sh`: OK, 366 paths exist.
- `bash tests/check-brand.sh`: clean.
- 29 Section 4 rows name a Phase 23 requirement or DSHR-01 for the goroutine files; the README paragraph and both acceptance sentences are present once each.
- Commits `bed515ba`, `b15d58e3`, `6e7b1660` are ancestors of HEAD. STATE.md and ROADMAP.md were not touched.

## Self-Check: PASSED
