---
phase: 03-portty-brand
plan: 06
subsystem: docs-ci-release
status: complete
tags: [brand, ci, release, hdiutil, codesign, readme, changelog, testing-md, brand-gate]
requires:
  - "In-app and web text plan (03-05): every code, HTML, plist and test file already says Portty"
  - "App identity plan (03-04): wails build writes build/bin/Portty.app, bundle ID app.portty.Portty, build/darwin/iconfile.icns"
  - "Update feed plan (03-02): updater dmgAssetFilter ^portty-v?\\d+\\.\\d+\\.\\d+.*-darwin-universal\\.dmg$"
provides:
  - "tests/check-brand.sh: brand gate (old-name content and file names, package-manager mentions outside internal/daemon/path.go, non-portty scottkw links); exit 0/1/2; runs in build.yml"
  - "release.yml builds Portty.app, copies the real iconfile.icns before tar/sign, notarizes with portty-notarize, builds portty-<tag>-darwin-universal.dmg with hdiutil and signs it with codesign"
  - "build.yml matrix entry portty, artifact portty-darwin-universal (build/bin/Portty.app), brand gate and verify-icons steps"
  - "distribute.yml and packaging/homebrew deleted (DIST-03)"
  - "Short Portty README, fresh CHANGELOG, old design docs deleted, frontend/package.json license MIT"
  - "TESTING.md reconciled: Portty title and context name, scottkw/portty gh api commands, gate rows, M-74"
  - "Zero case-insensitive old-name hits over tracked files outside .planning/ (BRND-05)"
affects: [phase-04, phase-06-release]
actuals:
  tokens: 54056
  tasks: 3
  commits: 4
plan_head_before: 44c8116c2d319c237f8fe55b6a0cca10f2785ec9
plan_head_after: 9aea24e7b73e029170d7017fa46a4c4b0f5f06b7
tech-stack:
  added: []
  patterns:
    - "Brand gate patterns spell each banned word with one letter in a bracket class; its output label for the package-manager category is built by string concatenation, so the script never matches itself"
    - "Gate scripts check git's exit status explicitly (set -uo pipefail, no -e) so a no-match status 1 is not confused with a git error"
key-files:
  created:
    - tests/check-brand.sh
  modified:
    - .github/workflows/build.yml
    - .github/workflows/release.yml
    - build.sh
    - tests/build-script.test.sh
    - README.md
    - CHANGELOG.md
    - frontend/package.json
    - CLAUDE.md
    - .claude/CLAUDE.md
    - .planning/PROJECT.md
    - TESTING.md
  deleted:
    - .github/workflows/distribute.yml
    - packaging/homebrew/agenthub.rb.template
    - docs/plugin-architecture-assessment.md
    - docs/superpowers/plans/2026-03-19-settings-modal-3-tab.md
    - docs/superpowers/specs/2026-03-19-settings-modal-3-tab-design.md
key-decisions:
  - "Brand gate is a script (tests/check-brand.sh) run from build.yml, not inline negated shell that set -e would ignore"
  - "Release DMG is a plain hdiutil UDZO image signed with codesign; no create-dmg, no package-manager install at release time (first exercised in Phase 6)"
  - "RELEASE_PUBLISH_TOKEN logic stays; only its comment changed since the tap workflow it fed is gone"
  - "Renamed build.yml context build (portty, darwin/universal, macos-latest); re-applying branch protection is a manual user step when CI is re-enabled"
  - "Historical TESTING.md notes keep their facts but cite current test names and 'upstream issue #NNN' instead of the old repo"
requirements-completed: [BRND-04, BRND-05, BRND-06, DIST-03]
coverage:
  - id: D1
    description: "Brand gate reports zero old-name hits, zero package-manager mentions outside path.go, only scottkw/portty links"
    requirement: BRND-05
    verification:
      - kind: other
        ref: "bash tests/check-brand.sh -> OK: brand gate clean"
        status: pass
  - id: D2
    description: "Release pipeline renamed for Portty with hdiutil DMG and codesign; Homebrew distribution removed"
    requirement: DIST-03
    verification:
      - kind: other
        ref: "ruby YAML parse of workflows; scripts/grep-gate.sh; grep hdiutil/portty DMG name in release.yml"
        status: pass
  - id: D3
    description: "README rewrite with Install, Use It From Another Mac, License; MIT in package.json"
    requirement: BRND-04
    verification:
      - kind: other
        ref: "Task 2 automated verify (sections, lead, ln -sf line, banned-word grep, package.json license)"
        status: pass
      - kind: manual_procedural
        ref: "TESTING.md M-74 (README logo light/dark on GitHub)"
        status: unknown
  - id: D4
    description: "All repository links point to github.com/scottkw/portty or https://portty.app"
    requirement: BRND-06
    verification:
      - kind: other
        ref: "bash tests/check-brand.sh (link check)"
        status: pass
duration: 53min
completed: 2026-11-16
---

# Phase 3 Plan 06: Portty Docs, CI and Release Pipeline Summary

**Homebrew distribution removed, the release pipeline builds and codesigns a `portty-<tag>-darwin-universal.dmg` with hdiutil, a brand gate (tests/check-brand.sh) runs in CI, README/CHANGELOG are rewritten for Portty, and the old product name now has zero hits in tracked files outside .planning/.**

## Performance

- **Duration:** about 53 min
- **Started:** 2026-11-16T18:06:52Z
- **Completed:** 2026-11-16T19:00Z
- **Tasks:** 3/3
- **Files:** 17 changed (5 deleted, 1 created, 11 modified)

## Accomplishments

- tests/check-brand.sh: four checks (content, filename, homebrew, link), exit 1 on findings, 2 on a git error (checked by running it outside a repo: `ERROR: git grep failed (status 128)`, exit 2).
- release.yml: Portty.app throughout, `cp build/darwin/iconfile.icns ...` without `|| true` (RESEARCH Pitfall 3 fixed), `portty-notarize`, `hdiutil create -volname "Portty" ... -format UDZO` then `codesign --force --timestamp`. SHA pins, attestation steps, checksums and the job graph unchanged. The DMG name matches the updater's `dmgAssetFilter`.
- build.yml: matrix `portty`, artifact `portty-darwin-universal` from build/bin/Portty.app, new "Brand gate" step after Checkout and "Verify brand assets" step after the traceability step.
- README (28 lines) and CHANGELOG (15 lines) rewritten; docs/ holds only the two Portty logos.
- TESTING.md: Portty title and overview, renamed context, Section 3 for scottkw/portty plus the manual re-apply note, three gate rows, rewritten Section 5 preconditions, M-74.

## Final Gate Results (Task 3, final tree at 9aea24e7)

| Check | Result |
|-------|--------|
| `bash tests/check-brand.sh` | `OK: brand gate clean` |
| `git grep -n -i -I 'agent[h]ub' -- ':!.planning'` | no output, exit 1 (baseline at plan start: 270 lines in 14 files) |
| `git ls-files -- ':(icase)*agent[h]ub*'` | no output |
| `go build ./... && go vet ./...` | pass |
| `go test -race -short ./...` | pass (exit 0, no FAIL lines; no flake hit) |
| Linux cross-compiles (playwright-fixture, ./internal/...) | pass |
| `pnpm -C frontend install --frozen-lockfile` | pass (lockfile untouched) |
| `pnpm -C frontend exec tsc --noEmit` | pass |
| `pnpm -C frontend test` (vitest) | 91 files, 1173 tests passed |
| Playwright chromium | 7 passed, 9 skipped (known skips) |
| `bash tests/build-script.test.sh` | 37 passed, 0 failed |
| `bash tests/check-traceability-paths.sh` | OK: 106 traceability paths exist |
| `bash scripts/grep-gate.sh` | all action refs SHA-pinned |
| `bash scripts/verify-icons.sh` | 27 passed, 0 failed |
| `wails build -platform darwin/arm64 -tags wailsassets` | exit 0; build/bin/Portty.app, CFBundleIdentifier app.portty.Portty; frontend/src/wailsjs restored, clean |

## Task Commits

1. **Task 1: release pipeline, Homebrew removal, brand gate** - `45cbe33c` (feat)
2. **Fix: brand gate matched its own category label** - `cf614670` (fix)
3. **Task 2: README, CHANGELOG, design docs, MIT metadata, CLAUDE.md** - `d144362c` (docs)
4. **Task 3: TESTING.md reconciled, final gates** - `9aea24e7` (docs)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Brand gate flagged its own `homebrew` output label**
- **Found during:** Task 2 verify
- **Issue:** The plan asks for a `homebrew: ` category prefix, but the literal in tests/check-brand.sh matched the gate's own pattern once the file was tracked. Task 1's verify passed only because the script was still untracked at that point (git grep scans tracked files).
- **Fix:** Build the label as `PM_PREFIX="home""brew"`; output is unchanged.
- **Files modified:** tests/check-brand.sh
- **Commit:** cf614670

**2. Commit hygiene: redid two local commits**
- The first attempt at the fix commit also picked up the staged `git rm` of the old design docs. Before anything was pushed, I ran `git reset --soft` back to 45cbe33c (working tree untouched) and recommitted as cf614670 (gate fix only) and d144362c (Task 2 with the deletions).

### Wording choices inside the plan's latitude
- M-54, M-60, M-61, M-63, M-66, M-67 and M-68: the "quit the installed old app" precondition was dropped. M-54 now says "Stop any older Portty daemon first", because its point is a stale daemon. M-55/M-56 use the separate-sockets sentence.
- Historical notes that named since-renamed tests now cite the current names (`TestSettingsMigration_LegacyV4Keys`, `TestFetchPeerSessionsMeta_IgnoresLegacyFields`, `PorttyNotificationDelegate`).

## Known Stubs

None.

## Risks / Notes for Phase 6

- release.yml cannot run in this phase. The hdiutil DMG holds only Portty.app with no /Applications shortcut, so "drag Portty to Applications" needs a second Finder window. Phase 6 cuts the first release and can add the shortcut if wanted (RESEARCH A3 assumption).
- The sign-macos banner comment still says the codesign/notarize/DMG steps were "MOVED VERBATIM"; that no longer holds for the DMG step. It is harmless and left for Phase 6.

## Manual Follow-ups for the User

- When CI is re-enabled, re-apply branch protection with the renamed context `build (portty, darwin/universal, macos-latest)` using the TESTING.md Section 3 command (not run here).
- Run the end-of-phase manual items M-70..M-74 (two Macs, tray, About, signed identity and notification, README logo on GitHub light/dark).

## Self-Check: PASSED

- FOUND: tests/check-brand.sh, README.md, CHANGELOG.md, TESTING.md
- MISSING (deleted on purpose): packaging/, .github/workflows/distribute.yml, docs/superpowers/, docs/plugin-architecture-assessment.md
- FOUND commits: 45cbe33c, cf614670, d144362c, 9aea24e7 (all ancestors of HEAD)
