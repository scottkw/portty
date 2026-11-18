---
phase: 06-release-updates
reviewed: 2026-11-18T13:40:00Z
depth: deep
files_reviewed: 22
files_reviewed_list:
  - internal/updater/update.go
  - internal/updater/install.go
  - internal/updater/service.go
  - internal/updater/update_test.go
  - internal/updater/install_test.go
  - internal/updater/service_test.go
  - internal/updater/real_darwin_test.go
  - internal/updater/release_workflow_test.go
  - app_update.go
  - app_update_test.go
  - app_update_release_test.go
  - updates_vmpreview.go
  - cmd/fakegh/main.go
  - app.go
  - main.go
  - tray.go
  - tray_objc_darwin.m
  - .github/workflows/release.yml
  - frontend/src/App.tsx
  - frontend/src/components/UpdateBanner.tsx
  - frontend/src/wailsjs/go/main/App.d.ts
  - frontend/src/wailsjs/go/main/App.js
findings:
  critical: 0
  warning: 4
  info: 6
  total: 10
  necessity: 0
status: issues_found
---

# Phase 6: Code Review Report

**Reviewed:** 2026-11-18T13:40:00Z
**Depth:** deep
**Files Reviewed:** 22
**Status:** issues_found

## Summary

Scope: `git diff 485c45d6..HEAD -- . ':!.planning'`, the updater port (check, download, verify, reveal), the app wiring (poller, alerts, tray, notify-once), the vmpreview fake feed, the release workflow's DMG notarize/staple/verify steps, and the banner frontend. I also read the Wails v2.10.2 internals the code relies on: bound calls run on their own goroutines, `EventsEmit` uses `ExecJS` (async on the main thread), and `MessageDialog` uses `runModal` behind a `dialogLock`.

Gates, re-run for this review: `go vet` is clean on `./internal/updater/ . ./cmd/fakegh/`. `go test -race -short` passes on `./internal/updater/` and `.` with the two known skips. The frontend banner and App.update vitest suites pass (22/22).

These parts hold up:
- **Download path.** The bytes that are hashed are the bytes written to disk (`io.MultiWriter(f, h, pw)`). The size is capped while streaming (`LimitReader(a.Size+1)` plus an exact count check). `.part` uses O_EXCL and mode 0600, and a symlink left at `.part` is removed first. Verify, quarantine and reveal all act on the same path, so there is no verify/reveal TOCTOU that crosses a privilege boundary.
- **URL checks.** The allowlist compares hosts exactly. The redirect check runs on every hop for both clients.
- **ETag/304.** It is correct.
- **State machine.** One operation runs at a time. Check returns early while busy, Download refuses unless the state is available or revealed, and every transition happens under `s.mu`.
- **Poller.** It drops the monotonic clock reading, so a laptop that sleeps still sees a 24 h cadence by the wall clock.
- **fakegh.** It refuses any address that is not loopback.
- **CI verify step.** The release.yml verify step runs the same checks as the client.

**Main concern (WR-01).** The client's Verify proves "a notarized DMG signed by Team S2K7P43927". It does not prove "the DMG of Portty vX.Y.Z". Team S2K7P43927 also signs GIFsmith, and an older genuine Portty DMG passes the same checks. Anyone who can change the release JSON or upload a release asset can get a rollback, or a different product, verified and shown in Finder as the new version. I checked locally that the DMG's signed `Identifier=` keeps the original file name after a rename, so binding it is a one-line fix.

**vmpreview containment.** The release build passes a literal `-tags wailsassets` (release.yml:92, build.sh:88), so the vmpreview tag cannot reach a release today. The gate that enforces this (`tests/check-brand.sh`) only runs in build.yml, though, and release.yml neither depends on build.yml nor runs the check itself (WR-02).

## Warnings

### WR-01: Verify does not bind the DMG to the release version or product (rollback / cross-product substitution)

**File:** `internal/updater/install.go:128-145` (also `internal/updater/service.go:174`, `.github/workflows/release.yml:213`, `.github/workflows/release.yml:227-238`)
**Issue:** `Verify` accepts any DMG that:
- passes `codesign --verify --strict`,
- has `TeamIdentifier=S2K7P43927`,
- and has spctl `source=Notarized Developer ID`.

The asset name `portty-v{X.Y.Z}-darwin-universal.dmg` and the digest both come from the release JSON. GitHub computes the digest for whatever file was uploaded, so neither one binds the file's contents to version X.Y.Z. Two cases pass every check:
- **Rollback.** An older genuine Portty DMG (for example, one with a known vulnerability), re-uploaded as `portty-v9.9.9-darwin-universal.dmg` on a new release.
- **Cross-product.** Any genuine GIFsmith DMG. It is the same team (06-CONTEXT: "same team as GIFsmith") and is notarized.

Either one is then shown as "Portty 9.9.9 is verified and shown in Finder." The attacker needs only release-asset write access to the repo, or control of the API response. They do not need the signing secrets in the `release` environment. For an app whose main risk is remote shell access, offering a rollback is the supply-chain gap that matters.

`codesign` seals the DMG's signing identifier into the signature, and renaming the file does not change it. I checked this locally:

```
hdiutil create ... portty-v1.2.3-darwin-universal.dmg; codesign --force --sign - portty-v1.2.3-...dmg
cp portty-v1.2.3-...dmg portty-v9.9.9-...dmg; codesign -dvv portty-v9.9.9-...dmg
  -> Identifier=portty-v1.2.3-darwin-universal
```
**Fix:**
1. In release.yml, make the identifier an explicit contract.
2. In `Verify`, require the identifier line.
3. Pass the expected name from `Service.Download` (`Verify(ctx, s.Run, path, TeamID, rel.DMG.Name, now())`).
4. Mirror the check in the release.yml verify step and in `release_workflow_test.go`.

```yaml
# release.yml "Create and sign DMG"
codesign --force --timestamp --identifier "portty-${VERSION}-darwin-universal" \
  --sign "$MACOS_CERTIFICATE_NAME" "build/bin/${DMG_NAME}"
# "Verify DMG like the in-app updater"
grep -qx "Identifier=portty-${VERSION}-darwin-universal" <<< "$DVV" || { echo "FAIL: DMG identifier"; exit 1; }
```
```go
// install.go
func Verify(ctx context.Context, run Runner, path, teamID, assetName string, now time.Time) error {
	...
	out, err := run(ctx, "/usr/bin/codesign", "-dvv", path)
	if err != nil || !hasLine(out, "TeamIdentifier="+teamID) ||
		!hasLine(out, "Identifier="+strings.TrimSuffix(assetName, ".dmg")) {
		return fmt.Errorf("%w: team", ErrVerify)
	}
```
Releases already published before this change carry the default identifier, which is the same string because it comes from the same file name. So the check also works against them. Add an install_test case with a wrong identifier.

### WR-02: The vmpreview gate is not enforced by the release pipeline

**File:** `.github/workflows/release.yml:9-36` (gate lives only in `.github/workflows/build.yml:26-27`)
**Issue:** `updates_vmpreview.go:1-7` says the vmpreview tag must never reach a release, and that `tests/check-brand.sh` enforces this in CI. That script runs only in build.yml. release.yml triggers on any `v*` tag push, does not depend on build.yml, and has no brand-gate step. A tag can point at any commit, including a branch commit that never passed the protected `build` check. If that commit's release.yml carries the tag (or `GOFLAGS=-tags=...vmpreview`), the release builds, signs, notarizes and publishes a binary whose updater talks plain http to `127.0.0.1:8788` and whose Tailscale and share code are faked. The `!vmpreview` test `app_update_release_test.go` cannot catch this: `go test ./...` in `validate` builds without the tag, so it checks the test binary, not the shipped one.
**Fix:** Run the gate in the release workflow, and check the actual artifact before it is tarred:
```yaml
# validate job, before tests
- name: Brand / vmpreview gate
  run: bash tests/check-brand.sh
# build-macos, after "Build macOS app"
- name: Release binary carries no VM-only fakes
  run: |
    if grep -aq '127.0.0.1:8788' build/bin/Portty.app/Contents/MacOS/Portty; then
      echo "FAIL: vmpreview fake update feed in release binary"; exit 1
    fi
```

### WR-03: Tests delete and overwrite the real user's `update_notified` file

**File:** `app_update_test.go:237-250` (through `app_update.go:124` -> `app.go:875-882`)
**Issue:** `notifiedVersionPath()` resolves to `os.UserConfigDir()/portty/update_notified`, which is `~/Library/Application Support/portty/update_notified` on the developer's Mac. `resetNotified` removes that file before and after each test, and the poller and manual-check tests write `9.9.9` into it. Running `go test .` on a machine that also runs an installed Portty (the author's setup) wipes that Portty's notify-once record, so the user gets a duplicate "Version X is available" notification. A real Portty writing the same file during a test run can also make the `notifiedFile` assertions flaky. This is the same class of leak as the open todo "daemon settings tests leak host settings.json".
**Fix:** On darwin, `os.UserConfigDir` reads `$HOME`, so isolate it per test:
```go
func resetNotified(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // configDir() -> temp; never touches the real Application Support
}
```
Alternatively, add a `notifiedPath string` field on App as a seam.

### WR-04: A newer release whose DMG is not yet installable counts as "up to date" for 24 h, and the manual check states something false

**File:** `internal/updater/service.go:98-99,116-117`, `app_update.go:97-108`, `app_update.go:181-182`
**Issue:** `Checker.Latest` returns `ErrNotInstallable` when a newer release has no valid DMG yet. `Service.Check` turns that into `State: "current"` with a nil error. Two things follow:
- **Auto check.** `autoCheck` treats this as a completed check and sets `last`, so the next check is 24 h away.
- **Manual check.** `checkForUpdatesManually` shows "Portty is up to date. Version 1.0.0 is the newest version." while 1.1.0 is published.

release.yml makes this window real. `softprops/action-gh-release` (release.yml:282-288) creates a published (non-draft) `vX.Y.Z` release and then uploads the DMG. While the upload runs, the asset is missing or has `state != "uploaded"`. Any Portty whose daily check lands in that window waits a further day. The same happens permanently if the upload fails. 06-CONTEXT lists 404, drafts, prereleases and bad tags as "up to date", but not "newer release without a usable DMG".
**Fix:** Count not-installable as an incomplete check so the hourly retry picks it up, and do not claim "newest version". For example, return a sentinel and treat it like a quiet failure:
```go
case errors.Is(err, ErrNotInstallable):
	log.Printf(...)
	return s.set(prev), &Error{Kind: "check", Text: "Version " + rel.Version + " isn't ready to download yet. Try again later."}
```
`autoCheck` already returns false on error, so it retries hourly and stays quiet. Also consider publishing as a draft and un-drafting after the upload (`draft: true` plus `gh release edit --draft=false`) to close the window at the source.

## Info

### IN-01: A backwards wall-clock jump stops automatic checks until the clock catches up

**File:** `app_update.go:86`
**Issue:** `now().Round(0).Sub(last) < updateEvery` is also true for negative durations. Suppose the clock was wrong in the future when a check completed (manual change, bad RTC before NTP) and is then corrected. `last` is now in the future, and no automatic check runs until wall time passes `last + 24h`. That can be months, or until Portty restarts. The remote Mac may run Portty for weeks without a restart.
**Fix:** `if d := now().Round(0).Sub(last); !last.IsZero() && d >= 0 && d < updateEvery { continue }`

### IN-02: Verify inherits what is left of the download's 10-minute deadline; a slow download is reported as a signature failure

**File:** `internal/updater/service.go:148,174-178`
**Issue:** `Verify`, which runs `codesign` and `spctl`, gets the same `ctx` as the download. If the download finishes near the 10-minute mark, the tools are killed mid-run. The user is told "The download didn't pass the signature check, so Portty deleted it." and a good file is deleted. On a security feature, a misleading verify failure teaches users to ignore it.
**Fix:** Give Verify and Reveal their own short context, for example `vctx, vcancel := context.WithTimeout(context.Background(), 2*time.Minute)`. Alternatively, map `ctx.Err() != nil` to the download-failed text.

### IN-03: A manual check queued behind an automatic check shares its 30 s budget

**File:** `app_update.go:170-171`, `internal/updater/service.go:78`
**Issue:** The manual check's 30 s context starts before `checkMu.Lock()`, and that lock does not respect the context. If an automatic check holds the lock for most of its own 30 s, the manual check gets about 0 s for `Latest` and shows "Portty couldn't reach GitHub" even when the network is fine.
**Fix:** Create the manual context after acquiring `checkMu`. For example, pass a timeout duration instead of a context, or have `Check` build the timeout itself.

### IN-04: The tray item in the revealed state re-downloads instead of re-revealing

**File:** `app_update.go:191-193`, `internal/updater/service.go:150`
**Issue:** After a successful reveal the tray still shows "Update Available…". Clicking it offers "Version X is available. Download". Choosing Download runs `Clean`, which deletes the verified DMG (possibly still mounted or open in Finder), then downloads and verifies again.
**Fix:** In `offerUpdate`, handle `"revealed"` by calling `updater.Reveal` on the kept file, or change the tray title to "Show Update in Finder" for that state.

### IN-05: Clicking the update notification has no handler

**File:** `tray_objc_darwin.m:84-95` (delegate implements only `willPresentNotification:`), `app_update.go:133`
**Issue:** `PorttyNotificationDelegate` does not implement `userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:`, so no app code runs when the user clicks "Version X is available." With the window hidden (StartHidden or quit-to-tray, which is the remote-Mac setup), the click most likely activates the app without showing the window or the banner. Confirm this in the Tart VM.
**Fix:** Implement the response method. For the `portty.update-available` identifier, call an exported Go hook that runs `offerUpdate()` on a goroutine, and call the completion handler.

### IN-06: The banner's mount snapshot can overwrite a newer event

**File:** `frontend/src/App.tsx:731-732`
**Issue:** `GetUpdateStatus().then(apply)` and `EventsOn('update:status', apply)` race. The snapshot is taken in Go before a later event is emitted, but the call reply can arrive after that event, rolling the banner back (for example, from `downloading` to `available`) until the next event arrives. The window is small (the first check is 10 s after launch) but can happen during a download started from the tray before the window loads.
**Fix:** Drop the snapshot if any event has already been applied:
```ts
let gotEvent = false
const offUpdate = EventsOn('update:status', (s) => { gotEvent = true; apply(s) })
GetUpdateStatus().then((s) => { if (!gotEvent) apply(s) }).catch(() => {})
```

---

_Reviewed: 2026-11-18T13:40:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
