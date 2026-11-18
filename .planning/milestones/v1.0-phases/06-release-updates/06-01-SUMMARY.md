---
phase: 06-release-updates
plan: 01
subsystem: updater
status: complete
tags: [updater, github-releases, codesign, notarization, release-pipeline]
requires: []
provides:
  - "internal/updater: Checker.Latest (ETag, ErrRateLimited), NewCheckClient, NewDownloadClient, Download, Verify, Reveal, Clean, ExecRunner"
  - "release.yml: DMG notarized, stapled and checked like the client before upload"
affects: [06-02, 06-04]
tech-stack:
  added: []
  patterns: ["injectable Runner for system tools by absolute path", "Go contract test pinning a workflow file"]
key-files:
  created:
    - internal/updater/update.go
    - internal/updater/install.go
    - internal/updater/update_test.go
    - internal/updater/install_test.go
    - internal/updater/real_darwin_test.go
    - internal/updater/release_workflow_test.go
  modified:
    - .github/workflows/release.yml
decisions:
  - "Ported GIFsmith internal/update nearly line for line; Portty adds ErrRateLimited (403/429), NewCheckClient (30 s, same-scheme-and-host redirects, max 5) and Reveal (open -R, wraps ErrReveal)"
  - "release.yml verify step greps exact lines (grep -qx) for TeamIdentifier and source=Notarized Developer ID, matching the client's hasLine"
metrics:
  duration: "~25 min"
  completed: 2026-11-18
actuals:
  tokens: 12200
  tasks: 3
  commits: 5
plan_head_before: 39f4341c9749340004aaade1ffb6d3fd187c9829
plan_head_after: 23ec6743924e358038f23fd86f02bb2b07d19ee5
---

# Phase 6 Plan 01: Updater Core Port and Notarized DMG Pipeline Summary

GIFsmith's update core is now `internal/updater` for Portty: an ETag-cached GitHub release check that knows about rate limits, a digest- and size-checked `.part` DMG download, a codesign / Team S2K7P43927 / notarization verify, and reveal in Finder. release.yml now notarizes and staples the DMG, then runs the client's own three checks on it before uploading, so a DMG the client would reject never gets published.

## Tasks

| Task | Name | Commits |
|------|------|---------|
| 1 (tracer) | End-to-end check, download, verify, reveal | 67c72c84 (RED), 721f8ecf (GREEN) |
| 2 | Full GIFsmith test matrix + real tools on real DMGs | aed88e47 |
| 3 | release.yml notarize, staple, self-verify + contract test | c9abf059 (RED), 23ec6743 (GREEN) |

## What was built

- `update.go`: `Repo=scottkw/portty`, `TeamID=S2K7P43927`, `DMGAssetName(v)` = `portty-v{v}-darwin-universal.dmg`, `GitHubAllow`, `Checker.Latest` (only the Accept, User-Agent `Portty` and If-None-Match headers; 304 reuses the cache; 404, drafts, prereleases and bad tags count as no release; 403/429 return `ErrRateLimited`), `NewCheckClient`.
- `install.go`: `NewDownloadClient`, `Download` (0600 `.part` file in a 0700 folder, exact size, SHA-256, rename only on success), `Runner`/`ExecRunner`, `Verify` (quarantine agent `Portty`), `Reveal` (`/usr/bin/open -R`), `Clean` (`portty-v*-darwin-universal.dmg[.part]`).
- `release.yml` sign-macos: new steps "Notarize and staple DMG" (notarytool submit with `--wait --timeout 60m`, then stapler staple and stapler validate) and "Verify DMG like the in-app updater". Both sit between "Create and sign DMG" and "Cleanup keychain". Timeout raised from 90 to 150. Stale "MOVED VERBATIM" banner rewritten. SHA pins, attestation, publish and checksum steps unchanged.

## Gate results

- `go vet ./internal/updater`: clean. `go test -race -count=1 ./internal/updater`: PASS. The new tests and the old go-selfupdate `TestCheck_*` / `TestDetectLatestFrom_*` pass side by side.
- Task 2 matrix count: 21 matching `--- PASS` lines (needed at least 14). No `scottkw/gifsmith` or `Gifsmith-` strings left in internal/updater.
- `TestReleaseWorkflow_*`: 3/3 PASS. `ruby -ryaml` parses release.yml. `stapler staple` appears 2 times, `stapler validate` 1, `timeout-minutes: 150` 1, `MOVED VERBATIM` 0.
- `bash tests/check-brand.sh`: clean.
- **TestRealDMG on the host (real codesign/spctl): PASS.**
  - GIFsmith v1.0.1 `Gifsmith-1.0.1-arm64.dmg` (sha256 `4119ccd1…faa1`, matches): accepted.
  - Byte-flipped copy: rejected at **codesign**.
  - Unsigned UDZO (`unsigned.dmg`): rejected at **codesign**, as expected.
  - Ad-hoc (`codesign -s -`, `adhoc.dmg`): rejected at **team**, as expected (`TeamIdentifier=not set`).
  - UPDATE_REAL_REVEAL was not set. Finder was never opened, and nothing was signed with the host Developer ID.
- Extra check: I pulled the release.yml verify step's shell out through the YAML parser and ran it on the same files. The GIFsmith DMG exits 0. The ad-hoc DMG exits 1 with "not signed by Team S2K7P43927", and the unsigned DMG exits 1 at codesign.

## Deviations from Plan

None in the plan's own scope. Minor additions inside the planned files:
- TestVerifyFailures also checks that each failure names the expected step (`: codesign`, `: team`, `: spctl`, `: quarantine`), not just that it wraps ErrVerify.
- TestLatestNotInstallable also checks that the version (1.1.0) comes back with ErrNotInstallable. It has an "other owner" case (`someone/portty`), and TestGitHubAllow has one too.

## Deferred Issues (out of scope, pre-existing)

- `go test -short ./internal/daemon`: `TestAPIAppearance_GetPatch` and `TestAPITerminalFont_GetPatch` fail on this host. The cause is the host's real `~/Library/Application Support/portty/settings.json` (`appearance: dark`, `terminalFontSize: 16`). It leaks into tests because `testDaemon` in `internal/daemon/api_test.go` does not reset those two fields after `NewSessionEngine()` loads disk settings. This is unrelated to 06-01: daemon does not import updater and no daemon file changed. I did not record it in deferred-items.md, to avoid a file conflict with the parallel 06-03.

## Known Stubs

None.

## Threat Flags

None. All new surface (outbound HTTPS to api.github.com and the release-asset hosts, system tool exec) is covered by T-06-01..T-06-06.

## Self-Check: PASSED

- All 7 key files exist. All 5 commits (67c72c84, 721f8ecf, aed88e47, c9abf059, 23ec6743) are ancestors of HEAD.
