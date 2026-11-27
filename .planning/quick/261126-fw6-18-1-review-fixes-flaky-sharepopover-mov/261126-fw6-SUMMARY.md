---
phase: quick-261126-fw6
plan: 01
status: complete
subsystem: release-pipeline, updater, frontend-tests
tags: [18.1-review, build.sh, verify-bundle, sign-bundle, mover, keepDaemon, vitest-flake]
key-files:
  modified:
    - build.sh
    - scripts/verify-bundle.sh
    - scripts/sign-bundle.sh
    - tests/verify-bundle.test.sh
    - tests/build-script.test.sh
    - frontend/src/components/__tests__/SharePopover.test.tsx
    - app_mover.go
    - app_mover_test.go
    - internal/updater/installer.go
    - internal/updater/installer_test.go
    - TESTING.md
commits: [8284f052, b1ec397d, 4eb9de0e]
completed: 2026-11-26
---

# Quick 261126-fw6 Summary

## Done

- **CR-01** `build.sh --sign` ends with `verify-bundle.sh "$APP" --notarized` (stapled ticket + `spctl -a -t exec` as Notarized Developer ID) instead of `spctl --type install`; the notarize zip is removed right after submit. CI release path unaffected: release.yml already ran `verify-bundle.sh build/bin/Portty.app --notarized` after stapling (pinned by `release_workflow_test.go`), and its DMG check uses `spctl -a -t open --context context:primary-signature`, untouched.
- **WR-01** a missing jq now fails the entitlements gate (`FAIL: entitlements gate needs jq on PATH`); stderr is no longer part of the compared values; the header lists the dependencies.
- **WR-02** the fake codesign is per file; new cases: unsigned, runtime-less and timestamp-less nested helper, bad main executable (each FAILs only on its own path), jq missing, and five `--notarized` cases with fake xcrun/spctl.
- **WR-03** sign-bundle.sh names a missing CFBundleIdentifier (the existing message, now reachable) or CFBundleExecutable instead of exiting silently.
- **SharePopover** Copied! case uses plain `vi.useFakeTimers()`.
- **Mover keepDaemon** `takeStaleKeep` takes the put-back result back before a move job starts when the rollback's new version is at or below the version being installed; restored if the installer never starts; kept for Cancel, open-installed and a possibly newer kept daemon (WR-04). The installer drops the job's keepDaemon on a healthy update.

## Evidence

- Mutation checks: against the pre-fix scripts the new verify-bundle cases fail (jq, both WR-03 cases); with the per-file loop deleted 11 cases fail; with `takeStaleKeep` disabled 5 mover cases fail; with the installer line removed the healthy case fails.
- `go test -race . ./internal/updater/` ok; `go vet ./...` and `go vet -tags vmpreview ./...` clean.
- tests/verify-bundle.test.sh 46/46 (was 34); tests/build-script.test.sh 49/49; check-brand OK; check-traceability-paths OK (237).
- SharePopover.test.tsx: 50 consecutive runs, 50 pass.

## Pending

- VM row for the mover stale-keepDaemon path (QT-FW6): PENDING. portty-test is in use by Phase 19; run when free: leave a failed keepDaemon result with no GUI start, open a newer DMG with an older copy installed, Replace, and expect `updated` with sessions intact.
- 18.1 review IN-01..IN-03 stay open (info).
