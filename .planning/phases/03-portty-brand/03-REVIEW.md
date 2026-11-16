---
phase: 03-portty-brand
reviewed: 2026-11-16T19:40:10Z
depth: standard
iteration: 2
files_reviewed: 4
files_reviewed_list:
  - tray_objc_darwin.m
  - .github/workflows/release.yml
  - wails.json
  - TESTING.md
findings:
  critical: 0
  warning: 0
  info: 11
  total: 11
  necessity: 0
status: clean
---

# Phase 3: Code Review Report (iteration 2)

**Reviewed:** 2026-11-16T19:40:10Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** clean (no critical or warning findings remain; 11 info items are open)

## Summary

This pass re-reviews the fix commits df89629b..684bae6f (`git diff df89629b~1`) in the four files they touched. It checks that WR-01..WR-04 and IN-01 are actually fixed and that the fixes added no new bugs. Info items IN-02..IN-09 from iteration 1 were out of the fixer's scope. They are copied below unchanged.

Ground truth: `go build ./... && go vet ./...` exit 0. `bash tests/check-traceability-paths.sh` reports OK (106 paths). I ran the stamp step's shell and jq logic locally on a copy of wails.json: `v1.2.0`, `v1.2.0-beta.1` and `v1.2.0-rc1` give `1.2.0`; `v1.2`, `vfoo` and `v1.2.3+meta` are rejected; the `©` in `copyright` survives the jq rewrite. actionlint is not installed, so I did not lint the workflow.

### Iteration-1 findings: verification

| ID | Status | Evidence |
|----|--------|----------|
| IN-01 | Fixed | `tray_objc_darwin.m:215-221`. The main-queue block releases the previous arrays, then stores `[nameArr retain]` and `[idArr retain]`. Every call makes fresh arrays, so the old and new objects can never be the same, and the release cannot free the array being stored. `[nil release]` on the first call does nothing. All reads (`menuWillOpen:`) and writes run on the serial main queue, so there is no race. |
| WR-01 | Fixed | `tray_objc_darwin.m:128,168,181,207`. `initStatusItem`, `updateTrayIcon`, `updateTrayTooltip` and `setTraySessionData` each wrap their bodies in `@autoreleasepool`. The file is compiled without ARC: `tray.go:6` passes only `-x objective-c`, and the explicit `retain`/`release` calls would not compile under ARC. Without ARC, `dispatch_async` copies the block before it returns, the copy retains every captured object pointer (`data1`, `data2`, `tip`, `nameArr`, `idArr`), and block disposal releases them once. So the objects outlive the pool drain and nothing is released twice. `makeTrayImage` returns an autoreleased image on the main queue, where GCD drains a pool. `removeStatusItem` and `setDockVisible` autorelease nothing on the cgo thread, so they need no pool. |
| WR-02 | Fixed | `wails.json:14` is now `0.1.0`. `release.yml:83-88` stamps `info.productVersion` from the tag before `wails build`. `build/darwin/Info.plist:15,19` fill both `CFBundleVersion` and `CFBundleShortVersionString` from `{{.Info.ProductVersion}}`. Quoting is correct: `"${GITHUB_REF_NAME#v}"` and `"${V%%-*}"` are quoted, the `[[ =~ ]]` pattern is unquoted as it must be, and `--arg` passes the value to jq as a string, not as jq code. The default `bash -e` shell on macOS runners supports `[[ ]]`. jq ships on `macos-latest`. If jq fails, the `&&` skips the `mv`, and the line's non-zero status is the script's exit status, so the step fails. No tracked file outside `.planning/` still mentions `2.2.0` as the app version. TESTING.md M-72 is updated to match. |
| WR-03 | Fixed | `release.yml:262,265`. `draft: contains(github.ref_name, '-rc')` and `prerelease: contains(github.ref_name, '-')`. Any hyphenated tag becomes a prerelease, which the updater skips. An `-rc` tag is both a draft and a prerelease, which is harmless. |
| WR-04 | Fixed | `release.yml:255-269`. The `token:` input is gone, so softprops falls back to `github.token`, and the publish job keeps `contents: write` (line 232). Someone with repo access still needs to delete the `RELEASE_PUBLISH_TOKEN` secret by hand. |

I found no new critical or warning issues. The fixes leave three minor items, listed as IN-10..IN-12.

## Info

### IN-02: Sidebar "none" placeholder text no longer matches its meaning

**File:** `frontend/src/components/Sidebar.tsx:14`, `frontend/src/lib/sidebarPeers.ts:28-34`
**Issue:** Once the filter was removed, `kind: 'none'` means "no other Portty Macs were found". A reachable peer with nothing shared now gets its own "Nothing shared" row. The placeholder still says "No shared sessions on other Macs.", which is now misleading.
**Fix:** Reword it, for example "No other Macs running Portty on your tailnet."

### IN-03: Empty "This Mac" exposes two identical "New Session" buttons

**File:** `frontend/src/components/Sidebar.tsx:96`, `frontend/src/components/Sidebar.tsx:110`
**Issue:** When there are no sessions, `.sidebar__group--empty` forces the header "+" visible, and the row button is rendered too. Both are named "New Session" with `aria-keyshortcuts="Meta+T"`, so VoiceOver lists two identical controls one after the other, and `getByRole('button', { name: 'New Session' })` becomes ambiguous.
**Fix:** Hide the header "+" (or remove it from the accessibility tree) while the empty-state row is shown, or give the two buttons distinct names.

### IN-04: Dev plist now shares the release bundle ID

**File:** `build/darwin/Info.dev.plist:12-13`
**Issue:** Before this phase, dev builds used `com.wails.{{.Name}}`, distinct from release. Now both `wails dev` and the installed app are `app.portty.Portty`. They therefore share WKWebView storage (the `portty:lastWorkDir` localStorage key), notification authorization, and LaunchServices identity, so `open -b app.portty.Portty` may launch the dev copy.
**Fix:** Use `app.portty.Portty.dev` in `Info.dev.plist`, unless a shared identity is intended (for example for notification testing). If it is, note that in the plist.

### IN-05: DMG has no Applications alias, but README says to drag Portty to Applications

**File:** `.github/workflows/release.yml:211`, `README.md:12`
**Issue:** `hdiutil create -srcfolder build/bin/Portty.app` produces a volume holding only `Portty.app` (verified locally), with no `/Applications` drop target. Also, `hdiutil create/attach` print deprecation warnings on this macOS (use `diskutil image`). It still works on `macos-latest` today.
**Fix:** Stage a folder first: `mkdir dmg && cp -R build/bin/Portty.app dmg/ && ln -s /Applications dmg/Applications && hdiutil create -volname Portty -srcfolder dmg …`.

### IN-06: build.yml comment says the check name "stays stable", but this phase changed it

**File:** `.github/workflows/build.yml:10-11`
**Issue:** Renaming the matrix entry changes the required check context from `build (agenthub, …)` to `build (portty, …)`. The comment says the name stays stable. CONTEXT notes that branch protection must be re-applied by hand, but the comment does not.
**Fix:** Reword it: "kept as a matrix so the required check context is `build (portty, darwin/universal, macos-latest)`; renaming the entry requires re-applying branch protection (TESTING.md §3)."

### IN-07: X-Portty marker is visible on the public internet while Funnel is on

**File:** `internal/webserver/server.go:475`, `internal/webserver/server.go:828`
**Issue:** The Funnel loopback listener serves the same `ws.mux`, so `GET /api/sessions` with no cap answers internet clients with `401` plus `X-Portty: 1`, which fingerprints the host as Portty. It reveals nothing beyond what the Funnel page already does, and Phase 4 removes Funnel.
**Fix:** None needed if Phase 4 removes Funnel as planned. Otherwise, emit the marker only when the request arrived on the tailnet listener.

### IN-08: `/agenthub` is no longer gitignored, and an untracked `agenthub` binary sits in the repo root

**File:** `.gitignore:2`
**Issue:** The old binary names were dropped from `.gitignore`, and `git status` still shows `?? agenthub`. A `git add -A` would commit a large binary. The brand gate would then fail CI on the filename.
**Fix:** Delete the stale binary, or keep `/agenthub` ignored until no local copies remain.

### IN-09: The brand gate's link check is case-sensitive and misses other GitHub URL forms

**File:** `tests/check-brand.sh:55-57`
**Issue:** The check covers only lowercase `github.com/scottkw/<repo>`. It does not catch `raw.githubusercontent.com/scottkw/…`, `api.github.com/repos/scottkw/…`, `scottkw.github.io`, or mixed-case hosts, so a link to another of the author's repos in those forms would pass BRND-06. The old-name check (`-i`) still catches any AgentHub repo URL, so the practical risk is low.
**Fix:** Add `-i` and widen the pattern, for example `'(github\.com|githubusercontent\.com|github\.com/repos)/scottkw/[A-Za-z0-9_.-]+'`, keeping the `portty$` allow rule case-insensitive.

### IN-10: The new pool comment says every Go-called entry point drains a pool; some still do not, and the file has other MRC leaks

**File:** `tray_objc_darwin.m:122-124` (comment), `tray_objc_darwin.m:231-233`, `tray_objc_darwin.m:247-255`, `tray_objc_darwin.m:268`, `tray_objc_darwin.m:35,45,56`, `tray_objc_darwin.m:143`
**Issue:** The comment added in a573d088 says "Entry points called from Go wrap their bodies in @autoreleasepool". `sendNotification` (lines 252-254 autorelease three `NSString`s on the cgo thread) and `hasValidBundleIdentifier` (called from Go via `notification_darwin.go:26`) do not. The file also leaks objects it allocates outright: `UNMutableNotificationContent` (line 268, never released), the three kinds of `NSMenuItem` allocated in `menuWillOpen:` (lines 35, 45, 56; the menu retains them, and the `alloc` reference is never released, so every menu open leaks one item per row), and the one-time `NSMenu` in `initStatusItem` (line 143). None of these is on the 5 s poll. Notifications fire only on quit-to-tray (`app.go:342`), and menu items leak only when the user opens the menu, so the real cost is tiny. All of it predates this phase, but the new comment now claims a rule that the file does not follow.
**Fix:** Wrap the bodies of `sendNotification` and `hasValidBundleIdentifier` in `@autoreleasepool`. Add `[item release]` (and the same for `openItem` and `quitItem`) after each `addItem:`, `[content release]` after building `req`, and `[menu release]` after `statusItem.menu = menu`. Or narrow the comment to "the tray entry points".

### IN-11: A local `build.sh` build shows "Version 0.1.0" in About whatever `BUILD_VERSION` is

**File:** `build.sh:75-78`, `wails.json:14`
**Issue:** WR-02's fix stamps the plist only in release.yml. `build.sh` still puts `BUILD_VERSION` (default `dev`) only into `-X main.Version`. So `BUILD_VERSION=v0.3.0 ./build.sh` produces an app whose About box says 0.1.0 while the update banner and `daemon.BuildVersion` say v0.3.0. The fixer chose this on purpose, and M-72 documents it. It only affects local builds.
**Fix:** If local builds should match, apply the same stamp in `build.sh` when `BUILD_VERSION` matches `^v?[0-9]+\.[0-9]+\.[0-9]+`, writing to a temporary copy of wails.json or restoring it afterwards so the working tree stays clean.

### IN-12: The build step pastes `${{ github.ref_name }}` into the shell script, and the new stamp check does not cover it

**File:** `.github/workflows/release.yml:91-93`
**Issue:** `-ldflags "-X main.Version=${{ github.ref_name }}"` expands the tag into the script text before bash runs. Git tag names may contain `$`, `(`, `)`, `;` and backticks, so a tag such as `v1.2.3-$(cmd)` would run `cmd` inside the double quotes. The new stamp step does not stop this, because it strips everything after the first `-` before checking the format. The job has `id-token: write` and `attestations: write`. Only someone who can already push tags can do this, so it is hardening, not a vulnerability. It predates this phase. The stamp step and the DMG step already read the tag through the safe `$GITHUB_REF_NAME` environment variable.
**Fix:** Use the environment variable here too: `-ldflags "-X main.Version=${GITHUB_REF_NAME}"`. Optionally, validate the whole tag in the stamp step, for example `[[ "$GITHUB_REF_NAME" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]`.

---

_Reviewed: 2026-11-16T19:40:10Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
_Iteration: 2_
