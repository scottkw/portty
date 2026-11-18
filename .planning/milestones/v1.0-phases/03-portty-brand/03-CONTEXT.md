# Phase 3: Portty Brand - Context

**Gathered:** 2026-11-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Everything a user sees or types says Portty: app name and bundle, CLI, Go module, config/data/socket paths, icons and logos, README/Help/in-app text, notifications, and the update feed. Homebrew packaging is removed. A case-insensitive search for "agenthub" over tracked files (excluding `.planning/`) returns zero hits. Requirements: BRND-01..06, NOTF-01, DIST-03.

Not in this phase: one-click sharing and Funnel removal (Phase 4), the full DMG update/download flow (DIST-02, v2), making the repo public (Phase 6).

</domain>

<decisions>
## Implementation Decisions

### Identity and storage
- Bundle ID is `app.portty.Portty` (matches the portty.app domain). App is `Portty.app`; `wails.json` productName Portty.
- Fresh start: Portty never reads, imports or writes AgentHub settings or data (`~/Library/Application Support/agenthub/`). No migration code.
- Config, data and the daemon socket live under `~/Library/Application Support/portty/` (`os.UserConfigDir()` + "portty": AgentHub's real layout, renamed; user confirmed 2026-11-16 after research showed AgentHub never used `~/.config`). Go module path `github.com/scottkw/portty`.
- An AgentHub daemon that is still running is left alone. Portty runs its own `portty daemon`; both apps can run side by side.

### Running alongside AgentHub on the tailnet
- Web/peer port moves from 7443 to 7444 (`internal/tailnet/tailnet.go` DefaultProbePort, webserver bind, docs, tests), so both apps can run on one Mac and AgentHub is never probed as a peer.
- The probe reply carries a Portty-only marker (e.g. an `X-Portty` response header). Peer discovery accepts only replies with that marker; the shared "capability required" text alone no longer qualifies.
- Peers still running AgentHub are not shown at all.
- Old saved links and capabilities are not carried over (follows the fresh start).

### Icons and logos
- Import `portty-brand/` per its README: copy over repo paths, rename `*-2x.png` to `*@2x.png`, regenerate `build/darwin/iconfile.icns` with `iconutil`, use the @2x tray icons on Retina, keep tray icons as template images.
- Delete `build/gen_progress_icons.go` (would overwrite the pre-rendered ring icons).
- Do not copy the Linux or Windows icons (macOS only).
- Delete `portty-brand/` after the files are in place, so each asset has one copy. Replace and delete the `agenthub-title-logo.png` copies (docs/, frontend/public/, frontend/src/assets/) with `portty-title-logo{,-dark}.png`.

### Text, docs and updates
- Start a fresh Portty `CHANGELOG.md`; AgentHub history stays in git.
- README is a short rewrite: what Portty is (a macOS terminal with remote access over Tailscale), install from the DMG linked at https://portty.app, two-Mac setup, MIT license.
- Update check queries `scottkw/portty`. When no release exists yet (or the API 404s while the repo is private), treat it as up to date with no error shown.
- Delete AgentHub-era design docs (`docs/plugin-architecture-assessment.md`, stale `docs/superpowers/` material) rather than rewriting them.
- Remove `packaging/homebrew` and every Homebrew mention in docs, scripts and workflows (DIST-03, already decided).

### Research follow-ups (orchestrator defaults, 2026-11-16)
- Wails bundle name comes from `wails.json` `name`: set both `name` and `outputfilename` to `Portty`; bundle ID goes in `build/darwin/Info.plist` and `Info.dev.plist`.
- Add the go-selfupdate `Filters` setting so a `.dmg` release asset is matched and the "newer release shows the banner" case works.
- Delete `build/gen_icon.go` too (regenerates old AgentHub art, same reason as `gen_progress_icons.go`).
- `internal/daemon/path.go` keeps adding `/opt/homebrew/bin` to PATH (finds the Tailscale CLI; code, not Homebrew distribution). Only stale Homebrew comments go.
- README Install gets one `ln -s` line to put `portty` on PATH, since Homebrew used to do that.
- Replace `brew install create-dmg` in `release.yml` with `hdiutil` (DIST-03).
- CLI `web start` binding 443 is left for Phase 4.
- Renaming the `build.yml` matrix entry changes the required check name; re-applying branch protection is a manual step noted for when CI is re-enabled.

### Claude's Discretion
- Exact marker header name and value; rename order and plan split; how test fixtures like `tests/fixtures/settings_agenthub_v4.2.json` are renamed or replaced; wording of in-app strings within the macOS-native tone set in Phase 2.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `portty-brand/` has app icon, AppIcon.iconset, tray icons (default, error, progress 25/50/75/100, each with -2x), and title logos light/dark.
- `tray.go` embeds `assets/tray_icon*.png` via `go:embed`; `tray_test.go` checks sizes (18x18), so @2x handling needs test updates.

### Established Patterns
- Config dir built in three places: `app.go` `configDir()`, `internal/daemon/engine.go` `daemonConfigDir()`, `internal/daemon/socket.go` `DefaultSocketPath()`, all `filepath.Join(base, "agenthub")`.
- Update check: `app.go` calls `updater.Check(ctx, configDir(), "scottkw/agenthub", ...)` (two call sites); `internal/updater` uses go-selfupdate.
- Peer probe: `internal/tailnet/tailnet.go` `agentHubProbeMarker = "capability required"`, `isAgentHubProbeResponse`, `DefaultProbePort = 7443`; webserver returns the marker in `internal/webserver/server.go`.

### Integration Points
- Biggest reference counts: CHANGELOG.md (99), README.md (76), TESTING.md (42), cmd_cli.go (29), tray_objc_darwin.m (24), .github/workflows/release.yml (20), tailnet (15+17 tests), main.go, app.go.
- `wails.json` productName, `build/darwin/Info.plist` CFBundleIdentifier, `build.sh`, `.github/workflows/{release,distribute}.yml`, `web/join.html`.
- Builds use `wails build -platform darwin/arm64 -tags wailsassets`; the output path will change from `build/bin/agenthub.app` to `build/bin/Portty.app` (update the VM deploy helper).

</code_context>

<specifics>
## Specific Ideas

- Brand: name Portty (CLI `portty`), mark is lowercase "p" with cursor bar, blue #2457F5, wordmark Bricolage Grotesque 700 (from `portty-brand/README.md`).
- Both Macs need the rebranded build at the same time, because the port and marker change makes old builds invisible as peers.

</specifics>

<deferred>
## Deferred Ideas

- Phase 2 human UAT items 2-5 (deferred, resume with `/gsd-verify-work 2`).
- Phase 5: replace the real overnight wait with accelerated tests (user question 2026-11-15).

</deferred>
