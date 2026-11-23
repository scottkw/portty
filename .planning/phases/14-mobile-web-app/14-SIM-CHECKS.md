# Phase 14 Simulator Checks

Date: 2026-11-22. Base commit: `c19dfc20` (branch `gsd/v1.2`). Xcode 27.2 (27B52). Runtime: iOS 26.5 (23F77), `com.apple.CoreSimulator.SimRuntime.iOS-26-5`. Device type: iPhone 17, one throwaway device named `portty-p14-spike`, deleted at the end of the run.

Rules this file was produced under:

- **Headless.** The device was created and started with `xcrun simctl boot` plus `bootstatus -b`. Simulator.app was never opened and no host mouse, keyboard or AppleScript events were sent. `pgrep -x Simulator` was empty at the start, after boot and at the end.
- **Loopback and CA only.** The share view was served by the Playwright fixture (`playwrightfixture,wailsassets` build) on `https://127.0.0.1`. Its CA was trusted with `simctl keychain add-root-cert` on the throwaway device only. Nothing touched tailscaled or a 100.x address.
- **Throwaway runner.** An XCUITest project generated with xcodegen in a temp directory (empty SwiftUI host app plus a UI-test bundle, ad-hoc simulator signing) drove `com.apple.mobilesafari` and `com.apple.springboard`. It was never committed and was deleted with the temp directory.
- **No secrets.** The fixture link and its token lived only in a 0600 file in the temp directory and are not written here. Screenshots are not committed (Safari's address bar and the Add sheet show the fixture host and port).
- **Fidelity caveat.** The simulator has iOS 26.5's WebKit and SpringBoard but no real suspension, cellular, roaming or Tailscale VPN. MOB-03 (reconnect after a real suspend) and WebKit bug 308073 therefore stay real-phone items.

## Home Screen spike (14-07)

| Check | Result | What was checked | Evidence |
|-------|--------|------------------|----------|
| H-00 | PASS | Preflight: Xcode, runtime, device, headless boot, fixture, CA, runner | Xcode 27.2 (27B52); runtime iOS 26.5 (23F77); device type iPhone 17; device created and booted by `simctl boot` (bootstatus finished); `pgrep -x Simulator` empty at start, after boot and at end; fixture printed READY=1 and wrote its CA PEM; `simctl keychain add-root-cert` succeeded; runner built (`build-for-testing`) after one build fix (Swift 5 language mode, to avoid actor-isolation errors in the test class). The UI-navigation fixes that followed (Safari 26's Share lives in the More menu, Add to Home Screen is behind View More and is a cell, the new icon lives in the App Library) were element-lookup fixes, not rig failures |
| H-01 | PASS | M-145 spike: Add to Home Screen from the share link in Safari, then launch the Portty icon from SpringBoard. Did the launch keep `session` and `cap`? | Query kept. Safari opened the link and the page loaded (info-hits 8 to 9). Share is under the "More" menu, then View More, then Add to Home Screen. The Add sheet's name field read "Portty" and "Open as Web App" was on (switch value 1). After Add the test terminated Safari, waited 4 s, and read info-hits N = 9. Tapping the "Portty" icon gave info-hits 10 (N+1) within 10 s, so the standalone launch itself called the info route, which only happens when both `session` and `cap` are in the launch URL. Screenshot `06-after-icon-launch.png` shows the share view (Session tab) with no Safari address bar and no Link No Longer Works. Standalone process: `UIKitApplication:com.apple.webapp` (bundle identifier `com.apple.webapp`), present in `launchctl list` after launch and absent before. An earlier identical run without terminating Safari also gave N to N+1 (7 to 8), but its screenshots were not kept |

### Extra check requested by the checkpoint caller (not part of H-01)

| Step | Result | Evidence |
|------|--------|----------|
| Share turned off, icon relaunched | Link No Longer Works | `POST /__test__/share-off` returned 204; web app terminated and the icon tapped again; screenshot `07-icon-after-share-off.png` shows "Link No Longer Works"; info-hits stayed at 10 (the fixture counts only requests that pass the capability check, so the old token no longer passes) |
| Share turned on again (new token), same icon relaunched | Still Link No Longer Works | `POST /__test__/share-on` returned 200 with a new token; icon relaunched; screenshot `08-icon-after-share-on-old-token.png` is byte-identical to 07; info-hits stayed at 10 |

So the icon carries the token it was added with. It survives only while that same share stays on; stopping and restarting sharing, which issues a new token, needs a fresh Add to Home Screen. This is a property of the locked design (token in the page URL, no `start_url`), not a failure of H-01.

### Other observations

- On a fresh iOS 26.5 device the new icon was **not** placed on the Home Screen page. It appeared only in the App Library, in an automatic "Webclips" folder (SpringBoard needed one swipe left to reach it). A real phone's "newly downloaded apps" setting may differ.
- iOS 26 Safari shows a one-time tip popover ("View Bookmarks, Share Menu, and Open Tabs") that must be dismissed; Share is no longer a toolbar button.
- The fixture serves no terminal output, so the share view in 06 is the Session tab over an empty terminal area; this spike judges the launch URL, not the finished UI (14-13 repeats it).
- The Add sheet showed the manifest icon and the name "Portty" (from the manifest and `apple-mobile-web-app-title`).

Screenshots (not committed), directory `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/p14-spike-shots.DL4VYuu3Bn` (sha256, first 12 hex):

| File | sha256 |
|------|--------|
| 01-safari-loaded.png | 6852bc999df5 |
| 02-more-menu.png | d7a370b5dca5 |
| 03-after-share-tap.png | 6471152062a1 |
| 03b-view-more.png | faca22413759 |
| 04-add-sheet.png | 0883c5151237 |
| 05-springboard.png | 74b0bdbe8149 |
| 05b-springboard-where-icon-is.png | d1213cf2bfdb |
| 06-after-icon-launch.png | ecad0b8481b7 |
| 07-icon-after-share-off.png | 82b16ce3b8a0 |
| 08-icon-after-share-on-old-token.png | 82b16ce3b8a0 |

## Spike cleanup

- The device was shut down and deleted (`simctl shutdown`, `simctl delete`); `simctl list devices` shows no `portty-p14-spike`. The CA trust and the icons went with it.
- The fixture process was stopped; the temp directory (fixture binary, CA PEM, 0600 link file, XCUITest project, derived data, logs) was deleted.
- `pgrep -x Simulator` was empty at the end; no host defaults were changed; no product code was changed.
- `cmd/playwright-fixture/dist` (gitignored) now holds a fresh copy of `frontend/dist`; it is not staged.
- The screenshot directory above is kept for the checkpoint; 14-14 deletes it with the other screenshots.

## Spike decision

**Decision:** continue

Date: 2026-11-22. The user's word: "continue". They accepted that the Home Screen icon works while the share it was added from stays on, and that after the share is stopped and started again (new token) the icon must be added again from the new link.

What it sets:

- 14-09, 14-10 and 14-11 run next. No redesign; `/gsd-discuss-phase 14` is not needed for the Home Screen flow.
- 14-14 asks for one real-phone look at M-145 (not the full M-145, because the rig answered H-01).
- The re-add-after-re-share behaviour is accepted as the current design. The deferred "icon that survives re-sharing: token-less `start_url` plus same-account token fetch" stays deferred.

Notes the spike leaves for later plans:

- **14-09 (launcher copy).** Safari on iOS 26 has no toolbar Share button. Share is under the "..." More menu, and "Add to Home Screen" is behind View More in the share sheet. Safari also shows a one-time tip popover first. The launcher text should say the icon stops working when sharing is turned off and must be added again from the new link.
- **14-13 (full simulator run).** Repeat S-04 with these labels: More (`MoreMenuButton`), Share, View More, the `Add to Home Screen` cell. On a fresh device the new icon is not put on the Home Screen. It lands only in the App Library, in an automatic "Webclips" folder, one swipe left of the Home Screen. Add share-off and re-share rows for the icon (it showed Link No Longer Works both times in this spike).
- **14-14 (real phone).** Say where the icon lands on the phone (Home Screen page or App Library only), and check the re-add-after-re-share behaviour once there.

## Full simulator run (14-13)

Date: 2026-11-22 (local; fixture started 2026-11-23T01:59:54Z). Base commit: `4fb792bc` (branch `gsd/v1.2`). Host macOS 27.2.1, Xcode 27.2 (27B52). Runtime: iOS 26.5 (23F77), `com.apple.CoreSimulator.SimRuntime.iOS-26-5`. Device type: iPhone 17 (402 x 874 pt, scale 3), one throwaway device named `portty-p14`, created for this run and deleted at its end (see Cleanup).

Rules this run was produced under (same as the spike above): headless `xcrun simctl boot` plus `bootstatus -b` (Simulator.app never opened, no host mouse, keyboard or AppleScript events); the share view served by the Playwright fixture on `https://127.0.0.1` with its per-run CA trusted on the throwaway device only; a throwaway XCUITest project (xcodegen, empty SwiftUI host plus a UI-test bundle, ad-hoc signing, Swift 5 language mode) driving `com.apple.mobilesafari` and `com.apple.springboard`; the fixture link lives only in a 0600 temp file and is not written here; screenshots are not committed.

Fidelity caveat (for 14-14's review): the simulator runs iOS 26.5's WebKit and SpringBoard but not real suspension, cellular, roaming or the Tailscale app's VPN, so MOB-03 (reconnect after a real suspend) and WebKit bug 308073 (WebSockets stuck CONNECTING after a resume) stay real-phone items.

| Check | Result | What was checked | Evidence |
|-------|--------|------------------|----------|
| S-00 | PASS | Preflight: Xcode, runtime, device type, headless boot, `pgrep -x Simulator`, fixture READY, CA, runner, keyboard bridge, WebKit build | Xcode 27.2 (27B52); runtime iOS 26.5 (23F77); device type iPhone 17; UDID created with `simctl create` and booted with `simctl boot`, `bootstatus -b` exit 0; `pgrep -x Simulator` empty at the start, after boot and after S-01; fixture printed BASE_URL, CAP, ADMIN_URL, CA_PEM and READY=1 (CA PEM 0600, 562 bytes, no key); `simctl keychain add-root-cert` exit 0 and Safari then loaded the HTTPS page without a certificate warning; runner built on the first `build-for-testing` (TEST BUILD SUCCEEDED, no fix needed). First run found Safari's first-launch start page (onboarding card) instead of the link, because `system.open` raced Safari's first launch; one `simctl openurl` nudge on the throwaway device loaded the page, and every later run opened the link directly (one rig fix, not a failure). Hardware keyboard: none bridged (headless boot), and the software keyboard shows on focus (`keyboards.firstMatch.exists` true). WebKit build: not visible (the fixture does not log the user agent; the device runs the iOS 26.5 runtime's WebKit) |
| S-01 | PASS | SC1 key bar in Safari: eight keys at least 44 x 44 pt in one row; Esc reaches the shell; Ctrl then c from the software keyboard gives 0x03 and Ctrl returns to off; typed `--`, `"` and a double space arrive as plain ASCII | Toolbar "Terminal keys" frame (0,723 402x53); eight keys found by accessibility name, frames in pt: Esc 51x44 at x=0, Tab 51x44 at 50, Ctrl 51x44 at 100 (a Switch, value 0 or 1), Left arrow 52x44 at 150, Up arrow 51x44 at 201, Down arrow 51x44 at 251, Right arrow 51x44 at 301, Vertical bar 52x44 at 351; all at y=728, so one row, minimum 51 x 44. Esc tap: input route hex `1b`. Ctrl tap: Ctrl value 1 (armed); `c` tapped on the software keyboard: hex `03`, Ctrl value back to 0. Typed `a--` gave `612d2d`, typed `"` gave `22`, typed `b` plus two spaces gave `622020` (no smart punctuation, no double-space period). Screenshot `S01-final.png` (software keyboard up, key bar above it, cursor line at the top) |
