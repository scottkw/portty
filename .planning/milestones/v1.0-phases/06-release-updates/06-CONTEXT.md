# Phase 6: Release Updates — Context

**Gathered:** 2026-11-18 (smart discuss, autonomous)
**Status:** Ready for planning

<domain>
Installed Portty copies learn about new GitHub releases of scottkw/portty on their own (launch + periodic) and on demand ("Check for Updates…"), notify the user, and one click downloads the release DMG, verifies SHA-256 digest + Developer ID Team + notarization, and reveals it in Finder. Port of `~/dev/gifsmith/internal/update`. Requirements UPD-01..UPD-04.
</domain>

<decisions>

### User decisions (2026-11-18)
- **Team ID:** hardcode `S2K7P43927` (same team as GIFsmith).
- **Notarization:** notarize **and staple the DMG** in `.github/workflows/release.yml` (in addition to the .app). The client keeps GIFsmith's full verify: `codesign --verify --strict`, Team ID from `codesign -dvv`, `spctl -a -t open --context context:primary-signature` → `source=Notarized Developer ID`.
- **Cadence:** first check ~10 s after launch, then every 24 h. No user toggle (no new settings; radical simplicity).
- **Manual check feedback:** native macOS alert. "Portty is up to date." / "Version X is available." (with Download + Later) / readable error text. OK dismisses.

### Decided autonomously (reversible technical)
| Area | Decision | Why cheap to undo |
|---|---|---|
| Library | Replace go-selfupdate + Masterminds/semver with a port of GIFsmith `internal/update` (update.go + install.go) into `internal/updater` | Self-contained package; deps removal is local |
| Digest source | GitHub asset `digest` field (`sha256:<hex>`), require `state=="uploaded"`, size cap 200 MiB, exact-host URL allowlist + redirect re-check; `checksums.txt` not used | Same as GIFsmith; swap is local |
| Asset name | `portty-v{X.Y.Z}-darwin-universal.dmg`, built from the validated version, never from JSON | Matches release.yml |
| Version | `v?X.Y.Z` strict; running version that doesn't parse (e.g. `dev`, `-beta`) → skip auto checks; manual check says "This build doesn't check for updates." | Local rule |
| Quiet failures | 404 / drafts / prereleases / bad tags → "up to date"; network errors, 403/429 rate limit → no popup on auto checks, readable error only on manual check; ETag/If-None-Match so 304s don't cost rate limit; periodic checks keep running | Local |
| Throttle | Drop the persisted 1 h `update_check.json` throttle; in-process 24 h ticker + ETag | Local |
| Notification | Native notification once per new version (remember last notified version in config dir), plus the in-app UpdateBanner and a tray "Update Available…" item | Local; one small file |
| State machine | GIFsmith's: idle → checking → current/available → downloading (progress) → verifying → revealed; one op at a time; download on its own 10 min deadline; typed error kinds check/download/verify/reveal | Local |
| After verify | Reveal in Finder (`open -R`), per UPD-03 (GIFsmith opens instead). Keep quarantine xattr set | One line |
| Download dir | `~/Library/Caches/app.portty.Portty/updates`, `.part` 0600 then rename; clean stale DMGs | Local |
| UI | Existing `UpdateBanner.tsx` gets Download → progress → "Verifying…" → "Shown in Finder" / error states; menu label becomes "Check for Updates…" | Local |
| Tests | Fake GitHub API (httptest) for check/404/draft/prerelease/rate-limit/304; injectable Runner for codesign/spctl; vitest for banner states | Local |

### Claude's Discretion
Exact copy beyond the strings above, file split inside `internal/updater`, how the alert is shown (Wails `runtime.MessageDialog` or native).
</decisions>

<code_context>
- `internal/updater/updater.go` (go-selfupdate, swallows errors, 1 h throttle) and `updater_test.go` — to be replaced.
- `app.go:1183-1247` `startUpdatePoller` / `runUpdateCheck` / `GetLastUpdateInfo` / `CheckForUpdates`; `app.go:84-87` `sendNotificationFunc` seam (flagged for Phase 6).
- `main.go:172-182` Help → "Check for Updates" (fire-and-forget); `menu_test.go:106` asserts label.
- `frontend/src/components/UpdateBanner.tsx` (Download opens release URL), `App.tsx:714-725` subscribe, `:1021` render; `App.test.tsx:283-353` source-text assertions.
- `notification_darwin.go:41` `sendNotification`.
- `main.go:33` `Version` (ldflags; `v` prefix from `GITHUB_REF_NAME` in release.yml, `${VERSION}` in build.sh:87).
- `.github/workflows/release.yml`: signs + notarizes + staples .app only; DMG `portty-${tag}-darwin-universal.dmg` codesigned but not notarized; uploads `*.dmg` + `checksums.txt`; `-rc` → draft, `-` → prerelease.
- Reference: `~/dev/gifsmith/internal/update/{update.go,install.go}`, `~/dev/gifsmith/internal/services/updates.go`, errors at `internal/services/errors.go:92`.
</code_context>

<specifics>
- Testing: fake GitHub API in tests and in a Tart VM run (point the checker at a local fake via a build/test seam, not a user setting). A real signed+notarized DMG can't be produced locally; verify-path tests use the injectable Runner, and a VM check confirms reveal-in-Finder with a locally built DMG (expect verify to reject an unsigned DMG — that rejection path is itself a check).
- Keep token-gated share links and CSP untouched; updater makes outbound HTTPS to api.github.com and github release asset hosts only.
</specifics>

<deferred>
- Auto-install / replace-in-place (out of scope: reveal only).
- Settings toggle to disable checks.
</deferred>
