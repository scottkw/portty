# Phase 3: Portty Brand - Research

**Researched:** 2026-11-16
**Domain:** Repo-wide rebrand of a Go/Wails v2/React macOS app (module path, bundle identity, CLI, on-disk paths, network identity, assets, docs, CI)
**Confidence:** HIGH (nearly every claim was checked against repo source, the Wails/go-selfupdate module source, the macOS SDK headers, or on-disk artifacts this session)

## Summary

This phase is a rename with five small behaviour changes: port 7443 → 7444, a Portty-only probe marker header, the update-feed slug, a native About panel, and Retina (@2x) tray icons. The rename is large but mechanical. 613 lines in 117 tracked files outside `.planning/` match "agenthub" case-insensitively, plus 5 tracked **filenames**. 57 `.go` files import `github.com/scottkw/agenthub/...`. A dry run (sed the import path, then `gofmt -l` each result) showed that the module rename adds **zero** gofmt diffs. The one file `gofmt -l` flags, `internal/relay/scrollback_altscreen_test.go`, is already unformatted on the current tree.

Six findings change what the plan should do:
1. **Wails names the `.app` after `wails.json` `name`, not `outputfilename`.** The UI-SPEC row "`outputfilename` / `name` = `Portty` / `portty`" would produce `build/bin/portty.app`, not `Portty.app`. Set **both** to `Portty`.
2. **The config dir is not `~/.config/agenthub`.** On macOS `os.UserConfigDir()` returns `~/Library/Application Support`, and the live dir is `~/Library/Application Support/agenthub/`. Keep `os.UserConfigDir()` and change only the leaf to `"portty"`. Don't hard-code `~/.config`.
3. **Wails ignores `build/darwin/iconfile.icns`.** It builds the bundle's `Resources/iconfile.icns` from `build/appicon.png` (`icns.Encode`). The real source of truth for the app icon is `build/appicon.png`. Still run the locked `iconutil` step, because `scripts/verify-icons.sh` checks that file, but don't rely on it.
4. **Homebrew was the only thing that put the CLI on PATH.** The cask's `binary` stanza symlinked `Contents/MacOS/AgentHub` to `agenthub`. With Homebrew gone, nothing installs `portty` on PATH. The README needs a one-line symlink step, or the user has to decide another way.
5. **The update check can never find a DMG release as written.** go-selfupdate only matches asset names ending in `darwin[-_]arm64` plus `.zip`/`.tar.gz`/… (no `.dmg`). So the UI-SPEC "newer release → banner" row can never fire. The "no release / 404 → silent" half already works (verified in code). A one-line `Filters` config fixes discovery.
6. **`release.yml` runs `brew install create-dmg`.** That is a Homebrew mention in a workflow (DIST-03). Replace it with the built-in `hdiutil create` plus `codesign`.

**Primary recommendation:** Split the work into five sequential plans that keep `go build ./... && go vet ./... && go test -race -short ./...` and vitest green at every boundary: (1) module path + paths + CLI/service names, (2) port 7444 + X-Portty marker + update feed, (3) brand assets + @2x tray loading, (4) app identity, in-app text, About panel and notifications, (5) docs, CI, Homebrew removal, TESTING.md and the zero-hit gate.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Identity and storage
- Bundle ID is `app.portty.Portty` (matches the portty.app domain). App is `Portty.app`; `wails.json` productName Portty.
- Fresh start: Portty never reads, imports or writes AgentHub settings or data (`~/.config/agenthub`). No migration code.
- Config, data and the daemon socket live under `~/.config/portty/` (same layout as AgentHub, renamed). Go module path `github.com/scottkw/portty`.
- An AgentHub daemon that is still running is left alone. Portty runs its own `portty daemon`; both apps can run side by side.

#### Running alongside AgentHub on the tailnet
- Web/peer port moves from 7443 to 7444 (`internal/tailnet/tailnet.go` DefaultProbePort, webserver bind, docs, tests), so both apps can run on one Mac and AgentHub is never probed as a peer.
- The probe reply carries a Portty-only marker (e.g. an `X-Portty` response header). Peer discovery accepts only replies with that marker; the shared "capability required" text alone no longer qualifies.
- Peers still running AgentHub are not shown at all.
- Old saved links and capabilities are not carried over (follows the fresh start).

#### Icons and logos
- Import `portty-brand/` per its README: copy over repo paths, rename `*-2x.png` to `*@2x.png`, regenerate `build/darwin/iconfile.icns` with `iconutil`, use the @2x tray icons on Retina, keep tray icons as template images.
- Delete `build/gen_progress_icons.go` (would overwrite the pre-rendered ring icons).
- Do not copy the Linux or Windows icons (macOS only).
- Delete `portty-brand/` after the files are in place, so each asset has one copy. Replace and delete the `agenthub-title-logo.png` copies (docs/, frontend/public/, frontend/src/assets/) with `portty-title-logo{,-dark}.png`.

#### Text, docs and updates
- Start a fresh Portty `CHANGELOG.md`; AgentHub history stays in git.
- README is a short rewrite: what Portty is (a macOS terminal with remote access over Tailscale), install from the DMG linked at https://portty.app, two-Mac setup, MIT license.
- Update check queries `scottkw/portty`. When no release exists yet (or the API 404s while the repo is private), treat it as up to date with no error shown.
- Delete AgentHub-era design docs (`docs/plugin-architecture-assessment.md`, stale `docs/superpowers/` material) rather than rewriting them.
- Remove `packaging/homebrew` and every Homebrew mention in docs, scripts and workflows (DIST-03, already decided).

### Claude's Discretion
- Exact marker header name and value; rename order and plan split; how test fixtures like `tests/fixtures/settings_agenthub_v4.2.json` are renamed or replaced; wording of in-app strings within the macOS-native tone set in Phase 2.

### Deferred Ideas (OUT OF SCOPE)
- Phase 2 human UAT items 2-5 (deferred, resume with `/gsd-verify-work 2`).
- Phase 5: replace the real overnight wait with accelerated tests (user question 2026-11-15).

Also out of scope per the CONTEXT phase boundary: one-click sharing and Funnel removal (Phase 4), the full DMG update/download flow (DIST-02, v2), making the repo public (Phase 6).

**UI design contract:** `03-UI-SPEC.md` (approved) is binding for every string, menu item, About panel field, asset placement and update-check behaviour. This research does not restate it. It flags the two places where the UI-SPEC conflicts with how the tools actually behave (see Pitfalls 1 and 2).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BRND-01 | Built app named Portty: bundle name, bundle ID, window title, menu bar, About box, tray tooltip | Wails bundle naming (Pattern 1, Pitfall 1); Info.plist has no wails.json field for the bundle ID; native About panel via `orderFrontStandardAboutPanelWithOptions:` (Pattern 5); tray strings live in **two** places (`tray_common.go` and `tray_objc_darwin.m`) |
| BRND-02 | App icon, tray icons (progress + error), title logos from `portty-brand/` | Asset dimensions checked; @2x multi-rep NSImage loader (Pattern 4); `go:embed` with `@` in the filename tested; Wails builds the icns from `build/appicon.png` (Pitfall 3); `build/gen_icon.go` also overwrites brand art |
| BRND-03 | CLI `portty`, Go module path, config/data dirs, socket names | Module rename procedure (Pattern 2); three config-dir builders; kardianos service name; no PATH install after Homebrew removal (Pitfall 4) |
| BRND-04 | README, help, in-app text describe Portty; no AgentHub/agent-CLI wording | Only `web/join.html:57` still has agent wording in user-facing source; Help/Welcome were removed in Phase 2; README/TESTING.md/CHANGELOG are the bulk |
| BRND-05 | Zero case-insensitive "agenthub" hits in tracked files (excl. `.planning/`) | Full hit inventory; 5 filenames; `.claude/CLAUDE.md` is generated from `.planning/PROJECT.md` (Pitfall 7); gate uses the `agent[h]ub` bracket trick so the gate doesn't match itself |
| BRND-06 | Website https://portty.app; links point there or github.com/scottkw/portty | URL hit list (README, CHANGELOG, distribute.yml, updater, main.go GitHub item, UpdateBanner test, TESTING.md `gh api` lines) |
| NOTF-01 | Native notifications under the Portty name | `UNUserNotificationCenter` path in `tray_objc_darwin.m`; name and icon come from the bundle; a new bundle ID means a fresh permission prompt; quit-body copy should move into a pure, testable function |
| DIST-03 | No Homebrew: `packaging/homebrew` and every Homebrew reference removed | `distribute.yml` exists only to update the tap, so delete it; `release.yml` `brew install create-dmg` → `hdiutil`; README/TESTING.md mentions; `internal/daemon/path.go` `/opt/homebrew/bin` is code, not distribution (Open Question 3) |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Regression test convention (repo `CLAUDE.md`, TESTING.md §6):** every phase that adds, renames or removes tests updates `TESTING.md`. That means Section 2 suite manifest, Section 5 manual items (M-NN) for native-only behaviour, and Section 4 traceability rows. The path column holds repo-relative `.go/.ts/.tsx/.sh` paths only. Run `bash tests/check-traceability-paths.sh` before committing. If the `build.yml` matrix entries change, re-apply branch protection with the new check context names (Section 3).
- **Chesterton's Fence / deletion discipline (`.claude/CLAUDE.md`, `~/dev/CLAUDE.md`):** know why each piece exists before removing it. This matters most in daemon, relay and capability code.
- **Security constraint:** keep the capability-gated share model, CSP and Funnel consent gates intact. The X-Portty marker is an identity hint, **not** an auth signal.
- **Stack:** keep Go/Wails/React/tsnet. No new UI libraries (UI-SPEC: no new deps).
- **Python venv rule:** not applicable (no Python).
- **Node:** pnpm (already used).
- **Silent fallbacks:** "let it crash". This doesn't conflict with the update-check "silent up to date" rule, which is a locked product decision that the updater already implements.
- **GSD workflow:** edits go through `/gsd-execute-phase`.
- **UAT (`~/dev/CLAUDE.md` + memory):** browser UAT uses dev-browser. Native GUI checks run in the Tart VM `portty-test`, not on the host. The VM copy step uses `tar czf - agenthub.app`, which must become `Portty.app` (the helper lives outside the repo in `~/.local/share/portty-vm-tools`).
- **Ground truth (orchestrator):** `go build ./... && go vet ./...`, `go test -race -short ./...`, `cd frontend && pnpm exec tsc --noEmit && pnpm exec vitest run`, `bash tests/check-traceability-paths.sh`. The IDE LSP is stale.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Bundle name / ID / Info.plist | Build (Wails packager + `build/darwin/Info.plist` template) | — | Wails v2 generates Info.plist from the template at package time |
| Window title, app menu, About panel | Native host (Go `main.go` + cgo ObjC) | — | Custom first submenu replaced the AppMenu role, so About must be native AppKit through cgo |
| Tray icon / tooltip / menu | Native host (`tray.go` embeds + `tray_objc_darwin.m`) | Go model (`tray_common.go`, tests only) | The ObjC menu delegate builds the real menu. `BuildMenuItems` is the tested model and must match it |
| Notifications | Native host (`notification_darwin.go` → ObjC `UNUserNotificationCenter`) | Go app (`QuitGUIOnly` copy) | The OS attributes the notification to the bundle |
| CLI name, usage, error prefixes | Go CLI (`main.go`, `cmd_*.go`) | — | Same binary, dispatched on argv |
| Config / socket / data paths | Go daemon + app (`configDir`, `daemonConfigDir`, `DefaultSocketPath`) | — | Three independent builders; all must change together |
| Background service registration | Go daemon (`internal/daemon/service.go`, kardianos) | OS launchd | The name becomes the LaunchAgent label/plist name |
| Peer discovery port + marker | Go tailnet client (`internal/tailnet`) | Go webserver (`internal/webserver/server.go` route) | The client probes; the server must emit the marker |
| Update feed | Go app (`app.go` → `internal/updater`) | GitHub Releases API | Slug string plus asset matching |
| Web pages text | Static (`web/*.html`, embedded) | — | Text only |
| Brand assets | Static repo files | Build (Wails icns from `build/appicon.png`) | — |
| Docs / CI / packaging | Repo tooling (`README.md`, `.github/workflows/*`, `build.sh`) | — | — |

## Standard Stack

No new libraries. Everything below is already in the repo or built into macOS.

### Core (existing, versions verified)
| Library / Tool | Version | Purpose | Note |
|---------|---------|---------|------|
| `github.com/wailsapp/wails/v2` | v2.10.2 in go.mod; **local CLI is v2.16.0** | App bundle packaging | `go version -m ~/go/bin/wails` shows v2.16.0, and `build.sh` warns on the mismatch. The darwin packager logic (bundle name, icns from appicon.png) is identical in both [VERIFIED: module cache v2.10.2 `pkg/commands/build/packager.go:64-161`; v2.16.0 raw source fetched from GitHub] |
| `github.com/creativeprojects/go-selfupdate` | v1.5.2 | Release detection | 404 → no error, not found; DMG never matches the default suffixes (Pitfall 5) [VERIFIED: module cache `github_source.go:67-88`, `detect.go:265-287`] |
| `github.com/kardianos/service` | v1.2.4 | `portty daemon install` LaunchAgent | Name becomes the launchd label |
| `iconutil` | macOS built-in (`/usr/bin/iconutil`) | iconset → icns | [VERIFIED: `command -v`] |
| `hdiutil` | macOS built-in (`/usr/bin/hdiutil`) | DMG creation (replaces Homebrew `create-dmg`) | [VERIFIED: `command -v`] |

**Installation:** none. Nothing is installed in this phase.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `hdiutil create` in release.yml | `npm i -g create-dmg` (a different tool by sindresorhus) | Adds a dependency and changes the DMG layout; hdiutil is built in and enough for a plain DMG [ASSUMED] |
| X-Portty header on one route | Header on every response (mux wrapper) | A global header shows the product on every response, including Funnel-public ones; one route is all the probe needs |

## Package Legitimacy Audit

Not applicable. This phase installs no external packages (npm, Go or other). The only tooling change, replacing Homebrew `create-dmg` with built-in `hdiutil`, removes a third-party install.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram (what changes, where data flows)

```
 user types `portty attach X`           user clicks Portty.app (Finder/Dock)
          │ (symlink on PATH → Contents/MacOS/Portty)        │
          ▼                                                  ▼
   main.go dispatch ──CLI──► EnsureDaemon(DefaultSocketPath)   runGUI ─► Wails (Title "Portty",
          │                    │  ~/Library/Application           menu "Portty": About/Settings/Hide/Quit)
          │                    │  Support/portty/daemon.sock        │            │
          │                    ▼                                    │            └─► About ─► cgo ─► NSApp
          │            spawn os.Executable() "daemon"               │                orderFrontStandardAboutPanel
          │                    │                                    ├─► tray (ObjC): 1x+2x template reps,
          │                    ▼                                    │    tooltip "Portty — n sessions"
          │      daemon ─► webserver bind <tailnet IP>:7444         ├─► QuitGUIOnly ─► UNUserNotificationCenter
          │                 GET /api/sessions ─► [X-Portty: 1]      │    (bundle app.portty.Portty)
          │                    ▲  + requireCapability (401/200)     └─► updater.Check("scottkw/portty")
          │                    │                                         ─► GitHub API: 404/none ⇒ silent
   peer Mac's Portty ─► tailnet.DiscoverAndProbe ─► https://peer:7444/api/sessions
                         accept iff (200|401) AND header X-Portty == "1"
                         (AgentHub on :7443 is never probed; AgentHub-shaped 401 without header rejected)
```

### Recommended plan split (sequential, green at each boundary)

| Plan | Scope | Why this order |
|------|-------|----------------|
| 03-01 | Module path (`go mod edit` + sed imports), 3 config-dir builders → `portty`, kardianos service name/display/description, CLI usage + error prefixes (`main.go`, `cmd_*.go`), `.gitignore` binary names, matching Go tests (`cmd_cli_test`, `dispatch_test`, `socket_test`, `service_test`), Go comment sweep in touched files | The module path rename must be one atomic commit. Everything after it imports the new path |
| 03-02 | Port 7444 (`tailnet.DefaultProbePort`, 3 literals in `internal/daemon/process.go`, `SettingsSheet.tsx` default + its test), X-Portty marker (server route + probe), updater slug (+ DMG filter, see Open Question 1), tests | Network identity is self-contained and needs its own test rewrite (tailnet_test probe cases) |
| 03-03 | Asset import (copy, `-2x`→`@2x`, `iconutil`), `tray.go` embeds the 6 @2x files, ObjC multi-rep loader, `tray_test` 36×36 assertions, delete `build/gen_progress_icons.go` (+ `build/gen_icon.go`, Open Question 2), title logos ×3 locations, trim `scripts/verify-icons.sh`, delete `portty-brand/` | The @2x files must exist before `//go:embed` names them (a missing embed file is a compile error) |
| 03-04 | `wails.json`, `Info.plist` + `Info.dev.plist`, window title, app menu (About Portty first, Hide/Quit Portty, remove GitHub item), About panel (new `about_darwin.go` + `about_darwin.m`), tray strings + ObjC class/log renames, notification identifier/title/body (extract pure func), `QuitConfirmModal`, `frontend/index.html`, `NewSessionModal` storage key, `web/*.html`, Basic realm, self-signed cert Org, matching tests, then `wails build` + VM smoke | Depends on 03-03 because both edit `tray_objc_darwin.m`. Doing it after assets lets the VM smoke see the final icons |
| 03-05 | README rewrite, fresh CHANGELOG, delete `docs/plugin-architecture-assessment.md`, `docs/superpowers/`, `packaging/homebrew/`, `.github/workflows/distribute.yml`; rename in `release.yml`/`build.yml`/`build.sh`/`tests/build-script.test.sh`, `hdiutil` swap; `frontend/package.json` description + `"license": "MIT"`; `CLAUDE.md` title; `.claude/CLAUDE.md` + PROJECT.md source (Pitfall 7); TESTING.md (title, §3 contexts + repo slug, M-items, new M-items, traceability rows); fixture rename; remaining comment sweep; zero-hit gate in `build.yml` | Text-only plus CI. The gate goes last because it only passes once everything else is done |

### Pattern 1: Wails v2 bundle naming and Info.plist
**What:** On darwin, `packageApplicationForDarwin` names the bundle `options.ProjectData.Name + ".app"` (when `BundleName` is empty, which it is for a single target). It moves the compiled binary to `Contents/MacOS/<OutputFilename>`. It renders `build/darwin/Info.plist` (or `Info.dev.plist` in Dev mode) as a Go template with project data. It writes `Resources/iconfile.icns` by encoding `build/appicon.png`. [VERIFIED: `~/go/pkg/mod/github.com/wailsapp/wails/v2@v2.10.2/pkg/commands/build/packager.go:64-161`, quoted: `bundlename = options.ProjectData.Name + ".app"`, `packedBinaryPath := filepath.Join(exeDir, options.ProjectData.OutputFilename)`, `err = processDarwinIcon(options.ProjectData, "appicon", resourceDir, "iconfile")`, `appIcon, err := buildassets.ReadFile(projectData, iconName+".png")`, `return icns.Encode(dest, srcImg)`.] The same lines exist in v2.16.0 [CITED: raw.githubusercontent.com/wailsapp/wails/v2.16.0/v2/pkg/commands/build/packager.go].

Evidence on disk: the current build is `build/bin/agenthub.app` (lowercase, from `"name": "agenthub"`). The executable inside is `Contents/MacOS/AgentHub` (from `"outputfilename": "AgentHub"`). The bundled `iconfile.icns` (279301 B) **differs** from `build/darwin/iconfile.icns` (506653 B) (`cmp` → differ). [VERIFIED: `ls`/`cmp`/`plutil -p` this session]

Wails' `Info` struct has no bundle-ID field: `CompanyName, ProductName, ProductVersion, Copyright, Comments, FileAssociations, Protocols` [VERIFIED: `internal/project/project.go:216-224`]. The bundle ID must be edited directly in the plist templates.

**Target `wails.json`** (current values [VERIFIED: wails.json:3-16]: `"name": "agenthub"`, `"outputfilename": "AgentHub"`, `"companyName": "AgentHub"`, `"productName": "AgentHub"`, `"copyright": "Copyright 2026 AgentHub"`, `"comments": "AI coding session manager"`):
```json
  "name": "Portty",
  "outputfilename": "Portty",
  ...
  "info": {
    "companyName": "Portty",
    "productName": "Portty",
    "productVersion": "2.2.0",
    "copyright": "Copyright © 2026 Ken Scott. MIT License.",
    "comments": "A macOS terminal with remote access over Tailscale."
  }
```
**Target `build/darwin/Info.plist`** (current [VERIFIED: build/darwin/Info.plist:8-13]: `<key>CFBundleDisplayName</key>` `<string>AgentHub</string>` … `<key>CFBundleIdentifier</key>` `<string>com.agenthub.app</string>`): `CFBundleDisplayName` → `Portty`, `CFBundleIdentifier` → `app.portty.Portty`. Keep `LSUIElement` true. Keep `CFBundleIconFile` `iconfile`. `Info.dev.plist` has `CFBundleDisplayName` `AgentHub` and `CFBundleIdentifier` `com.wails.{{.Name}}`. Per UI-SPEC, set both to the Portty values.

Leave `productVersion` alone. Versioning is Phase 6.

### Pattern 2: Go module path rename (atomic)
```bash
go mod edit -module github.com/scottkw/portty
git ls-files '*.go' | xargs grep -l 'github.com/scottkw/agenthub' \
  | xargs sed -i '' 's|github.com/scottkw/agenthub|github.com/scottkw/portty|g'   # BSD sed on macOS
gofmt -l $(git ls-files '*.go') | grep -v '^internal/relay/scrollback_altscreen_test.go$'  # expect empty
go build ./... && go vet ./...
```
- 57 `.go` files import the path. The dry run produced no new gofmt diffs: `scottkw/portty` sorts in the same position as `scottkw/agenthub` against every neighbour (`charmbracelet` < `scottkw` < `skip2` < `wailsapp`) [VERIFIED: dry run this session].
- Non-Go files that contain the module path: `go.mod`, `.github/workflows/distribute.yml` (deleted in 03-05), `README.md`, `CHANGELOG.md`, `packaging/homebrew/…` (deleted). No `-ldflags -X github.com/scottkw/agenthub/...` anywhere. Version injection is `-X main.Version`, which is unaffected [VERIFIED: git grep].
- The wailsjs bindings use Go package names (`main`, `updater`, …), not the module path, so no regeneration is needed for the rename. `frontend/src/wailsjs` has zero "agenthub" hits [VERIFIED: git grep]. Don't add new exported `App` methods in this phase (About should be a package func), so the hand-maintained `App.d.ts` stays untouched. After any `wails build`, restore with `git checkout -- frontend/src/wailsjs` (orchestrator fact).

### Pattern 3: Config/socket/service paths
The three builders, all `filepath.Join(base, "agenthub")` with `base := os.UserConfigDir()`:
- `app.go:806-815` `configDir()`: `dir := filepath.Join(base, "agenthub")` [VERIFIED: Read app.go:806-815]
- `internal/daemon/engine.go:57-67` `daemonConfigDir()`: `dir := filepath.Join(base, "agenthub")` [VERIFIED: Read]
- `internal/daemon/socket.go:15-23` `DefaultSocketPath()`: `dir := filepath.Join(base, "agenthub")` … `return filepath.Join(dir, "daemon.sock")` [VERIFIED: Read]

Change each leaf to `"portty"` and fix the comments, which wrongly say `~/.config/agenthub`. On darwin, `os.UserConfigDir` returns `$HOME/Library/Application Support` [VERIFIED: `go doc os.UserConfigDir`; live dir `~/Library/Application Support/agenthub/` holds `settings.json`, `capability.key`, `daemon.sock`, `chats/`, `aliases.json`, `ct_disclosed`, `opencode-tui.json`; `~/.config/agenthub` does not exist]. So Portty's real location becomes `~/Library/Application Support/portty/`. That matches the CONTEXT intent ("same layout as AgentHub, renamed") even though the CONTEXT names `~/.config`.

Don't consolidate the three builders into one helper in this phase. The engine.go comment explains why: "internal packages cannot import main". Changing that would be a refactor, not a rename.

kardianos service [VERIFIED: Read internal/daemon/service.go:54-65]: `Name: "agenthub-daemon"`, `DisplayName: "AgentHub Daemon"`, `Description: "AgentHub session manager daemon"` → `portty-daemon` / `Portty Daemon` / `Keeps Portty terminal sessions running in the background.` (UI-SPEC). `service_test.go:17-24` asserts these three literals.

### Pattern 4: Retina template tray icons (two NSBitmapImageReps)
Today every path does `[[NSImage alloc] initWithData:data]` + `setSize:18×18` + `setTemplate:YES` from one PNG (`tray_objc_darwin.m` `initStatusItem`, `updateTrayIcon`). Change the C signatures to take a 1x and a 2x buffer. Build one NSImage with two reps, both sized 18×18 points:
```objc
// Source: AppKit NSImage multi-representation pattern [ASSUMED: standard AppKit usage, not fetched this session]
static NSImage *makeTrayImage(NSData *d1, NSData *d2) {
    NSImage *img = [[NSImage alloc] initWithSize:NSMakeSize(18, 18)];
    NSBitmapImageRep *r1 = [NSBitmapImageRep imageRepWithData:d1];
    [r1 setSize:NSMakeSize(18, 18)];          // 18 px → 1x
    [img addRepresentation:r1];
    NSBitmapImageRep *r2 = [NSBitmapImageRep imageRepWithData:d2];
    [r2 setSize:NSMakeSize(18, 18)];          // 36 px in 18 pt → 2x
    [img addRepresentation:r2];
    [img setTemplate:YES];
    return img;
}
void initStatusItem(const void *d1, int n1, const void *d2, int n2);   // copy both to NSData BEFORE dispatch_async
void updateTrayIcon(const void *d1, int n1, const void *d2, int n2);
```
- **Copy both buffers into `NSData` synchronously before `dispatch_async`.** The existing comments explain why: the Go pointer is only valid during the cgo call. Keep that.
- The file uses manual retain/release (`[statusItem retain]`), so it is not ARC. Don't add ARC-only syntax.
- Go side: `trayIconBytesForState` returns a pair (`[]byte, []byte`) or a small struct, plus six new `//go:embed assets/tray_icon*@2x.png` vars. `go:embed` accepts `@` in filenames [VERIFIED: built a scratch module embedding `assets/tray_icon@2x.png` with go1.27.4, printed `1`].
- Brand files are the expected sizes: 1x 18×18 and -2x 36×36 for all 6 states, iconset 16…1024 (10 files), appicon 1024², logos 1400×360, all with alpha [VERIFIED: `sips` this session]. `assets/appicon.png` == `build/appicon.png` (`cmp` identical).

### Pattern 5: Native About panel via cgo
Follow the existing `appearance_darwin.go` + `appearance_darwin.m` pattern: a separate `.m` so ObjC symbols compile once, the `_darwin` filename suffix, and `dispatch_async` to the main queue, never `dispatch_sync` (appearance_darwin.m documents a deadlock risk with Wails).
```objc
// about_darwin.m
#import <Cocoa/Cocoa.h>
void showAboutPanel(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSFont *f = [NSFont systemFontOfSize:[NSFont smallSystemFontSize]];
        NSMutableParagraphStyle *p = [[NSMutableParagraphStyle alloc] init];
        p.alignment = NSTextAlignmentCenter;
        NSMutableAttributedString *credits = [[NSMutableAttributedString alloc]
            initWithString:@"A macOS terminal with remote access over Tailscale.\n"
                attributes:@{NSFontAttributeName: f, NSParagraphStyleAttributeName: p,
                             NSForegroundColorAttributeName: NSColor.secondaryLabelColor}];
        [credits appendAttributedString:[[NSAttributedString alloc] initWithString:@"portty.app"
                attributes:@{NSFontAttributeName: f, NSParagraphStyleAttributeName: p,
                             NSLinkAttributeName: [NSURL URLWithString:@"https://portty.app"]}]];
        [NSApp activateIgnoringOtherApps:YES];
        [NSApp orderFrontStandardAboutPanelWithOptions:@{
            NSAboutPanelOptionCredits: credits,
            NSAboutPanelOptionVersion: @"",      // blank build number → "Version 2.2.0", no "(2.2.0)"
        }];
    });
}
```
```go
// about_darwin.go — seam so menu_test can record the click without AppKit
// #cgo darwin CFLAGS: -x objective-c
// #cgo darwin LDFLAGS: -framework Cocoa
// void showAboutPanel(void);
import "C"
var menuShowAbout = func() { C.showAboutPanel() }
```
- `NSAboutPanelOptionVersion`: "If not specified, obtain from the CFBundleVersion key in infoDictionary; if not specified or empty string, leave blank." `NSAboutPanelOptionApplicationName` defaults to `CFBundleName`. `NSAboutPanelOptionApplicationIcon` defaults to `NSApplicationIcon`. Credits as an attributed string render in dark appearance correctly on the 10.15+ SDK. [VERIFIED: MacOSX.sdk `AppKit.framework/Headers/NSApplication.h:552-568`]
- Menu: `appSub.AddText("About Portty", nil, func(_ *menu.CallbackData) { menuShowAbout() })` as the **first** item, followed by a separator. `menu_test.go` `TestAppMenu_Structure` currently asserts `items[0].SubMenu.Items[0].Label != "Settings…"` → it must expect "About Portty" first. The test's accelerator table also hard-codes `{"AgentHub", "Settings…" …}` [VERIFIED: menu_test.go:102-133 via grep].
- Whether a clickable `NSLinkAttributeName` renders in the standard panel is UI-SPEC's open item. Keep the M-item backstop.

### Pattern 6: Portty-only probe marker
**Server:** wrap only the probed route so the header is present on both the 401 (no cap) and 200 paths. `http.Error` keeps headers that were already set.
```go
// internal/webserver/server.go:826 currently:
//   mux.HandleFunc("GET /api/sessions", ws.requireCapability(ws.handleListSessions))   [VERIFIED: Read server.go:822-826]
// constants live in internal/tailnet: const MarkerHeader, MarkerValue = "X-Portty", "1"
func porttyMarker(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(tailnet.MarkerHeader, tailnet.MarkerValue)
		next(w, r)
	}
}
mux.HandleFunc("GET /api/sessions", porttyMarker(ws.requireCapability(ws.handleListSessions)))
```
**Client** (current [VERIFIED: Read tailnet.go:82-108]: `const agentHubProbeMarker = "capability required"`, and `isAgentHubProbeResponse` returns true on `http.StatusOK` or on a 401 whose first 256 body bytes contain the marker):
```go
func isPorttyProbeResponse(resp *http.Response) bool {
	if resp.Header.Get("X-Portty") != "1" {   // AgentHub-shaped 401 or any foreign 200 → not a peer
		return false
	}
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized
}
```
- Define `MarkerHeader = "X-Portty"` and `MarkerValue = "1"` in `internal/tailnet` and import them from `internal/webserver`. `internal/tailnet` imports no `internal/...` package and webserver doesn't import tailnet today, so this adds no import cycle [VERIFIED: grep of imports this session]. Don't go the other way (tailnet → webserver).
- The body read (`io.LimitReader 256`) is no longer needed. Drop it rather than keeping a dead branch.
- Port: `tailnet.DefaultProbePort = 7443` → `7444` [VERIFIED: Read tailnet.go:27-29]. Replace the three literals in `internal/daemon/process.go` (`api.RestartWebServer(h.IP, 7443, …)` line 49, `api.AutoStartWebServer(h.IP, 7443, …)` line 125, `api.AutoStartWebServer(lanIP, 7443, …)` line 136 [VERIFIED: Read]) with `tailnet.DefaultProbePort`. The daemon package already imports tailnet (`tailnet_cache.go`). `frontend/src/components/SettingsSheet.tsx:109` `useState<number>(7443)` [VERIFIED: Read] → `7444`, and update `SettingsSheet.tailscale-status.test.tsx` (4 occurrences of 7443) and `remoteSession.test.ts` (example URL). The `tailnet.go:28` comment "This matches the default in SettingsPanel.tsx" is stale (the file is `SettingsSheet.tsx`).

### Pattern 7: Update feed
`app.go:1352` and `app.go:1376` pass `"scottkw/agenthub"` to `updater.Check` [VERIFIED: sed this session]. Hoist it to one `const updateSlug = "scottkw/portty"`. `updater.Check` already returns `nil, nil` on `err != nil || !found`. go-selfupdate's `GitHubSource.ListReleases` turns an HTTP 404 into `return nil, nil` ("repository not found or release not found. It's not an error here"). So a private repo or no releases leads to `found=false` and the check stays silent [VERIFIED: go-selfupdate@v1.5.2 `github_source.go:67-88`, `updater.go Check`]. Rate-limit and network errors go through `err != nil`, which is also silent.

**DMG discovery fix (recommended, Open Question 1):** current `DefaultDetect` [VERIFIED: Read updater.go:53-59] calls `selfupdate.DetectLatest`. Without `Filters`, assets match only when the lowercased name ends in `{os}{-|_}{arch}{ext}`, with ext in `.zip .tar.gz .tgz .gzip .gz .tar.xz .xz .bz2 ""` [VERIFIED: detect.go:265-287]. `portty-v1.0.0-darwin-universal.dmg` never matches, so a real release reads as "not found" forever. Minimal fix:
```go
up, err := selfupdate.NewUpdater(selfupdate.Config{
	Filters: []string{`^portty-v?\d+\.\d+\.\d+.*-darwin-universal\.dmg$`}, // names are lowercased before matching
})
if err != nil { return "", false, err }
latest, found, err := up.DetectLatest(ctx, selfupdate.ParseSlug(slug))
```
With `Filters` set, suffix matching is skipped and any filter match selects the asset (`detect.go:211-240`). Drafts and prereleases are still skipped by default (`detect.go:184-191`), which matches the UI-SPEC row "draft or prerelease only → nothing". This can be unit-tested with a fake `selfupdate.Source` (`Config.Source`). Keep the test offline.

### Anti-Patterns to Avoid
- **Blanket `sed s/AgentHub/Portty/` over comments and docs.** It turns "Legacy AgentHub keys are ignored" into a false statement ("Legacy Portty keys"). Review each comment hit. Many should be deleted or reworded ("the upstream app"), not renamed.
- **Negative assertions that spell the banned word.** `App.test.tsx:200-201` asserts `not.toContain('agenthub:terminalTheme')`. Rewrite as `not.toMatch(/terminalTheme|uiTheme/)`.
- **Hard-coding `~/.config/portty`.** Use `os.UserConfigDir()`.
- **A global response header** for the marker. One route is enough.
- **Moving `tray_objc_darwin.m` code into a Go file preamble.** The header comment explains that ObjC classes live in the `.m` to avoid duplicate-symbol link errors in `go test`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| About box | HTML dialog / custom NSWindow | `orderFrontStandardAboutPanelWithOptions:` | Native look (LOOK-01), VoiceOver, Light/Dark for free; UI-SPEC forbids a fallback |
| Retina tray image | Pick 1x vs 2x by `backingScaleFactor` at runtime | One NSImage with two reps | AppKit picks the rep per screen, including when a window moves between displays |
| DMG creation | Homebrew `create-dmg` | `hdiutil create -volname Portty -srcfolder build/bin/Portty.app -ov -format UDZO "$DMG"` + `codesign --sign "$ID" "$DMG"` | Built in, no Homebrew [ASSUMED: same contents as create-dmg's default; CI not run this session] |
| icns | Custom Go icns writer (`build/gen_icon.go`) | Wails packager (`build/appicon.png`) + `iconutil` for the tracked `build/darwin/iconfile.icns` | Already handled |
| Asset-name matching for updates | Hand-written GitHub API client | go-selfupdate `Config.Filters` | One line; keeps the existing seam and tests |

**Key insight:** almost nothing here needs new code. The risk is missing a spot: a second copy of a string (tray menu in ObjC **and** Go), a literal port in `process.go`, a generated file (`.claude/CLAUDE.md`), or a filename.

## Runtime State Inventory

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `~/Library/Application Support/agenthub/` on this Mac: `settings.json`, `capability.key`, `daemon.sock`, `chats/`, `aliases.json`, `ct_disclosed`, `opencode-tui.json` [VERIFIED: ls]. WebKit/UserDefaults data keyed by bundle ID `com.agenthub.app` (`~/Library/WebKit/…`, `~/Library/Preferences/…`) holds localStorage (`agenthub:lastWorkDir`) [ASSUMED location] | **None (fresh start, locked).** Code edit only: the leaf becomes `portty`, so Portty creates a new dir and a new signing key. Old AgentHub caps can't validate against Portty's new key, which is a security plus. A new bundle ID means a new WebKit store, so localStorage starts empty. No data migration |
| Live service config | AgentHub dev daemon currently running from this tree, bound to the agenthub socket and :7443. Tailscale `serve`/Funnel config lives in tailscaled, not git (only if Funnel was enabled) | None. Both apps coexist (different socket, different port). AgentHub left alone (locked). Funnel config is Phase 4 |
| OS-registered state | kardianos LaunchAgent: none installed on this Mac (`~/Library/LaunchAgents` has no agenthub/portty plist) [VERIFIED: ls]. Office/home Macs: not checked [ASSUMED none]. macOS notification permission and Login Items entries are keyed by bundle ID | Code edit only (service name `portty-daemon`). New bundle ID means macOS asks notification permission again (expected; UI-SPEC backstop M-item). If an `agenthub-daemon` LaunchAgent exists on the other Macs, leave it (locked: AgentHub left alone) |
| Secrets/env vars | Release secrets referenced by name in `release.yml` (`MACOS_*`, `RELEASE_PUBLISH_TOKEN`); keychain profile name `"agenthub-notarize"` is created at CI time (`xcrun notarytool store-credentials`) | Rename the profile string to `portty-notarize` in `release.yml` and `build.sh` (created fresh each run, so nothing to migrate). Secret names are unchanged. `RELEASE_PUBLISH_TOKEN` existed only so the release event triggers `distribute.yml`. Once that's deleted, update the comment and leave the token logic for Phase 6 |
| Build artifacts | `build/bin/agenthub.app` (gitignored; the macOS FS is case-insensitive, so `Portty.app` is a separate dir); root `go build` output `/agenthub` (gitignored, line 2); VM copy at `~/ptest/agenthub.app` in Tart VM `portty-test`; VM helper command `tar czf - agenthub.app` in `~/.local/share/portty-vm-tools` (outside repo) | Delete stale `build/bin/agenthub.app` locally (optional). `.gitignore`: `/agenthub`, `/agenthub.exe`, `/agenthub-cli`, `/agenthub-cli.exe` → `/portty` (Windows variants are moot; `*.exe` is already ignored). Update the VM deploy command to `Portty.app` (not a repo change) |

**Canonical question answered:** once every file is updated, the only places still holding "agenthub" are the user's own AgentHub install and data (kept on purpose) and the external VM helper script.

## Common Pitfalls

### Pitfall 1: UI-SPEC's `name` value gives `portty.app`, not `Portty.app`
**What goes wrong:** The UI-SPEC table says "`wails.json` `outputfilename` / `name` → `Portty` / `portty`". Wails names the bundle from `name`, so the build lands at `build/bin/portty.app`. That breaks success criterion 1 and every `build/bin/Portty.app` path in CI.
**How to avoid:** Set `"name": "Portty"`. `name` is also the template value in `Info.dev.plist` (`com.wails.{{.Name}}`), which this phase replaces anyway.
**Warning signs:** `ls build/bin` after `wails build`.

### Pitfall 2: Treating the CONTEXT path `~/.config/portty` literally
**What goes wrong:** Someone "fixes" the code to `~/.config/portty`, which diverges from how the app has always behaved and from the daemon socket path.
**How to avoid:** Keep `os.UserConfigDir()`. Update the misleading comments (`app.go:806`, `engine.go:57`, `api_test.go:31,982`, `engine_test.go:420` mention `~/.config/agenthub`).

### Pitfall 3: Regenerating `build/darwin/iconfile.icns` and expecting the app icon to change
**What goes wrong:** Wails rewrites `Resources/iconfile.icns` from `build/appicon.png` on every build. The `iconutil` output is never shipped by Wails. (release.yml's "Copy branded ICNS" step copies a nonexistent `build/appicon.icns` with `|| true`, so it's a silent no-op.)
**How to avoid:** Make sure `build/appicon.png` is the Portty 1024px icon (it is, after copy). Run `iconutil` anyway (locked, and checked by verify-icons.sh). Either fix the release.yml copy step to use `build/darwin/iconfile.icns` or delete it (Chesterton: it exists to get a multi-resolution hand-made icns into the bundle; with the brand's iconset it is now worth fixing to `cp build/darwin/iconfile.icns build/bin/Portty.app/Contents/Resources/iconfile.icns` *before* signing).

### Pitfall 4: No way to get `portty` on PATH after Homebrew removal
**What goes wrong:** The cask's `binary "#{appdir}/AgentHub.app/Contents/MacOS/AgentHub", target: "agenthub"` was the only installer of the CLI [VERIFIED: packaging/homebrew/agenthub.rb.template]. A DMG install gives only `Portty.app/Contents/MacOS/Portty`. `portty attach` (success criterion 3) won't work from a shell.
**How to avoid:** Add one line to the README Install section: `sudo mkdir -p /usr/local/bin && sudo ln -sf /Applications/Portty.app/Contents/MacOS/Portty /usr/local/bin/portty`. The daemon respawns from `os.Executable()`, which works through the symlink. See Open Question 4 (the UI-SPEC README section list doesn't mention the CLI).

### Pitfall 5: Update banner can never appear for a DMG release
See Pattern 7. Silent-on-404 already works. Positive detection is broken by asset naming. Without the Filter, the UI-SPEC row "newer release exists → banner" can't pass, but no test catches it, because `DefaultDetect` is injected away in tests.

### Pitfall 6: Two sources of tray menu text
`tray_common.go` `BuildMenuItems` ("Open AgentHub" … "Quit") is the unit-tested model. The ObjC `AgentHubMenuDelegate menuWillOpen:` builds the real menu with its own literals `@"Open AgentHub"` and `@"Quit"`, and `initStatusItem` sets `toolTip = @"AgentHub"`. Change both and keep them identical: "Open Portty" / "Quit Portty".

### Pitfall 7: `.claude/CLAUDE.md` is generated from `.planning/PROJECT.md`
**What goes wrong:** `.claude/CLAUDE.md` is tracked and has 3 hits. They sit inside `<!-- GSD:project-start source:PROJECT.md -->`, copied from PROJECT.md "What This Is" and "Constraints". Editing only `.claude/CLAUDE.md` gets undone the next time GSD regenerates it. Editing only PROJECT.md leaves the tracked copy dirty. (PROJECT.md itself is under the `.planning/` exclusion.)
**How to avoid:** Reword the copied PROJECT.md sections (e.g. "a stripped-down fork of the author's earlier agent-CLI app") and the matching `.claude/CLAUDE.md` block in the same commit. Keep the full AgentHub history in PROJECT.md's Context/Key Decisions sections, which are not copied. Root `CLAUDE.md:1` `# AgentHub — Claude Instructions` → `# Portty — Claude Instructions`.

### Pitfall 8: Embedding @2x files before they exist
A `//go:embed` naming a missing file fails compilation for the whole `main` package. Import assets first, in the same plan or an earlier one.

### Pitfall 9: Branch-protection context name changes
`build.yml` matrix `name: agenthub` creates the required check context `build (agenthub, darwin/universal, macos-latest)`. Renaming it to `portty` changes the context. TESTING.md §3 holds the `gh api repos/scottkw/agenthub/branches/main/protection` commands. The standing rule requires re-applying protection. Branch protection on a private repo may need a paid plan. Make this a human-action note, not an automated step.

### Pitfall 10: Self-matching zero-hit gate
A CI step or test that greps for the literal "agenthub" is itself a hit. Use `git grep -n -i -I 'agent[h]ub' -- ':!.planning'` and `git ls-files | grep -i 'agent[h]ub'`, both of which must print nothing.

### Pitfall 11: CLI `web start` binds port 443, not the probe port
`cmd_cli.go` `cmdWebStart` uses `port := 443`. The daemon auto-start uses 7443 (→ 7444). A server started from the CLI is never discovered by peers. This predates the phase. Changing the port isn't strictly part of the rebrand (Open Question 5).

## Code Examples

### Pure, testable quit-notification copy (NOTF-01)
`QuitGUIOnly` calls `runtime.WindowHide(a.ctx)`, which `log.Fatal`s without a Wails frontend in the context (`wails/v2 pkg/runtime/runtime.go getFrontend`) [VERIFIED: module cache]. So `QuitGUIOnly` can't be unit-tested directly. Mirror `trayTooltip`:
```go
// tray_common.go (no build tag, testable)
const quitNotificationID, quitNotificationTitle = "portty.quit-gui-only", "Portty"
func quitToMenuBarBody(n int) string {
	switch n {
	case 0:
		return "Portty is still running in the menu bar."
	case 1:
		return "Portty is still running in the menu bar with 1 session open."
	default:
		return fmt.Sprintf("Portty is still running in the menu bar with %d sessions open.", n)
	}
}
// app.go QuitGUIOnly: a.sendNotificationFunc(quitNotificationID, quitNotificationTitle, quitToMenuBarBody(count))
```
Current code [VERIFIED: Read app.go:342-348]: `body = "AgentHub is still running in the background. 1 session active."` / `"AgentHub is still running in the background. %d sessions active."` / `a.sendNotificationFunc("agenthub.quit-gui-only", "AgentHub", body)`. Note the current code has no 0-session branch ("0 sessions active"), which the UI-SPEC fixes.

### Release DMG without Homebrew
```bash
VERSION="${GITHUB_REF_NAME}"
DMG_NAME="portty-${VERSION}-darwin-universal.dmg"
hdiutil create -volname "Portty" -srcfolder build/bin/Portty.app -ov -format UDZO "build/bin/${DMG_NAME}"
codesign --force --timestamp --sign "$MACOS_CERTIFICATE_NAME" "build/bin/${DMG_NAME}"
```
[ASSUMED: CI not exercised this session; the planner should treat release.yml edits as untested until Phase 6 cuts a release.]

### Zero-hit gate (build.yml step)
```yaml
      - name: Brand gate (BRND-05, DIST-03)
        shell: bash
        run: |
          ! git grep -n -i -I 'agent[h]ub' -- ':!.planning'
          ! git ls-files | grep -i 'agent[h]ub'
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `NSUserNotification` | `UNUserNotificationCenter` (already used) | macOS 10.14 | Permission is keyed by bundle ID, so the rename re-prompts |
| Single-rep 18px tray PNG | 1x + 2x reps, template | — | Crisp on Retina |
| Homebrew cask distribution | DMG from GitHub Releases linked at portty.app | DIST-03 | CLI needs a manual symlink (Pitfall 4) |

**Deprecated/outdated in repo:** `build/gen_progress_icons.go` (bottom-bar glyphs), `build/gen_icon.go` (generates the AgentHub navy "A" icon and the old tray icons into `appicon.png`, `tray_icon.png`, `tray_icon_error.png`). Both are `//go:build ignore`, and neither is referenced by `go:generate` or any script [VERIFIED: git grep `go:generate` → none; headers read].

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Two `NSBitmapImageRep`s sized 18×18 pt in one template NSImage render the 36px rep on Retina | Pattern 4 | Blurry tray icons; caught by the new manual M-item |
| A2 | `NSLinkAttributeName` in About credits is clickable | Pattern 5 | Plain-text link; UI-SPEC accepts this and records it |
| A3 | `hdiutil` + `codesign` DMG is acceptable in place of `create-dmg` | Code Examples | Release job differs; untested until Phase 6 |
| A4 | macOS shows notifications under the bundle's name and icon, and re-prompts for permission on a new bundle ID; unsigned/ad-hoc builds may not get `UNUserNotificationCenter` authorization (M-41 used a Developer-ID-signed build) | NOTF-01 | Notification check fails in the VM if the build isn't signed; use a signed build for the M-item |
| A5 | No `agenthub-daemon` LaunchAgent exists on the office/home Macs | Runtime State | None functionally (left alone by decision) |
| A6 | WebKit localStorage for the app lives under a bundle-ID-keyed path, so it resets | Runtime State | Last-used folder may carry over (harmless) |
| A7 | The `/usr/local/bin` symlink is the right CLI install method for this user | Pitfall 4 | User may prefer `~/.local/bin` or an in-app installer |

## Open Questions

1. **Fix DMG asset matching in the updater now, or in Phase 6?**
   - Known: the slug change plus silent 404 meet the CONTEXT decision as written. Without `Filters`, no DMG release is ever detected (verified in go-selfupdate source).
   - Recommendation: include the one-line `Filters` config plus an offline fake-`Source` test in plan 03-02. It's small, it touches the same function, and it makes the UI-SPEC "newer release → banner" row actually achievable. If the planner wants to keep scope tight, record it as a Phase 6 (UPD-01) blocker in STATE.md instead.
2. **Delete `build/gen_icon.go` too?**
   - It writes the AgentHub icon and tray art over `build/appicon.png` and `assets/tray_icon{,_error}.png` if anyone runs it. That's the same reason CONTEXT gives for deleting `gen_progress_icons.go`. Its only brand hit is one comment.
   - Recommendation: delete it (Chesterton satisfied: its sole purpose was generating the art the brand files replace). The user can override.
3. **Do code-level Homebrew paths count for DIST-03?**
   - `internal/daemon/path.go` prepends `/opt/homebrew/bin` and `/usr/local/bin` so the Tailscale CLI is found. `path_test.go` is named `TestAugmentServicePath_HomebrewTailscaleOnly`. The `main.go:67` comment mentions "Homebrew, volta, nvm" (stale).
   - Success criterion 7 covers "docs, scripts or workflows". These are code paths about where Tailscale is installed, not about distributing Portty.
   - Recommendation: keep `path.go` and `path_test.go` (a real directory, explained by its comment) and fix the stale `main.go` comment. Also change the fixture's `"/opt/homebrew/bin/tailscale"` value to `/usr/local/bin/tailscale` (coverage is unaffected). Gate with `git grep -n -i 'homebrew\|brew install\|brew tap' -- ':!.planning' ':!internal/daemon/path.go' ':!internal/daemon/path_test.go'` → empty.
4. **README: CLI install line?** The UI-SPEC README sections are Install / Use It From Another Mac / License. Recommendation: one symlink line inside Install (Pitfall 4). Otherwise BRND-03's `portty attach` only works by full bundle path.
5. **CLI `web start` port 443 vs 7444?** Recommendation: leave it as is in this phase and note it for Phase 4 (share redesign touches `serve`/`web`). The rebrand moves only the 7443 sites.
6. **`assets/appicon.png` is unreferenced by code.** UI-SPEC places the Portty icon there anyway (it was there in AgentHub). Recommendation: follow UI-SPEC. Removing it is a later simplicity call.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | build/test | ✓ | go1.27.4 (go.mod `go 1.26.4`) | — |
| Wails CLI | `wails build` | ✓ | v2.16.0 (go.mod pins v2.10.2; same packager behaviour) | `go install …/wails@v2.10.2` to match CI |
| iconutil | icns regen | ✓ | macOS built-in | — |
| hdiutil | DMG (CI) | ✓ | macOS built-in | — |
| ImageMagick `identify` | `scripts/verify-icons.sh` | ✓ (via Homebrew on host) | — | `sips -g pixelWidth` (built-in). Switch the script to `sips` so it doesn't depend on Homebrew |
| Node / pnpm | vitest/tsc | ✓ | v24.22.1 / 12.6.0 | — |
| Tart VM `portty-test` | native GUI smoke (About, tray, notification) | not probed | — | Host check only if the user allows |

**Missing dependencies with no fallback:** none.

## Validation Architecture

Skipped: `workflow.nyquist_validation` is `false` in `.planning/config.json`. The per-requirement test changes are still listed below so the TESTING.md standing rule can be followed.

| Req | Test change |
|-----|-------------|
| BRND-01 | `menu_test.go` (About first, Portty labels, GitHub item gone, About callback hits `menuShowAbout` seam); `tray_common_test.go` (tooltip + "Open Portty"/"Quit Portty"); vitest `QuitConfirmModal.test.tsx` |
| BRND-02 | `tray_test.go` 18×18 for 1x and 36×36 for 6 @2x files |
| BRND-03 | `socket_test.go` suffix `portty/daemon.sock`; `service_test.go`; `cmd_cli_test.go`/`dispatch_test.go` usage strings; vitest `NewSessionModal.test.tsx` key `portty:lastWorkDir` |
| BRND-06/NOTF-01 | `updater_test.go` `testSlug = "scottkw/portty"` (+ filter test if OQ1 accepted); new `quitToMenuBarBody` 0/1/n test; `notification_darwin_test.go` identifier; `UpdateBanner.test.tsx` URL |
| Port/marker | `tailnet_test.go`: header+200 → true, header+401 → true, 401 "capability required" without header → false, 200 without header → false; webserver test: unauthenticated `GET /api/sessions` → 401 **with** `X-Portty: 1` |
| BRND-05 | CI brand gate step (Pitfall 10) |
| Fixture | `git mv tests/fixtures/settings_agenthub_v4.2.json tests/fixtures/settings_legacy_v4.json`; rename `TestSettingsMigration_LegacyAgentHubV42` → `TestSettingsMigration_LegacyV4Keys`; rename `TestFetchPeerSessionsMeta_LegacyAgentHubPeer` → `…_IgnoresLegacyFields`. Assertions unchanged, so coverage is kept. Update TESTING.md rows (NAV-03, STRP-02, STRP-01) |
| Manual (TESTING.md §5) | New M-items: About panel (Light/Dark, single version line, link); tray icons crisp on Retina in all 6 states, light and dark menu bars, highlighted; notification permission prompt and attribution under "Portty" (signed build); README dark/light logo. Rewrite the existing "Quit any installed AgentHub and stop its daemon (shared socket and port 7443)" preconditions in M-57…M-68: they're no longer needed now that paths and ports are separate |

## Security Domain

`security_enforcement: true`, ASVS level 1.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no change | Capability tokens unchanged. Fresh signing key in the new dir |
| V3 Session Management | no | — |
| V4 Access Control | yes (must not regress) | `requireCapability` stays the only gate on `/api/sessions`. The marker wrapper runs *outside* it and adds only a header |
| V5 Input Validation | yes | Probe reads only a response header; drop the body read entirely |
| V6 Cryptography | no change | — |
| V14 Configuration | yes | Bundle ID/entitlements: `build/entitlements.plist` has only `network.client`/`network.server`, nothing bundle-ID-specific [VERIFIED: cat] |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Tailnet host forges `X-Portty: 1` to appear as a peer | Spoofing | Same trust level as before (tailnet membership). Listing a peer grants nothing: attach still needs a capability from that peer. Document that the marker is not an auth signal |
| Product fingerprinting through the header on Funnel-public routes | Information disclosure | Header on one route only. Funnel itself is removed in Phase 4 |
| Reading AgentHub's `capability.key`/settings | Elevation / Tampering | Fresh start: separate dir, no migration code (locked) |
| Update check to the wrong repo | Spoofing | Slug is one constant; HTTPS GitHub API; the check never downloads |
| Socket permission regression during rename | Elevation | Keep `os.MkdirAll(dir, 0700)` in all three builders |

## Sources

### Primary (HIGH confidence)
- Repo source read this session: `wails.json`, `build/darwin/Info.plist`, `Info.dev.plist`, `main.go`, `app.go`, `tray.go`, `tray_common.go`, `tray_objc_darwin.m`, `notification_darwin.go`, `appearance_darwin.{go,m}`, `internal/tailnet/tailnet.go`, `internal/daemon/{socket,engine,service,process}.go`, `internal/webserver/{server,capability_mw}.go`, `internal/updater/updater.go`, `scripts/verify-icons.sh`, `packaging/homebrew/agenthub.rb.template`, `.github/workflows/{build,release,distribute}.yml`, `build.sh`, `.gitignore`, `build/gen_*.go`, `tests/*`
- Module cache: `wailsapp/wails/v2@v2.10.2` (`pkg/commands/build/packager.go`, `pkg/buildassets/buildassets.go`, `internal/project/project.go`, `cmd/wails/build.go`, `pkg/runtime/runtime.go`); `creativeprojects/go-selfupdate@v1.5.2` (`detect.go`, `github_source.go`, `config.go`, `arch.go`)
- macOS SDK header `AppKit.framework/Headers/NSApplication.h:552-568` (About panel option keys)
- `go doc os.UserConfigDir`; on-disk `~/Library/Application Support/agenthub/`, `build/bin/agenthub.app` (plist, executable name, icns cmp)
- Experiments: `go:embed` with `@` filename; module-rename gofmt dry run; `sips` asset sizes

### Secondary (MEDIUM confidence)
- raw.githubusercontent.com/wailsapp/wails/v2.16.0/v2/pkg/commands/build/packager.go (confirms v2.16.0 packager matches v2.10.2)

### Tertiary (LOW confidence)
- NSImage multi-rep Retina behaviour, `UNUserNotificationCenter` per-bundle permission semantics, hdiutil DMG equivalence (training knowledge; see Assumptions Log)

## Metadata

**Confidence breakdown:**
- Standard stack / build behaviour: HIGH (source plus on-disk artifacts)
- Architecture / plan split: HIGH (dependencies traced: embeds, shared `.m` file, atomic module rename)
- Pitfalls: HIGH for 1-5 and 7-11 (verified); MEDIUM for 6 (verified both copies exist; the fix is trivial)
- Native rendering (Retina reps, About link, notification attribution): MEDIUM/LOW, covered by manual M-items

**Research date:** 2026-11-16
**Valid until:** 2026-12-16 (stable; re-check if the Wails CLI or go-selfupdate version changes)
