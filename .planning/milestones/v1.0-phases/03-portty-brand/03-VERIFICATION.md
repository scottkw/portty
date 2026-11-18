---
phase: 03-portty-brand
verified: 2026-11-16T19:47:40Z
status: human_needed
score: 7/8 must-haves verified
covered_files:
  - ".claude/CLAUDE.md"
  - ".github/workflows/build.yml"
  - ".github/workflows/release.yml"
  - ".planning/phases/03-portty-brand/03-01-PLAN.md"
  - ".planning/phases/03-portty-brand/03-01-SUMMARY.md"
  - ".planning/phases/03-portty-brand/03-02-PLAN.md"
  - ".planning/phases/03-portty-brand/03-02-SUMMARY.md"
  - ".planning/phases/03-portty-brand/03-03-PLAN.md"
  - ".planning/phases/03-portty-brand/03-03-SUMMARY.md"
  - ".planning/phases/03-portty-brand/03-04-PLAN.md"
  - ".planning/phases/03-portty-brand/03-04-SUMMARY.md"
  - ".planning/phases/03-portty-brand/03-05-PLAN.md"
  - ".planning/phases/03-portty-brand/03-05-SUMMARY.md"
  - ".planning/phases/03-portty-brand/03-06-PLAN.md"
  - ".planning/phases/03-portty-brand/03-06-SUMMARY.md"
  - "CHANGELOG.md"
  - "CLAUDE.md"
  - "README.md"
  - "TESTING.md"
  - "about_darwin.go"
  - "about_darwin.m"
  - "app.go"
  - "app_exit_test.go"
  - "app_set_tray_progress_test.go"
  - "app_test.go"
  - "build.sh"
  - "build/darwin/Info.dev.plist"
  - "build/darwin/Info.plist"
  - "cmd/playwright-fixture/main.go"
  - "cmd_attach.go"
  - "cmd_attach_resize.go"
  - "cmd_attach_test.go"
  - "cmd_cli.go"
  - "cmd_cli_test.go"
  - "cmd_daemon.go"
  - "cmd_daemon_test.go"
  - "cmd_remote.go"
  - "cmd_remote_test.go"
  - "dispatch_test.go"
  - "frontend/e2e/progress.spec.ts"
  - "frontend/e2e/web-links-live-toggle.spec.ts"
  - "frontend/e2e/web-plugin-hot-swap.spec.ts"
  - "frontend/index.html"
  - "frontend/package.json"
  - "frontend/src/App.tsx"
  - "frontend/src/components/NewSessionModal.tsx"
  - "frontend/src/components/QuitConfirmModal.tsx"
  - "frontend/src/components/SettingsSheet.tsx"
  - "frontend/src/components/Sidebar.tsx"
  - "frontend/src/components/TerminalPanel.tsx"
  - "frontend/src/components/__tests__/App.nav.test.tsx"
  - "frontend/src/components/__tests__/App.open-remote.test.tsx"
  - "frontend/src/components/__tests__/App.test.tsx"
  - "frontend/src/components/__tests__/NewSessionModal.test.tsx"
  - "frontend/src/components/__tests__/QuitConfirmModal.test.tsx"
  - "frontend/src/components/__tests__/RegenerateKeyModal.test.tsx"
  - "frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx"
  - "frontend/src/components/__tests__/Sidebar.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.test.tsx"
  - "frontend/src/components/__tests__/UpdateBanner.test.tsx"
  - "frontend/src/components/__tests__/style.contrast.test.ts"
  - "frontend/src/lib/__tests__/remoteSession.test.ts"
  - "frontend/src/lib/__tests__/sidebarPeers.test.ts"
  - "frontend/src/lib/__tests__/stripAnsi.test.ts"
  - "frontend/src/lib/sidebarPeers.ts"
  - "frontend/src/style.css"
  - "go.mod"
  - "internal/attach/attach.go"
  - "internal/capability/capability_fuzz_test.go"
  - "internal/capability/capability_test.go"
  - "internal/capability/joincode_test.go"
  - "internal/capability/keystore_test.go"
  - "internal/daemon/api.go"
  - "internal/daemon/api_disconnect_test.go"
  - "internal/daemon/api_test.go"
  - "internal/daemon/chat_removed_test.go"
  - "internal/daemon/client.go"
  - "internal/daemon/engine.go"
  - "internal/daemon/engine_exit_test.go"
  - "internal/daemon/engine_migration_test.go"
  - "internal/daemon/engine_test.go"
  - "internal/daemon/files_removed_test.go"
  - "internal/daemon/funnel_test.go"
  - "internal/daemon/plugin_settings.go"
  - "internal/daemon/process.go"
  - "internal/daemon/remote_ws_proxy.go"
  - "internal/daemon/remote_ws_proxy_test.go"
  - "internal/daemon/service.go"
  - "internal/daemon/service_test.go"
  - "internal/daemon/socket.go"
  - "internal/daemon/socket_test.go"
  - "internal/daemon/staticapp.go"
  - "internal/daemon/tailnet_cache.go"
  - "internal/relay/hub_test.go"
  - "internal/relay/oscabsorb_relay_test.go"
  - "internal/relay/scrollback_altscreen_test.go"
  - "internal/relay/server.go"
  - "internal/relay/server_test.go"
  - "internal/statusbar/bar_test.go"
  - "internal/tailnet/tailnet.go"
  - "internal/tailnet/tailnet_test.go"
  - "internal/updater/updater.go"
  - "internal/updater/updater_test.go"
  - "internal/webserver/app_bundle_test.go"
  - "internal/webserver/assets_test.go"
  - "internal/webserver/auth.go"
  - "internal/webserver/auth_test.go"
  - "internal/webserver/brand_pages_test.go"
  - "internal/webserver/capability_mw.go"
  - "internal/webserver/capability_test.go"
  - "internal/webserver/capability_test_helpers.go"
  - "internal/webserver/find_bar_test.go"
  - "internal/webserver/findbar_perf_e2e_test.go"
  - "internal/webserver/fuzz_test.go"
  - "internal/webserver/inject_test.go"
  - "internal/webserver/join_test.go"
  - "internal/webserver/localip_test.go"
  - "internal/webserver/origin_mw_test.go"
  - "internal/webserver/oscabsorb_relay_test.go"
  - "internal/webserver/plugin_config_stream_test.go"
  - "internal/webserver/probe_marker_test.go"
  - "internal/webserver/rwgate_test.go"
  - "internal/webserver/selfcert.go"
  - "internal/webserver/selfcert_test.go"
  - "internal/webserver/server.go"
  - "internal/webserver/server_test.go"
  - "internal/webserver/session_ended_test.go"
  - "internal/webserver/web_links_test.go"
  - "main.go"
  - "menu_test.go"
  - "notification_darwin_test.go"
  - "scripts/verify-icons.sh"
  - "tests/build-script.test.sh"
  - "tests/check-brand.sh"
  - "tray.go"
  - "tray_common.go"
  - "tray_common_test.go"
  - "tray_objc_darwin.m"
  - "tray_test.go"
  - "wails.json"
  - "web/assets/terminal.js"
  - "web/dashboard.html"
  - "web/join.html"
  - "web/terminal.html"
covered_digest: "v4:sha256:b1cd622812bd5ac389c0cd1773a9ef48d5200666ce8d58ca26d165e13cf8b18f"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "M-73: With a Developer-ID-signed Portty.app, choose Quit GUI Only with 0, 1 and 2 sessions open."
    expected: "First use prompts notification permission for Portty; each notification is attributed to Portty with the Portty icon and body 'Portty is still running in the menu bar.' / '... with 1 session open.' / '... with 2 sessions open.'. Finder, Dock and Cmd-Tab show Portty."
    why_human: "UNUserNotificationCenter attribution comes from the signed bundle at runtime; an unbundled or unsigned process skips the native path by design (hasAppBundleID). Copy is covered by TestQuitToMenuBarBody."
  - test: "M-70: Run Portty on two Macs on one tailnet (one optionally also running the upstream app on 7443)."
    expected: "Each Portty lists the other under Other Macs (probe on 7444 sees X-Portty: 1); a Mac running only the upstream app is not listed; both apps run side by side on one Mac without a port collision."
    why_human: "Needs two real tailnet nodes. Probe classification is unit-tested (TestProbePeer_* and TestProbeMarker_*), but live tsnet discovery cannot be exercised locally."
  - test: "M-74: View README.md on github.com/scottkw/portty with the light theme, then the dark theme."
    expected: "Light theme shows docs/portty-title-logo.png, dark theme shows docs/portty-title-logo-dark.png, 400 px wide, alt text Portty."
    why_human: "GitHub's <picture>/prefers-color-scheme rendering is only visible on github.com; the repo is private."
audit_acknowledged:
  milestone: v1.0
  at: 2026-11-18
  status: human_needed
---

# Phase 3: Portty Brand Verification Report

**Phase Goal:** Everything a user sees or types says Portty: app, CLI, paths, icons, docs, notifications, and update feed
**Verified:** 2026-11-16T19:47:40Z
**Status:** human_needed
**Re-verification:** No, initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Built app is Portty.app with a Portty bundle ID; window title, menu bar, About box and tray tooltip say Portty | ✓ VERIFIED | `build/bin/Portty.app/Contents/Info.plist` (PlistBuddy): CFBundleName/DisplayName `Portty`, CFBundleIdentifier `app.portty.Portty`, executable `Contents/MacOS/Portty`, GetInfoString and MIT copyright as specified. `wails.json` name/outputfilename `Portty`. `main.go:71` Title `Portty`; app menu About Portty / Settings… / Hide Portty / Quit Portty, Help menu = Portty Help + Check for Updates (TestAppMenu_Structure, _HelpOpensPorttyURL, _AboutOpensNativePanel pass with -count=1). `tray_common.go` trayTooltip `Portty — no sessions / 1 session / N sessions` (TestTrayTooltip). Human-observed in VM: menu bar reads Portty; tray menu Open Portty / session / Quit Portty. `about_darwin.m` calls orderFrontStandardAboutPanelWithOptions with the credits and portty.app link. M-72 observed in the Tart VM 2026-11-16 (opened via an Accessibility AXPress helper): native panel with the Portty icon, "Portty", one "Version 0.1.0" line, the description, a clickable portty.app link (opens https://portty.app in Safari) and the MIT copyright, readable in Light and Dark. |
| 2 | App icon, tray icons (progress quartiles and error) and title logos (light and dark) are the `portty-brand/` assets | ✓ VERIFIED | Git blob comparison against the deleted `portty-brand/` tree at `728977b3^`: all 12 tray PNGs (6 states x 1x/@2x), build/appicon.png, assets/appicon.png, all 10 iconset PNGs and all 6 title-logo copies are byte-identical. `scripts/verify-icons.sh` 27/27 PASS (sizes, icns >100 KB, generators and portty-brand/ gone). `tray.go` embeds all 12 files; `makeTrayImage` builds one template NSImage with 18 px and 36 px reps at 18x18 pt. Built bundle ships `Resources/iconfile.icns`. VM: tray shows the new "p" icon in Light/Dark. |
| 3 | CLI is `portty` (`portty daemon`, `portty attach`), Go module is portty, config/data/socket in portty-named locations, never AgentHub's | ✓ VERIFIED | `go.mod`: `module github.com/scottkw/portty`. `filepath.Join(base, "portty")` under os.UserConfigDir() in socket.go:19, engine.go:65, app.go:807 (CLI, daemon and GUI agree). `cmd_cli.go` usage `Usage: portty [command] [flags]`; `unknownCommandMessage` wired at main.go:241 (TestRunCLI_UnknownCommand, TestDispatchHelp pass). `daemon` and `attach` dispatch in runCLI. Service `portty-daemon` / `Portty Daemon` / `Keeps Portty terminal sessions running in the background.` (TestNewServiceConfig_Fields). Old name: zero hits anywhere outside .planning, so no path to the old directory can exist. Port 7444; `7443` has zero tracked hits. |
| 4 | README, Help, Settings and other user-facing text contain no AgentHub or agent-CLI wording and describe Portty as a terminal with remote access | ✓ VERIFIED | README rewritten per UI-SPEC (logo picture block, lead line, Install, Use It From Another Mac, License). `git grep -i agent` over frontend/src, web, README, CHANGELOG, docs (non-test) returns only `magenta` palette keys. Help menu and in-app help link both open `https://portty.app` (helpURL, HELP_URL). Web pages: TestBrandPages_DashboardSaysPortty / _JoinSaysPortty / _TerminalPageSaysPortty / _NoAgentWording pass. Quit dialog `Quit Portty?`, document title Portty, `portty:lastWorkDir`, realm `Portty`, cert org `Portty Local`. |
| 5 | Native notifications appear under the Portty name; update check queries Portty's release feed and handles "no release yet" without error | ? UNCERTAIN (notification attribution only) | Update feed verified: `app.go` `updateSlug = "scottkw/portty"` used by both runUpdateCheck and CheckForUpdates; `dmgAssetFilter` matches the release.yml DMG name; go-selfupdate v1.5.2 GitHubSource maps 404 to nil (github_source.go:75), and `updater.Check` swallows errors/not-found (TestCheck_NotFound, TestCheck_DetectError, TestDetectLatestFrom_NoReleases, _SkipsDraftAndPrerelease pass). Notification path: QuitGUIOnly sends `portty.quit-gui-only` / title `Portty` / quitToMenuBarBody(n) through UNUserNotificationCenter; attribution under the Portty name needs a signed bundle (M-73). |
| 6 | Root LICENSE is still MIT; README and frontend/package.json declare MIT | ✓ VERIFIED | `LICENSE` has no diff vs main (last touched a000c63a), "MIT License / Copyright (c) 2026 Ken Scott". README `MIT. See [LICENSE](LICENSE).`; `frontend/package.json` `"license": "MIT"`. |
| 7 | `packaging/homebrew` is gone and no docs, scripts or workflows mention Homebrew | ✓ VERIFIED | `packaging/` does not exist; workflows are build.yml, e2e.yml, release.yml (distribute.yml deleted). Release DMG built with `hdiutil create` and signed with codesign. The only Homebrew mentions in tracked files are `internal/daemon/path.go` and its test (runtime PATH for the Tailscale binary, not distribution; sanctioned by plan 03-06 and excepted by check-brand.sh). |
| 8 | Case-insensitive search for "agenthub" over tracked files (excluding .planning/) returns zero hits; links point to portty.app or github.com/scottkw/portty | ✓ VERIFIED | `git grep -n -i -I 'agent[h]ub' -- ':!.planning'` exit 1, no output; no tracked file name contains it. `tests/check-brand.sh`: `OK: brand gate clean`, wired into build.yml line 27. The only `github.com/scottkw/` references are `scottkw/portty`; external links in README/CHANGELOG/frontend are `https://portty.app`. |

**Score:** 7/8 truths verified (0 present, behavior-unverified). Truths 1 and 5 are verified for everything but one runtime-rendered surface each, which needs a human (M-72, M-73).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `go.mod` | module github.com/scottkw/portty | ✓ VERIFIED | Line 1 |
| `internal/daemon/socket.go`, `engine.go`, `app.go` | portty config dir | ✓ VERIFIED | All three join `"portty"` under UserConfigDir |
| `cmd_cli.go` | usage + unknownCommandMessage | ✓ VERIFIED | Wired from main.go:241 |
| `internal/daemon/service.go` | portty-daemon service | ✓ VERIFIED | Lines 55-57 |
| `tests/fixtures/settings_legacy_v4.json` | renamed fixture | ✓ VERIFIED | Tracked; no old-name file names |
| `internal/tailnet/tailnet.go` | port 7444, X-Portty marker check | ✓ VERIFIED | `DefaultProbePort = 7444`, `isPorttyProbeResponse` |
| `internal/webserver/server.go` | marker on GET /api/sessions only | ✓ VERIFIED | `porttyMarker(ws.requireCapability(...))` line 828 |
| `internal/updater/updater.go` | DMG filter, 404-silent detect | ✓ VERIFIED | Filters + detectLatestFrom |
| `tray.go` / `tray_objc_darwin.m` | @2x embeds, makeTrayImage | ✓ VERIFIED | 12 embeds; template image with two reps; @autoreleasepool on Go-called entry points (WR-01/IN-01) |
| `build/darwin/iconfile.icns`, iconset, appicon | Portty icons | ✓ VERIFIED | Byte-identical to brand source; verify-icons PASS |
| `wails.json`, `build/darwin/Info*.plist` | Portty bundle | ✓ VERIFIED | Rendered bundle confirmed with PlistBuddy |
| `about_darwin.go` / `about_darwin.m` | native About panel | ✓ VERIFIED (code) | Wired through menuShowAbout seam; rendering is M-72 |
| `tray_common.go` | tooltip, menu labels, quit notification | ✓ VERIFIED | Unit-tested |
| `web/*.html`, `auth.go`, `selfcert.go` | Portty web surfaces | ✓ VERIFIED | brand_pages_test passes; CSP asserted |
| `tests/check-brand.sh` | brand gate | ✓ VERIFIED | Clean; CI step present |
| `.github/workflows/release.yml` | Portty.app, portty-notarize, hdiutil DMG, icns copy | ✓ VERIFIED | Lines 85-212; version stamped from GITHUB_REF_NAME; prerelease on hyphenated tags |
| `README.md`, `CHANGELOG.md`, `frontend/package.json` | rewritten, MIT | ✓ VERIFIED | See truths 4 and 6 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| main.go runCLI | socket.go DefaultSocketPath | EnsureDaemon(DefaultSocketPath()) | ✓ WIRED | main.go:204 |
| runCLI default branch | unknownCommandMessage | fmt.Fprint(os.Stderr, ...) | ✓ WIRED | main.go:241 |
| tailnet probePeer | webserver GET /api/sessions | HTTPS :7444 + X-Portty | ✓ WIRED | Both sides use tailnet.MarkerHeader/Value |
| daemon process.go | tailnet.DefaultProbePort | AutoStart/RestartWebServer | ✓ WIRED | process.go:50, 126, 137 |
| app.go update checks | updater.Check → DefaultDetect | updateSlug | ✓ WIRED | app.go:1349, 1373 |
| assets/tray_icon*@2x.png | tray.go | go:embed | ✓ WIRED | Compiles; 12 embeds |
| tray.go | makeTrayImage | cgo initStatusItem/updateTrayIcon | ✓ WIRED | VM: tray icon and menu survive 4+ polls |
| main.go About Portty | about_darwin.m showAboutPanel | menuShowAbout → showNativeAboutPanel | ✓ WIRED | TestAppMenu_AboutOpensNativePanel |
| app.go QuitGUIOnly | quitToMenuBarBody | sendNotificationFunc | ✓ WIRED | app.go:342 |
| wails.json | build/bin/Portty.app/Contents/MacOS/Portty | Wails packager | ✓ WIRED | Built bundle inspected |
| build.yml | tests/check-brand.sh | `run: bash tests/check-brand.sh` | ✓ WIRED | build.yml:27 |
| release.yml DMG name | updater dmgAssetFilter | `portty-${GITHUB_REF_NAME}-darwin-universal.dmg` | ✓ WIRED | `portty-v1.2.3-darwin-universal.dmg` matches `^portty-v?\d+\.\d+\.\d+.*-darwin-universal\.dmg$` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| UpdateBanner | update.releaseURL | updater.Check → GitHub releases of scottkw/portty | Yes (built from slug + detected version) | ✓ FLOWING |
| Tray tooltip / quit notification body | session count | refreshTrayState from daemon ListSessions | Yes | ✓ FLOWING |
| About panel version/name/copyright | Info.plist | Wails-rendered bundle plist | Yes (release stamps productVersion from the tag) | ✓ FLOWING |

### Behavioral Spot-Checks and Ground Truth Commands

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go build, vet, race tests | `go build ./... && go vet ./... && go test -race -short ./...` | exit 0, 11 packages ok (the known flake did not fire) | ✓ PASS |
| TypeScript | `pnpm -C frontend exec tsc --noEmit` | exit 0 | ✓ PASS |
| Vitest | `pnpm -C frontend exec vitest run` | 91 files, 1173 tests passed | ✓ PASS |
| Brand gate | `bash tests/check-brand.sh` | `OK: brand gate clean` | ✓ PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | `OK: 106 traceability paths exist` | ✓ PASS |
| Icons | `bash scripts/verify-icons.sh` | 27 passed, 0 failed | ✓ PASS |
| Zero-hit search | `git grep -n -i -I 'agent[h]ub' -- ':!.planning'` | no output (exit 1) | ✓ PASS |
| Named tests, uncached | `go test -count=1 -run 'TestDispatchHelp$\|TestRunCLI_UnknownCommand\|TestProbeMarker_\|TestBrandPages_\|TestDetectLatestFrom_NoReleases\|TestCheck_NotFound\|TestQuitToMenuBarBody\|TestTrayTooltip'` plus tailnet, menu, service tests | all PASS | ✓ PASS |
| Asset provenance | git blob hash of each asset vs `728977b3^:portty-brand/...` | 30/30 MATCH | ✓ PASS |
| Built bundle | PlistBuddy on build/bin/Portty.app | Portty / app.portty.Portty / Portty executable | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` probes are declared by this phase. Step 7c: not applicable.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| BRND-01 | 03-04, 03-05 | App named Portty: bundle name/ID, title, menu bar, About, tray tooltip | ? NEEDS HUMAN (About panel only) | Truth 1; M-72 |
| BRND-02 | 03-03 | Icons and logos from portty-brand/ | ✓ SATISFIED | Truth 2 (byte-identical) |
| BRND-03 | 03-01, 03-02, 03-05 | CLI, module, dirs, socket use portty | ✓ SATISFIED | Truth 3 |
| BRND-04 | 03-05, 03-06 | README/help/in-app text describe Portty, no agent wording | ✓ SATISFIED | Truth 4 |
| BRND-05 | all plans | Zero old-name hits outside .planning | ✓ SATISFIED | Truth 8; check-brand.sh in CI |
| BRND-06 | 03-02, 03-04, 03-05, 03-06 | Links point to portty.app / scottkw/portty | ✓ SATISFIED | Truth 8; helpURL, HELP_URL, updateSlug, About credits |
| NOTF-01 | 03-02, 03-04 | Notifications under the Portty name | ? NEEDS HUMAN | Truth 5; M-73 |
| DIST-03 | 03-06 | No Homebrew distribution | ✓ SATISFIED | Truth 7 |

No orphaned requirements: every ID REQUIREMENTS.md maps to Phase 3 is claimed by at least one plan.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (phase-modified files) | - | TBD / FIXME / XXX | - | None found |
| `frontend/public/portty-title-logo*.png`, `frontend/src/assets/portty-title-logo*.png` | - | Assets not referenced by any code | ℹ️ Info | The old-name copies were also unreferenced before the phase; kept by CONTEXT decision. Only `docs/` copies are used (README). Candidate for a later cut. |
| `main.go` | 203-241 | Unknown command runs EnsureDaemon before printing the error | ℹ️ Info | `portty bogus` starts the daemon, then prints the correct Portty message. Pre-existing order, not a brand regression. |
| `notification_darwin.go` | 38-40 | Comment cites notification_windows.go / notification_linux.go, which do not exist | ℹ️ Info | Stale comment only |
| `.planning/REQUIREMENTS.md` | 14 | BRND-05 checkbox unchecked while the traceability table says Complete | ℹ️ Info | Bookkeeping; BRND-01/02 also still Pending pending M-71/M-72 |
| Repo settings | - | `RELEASE_PUBLISH_TOKEN` secret (REVIEW-FIX WR-04) | ℹ️ Info | The workflow no longer uses it; the user should delete the secret in GitHub settings |

### Human Verification Required

#### 1. About Portty panel (M-72)

**Test:** On the host, launch build/bin/Portty.app, choose App menu > About Portty in Light, then Dark; choose it again.
**Expected:** Native panel with the Portty icon, "Portty", one "Version 0.1.0" line, the description, "portty.app" (record whether it is a clickable link), and "Copyright © 2026 Ken Scott. MIT License."; the second click reuses the panel.
**Why human:** AppKit draws it at runtime and synthetic clicks in the VM can't open the app menu.

#### 2. Notifications under the Portty name (M-73)

**Test:** With a Developer-ID-signed build, Quit GUI Only with 0, 1 and 2 sessions.
**Expected:** Permission prompt names Portty; notifications show the Portty icon and name with the 0/1/n bodies.
**Why human:** Attribution comes from the signed bundle; unbundled processes skip the native path.

#### 3. Two-Mac peer discovery on 7444 (M-70)

**Test:** Run Portty on two tailnet Macs (optionally the upstream app on one).
**Expected:** Each lists the other; upstream-only hosts are not listed; no port collision side by side.
**Why human:** Needs two real tailnet nodes.

#### 4. README logo on GitHub (M-74)

**Test:** View the README on github.com in light and dark themes.
**Expected:** Light/dark logos switch, 400 px, alt Portty.
**Why human:** Only visible on github.com (private repo).

#### 5. Tray progress and error states (M-71 residual)

**Test:** Drive progress 25/50/75/100 with the OSC 9;4 fixture, then the error state; check highlighted.
**Expected:** Portty mark in every state, crisp, tinted correctly.
**Why human:** Only the idle icon was observed in the VM. Low risk: art is byte-identical and goes through the same code path.

### Gaps Summary

No automatable gaps. Every ground truth command passes, the zero-hit search returns nothing, the built bundle is Portty.app with bundle ID app.portty.Portty, and every icon and logo is byte-identical to the deleted `portty-brand/` source. What is left are surfaces that only render at runtime: the About panel (M-72), notification attribution on a signed build (M-73), live two-Mac discovery (M-70), the GitHub README header (M-74), and the non-idle tray states (M-71 residual). The status is human_needed, not passed, until those are checked.

---

_Verified: 2026-11-16T19:47:40Z_
_Verifier: Claude (gsd-verifier)_


## Orchestrator follow-up (2026-11-16, Tart VM)

- **M-72 passed:** About Portty opened via `axmenu app.portty.Portty Portty "About Portty"`; contents as specified; link clickable; Light and Dark readable.
- **M-71 passed:** idle "p" icon; 50% progress ring (Progress plugin on, OSC 9;4;1;50) with crisp @2x art and the tab underline at 50%; error icon ("p" with "!") after stopping the daemon; tray menu works across 4+ polls after the WR-01/IN-01 memory fixes. 25/75/100 use the same code path with byte-identical art.
- **Remaining human items:** M-70 (two Macs), M-73 (Developer-ID-signed build), M-74 (README on github.com; repo private).
- **Noted for follow-up (not a Phase 3 criterion):** the Settings Progress plugin row still says "Default OFF in v3.2 — flips ON in v3.3 after field validation." (inherited AgentHub version wording).
