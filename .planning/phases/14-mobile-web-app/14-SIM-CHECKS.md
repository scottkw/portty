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
- The screenshot directory above was deleted by 14-14 on 2026-11-22 after all 10 files matched the sha256 list above.

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
| S-02 | PASS | SC1 keyboard inset in Safari: with the software keyboard up, the toolbar's bottom edge is within 8 pt above the keyboard's top edge and not below it; the terminal input element sits above the toolbar; Esc keeps its size (no focus zoom); landscape too | Portrait, keyboard up: keyboard (0,583 402x233), toolbar (0,417 402x49), so the bar's bottom edge (466) is 117 pt above the keyboard's top edge (583): not within 8 pt, though not below it. Input element `.xterm-helper-textarea` "Terminal input" (8,66 384x10) bottom 76, above the toolbar top 417: pass. Esc 51x44 before focusing and 51x44 after: no focus zoom, pass. Landscape, keyboard up: keyboard (75,238 724x162), toolbar (0,121 874x49) bottom 170, gap 68 pt; input (8,68 600x10) bottom 78 above the toolbar top 121: pass. Back to portrait with the keyboard still up: toolbar (0,449 402x49), keyboard (0,583 402x233), gap 85 pt (the bar sat 32 pt lower than on the first portrait pass, so the settled position is not identical). What fills the gap: the screenshots show Safari's own iOS 26 floating URL pill ("127.0.0.1") and its form-assist bar (up, down, check) between the key bar and the keyboard; Safari shrinks the page's visual viewport to end above them, so the bar sits directly above that Safari chrome rather than directly above the keys. The key bar and the cursor line stay visible and above all of it, nothing is hidden, but the literal 8 pt criterion fails; S-05 repeats the measurement in the standalone view (no URL pill) and 14-14 should judge whether the gap matters on the real phone. Screenshots `S02-portrait.png`, `S02-landscape.png`, `S02-portrait-again.png`. **Measured result unchanged: the literal 8 pt criterion failed as written above. accepted limit, user decision 2026-11-22:** "the gap is iOS's own form-assist bar (⌃ ⌄ ✓), which iOS shows above the keyboard for any web text field even in the standalone app; Portty's key bar sits directly on top of it (confirmed in the user's real-iPhone screenshot, standalone)". |
| S-03 | PASS | SC1 pinch: the terminal font changes with the pinch, not the page zoom, with one grid claim per pinch (resize count) | Pinch out (`pinch(withScale: 2.0, velocity: 1.0)` on the web view): Esc width 51 before and 51 after (no page zoom, within 1 pt); fixture grid resize 3 to 4 (exactly one resize), columns 48 to 20, rows 5 to 16. Pinch in: scale 0.5 with velocity -1.0 on the whole web view opened Safari's tab overview twice (not the page's font change, no grid change, resizes stayed 4), and 0.8 on the whole web view changed nothing (cols 20, resizes 4); the same pinch-in on the terminal element ("tab panel", 402x714, scale 0.7, velocity -0.5) gave resize 4 to 5 (exactly one), columns 20 to 28, rows 16 to 24. So both directions claim exactly once when the fingers start on the terminal; the whole-web-view pinch-in's fingers start near the screen edges and Safari's own pinch gesture took it, which the rig cannot tell apart from a real thumb and finger, so a pinch-in that opens the tab overview is a real-phone look for 14-14. Screenshots `S03-pinch-out.png`, `S03-pinch-in-panel.png` (`S03-pinch-in.png` and `S03-pinch-in-08.png` show the tab overview and the no-op) |
| S-04 | PASS | M-145, SC2, decisive: Add to Home Screen from the share link in Safari, then launch the Portty icon from SpringBoard. Did the launch keep `session` and `cap`? | Query kept. Flow on iOS 26.5: More (`MoreMenuButton`), Share, View More, the `Add to Home Screen` cell. The Add sheet's name field value was "Portty" (the sheet header shows the manifest icon and name) and "Open as Web App" switch value 1 (on). After Add the test terminated Safari. info-hits read N = 8, still 8 after Safari was gone. The icon was not on the Home Screen page (`icons["Portty"]` existed but had a 0x0 frame there); one swipe left to the App Library, where the "Portty" icon (frame 306,188 68x91) was tapped. Within 6 s info-hits was 9 (N+1): the launch URL carried both `session` and `cap`. The standalone view showed the key bar ("Terminal keys", Esc at 0,792 51x44) over the full 402x874 window, no Safari address bar and no Link No Longer Works. Standalone process: `UIKitApplication:com.apple.webapp`, bundle identifier `com.apple.webapp`, which XCUITest could query directly (so S-05..S-07 observations are direct, not indirect). Screenshots `S04-more.png`, `S04-share.png`, `S04-add-sheet.png`, `S04-springboard.png`, `S04-applibrary.png`, `S04-standalone.png` |
| S-05 | PASS | Standalone safe areas in light and dark, with the armed Ctrl readable in both | Window 402x874. Light and dark gave identical frames: terminal panel (0,62 402x812), so its top edge (62) sits below the status bar; toolbar (0,787 402x87) and the keys at y=792..836, so the keys' bottom edge is 38 pt above the window bottom (at least 20 pt) and the toolbar's padding fills the home-indicator band. Ctrl tapped: switch value 1 in both appearances; the screenshots show the accent-blue fill with white label and white dot, readable on the light and dark toolbar; tapped again: value 0. Extra, keyboard up in the standalone view: keyboard (0,583 402x233), toolbar (0,449 402x49), the bar's bottom 85 pt above the keyboard (the same Safari form-assist bar gap as S-02, here without the URL pill), the input element (bottom 76) above the toolbar; the key bar is still not directly above the keys. Screenshots `S05-light-armed.png`, `S05-light-cancelled.png`, `S05-dark-armed.png`, `S05-dark-cancelled.png`, `S05-standalone-keyboard.png` |
| S-06 | PASS | Open PDF's way back in the standalone view (T-14-41): Done is visible below the status bar and returns without a relaunch; old path in a Safari tab | POST `/__test__/preview-pdf` gave 200 `{"viewers":1}`; the strip showed "Session" and "fixture.pdf" tabs with the "Preview opened: fixture.pdf" status. On the fixture.pdf tab, Open PDF (154,504 94x28) opened 14-08's in-app layer: title "fixture.pdf", "Open full PDF" and "Done" in a bar below the status bar, Done at (354,62 49x44) and hittable; one image element "fixture.pdf" (0,107 402x402), one page, drawn blank because the fixture's 329-byte PDF has no visible marks. Done tapped: the preview pane returned (Open PDF present) and the Session tab showed the key bar again, the standalone app stayed running (state 4), no relaunch. In a Safari tab: Open PDF (154,455 94x28) opened a new Safari tab on the PDF with iOS's "1 of 1" page badge, the "Back to Portty" breadcrumb and Safari's back button and address bar, so the old path leaves the session tab but has a way back through Safari's own chrome. Screenshots `S06-preview.png`, `S06-pdf-tab.png`, `S06-pdf-layer.png`, `S06-after-done.png`, `S06-safari-open-pdf.png` |
| S-07 | PASS | Stopped, Reload and the icon dying (SC2) | With the session showing in the standalone view (key bar present), POST `/__test__/share-off` gave 204; within the poll the banner read "fixture-host stopped sharing this session." with a "Reload" button (297,122 57x44, at least 44 x 44) and a dismiss X (`S07-stopped-banner.png`). Reload tapped: the view showed "Link No Longer Works", no key bar (`S07-after-reload.png`). Standalone app terminated and the icon launched again from the App Library: "Link No Longer Works", no key bar, info-hits stayed 10 (the old token fails the capability check, as in the spike) (`S07-icon-relaunch.png`) |
| S-08 | PASS | Ended, Reload | POST `/__test__/share-on` gave a new capability, written to a 0600 link file and not printed; the new link opened in Safari showed the key bar (info-hits 11). POST `/__test__/end` gave 204; the banner read "Session ended — the owner stopped this session." with a "Reload" button (297,70 57x44) and a dismiss X, the key bar still shown below the empty terminal (`S08-ended-banner.png`) |

Notes for 14-14 (what the rig could not or did not settle):

- The key bar sits above Safari's own iOS 26 form-assist bar (and URL pill in Safari) rather than directly above the keyboard, 117 pt up on the first portrait pass and 85 pt up elsewhere (S-02, S-05). Nothing is hidden; whether the gap matters on the real phone is a look for 14-14.
- Pinch-in with whole-web-view fingers opened Safari's tab overview in the simulator (S-03). A real two-finger pinch on the terminal is a real-phone item; the standalone app has no tab overview.
- The fixture serves no terminal output and its PDF is blank, so no screenshot shows terminal text or PDF marks; font changes (S-03) are evidenced by the grid readings only, and the two pinch-in screenshots are byte-identical.
- The icon is only in the App Library on a fresh device (S-04); a real phone's "newly downloaded apps" setting may differ.
- The simulator's keyboard shows iOS's one-time "slide to type" tip over the keys the first time it re-appears; the rig dismissed it with Continue where it blocked a step. Real-phone first use may show it too.
- Not covered by the rig: real suspension and resume (WebKit bug 308073), cellular or Wi-Fi roaming, the Tailscale app's VPN, MOB-03's reconnect, and the real-hardware M-145 look.

## Screenshots

Not committed. The directory (`/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/p14-shots.TJxqBvzGDp`) was deleted by 14-14 on 2026-11-22 after all 30 files matched the sha256 list below (see `## Cleanup (14-14)` in 14-VM-CHECKS.md). The Safari address bar, the Add sheet and the share sheet show the fixture host and port (and the Add sheet a truncated link), which is why they stay out of git. File, then first 12 hex of sha256:

| File | sha256 |
|------|--------|
| dbg1.png (Safari first-launch start page, debug) | a3b7f98e873a |
| dbg2.png (first load with the tip popover, debug) | f0b460df290a |
| S01-final.png | 7ca4f1b03408 |
| S02-portrait.png | b0190dcaad53 |
| S02-landscape.png | f7ddd58e2af0 |
| S02-portrait-again.png | 1e557fb21721 |
| S03-pinch-out.png | 66abc072a778 |
| S03-pinch-in.png (tab overview) | 4989627ad713 |
| S03-pinch-in-08.png | 52f9716601f4 |
| S03-pinch-in-panel.png | 52f9716601f4 |
| S04-more.png | e65f1ca3ad89 |
| S04-share.png | ba1b3d46aa4c |
| S04-add-sheet.png | 3a74ee58193c |
| S04-springboard.png | 30265e4fc0ad |
| S04-applibrary.png | b419e278b93a |
| S04-standalone.png | 2333397bba30 |
| S05-light-armed.png | 9b5052d8bbfa |
| S05-light-cancelled.png | e1471727871e |
| S05-dark-armed.png | 2df0b5e282c4 |
| S05-dark-cancelled.png | 23756ed66ad4 |
| S05-standalone-keyboard.png | d74e9045dcec |
| S06-preview.png | ea09243b3109 |
| S06-pdf-tab.png | e973a08e3368 |
| S06-pdf-layer.png | 4a61f6dfc0f8 |
| S06-after-done.png | 4cbb2b3622e7 |
| S06-safari-open-pdf.png | ac84c42ed67e |
| S07-stopped-banner.png | 5bdc4ff59c88 |
| S07-after-reload.png | a099924f5413 |
| S07-icon-relaunch.png | ff6a092f3d88 |
| S08-ended-banner.png | d190ce639e19 |

## Cleanup

- The fixture process was stopped (`pgrep -f` for it found nothing) and the device `portty-p14` was shut down and deleted with `simctl shutdown` and `simctl delete`; `simctl list devices` shows no `portty-p14`. The trusted CA and the Home Screen icon went with it.
- The temp directory (fixture binary, CA PEM, 0600 link files, XCUITest project, DerivedData, result bundles, logs) was deleted. `cmd/playwright-fixture/dist` is the gitignored build output and is not staged.
- `pgrep -x Simulator` was empty at the start, after boot, after S-01 and at the end; no host defaults, product code or packages were changed (only `pnpm install --frozen-lockfile --prefer-offline`, a no-op).
- Rig fixes: one (the first run raced Safari's first-launch start page, see S-00). XCUITest gotchas that cost a retry each: Ctrl is a Switch, not a Button; a fresh test method needs `activate()` before querying the app.
