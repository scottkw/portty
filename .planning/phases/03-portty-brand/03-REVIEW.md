---
phase: 03-portty-brand
reviewed: 2026-11-16T19:29:46Z
depth: standard
files_reviewed: 134
files_reviewed_list:
  - .claude/CLAUDE.md
  - .github/workflows/build.yml
  - .github/workflows/release.yml
  - .gitignore
  - CHANGELOG.md
  - CLAUDE.md
  - README.md
  - TESTING.md
  - about_darwin.go
  - about_darwin.m
  - app.go
  - app_exit_test.go
  - app_set_tray_progress_test.go
  - app_test.go
  - build.sh
  - build/darwin/Info.dev.plist
  - build/darwin/Info.plist
  - cmd/playwright-fixture/main.go
  - cmd_attach.go
  - cmd_attach_resize.go
  - cmd_attach_test.go
  - cmd_cli.go
  - cmd_cli_test.go
  - cmd_daemon.go
  - cmd_daemon_test.go
  - cmd_remote.go
  - cmd_remote_test.go
  - dispatch_test.go
  - frontend/e2e/progress.spec.ts
  - frontend/e2e/web-links-live-toggle.spec.ts
  - frontend/e2e/web-plugin-hot-swap.spec.ts
  - frontend/index.html
  - frontend/package.json
  - frontend/src/App.tsx
  - frontend/src/components/NewSessionModal.tsx
  - frontend/src/components/QuitConfirmModal.tsx
  - frontend/src/components/SettingsSheet.tsx
  - frontend/src/components/Sidebar.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/__tests__/App.nav.test.tsx
  - frontend/src/components/__tests__/App.open-remote.test.tsx
  - frontend/src/components/__tests__/App.test.tsx
  - frontend/src/components/__tests__/NewSessionModal.test.tsx
  - frontend/src/components/__tests__/QuitConfirmModal.test.tsx
  - frontend/src/components/__tests__/RegenerateKeyModal.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx
  - frontend/src/components/__tests__/Sidebar.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.test.tsx
  - frontend/src/components/__tests__/UpdateBanner.test.tsx
  - frontend/src/components/__tests__/style.contrast.test.ts
  - frontend/src/lib/__tests__/remoteSession.test.ts
  - frontend/src/lib/__tests__/sidebarPeers.test.ts
  - frontend/src/lib/__tests__/stripAnsi.test.ts
  - frontend/src/lib/sidebarPeers.ts
  - frontend/src/style.css
  - go.mod
  - internal/attach/attach.go
  - internal/capability/capability_fuzz_test.go
  - internal/capability/capability_test.go
  - internal/capability/joincode_test.go
  - internal/capability/keystore_test.go
  - internal/daemon/api.go
  - internal/daemon/api_disconnect_test.go
  - internal/daemon/api_test.go
  - internal/daemon/chat_removed_test.go
  - internal/daemon/client.go
  - internal/daemon/engine.go
  - internal/daemon/engine_exit_test.go
  - internal/daemon/engine_migration_test.go
  - internal/daemon/engine_test.go
  - internal/daemon/files_removed_test.go
  - internal/daemon/funnel_test.go
  - internal/daemon/plugin_settings.go
  - internal/daemon/process.go
  - internal/daemon/remote_ws_proxy.go
  - internal/daemon/remote_ws_proxy_test.go
  - internal/daemon/service.go
  - internal/daemon/service_test.go
  - internal/daemon/socket.go
  - internal/daemon/socket_test.go
  - internal/daemon/staticapp.go
  - internal/daemon/tailnet_cache.go
  - internal/relay/hub_test.go
  - internal/relay/oscabsorb_relay_test.go
  - internal/relay/scrollback_altscreen_test.go
  - internal/relay/server.go
  - internal/relay/server_test.go
  - internal/statusbar/bar_test.go
  - internal/tailnet/tailnet.go
  - internal/tailnet/tailnet_test.go
  - internal/updater/updater.go
  - internal/updater/updater_test.go
  - internal/webserver/app_bundle_test.go
  - internal/webserver/assets_test.go
  - internal/webserver/auth.go
  - internal/webserver/auth_test.go
  - internal/webserver/brand_pages_test.go
  - internal/webserver/capability_mw.go
  - internal/webserver/capability_test.go
  - internal/webserver/capability_test_helpers.go
  - internal/webserver/find_bar_test.go
  - internal/webserver/findbar_perf_e2e_test.go
  - internal/webserver/fuzz_test.go
  - internal/webserver/inject_test.go
  - internal/webserver/join_test.go
  - internal/webserver/localip_test.go
  - internal/webserver/origin_mw_test.go
  - internal/webserver/oscabsorb_relay_test.go
  - internal/webserver/plugin_config_stream_test.go
  - internal/webserver/probe_marker_test.go
  - internal/webserver/rwgate_test.go
  - internal/webserver/selfcert.go
  - internal/webserver/selfcert_test.go
  - internal/webserver/server.go
  - internal/webserver/server_test.go
  - internal/webserver/session_ended_test.go
  - internal/webserver/web_links_test.go
  - main.go
  - menu_test.go
  - notification_darwin_test.go
  - scripts/verify-icons.sh
  - tests/build-script.test.sh
  - tests/check-brand.sh
  - tests/fixtures/settings_legacy_v4.json
  - tray.go
  - tray_common.go
  - tray_common_test.go
  - tray_objc_darwin.m
  - tray_test.go
  - wails.json
  - web/assets/terminal.js
  - web/dashboard.html
  - web/join.html
  - web/terminal.html
findings:
  critical: 0
  warning: 4
  info: 9
  total: 13
  necessity: 1
status: issues_found
---

# Phase 3: Code Review Report

**Reviewed:** 2026-11-16T19:29:46Z
**Depth:** standard
**Files Reviewed:** 134
**Status:** issues_found

## Summary

I reviewed the diff 136bcb52..HEAD over the 134 listed files at standard depth. Most of the diff is mechanical renaming (module path, strings, comments, tests). I spent most of the time on the areas the orchestrator flagged as high-risk.

Ground truth checked: `go build ./... && go vet ./...` exit 0; `pnpm -C frontend exec tsc --noEmit` exit 0; `go test -short ./internal/tailnet ./internal/updater` passes, as do `./internal/webserver -run ProbeMarker` and `. -run 'Tray|Menu|Quit'`; `bash tests/check-brand.sh` reports clean; `bash scripts/verify-icons.sh` passes 27/27; `bash tests/check-traceability-paths.sh` reports OK.

What I verified and found sound:

- **X-Portty marker placement.** `porttyMarker(ws.requireCapability(ws.handleListSessions))` (`internal/webserver/server.go:828`) sets the header before the cap check, so the unauthenticated 401 carries it. The header grants nothing. `requireCapability` is unchanged, and `http.Error` leaves custom headers alone. The probe (`internal/tailnet/tailnet.go:95-101`) rejects any reply without `X-Portty: 1` and never reads the body. Every caller that feeds the sidebar goes through `DiscoverAndProbe` (`internal/daemon/api.go:1445`). The local-mode basicAuth 401 has no marker, but local mode is never probed over the tailnet.
- **go-selfupdate filter.** I read the v1.5.2 source. With `Filters` set, `findAssetFromRelease` matches only the lowercased asset name against the filters and skips the OS/arch suffix rule. `^portty-v?\d+\.\d+\.\d+.*-darwin-universal\.dmg$` matches the `portty-${GITHUB_REF_NAME}-darwin-universal.dmg` name that release.yml produces. The browser-URL fallback cannot match because of the `^portty` anchor. A nil `Source` falls back to `NewGitHubSource`, which maps a 404 to `(nil, nil)`. `Check` swallows every error. Drafts and prereleases are skipped by default.
- **Config dir and socket.** All three builders (`app.go:803`, `internal/daemon/engine.go:61`, `internal/daemon/socket.go:15`) use `os.UserConfigDir()/portty` with `MkdirAll(…, 0700)`. No tracked code reads the old app's directory.
- **About panel.** `dispatch_async` to the main queue. MRC is balanced: `para`, `link` and `credits` are each released once, after AppKit copies them. The dictionary literals are autoreleased inside the main-queue block, where GCD drains a pool.
- **check-brand.sh.** The negated grep is anchored (`:github\.com/scottkw/portty$`). An empty here-string yields one empty line, which `report` skips. A git error exits 2 instead of passing. The script cannot match its own text.
- **hdiutil.** I tested `hdiutil create -srcfolder X.app` locally: the volume root holds `X.app`, not its `Contents/`.
- **cgo pointers.** The tray `[]byte` arguments hold no Go pointers. Both `.m` entry points copy the bytes into `NSData` before `dispatch_async`.

The four warnings: a slow ObjC leak on the 5 s tray poll that this phase doubled; an About box that shows the inherited `2.2.0` version instead of the release tag; release.yml publishing `-beta`/`-alpha` tags as full releases, which this phase's updater fix now offers as updates; and a leftover PAT used for a downstream workflow this phase deleted.

## Warnings

### WR-01: Tray icon NSData is autoreleased on a Go thread with no pool; it leaks every 5 s, now twice per tick

**File:** `tray_objc_darwin.m:126-127`, `tray_objc_darwin.m:163-164` (called from `tray.go` `updateTray`, driven by `app.go:1286` 5-second ticker via `refreshTrayState` at `app.go:1316`)
**Issue:** `[NSData dataWithBytes:length:]` returns an autoreleased object. `initStatusItem` and `updateTrayIcon` run on whatever Go M thread made the cgo call, and that thread has no `@autoreleasepool`. The ObjC runtime then creates an implicit per-thread pool that drains only when the thread exits, and Go M threads almost never exit. The dispatched block retains each `NSData` while it runs, but the autorelease reference is never balanced, so every `NSData` lives forever. `updateTray` calls `updateTrayIcon` on every 5 s poll whether or not the state changed. Before this phase that leaked one ~425 B `NSData` per tick. Now it leaks two (~425 B plus ~780 B), about 21 MB/day for an app built to stay up overnight. The pattern predates this phase, but the @2x change doubled the allocation and the leak.
**Fix:** Drain a pool around the copies. The block still keeps the data alive, because a copied block retains captured ObjC objects even under MRC:
```objc
void updateTrayIcon(const void *d1, int n1, const void *d2, int n2) {
    @autoreleasepool {
        NSData *data1 = [NSData dataWithBytes:d1 length:n1];
        NSData *data2 = [NSData dataWithBytes:d2 length:n2];
        dispatch_async(dispatch_get_main_queue(), ^{
            if (statusItem == nil) return;
            statusItem.button.image = makeTrayImage(data1, data2);
        });
    }
}
```
Do the same in `initStatusItem`. Also skip the cgo call in `updateTray` when the `(connected, quartile)` pair has not changed since the last call. Do not wrap `setTraySessionData` the same way without first fixing IN-01, because it relies on this leak.

### WR-02: About box shows "Version 2.2.0" (the upstream app's version) instead of the release tag

**File:** `about_darwin.m:42`, `wails.json:14`, `build/darwin/Info.plist:15-19`, `.github/workflows/release.yml:80-82`
**Issue:** The new About panel takes its version from `CFBundleShortVersionString`, which Wails renders from `wails.json` `productVersion` (`2.2.0`, inherited unchanged). release.yml stamps only `-X main.Version=${{ github.ref_name }}`. So a tagged Portty build such as `v0.1.0` shows "Version 2.2.0" in About, while the update banner and `daemon.BuildVersion` report `v0.1.0`. BRND-01 lists the About box as a Portty identity surface. RESEARCH defers versioning to Phase 6, but release.yml is live now, and any tag pushed before Phase 6 ships a DMG with a wrong, AgentHub-era version in About. That same stale version also sits in `CFBundleVersion`, which Finder's Get Info and notarization records display.
**Fix:** Stamp the plist version from the tag in release.yml before `wails build` (and in build.sh from `VERSION`):
```yaml
- name: Stamp bundle version from tag
  run: |
    V="${GITHUB_REF_NAME#v}"
    jq --arg v "$V" '.info.productVersion = $v' wails.json > wails.json.tmp && mv wails.json.tmp wails.json
```
Or at least set `productVersion` to a Portty-era placeholder (`0.0.0`) so the stale upstream number never ships.

### WR-03: Pre-release tags other than `-rc` publish as full releases, which the fixed updater now offers to every user

**File:** `.github/workflows/release.yml:252`, `internal/updater/updater.go:65-75`
**Issue:** `draft: ${{ contains(github.ref, '-rc') }}` is the only gate, and `prerelease:` is never set. A tag like `v1.2.0-beta.1` or `v1.2.0-alpha` (both match `on.push.tags: 'v*'`) therefore becomes a normal, non-prerelease GitHub release with a DMG named `portty-v1.2.0-beta.1-darwin-universal.dmg`. Before this phase the updater could never match a `.dmg`, so this did no harm. With the new `Filters` setting, which accepts `\d+\.\d+\.\d+.*`, `DetectLatest` returns that beta, because go-selfupdate skips only releases flagged `prerelease`. `semver` ranks `1.2.0-beta.1` above `1.1.x`, so every stable user gets an update banner pointing at a beta.
**Fix:** Mark any hyphenated tag as a prerelease (and keep `-rc` as draft if that is still wanted):
```yaml
draft: ${{ contains(github.ref_name, '-rc') }}
prerelease: ${{ contains(github.ref_name, '-') }}
```
Optionally tighten `dmgAssetFilter` to `^portty-v?\d+\.\d+\.\d+-darwin-universal\.dmg$` so a hyphenated build can never match.

### WR-04: Phantom requirement — RELEASE_PUBLISH_TOKEN PAT kept for a downstream workflow this phase deleted

**File:** `.github/workflows/release.yml:253-257`
**Category:** necessity
**Issue:** The PAT existed only so the `release.published` event would trigger `distribute.yml`, because releases published with `GITHUB_TOKEN` do not fire workflows. This phase deletes `distribute.yml` (DIST-03), and no workflow still triggers on that event: `.github/workflows/` now holds only `build.yml`, `e2e.yml` and `release.yml`, and none of them has a `release:` or `workflow_run` trigger. The requirement has lapsed, as the repository shows. Keeping it means the publish job runs with a long-lived personal token, a broader credential than the job-scoped `GITHUB_TOKEN` it already has `contents: write` for, and gains nothing in return. CLAUDE.md asks to keep the security posture tight around releases.
**Fix:** Use `token: ${{ secrets.GITHUB_TOKEN }}` (or drop the `token:` line; the action defaults to it), remove the stale comment, and delete the `RELEASE_PUBLISH_TOKEN` repo secret.

## Info

### IN-01: `setTraySessionData` stores autoreleased arrays in globals without retaining them (latent use-after-free; pre-existing, outside the diff)

**File:** `tray_objc_darwin.m:196-207`
**Issue:** Under MRC, `[NSMutableArray arrayWithCapacity:]` objects go into `menuSessionNames` and `menuSessionIDs` without a `retain`, and the previous arrays are never released. This works today only because of the never-drained Go-thread pool described in WR-01. If someone fixes WR-01 by wrapping this function in `@autoreleasepool`, or the Go M thread exits, the globals dangle and `menuWillOpen:` crashes.
**Fix:** In the main-queue block, `[menuSessionNames release]; menuSessionNames = [nameArr retain];` (same for IDs), then wrap the copying in `@autoreleasepool`.

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

**File:** `.github/workflows/release.yml:201`, `README.md:12`
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
**Issue:** The old binary names were dropped from `.gitignore`, and `git status` shows `?? agenthub`. A `git add -A` would commit a large binary. The brand gate would then fail CI on the filename.
**Fix:** Delete the stale binary, or keep `/agenthub` ignored until no local copies remain.

### IN-09: The brand gate's link check is case-sensitive and misses other GitHub URL forms

**File:** `tests/check-brand.sh:55-57`
**Issue:** The check covers only lowercase `github.com/scottkw/<repo>`. It does not catch `raw.githubusercontent.com/scottkw/…`, `api.github.com/repos/scottkw/…`, `scottkw.github.io`, or mixed-case hosts, so a link to another of the author's repos in those forms would pass BRND-06. The old-name check (`-i`) still catches any AgentHub repo URL, so the practical risk is low.
**Fix:** Add `-i` and widen the pattern, for example `'(github\.com|githubusercontent\.com|github\.com/repos)/scottkw/[A-Za-z0-9_.-]+'`, keeping the `portty$` allow rule case-insensitive.

---

_Reviewed: 2026-11-16T19:29:46Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
