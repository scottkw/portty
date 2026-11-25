---
phase: 03-portty-brand
fixed_at: 2026-11-16T19:35:28Z
review_path: .planning/phases/03-portty-brand/03-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 3: Code Review Fix Report

**Fixed at:** 2026-11-16T19:35:28Z
**Source review:** .planning/phases/03-portty-brand/03-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (WR-01..WR-04, plus IN-01, which WR-01 depends on)
- Fixed: 5
- Skipped: 0

Scope note: WR-04 carries `**Category:** necessity`, which `critical_warning` normally excludes. The orchestrator put it in scope explicitly. IN-02..IN-09 were out of scope and remain open in REVIEW.md.

## Fixed Issues

### IN-01: `setTraySessionData` stores autoreleased arrays in globals without retaining them

**Files modified:** `tray_objc_darwin.m`
**Commit:** df89629b
**Applied fix:** The main-queue block now releases the previous `menuSessionNames`/`menuSessionIDs` and stores `[nameArr retain]`/`[idArr retain]`. The globals own their arrays, so they no longer depend on the never-drained Go-thread pool. Committed before WR-01 so the pool fix cannot leave the globals dangling.
**Status:** fixed: requires human verification (MRC ownership change; a mistake shows up only as a crash when the tray menu opens in the live app)

### WR-01: Tray icon NSData is autoreleased on a Go thread with no pool

**Files modified:** `tray_objc_darwin.m`
**Commit:** a573d088
**Applied fix:** Wrapped the bodies of the four Go-called tray entry points that autorelease on the cgo thread in `@autoreleasepool`: `initStatusItem`, `updateTrayIcon`, `updateTrayTooltip` (also called on every poll, with the same leak), and `setTraySessionData` (safe after IN-01). Added a one-line comment explaining why. The copied dispatch block retains its captured objects under MRC, so the data outlives the pool drain. I did not add the optional "skip the cgo call when `(connected, quartile)` is unchanged" change, because the pool removes the leak.
**Status:** fixed: requires human verification (MRC memory management. Confirm in the Tart VM that the tray icon, tooltip and session menu still work after several 5 s polls, and optionally that `leaks`/Activity Monitor shows flat memory)

### WR-02: About box shows "Version 2.2.0" instead of the release tag

**Files modified:** `wails.json`, `.github/workflows/release.yml`, `TESTING.md`
**Commit:** d64b525a
**Applied fix:** `wails.json` `info.productVersion` is now the neutral `0.1.0`. release.yml has a new "Stamp bundle version from tag" step before `wails build`. It takes `GITHUB_REF_NAME`, strips the leading `v` and any pre-release suffix (plist versions must be numeric, so `v1.2.0-beta.1` becomes `1.2.0`), fails the job unless the result is `X.Y.Z`, and writes the version into wails.json with `jq --arg`. `main.Version` and the bundle version therefore come from the same tag. TESTING.md M-72 now expects "Version 0.1.0" on a local build, or the tag's X.Y.Z on a release build. build.sh is unchanged: local builds show 0.1.0.
**Status:** fixed

### WR-03: Pre-release tags other than `-rc` publish as full releases

**Files modified:** `.github/workflows/release.yml`
**Commit:** b5eabad6
**Applied fix:** `draft: ${{ contains(github.ref_name, '-rc') }}` (switched from `github.ref` to `ref_name`) and a new `prerelease: ${{ contains(github.ref_name, '-') }}`, so any `-rc`/`-beta`/`-alpha`/… tag becomes a GitHub prerelease, which go-selfupdate skips. I did not take the optional tighter `dmgAssetFilter`.
**Status:** fixed

### WR-04: RELEASE_PUBLISH_TOKEN PAT kept for a downstream workflow this phase deleted

**Files modified:** `.github/workflows/release.yml`
**Commit:** 684bae6f
**Applied fix:** I checked that no remaining workflow (`build.yml`, `e2e.yml`, `release.yml`) has a `release:` or `workflow_run` trigger, and that `RELEASE_PUBLISH_TOKEN` appears nowhere else in tracked files outside `.planning/`. I then removed the `token:` input, so softprops/action-gh-release uses the job's `GITHUB_TOKEN` (the publish job already has `contents: write`). I replaced the stale comment with a note that the `RELEASE_PUBLISH_TOKEN` repo secret is unused and can be deleted. I did not touch any GitHub settings.
**Status:** fixed: requires human verification (whoever has access should delete the `RELEASE_PUBLISH_TOKEN` secret in the repo settings)

## Verification

All gates ran in the main checkout (`the repo root`, branch `gsd/phase-03-portty-brand`) after the last fix commit. There was no worktree.

- `go build ./... && go vet ./... && go test -race -short ./...`: passed, no failing packages
- `pnpm -C frontend exec tsc --noEmit`: passed
- `pnpm -C frontend exec vitest run`: 1173/1173 passed
- `bash tests/check-traceability-paths.sh`: OK (106 paths)
- `bash tests/check-brand.sh`: clean
- release.yml parses as YAML. I ran the stamp step's shell logic locally: `v1.2.0` → 1.2.0, `v1.2.0-beta.1` → 1.2.0, `v1.2` and `vfoo` rejected.
- Not run: a wails build or a GUI check (no GUI work in this pass), `tests/build-script.test.sh` (build.sh unchanged), and the release workflow itself (it needs a tag push).

---

_Fixed: 2026-11-16T19:35:28Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
