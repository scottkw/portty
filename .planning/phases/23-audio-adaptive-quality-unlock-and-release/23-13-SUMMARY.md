---
phase: 23-audio-adaptive-quality-unlock-and-release
plan: 13
subsystem: release
tags: [release, rc, dmg, notarization, updater, m-172]
requires:
  - phase: 18.1-apple-silicon-release-pipeline
    provides: release.yml two-name DMG publish, verify-bundle.sh, TestRealDMG
  - phase: 23-audio-adaptive-quality-unlock-and-release
    provides: 23-12 TESTING.md M-172 From Phase 23 note
provides:
  - v2.0.0-rc.1 and v2.0.0-rc.2 draft prereleases, both at db46cb91, verified end to end
  - 23-RELEASE-CHECKS.md rows R-00..R-07, R-12, R-13, all PASS
  - TESTING.md M-172 Last run for steps 1 to 3
affects: [23-14, 23-15, 23-16]
requirements-completed: [DCTL-07, DESK-07, UNLK-01]
actuals:
  tokens: 3550
  tasks: 3
  commits: 2
plan_head_before: db46cb913b42212f7727852aa0bc6ddb8b32e7ac
plan_head_after: 9ee8e3e4453cabaa6ffd8495a4179895403f00a3
key-files:
  created: []
  modified: [.planning/phases/23-audio-adaptive-quality-unlock-and-release/23-RELEASE-CHECKS.md, TESTING.md]
key-decisions:
  - "rc.2 was tagged on the same SHA as rc.1 (db46cb91), so R-00's gates cover both; the two builds still differ (own version stamp, own DMG identifiers and checksums), which is what 23-14's two-build install needs"
status: complete
---

# Phase 23 Plan 13: v2.0.0-rc.1 and rc.2 release dry runs Summary

Both rc tags ran the Phase 18.1 pipeline green on every gate, produced draft prereleases with both DMG names and matching two-line checksums, and all four DMGs pass the updater's real-tool verify (TestRealDMG) and `verify-bundle.sh --notarized` on the read-only mounted app.

Tagged SHA: `db46cb913b42212f7727852aa0bc6ddb8b32e7ac` (v2.0.0-rc.1 and v2.0.0-rc.2, pushed by the user; Release runs 46168334228 and 46168355893).

`commits: 2` is measured from the ledger (`git rev-list --count db46cb91..9ee8e3e4`) and counts the Task 1 and Task 3 commits; Task 2 was the user's push and made no commit. This SUMMARY is committed after them.

## Commits

| Task | Commit | What |
|------|--------|------|
| 1 | `8851430b` | 23-RELEASE-CHECKS.md criteria and R-00 preflight PASS |
| 2 | (none) | User pushed v2.0.0-rc.1 and v2.0.0-rc.2 at `db46cb91` |
| 3 | `9ee8e3e4` | R-01..R-07, R-12, R-13 results; TESTING.md M-172 Last run |

## Rows

| Row | Result | One line |
|-----|--------|----------|
| R-00 | PASS | Local release gates, full Go race suite, vitest and CI Build green on `db46cb91` (Task 1) |
| R-01 | PASS | rc.1 Release run success, every step in validate, build-macos, sign-macos and publish success |
| R-02 | PASS | rc.1 draft and prerelease, exactly the two DMG names plus checksums.txt, notes open with the Apple Silicon line |
| R-03 | PASS | checksums.txt two lines, both sums match the downloads (and GitHub's asset digests) |
| R-04 | PASS | TestRealDMG on the rc.1 universal DMG passed, not skipped; identifier equals the asset name |
| R-05 | PASS | The same for the rc.1 arm64 DMG |
| R-06 | PASS | Mounted rc.1 universal app: verify-bundle.sh --notarized exit 0, DR equals v1.2's, arm64, minos 14.0; detached |
| R-07 | PASS | The same for the rc.1 arm64 DMG |
| R-12 | PASS | rc.2 run green, draft prerelease with both rc.2 names, two-line checksums match |
| R-13 | PASS | Both rc.2 DMGs pass TestRealDMG (not skipped) and mounted verify-bundle.sh --notarized; detached |

Host hygiene: downloads went to fresh scratch directories in the session scratchpad, each DMG was mounted with `hdiutil attach -readonly -nobrowse -noautoopen` on a scratch mountpoint and detached, nothing from a DMG was opened or run, and the scratch downloads and mountpoints were deleted. `mount` shows no Portty volume. No release file changed.

## Next

- 18.1-UAT.md's M-172 item can now be run with `/gsd-verify-work 18.1` (steps 1 to 3 have their evidence here; step 4 stays for 23-16 at the real tag).
- 23-14: the owner installs rc.1 from its draft on the office Mac, then rc.2 (M-169 part 1, M-177).

## Deviations from Plan

None - plan executed exactly as written.

## Observations (not failures)

- Both drafts show `targetCommitish` main in the release JSON; the tags themselves point at `db46cb91`, and GitHub publishes a draft against its existing tag, so this has no effect.
- Both rc bundles carry CFBundleShortVersionString 2.0.0 (release.yml strips the prerelease suffix for the plist; `main.Version` keeps v2.0.0-rc.N). The in-app updater skips prereleases (`internal/updater/update.go`), so rc.1 to rc.2 is a manual install in 23-14, not an updater offer.

## Self-Check: PASSED

- 23-RELEASE-CHECKS.md: 9 of R-01..R-07, R-12, R-13 carry RESULT=PASS (plus R-00).
- TESTING.md: one Last run line names v2.0.0-rc.1 with 2026-11-28.
- No Portty volume mounted; `bash tests/check-brand.sh` clean; `bash tests/check-traceability-paths.sh` OK.
- Commits `8851430b` and `9ee8e3e4` are ancestors of HEAD.
